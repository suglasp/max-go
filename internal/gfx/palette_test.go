// palette_test.go : tests COLOR.PAL parsing and generated lookup tables.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gfx

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

func loadRealPal(t *testing.T) *PalFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testutil.DataDir(t), "COLOR.PAL"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParsePalFile(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestParseRealColorPal(t *testing.T) {
	f := loadRealPal(t)

	// Golden hash of the 768-byte palette block of the retail 1.0.4 data.
	sum := sha256.Sum256(f.Palette[:])
	const want = "45c498e68ba89644dcc14eef509bd7e908b392b46e722f948bd1a0eb545bfd5d"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("palette hash = %s, want %s", got, want)
	}

	if r, g, b := f.Palette.RGB(0); r != 0 || g != 0 || b != 0 {
		t.Errorf("entry 0 = (%d,%d,%d), want black", r, g, b)
	}
	if r, g, b := f.Palette.RGB(int(ColorRed)); r != 63 || g != 0 || b != 0 {
		t.Errorf("COLOR_RED entry = (%d,%d,%d), want (63,0,0)", r, g, b)
	}
	for i, c := range f.Palette {
		if c > 63 {
			t.Fatalf("palette byte %d = %d, exceeds 6-bit range", i, c)
		}
	}
	if r, _, _ := f.Palette.RGBA8(int(ColorRed)); r != 252 {
		t.Errorf("RGBA8 red = %d, want 252 (6-bit*4, faithful to original)", r)
	}

	// Retail COLOR.PAL carries no 'NEWC' intensity table.
	if f.Intensity != nil {
		t.Error("retail COLOR.PAL unexpectedly has an intensity table")
	}
	/* // note: ported but unused code in Go port
	// Without a table, ColorIntensity is identity (a deviation from the original).
	if f.ColorIntensity(0x12600, 42) != 42 {
		t.Error("ColorIntensity without table should be identity")
	}
	if f.LightenTable()[42] != 42 {
		t.Error("LightenTable without intensity table should be identity")
	}
	*/
}

/* // note: ported but unused code in Go port
func TestRGB2ColorAgainstPalette(t *testing.T) {
	f := loadRealPal(t)

	// Mapping a palette entry's own RGB through the RGB555 table must
	// land on a color very close to that entry (often the entry
	// itself). This validates the r<<10|g<<5|b bit layout.
	worst := 0
	for _, i := range []int{1, 32, 64, 130, 200, 255} {
		r, g, b := f.Palette.RGB(i)
		rgb555 := int(r>>1)<<10 | int(g>>1)<<5 | int(b>>1)
		j := f.RGB2Color(rgb555)
		jr, jg, jb := f.Palette.RGB(int(j))
		if d := ColorDistance(r, g, b, jr, jg, jb); d > worst {
			worst = d
		}
	}
	if worst > 27 { // 3 units per channel (5-bit quantization error)
		t.Errorf("RGB2Color round trip worst distance = %d; bit layout suspect", worst)
	}
}
*/

func TestMapColor(t *testing.T) {
	f := loadRealPal(t)

	// Exact palette colors must map to a perfect match.
	r, g, b := f.Palette.RGB(2)
	idx := f.Palette.MapColor(r, g, b, true)
	ir, ig, ib := f.Palette.RGB(int(idx))
	if ColorDistance(r, g, b, ir, ig, ib) != 0 {
		t.Errorf("MapColor(full) of palette entry not exact: got %d", idx)
	}

	// Restricted scan never returns an excluded animated index.
	for _, c := range []struct{ r, g, b uint8 }{{63, 0, 0}, {0, 63, 0}, {0, 0, 63}, {30, 30, 30}} {
		i := int(f.Palette.MapColor(c.r, c.g, c.b, false))
		if (i >= 9 && i <= 31) || (i >= 96 && i <= 127) {
			t.Errorf("MapColor(restricted) returned excluded index %d", i)
		}
	}
}

