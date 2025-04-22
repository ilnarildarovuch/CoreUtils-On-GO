package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "strings"
    "syscall"
    "time"
)

var (
    envVars = make(map[string]string)
)

func updateUptime() {
    startTime := time.Now()
    idleTime := 0.0

    for {
        uptimeFile, err := os.OpenFile("/proc/uptime", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
	    fmt.Printf("Error opening uptime file: %s\n", err)
	    return
        }

        elapsed := time.Since(startTime).Seconds()
        _, err = uptimeFile.WriteString(fmt.Sprintf("%.1f %.1f\n", elapsed, idleTime))
        if err != nil {
            fmt.Printf("Error writing to uptime file: %s\n", err)
            return
        }

        uptimeFile.Close()
        time.Sleep(1 * time.Second)
    }
}

func main() {
    // only if proc is unmounted, too hacky:
    go updateUptime()
    fmt.Print("\033[H\033[2J") // clear screen
    help, _ := os.ReadFile("/usr/possibilities")

    sigintChan := make(chan os.Signal, 1)
    sigtstpChan := make(chan os.Signal, 1)
    signal.Notify(sigintChan, syscall.SIGINT)
    signal.Notify(sigtstpChan, syscall.SIGTSTP)

    scanner := bufio.NewScanner(os.Stdin)

    for {
        cwd, _ := os.Getwd()
        fmt.Printf("default@localhost:%s$ ", cwd) // hard-coded

        scanner.Scan()
        input := scanner.Text()

        if input == "" {
            continue
        }

        args := strings.Fields(input)
        if len(args) == 0 {
            continue
        }

        switch args[0] {
        case "exit", "quit":
            fmt.Println("Goodbye!")
            time.Sleep(time.Second)
			syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
			return
		case "reboot":
			fmt.Println("Rebooting...")
			time.Sleep(time.Second)
			syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
			return
		case "help":
			fmt.Println(string(help))
			continue
        case "export":
            if len(args) < 2 {
                fmt.Println("Usage: export VAR=value")
                continue
            }
            parts := strings.SplitN(args[1], "=", 2)
            if len(parts) != 2 {
                fmt.Println("Usage: export VAR=value")
                continue
            }
            envVars[parts[0]] = parts[1]
            continue
        case "cd":
            if len(args) < 2 {
                homeDir, err := os.UserHomeDir()
                if err != nil {
                    fmt.Printf("Error getting home directory: %v\n", err)
                    continue
                }
                err = syscall.Chdir(homeDir)
                if err != nil {
                    fmt.Printf("Error changing directory: %v\n", err)
                }
                continue
            }
            err := syscall.Chdir(args[1])
            if err != nil {
                fmt.Printf("Error changing directory: %v\n", err)
            }
            continue
        }

        // Handle redirection
        var outputFile *os.File
        redirectIndex := -1
        redirectMode := ""

        for i, arg := range args {
            if arg == ">" || arg == ">>" {
                if i+1 >= len(args) {
                    fmt.Println("Error: Missing filename after redirection operator")
                    continue
                }
                redirectIndex = i
                redirectMode = arg
                break
            }
        }

        var cmdArgs []string
        if redirectIndex != -1 {
            cmdArgs = args[:redirectIndex]
            filename := args[redirectIndex+1]

            var err error
            if redirectMode == ">" {
                outputFile, err = os.Create(filename)
            } else {
                outputFile, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
            }

            if err != nil {
                fmt.Printf("Error opening file: %s\n", err)
                continue
            }
            defer outputFile.Close()
        } else {
            cmdArgs = args
        }

        cmd := exec.Command("/bin/" + cmdArgs[0], cmdArgs[1:]...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr

        // Apply environment variables
        for key, value := range envVars {
            cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
        }

        if outputFile != nil {
            cmd.Stdout = outputFile
        }

        err := cmd.Start()
        if err != nil {
            fmt.Printf("Error executing command: %s\n", err)
            continue
        }

        go func() {
            select {
            case <-sigintChan:
                if cmd.Process != nil {
                    cmd.Process.Signal(syscall.SIGINT)
                }
            case <-sigtstpChan:
                if cmd.Process != nil {
                    cmd.Process.Signal(syscall.SIGTSTP)
                }
            }
        }()

        err = cmd.Wait()
        if err != nil {
            fmt.Printf("Error executing command: %s\n", err)
        }
    }
}
