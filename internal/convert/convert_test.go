// convert_test.go : tests resource classification and directory conversion.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/restool"
)

func le16(v int16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name string
		want Format
		ok   bool
		ext  string
	}{
		{"bmp", FormatBMP, true, ".bmp"},
		{"BMP", FormatBMP, true, ".bmp"},
		{"png", FormatPNG, true, ".png"},
		{"tga", FormatBMP, false, ".bmp"},
	}
	for _, tc := range tests {
		got, ok := ParseFormat(tc.name)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseFormat(%q) = (%v, %v), want (%v, %v)", tc.name, got, ok, tc.want, tc.ok)
		}
		if got.Ext() != tc.ext {
			t.Errorf("ParseFormat(%q).Ext() = %q, want %q", tc.name, got.Ext(), tc.ext)
		}
	}
}

func TestConvertResourceDetection(t *testing.T) {
	pal := grayscalePalette()

	// Simple image: 2x2, exact 8 + w*h bytes.
	simple := append(append(append(le16(2), le16(2)...), le16(0)...), le16(0)...)
	simple = append(simple, 0, 1, 2, 3)

	// Palette resource: 256x3 raw bytes.
	palData := make([]byte, 768)
	for i := range palData {
		palData[i] = byte(i)
	}

	tests := []struct {
		name  string
		tag   string
		data  []byte
		image bool   // output is an image, so the suffix follows the format
		want  string // expected suffix when not an image, "" means not converted
	}{
		{name: "simple", tag: "BTN", data: simple, image: true},
		{name: "palette", tag: "PAL", data: palData, image: true},
		{name: "text", tag: "MAIN_MSC", data: []byte("main_msc.msc\x00"), want: ".txt"},
		{name: "binary", tag: "BLOB", data: []byte{0x00, 0x01, 0xff, 0xfe, 0x80, 0x03, 0x7f}},
	}
	for _, tc := range tests {
		for _, format := range []Format{FormatBMP, FormatPNG} {
			t.Run(tc.name+"/"+format.Ext(), func(t *testing.T) {
				want := tc.want
				if tc.image {
					want = format.Ext()
				}
				suffix, out, ok := ConvertResource(tc.tag, tc.data, &pal, format)
				if want == "" {
					if ok {
						t.Fatalf("expected no conversion, got suffix %q", suffix)
					}
					return
				}
				if !ok || suffix != want {
					t.Fatalf("got (%q, %v), want suffix %q", suffix, ok, want)
				}
				switch suffix {
				case ".png":
					if _, err := png.Decode(bytes.NewReader(out)); err != nil {
						t.Fatalf("output is not valid PNG: %v", err)
					}
				case ".bmp":
					if len(out) < 2 || out[0] != 'B' || out[1] != 'M' {
						t.Fatalf("output is not a BMP")
					}
				}
			})
		}
	}
}

// The walk reports every file it touches, in one of three states, and never
// stops on the one it cannot convert.
func TestConvertDirReportsEveryFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("MAIN_MSC", []byte("main_msc.msc\x00"))
	write("BLOB", []byte{0x00, 0x01, 0xff, 0xfe, 0x80})
	write("OLD.png", []byte("not really a png"))
	write(restool.ManifestName, []byte("# max-go RES manifest v1\n"))

	seen := map[string]FileResult{}
	rep, err := ConvertDir(dir, Options{Format: FormatBMP, OnFile: func(r FileResult) {
		seen[r.Source] = r
	}})
	if err != nil {
		t.Fatalf("ConvertDir: %v", err)
	}

	if rep.Converted != 1 || rep.AlreadyDone != 1 || rep.Failed != 1 {
		t.Fatalf("report = %+v, want 1 converted, 1 already done, 1 failed", rep)
	}
	if _, ok := seen[restool.ManifestName]; ok {
		t.Errorf("the manifest should not be reported as an asset")
	}
	if r := seen["MAIN_MSC"]; r.Status != StatusConverted || r.Ext() != "txt" {
		t.Errorf("MAIN_MSC = %+v, want a converted txt", r)
	}
	if r := seen["OLD.png"]; r.Status != StatusAlreadyConverted {
		t.Errorf("OLD.png = %+v, want already converted", r)
	}
	if r := seen["BLOB"]; r.Status != StatusFailed || r.Err == nil {
		t.Errorf("BLOB = %+v, want a failure carrying its reason", r)
	}
}

func TestAsText(t *testing.T) {
	if s, ok := asText([]byte("kbuy0.wav\x00\x00")); !ok || s != "kbuy0.wav" {
		t.Fatalf("asText sound ref: got (%q, %v)", s, ok)
	}
	if _, ok := asText([]byte{0x01, 0x02, 0x00, 0x03}); ok {
		t.Fatalf("asText should reject binary with control bytes")
	}
	if _, ok := asText(nil); ok {
		t.Fatalf("asText should reject empty")
	}
}