func TestDarkeningTable(t *testing.T) {
	// A gray ramp in the allowed band 32..63 (avoiding MapColor's excluded
	// animated ranges 9..31 and 96..127): entry i holds value 2*(i-32).
	var p Palette
	for i := 32; i <= 63; i++ {
		v := uint8(2 * (i - 32))
		p.SetRGB(i, v, v, v)
	}
	dark := p.DarkeningTable()

	// The shadow of a bright entry must be a strictly darker palette entry.
	for _, i := range []int{63, 56, 48} {
		sr, _, _ := p.RGB(i)
		dr, _, _ := p.RGB(int(dark[i]))
		if dr >= sr {
			t.Errorf("darken[%d]: shadow value %d not darker than source %d", i, dr, sr)
		}
	}
	// Black stays black (max(7,0)*31/63 = 3 -> nearest is the darkest gray).
	if dr, _, _ := p.RGB(int(dark[32])); dr > 6 {
		t.Errorf("darken of black mapped to value %d, want near-black", dr)
	}
}

func TestRedTintTable(t *testing.T) {
	// A gray ramp (32..47) plus a pure red ramp (48..63), both in the allowed band
	// 32..63 (MapColor excludes 9..31 / 96..127), so the tint has red entries to map to.
	var p Palette
	for i := 32; i <= 47; i++ {
		v := uint8(2 * (i - 32))
		p.SetRGB(i, v, v, v) // gray
	}
	for i := 48; i <= 63; i++ {
		p.SetRGB(i, uint8(2*(i-48)+33), 0, 0) // pure red 33..63
	}
	red := p.RedTintTable(31)

	// A gray entry tints to a red dominant entry (green/blue killed).
	for _, i := range []int{40, 44, 47} {
		rr, gg, bb := p.RGB(int(red[i]))
		if rr <= gg || rr <= bb {
			t.Errorf("redTint[%d] -> (%d,%d,%d): want red-dominant", i, rr, gg, bb)
		}
	}
	// factor 0 defaults to 31 (no division by zero, still red dominant).
	rr, gg, bb := p.RGB(int(p.RedTintTable(0)[44]))
	if rr <= gg || rr <= bb {
		t.Errorf("redTint(factor 0)[44] -> (%d,%d,%d): want red-dominant", rr, gg, bb)
	}
}

func TestBrightnessTable(t *testing.T) {
	// A gray ramp in the allowed band 32..63 (avoiding MapColor's excluded
	// animated ranges): entry i holds value 2*(i-32).
	var p Palette
	for i := 32; i <= 63; i++ {
		v := uint8(2 * (i - 32))
		p.SetRGB(i, v, v, v)
	}

	// Level 7 is identity (color * 7/7 maps to itself).
	full := p.BrightnessTable(7)
	for _, i := range []int{32, 48, 63} {
		sr, _, _ := p.RGB(i)
		fr, _, _ := p.RGB(int(full[i]))
		if fr != sr {
			t.Errorf("BrightnessTable(7)[%d]: %d != identity %d", i, fr, sr)
		}
	}

	// The shadow level (5) dims a bright entry but stays lighter than the
	// darker DarkeningTable (5/7 ~ 0.71 vs 31/63 ~ 0.49).
	bright := p.BrightnessTable(5)
	dark := p.DarkeningTable()
	br, _, _ := p.RGB(int(bright[63]))
	dr, _, _ := p.RGB(int(dark[63]))
	if br <= dr {
		t.Errorf("brightness-5 shadow (%d) should be lighter than darkening (%d)", br, dr)
	}
	if br >= 126 { // must still be a real dim of the source (126)
		t.Errorf("brightness-5 shadow of 126 not dimmed: %d", br)
	}
}

