package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	version = "1.0.0"
)

var (
	archive           = flag.Bool("a", false, "same as -dR --preserve=all")
	attributesOnly    = flag.Bool("attributes-only", false, "don't copy the file data, just the attributes")
	backup            = flag.String("backup", "", "make a backup of each existing destination file")
	noBackup          = flag.Bool("b", false, "like --backup but does not accept an argument")
	copyContents      = flag.Bool("copy-contents", false, "copy contents of special files when recursive")
	debug             = flag.Bool("debug", false, "explain how a file is copied. Implies -v")
	force             = flag.Bool("f", false, "if an existing destination file cannot be opened, remove it and try again")
	interactive       = flag.Bool("i", false, "prompt before overwrite (overrides a previous -n option)")
	link              = flag.Bool("l", false, "hard link files instead of copying")
	dereference       = flag.Bool("L", false, "always follow symbolic links in SOURCE")
	noClobber         = flag.Bool("n", false, "silently skip existing files")
	noDereference     = flag.Bool("P", false, "never follow symbolic links in SOURCE")
	preserve          = flag.String("preserve", "", "preserve the specified attributes (mode,ownership,timestamps,links,context,xattr,all)")
	noPreserve        = flag.String("no-preserve", "", "don't preserve the specified attributes")
	parents           = flag.Bool("parents", false, "use full source file name under DIRECTORY")
	recursive         = flag.Bool("r", false, "copy directories recursively")
	recursiveLong     = flag.Bool("R", false, "equivalent to -r")
	reflink           = flag.String("reflink", "auto", "control clone/CoW copies (auto, always, never)")
	removeDestination = flag.Bool("remove-destination", false, "remove each existing destination file before attempting to open it")
	sparse            = flag.String("sparse", "auto", "control creation of sparse files (auto, always, never)")
	stripSlashes      = flag.Bool("strip-trailing-slashes", false, "remove any trailing slashes from each SOURCE argument")
	symbolicLink      = flag.Bool("s", false, "make symbolic links instead of copying")
	suffix            = flag.String("S", "", "override the usual backup suffix")
	targetDirectory   = flag.String("t", "", "copy all SOURCE arguments into DIRECTORY")
	noTargetDirectory = flag.Bool("T", false, "treat DEST as a normal file")
	update            = flag.String("update", "", "control which existing files are updated (all,none,none-fail,older)")
	verbose           = flag.Bool("v", false, "explain what is being done")
	keepDirSymlink    = flag.Bool("keep-directory-symlink", false, "follow existing symlinks to directories")
	oneFileSystem     = flag.Bool("x", false, "stay on this file system")
	versionFlag       = flag.Bool("version", false, "output version information and exit")
)

