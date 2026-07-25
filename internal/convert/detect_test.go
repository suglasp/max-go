// detect_test.go : tests asset detection by extension and header.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// flicHeader builds the leading bytes of a FLI/FLC file header.
func flicHeader(kind uint16) []byte {
	b := make([]byte, 6)
	binary.LittleEndian.PutUint16(b[4:6], kind)
	return b
}

func TestDetectByExtension(t *testing.T) {
	tests := []struct {
		path string
		want Kind
	}{
		{"main_msc.msc", KindAudio},
		{"KBUY0.WAV", KindAudio},
		{"portrait.flc", KindAnimation},
		{"old.FLI", KindAnimation},
		{"MAXINT.mve", KindMovie},
		{"P_TRANSP", KindResource},
	}
	for _, tc := range tests {
		if got := Detect(tc.path, nil); got != tc.want {
			t.Errorf("Detect(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// A resource unpacked from a RES archive has no extension, so the leading bytes
// have to carry the classification.
func TestDetectByContent(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Kind
	}{
		{"mve", []byte(mveTag), KindMovie},
		{"wav", encodeWAV(22050, 1, []int16{0}), KindAudio},
		{"flc", flicHeader(flicTypeFLC), KindAnimation},
		{"fli", flicHeader(flicTypeFLI), KindAnimation},
		{"raw", []byte{1, 2, 3, 4, 5, 6}, KindResource},
	}
	for _, tc := range tests {
		if got := Detect("BLOB", tc.data); got != tc.want {
			t.Errorf("Detect(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A .MSC is a RIFF/WAVE under another name, so it converts by renaming.
func TestConvertFileAudio(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main_msc.msc")
	wav := encodeWAV(22050, 1, []int16{1, 2, 3})
	if err := os.WriteFile(src, wav, 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "out")
	written, err := ConvertFile(src, out, Options{Format: FormatBMP})
	if err != nil {
		t.Fatalf("ConvertFile: %v", err)
	}
	if len(written) != 1 || written[0] != "main_msc.wav" {
		t.Fatalf("written = %v, want [main_msc.wav]", written)
	}
	got, err := os.ReadFile(filepath.Join(out, "main_msc.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(wav) {
		t.Fatalf("converted audio changed size: %d, want %d", len(got), len(wav))
	}
}

func TestConvertFileUnrecognised(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "BLOB")
	if err := os.WriteFile(src, []byte{0x00, 0x01, 0xff, 0xfe, 0x80}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ConvertFile(src, filepath.Join(dir, "out"), Options{}); err == nil {
		t.Fatalf("expected an error for an unrecognised resource")
	}
}
