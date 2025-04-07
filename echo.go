package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	version    = "1.0.0"
)

var (
	noNewline      = flag.Bool("n", false, "do not output the trailing newline")
	enableEscape   = flag.Bool("e", false, "enable interpretation of backslash escapes")
	disableEscape  = flag.Bool("E", false, "disable interpretation of backslash escapes")
	showHelp       = flag.Bool("help", false, "display this help and exit")
	showVersion    = flag.Bool("version", false, "output version information and exit")
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
	processEcho(args)
}

func usage() {
	fmt.Printf(`Usage: %s [SHORT-OPTION]... [STRING]...
  or:  %s LONG-OPTION
Echo the STRING(s) to standard output.

Options:
`, os.Args[0], os.Args[0])
	flag.PrintDefaults()
	fmt.Printf(`
If -e is in effect, the following sequences are recognized:

  \\\\      backslash
  \\a      alert (BEL)
  \\b      backspace
  \\c      produce no further output
  \\e      escape
  \\f      form feed
  \\n      new line
  \\r      carriage return
  \\t      horizontal tab
  \\v      vertical tab
  \\0NNN   byte with octal value NNN (1 to 3 digits)
  \\xHH    byte with hexadecimal value HH (1 to 2 digits)

Consider using the printf(1) command instead,
as it avoids problems when outputting option-like strings.
`)
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func processEcho(args []string) {
	processEscapes := *enableEscape || (!*disableEscape && os.Getenv("POSIXLY_CORRECT") == "")

	var output strings.Builder
	first := true

	for _, arg := range args {
		if !first {
			output.WriteString(" ")
		}
		first = false

		if processEscapes {
			output.WriteString(processEscapeSequences(arg))
		} else {
			output.WriteString(arg)
		}
	}

	if !*noNewline {
		output.WriteString("\n")
	}

	fmt.Print(output.String())
}

func processEscapeSequences(s string) string {
	var result strings.Builder
	i := 0
	n := len(s)

	for i < n {
		if s[i] == '\\' && i+1 < n {
			switch s[i+1] {
			case 'a':
				result.WriteByte('\a')
				i += 2
			case 'b':
				result.WriteByte('\b')
				i += 2
			case 'c':
				// stop further output
				return result.String()
			case 'e':
				result.WriteByte('\x1B')
				i += 2
			case 'f':
				result.WriteByte('\f')
				i += 2
			case 'n':
				result.WriteByte('\n')
				i += 2
			case 'r':
				result.WriteByte('\r')
				i += 2
			case 't':
				result.WriteByte('\t')
				i += 2
			case 'v':
				result.WriteByte('\v')
				i += 2
			case 'x':
				if i+2 < n && isHexDigit(s[i+2]) {
					val := hexToByte(s[i+2])
					i += 3
					if i < n && isHexDigit(s[i]) {
						val = val*16 + hexToByte(s[i])
						i++
					}
					result.WriteByte(val)
				} else {
					// output as-is
					result.WriteString("\\x")
					i += 2
				}
			case '0':
				if i+2 < n && isOctalDigit(s[i+2]) {
					val := s[i+2] - '0'
					i += 3
					if i < n && isOctalDigit(s[i]) {
						val = val*8 + (s[i] - '0')
						i++
						if i < n && isOctalDigit(s[i]) {
							val = val*8 + (s[i] - '0')
							i++
						}
					}
					result.WriteByte(val)
				} else {
					// output as-is
					result.WriteByte('\\')
					i++
				}
			case '\\':
				result.WriteByte('\\')
				i += 2
			default:
				// output as-is
				result.WriteByte('\\')
				result.WriteByte(s[i+1])
				i += 2
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}

	return result.String()
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isOctalDigit(c byte) bool {
	return c >= '0' && c <= '7'
}

func hexToByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}