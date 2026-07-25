// writer_test.go : testing unit for writer.go - byte exact .WRL round trips.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package world

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

// firstDiff returns the index of the first differing byte, or -1 when equal.
func firstDiff(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}

// TestEncodeMapRoundTripSynthetic round trips the synthetic 2x2 map, no data.
func TestEncodeMapRoundTripSynthetic(t *testing.T) {
	data := synthMap()
	m, err := ParseMap(data)
	if err != nil {
		t.Fatal(err)
	}
	out, err := EncodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs at byte %d", firstDiff(out, data))
	}
}

// TestEncodeMapRoundTripRetail round trips every retail .WRL: parse the raw
// file, re-encode, require an exact byte match. Uses ParseMap (not LoadMap) so
// the hash keyed retail data fixes never run - EncodeMap must reproduce the
// file verbatim, including the raw 8-bit palette.
func TestEncodeMapRoundTripRetail(t *testing.T) {
	dir := testutil.DataDir(t)
	for _, pattern := range worldFiles {
		for i := 1; i <= 6; i++ {
			name := fmt.Sprintf(pattern, i)
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			m, err := ParseMap(data)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			out, err := EncodeMap(m)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !bytes.Equal(out, data) {
				t.Errorf("%s: round trip differs (encoded %d bytes vs %d) at offset %d",
					name, len(out), len(data), firstDiff(out, data))
			}
		}
	}
}

// TestSaveMapRoundTrip writes a parsed map to a temp file and reads it back
// byte for byte.
func TestSaveMapRoundTrip(t *testing.T) {
	data := synthMap()
	m, err := ParseMap(data)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "out.wrl")
	if err := SaveMap(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("saved file differs at byte %d", firstDiff(got, data))
	}
}

// TestEncodeMapValidates rejects a Map whose slices are inconsistent instead of
// writing a corrupt file.
func TestEncodeMapValidates(t *testing.T) {
	m, err := ParseMap(synthMap())
	if err != nil {
		t.Fatal(err)
	}
	m.Minimap = m.Minimap[:len(m.Minimap)-1]
	if _, err := EncodeMap(m); err == nil {
		t.Error("a short minimap should fail to encode")
	}
}
