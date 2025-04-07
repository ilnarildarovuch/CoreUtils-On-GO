package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strings"
	"os"
	"path/filepath"
)

const (
	version    = "1.1.0"
	maxLineLen = 10 * 1024 * 1024 // 10MB
)

var (
	number              = flag.Bool("n", false, "number all output lines")
	numberLong          = flag.Bool("number", false, "equivalent to -n")
	numberNonblank      = flag.Bool("b", false, "number nonempty output lines, overrides -n")
	numberNonblankLong  = flag.Bool("number-nonblank", false, "equivalent to -b")
	squeezeBlank        = flag.Bool("s", false, "suppress repeated empty output lines")
	squeezeBlankLong    = flag.Bool("squeeze-blank", false, "equivalent to -s")
	showEnds            = flag.Bool("E", false, "display $ at end of each line")
	showEndsLong        = flag.Bool("show-ends", false, "equivalent to -E")
	showNonprinting     = flag.Bool("v", false, "use ^ and M- notation, except for LFD and TAB")
	showNonprintingLong = flag.Bool("show-nonprinting", false, "equivalent to -v")
	showTabs            = flag.Bool("T", false, "display TAB characters as ^I")
	showTabsLong        = flag.Bool("show-tabs", false, "equivalent to -T")
	showAll             = flag.Bool("A", false, "equivalent to -vET")
	showAllLong         = flag.Bool("show-all", false, "equivalent to -vET")
	showET              = flag.Bool("e", false, "equivalent to -vE")
	showETLong          = flag.Bool("show-ends-tab", false, "equivalent to -vE")
	showVT              = flag.Bool("t", false, "equivalent to -vT")
	showVTLong          = flag.Bool("show-tabs-nonprinting", false, "equivalent to -vT")
	showVersion         = flag.Bool("version", false, "output version information and exit")
	followSymlinks      = flag.Bool("L", false, "follow symbolic links (default false)")
)

func main() {

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	handleCombinedOptions()

	exitCode := 0
	files := getInputFiles()

	for _, file := range files {
		if err := processFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], file, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Secure cat %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [FILE]...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s -n file.txt\n  %s -v binary.data\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func handleCombinedOptions() {
	if *showAll {
		*showNonprinting = true
		*showEnds = true
		*showTabs = true
	}
	if *showET {
		*showNonprinting = true
		*showEnds = true
	}
	if *showVT {
		*showNonprinting = true
		*showTabs = true
	}
	if *numberNonblank {
		*number = true
	}
}

func getInputFiles() []string {
	files := flag.Args()
	if len(files) == 0 {
		return []string{"-"}
	}
	return files
}

func processFile(filename string) error {
	if filename == "-" {
		return processStdin()
	}
	return processRegularFile(filename)
}

func processStdin() error {
	if *showNonprinting || *showTabs || *showEnds || *number || *numberNonblank || *squeezeBlank {
		return processWithOptions(os.Stdin, os.Stdout)
	}
	_, err := io.Copy(os.Stdout, os.Stdin)
	return err
}

func processRegularFile(filename string) error {
	if _, err := os.Lstat(filename); err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}

	var input *os.File
	var err error

	if *followSymlinks {
		input, err = os.Open(filename)
	} else {
		input, err = os.OpenFile(filename, os.O_RDONLY, 0)
		if err != nil && os.IsPermission(err) {
			if resolved, err := filepath.EvalSymlinks(filename); err == nil && resolved != filename {
				return fmt.Errorf("refusing to follow symlink")
			}
			input, err = os.Open(filename)
		}
	}

	if err != nil {
		return fmt.Errorf("cannot open: %w", err)
	}
	defer safeClose(input)

	if *showNonprinting || *showTabs || *showEnds || *number || *numberNonblank || *squeezeBlank {
		return processWithOptions(input, os.Stdout)
	}

	_, err = io.Copy(os.Stdout, input)
	return err
}

func safeClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: error closing file: %v\n", err)
	}
}

func processWithOptions(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineLen)

	lineNum := 1
	prevBlank := false

	for scanner.Scan() {
		line := scanner.Text()
		isBlank := len(line) == 0

		if *squeezeBlank && isBlank && prevBlank {
			continue
		}
		prevBlank = isBlank

		if (*number && !*numberNonblank) || (*numberNonblank && !isBlank) {
			fmt.Fprintf(output, "%6d\t", lineNum)
			lineNum++
		}

		if *showNonprinting || *showTabs {
			line = processNonprinting(line)
		}

		fmt.Fprint(output, line)

		if *showEnds {
			fmt.Fprint(output, "$")
		}

		fmt.Fprintln(output)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	return nil
}

func processNonprinting(s string) string {
	if !(*showNonprinting || *showTabs) {
		return s
	}

	var buf strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' && *showTabs:
			buf.WriteString("^I")
		case r >= 32 && r < 127:
			buf.WriteRune(r)
		case r == 127:
			buf.WriteString("^?")
		case r < 32:
			buf.WriteString(fmt.Sprintf("^%c", r+64))
		case r >= 128 && r < 128+32:
			buf.WriteString(fmt.Sprintf("M-^%c", r-128+64))
		case r >= 128+32 && r < 128+127:
			buf.WriteString(fmt.Sprintf("M-%c", r-128))
		case r >= 128+127:
			buf.WriteString("M-^?")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}