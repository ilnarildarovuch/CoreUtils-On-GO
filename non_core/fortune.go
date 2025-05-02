package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	version      = "1.0.1"
	maxLineLen   = 10 * 1024 * 1024
	fortuneDir   = "/usr/share/games/fortunes"
	offensiveDir = "/usr/share/games/fortunes/off"
	headerSize   = 24
)

var (
	offensive      = flag.Bool("o", false, "offensive fortunes only")
	allForts       = flag.Bool("a", false, "any fortune allowed")
	shortOnly      = flag.Bool("s", false, "short fortunes only")
	longOnly       = flag.Bool("l", false, "long fortunes only")
	showVersion    = flag.Bool("version", false, "output version information")
	equalProb      = flag.Bool("e", false, "equal probability distribution")
	findFiles      = flag.Bool("f", false, "find fortune files")
	wait           = flag.Bool("w", false, "wait after display")
	showFilename   = flag.Bool("c", false, "show filename")
	locale         = os.Getenv("LANG")
)

type fortuneFile struct {
	path    string
	datFile string
	count   uint32
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	rand.Seed(time.Now().UnixNano())

	files, err := collectFortuneFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting files: %v\n", err)
		os.Exit(1)
	}

	if *findFiles {
		for _, f := range files {
			fmt.Println(f.path)
		}
		return
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No fortune files found")
		os.Exit(1)
	}

	selected := selectFortune(files)
	if selected == nil {
		fmt.Fprintln(os.Stderr, "No matching fortune found")
		os.Exit(1)
	}

	fortune, err := readFortune(selected)
	for err != nil {
		selected = selectFortune(files)
		if selected == nil {
			break
		}
		fortune, err = readFortune(selected)
	}

	if *showFilename {
		fmt.Printf("(%s)\n%%\n", filepath.Base(selected.path))
	}
	fmt.Println(fortune)

	if *wait {
		time.Sleep(time.Duration(len(fortune)/20) * time.Second)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "fortune %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... [FILE]...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func collectFortuneFiles() ([]fortuneFile, error) {
	var files []fortuneFile
	dirs := getSearchDirs()

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || filepath.Ext(path) == ".dat" {
				return nil
			}

			datFile := path + ".dat"
			if _, err := os.Stat(datFile); os.IsNotExist(err) {
				return nil
			}

			count, err := readDatFile(datFile)
			if err != nil {
				return nil
			}

			files = append(files, fortuneFile{
				path:    path,
				datFile: datFile,
				count:   count,
			})
			return nil
		})

		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func getSearchDirs() []string {
	if *allForts {
		return []string{fortuneDir, offensiveDir}
	}
	if *offensive {
		return []string{offensiveDir}
	}
	if locale != "" {
		return []string{filepath.Join(fortuneDir, locale), fortuneDir}
	}
	return []string{fortuneDir}
}

func readDatFile(path string) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Read entire 24-byte header
	var header struct {
		Version   uint32
		NumStr    uint32
		LongLen   uint32
		ShortLen  uint32
		Flags     uint32
		Stuff     [4]byte
	}
	
	err = binary.Read(file, binary.BigEndian, &header)
	if err != nil {
		return 0, fmt.Errorf("invalid header: %v", err)
	}
	
	return header.NumStr, nil
}

func selectFortune(files []fortuneFile) *fortuneFile {
	total := uint32(0)
	for _, f := range files {
		total += f.count
	}
	if total == 0 {
		return nil
	}

	target := rand.Uint32() % total
	current := uint32(0)
	for _, f := range files {
		current += f.count
		if current > target {
			return &f
		}
	}
	return nil
}

func readFortune(f *fortuneFile) (string, error) {
	dat, err := os.Open(f.datFile)
	if err != nil {
		return "", err
	}
	defer dat.Close()

	offset := int64(4 + (rand.Intn(int(f.count)) * 8))
	dat.Seek(offset, 0)

	var pos [2]int32
	if err := binary.Read(dat, binary.BigEndian, &pos); err != nil {
		return "", err
	}

	file, err := os.Open(f.path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	file.Seek(int64(pos[0]), 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, maxLineLen), maxLineLen)

	var buf bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "%" {
			break
		}
		buf.WriteString(line + "\n")
	}

	return strings.TrimSpace(buf.String()), nil
}
