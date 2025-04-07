package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type ChownMode int
type Verbosity int

const (
	version = "1.0.0"
	CHOWN ChownMode = iota
	CHGRP
	V_normal Verbosity = iota
	V_changes_only
	V_high
)

var (
	recursive           = flag.Bool("R", false, "operate on files and directories recursively")
	recursiveLong       = flag.Bool("recursive", false, "equivalent to -R")
	changes             = flag.Bool("c", false, "like verbose but report only when a change is made")
	changesLong         = flag.Bool("changes", false, "equivalent to -c")
	dereference        = flag.Bool("dereference", false, "affect the referent of each symbolic link (default)")
	noDereference      = flag.Bool("h", false, "affect symbolic links instead of any referenced file")
	noDereferenceLong  = flag.Bool("no-dereference", false, "equivalent to -h")
	from               = flag.String("from", "", "change the ownership only if current owner/group matches")
	preserveRoot       = flag.Bool("preserve-root", false, "fail to operate recursively on '/'")
	noPreserveRoot     = flag.Bool("no-preserve-root", false, "do not treat '/' specially (default)")
	quiet              = flag.Bool("f", false, "suppress most error messages")
	quietLong          = flag.Bool("quiet", false, "equivalent to -f")
	silent             = flag.Bool("silent", false, "equivalent to -f")
	verbose            = flag.Bool("v", false, "output a diagnostic for every file processed")
	verboseLong        = flag.Bool("verbose", false, "equivalent to -v")
	reference          = flag.String("reference", "", "use RFILE's ownership rather than explicit values")
	showVersion        = flag.Bool("version", false, "output version information and exit")
	chownMode          = CHOWN
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	handleCombinedOptions()

	exitCode := 0
	args := flag.Args()

	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	// Determine if chgrp
	if strings.HasSuffix(os.Args[0], "chgrp") || strings.HasSuffix(os.Args[0], "chgrp.exe") {
		chownMode = CHGRP
	}

	var uid, gid int = -1, -1
	var requiredUid, requiredGid int = -1, -1
	var userName, groupName string

	if *reference != "" {
		var err error
		uid, gid, err = getFileOwnership(*reference)
		if err != nil {
			log.Fatalf("failed to get attributes of %s: %v", *reference, err)
		}
		userName = getUserName(uid)
		groupName = getGroupName(gid)
	} else {
		ownerSpec := args[0]
		if chownMode == CHGRP {
			ownerSpec = ":" + ownerSpec
		}

		var err error
		uid, gid, requiredUid, requiredGid, userName, groupName, err = parseOwnerSpec(ownerSpec)
		if err != nil {
			log.Fatalf("invalid owner specification: %v", err)
		}

		args = args[1:]
	}

	if len(args) == 0 {
		log.Fatal("missing file operand")
	}

	verbosity := V_normal
	if *verbose {
		verbosity = V_high
	} else if *changes {
		verbosity = V_changes_only
	}

	options := ChownOptions{
		recurse:               *recursive,
		verbosity:             verbosity,
		forceSilent:           *quiet,
		affectSymlinkReferent: !*noDereference,
		userName:              userName,
		groupName:             groupName,
		preserveRoot:          *preserveRoot,
	}

	for _, file := range args {
		if err := chownFile(file, uid, gid, requiredUid, requiredGid, &options); err != nil {
			if !options.forceSilent {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", os.Args[0], file, err)
			}
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "chown/chgrp %s\n", version)
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]... OWNER[:GROUP] FILE...\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "   or: %s [OPTION]... --reference=RFILE FILE...\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n  %s root /u\n  %s root:staff /u\n  %s -R root /u\n", 
		os.Args[0], os.Args[0], os.Args[0])
}

func printVersion() {
	fmt.Printf("%s %s\n", os.Args[0], version)
}

func handleCombinedOptions() {
	if *recursiveLong {
		*recursive = true
	}
	if *changesLong {
		*changes = true
	}
	if *noDereferenceLong {
		*noDereference = true
	}
	if *quietLong || *silent {
		*quiet = true
	}
	if *verboseLong {
		*verbose = true
	}
}

type ChownOptions struct {
	recurse               bool
	verbosity             Verbosity
	forceSilent           bool
	affectSymlinkReferent bool
	userName              string
	groupName             string
	preserveRoot          bool
}

func getFileOwnership(filename string) (int, int, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return -1, -1, err
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, -1, fmt.Errorf("could not get file stats")
	}
	return int(stat.Uid), int(stat.Gid), nil
}

