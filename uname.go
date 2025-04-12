package main

import (
	"flag"
	"fmt"
	"strings"
	"os"
	"runtime"
	"syscall"
)

const (
	version = "1.0.0"
	PrintKernelName        = 1 << iota
	PrintNodename
	PrintKernelRelease
	PrintKernelVersion
	PrintMachine
	PrintProcessor
	PrintHardwarePlatform
	PrintOperatingSystem
)

var (
	all              = flag.Bool("a", false, "print all information")
	kernelName       = flag.Bool("s", false, "print the kernel name")
	nodename         = flag.Bool("n", false, "print the network node hostname")
	kernelRelease    = flag.Bool("r", false, "print the kernel release")
	kernelVersion    = flag.Bool("v", false, "print the kernel version")
	machine          = flag.Bool("m", false, "print the machine hardware name")
	processor        = flag.Bool("p", false, "print the processor type")
	hardwarePlatform = flag.Bool("i", false, "print the hardware platform")
	operatingSystem  = flag.Bool("o", false, "print the operating system")
	showVersion      = flag.Bool("version", false, "output version information and exit")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	toprint := decodeSwitches()

	if toprint == 0 {
		toprint = PrintKernelName
	}

	printSystemInfo(toprint)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, `
Print certain system information. With no OPTION, same as -s.

Options:
  -a, --all                print all information
  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
  -v, --kernel-version     print the kernel version
  -m, --machine            print the machine hardware name
  -p, --processor          print the processor type
  -i, --hardware-platform  print the hardware platform
  -o, --operating-system   print the operating system
  --version                output version information and exit`)
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func decodeSwitches() int {
	var toprint int

	if *all {
		return ^0 // Set all bits
	}

	if *kernelName {
		toprint |= PrintKernelName
	}
	if *nodename {
		toprint |= PrintNodename
	}
	if *kernelRelease {
		toprint |= PrintKernelRelease
	}
	if *kernelVersion {
		toprint |= PrintKernelVersion
	}
	if *machine {
		toprint |= PrintMachine
	}
	if *processor {
		toprint |= PrintProcessor
	}
	if *hardwarePlatform {
		toprint |= PrintHardwarePlatform
	}
	if *operatingSystem {
		toprint |= PrintOperatingSystem
	}

	return toprint
}

func printSystemInfo(toprint int) {
	var utsname syscall.Utsname
	if err := syscall.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system name: %v\n", os.Args[0], err)
		os.Exit(1)
	}

	printed := false
	printElement := func(s string) {
		if printed {
			fmt.Print(" ")
		}
		fmt.Print(s)
		printed = true
	}

	if toprint&PrintKernelName != 0 {
		printElement(utsnameString(int8ToByte(utsname.Sysname)))
	}
	if toprint&PrintNodename != 0 {
		printElement(utsnameString(int8ToByte(utsname.Nodename)))
	}
	if toprint&PrintKernelRelease != 0 {
		printElement(utsnameString(int8ToByte(utsname.Release)))
	}
	if toprint&PrintKernelVersion != 0 {
		printElement(utsnameString(int8ToByte(utsname.Version)))
	}
	if toprint&PrintMachine != 0 {
		printElement(utsnameString(int8ToByte(utsname.Machine)))
	}
	if toprint&PrintProcessor != 0 {
		printElement(getProcessorInfo())
	}
	if toprint&PrintHardwarePlatform != 0 {
		printElement(getHardwarePlatform())
	}
	if toprint&PrintOperatingSystem != 0 {
		printElement(runtime.GOOS)
	}

	fmt.Println()
}

func utsnameString(buf [65]byte) string {
	var i int
	for ; i < len(buf); i++ {
		if buf[i] == 0 {
			break
		}
	}
	return string(buf[:i])
}

func int8ToByte(arr [65]int8) [65]byte {
	var byteArr [65]byte
	for i, v := range arr {
		byteArr[i] = byte(v)
	}
	return byteArr
}

func getProcessorInfo() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "model name") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return runtime.GOARCH
}

func getHardwarePlatform() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/sys/firmware/devicetree/base/model"); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return runtime.GOARCH
}
