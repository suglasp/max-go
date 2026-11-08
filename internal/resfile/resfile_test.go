// resfile_test.go : testing unit for resfile.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package resfile

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

func TestNormTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ILOGO\x00/2", "ILOGO"},   // garbage after NUL (real PATCHES.RES data)
		{"mainpic", "MAINPIC"},     // case insensitive
		{"D_DEFALT", "D_DEFALT"},   // full 8 chars
		{"D_DEFALTXY", "D_DEFALT"}, // longer than 8
		{"\x00garbage", ""},        // empty tag
		{" FONT_1 ", "FONT_1"},     // stray spaces
	}
	for _, tt := range tests {
		if got := NormTag(tt.in); got != tt.want {
			t.Errorf("NormTag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// writeArchive builds a minimal RES0 archive for synthetic tests.
func writeArchive(t *testing.T, name string, entries map[string][]byte, order []string) string {
	t.Helper()
	var data bytes.Buffer
	var index bytes.Buffer

	data.Write(make([]byte, HeaderSize)) // Reserve the header until the index offset is known.
	for _, tag := range order {
		var rec [EntrySize]byte
		copy(rec[:TagLen], tag)
		binary.LittleEndian.PutUint32(rec[8:12], uint32(data.Len()))
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(entries[tag])))
		data.Write(entries[tag])
		index.Write(rec[:])
	}
	indexOffset := data.Len()
	data.Write(index.Bytes())

	out := data.Bytes()
	copy(out[:4], Magic)
	binary.LittleEndian.PutUint32(out[4:8], uint32(indexOffset))
	binary.LittleEndian.PutUint32(out[8:12], uint32(index.Len()))

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSyntheticArchive(t *testing.T) {
	path := writeArchive(t, "test.res",
		map[string][]byte{"ALPHA": []byte("aaaa"), "BETA": []byte("bb")},
		[]string{"ALPHA", "BETA"})

	f, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := f.Tags(); len(got) != 2 || got[0] != "ALPHA" || got[1] != "BETA" {
		t.Errorf("Tags() = %v", got)
	}
	if !f.Has("alpha") {
		t.Error("Has(alpha) = false, want case-insensitive true")
	}
	data, err := f.Read("BETA")
	if err != nil || string(data) != "bb" {
		t.Errorf("Read(BETA) = %q, %v", data, err)
	}
	if _, err := f.Read("GAMMA"); err == nil {
		t.Error("Read(GAMMA) should fail")
	}
}

func TestBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.res")
	if err := os.WriteFile(path, []byte("NOTRES bla bla bla"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFile(path); err == nil {
		t.Error("OpenFile should reject bad magic")
	}
}

func TestLibraryOverride(t *testing.T) {
	patch := writeArchive(t, "patch.res",
		map[string][]byte{"ILOGO": []byte("patched")}, []string{"ILOGO"})
	base := writeArchive(t, "base.res",
		map[string][]byte{"ILOGO": []byte("original"), "OTHER": []byte("x")},
		[]string{"ILOGO", "OTHER"})

	lib, err := Open(patch, base)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	data, err := lib.Read("ILOGO")
	if err != nil || string(data) != "patched" {
		t.Errorf("Read(ILOGO) = %q, %v; want the patch archive to win", data, err)
	}
	data, err = lib.Read("OTHER")
	if err != nil || string(data) != "x" {
		t.Errorf("Read(OTHER) = %q, %v", data, err)
	}
}

func TestRealGameData(t *testing.T) {
	dir := testutil.DataDir(t)

	lib, err := OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	files := lib.Files()
	if len(files) != 2 {
		t.Fatalf("OpenDir opened %d archives, want 2 (PATCHES.RES + MAX.RES)", len(files))
	}
	// These counts describe M.A.X. 1.0.4 data with the 0.7.2 patches.
	if n := len(files[0].Tags()); n != 5 {
		t.Errorf("PATCHES.RES has %d entries, want 5", n)
	}
	if n := len(files[1].Tags()); n < 1500 {
		t.Errorf("MAX.RES has %d entries, want >= 1500", n)
	}

	for _, tag := range []string{"ILOGO", "FONT_1", "D_DEFALT", "SC_SCHEM"} {
		if _, err := lib.Read(tag); err != nil {
			t.Errorf("library is missing expected tag %q", tag)
		}
	}

	// PATCHES.RES must win for tags it contains.
	patched, err := files[0].Read("ILOGO")
	if err != nil {
		t.Fatal(err)
	}
	viaLib, err := lib.Read("ILOGO")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(patched, viaLib) {
		t.Error("lib.Read(ILOGO) did not return the PATCHES.RES version")
	}
}

// TestDataPathIgnoresCase covers the lower case map names MAX.RES stores
// against the upper case files on a case sensitive disk.
func TestDataPathIgnoresCase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SNOW_1.WRL"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A case insensitive disk opens the asked name as is, a case sensitive one
	// needs the name found on disk: either way the path must open.
	if got := DataPath(dir, "snow_1.wrl"); !fileExists(got) {
		t.Fatalf("DataPath(snow_1.wrl) = %q, which does not open", got)
	}

	// No match keeps the plain join, so the open error names the missing file.
	if got, want := DataPath(dir, "green_1.wrl"), filepath.Join(dir, "green_1.wrl"); got != want {
		t.Fatalf("DataPath(green_1.wrl) = %q, want %q", got, want)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
