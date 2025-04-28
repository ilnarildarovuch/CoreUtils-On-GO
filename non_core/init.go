package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Config
const (
	INITTAB_PATH  = "/etc/inittab"
	SYSLOG_PATH   = "/var/log/syslog" 
	RC_DIR        = "/etc/rc.d"
	INITCTL_FIFO  = "/run/initctl"
	CGROUP_ROOT   = "/sys/fs/cgroup"
	RECOVERY_MODE = "recovery"
)

// Structs
type Process struct {
	ID        string
	Runlevels string
	Action    string
	Command   string
	PID       int
	Status    string
	Cgroup    string
	Namespace string
}

type Runlevel struct {
	Level     string
	Services []string
}

var (
	currentRunlevel  = "2"
	processes       []Process
	namespaces      = make(map[string]bool)
	cgroups         = make(map[string]string)
	recoveryMode    = false
)

func main() {
	if os.Getpid() != 1 {
		PrintLn("Must run as PID 1")
		os.Exit(1)
	}
	fmt.Printf("\033[2J\033[1;1H")
	PrintLn("Starting initialization...")

	initSystem()
	startBootSequence()
	startEmergencyShell()
}

// System init
func initSystem() {
	checkRecoveryMode()
	mountVirtualFS()
	loadInittab()
	initCgroups()
	createNamespaces()
	setupTTY()
	setupSignals()
	startInitctlServer()
}

func startBootSequence() {
	PrintLn("Starting system boot sequence")
	manageServices(currentRunlevel)
}

func setupTTY() {
	PrintLn("Initializing TTY")
	syscall.Setsid()
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(0), uintptr(syscall.TIOCSCTTY), 1)

	os.Setenv("PATH", os.Getenv("PATH")+":/bin")

	f, _ := os.OpenFile("/proc/sys/kernel/printk", os.O_WRONLY, 0) // only critical
	defer f.Close()
	f.WriteString("2 0 0 0")
}


func checkRecoveryMode() {
	PrintLn("Checking for recovery mode")
	if kernelParamExists(RECOVERY_MODE) {
		PrintLn("Entering recovery mode...")
		recoveryMode = true
		currentRunlevel = "1"
		enableSingleUserMode()
	}
}

func enableSingleUserMode() {
	PrintLn("Entering single user mode")
	// emergency shell
	cmd := exec.Command("/bin/sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func kernelParamExists(param string) bool {
	data, _ := ioutil.ReadFile("/proc/cmdline")
	return strings.Contains(string(data), param)
}

func initCgroups() {
	PrintLn("Initializing cgroups")
	for _, subsys := range []string{"cpu", "memory", "devices"} {
		path := filepath.Join(CGROUP_ROOT, subsys, "init.scope")
		os.MkdirAll(path, 0755)
		cgroups[subsys] = path
	}
}

func applyCgroup(pid int) {
	for _, path := range cgroups {
		tasks := filepath.Join(path, "tasks")
		ioutil.WriteFile(tasks, []byte(fmt.Sprint(pid)), 0644)
	}
}

func createNamespaces() {
	PrintLn("Creating namespaces")
	namespaces["pid"] = true
	namespaces["mount"] = true
}

func createNamespace(proc *Process) {
	flags := syscall.CLONE_NEWPID
	if proc.Namespace == "full" {
		flags |= syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC
	}
	syscall.Syscall(syscall.SYS_UNSHARE, uintptr(flags), 0, 0)
}

// LSB (does not work)
func executeLSB(script string, action string) error {
	cmd := exec.Command("/bin/sh", script, action)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	return cmd.Run()
}

func manageServices(runlevel string) {
	Printf("Managing services for runlevel %s\n", runlevel)
	dir := filepath.Join(RC_DIR, "rc"+runlevel+".d")
	files, _ := ioutil.ReadDir(dir)

	for _, f := range files {
		script := filepath.Join(dir, f.Name())
		switch {
		case strings.HasPrefix(f.Name(), "S"):
			executeLSB(script, "start")
		case strings.HasPrefix(f.Name(), "K"):
			executeLSB(script, "stop")
		}
	}
}

func loadInittab() {
	PrintLn("Loading inittab")
	file, _ := os.Open(INITTAB_PATH)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parseInittabLine(scanner.Text())
	}
}

