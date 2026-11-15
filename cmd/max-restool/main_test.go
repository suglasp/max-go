// main_test.go : tests max-restool argument handling and exit codes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/restool"
)

// tinyArchive packs one resource and returns the archive path.
func tinyArchive(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "HELLO"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "tiny.res")
	if err := restool.Pack(src, archive); err != nil {
		t.Fatal(err)
	}
	return archive
}

// runQuiet runs the tool and drops both streams.
func runQuiet(args []string) int {
	return run(args, io.Discard, io.Discard)
}

// runCapture runs the tool and returns the exit code with both streams.
func runCapture(args []string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// Running without arguments prints usage and succeeds.
func TestRunNoArgs(t *testing.T) {
	if code := runQuiet(nil); code != 0 {
		t.Fatalf("runQuiet(nil) = %d, want 0", code)
	}
}

// Every help alias succeeds.
func TestRunHelpAliases(t *testing.T) {
	for _, arg := range []string{"help", "-h", "-help", "--help"} {
		if code := runQuiet([]string{arg}); code != 0 {
			t.Errorf("run(%q) = %d, want 0", arg, code)
		}
	}
}

// An unknown verb returns the usage error code.
func TestRunUnknownCommand(t *testing.T) {
	if code := runQuiet([]string{"frobnicate"}); code != 2 {
		t.Fatalf("run(unknown) = %d, want 2", code)
	}
}

// The conversion flags moved to max-convert, so extract is no longer a verb.
func TestRunExtractVerbIsGone(t *testing.T) {
	if code := runQuiet([]string{"extract", "a.res", "d"}); code != 2 {
		t.Fatalf("run(extract) = %d, want 2", code)
	}
}

// Each verb reports a usage error when its positional arguments are missing.
func TestRunWrongArgCount(t *testing.T) {
	cases := [][]string{
		{"list"},
		{"unpack", "onlyarchive"},
		{"pack", "onlydir"},
	}
	for _, args := range cases {
		if code := runQuiet(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// A valid verb with a missing archive returns the runtime error code.
func TestRunMissingArchive(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.res")
	if code := runQuiet([]string{"list", missing}); code != 1 {
		t.Errorf("run(list missing) = %d, want 1", code)
	}
	if code := runQuiet([]string{"unpack", missing, t.TempDir()}); code != 1 {
		t.Errorf("run(unpack missing) = %d, want 1", code)
	}
	if code := runQuiet([]string{"-res", missing}); code != 1 {
		t.Errorf("run(-res missing) = %d, want 1", code)
	}
}

// Each spelling of the archive flag unpacks into the default folder.
func TestRunFlagFormDefaultsToExtracted(t *testing.T) {
	archive := tinyArchive(t)
	for _, name := range []string{"-res", "-file", "-archive"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if code := runQuiet([]string{name, archive}); code != 0 {
				t.Fatalf("run(%s) = %d, want 0", name, code)
			}
			if _, err := os.Stat(filepath.Join(restool.DefaultUnpackDir, "HELLO")); err != nil {
				t.Fatalf("resource not unpacked into %s: %v", restool.DefaultUnpackDir, err)
			}
		})
	}
}

// -path sends the resources somewhere else.
func TestRunFlagFormPathOverride(t *testing.T) {
	archive := tinyArchive(t)
	dir := filepath.Join(t.TempDir(), "elsewhere")
	if code := runQuiet([]string{"-res", archive, "-path", dir}); code != 0 {
		t.Fatalf("run(-res -path) = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "HELLO")); err != nil {
		t.Fatalf("resource not unpacked into -path folder: %v", err)
	}
}

// The flag form needs an archive, takes no positional arguments, and rejects
// flags it does not know.
func TestRunFlagFormUsageErrors(t *testing.T) {
	cases := [][]string{
		{"-path", "somewhere"},         // no archive named
		{"-res"},                       // flag without its value
		{"-res", "a.res", "stray"},     // positional leftover
		{"-frobnicate", "a.res"},       // unknown flag
		{"-res", "a.res", "-path", ""}, // empty destination
	}
	for _, args := range cases {
		if code := runQuiet(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// Every bad command line says which usage error happened, with the tool prefix, and exits 2.
func TestRunCommandLineErrorMessages(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown command", []string{"frobnicate"}, `max-restool: unknown command "frobnicate", want list, unpack, pack, help or a -res, -file or -archive flag`},
		{"unknown flag", []string{"-bogus", "a.res"}, "max-restool: flag provided but not defined: -bogus"},
		{"flag without value", []string{"-res"}, "max-restool: flag needs an argument: -res"},
		{"empty archive res", []string{"-res="}, "max-restool: flag -res needs a value"},
		{"empty archive file", []string{"-file="}, "max-restool: flag -file needs a value"},
		{"empty archive archive", []string{"-archive="}, "max-restool: flag -archive needs a value"},
		{"empty path", []string{"-res", "a.res", "-path="}, "max-restool: flag -path needs a value"},
		{"two aliases", []string{"-res", "a.res", "-file", "b.res"}, "max-restool: flags -res and -file both name the archive, give only one"},
		{"two aliases same value", []string{"-archive", "a.res", "-res", "a.res"}, "max-restool: flags -res and -archive both name the archive, give only one"},
		{"no archive", []string{"-path", "somewhere"}, "max-restool: no archive named, give -res, -file or -archive <archive.res>"},
		{"stray argument", []string{"-res", "a.res", "stray"}, `max-restool: unexpected argument "stray"`},
		{"list without archive", []string{"list"}, "max-restool: list needs <archive.res>, got 0 argument(s)"},
		{"unpack one path", []string{"unpack", "a.res"}, "max-restool: unpack needs <archive.res> <dir>, got 1 argument(s)"},
		{"pack three paths", []string{"pack", "a", "b", "c"}, "max-restool: pack needs <dir> <archive.res>, got 3 argument(s)"},
		{"list empty path", []string{"list", ""}, "max-restool: list: the path <archive.res> is empty"},
		{"unpack empty dir", []string{"unpack", "a.res", " "}, "max-restool: unpack: the path <dir> is empty"},
		{"pack empty archive", []string{"pack", "d", ""}, "max-restool: pack: the path <archive.res> is empty"},
	}
	for _, tc := range cases {
		code, _, stderr := runCapture(tc.args)
		if code != 2 {
			t.Errorf("%s: exit %d, want 2", tc.name, code)
		}
		if !strings.HasPrefix(stderr, tc.want) {
			t.Errorf("%s: stderr %q does not start with %q", tc.name, stderr, tc.want)
		}
	}
}

// A runtime failure carries the tool prefix and exits 1.
func TestRunRuntimeErrorsArePrefixed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.res")
	for _, args := range [][]string{{"list", missing}, {"unpack", missing, t.TempDir()}, {"-res", missing}} {
		code, _, stderr := runCapture(args)
		if code != 1 {
			t.Errorf("run(%v) = %d, want 1", args, code)
		}
		if !strings.HasPrefix(stderr, "max-restool: ") {
			t.Errorf("run(%v) stderr %q lacks the tool prefix", args, stderr)
		}
	}
}

// -h after the archive flag still prints the usage and exits 0.
func TestRunFlagFormHelpExitsZero(t *testing.T) {
	code, _, stderr := runCapture([]string{"-res", "a.res", "-h"})
	if code != 0 || !strings.Contains(stderr, "usage:") {
		t.Fatalf("run(-res a.res -h) = %d, stderr %q, want 0 and the usage", code, stderr)
	}
}
