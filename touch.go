package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	version = "1.0.0"
	ATIME   = 1 << iota
	MTIME
)

var (
	accessTime        = flag.Bool("a", false, "change only the access time")
	noCreate          = flag.Bool("c", false, "do not create any files")
	noCreateLong      = flag.Bool("no-create", false, "equivalent to -c")
	date              = flag.String("d", "", "parse STRING and use it instead of current time")
	dateLong          = flag.String("date", "", "equivalent to -d")
	noDereference     = flag.Bool("h", false, "affect each symbolic link instead of any referenced file")
	noDereferenceLong = flag.Bool("no-dereference", false, "equivalent to -h")
	modificationTime  = flag.Bool("m", false, "change only the modification time")
	reference         = flag.String("r", "", "use this file's times instead of current time")
	referenceLong     = flag.String("reference", "", "equivalent to -r")
	timestamp         = flag.String("t", "", "use specified time instead of current time (format: [[CC]YY]MMDDhhmm[.ss])")
	timeOption        = flag.String("time", "", "specify which time to change: atime/access/use or mtime/modify")
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
		usage()
		os.Exit(1)
	}

	// Determine which timestamps to change
	changeTimes := 0
	if *accessTime {
		changeTimes |= ATIME
	}
	if *modificationTime {
		changeTimes |= MTIME
	}
	if changeTimes == 0 {
		changeTimes = ATIME | MTIME
	}

	// Handle --time option
	if *timeOption != "" {
		switch strings.ToLower(*timeOption) {
		case "atime", "access", "use":
			changeTimes = ATIME
		case "mtime", "modify":
			changeTimes = MTIME
		default:
			fmt.Fprintf(os.Stderr, "invalid time specification: %s\n", *timeOption)
			os.Exit(1)
		}
	}

	// Get the new times to set
	var newTimes [2]time.Time
	var useCurrentTime bool

	if *reference != "" {
		// Use times from reference file
		refFileInfo, err := getFileInfo(*reference, *noDereference)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get attributes of %s: %v\n", *reference, err)
			os.Exit(1)
		}
		newTimes[0] = getAtime(refFileInfo)
		newTimes[1] = getMtime(refFileInfo)
	} else if *date != "" {
		// Parse custom date string
		t, err := parseDateTime(*date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid date format %s: %v\n", *date, err)
			os.Exit(1)
		}
		newTimes[0] = t
		newTimes[1] = t
	} else if *timestamp != "" {
		// Parse timestamp format
		t, err := parseTimestamp(*timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid timestamp format %s: %v\n", *timestamp, err)
			os.Exit(1)
		}
		newTimes[0] = t
		newTimes[1] = t
	} else {
		useCurrentTime = true
	}

	exitCode := 0
	for _, file := range args {
		if err := touchFile(file, changeTimes, newTimes, useCurrentTime); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], file, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "touch %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... FILE...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nUpdate the access and modification times of each FILE to the current time.")
	fmt.Fprintln(os.Stderr, "A FILE argument that does not exist is created empty, unless -c or -h is supplied.")
	fmt.Fprintln(os.Stderr, "A FILE argument string of - is handled specially and causes touch to change the times")
	fmt.Fprintln(os.Stderr, "of the file associated with standard output.\n")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s file.txt\n  %s -a -m -t 202312251200 file.txt\n",
		os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func handleCombinedOptions() {
	if *noCreateLong {
		*noCreate = true
	}
	if *noDereferenceLong {
		*noDereference = true
	}
	if *dateLong != "" {
		*date = *dateLong
	}
	if *referenceLong != "" {
		*reference = *referenceLong
	}
}

func touchFile(file string, changeTimes int, newTimes [2]time.Time, useCurrentTime bool) error {
	if file == "-" {
		return touchStdout()
	}

	fileInfo, err := getFileInfo(file, *noDereference)
	fileExists := err == nil

	if !fileExists {
		if *noCreate {
			return nil
		}
		f, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("cannot create file: %w", err)
		}
		f.Close()
		fileInfo, err = getFileInfo(file, *noDereference)
		if err != nil {
			return fmt.Errorf("cannot stat newly created file: %w", err)
		}
	}

	var atime, mtime time.Time
	if useCurrentTime {
		now := time.Now()
		if changeTimes&ATIME != 0 {
			atime = now
		} else {
			atime = getAtime(fileInfo)
		}
		if changeTimes&MTIME != 0 {
			mtime = now
		} else {
			mtime = getMtime(fileInfo)
		}
	} else {
		if changeTimes&ATIME != 0 {
			atime = newTimes[0]
		} else {
			atime = getAtime(fileInfo)
		}
		if changeTimes&MTIME != 0 {
			mtime = newTimes[1]
		} else {
			mtime = getMtime(fileInfo)
		}
	}

	if *noDereference {
	}
	err = os.Chtimes(file, atime, mtime)
	if err != nil {
		return fmt.Errorf("setting times: %w", err)
	}

	return nil
}

func touchStdout() error {
	now := time.Now()
	return os.Chtimes("/proc/self/fd/1", now, now)
}

func getFileInfo(path string, noDereference bool) (os.FileInfo, error) {
	if noDereference {
		return os.Lstat(path)
	}
	return os.Stat(path)
}

func getAtime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}

func getMtime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}

func parseDateTime(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"15:04:05",
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
	}

	if strings.ToLower(dateStr) == "now" {
		return time.Now(), nil
	}

	return time.Time{}, fmt.Errorf("unrecognized date format")
}

func parseTimestamp(timestampStr string) (time.Time, error) {
	// Format: [[CC]YY]MMDDhhmm[.ss]
	// Where CC is the first two digits of the year (century-1)
	// YY is the last two digits of the year
	// MM is the month (01-12)
	// DD is the day (01-31)
	// hh is the hour (00-23)
	// mm is the minute (00-59)
	// ss is the second (00-60)

	// Remove any dots from the timestamp
	cleanStr := strings.ReplaceAll(timestampStr, ".", "")

	switch len(cleanStr) {
	case 8: // MMDDhhmm
		return time.Parse("01021504", cleanStr)
	case 10: // YYMMDDhhmm
		return time.Parse("0601021504", cleanStr)
	case 12: // CCYYMMDDhhmm
		return time.Parse("200601021504", cleanStr)
	case 14: // MMDDhhmm.ss (12+2)
		return time.Parse("01021504.00", timestampStr)
	default:
		return time.Time{}, fmt.Errorf("invalid timestamp format")
	}
}