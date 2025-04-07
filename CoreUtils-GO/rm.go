package main

import (
	"flag"
	"fmt"
	"os"
	"io"
	"path/filepath"
	"strings"
	"syscall"
)

type InteractiveMode int

const (
	version = "1.0.0"
	RMI_NEVER InteractiveMode = iota
	RMI_SOMETIMES
	RMI_ALWAYS
)

type RmOptions struct {
	ignoreMissingFiles      bool
	interactive            InteractiveMode
	oneFileSystem          bool
	removeEmptyDirectories bool
	recursive              bool
	preserveAllRoot        bool
	stdinTty               bool
	verbose                bool
}

var (
	force             = flag.Bool("f", false, "ignore nonexistent files and arguments, never prompt")
	forceLong         = flag.Bool("force", false, "equivalent to -f")
	interactive       = flag.Bool("i", false, "prompt before every removal")
	interactiveOnce   = flag.Bool("I", false, "prompt once before removing more than three files, or when removing recursively")
	interactiveOption = flag.String("interactive", "", "prompt according to WHEN: never, once (-I), or always (-i)")
	oneFileSystem     = flag.Bool("one-file-system", false, "when removing recursively, skip directories on different file systems")
	noPreserveRoot    = flag.Bool("no-preserve-root", false, "do not treat '/' specially")
	preserveRoot      = flag.String("preserve-root", "", "do not remove '/' (default); with 'all', reject arguments on separate devices")
	recursive         = flag.Bool("r", false, "remove directories and their contents recursively")
	recursiveLong     = flag.Bool("recursive", false, "equivalent to -r")
	recursiveAlt      = flag.Bool("R", false, "equivalent to -r")
	dir               = flag.Bool("d", false, "remove empty directories")
	dirLong           = flag.Bool("dir", false, "equivalent to -d")
	verbose           = flag.Bool("v", false, "explain what is being done")
	verboseLong       = flag.Bool("verbose", false, "equivalent to -v")
	showVersion       = flag.Bool("version", false, "output version information and exit")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	handleCombinedOptions()

	args := flag.Args()
	if len(args) == 0 {
		if *force {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "rm: missing operand\n")
		usage()
		os.Exit(1)
	}

	options := RmOptions{
		ignoreMissingFiles:      *force || *forceLong,
		interactive:            determineInteractiveMode(),
		oneFileSystem:          *oneFileSystem,
		removeEmptyDirectories: *dir || *dirLong,
		recursive:              *recursive || *recursiveLong || *recursiveAlt,
		preserveAllRoot:        *preserveRoot == "all",
		stdinTty:               isTerminal(os.Stdin.Fd()),
		verbose:                *verbose || *verboseLong,
	}

	if *noPreserveRoot {
		// Handle --no-preserve-root
		if !strings.HasPrefix(os.Args[len(os.Args)-len(args)-1], "--no-preserve-root") {
			fmt.Fprintf(os.Stderr, "rm: you may not abbreviate the --no-preserve-root option\n")
			os.Exit(1)
		}
	}

	if options.interactive == RMI_SOMETIMES && (options.recursive || len(args) > 3) {
		if !promptOnce(len(args), options.recursive) {
			os.Exit(0)
		}
	}

	exitCode := 0
	for _, file := range args {
		if err := removeFile(file, &options); err != nil {
			fmt.Fprintf(os.Stderr, "rm: %s: %v\n", file, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [FILE]...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Remove (unlink) the FILE(s).")
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
By default, rm does not remove directories. Use the --recursive (-r or -R)
option to remove each listed directory, too, along with all of its contents.

Any attempt to remove a file whose last file name component is '.' or '..'
is rejected with a diagnostic.

To remove a file whose name starts with a '-', for example '-foo',
use one of these commands:
  %s -- -foo
  %s ./-foo

If you use rm to remove a file, it might be possible to recover
some of its contents, given sufficient expertise and/or time. For greater
assurance that the contents are unrecoverable, consider using shred(1).
`, os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("rm (Go implementation) %s\n", version)
}

func handleCombinedOptions() {
	// Handled in RmOptions initialization :-)
}

func determineInteractiveMode() InteractiveMode {
	switch {
	case *interactive:
		return RMI_ALWAYS
	case *interactiveOnce:
		return RMI_SOMETIMES
	case *interactiveOption != "":
		switch strings.ToLower(*interactiveOption) {
		case "never", "no", "none":
			return RMI_NEVER
		case "once":
			return RMI_SOMETIMES
		case "always", "yes":
			return RMI_ALWAYS
		default:
			fmt.Fprintf(os.Stderr, "rm: invalid interactive mode: %s\n", *interactiveOption)
			os.Exit(1)
		}
	}
	return RMI_NEVER
}

func isTerminal(fd uintptr) bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func promptOnce(nFiles int, recursive bool) bool {
	question := fmt.Sprintf("rm: remove %d argument", nFiles)
	if nFiles > 1 {
		question += "s"
	}
	if recursive {
		question += " recursively"
	}
	question += "? "

	fmt.Fprintf(os.Stderr, question)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func removeFile(path string, options *RmOptions) error {
	base := filepath.Base(path)
	if base == "." || base == ".." {
		return fmt.Errorf("cannot remove '.' or '..'")
	}

	if strings.HasPrefix(path, "-") && path != "-" {
		return fmt.Errorf("invalid option -- '%s'\nTry 'rm -- %s' or 'rm ./%s'", 
			strings.TrimPrefix(path, "-"), path, path)
	}

	fileInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) && options.ignoreMissingFiles {
			return nil
		}
		return err
	}

	if path == "/" && !*noPreserveRoot {
		return fmt.Errorf("it is dangerous to operate recursively on '/'")
	}

	if options.interactive == RMI_ALWAYS {
		if !confirmRemoval(path) {
			return nil
		}
	}

	if fileInfo.IsDir() {
		return handleDirectory(path, fileInfo, options)
	}

	if options.verbose {
		fmt.Printf("removed '%s'\n", path)
	}
	return os.Remove(path)
}

func handleDirectory(path string, fileInfo os.FileInfo, options *RmOptions) error {
	empty, err := isDirEmpty(path)
	if err != nil {
		return err
	}

	if empty && (options.removeEmptyDirectories || options.recursive) {
		if options.verbose {
			fmt.Printf("removed directory '%s'\n", path)
		}
		return os.Remove(path)
	}

	if options.recursive {
		return removeDirectoryRecursive(path, options)
	}

	return fmt.Errorf("is a directory")
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}
	return len(names) == 0, nil
}

func removeDirectoryRecursive(path string, options *RmOptions) error {
	parentDev, err := getDevice(path)
	if err != nil {
		return err
	}

	return filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			if options.ignoreMissingFiles && os.IsPermission(err) {
				return nil
			}
			return err
		}

		if currentPath == "/" && !*noPreserveRoot {
			return filepath.SkipDir
		}

		if options.oneFileSystem {
			currentDev, err := getDevice(currentPath)
			if err != nil {
				return err
			}
			if currentDev != parentDev {
				return filepath.SkipDir
			}
		}

		if options.interactive == RMI_ALWAYS {
			if !confirmRemoval(currentPath) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if currentPath == path {
			return nil
		}

		if options.verbose {
			fmt.Printf("removed '%s'\n", currentPath)
		}

		if info.IsDir() {
			return os.Remove(currentPath)
		}
		return os.Remove(currentPath)
	})
}

func getDevice(path string) (uint64, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0, err
	}
	return stat.Dev, nil
}

func confirmRemoval(path string) bool {
	fmt.Printf("rm: remove '%s'? ", path)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}