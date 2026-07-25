// restool_test.go : testing unit for restool.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package restool

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeTestArchive(t *testing.T, entries map[string][]byte, order []string) string {
	t.Helper()
	var out bytes.Buffer
	var index bytes.Buffer
	out.Write(make([]byte, headerSize))
	for _, tag := range order {
		var rec [entrySize]byte
		copy(rec[:8], tag)
		binary.LittleEndian.PutUint32(rec[8:12], uint32(out.Len()))
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(entries[tag])))
		out.Write(entries[tag])
		index.Write(rec[:])
	}
	indexOffset := out.Len()
	out.Write(index.Bytes())
	buf := out.Bytes()
	copy(buf[:4], magic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(indexOffset))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(index.Len()))

	path := filepath.Join(t.TempDir(), "test.res")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListUnpackPackRoundTrip(t *testing.T) {
	src := writeTestArchive(t,
		map[string][]byte{
			"ALPHA":    []byte("first"),
			"BETA":     []byte{0, 1, 2, 3},
			"LONGTAG8": []byte("eight"),
		},
		[]string{"BETA", "ALPHA", "LONGTAG8"})

	entries, err := List(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{entries[0].Tag, entries[1].Tag, entries[2].Tag}; got[0] != "BETA" || got[1] != "ALPHA" || got[2] != "LONGTAG8" {
		t.Fatalf("List order = %v", got)
	}
	if entries[0].Size != 4 || entries[1].Size != 5 || entries[2].Size != 5 {
		t.Fatalf("List sizes = %+v", entries)
	}

	dir := filepath.Join(t.TempDir(), "extract")
	if err := Unpack(src, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, manifestName)); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "roundtrip.res")
	if err := Pack(dir, dst); err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("extract + pack did not reproduce archive bytes")
	}
}

func TestUnpackCreatesOutputDirectory(t *testing.T) {
	src := writeTestArchive(t, map[string][]byte{"ALPHA": []byte("first")}, []string{"ALPHA"})
	dir := filepath.Join(t.TempDir(), "missing", "extract")

	if err := Unpack(src, dir); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("Unpack did not create output directory: stat=%v err=%v", st, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ALPHA")); err != nil {
		t.Fatalf("extracted file missing: %v", err)
	}
}

func TestPackWithoutManifestUsesSortedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "BETA"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ALPHA"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "packed.res")
	if err := Pack(dir, out); err != nil {
		t.Fatal(err)
	}
	entries, err := List(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Tag != "ALPHA" || entries[1].Tag != "BETA" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestPackRejectsManifestHashMismatch(t *testing.T) {
	src := writeTestArchive(t, map[string][]byte{"ALPHA": []byte("first")}, []string{"ALPHA"})
	dir := filepath.Join(t.TempDir(), "extract")
	if err := Unpack(src, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ALPHA"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Pack(dir, filepath.Join(t.TempDir(), "bad.res")); err == nil {
		t.Fatal("Pack succeeded with a stale manifest hash")
	}
}
