// multi_test.go : testing unit for multi.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sprites

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// loadResource reads one tag from the resource library beside the game data, or skips the test when no data directory is configured.
func loadResource(t *testing.T, tag string) []byte {
	t.Helper()
	lib, err := resfile.OpenDir(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	data, err := lib.Read(tag)
	if err != nil {
		t.Fatalf("read %s: %v", tag, err)
	}
	return data
}

func opaqueCount(m *Multi) int {
	n := 0
	for i := range m.Frames {
		for _, o := range m.Frames[i].Opaque {
			if o {
				n++
			}
		}
	}
	return n
}

func TestDecodeBomberSprite(t *testing.T) {
	m, err := DecodeMultiSprite(loadResource(t, "BOMBER"))
	if err != nil {
		t.Fatal(err)
	}
	// 16 frames = 8 facing angles in two passes (the BOMBER body sprite).
	if len(m.Frames) != 16 {
		t.Fatalf("frames = %d, want 16", len(m.Frames))
	}
	f0 := m.Frames[0]
	if f0.W != 62 || f0.H != 53 || f0.HotX != 30 || f0.HotY != 22 {
		t.Errorf("frame 0 = %dx%d hot %d,%d, want 62x53 hot 30,22", f0.W, f0.H, f0.HotX, f0.HotY)
	}
	if got := opaqueCount(m); got != 26075 {
		t.Errorf("opaque pixel total = %d, want 26075", got)
	}
	// Golden over frame 0's full pixel buffer (transparent = 0).
	sum := sha256.Sum256(f0.Pix)
	const want = "229d33dff0952bfcf35076833163804460f87618807cc8c535f5706fa2e5ee34"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("frame 0 pixel hash = %s, want %s", got, want)
	}
}

func TestDecodeBomberShadow(t *testing.T) {
	m, err := DecodeMultiShadow(loadResource(t, "S_BOMBER"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Frames) != 16 {
		t.Fatalf("frames = %d, want 16", len(m.Frames))
	}
	f0 := m.Frames[0]
	if f0.W != 62 || f0.H != 53 {
		t.Errorf("frame 0 = %dx%d, want 62x53", f0.W, f0.H)
	}
	// Shadow runs set only the silhouette mask; their pixel buffer remains zeroed.
	for _, p := range f0.Pix {
		if p != 0 {
			t.Fatalf("shadow frame has non-zero pixel %d", p)
		}
	}
	if opaqueCount(m) == 0 {
		t.Fatal("shadow silhouette is empty")
	}
	mask := make([]byte, len(f0.Opaque))
	for i, o := range f0.Opaque {
		if o {
			mask[i] = 1
		}
	}
	sum := sha256.Sum256(mask)
	const want = "7cb525d81e47a212426a2d69f88d5486919f3f06c023e74dc8e005486faf000e"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("frame 0 silhouette hash = %s, want %s", got, want)
	}
}

func TestTeamColorTable(t *testing.T) {
	// Outside the colorable band every team maps identically.
	for _, team := range []TeamColor{TeamRed, TeamGreen, TeamBlue, TeamGray, TeamDerelict} {
		tab := TeamColorTable(team)
		for _, i := range []int{0, 31, 40, 47, 64, 100, 255} {
			if tab[i] != uint8(i) {
				t.Errorf("team %d: table[%d] = %d, want identity", team, i, tab[i])
			}
		}
	}
	// The ranges 32..39 and 48..63 repeat the eight-entry ramp, while the gap at 40..47 remains unchanged.
	red := TeamColorTable(TeamRed)
	if red[32] != 56 || red[39] != 63 || red[48] != 56 || red[55] != 63 || red[56] != 56 || red[63] != 63 {
		t.Errorf("red band remap wrong: %d %d %d %d %d %d",
			red[32], red[39], red[48], red[55], red[56], red[63])
	}
	// Green's ramp is the band's own indices, so 32..39 stay identity.
	green := TeamColorTable(TeamGreen)
	if green[32] != 32 || green[39] != 39 || green[48] != 32 || green[63] != 39 {
		t.Errorf("green band remap wrong: %d %d %d %d", green[32], green[39], green[48], green[63])
	}
}

func TestMultiFrameDraw(t *testing.T) {
	f := MultiFrame{
		W: 2, H: 2, HotX: 1, HotY: 1,
		Pix:    []uint8{10, 0, 0, 20},
		Opaque: []bool{true, false, false, true},
	}
	dst := gfx.New(4, 4)
	dst.Fill(5)
	f.Draw(dst, 2, 2, nil) // hotspot (1,1) -> top left at (1,1)
	if dst.At(1, 1) != 10 || dst.At(2, 2) != 20 {
		t.Errorf("opaque pixels not placed: (1,1)=%d (2,2)=%d", dst.At(1, 1), dst.At(2, 2))
	}
	if dst.At(2, 1) != 5 || dst.At(1, 2) != 5 {
		t.Errorf("transparent pixels overwrote background")
	}

	// Remap applies per pixel.
	var remap [256]uint8
	for i := range remap {
		remap[i] = uint8(i)
	}
	remap[10] = 99
	dst.Fill(5)
	f.Draw(dst, 2, 2, &remap)
	if dst.At(1, 1) != 99 {
		t.Errorf("remap not applied: (1,1)=%d, want 99", dst.At(1, 1))
	}

	// Clipping: anchor at (0,0) puts the top left at (-1,-1); only the
	// bottom right sprite pixel lands on screen at (0,0).
	dst.Fill(5)
	f.Draw(dst, 0, 0, nil)
	if dst.At(0, 0) != 20 {
		t.Errorf("clipped draw: (0,0)=%d, want 20", dst.At(0, 0))
	}
}

