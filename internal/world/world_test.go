// world_test.go : testing unit for world.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package world

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

var worldFiles = []string{
	"SNOW_%d.WRL", "CRATER_%d.WRL", "GREEN_%d.WRL", "DESERT_%d.WRL",
}

func TestAllPreviewsLoad(t *testing.T) {
	dir := testutil.DataDir(t)
	count := 0
	for _, pattern := range worldFiles {
		for i := 1; i <= 6; i++ {
			name := fmt.Sprintf(pattern, i)
			p, err := LoadPreview(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if p.W != 112 || p.H != 112 {
				t.Errorf("%s is %dx%d, want 112x112", name, p.W, p.H)
			}
			// Retail files carry "WRL\x01\x00" or "DMO\x01\x00"; the
			// engine never validates the magic.
			if m := string(p.Magic[:3]); m != "WRL" && m != "DMO" {
				t.Errorf("%s magic = %q", name, p.Magic)
			}
			for j, c := range p.Palette {
				if c > 63 {
					t.Fatalf("%s palette byte %d = %d exceeds 6-bit", name, j, c)
				}
			}
			count++
		}
	}
	if count != 24 {
		t.Errorf("loaded %d worlds, want 24", count)
	}
}

func TestGreen1Golden(t *testing.T) {
	dir := testutil.DataDir(t)
	p, err := LoadPreview(filepath.Join(dir, "GREEN_1.WRL"))
	if err != nil {
		t.Fatal(err)
	}

	// Golden recorded from the retail 1.0.4 data.
	h := sha256.New()
	h.Write(p.Minimap)
	h.Write(p.Palette[:])
	const want = "05e20cb11009b9a93afd8b93391244cbe1148802253cdad75c463600fa15932a"
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		t.Errorf("GREEN_1 hash = %s, want %s", got, want)
	}

	if buf := p.MinimapBuffer(); buf.W != 112 || buf.H != 112 || len(buf.Pix) < 112*112 {
		t.Error("MinimapBuffer shape wrong")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := ParsePreview([]byte("DMO")); err == nil {
		t.Error("short header should fail")
	}
	bad := make([]byte, 64)
	copy(bad, "DMO\x01\x00")
	bad[5] = 0xFF // width -? int16 0x00FF = 255 ok; set huge:
	bad[6] = 0x7F
	if _, err := ParsePreview(bad); err == nil {
		t.Error("implausible size should fail")
	}
}
