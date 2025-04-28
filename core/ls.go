package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

const (
	version = "1.0.0"
)

var (
	all           = flag.Bool("a", false, "do not ignore entries starting with .")
	almostAll     = flag.Bool("A", false, "do not list implied . and ..")
	longFormat    = flag.Bool("l", false, "use a long listing format")
	humanReadable = flag.Bool("h", false, "with -l, print human readable sizes")
	recursive     = flag.Bool("R", false, "list subdirectories recursively")
	reverse       = flag.Bool("r", false, "reverse order while sorting")
	sizeSort      = flag.Bool("S", false, "sort by file size")
	timeSort      = flag.Bool("t", false, "sort by modification time")
	showVersion   = flag.Bool("version", false, "output version information and exit")
	colorOutput   = flag.String("color", "auto", "colorize the output (always, auto, never)")
	onePerLine    = flag.Bool("1", false, "list one file per line")
	showInode     = flag.Bool("i", false, "print the index number of each file")
	ignoreBackups = flag.Bool("B", false, "do not list implied entries ending with ~")
	showHelp      = flag.Bool("help", false, "display this help and exit")
)

type FileInfo struct {
	fs.FileInfo
	Path       string
	LinkTarget string
}

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

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	useColor := shouldUseColor()

	for _, path := range paths {
		if err := listPath(path, useColor); err != nil {
			fmt.Fprintf(os.Stderr, "ls: %s: %v\n", path, err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "ls %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [FILE]...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s -l\n  %s -a /tmp\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("ls %s\n", version)
}

func shouldUseColor() bool {
	switch *colorOutput {
	case "always":
		return true
	case "never":
		return false
	case "auto":
		return isTerminal(os.Stdout)
	default:
		return isTerminal(os.Stdout)
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func listPath(path string, useColor bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return listDirectory(path, useColor)
	}

	return listFile(path, info, useColor)
}

func listDirectory(path string, useColor bool) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if shouldSkipEntry(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		var linkTarget string
		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget, _ = os.Readlink(fullPath)
		}

		files = append(files, FileInfo{
			FileInfo:   info,
			Path:       entry.Name(),
			LinkTarget: linkTarget,
		})
	}

	sortFiles(files)

	if *longFormat {
		printLongFormat(files, path, useColor)
	} else if *onePerLine {
		printOnePerLine(files, useColor)
	} else {
		printMultiColumn(files, useColor)
	}

	if *recursive {
		for _, file := range files {
			if file.IsDir() && !isDotOrDotDot(file.Name()) {
				fmt.Printf("\n%s:\n", filepath.Join(path, file.Name()))
				listDirectory(filepath.Join(path, file.Name()), useColor)
			}
		}
	}

	return nil
}

func shouldSkipEntry(name string) bool {
	if *all {
		return false
	}
	if *almostAll {
		return name == "." || name == ".."
	}
	if *ignoreBackups && strings.HasSuffix(name, "~") {
		return true
	}
	return name[0] == '.'
}

func isDotOrDotDot(name string) bool {
	return name == "." || name == ".."
}

func sortFiles(files []FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		if *sizeSort {
			if files[i].Size() != files[j].Size() {
				if *reverse {
					return files[i].Size() > files[j].Size()
				}
				return files[i].Size() < files[j].Size()
			}
		} else if *timeSort {
			iTime := files[i].ModTime()
			jTime := files[j].ModTime()
			if !iTime.Equal(jTime) {
				if *reverse {
					return iTime.After(jTime)
				}
				return iTime.Before(jTime)
			}
		}

		if *reverse {
			return files[i].Name() > files[j].Name()
		}
		return files[i].Name() < files[j].Name()
	})
}

func printLongFormat(files []FileInfo, dirPath string, useColor bool) {
	var totalBlocks int64
	for _, file := range files {
		totalBlocks += blocks(file)
	}
	fmt.Printf("total %d\n", totalBlocks)

	for _, file := range files {
		printFileLong(file, dirPath, useColor)
	}
}

