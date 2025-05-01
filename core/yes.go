package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	version     = "1.0.0"
)

var (
	showHelp    = flag.Bool("help", false, "display this help and exit")
	showVersion = flag.Bool("version", false, "output version information and exit")
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

	args := flag.Args()
	output := "y"
	if len(args) > 0 {
		output = strings.Join(args, " ")
	}

	buf := []byte(output + "\n")

	for {
		_, err := os.Stdout.Write(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", os.Args[0], err)
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [STRING]...\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  or:  %s OPTION\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nRepeatedly output a line with all specified STRING(s), or 'y'.")
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s\n  %s hello world\n", os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}