func parseOwnerSpec(spec string) (uid, gid, requiredUid, requiredGid int, userName, groupName string, err error) {
	if *from != "" {
		parts := strings.Split(*from, ":")
		if len(parts) > 0 && parts[0] != "" {
			var name string
			requiredUid, name, err = lookupUser(parts[0])
			if err != nil {
				return
			}
			_ = name // not needed for requiredUid
		}
		if len(parts) > 1 && parts[1] != "" {
			var name string
			requiredGid, name, err = lookupGroup(parts[1])
			if err != nil {
				return
			}
			_ = name // not needed for requiredGid
		}
	}

	parts := strings.Split(spec, ":")
	if len(parts) == 0 {
		return -1, -1, -1, -1, "", "", fmt.Errorf("invalid owner specification")
	}

	if parts[0] != "" {
		uid, userName, err = lookupUser(parts[0])
		if err != nil {
			return
		}
	}

	if len(parts) > 1 && parts[1] != "" {
		gid, groupName, err = lookupGroup(parts[1])
		if err != nil {
			return
		}
	}

	return
}

func lookupUser(userSpec string) (int, string, error) {
	if uid, err := strconv.Atoi(userSpec); err == nil {
		u, err := user.LookupId(userSpec)
		if err != nil {
			return uid, "", nil
		}
		return uid, u.Username, nil
	}

	u, err := user.Lookup(userSpec)
	if err != nil {
		return -1, "", fmt.Errorf("invalid user '%s'", userSpec)
	}
	uid, _ := strconv.Atoi(u.Uid)
	return uid, u.Username, nil
}

func lookupGroup(groupSpec string) (int, string, error) {
	if gid, err := strconv.Atoi(groupSpec); err == nil {
		g, err := user.LookupGroupId(groupSpec)
		if err != nil {
			return gid, "", nil
		}
		return gid, g.Name, nil
	}

	g, err := user.LookupGroup(groupSpec)
	if err != nil {
		return -1, "", fmt.Errorf("invalid group '%s'", groupSpec)
	}
	gid, _ := strconv.Atoi(g.Gid)
	return gid, g.Name, nil
}

func getUserName(uid int) string {
	if uid == -1 {
		return ""
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return strconv.Itoa(uid)
	}
	return u.Username
}

func getGroupName(gid int) string {
	if gid == -1 {
		return ""
	}
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return strconv.Itoa(gid)
	}
	return g.Name
}

func chownFile(path string, uid, gid, requiredUid, requiredGid int, options *ChownOptions) error {
	if options.recurse {
		return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if options.preserveRoot && filePath == "/" && path != "/" {
				return filepath.SkipDir
			}

			return changeOwnership(filePath, uid, gid, requiredUid, requiredGid, options)
		})
	}
	return changeOwnership(path, uid, gid, requiredUid, requiredGid, options)
}

func changeOwnership(path string, uid, gid, requiredUid, requiredGid int, options *ChownOptions) error {
	var fileInfo os.FileInfo
	var err error

	if options.affectSymlinkReferent {
		fileInfo, err = os.Stat(path)
	} else {
		fileInfo, err = os.Lstat(path)
	}

	if err != nil {
		return err
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("could not get file stats")
	}

	if requiredUid != -1 && int(stat.Uid) != requiredUid {
		return nil
	}
	if requiredGid != -1 && int(stat.Gid) != requiredGid {
		return nil
	}

	newUid := stat.Uid
	if uid != -1 {
		newUid = uint32(uid)
	}

	newGid := stat.Gid
	if gid != -1 {
		newGid = uint32(gid)
	}

	if newUid == stat.Uid && newGid == stat.Gid {
		return nil
	}

	if options.affectSymlinkReferent {
		err = os.Chown(path, int(newUid), int(newGid))
	} else {
		err = os.Lchown(path, int(newUid), int(newGid))
	}

	if err != nil {
		return err
	}

	if options.verbosity == V_high || (options.verbosity == V_changes_only && (newUid != stat.Uid || newGid != stat.Gid)) {
		displayName := path
		if !options.affectSymlinkReferent {
			displayName = fmt.Sprintf("%s (symlink)", path)
		}

		changedOwner := options.userName
		if changedOwner == "" {
			changedOwner = strconv.Itoa(uid)
		}

		changedGroup := options.groupName
		if changedGroup == "" {
			changedGroup = strconv.Itoa(gid)
		}

		fmt.Printf("changed ownership of '%s' from %s:%s to %s:%s\n",
			displayName,
			getUserName(int(stat.Uid)), getGroupName(int(stat.Gid)),
			changedOwner, changedGroup)
	}

	return nil
}