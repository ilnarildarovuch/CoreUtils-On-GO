package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"bufio"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Text formatting
	Bold      string `json:"bold"`
	Separator string `json:"separator"`

	// Color blocks
	ColorBlocks string `json:"color_blocks"`
	BlockWidth  int    `json:"block_width"`
	BlockRange  []int  `json:"block_range"`
}

type SystemInfo struct {
	Title  string
	OS     string
	Host   string
	User   string
	Kernel string
	Uptime string
}

var (
	config Config
	info   SystemInfo

	// ANSI color codes
	reset         = "\033[0m"
	bold          = "\033[1m"
	subtitleColor = "\033[35m"
	colonColor    = "\033[36m"
	infoColor     = "\033[37m"

	colorBlocks = []string{
		"\033[30m", "\033[31m", "\033[32m", "\033[33m",
		"\033[34m", "\033[35m", "\033[36m", "\033[37m",
		"\033[90m", "\033[91m", "\033[92m", "\033[93m",
		"\033[94m", "\033[95m", "\033[96m", "\033[97m",
	}
)

func main() {
	initConfig()
	gatherInfo()
	info.Title = fmt.Sprintf("%s@%s", info.User, info.Host)
	displayInfo()
}

func initConfig() {
	config = Config{
		Bold:       "on",
		Separator:  ":",
		ColorBlocks: "on",
		BlockWidth:  3,
		BlockRange:  []int{0, 15},
	}
}

func gatherInfo() {
	info.OS = detectOS()
	info.Host = getHost()
	info.User = getUser()
	info.Kernel = getKernel()
	info.Uptime = getUptime()
}

func displayInfo() {
	printAscii()

	if config.ColorBlocks == "on" {
		displayColorBlocks()
	}
}

func detectOS() string { // Ummmm...
	switch runtime.GOOS {
	case "linux":
		return "X/Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	case "netbsd":
		return "NetBSD"
	default:
		return runtime.GOOS
	}
}

func getHost() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "Unknown"
	}
	return hostname
}

func getUser() string {
	current, err := user.Current()
	if err != nil {
		return "Unknown"
	}
	return current.Username
}

func getKernel() string {
	switch runtime.GOOS {
	case "linux", "darwin":
		out, err := exec.Command("/bin/uname", "-r").Output()
		if err != nil {
			return "Unknown"
		}
		return strings.TrimSpace(string(out))
	default:
		out, err := exec.Command("/bin/uname", "-sr").Output()
		if err != nil {
			return "Unknown"
		}
		return strings.TrimSpace(string(out))
	}
}

func getUptime() string {
	switch runtime.GOOS {
	case "linux":
		file, err := os.Open("/proc/uptime")
		if err != nil {
			return "Unknown"
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Scan()
		fields := strings.Fields(scanner.Text())
		if len(fields) < 1 {
			return "Unknown"
		}

		uptime, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return "Unknown"
		}

		return formatUptime(uptime)
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
		if err != nil {
			return "Unknown"
		}

		bootTimeStr := strings.TrimSpace(string(out))
		secIndex := strings.Index(bootTimeStr, "sec = ")
		if secIndex == -1 {
			return "Unknown"
		}

		secStr := bootTimeStr[secIndex+6:]
		commaIndex := strings.Index(secStr, ",")
		if commaIndex == -1 {
			return "Unknown"
		}

		secStr = secStr[:commaIndex]
		sec, err := strconv.ParseInt(secStr, 10, 64)
		if err != nil {
			return "Unknown"
		}

		uptime := time.Now().Unix() - sec
		return formatUptime(float64(uptime))
	default:
		return "Unknown"
	}
}

func formatUptime(seconds float64) string {
	days := int(seconds / 86400)
	hours := int((seconds - float64(days*86400)) / 3600)
	minutes := int((seconds - float64(days*86400) - float64(hours*3600)) / 60)
	secs := int(seconds - float64(days*86400) - float64(hours*3600) - float64(minutes*60))

	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
}

func printInfo(title, value string) string {
	if value == "" {
		return ""
	}

	separator := config.Separator
	if separator == ":" {
		separator = colonColor + ":" + infoColor
	}

	return fmt.Sprintf("%s%s%s%s %s",
		subtitleColor, bold, title,
		separator,
		infoColor+value+reset)
}

func printAscii() {
	asciiArt :=
		"         _nnnn_            " + info.Title + "\n" +
		"        dGGGGMMb           " + "-------------" + "\n" +
		"       @p~qp~~qMb          " + printInfo("OS", info.OS) + "\n" +
		"       M|@||@) M|          " + printInfo("Host", info.Host) + "\n" +
		"       @,----.JM|          " + printInfo("Kernel", info.Kernel) + "\n" +
		"      JS^\\__/  qKL         " + printInfo("Uptime", info.Uptime) + "\n" +
		"     dZP        qKRb\n" +
		"    dZP          qKKb\n" +
		"   fZP            SMMb\n" +
		"   HZM            MMMM\n" +
		"   FqM            MMMM\n" +
		" __| ''.        |\\dS''qML\n" +
		" |    `.       | `' \\Zq\n" +
		"_)      \\.___.,|     .'\n" +
		"\\____   )MMMMMP|   .'\n" +
		"     `-'       `--'\n"

	fmt.Println(asciiArt)
}

func displayColorBlocks() {
	for i := config.BlockRange[0]; i <= config.BlockRange[1]; i++ {
		if i > 0 && i%8 == 0 {
			fmt.Println()
		}

		block := strings.Repeat(" ", config.BlockWidth)
		fmt.Printf("%s%s%s", colorBlocks[i], block, reset)
	}
	fmt.Println()
}
