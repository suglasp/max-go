// map_test.go : testing unit for world map loading in world.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package world

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/testutil"
)

func TestAllMapsLoadFull(t *testing.T) {
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
			if m.W != 112 || m.H != 112 {
				t.Errorf("%s is %dx%d cells, want 112x112", name, m.W, m.H)
			}
			if m.TileCount <= 0 || m.TileCount > 1000 {
				t.Errorf("%s tile count %d implausible", name, m.TileCount)
			}
			// The layout accounts for every byte of the file.
			expect := 9 + m.W*m.H + m.W*m.H*2 + 2 +
				m.TileCount*TileSize*TileSize + gfx.PaletteSize*gfx.PaletteStride + m.TileCount
			if len(data) != expect {
				t.Errorf("%s size %d, layout predicts %d", name, len(data), expect)
			}
			land := 0
			for _, s := range m.Surface {
				if s == SurfaceLand {
					land++
				}
			}
			if land == 0 {
				t.Errorf("%s has no land cells", name)
			}
			if buf := m.Tile(m.TileCount - 1); buf.W != TileSize || buf.H != TileSize {
				t.Errorf("%s tile buffer shape wrong", name)
			}
		}
	}
}

func TestGreen1MapGolden(t *testing.T) {
	dir := testutil.DataDir(t)
	m, err := LoadMap(filepath.Join(dir, "GREEN_1.WRL"))
	if err != nil {
		t.Fatal(err)
	}

	// Golden recorded from the retail 1.0.4 data.
	h := sha256.New()
	ids := make([]byte, len(m.TileIDs)*2)
	for i, id := range m.TileIDs {
		binary.LittleEndian.PutUint16(ids[i*2:], id)
	}
	h.Write(ids)
	h.Write(m.Tiles)
	h.Write(m.PassTable)
	h.Write(m.Surface)
	const want = "9203b34669be72ba37f90d8c6fed02b72189ae1ff5c380099e2729c17d627552"
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		t.Errorf("GREEN_1 map hash = %s, want %s", got, want)
	}

	if got := m.SurfaceAt(-1, 0); got != SurfaceNone {
		t.Errorf("SurfaceAt out of bounds = %d, want SurfaceNone", got)
	}
}

// TestRetailBugFixes checks the data corrections guarded by hash against
// the retail files (skipped when the data set differs from the original
// reference hashes).
func TestRetailBugFixes(t *testing.T) {
	dir := testutil.DataDir(t)

	check := func(name, refHash string, verify func(*Map) error) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != refHash {
			t.Skipf("%s does not match the reference hash; fix not applicable", name)
		}
		m, err := LoadMap(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := verify(m); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	check("GREEN_4.WRL", hashGreen4, func(m *Map) error {
		for i, id := range m.TileIDs {
			if id == 190 && m.Surface[i] != SurfaceLand {
				return fmt.Errorf("cell %d with tile 190 is %d, want land", i, m.Surface[i])
			}
		}
		return nil
	})
	check("SNOW_3.WRL", hashSnow3, func(m *Map) error {
		if got := m.TileIDs[m.W*56+56]; got != 55 {
			return fmt.Errorf("tile at (57,57) = %d, want 55", got)
		}
		return nil
	})
}

// synthMap builds a minimal valid 2x2 map with two tiles.
func synthMap() []byte {
	var b []byte
	b = append(b, 'W', 'R', 'L', 1, 0)    // magic + layer count 1
	b = append(b, 2, 0, 2, 0)             // W=2, H=2
	b = append(b, 10, 11, 12, 13)         // minimap
	b = append(b, 0, 0, 1, 0, 1, 0, 0, 0) // tile ids 0,1,1,0
	b = append(b, 2, 0)                   // tile count
	b = append(b, make([]byte, 2*TileSize*TileSize)...)
	b = append(b, make([]byte, gfx.PaletteSize*gfx.PaletteStride)...)
	b = append(b, 0, 1) // pass table: land, water
	return b
}

func TestParseMapSynthetic(t *testing.T) {
	m, err := ParseMap(synthMap())
	if err != nil {
		t.Fatal(err)
	}
	if m.TileCount != 2 {
		t.Errorf("tile count = %d, want 2", m.TileCount)
	}
	wantSurface := []uint8{SurfaceLand, SurfaceWater, SurfaceWater, SurfaceLand}
	for i, want := range wantSurface {
		if m.Surface[i] != want {
			t.Errorf("surface[%d] = %d, want %d", i, m.Surface[i], want)
		}
	}
	if got := m.SurfaceAt(1, 0); got != SurfaceWater {
		t.Errorf("SurfaceAt(1,0) = %d, want water", got)
	}
}

func TestParseMapErrors(t *testing.T) {
	full := synthMap()
	cuts := []struct {
		name string
		size int
	}{
		{"tile ids", 9 + 4 + 4},
		{"tile pixels", 9 + 4 + 8 + 2 + 100},
		{"pass table", len(full) - 1},
	}
	for _, c := range cuts {
		if _, err := ParseMap(full[:c.size]); err == nil {
			t.Errorf("truncated at %s should fail", c.name)
		}
	}

	bad := append([]byte(nil), full...)
	bad[len(bad)-1] = 9 // pass value out of range
	if _, err := ParseMap(bad); err == nil {
		t.Error("bad pass table value should fail")
	}

	bad = append([]byte(nil), full...)
	bad[9+4] = 7 // tile id 7 of 2
	if _, err := ParseMap(bad); err == nil {
		t.Error("out of range tile id should fail")
	}
}