func TestMultiFrameDrawShadow(t *testing.T) {
	f := MultiFrame{W: 1, H: 1, Pix: []uint8{0}, Opaque: []bool{true}}
	dst := gfx.New(2, 2)
	dst.Fill(7)
	var darken [256]uint8
	darken[7] = 3
	f.DrawShadow(dst, 0, 0, &darken)
	if dst.At(0, 0) != 3 {
		t.Errorf("shadow darken: (0,0)=%d, want 3", dst.At(0, 0))
	}
	if dst.At(1, 1) != 7 {
		t.Errorf("shadow touched a pixel outside its 1x1 mask")
	}
}

func TestDecodeMultiErrors(t *testing.T) {
	if _, err := DecodeMultiSprite([]byte{1}); err == nil {
		t.Error("1-byte input should fail")
	}
	// count claims 4 frames but no offset table follows.
	if _, err := DecodeMultiSprite([]byte{4, 0}); err == nil {
		t.Error("missing frame table should fail")
	}
	// count ok, offset points past the end.
	bad := []byte{1, 0, 0xFF, 0xFF, 0xFF, 0x7F}
	if _, err := DecodeMultiSprite(bad); err == nil {
		t.Error("out-of-range frame offset should fail")
	}
}

// fillFrame builds a fully opaque w-by-h frame filled with val and anchored at the given hotspot.
func fillFrame(w, h, hotX, hotY int, val uint8) MultiFrame {
	pix := make([]uint8, w*h)
	op := make([]bool, w*h)
	for i := range pix {
		pix[i] = val
		op[i] = true
	}
	return MultiFrame{W: w, H: h, HotX: hotX, HotY: hotY, Pix: pix, Opaque: op}
}

func TestDrawScaledOneToOneMatchesDraw(t *testing.T) {
	f := fillFrame(3, 2, 1, 1, 4)
	f.Pix[0], f.Pix[5] = 9, 12 // a couple of distinct pixels
	a := gfx.New(8, 8)
	b := gfx.New(8, 8)
	f.Draw(a, 5, 4, nil)
	f.DrawScaled(b, 5, 4, 1, 1, nil)
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			t.Fatalf("DrawScaled(1:1) differs from Draw at pixel %d: %d vs %d", i, b.Pix[i], a.Pix[i])
		}
	}
}

func TestDrawScaledUpscaleCoversArea(t *testing.T) {
	f := fillFrame(4, 4, 0, 0, 5)
	dst := gfx.New(40, 40)
	var remap [256]uint8
	remap[5] = 9
	// 4x zoom (num/den = 16/4): a 4x4 frame becomes 16x16, hotspot at (8,8).
	f.DrawScaled(dst, 8, 8, 16, 4, &remap)
	// Corners of the scaled footprint are written and remapped.
	for _, p := range [][2]int{{8, 8}, {8 + 15, 8}, {8, 8 + 15}, {8 + 15, 8 + 15}} {
		if got := dst.At(p[0], p[1]); got != 9 {
			t.Errorf("scaled pixel (%d,%d) = %d, want 9", p[0], p[1], got)
		}
	}
	// Just outside the 16-wide footprint stays clear.
	if dst.At(8+16, 8) != 0 {
		t.Error("DrawScaled wrote past its scaled width")
	}
}

func TestDrawScaledHotspotAnchored(t *testing.T) {
	// 2x2 frame, hotspot (1,1), drawn at 2x: scaled hotspot is (2,2), so the
	// scaled top left lands at draw point minus (2,2).
	f := fillFrame(2, 2, 1, 1, 6)
	dst := gfx.New(20, 20)
	f.DrawScaled(dst, 10, 10, 2, 1, nil)
	if dst.At(8, 8) != 6 {
		t.Errorf("scaled top-left = %d at (8,8), want 6 (hotspot anchoring)", dst.At(8, 8))
	}
}

func TestDrawShadowScaledDarkens(t *testing.T) {
	f := fillFrame(2, 2, 0, 0, 0)
	dst := gfx.New(20, 20)
	dst.Fill(5)
	var darken [256]uint8
	darken[5] = 2
	f.DrawShadowScaled(dst, 0, 0, 16, 4, &darken) // 2x2 -> 8x8 silhouette
	if dst.At(0, 0) != 2 || dst.At(7, 7) != 2 {
		t.Errorf("shadow scaled did not darken footprint: (0,0)=%d (7,7)=%d", dst.At(0, 0), dst.At(7, 7))
	}
	if dst.At(8, 8) != 5 {
		t.Error("shadow scaled darkened past its footprint")
	}
}
