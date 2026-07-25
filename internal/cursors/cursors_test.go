// cursors_test.go : tests cursor decoding, animation, and retail resources.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package cursors

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// simpleImage builds an ImageSimpleHeader resource: w, h, ulx, uly (int16
// LE) followed by w*h palette indices.
func simpleImage(w, h, ulx, uly int, pix []byte) []byte {
	b := make([]byte, 8+len(pix))
	binary.LittleEndian.PutUint16(b[0:], uint16(int16(w)))
	binary.LittleEndian.PutUint16(b[2:], uint16(int16(h)))
	binary.LittleEndian.PutUint16(b[4:], uint16(int16(ulx)))
	binary.LittleEndian.PutUint16(b[6:], uint16(int16(uly)))
	copy(b[8:], pix)
	return b
}

type fakeReader map[string][]byte

func (f fakeReader) Read(tag string) ([]byte, error) {
	if d, ok := f[tag]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("missing %s", tag)
}

func TestDecodeCursorStatic(t *testing.T) {
	pix := []byte{7, 1, 2, 3, 4, 5, 6, 8, 9} // 3x3, pix[0]=7 is transparent
	c, err := decodeCursor(simpleImage(3, 3, 1, 2, pix))
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 3 || c.H != 3 || c.FrameCount != 1 {
		t.Fatalf("dims = %dx%d frames=%d, want 3x3 frames=1", c.W, c.H, c.FrameCount)
	}
	if c.HotX != 1 || c.HotY != 2 {
		t.Fatalf("hotspot = (%d,%d), want (1,2)", c.HotX, c.HotY)
	}
	if c.Transparent != 7 {
		t.Fatalf("transparent = %d, want 7", c.Transparent)
	}
	if c.Animated() {
		t.Fatal("static cursor reported as animated")
	}
}

func TestDecodeCursorAnimated(t *testing.T) {
	// 2 wide, 6 tall -> 3 frames of 2x2. Each frame filled with its index.
	pix := []byte{
		0, 0, 0, 0, // frame 0
		1, 1, 1, 1, // frame 1
		2, 2, 2, 2, // frame 2
	}
	c, err := decodeCursor(simpleImage(2, 6, 0, 0, pix))
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 2 || c.H != 2 || c.FrameCount != 3 {
		t.Fatalf("got %dx%d frames=%d, want 2x2 frames=3", c.W, c.H, c.FrameCount)
	}
	for f := 0; f < 3; f++ {
		buf := c.Frame(f)
		if got := buf.At(0, 0); got != uint8(f) {
			t.Fatalf("frame %d pixel = %d, want %d", f, got, f)
		}
	}
}

func TestLoadTolerant(t *testing.T) {
	r := fakeReader{"HANDPTR": simpleImage(4, 4, 0, 0, make([]byte, 16))}
	cat, err := LoadCatalog(r)
	if err == nil {
		t.Fatal("expected a joined error for the missing cursors")
	}
	if cat.Get(Hand) == nil {
		t.Fatal("Hand should have loaded")
	}
	if cat.Get(Path) != nil {
		t.Fatal("Path should be absent")
	}
	if got := len(cat.Missing()); got != Count-1 {
		t.Fatalf("Missing() = %d, want %d", got, Count-1)
	}
}

func TestManagerSwitchAndAnimate(t *testing.T) {
	cat := &Catalog{}
	cat.cursors[Hand] = &Cursor{W: 2, H: 2, FrameCount: 1, pix: make([]byte, 4)}
	cat.cursors[Path] = &Cursor{W: 2, H: 2, FrameCount: 3, pix: make([]byte, 12)}

	m := NewManager(cat)
	if m.Current() != Hand {
		t.Fatalf("default cursor = %d, want Hand", m.Current())
	}

	// Static cursor never animates.
	m.Tick(1000)
	if m.FrameIndex() != 0 {
		t.Fatalf("static cursor advanced to frame %d", m.FrameIndex())
	}

	m.Set(Path)
	if m.FrameIndex() != 0 {
		t.Fatal("Set should reset the animation frame")
	}
	m.Tick(FrameDurationMS - 1)
	if m.FrameIndex() != 0 {
		t.Fatalf("advanced too early to frame %d", m.FrameIndex())
	}
	m.Tick(1) // now at exactly one frame duration
	if m.FrameIndex() != 1 {
		t.Fatalf("frame = %d, want 1", m.FrameIndex())
	}
	m.Tick(2 * FrameDurationMS) // +2, wraps 1 -> 0 (3 frames)
	if m.FrameIndex() != 0 {
		t.Fatalf("frame = %d, want wrap to 0", m.FrameIndex())
	}

	// Selecting the same id does not reset a running animation.
	m.Tick(FrameDurationMS)
	prev := m.FrameIndex()
	m.Set(Path)
	if m.FrameIndex() != prev {
		t.Fatalf("re-Set same id reset frame %d -> %d", prev, m.FrameIndex())
	}
}

/* // note: ported but unused code in Go port
func TestDefaultWindowCursor(t *testing.T) {
	if got := DefaultWindowCursor(0); got != Hand {
		t.Fatalf("window 0 = %d, want Hand", got)
	}
	if got := DefaultWindowCursor(38); got != Map {
		t.Fatalf("window 38 = %d, want Map", got)
	}
	if got := DefaultWindowCursor(39); got != ArrowN {
		t.Fatalf("window 39 = %d, want ArrowN", got)
	}
	if got := DefaultWindowCursor(9999); got != Hand {
		t.Fatalf("out-of-range = %d, want Hand", got)
	}
}
*/

// TestLoadRealData loads every cursor from the retail archive and pins the
// known shapes, including the animated PTR_PATH (ten 23x23 frames).
func TestLoadRealData(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog(lib)
	if err != nil {
		t.Fatalf("not all cursors loaded: %v", err)
	}
	if miss := cat.Missing(); len(miss) != 0 {
		t.Fatalf("missing cursors: %v", miss)
	}

	checks := []struct {
		id           ID
		w, h, frames int
	}{
		{Hidden, 1, 1, 1},
		{Hand, 18, 18, 1},
		{ArrowN, 16, 16, 1},
		{Map, 25, 25, 1},
		{Path, 23, 23, 10}, // the animated waypoint cursor
	}
	for _, c := range checks {
		cur := cat.Get(c.id)
		if cur == nil {
			t.Fatalf("cursor %d did not load", c.id)
		}
		if cur.W != c.w || cur.H != c.h || cur.FrameCount != c.frames {
			t.Fatalf("cursor %d = %dx%d frames=%d, want %dx%d frames=%d",
				c.id, cur.W, cur.H, cur.FrameCount, c.w, c.h, c.frames)
		}
	}
}
