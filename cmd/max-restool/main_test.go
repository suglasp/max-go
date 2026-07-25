// main_test.go : tests max-restool argument handling and exit codes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"os"
	"path/filepath"
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

// Running without arguments prints usage and succeeds.
func TestRunNoArgs(t *testing.T) {
	if code := run(nil); code != 0 {
		t.Fatalf("run(nil) = %d, want 0", code)
	}
}

// Every help alias succeeds.
func TestRunHelpAliases(t *testing.T) {
	for _, arg := range []string{"help", "-h", "-help", "--help"} {
		if code := run([]string{arg}); code != 0 {
			t.Errorf("run(%q) = %d, want 0", arg, code)
		}
	}
}

// An unknown verb returns the usage error code.
func TestRunUnknownCommand(t *testing.T) {
	if code := run([]string{"frobnicate"}); code != 2 {
		t.Fatalf("run(unknown) = %d, want 2", code)
	}
}

// The conversion flags moved to max-convert, so extract is no longer a verb.
func TestRunExtractVerbIsGone(t *testing.T) {
	if code := run([]string{"extract", "a.res", "d"}); code != 2 {
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
		if code := run(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// A valid verb with a missing archive returns the runtime error code.
func TestRunMissingArchive(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.res")
	if code := run([]string{"list", missing}); code != 1 {
		t.Errorf("run(list missing) = %d, want 1", code)
	}
	if code := run([]string{"unpack", missing, t.TempDir()}); code != 1 {
		t.Errorf("run(unpack missing) = %d, want 1", code)
	}
	if code := run([]string{"-res", missing}); code != 1 {
		t.Errorf("run(-res missing) = %d, want 1", code)
	}
}

// Each spelling of the archive flag unpacks into the default folder.
func TestRunFlagFormDefaultsToExtracted(t *testing.T) {
	archive := tinyArchive(t)
	for _, name := range []string{"-res", "-file", "-archive"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if code := run([]string{name, archive}); code != 0 {
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
	if code := run([]string{"-res", archive, "-path", dir}); code != 0 {
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
		if code := run(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}
