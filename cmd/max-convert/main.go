// main.go : is the max-convert entry point for turning M.A.X. assets into images, sounds, and text.
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

	"github.com/suglasp/max-go/internal/convert"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
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

	case "res":
		// Convert every resource unpacked by max-restool.
		return runRes(args[1:])

	case "file":
		// Detect and convert one asset.
		return runFile(args[1:])

	default:
		usage()
		return 2
	}
}

// flags builds the option set both verbs share.
func flags(name string) (*flag.FlagSet, *string, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	format := fs.String("format", "bmp", "image format: bmp or png")
	pal := fs.String("pal", "", "path to COLOR.PAL (default: COLOR.PAL beside the input)")
	return fs, format, pal
}

func runRes(args []string) int {
	fs, format, palPath := flags("res")
	path := fs.String("path", "", "folder of unpacked resources")
	if err := fs.Parse(args); err != nil || fs.NArg() > 1 {
		usage()
		return 2
	}
	imgFormat, ok := convert.ParseFormat(*format)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown format %q, want bmp or png\n", *format)
		return 2
	}

	dir, ok := resolveDir(*path, fs.Args())
	if !ok {
		usage()
		return 2
	}
	pal, err := loadPalette(os.Stderr, *palPath, dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	rep, err := convert.ConvertDir(dir, convert.Options{
		Palette:     pal,
		DumpPalette: pal != nil,
		Format:      imgFormat,
		OnFile:      func(r convert.FileResult) { printFileResult(os.Stdout, r) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("converted %d, already converted %d, failed %d\n", rep.Converted, rep.AlreadyDone, rep.Failed)
	return 0
}

// printFileResult reports each file as the walk proceeds so large directories
// continue to show progress. A failed file names the reason on the same line.
func printFileResult(w io.Writer, r convert.FileResult) {
	switch r.Status {
	case convert.StatusAlreadyConverted:
		fmt.Fprintf(w, "file %s already converted!\n", r.Source)

	case convert.StatusFailed:
		if r.Err == nil {
			fmt.Fprintf(w, "file %s failed to convert!\n", r.Source)
			break
		}
		fmt.Fprintf(w, "file %s failed to convert: %v\n", r.Source, r.Err)

	case convert.StatusConverted:
		// Animations and movies can produce many frames. Show the source file
		// instead of listing every frame.
		more := ""
		if rest := len(r.Outputs) - 1; rest > 0 {
			more = fmt.Sprintf(" (+%d more)", rest)
		}
		fmt.Fprintf(w, "file %s converted to %s [%s]%s\n", r.Source, r.Outputs[0], r.Ext(), more)
	}
}

func runFile(args []string) int {
	fs, format, palPath := flags("file")
	if err := fs.Parse(args); err != nil || fs.NArg() != 2 {
		usage()
		return 2
	}
	imgFormat, ok := convert.ParseFormat(*format)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown format %q, want bmp or png\n", *format)
		return 2
	}

	src, outDir := fs.Arg(0), fs.Arg(1)
	pal, err := loadPalette(os.Stderr, *palPath, filepath.Dir(src))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	written, err := convert.ConvertFile(src, outDir, convert.Options{Palette: pal, Format: imgFormat})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, name := range written {
		fmt.Println(name)
	}
	return 0
}

// resolveDir uses -path, a positional folder, or max-restool's default folder,
// in that order. It rejects commands that specify both explicit forms.
func resolveDir(path string, rest []string) (string, bool) {
	switch {
	case path != "" && len(rest) > 0:
		return "", false
	case path != "":
		return path, true
	case len(rest) == 1:
		return rest[0], true
	case len(rest) == 0:
		return restool.DefaultUnpackDir, true
	}
	return "", false
}

// loadPalette reads an explicit COLOR.PAL or looks beside the input. A missing
// or invalid implicit palette selects the grayscale fallback and writes one
// warning to w saying why, while an invalid explicit palette returns an error.
func loadPalette(w io.Writer, explicit, dir string) (*gfx.Palette, error) {
	path := explicit
	if path == "" {
		path = resfile.DataPath(dir, "COLOR.PAL")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if explicit == "" {
			fmt.Fprintf(w, "warning: palette %s is missing or unreadable (%v), output will be grayscale\n", path, err)
			return nil, nil
		}
		return nil, err
	}
	pf, err := gfx.ParsePalFile(data)
	if err != nil {
		if explicit == "" {
			fmt.Fprintf(w, "warning: palette %s is invalid (%v), output will be grayscale\n", path, err)
			return nil, nil
		}
		return nil, err
	}
	return &pf.Palette, nil
}

func usage() {
	out := os.Stderr
	fmt.Fprintf(out, "usage:\n")
	fmt.Fprintf(out, "  max-convert help\n")
	fmt.Fprintf(out, "  max-convert res  [-format bmp|png] [-pal COLOR.PAL] [-path <dir>]\n")
	fmt.Fprintf(out, "  max-convert file [-format bmp|png] [-pal COLOR.PAL] <input> <outdir>\n")
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "res converts every resource in a directory written by max-restool unpack,\n")
	fmt.Fprintf(out, "leaving the raw files and the manifest alone so the directory still packs\n")
	fmt.Fprintf(out, "back into a bit identical archive. It reads %s under the current folder\n", restool.DefaultUnpackDir)
	fmt.Fprintf(out, "unless -path names another one. file converts one asset and works out what\n")
	fmt.Fprintf(out, "it is: .MSC and .WAV become a .wav, .FLC and .MVE become numbered frames,\n")
	fmt.Fprintf(out, "and a movie also yields its sound track.\n")
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "BMP is the default image format. It carries no alpha channel, so a\n")
	fmt.Fprintf(out, "transparent pixel comes out fuchsia; use -format png to keep real\n")
	fmt.Fprintf(out, "transparency.\n")
}
