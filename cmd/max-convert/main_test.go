// main_test.go : tests max-convert argument handling, exit codes, and palette resolution.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/convert"
	"github.com/suglasp/max-go/internal/restool"
)

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

// Each verb reports a usage error when its positional arguments are wrong.
func TestRunWrongArgCount(t *testing.T) {
	cases := [][]string{
		{"res", "a", "b"},
		{"res", "-path", "a", "b"}, // the folder named twice
		{"file"},
		{"file", "onlyinput"},
	}
	for _, args := range cases {
		if code := run(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

// With no folder named, res reads the one max-restool unpacks into.
func TestResolveDirDefaultsToUnpackDir(t *testing.T) {
	dir, ok := resolveDir("", nil)
	if !ok || dir != restool.DefaultUnpackDir {
		t.Fatalf("resolveDir(no folder) = (%q, %v), want (%q, true)", dir, ok, restool.DefaultUnpackDir)
	}
}

// -path and a positional folder both win over the default, but not together.
func TestResolveDirOverrides(t *testing.T) {
	if dir, ok := resolveDir("byflag", nil); !ok || dir != "byflag" {
		t.Errorf("resolveDir(-path) = (%q, %v), want (byflag, true)", dir, ok)
	}
	if dir, ok := resolveDir("", []string{"positional"}); !ok || dir != "positional" {
		t.Errorf("resolveDir(positional) = (%q, %v), want (positional, true)", dir, ok)
	}
	if _, ok := resolveDir("byflag", []string{"positional"}); ok {
		t.Errorf("naming the folder twice should be refused")
	}
}

// An image format the encoder does not know is a usage error.
func TestRunUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	if code := run([]string{"res", "-format", "tga", dir}); code != 2 {
		t.Errorf("run(res -format tga) = %d, want 2", code)
	}
	if code := run([]string{"file", "-format", "tga", "in", dir}); code != 2 {
		t.Errorf("run(file -format tga) = %d, want 2", code)
	}
}

// An empty directory converts nothing, which is not an error.
func TestRunResEmptyDir(t *testing.T) {
	if code := run([]string{"res", t.TempDir()}); code != 0 {
		t.Fatalf("run(res, empty dir) = %d, want 0", code)
	}
}

// A bare res command searches the current directory for the default folder.
// A missing folder is a runtime error rather than a usage error.
func TestRunResBareUsesDefaultFolder(t *testing.T) {
	t.Chdir(t.TempDir())
	if code := run([]string{"res"}); code != 1 {
		t.Fatalf("run(res) with no %s folder = %d, want 1", restool.DefaultUnpackDir, code)
	}
}

// A folder walk converts supported files, skips previous output, and continues
// past unsupported files.
func TestRunResWalksFolder(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("MAIN_MSC", []byte("main_msc.msc\x00"))       // text resource
	write("BLOB", []byte{0x00, 0x01, 0xff, 0xfe, 0x80}) // nothing to make of it
	write("OLD.bmp", []byte("BM"))                      // output from an earlier run

	if code := run([]string{"res", "-path", dir}); code != 0 {
		t.Fatalf("run(res -path) = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "MAIN_MSC.txt")); err != nil {
		t.Errorf("text resource was not converted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "OLD.bmp.bmp")); err == nil {
		t.Errorf("an already converted file was converted again")
	}
}

// A missing input file is a run error, not a usage error.
func TestRunFileMissingInput(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.wav")
	if code := run([]string{"file", missing, dir}); code != 1 {
		t.Fatalf("run(file missing) = %d, want 1", code)
	}
}

// Without -pal, a missing COLOR.PAL selects the grayscale fallback.
func TestLoadPaletteMissingIsNotAnError(t *testing.T) {
	pal, err := loadPalette(io.Discard, "", t.TempDir())
	if err != nil {
		t.Fatalf("loadPalette: %v", err)
	}
	if pal != nil {
		t.Fatalf("expected no palette, got one")
	}
}

// An explicit -pal that cannot be read or parsed is an error.
func TestLoadPaletteExplicitBad(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadPalette(io.Discard, filepath.Join(dir, "nope.pal"), dir); err == nil {
		t.Errorf("expected an error for a missing explicit palette")
	}

	bad := filepath.Join(dir, "bad.pal")
	if err := os.WriteFile(bad, []byte("too short"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPalette(io.Discard, bad, dir); err == nil {
		t.Errorf("expected an error for an unparseable explicit palette")
	}
}

// The converter finds COLOR.PAL beside the input without an explicit path.
func TestLoadPaletteBesideInput(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 768+32768)
	for i := 0; i < 768; i++ {
		data[i] = byte(i % 64)
	}
	if err := os.WriteFile(filepath.Join(dir, "COLOR.PAL"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	pal, err := loadPalette(io.Discard, "", dir)
	if err != nil {
		t.Fatalf("loadPalette: %v", err)
	}
	if pal == nil {
		t.Fatalf("expected the neighbouring COLOR.PAL to be loaded")
	}
}

// A failed file prints its name and the reason it failed.
func TestPrintFileResultFailedShowsReason(t *testing.T) {
	var buf bytes.Buffer
	printFileResult(&buf, convert.FileResult{
		Source: "BAD_RES",
		Status: convert.StatusFailed,
		Err:    errors.New("unsupported header"),
	})
	want := "file BAD_RES failed to convert: unsupported header\n"
	if got := buf.String(); got != want {
		t.Fatalf("printed %q, want %q", got, want)
	}
}

// Converted and already converted files print the same lines as before.
func TestPrintFileResultOtherStatuses(t *testing.T) {
	cases := []struct {
		name string
		res  convert.FileResult
		want string
	}{
		{"already", convert.FileResult{Source: "A", Status: convert.StatusAlreadyConverted},
			"file A already converted!\n"},
		{"converted", convert.FileResult{Source: "B", Status: convert.StatusConverted, Outputs: []string{"B.txt"}},
			"file B converted to B.txt [txt]\n"},
		{"many frames", convert.FileResult{Source: "C", Status: convert.StatusConverted, Outputs: []string{"C_0.bmp", "C_1.bmp", "C_2.bmp"}},
			"file C converted to C_0.bmp [bmp] (+2 more)\n"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		printFileResult(&buf, tc.res)
		if got := buf.String(); got != tc.want {
			t.Errorf("%s: printed %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A missing implicit COLOR.PAL warns once, names the path, and says grayscale.
func TestLoadPaletteMissingWarns(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if _, err := loadPalette(&buf, "", dir); err != nil {
		t.Fatalf("loadPalette: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "COLOR.PAL") || !strings.Contains(out, "missing") || !strings.Contains(out, "grayscale") {
		t.Fatalf("warning %q lacks path, reason or grayscale note", out)
	}
	if n := strings.Count(out, "\n"); n != 1 {
		t.Fatalf("want exactly one warning line, got %d: %q", n, out)
	}
}

// An implicit COLOR.PAL of the wrong size warns that it is invalid.
func TestLoadPaletteInvalidWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "COLOR.PAL"), []byte("too short"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	pal, err := loadPalette(&buf, "", dir)
	if err != nil || pal != nil {
		t.Fatalf("loadPalette = (%v, %v), want (nil, nil)", pal, err)
	}
	out := buf.String()
	if !strings.Contains(out, "invalid") || !strings.Contains(out, "grayscale") {
		t.Fatalf("warning %q lacks reason or grayscale note", out)
	}
}

// A valid palette beside the input gives no warning.
func TestLoadPaletteValidNoWarning(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "COLOR.PAL"), make([]byte, 768+32768), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	pal, err := loadPalette(&buf, "", dir)
	if err != nil || pal == nil {
		t.Fatalf("loadPalette = (%v, %v), want a palette", pal, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected warning %q", buf.String())
	}
}