type cpOptions struct {
	archive              bool
	attributesOnly       bool
	backupType           string
	copyContents         bool
	debug                bool
	force                bool
	interactive          bool
	hardLink             bool
	dereference          bool
	noClobber            bool
	noDereference        bool
	preserveAttrs        []string
	noPreserveAttrs      []string
	parents              bool
	recursive            bool
	reflinkMode          string
	removeDestination    bool
	sparseMode           string
	stripTrailingSlashes bool
	symbolicLink         bool
	backupSuffix         string
	targetDirectory      string
	noTargetDirectory    bool
	updateMode           string
	verbose              bool
	keepDirectorySymlink bool
	oneFileSystem        bool
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *versionFlag {
		printVersion()
		os.Exit(0)
	}

	options := parseOptions()

	if *noTargetDirectory && *targetDirectory != "" {
		fmt.Fprintf(os.Stderr, "cannot combine --target-directory (-t) and --no-target-directory (-T)\n")
		os.Exit(1)
	}

	if *link && *symbolicLink {
		fmt.Fprintf(os.Stderr, "cannot make both hard and symbolic links\n")
		os.Exit(1)
	}

	sources, dest := getSourceDest(flag.Args(), options)

	if err := validateArgs(sources, dest, options); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if err := copyFiles(sources, dest, options); err != nil {
		fmt.Fprintf(os.Stderr, "cp: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Secure cp %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [-T] SOURCE DEST\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  or:  %s [OPTION]... SOURCE... DIRECTORY\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  or:  %s [OPTION]... -t DIRECTORY SOURCE...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nCopy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.")
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s file.txt copy.txt\n  %s -r dir1 dir2\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func parseOptions() cpOptions {
	recursive := *recursive || *recursiveLong

	if *archive {
		*preserve = "all"
		*noDereference = true
		recursive = true
	}

	backupType := ""
	if *noBackup {
		backupType = "simple"
	} else if *backup != "" {
		backupType = *backup
	}

	return cpOptions{
		archive:              *archive,
		attributesOnly:       *attributesOnly,
		backupType:           backupType,
		copyContents:         *copyContents,
		debug:                *debug,
		force:                *force,
		interactive:          *interactive,
		hardLink:             *link,
		dereference:          *dereference,
		noClobber:            *noClobber,
		noDereference:        *noDereference,
		preserveAttrs:        strings.Split(*preserve, ","),
		noPreserveAttrs:      strings.Split(*noPreserve, ","),
		parents:              *parents,
		recursive:            recursive,
		reflinkMode:          *reflink,
		removeDestination:    *removeDestination,
		sparseMode:           *sparse,
		stripTrailingSlashes: *stripSlashes,
		symbolicLink:         *symbolicLink,
		backupSuffix:         *suffix,
		targetDirectory:      *targetDirectory,
		noTargetDirectory:    *noTargetDirectory,
		updateMode:           *update,
		verbose:              *verbose || *debug,
		keepDirectorySymlink: *keepDirSymlink,
		oneFileSystem:        *oneFileSystem,
	}
}

func getSourceDest(args []string, options cpOptions) ([]string, string) {
	if options.noTargetDirectory {
		if len(args) != 2 {
			fmt.Fprintf(os.Stderr, "missing destination file operand after %s\n", args[0])
			os.Exit(1)
		}
		return []string{args[0]}, args[1]
	}

	if options.targetDirectory != "" {
		return args, options.targetDirectory
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "missing file operand")
		os.Exit(1)
	}

	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	return sources, dest
}

func validateArgs(sources []string, dest string, options cpOptions) error {
	if options.parents && !options.recursive {
		return fmt.Errorf("--parents requires --recursive")
	}

	if options.parents && !isDir(dest) {
		return fmt.Errorf("with --parents, the destination must be a directory")
	}

	if len(sources) > 1 && !isDir(dest) && !options.noTargetDirectory {
		return fmt.Errorf("target %s is not a directory", dest)
	}

	return nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func copyFiles(sources []string, dest string, options cpOptions) error {
	for _, src := range sources {
		if options.stripTrailingSlashes {
			src = strings.TrimRight(src, "/")
		}

		if err := copyFile(src, dest, options); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dest string, options cpOptions) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %v", src, err)
	}

	if options.parents {
		return copyWithParents(src, dest, srcInfo, options)
	}

	if isDir(dest) {
		base := filepath.Base(src)
		dest = filepath.Join(dest, base)
	}

	if srcInfo.IsDir() {
		if !options.recursive {
			return fmt.Errorf("-r not specified; omitting directory %q", src)
		}
		return copyDir(src, dest, options)
	}

	if options.symbolicLink {
		return os.Symlink(src, dest)
	}

	if options.hardLink {
		return os.Link(src, dest)
	}

	return copyRegularFile(src, dest, srcInfo, options)
}

func copyWithParents(src, dest string, srcInfo os.FileInfo, options cpOptions) error {
	relPath := strings.TrimPrefix(src, string(filepath.Separator))
	fullDest := filepath.Join(dest, relPath)

	if err := os.MkdirAll(filepath.Dir(fullDest), 0755); err != nil {
		return fmt.Errorf("cannot create parent directories for %q: %v", fullDest, err)
	}

	if srcInfo.IsDir() {
		return copyDir(src, fullDest, options)
	}

	return copyRegularFile(src, fullDest, srcInfo, options)
}

func copyDir(src, dest string, options cpOptions) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("cannot create directory %q: %v", dest, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("cannot read directory %q: %v", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		entryInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("cannot get info for %q: %v", srcPath, err)
		}

		if entryInfo.IsDir() {
			if err := copyDir(srcPath, destPath, options); err != nil {
				return err
			}
		} else {
			if err := copyRegularFile(srcPath, destPath, entryInfo, options); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyRegularFile(src, dest string, srcInfo os.FileInfo, options cpOptions) error {
	if options.verbose {
		fmt.Printf("%s -> %s\n", src, dest)
	}

	if options.noClobber && fileExists(dest) {
		return nil
	}

	if options.removeDestination && fileExists(dest) {
		if err := os.Remove(dest); err != nil {
			return fmt.Errorf("cannot remove destination %q: %v", dest, err)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open source file %q: %v", src, err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		if options.force {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("cannot remove destination %q: %v", dest, err)
			}
			destFile, err = os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
			if err != nil {
				return fmt.Errorf("cannot create destination file %q: %v", dest, err)
			}
		} else {
			return fmt.Errorf("cannot create destination file %q: %v", dest, err)
		}
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("error copying %q to %q: %v", src, dest, err)
	}

	if len(options.preserveAttrs) > 0 || *archive {
		if err := preserveAttributes(src, dest, srcInfo, options); err != nil {
			return fmt.Errorf("cannot preserve attributes for %q: %v", dest, err)
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func preserveAttributes(src, dest string, srcInfo os.FileInfo, options cpOptions) error {
	srcStat, ok := srcInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot get source file stats")
	}

	if contains(options.preserveAttrs, "timestamps") || contains(options.preserveAttrs, "all") {
		times := []syscall.Timespec{
			srcStat.Atim,
			srcStat.Mtim,
		}
		if err := syscall.UtimesNano(dest, times); err != nil {
			return fmt.Errorf("cannot preserve timestamps: %v", err)
		}
	}

	if contains(options.preserveAttrs, "ownership") || contains(options.preserveAttrs, "all") {
		if err := os.Chown(dest, int(srcStat.Uid), int(srcStat.Gid)); err != nil {
			return fmt.Errorf("cannot preserve ownership: %v", err)
		}
	}

	if contains(options.preserveAttrs, "mode") || contains(options.preserveAttrs, "all") {
		if err := os.Chmod(dest, srcInfo.Mode()); err != nil {
			return fmt.Errorf("cannot preserve mode: %v", err)
		}
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}