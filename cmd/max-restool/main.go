// main.go : is the max-restool entry point for listing, unpacking, and packing M.A.X. RES archives.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/restool"
)

// Exit codes, the same in every max-go tool: 0 for success (help included), 1 for a runtime failure
// (an archive or folder that cannot be read or written), 2 for a bad command line.
const (
	exitRuntime     = 1
	exitCommandLine = 2
)

// archiveFlags are the aliases that name the archive in the short form.
var archiveFlags = []string{"res", "file", "archive"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 0
	}

	switch args[0] {
	case "help", "-h", "-help", "--help":
		// Print command help.
		usage(stderr)
		return 0
	}

	// A leading dash means the short form: name an archive and it unpacks.
	if strings.HasPrefix(args[0], "-") {
		return unpackByFlag(args, stdout, stderr)
	}

	switch args[0] {
	case "list":
		// List the files in a RES archive.
		if err := checkPaths("list", args[1:], "<archive.res>"); err != nil {
			return badCommandLine(stderr, err)
		}

		if err := list(stdout, args[1]); err != nil {
			return runtimeFailure(stderr, err)
		}

	case "unpack":
		// Unpack a RES archive into loose raw files.
		if err := checkPaths("unpack", args[1:], "<archive.res>", "<dir>"); err != nil {
			return badCommandLine(stderr, err)
		}

		if err := restool.Unpack(args[1], args[2]); err != nil {
			return runtimeFailure(stderr, err)
		}

	case "pack":
		// Pack loose raw files into a RES archive.
		if err := checkPaths("pack", args[1:], "<dir>", "<archive.res>"); err != nil {
			return badCommandLine(stderr, err)
		}

		dst := args[2]
		if dir := filepath.Dir(dst); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return runtimeFailure(stderr, err)
			}
		}

		if err := restool.Pack(args[1], dst); err != nil {
			return runtimeFailure(stderr, err)
		}

	default:
		return badCommandLine(stderr, fmt.Errorf("unknown command %q, want list, unpack, pack, help or a -res, -file or -archive flag", args[0]))
	}

	return 0
}

// badCommandLine prints the reason in the max-restool form with a hint and returns the command line exit code.
func badCommandLine(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "max-restool: %v\nRun max-restool help for the usage.\n", err)
	return exitCommandLine
}

// runtimeFailure prints a runtime error in the max-restool form and returns the runtime exit code.
func runtimeFailure(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "max-restool: %v\n", err)
	return exitRuntime
}

// checkPaths refuses a verb given the wrong number of paths or an empty one. names label the paths in order.
func checkPaths(verb string, paths []string, names ...string) error {
	if len(paths) != len(names) {
		return fmt.Errorf("%s needs %s, got %d argument(s)", verb, strings.Join(names, " "), len(paths))
	}
	for i, path := range paths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("%s: the path %s is empty", verb, names[i])
		}
	}
	return nil
}

// unpackByFlag handles the short form, where a flag names the archive and the
// default folder receives its files. The -file, -archive, and -res flags are
// aliases, and only one of them may be given.
func unpackByFlag(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("unpack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	archives := map[string]*string{}
	for _, name := range archiveFlags {
		archives[name] = fs.String(name, "", "archive to unpack")
	}
	dir := fs.String("path", restool.DefaultUnpackDir, "folder to unpack into")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage(stderr)
			return 0
		}
		return badCommandLine(stderr, err)
	}
	if fs.NArg() != 0 {
		return badCommandLine(stderr, fmt.Errorf("unexpected argument %q", fs.Arg(0)))
	}

	var given []string // the alias flags that were given, in the order of archiveFlags
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, name := range archiveFlags {
		if provided[name] {
			given = append(given, name)
		}
	}
	for _, name := range given {
		if strings.TrimSpace(*archives[name]) == "" {
			return badCommandLine(stderr, fmt.Errorf("flag -%s needs a value", name))
		}
	}
	if provided["path"] && strings.TrimSpace(*dir) == "" {
		return badCommandLine(stderr, fmt.Errorf("flag -path needs a value"))
	}
	if len(given) > 1 {
		return badCommandLine(stderr, fmt.Errorf("flags -%s and -%s both name the archive, give only one", given[0], given[1]))
	}
	if len(given) == 0 {
		return badCommandLine(stderr, fmt.Errorf("no archive named, give -res, -file or -archive <archive.res>"))
	}

	archive := *archives[given[0]]
	if err := restool.Unpack(archive, *dir); err != nil {
		return runtimeFailure(stderr, err)
	}
	fmt.Fprintf(stdout, "unpacked %s into %s\n", archive, *dir)
	return 0
}

func list(stdout io.Writer, path string) error {
	entries, err := restool.List(path)

	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Fprintf(stdout, "%-8s %d\n", entry.Tag, entry.Size)
	}

	return nil
}

func usage(out io.Writer) {
	fmt.Fprintf(out, "usage:\n")
	fmt.Fprintf(out, "  max-restool help\n")
	fmt.Fprintf(out, "  max-restool -res|-file|-archive <archive.res> [-path <dir>]\n")
	fmt.Fprintf(out, "  max-restool list <archive.res>\n")
	fmt.Fprintf(out, "  max-restool unpack <archive.res> <dir>\n")
	fmt.Fprintf(out, "  max-restool pack <dir> <archive.res>\n")
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "Naming an archive by flag unpacks it into %s under the current folder,\n", restool.DefaultUnpackDir)
	fmt.Fprintf(out, "unless -path names another one. The unpack verb spells the same thing out.\n")
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "unpack writes every resource as raw bytes plus a manifest, and pack turns\n")
	fmt.Fprintf(out, "such a directory back into a bit identical archive. The archive stays the\n")
	fmt.Fprintf(out, "only concern here: to turn resources into images, sounds or text, run\n")
	fmt.Fprintf(out, "max-convert over the unpacked directory.\n")
}
