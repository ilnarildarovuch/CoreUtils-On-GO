package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	version    = "1.1"
	tabSpaces  = "        "
	maxLineLen = 10 * 1024 * 1024
	pathSep    = ":"
)

var (
	borg       = flag.Bool("b", false, "borg mode")
	borgLong   = flag.Bool("borg", false, "equivalent to -b")
	dead       = flag.Bool("d", false, "dead mode")
	deadLong   = flag.Bool("dead", false, "equivalent to -d")
	greedy     = flag.Bool("g", false, "greedy mode")
	greedyLong = flag.Bool("greedy", false, "equivalent to -g")
	paranoid   = flag.Bool("p", false, "paranoid mode")
	paranoidLong = flag.Bool("paranoid", false, "equivalent to -p")
	stoned     = flag.Bool("s", false, "stoned mode")
	stonedLong = flag.Bool("stoned", false, "equivalent to -s")
	tired      = flag.Bool("t", false, "tired mode")
	tiredLong  = flag.Bool("tired", false, "equivalent to -t")
	wired      = flag.Bool("w", false, "wired mode")
	wiredLong  = flag.Bool("wired", false, "equivalent to -w")
	young      = flag.Bool("y", false, "young mode")
	youngLong  = flag.Bool("young", false, "equivalent to -y")

	eyes       = flag.String("e", "oo", "eye string")
	eyesLong   = flag.String("eyes", "oo", "equivalent to -e")
	cowfile    = flag.String("f", "default.cow", "cow file")
	cowfileLong = flag.String("file", "default.cow", "equivalent to -f")
	tongue     = flag.String("T", "  ", "tongue string")
	tongueLong = flag.String("tongue", "  ", "equivalent to -T")
	wrapCol    = flag.Int("W", 40, "wrap column")
	wrapColLong = flag.Int("wrap", 40, "equivalent to -W")

	expandTabs = flag.Bool("n", false, "expand tabs to spaces")
	noExpandTabsLong = flag.Bool("no-expand-tabs", false, "equivalent to -n")
	help       = flag.Bool("h", false, "show help")
	helpLong   = flag.Bool("help", false, "equivalent to -h")
	list       = flag.Bool("l", false, "list cowfiles")
	listLong   = flag.Bool("list", false, "equivalent to -l")
	showVersion = flag.Bool("version", false, "output version information")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	handleCombinedFlags()

	if *help {
		usage()
		os.Exit(0)
	}

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if *list {
		listCowfiles(getCowpath())
		os.Exit(0)
	}

	messageLines := readMessage()
	balloonLines := constructBalloon(messageLines, isThinkMode())
	eyesVal, tongueVal := configureFace()

	cowPath, err := findCowFile(*cowfile, getCowpath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}

	cowContent, err := generateCow(cowPath, eyesVal, tongueVal, getThoughts())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}

	printOutput(balloonLines, cowContent)
}

func handleCombinedFlags() {
	*borg = *borg || *borgLong
	*dead = *dead || *deadLong
	*greedy = *greedy || *greedyLong
	*paranoid = *paranoid || *paranoidLong
	*stoned = *stoned || *stonedLong
	*tired = *tired || *tiredLong
	*wired = *wired || *wiredLong
	*young = *young || *youngLong
	*eyes = firstNonDefault(*eyes, *eyesLong, "oo")
	*cowfile = firstNonDefault(*cowfile, *cowfileLong, "default.cow")
	*tongue = firstNonDefault(*tongue, *tongueLong, "  ")
	*wrapCol = firstNonZero(*wrapCol, *wrapColLong)
	*expandTabs = *expandTabs || *noExpandTabsLong
	*help = *help || *helpLong
	*list = *list || *listLong
}

func firstNonDefault(a, b, def string) string {
	if a != def {
		return a
	}
	return b
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func usage() {
	fmt.Fprintf(os.Stderr, "cowsay %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [MESSAGE]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s -e ^^ Hello!\n  %s -f tux \"Linux rocks\"\n", 
		os.Args[0], os.Args[0])
	os.Exit(0)
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func getCowpath() string {
	if cp := os.Getenv("COWPATH"); cp != "" {
		return cp
	}
	return "/usr/share/cowsay/cows"
}

func readMessage() []string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), maxLineLen)

	var lines []string
	if flag.NArg() == 0 {
		for scanner.Scan() {
			lines = append(lines, processLine(scanner.Text())...)
		}
	} else {
		lines = processLine(strings.Join(flag.Args(), " "))
	}

	if !*expandTabs {
		return wrapText(strings.Join(lines, " "), *wrapCol)
	}
	return lines
}

