package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "os/signal"
    "os/user"
    "strings"
    "syscall"
    "time"
)

var (
    envVars = make(map[string]string)
)

func main() {
    help, _ := os.ReadFile("/usr/possibilities")
    rc_message, _ := os.ReadFile("/usr/.rcm")

    sigintChan := make(chan os.Signal, 1)
    sigtstpChan := make(chan os.Signal, 1)
    signal.Notify(sigintChan, syscall.SIGINT)
    signal.Notify(sigtstpChan, syscall.SIGTSTP)

    scanner := bufio.NewScanner(os.Stdin)

    fmt.Println(string(rc_message))

mainLoop:
    for {
        cwd, _ := os.Getwd()
        host, _ := os.Hostname()
        user, _ := user.Current()

        fmt.Printf("\033[32m%s@%s\033[0m:\033[34m%s\033[0m$ ", user.Username, host, cwd)

        scanner.Scan()
        input := scanner.Text()

        if input == "" {
            continue
        }

        args := strings.Fields(input)
        if len(args) == 0 {
            continue
        }

        pipelineCommands := splitPipeline(args)

        valid := true
        for _, cmd := range pipelineCommands {
            if len(cmd) == 0 {
                fmt.Println("Invalid pipe syntax")
                valid = false
                break
            }
        }
        if !valid {
            continue
        }

        if len(pipelineCommands) > 1 {
            for _, cmdParts := range pipelineCommands {
                if len(cmdParts) == 0 {
                    continue
                }
                cmdName := cmdParts[0]
                switch cmdName {
                case "exit", "quit", "reboot", "help", "export", "cd":
                    fmt.Printf("Error: Built-in command '%s' cannot be part of a pipeline\n", cmdName)
                    continue mainLoop
                }
            }

            lastCmdParts := pipelineCommands[len(pipelineCommands)-1]
            var outputFile *os.File
            redirectIndex := -1
            redirectMode := ""

            for i, arg := range lastCmdParts {
                if arg == ">" || arg == ">>" {
                    if i+1 >= len(lastCmdParts) {
                        fmt.Println("Error: Missing filename after redirection operator")
                        continue mainLoop
                    }
                    redirectIndex = i
                    redirectMode = arg
                    break
                }
            }

            var lastCmdArgs []string
            if redirectIndex != -1 {
                lastCmdArgs = lastCmdParts[:redirectIndex]
                filename := lastCmdParts[redirectIndex+1]

                var err error
                if redirectMode == ">" {
                    outputFile, err = os.Create(filename)
                } else {
                    outputFile, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
                }

                if err != nil {
                    fmt.Printf("Error opening file: %s\n", err)
                    continue mainLoop
                }
                defer outputFile.Close()
            } else {
                lastCmdArgs = lastCmdParts
            }

            cmds := make([]*exec.Cmd, len(pipelineCommands))
            for i, parts := range pipelineCommands {
                if i == len(pipelineCommands)-1 {
                    cmds[i] = exec.Command(lastCmdArgs[0], lastCmdArgs[1:]...)
                    cmds[i].Stdout = os.Stdout
                    cmds[i].Stderr = os.Stderr
                    if outputFile != nil {
                        cmds[i].Stdout = outputFile
                    }
                } else {
                    cmds[i] = exec.Command(parts[0], parts[1:]...)
                    cmds[i].Stderr = os.Stderr
                }
                cmds[i].Env = os.Environ()
                for key, value := range envVars {
                    cmds[i].Env = append(cmds[i].Env, fmt.Sprintf("%s=%s", key, value))
                }
            }

            var pipes []*os.File
            for i := 0; i < len(cmds)-1; i++ {
                reader, writer, err := os.Pipe()
                if err != nil {
                    fmt.Printf("Error creating pipe: %v\n", err)
                    continue mainLoop
                }
                cmds[i].Stdout = writer
                cmds[i+1].Stdin = reader
                pipes = append(pipes, reader, writer)
            }

            for _, cmd := range cmds {
                err := cmd.Start()
                if err != nil {
                    fmt.Printf("Error starting command: %v\n", err)
                    for _, p := range pipes {
                        p.Close()
                    }
                    continue mainLoop
                }
            }

            for _, p := range pipes {
                p.Close()
            }

            done := make(chan struct{})
            go func() {
                for {
                    select {
                    case <-sigintChan:
                        for _, cmd := range cmds {
                            if cmd.Process != nil {
                                cmd.Process.Signal(syscall.SIGINT)
                            }
                        }
                    case <-sigtstpChan:
                        for _, cmd := range cmds {
                            if cmd.Process != nil {
                                cmd.Process.Signal(syscall.SIGTSTP)
                            }
                        }
                    case <-done:
                        return
                    }
                }
            }()

            for _, cmd := range cmds {
                err := cmd.Wait()
                if err != nil {
                    if exitErr, ok := err.(*exec.ExitError); ok {
                        if exitErr.ProcessState.ExitCode() == -1 {
                            continue
                        }
                    }
                    fmt.Printf("Error waiting for command: %v\n", err)
                }
            }
            close(done)

            continue
        }

        args = pipelineCommands[0]

        switch args[0] {
        case "exit":
            os.Exit(0)
        case "quit":
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
                    continue mainLoop
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

        cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
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
            if strings.Contains(err.Error(), "no") {
                fmt.Println(err.Error())
            }
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

        cmd.Wait()
    }
}

func splitPipeline(args []string) [][]string {
    var commands [][]string
    var currentCmd []string
    for _, arg := range args {
        if arg == "|" {
            if len(currentCmd) > 0 {
                commands = append(commands, currentCmd)
                currentCmd = nil
            }
        } else {
            currentCmd = append(currentCmd, arg)
        }
    }
    if len(currentCmd) > 0 {
        commands = append(commands, currentCmd)
    }
    return commands
}