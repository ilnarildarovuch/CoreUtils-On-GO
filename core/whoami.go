package main

import (
    "flag"
    "fmt"
    "os"
    "os/user"
    "syscall"
)

const (
    version = "1.0.0"
)
var (
    showVersion = flag.Bool("version", false, "output version information and exit")
)

func main() {
    flag.Usage = usage
    flag.Parse()

    if *showVersion {
        printVersion()
        os.Exit(0)
    }

    args := flag.Args()
    if len(args) > 0 {
        fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", os.Args[0], args[0])
        usage()
        os.Exit(1)
    }

    exitCode := run()
    os.Exit(exitCode)
}

func usage() {
    fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
    fmt.Fprintln(os.Stderr, `
Print the user name associated with the current effective user ID.
Same as id -un.

Options:`)
    flag.PrintDefaults()
    fmt.Fprintln(os.Stderr, `
Examples:
  whoami
  whoami --version`)
}

func printVersion() {
    fmt.Printf("%s %s\n", os.Args[0], version)
}

func run() int {
    uid := syscall.Geteuid()

    u, err := user.LookupId(fmt.Sprintf("%d", uid))
    if err != nil {
        fmt.Fprintf(os.Stderr, "%s: cannot find name for user ID %d: %v\n", os.Args[0], uid, err)
        return 1
    }

    fmt.Println(u.Username)
    return 0
}