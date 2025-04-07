package main

import (
    "flag"
    "fmt"
    "os"
    "strings"
)

const (
	version = "1.0.0"
)

var (
    termType  = flag.String("T", "", "use this instead of $TERM")
    printVersion = flag.Bool("V", false, "print curses-version")
    noScrollback = flag.Bool("x", false, "do not try to clear scrollback")
)

func main() {
    flag.Usage = usage
    flag.Parse()

    if *printVersion {
        fmt.Println(version)
        os.Exit(0)
    }

    args := flag.Args()
    if len(args) > 0 {
        fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", os.Args[0], args[0])
        usage()
        os.Exit(1)
    }

    terminalType := *termType
    if terminalType == "" {
        terminalType = os.Getenv("TERM")
    }
    if terminalType == "" {
        terminalType = "xterm"
    }

    clearTerminal(terminalType, *noScrollback)
}

func usage() {
    fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
    fmt.Fprintln(os.Stderr, `
Clear the terminal screen.

Options:
  -T TERM     use this instead of $TERM
  -V          print curses-version
  -x          do not try to clear scrollback`)
}

func clearTerminal(term string, noScrollback bool) {
    switch strings.ToLower(term) {
    case "xterm", "xterm-256color", "linux", "vt100":
        fmt.Print("\033[H\033[2J")

        if !noScrollback {
            fmt.Print("\033[3J")
        }
    default:
        fmt.Fprintf(os.Stderr, "%s: unsupported terminal type '%s'\n", os.Args[0], term)
        os.Exit(1)
    }
}