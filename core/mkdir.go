package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	version = "1.0.0"
)

var (
	mode          = flag.String("m", "", "set file mode (as in chmod)")
	modeLong      = flag.String("mode", "", "equivalent to -m")
	parents       = flag.Bool("p", false, "no error if existing, make parent directories as needed")
	parentsLong   = flag.Bool("parents", false, "equivalent to -p")
	verbose       = flag.Bool("v", false, "print a message for each created directory")
	verboseLong   = flag.Bool("verbose", false, "equivalent to -v")
	showVersion   = flag.Bool("version", false, "output version information and exit")
	context       = flag.String("Z", "", "set SELinux security context")
	contextLong   = flag.String("context", "", "set SELinux/SMACK security context")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	handleCombinedOptions()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", os.Args[0])
		usage()
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range flag.Args() {
		if err := createDir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %v\n", os.Args[0], dir, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "mkdir %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... DIRECTORY...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s -p dir/subdir\n  %s -m 755 newdir\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func handleCombinedOptions() {
	if *modeLong != "" {
		*mode = *modeLong
	}
	if *parentsLong {
		*parents = true
	}
	if *verboseLong {
		*verbose = true
	}
	if *contextLong != "" {
		*context = *contextLong
	}
}

func createDir(path string) error {
	var perm os.FileMode = 0777
	if *mode != "" {
		var err error
		perm, err = parseMode(*mode)
		if err != nil {
			return fmt.Errorf("invalid mode '%s': %v", *mode, err)
		}
	}

	if *parents {
		return createDirWithParents(path, perm)
	}

	err := os.Mkdir(path, perm)
	if err != nil {
		return err
	}

	if *verbose {
		fmt.Printf("mkdir: created directory '%s'\n", path)
	}

	return nil
}

func createDirWithParents(path string, perm os.FileMode) error {
	components := splitPath(path)

	currentPath := ""
	if path[0] == '/' {
		currentPath = "/"
	}

	for _, component := range components {
		if currentPath != "" && currentPath != "/" {
			currentPath += "/"
		}
		currentPath += component

		if _, err := os.Stat(currentPath); err == nil {
			continue
		}

		err := os.Mkdir(currentPath, perm)
		if err != nil {
			return err
		}

		if *verbose {
			fmt.Printf("mkdir: created directory '%s'\n", currentPath)
		}

		if *context != "" {
			// Placeholder... Eh
		}
	}

	return nil
}

func splitPath(path string) []string {
	var components []string
	current := ""

	for _, c := range path {
		if c == '/' {
			if current != "" {
				components = append(components, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		components = append(components, current)
	}

	return components
}

func parseMode(modeStr string) (os.FileMode, error) {
	var mode os.FileMode
	_, err := fmt.Sscanf(modeStr, "%o", &mode)
	if err != nil {
		return 0, fmt.Errorf("invalid octal number")
	}
	return mode, nil
}