func processLine(line string) []string {
	if *expandTabs {
		return []string{strings.ReplaceAll(line, "\t", tabSpaces)}
	}
	return []string{line}
}

func constructBalloon(lines []string, isThink bool) []string {
	if len(lines) == 0 {
		return []string{"< no message >"}
	}

	maxLen := maxLineLength(lines)
	var balloon []string

	top := " " + strings.Repeat("_", maxLen+2)
	balloon = append(balloon, top)

	for i, line := range lines {
		left, right := getBorders(i, len(lines), isThink)
		padded := fmt.Sprintf("%-*s", maxLen, line)
		balloon = append(balloon, fmt.Sprintf("%s %s %s", left, padded, right))
	}

	bottom := " " + strings.Repeat("-", maxLen+2)
	return append(balloon, bottom)
}

func maxLineLength(lines []string) int {
	max := 0
	for _, line := range lines {
		if len(line) > max {
			max = len(line)
		}
	}
	return max
}

func getBorders(index, total int, isThink bool) (string, string) {
	if isThink {
		return "(", ")"
	}
	if total == 1 {
		return "<", ">"
	}
	switch index {
	case 0:
		return "/", "\\"
	case total-1:
		return "\\", "/"
	default:
		return "|", "|"
	}
}

func configureFace() (string, string) {
	e, t := *eyes, *tongue
	switch {
	case *borg: e = "=="
	case *dead: e, t = "xx", "U "
	case *greedy: e = "$$"
	case *paranoid: e = "@@"
	case *stoned: e, t = "**", "U "
	case *tired: e = "--"
	case *wired: e = "OO"
	case *young: e = ".."
	}
	return ensureLength(e, 2), ensureLength(t, 2)
}

func ensureLength(s string, l int) string {
	if len(s) > l {
		return s[:l]
	}
	return s + strings.Repeat(" ", l-len(s))
}

func generateCow(path, eyes, tongue, thoughts string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read cow file: %w", err)
	}

	replacer := strings.NewReplacer(
		"$eyes", eyes,
		"${eyes}", eyes,
		"$tongue", tongue,
		"${tongue}", tongue,
		"$thoughts", thoughts,
	)
	return replacer.Replace(string(content)), nil
}

func isThinkMode() bool {
	return strings.Contains(strings.ToLower(os.Args[0]), "think")
}

func getThoughts() string {
	if isThinkMode() {
		return "o"
	}
	return "\\"
}

func wrapText(text string, width int) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		result = append(result, wrapLine(line, width)...)
	}
	return result
}

func wrapLine(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	return append(lines, current)
}

func listCowfiles(cowpath string) {
	found := make(map[string]bool)
	for _, dir := range strings.Split(cowpath, pathSep) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var cows []string
		for _, entry := range entries {
			if name := entry.Name(); strings.HasSuffix(name, ".cow") {
				cows = append(cows, name[:len(name)-4])
				found[name[:len(name)-4]] = true
			}
		}

		if len(cows) > 0 {
			fmt.Printf("Cow files in %s:\n", dir)
			sort.Strings(cows)
			fmt.Println(strings.Join(cows, " "))
		}
	}

	if len(found) == 0 {
		fmt.Fprintf(os.Stderr, "%s: no cowfiles found in %s\n", os.Args[0], cowpath)
	}
}

func findCowFile(name, cowpath string) (string, error) {
	if strings.Contains(name, string(os.PathSeparator)) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		if _, err := os.Stat(name + ".cow"); err == nil {
			return name + ".cow", nil
		}
		return "", fmt.Errorf("cowfile not found: %s", name)
	}

	for _, dir := range strings.Split(cowpath, pathSep) {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		if _, err := os.Stat(path + ".cow"); err == nil {
			return path + ".cow", nil
		}
	}

	return "", fmt.Errorf("cowfile '%s' not found in COWPATH", name)
}

func printOutput(balloon []string, cow string) {
	for _, line := range balloon {
		fmt.Println(line)
	}
	fmt.Println(cow)
}