func blocks(file fs.FileInfo) int64 {
	return (file.Size() + 511) / 512
}

func printFileLong(file FileInfo, dirPath string, useColor bool) {
	mode := file.Mode()
	modTime := file.ModTime().Format("Jan _2 15:04")

	if *humanReadable {
		fmt.Print(humanSize(file.Size()), " ")
	} else {
		fmt.Print(file.Size(), " ")
	}

	if *showInode {
		if stat, ok := file.Sys().(*syscall.Stat_t); ok {
			fmt.Printf("%d ", stat.Ino)
		} else {
			fmt.Print("? ")
		}
	}

	fmt.Printf("%s %s ", mode.String(), modTime)

	if useColor {
		printWithColor(file.Name(), file.Mode())
	} else {
		fmt.Print(file.Name())
	}

	if mode&os.ModeSymlink != 0 && file.LinkTarget != "" {
		fmt.Printf(" -> %s", file.LinkTarget)
	}

	fmt.Println()
}

func humanSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(size)/float64(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(size)/float64(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(size)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func printOnePerLine(files []FileInfo, useColor bool) {
	for _, file := range files {
		if useColor {
			printWithColor(file.Name(), file.Mode())
			fmt.Println()
		} else {
			fmt.Println(file.Name())
		}
	}
}

func printMultiColumn(files []FileInfo, useColor bool) {
	width := getTerminalWidth()
	if width <= 0 {
		width = 80
	}

	var maxLen int
	for _, file := range files {
		nameLen := utf8.RuneCountInString(file.Name())
		if nameLen > maxLen {
			maxLen = nameLen
		}
	}

	cols := width / (maxLen + 2)
	if cols < 1 {
		cols = 1
	}

	rows := (len(files) + cols - 1) / cols

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			idx := i + j*rows
			if idx >= len(files) {
				continue
			}
			name := files[idx].Name()
			if useColor {
				printWithColor(name, files[idx].Mode())
				fmt.Print(strings.Repeat(" ", maxLen+2-utf8.RuneCountInString(name)))
			} else {
				fmt.Printf("%-*s", maxLen+2, name)
			}
		}
		fmt.Println()
	}
}

func getTerminalWidth() int {
	if colsStr := os.Getenv("COLUMNS"); colsStr != "" {
		if cols, err := strconv.Atoi(colsStr); err == nil && cols > 0 {
			return cols
		}
	}

	return 80
}

func listFile(path string, info fs.FileInfo, useColor bool) error {
	if *longFormat {
		printFileLong(FileInfo{
			FileInfo: info,
			Path:     filepath.Base(path),
		}, filepath.Dir(path), useColor)
	} else {
		if useColor {
			printWithColor(filepath.Base(path), info.Mode())
			fmt.Println()
		} else {
			fmt.Println(filepath.Base(path))
		}
	}
	return nil
}

func printWithColor(name string, mode os.FileMode) {
	// ANSI color codes
	const (
		Reset      = "\033[0m"
		Bold       = "\033[1m"
		Red        = "\033[31m"
		Green      = "\033[32m"
		Yellow     = "\033[33m"
		Blue       = "\033[34m"
		Magenta    = "\033[35m"
		Cyan       = "\033[36m"
		White      = "\033[37m"
		BoldBlue   = "\033[1;34m"
		BoldCyan   = "\033[1;36m"
		BoldGreen  = "\033[1;32m"
		BoldYellow = "\033[1;33m"
	)

	switch {
	case mode.IsDir():
		fmt.Print(BoldBlue + name + Reset)
	case mode&os.ModeSymlink != 0:
		fmt.Print(BoldCyan + name + Reset)
	case mode&0111 != 0: // Executable
		fmt.Print(BoldGreen + name + Reset)
	default:
		fmt.Print(name)
	}
}