func TestIntensityTables(t *testing.T) {
	f := loadRealPal(t)

	// Table1 with zero blend factor1 must nearly match identity (each color
	// snaps to itself, distance 0 terminates the search at i).
	t1 := f.Palette.IntensityTable1(0, 0, 0, 0, 100)
	for i := 0; i < 256; i++ {
		r, g, b := f.Palette.RGB(i)
		jr, jg, jb := f.Palette.RGB(int(t1[i]))
		if ColorDistance(r, g, b, jr, jg, jb) != 0 {
			t.Errorf("IntensityTable1 identity broke at %d -> %d", i, t1[i])
		}
	}

	// Table3 darkening (level < factor) must not brighten black.
	t3 := f.Palette.IntensityTable3(20, 20, 20, 63)
	r, g, b := f.Palette.RGB(int(t3[0]))
	if int(r)+int(g)+int(b) > 24 {
		t.Errorf("IntensityTable3 mapped black to bright (%d,%d,%d)", r, g, b)
	}

	// Table2 red tint: a gray input should land on a reddish entry.
	t2 := f.Palette.IntensityTable2(63, 0, 0)
	gray := f.Palette.MapColor(40, 40, 40, false)
	r, g, b = f.Palette.RGB(int(t2[gray]))
	if r <= g || r <= b {
		t.Errorf("IntensityTable2 red tint of gray gave (%d,%d,%d)", r, g, b)
	}

	// Determinism golden: hash of all three tables together, recorded
	// from the retail 1.0.4 palette.
	h := sha256.New()
	h.Write(t1[:])
	h.Write(t2[:])
	h.Write(t3[:])
	const wantHash = "bd3d18197d393bd3082122357e022e024216c7dff7d1b4996073c066fc558068"
	if got := hex.EncodeToString(h.Sum(nil)); got != wantHash {
		t.Errorf("intensity tables hash = %s, want %s", got, wantHash)
	}
}

func TestParsePalFileWithIntensity(t *testing.T) {
	// Synthetic file exercising the 'NEWC' branch.
	data := make([]byte, 768+32768+4+65536)
	data[3] = 63                                           // entry 1 red
	copy(data[768+32768:], []byte{0x43, 0x57, 0x45, 0x4E}) // 'NEWC' LE tag bytes
	data[768+32768+4+42*256+147] = 7                       // Intensity[42][147]

	f, err := ParsePalFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Intensity == nil {
		t.Fatal("intensity table not parsed")
	}
	if f.Intensity[42][147] != 7 {
		t.Errorf("Intensity[42][147] = %d, want 7", f.Intensity[42][147])
	}

	if _, err := ParsePalFile(data[:100]); err == nil {
		t.Error("short file should fail")
	}
	if _, err := ParsePalFile(data[:768+32768+4+100]); err == nil {
		t.Error("truncated intensity table should fail")
	}
}

// TestLoadBasePalette checks the COLOR.PAL loader the game, the map editor and the debug screens share:
// a valid file gives its palette, a missing or a short file gives the zero palette and an error.
func TestLoadBasePalette(t *testing.T) {
	dir := t.TempDir()

	if pal, err := LoadBasePalette(dir); err == nil || pal != (Palette{}) {
		t.Errorf("missing file: palette zero = %v, err = %v; want the zero palette and an error", pal == (Palette{}), err)
	}

	data := make([]byte, PalFileBaseSize)
	data[3], data[4], data[5] = 63, 31, 7 // entry 1
	if err := os.WriteFile(filepath.Join(dir, "COLOR.PAL"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	pal, err := LoadBasePalette(dir)
	if err != nil {
		t.Fatalf("valid file: %v", err)
	}
	if r, g, b := pal.RGB(1); r != 63 || g != 31 || b != 7 {
		t.Errorf("entry 1 = %d,%d,%d, want 63,31,7", r, g, b)
	}

	if err := os.WriteFile(filepath.Join(dir, "COLOR.PAL"), data[:100], 0o644); err != nil {
		t.Fatal(err)
	}
	if pal, err := LoadBasePalette(dir); err == nil || pal != (Palette{}) {
		t.Errorf("short file: palette zero = %v, err = %v; want the zero palette and an error", pal == (Palette{}), err)
	}
}