func parseInittabLine(line string) {
	if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		return
	}

	parts := strings.Split(line, ":")
	if len(parts) < 4 {
		return
	}

	process := Process{
		ID:        parts[0],
		Runlevels: parts[1],
		Action:    parts[2],
		Command:   parts[3],
	}
	processes = append(processes, process)
}

func startProcess(proc *Process) {
	PrintLn("Starting process", proc.ID)
	cmd := exec.Command("/bin/sh", "-c", proc.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:     true,
		Cloneflags: getCloneFlags(proc),
	}

	applyCgroup(os.Getpid())
	createNamespace(proc)

	if err := cmd.Start(); err != nil {
		logError(proc.ID, err)
		return
	}

	proc.PID = cmd.Process.Pid
	monitorProcess(proc, cmd)
}

func monitorProcess(proc *Process, cmd *exec.Cmd) {
	go func() {
		cmd.Wait()
		if proc.Action == "respawn" && !recoveryMode {
			startProcess(proc)
		}
	}()
}

func setupSignals() {
	PrintLn("Setting up signals")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGCHLD,
		syscall.SIGUSR1,
	)

	go handleSignals(sigCh)
}

func handleSignals(ch <-chan os.Signal) {
	for sig := range ch {
		switch sig {
		case syscall.SIGHUP:
			reloadConfig()
		case syscall.SIGUSR1:
			enterRecoveryMode()
		}
	}
}

// Helpers
func mountVirtualFS() {
	syscall.Mount("proc", "/proc", "proc", 0, "")
	syscall.Mount("sysfs", "/sys", "sysfs", 0, "")
	syscall.Mount("udev", "/dev", "devtmpfs", 0, "")
}

func getCloneFlags(proc *Process) uintptr {
	flags := syscall.CLONE_NEWPID
	if namespaces["mount"] {
		flags |= syscall.CLONE_NEWNS
	}
	return uintptr(flags)
}

func logError(id string, err error) {
	Printf("Error in process %s: %v\n", id, err)
}

func enterRecoveryMode() {
	PrintLn("Entering recovery mode")
	recoveryMode = true
	killAllProcesses()
	startEmergencyShell()
}

func killAllProcesses() {
	PrintLn("Terminating all processes")
	syscall.Kill(-1, syscall.SIGTERM)
	time.Sleep(2 * time.Second)
	syscall.Kill(-1, syscall.SIGKILL)
}

func startEmergencyShell() {
	PrintLn("Starting emergency shell")
	cmd := exec.Command("/bin/sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func startInitctlServer() {
	PrintLn("Starting initctl server")
	os.Remove(INITCTL_FIFO)
	syscall.Mkfifo(INITCTL_FIFO, 0600)

	go func() {
		f, _ := os.Open(INITCTL_FIFO)
		defer f.Close()

		for {
			var req struct {
				Magic    [4]byte
				Cmd      int32
				Runlevel int32
			}
			binary.Read(f, binary.LittleEndian, &req)
			handleInitRequest(req)
		}
	}()
}

func handleInitRequest(req struct {
	Magic    [4]byte
	Cmd      int32
	Runlevel int32
}) {
	if string(req.Magic[:]) != "INIT" {
		return
	}

	switch req.Cmd {
	case 0: // Change runlevel
		changeRunlevel(string(rune(req.Runlevel)))
	case 1: // Shutdown
		shutdown()
	}
}

func reloadConfig() {
	PrintLn("Reloading configuration")
	loadInittab()
}

func changeRunlevel(level string) {
	Printf("Changing to runlevel %s\n", level)
	currentRunlevel = level
	manageServices(level)
}

func shutdown() {
	PrintLn("Shutting down")
	manageServices("0")
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func debug(format string, a ...interface{}) {
	f, _ := os.OpenFile(SYSLOG_PATH, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.WriteString(fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, a...)))
	defer f.Close()
}

func Printf(format string, a ...interface{}) (n int, err error) {
	debug(format, a)
	return fmt.Printf("[\033[32m*\033[0m] " + format, a...)
}

func PrintLn(a ...interface{}) (n int, err error) {
	debug("", a)
	return fmt.Println(append([]interface{}{"[\033[32m*\033[0m] "}, a...)...)
}