// fonts_test.go : tests FON decoding and GNW text rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package fonts

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/testutil"
)

func loadSet(t *testing.T) *Set {
	t.Helper()
	s, err := LoadSet(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLoadAllRealFonts(t *testing.T) {
	s := loadSet(t)

	// Heights as hardcoded in text.cpp FontMetas.
	wantHeights := []int{10, 16, 9, 9, 11, 10, 16}
	for i, want := range wantHeights {
		f := s.Get(i)
		if f == nil {
			t.Fatalf("font %d missing", i)
		}
		if f.Height != want {
			t.Errorf("FONT%d height = %d, want %d", i, f.Height, want)
		}
		if len(f.Glyphs) != 128 && len(f.Glyphs) != 256 {
			t.Errorf("FONT%d has %d glyphs", i, len(f.Glyphs))
		}
		if f.GlyphWidth('A') <= 0 {
			t.Errorf("FONT%d glyph 'A' has no width", i)
		}
	}
}

func TestGlyphArtFont1(t *testing.T) {
	// Visual check of bit order and orientation: FONT1 'A' must look
	// like an A (rows with the most significant bit first).
	s := loadSet(t)
	f := s.Get(Font1)
	g := f.glyph('A')

	var art strings.Builder
	ink := 0
	for y := 0; y < f.Height; y++ {
		for x := 0; x < g.Width; x++ {
			if g.Pixel(x, y) {
				art.WriteByte('#')
				ink++
			} else {
				art.WriteByte('.')
			}
		}
		art.WriteByte('\n')
	}
	t.Logf("FONT1 'A' (%dx%d):\n%s", g.Width, f.Height, art.String())
	if ink < 10 {
		t.Errorf("glyph 'A' has only %d set pixels", ink)
	}
}

func TestBlitString(t *testing.T) {
	s := loadSet(t)
	f := s.Get(Font1)

	w := f.Width("M.A.X.")
	buf := gfx.New(w, f.Height)
	f.Blit(buf, "M.A.X.", 0, 0x04) // plain, color 4

	ink := 0
	for _, c := range buf.Pix {
		if c == 4 {
			ink++
		} else if c != 0 {
			t.Fatalf("unexpected color %d in plain blit", c)
		}
	}
	if ink == 0 {
		t.Fatal("nothing rendered")
	}
	// Golden recorded from the retail FONT1.FON.
	sum := sha256.Sum256(buf.Pix)
	const want = "406b1c8d917314c5052eb3b29a79ba5dc3ad777a2c98e1f0ab598e1d19e04c3c"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("blit hash = %s, want %s (w=%d ink=%d)", got, want, w, ink)
	}
}

func TestBlitFlags(t *testing.T) {
	s := loadSet(t)
	f := s.Get(Font2)

	// Underline paints the bottom row across the advance width.
	buf := gfx.New(f.Width("Hi"), f.Height)
	f.Blit(buf, "Hi", 0, 0x05|FlagUnderline)
	bottom := buf.Row(f.Height - 1)
	for x, c := range bottom {
		if c != 5 {
			t.Errorf("underline missing at x=%d (=%d)", x, c)
			break
		}
	}

	// Monospace: total advance is cell * len, wider than proportional.
	if f.MonoWidth("ii") <= f.Width("ii") {
		t.Errorf("MonoWidth(ii)=%d not wider than Width(ii)=%d", f.MonoWidth("ii"), f.Width("ii"))
	}
	mw := f.MonoWidth("ii")
	buf = gfx.New(mw, f.Height)
	f.Blit(buf, "ii", 0, 0x05|FlagMonospace)

	// Outline: color-0 pixels neighbor the glyph pixels.
	buf = gfx.New(f.Width("O")+2, f.Height+1)
	buf.Fill(9)
	f.Blit(buf, "O", 0, 0x05|FlagOutline)
	outline := 0
	for _, c := range buf.Pix {
		if c == 0 {
			outline++
		}
	}
	if outline == 0 {
		t.Error("outline flag drew no outline pixels")
	}

	// Truncation: a tiny maxWidth draws at most that many columns.
	buf = gfx.New(100, f.Height)
	f.Blit(buf, "WWWWWWWW", 7, 0x05)
	for y := 0; y < f.Height; y++ {
		for x := 8; x < 100; x++ {
			if buf.At(x, y) != 0 {
				t.Fatalf("pixel beyond maxWidth at (%d,%d)", x, y)
			}
		}
	}
}

func TestBlitShadowed(t *testing.T) {
	s := loadSet(t)
	f := s.Get(Font1)
	c := Color{Base: 165, Outline: 175, Shadow: 199}

	buf := gfx.New(f.Width("MAX")+2, f.Height+2)
	buf.Fill(9)
	f.BlitShadowed(buf, "MAX", 0, c)

	counts := map[uint8]int{}
	for _, px := range buf.Pix {
		counts[px]++
	}
	if counts[c.Base] == 0 {
		t.Error("no base-color pixels")
	}
	if counts[c.Outline] == 0 {
		t.Error("no inner bevel pixels (down-right neighbor inside glyph)")
	}
	if counts[c.Shadow] == 0 {
		t.Error("no drop shadow pixels")
	}
	if counts[9] == 0 {
		t.Error("background fully overwritten; blit must be transparent")
	}

	// Every shadow pixel was cast by a text pixel up and to the left of it;
	// that pixel may itself have become bevel in a later (higher) pass of
	// the cascade that runs from the bottom up.
	for y := 1; y < buf.H; y++ {
		for x := 1; x < buf.W; x++ {
			if buf.At(x, y) == c.Shadow {
				if ul := buf.At(x-1, y-1); ul != c.Base && ul != c.Outline {
					t.Fatalf("orphan shadow at (%d,%d), up-left=%d", x, y, ul)
				}
			}
		}
	}
}

func TestSetSelection(t *testing.T) {
	s := loadSet(t)
	if err := s.SetFont(Font5); err != nil {
		t.Fatal(err)
	}
	if s.GetFont() != Font5 || s.Font() != s.Get(Font5) {
		t.Error("SetFont/GetFont mismatch")
	}
	if err := s.SetFont(9); err == nil {
		t.Error("SetFont(9) should fail (only 0..6 exist)")
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := Decode([]byte{1, 2, 3}); err == nil {
		t.Error("short header should fail")
	}
	// Header with absurd count.
	bad := make([]byte, 64)
	bad[0] = 0xFF
	bad[1] = 0xFF
	if _, err := Decode(bad); err == nil {
		t.Error("absurd glyph count should fail")
	}
}
