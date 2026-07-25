// main.go : is the max-restool entry point for listing, unpacking, and packing M.A.X. RES archives.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/restool"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "-help", "--help":
		// Print command help.
		usage()
		return 0
	}

	// A leading dash means the short form: name an archive and it unpacks.
	if strings.HasPrefix(args[0], "-") {
		return unpackByFlag(args)
	}

	switch args[0] {
	case "list":
		// List the files in a RES archive.
		if len(args) != 2 {
			usage()
			return 2
		}

		if err := list(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

	case "unpack":
		// Unpack a RES archive into loose raw files.
		if len(args) != 3 {
			usage()
			return 2
		}

		if err := restool.Unpack(args[1], args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

	case "pack":
		// Pack loose raw files into a RES archive.
		if len(args) != 3 {
			usage()
			return 2
		}

		dst := args[2]
		if dir := filepath.Dir(dst); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}

		if err := restool.Pack(args[1], dst); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

	default:
		usage()
		return 2
	}

	return 0
}

// unpackByFlag handles the short form, where a flag names the archive and the
// default folder receives its files. The -file, -archive, and -res flags are
// aliases.
func unpackByFlag(args []string) int {
	fs := flag.NewFlagSet("unpack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var archive string
	for _, name := range []string{"res", "file", "archive"} {
		fs.StringVar(&archive, name, "", "archive to unpack")
	}
	dir := fs.String("path", restool.DefaultUnpackDir, "folder to unpack into")

	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || archive == "" || *dir == "" {
		usage()
		return 2
	}

	if err := restool.Unpack(archive, *dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("unpacked %s into %s\n", archive, *dir)
	return 0
}

func list(path string) error {
	entries, err := restool.List(path)

	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Printf("%-8s %d\n", entry.Tag, entry.Size)
	}

	return nil
}

func usage() {
	out := os.Stderr
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
