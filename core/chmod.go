package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	version     = "1.0.0"
)

var (
	changesOnly       = flag.Bool("c", false, "like verbose but report only when a change is made")
	changesOnlyLong   = flag.Bool("changes", false, "equivalent to -c")
	forceSilent       = flag.Bool("f", false, "suppress most error messages")
	forceSilentLong   = flag.Bool("silent", false, "equivalent to -f")
	forceSilentAlt    = flag.Bool("quiet", false, "equivalent to -f")
	verbose           = flag.Bool("v", false, "output a diagnostic for every file processed")
	verboseLong       = flag.Bool("verbose", false, "equivalent to -v")
	dereference       = flag.Bool("dereference", false, "affect the referent of each symbolic link")
	noDereference     = flag.Bool("h", false, "affect each symbolic link, rather than the referent")
	noDereferenceLong = flag.Bool("no-dereference", false, "equivalent to -h")
	preserveRoot      = flag.Bool("preserve-root", false, "fail to operate recursively on '/'")
	noPreserveRoot    = flag.Bool("no-preserve-root", false, "do not treat '/' specially (default)")
	reference         = flag.String("reference", "", "use RFILE's mode instead of MODE values")
	recursive         = flag.Bool("R", false, "change files and directories recursively")
	recursiveLong     = flag.Bool("recursive", false, "equivalent to -R")
	showHelp          = flag.Bool("help", false, "display this help and exit")
	showVersion       = flag.Bool("version", false, "output version information and exit")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if *showHelp {
		usage()
		os.Exit(0)
	}

	handleCombinedOptions()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	mode := args[0]
	files := args[1:]

	if *reference != "" {
		refMode, err := getFileMode(*reference)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], *reference, err)
			os.Exit(1)
		}
		mode = fmt.Sprintf("%#o", refMode.Perm())
	}

	modeBits, err := parseMode(mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid mode: %s\n", os.Args[0], mode)
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := changeMode(file, modeBits); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], file, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... MODE[,MODE]... FILE...\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  or:  %s [OPTION]... OCTAL-MODE FILE...\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  or:  %s [OPTION]... --reference=RFILE FILE...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nChange the mode of each FILE to MODE.")
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s 644 file.txt\n  %s u=rw,go=r file.txt\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func handleCombinedOptions() {
	if *changesOnlyLong {
		*changesOnly = true
	}
	if *forceSilentLong || *forceSilentAlt {
		*forceSilent = true
	}
	if *verboseLong {
		*verbose = true
	}
	if *noDereferenceLong {
		*noDereference = true
	}
	if *recursiveLong {
		*recursive = true
	}
}

func getFileMode(path string) (os.FileMode, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fileInfo.Mode(), nil
}

func parseMode(modeStr string) (os.FileMode, error) {
	if modeStr[0] >= '0' && modeStr[0] <= '7' {
		mode, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return 0, err
		}
		return os.FileMode(mode), nil
	}

	var mode os.FileMode
	lowerModeStr := strings.ToLower(modeStr)
	parts := strings.Split(lowerModeStr, ",")
	for _, part := range parts {
		switch {
		case strings.Contains(part, "u="):
			mode |= parseWhoPerm(part, "u=", 0600)
		case strings.Contains(part, "g="):
			mode |= parseWhoPerm(part, "g=", 0060)
		case strings.Contains(part, "o="):
			mode |= parseWhoPerm(part, "o=", 0007)
		case strings.Contains(part, "a="):
			mode |= parseWhoPerm(part, "a=", 0666)
		case part == "+x":
			mode |= 0111
		case part == "-x":
			mode &^= 0111
		default:
			return 0, fmt.Errorf("invalid mode specification: %s", part)
		}
	}
	return mode, nil
}

func parseWhoPerm(part, prefix string, mask uint32) os.FileMode {
	perms := strings.TrimPrefix(part, prefix)
	var mode os.FileMode
	for _, c := range perms {
		switch c {
		case 'r':
			mode |= os.FileMode(mask & 0444)
		case 'w':
			mode |= os.FileMode(mask & 0222)
		case 'x':
			mode |= os.FileMode(mask & 0111)
		}
	}
	return mode
}

func changeMode(path string, mode os.FileMode) error {
	if *recursive {
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				if !*forceSilent {
					return err
				}
				return nil
			}

			if *preserveRoot && p == "/" {
				return fmt.Errorf("it is dangerous to operate recursively on '/'")
			}

			return applyMode(p, info, mode)
		})
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	return applyMode(path, info, mode)
}

func applyMode(path string, info os.FileInfo, mode os.FileMode) error {
	var targetMode os.FileMode

	if info.Mode()&os.ModeSymlink != 0 {
		if *noDereference {
			if *verbose {
				fmt.Printf("mode of %s retained as symbolic link\n", path)
			}
			return nil
		}
		// Dereference symlink
		_, err := os.Stat(path)
		if err != nil {
			if !*forceSilent {
				return err
			}
			return nil
		}
		targetMode = mode
		if info.IsDir() {
			targetMode |= os.ModeDir
		}
	} else {
		targetMode = mode
		// Preserve file type bits
		targetMode |= info.Mode() &^ os.ModePerm
	}

	if info.Mode().Perm() == targetMode.Perm() {
		if *verbose {
			fmt.Printf("mode of %s retained as %04o\n", path, info.Mode().Perm())
		}
		return nil
	}

	err := os.Chmod(path, targetMode)
	if err != nil {
		return err
	}

	if *verbose || *changesOnly {
		newInfo, err := os.Lstat(path)
		if err != nil {
			if !*forceSilent {
				return err
			}
			return nil
		}

		if newInfo.Mode().Perm() != info.Mode().Perm() {
			fmt.Printf("mode of %s changed from %04o to %04o\n", 
				path, info.Mode().Perm(), newInfo.Mode().Perm())
		} else if *verbose {
			fmt.Printf("mode of %s retained as %04o\n", path, info.Mode().Perm())
		}
	}

	return nil
}