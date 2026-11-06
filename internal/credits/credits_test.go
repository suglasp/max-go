// credits_test.go : tests timing and rendering for the scrolling credits.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package credits

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/testutil"
)

func TestDefaultLines(t *testing.T) {
	lines := DefaultLines(func(key string) string { return "key:" + key })
	if len(lines) != 229 {
		t.Fatalf("line count = %d, want 229", len(lines))
	}
	if lines[0] != (Line{Kind: LineTitle, Text: "key:0a6f"}) {
		t.Fatalf("first line = %+v", lines[0])
	}
	separators := 0
	titles := 0
	for _, line := range lines {
		switch line.Kind {
		case LineSeparator:
			separators++
		case LineTitle:
			titles++
		}
	}
	if separators < 40 {
		t.Fatalf("separator count = %d, want many", separators)
	}
	if titles < 50 {
		t.Fatalf("title count = %d, want many", titles)
	}
}

func TestAdvanceMath(t *testing.T) {
	s := New(solidBG(11), nil, []Line{{Kind: LineText, Text: "A"}}, 0xff, 0x04)
	// Smooth scroll: the offset tracks total elapsed time (9 px / 100 ms), so a
	// short frame nudges it a pixel or two rather than jumping in 9 px steps.
	s.Advance(50)
	if s.offset != 4 { // 50*9/100 = 4
		t.Fatalf("offset after 50ms = %d, want 4", s.offset)
	}
	s.Advance(50)
	if s.offset != 9 { // 100*9/100 = 9
		t.Fatalf("offset after 100ms = %d, want 9", s.offset)
	}
	// Many small advances accumulate to the same place as one big one.
	for i := 0; i < 10; i++ {
		s.Advance(10)
	}
	if s.offset != 18 { // 200*9/100 = 18
		t.Fatalf("offset after accumulated 200ms = %d, want 18", s.offset)
	}
}

func TestLifecycle(t *testing.T) {
	lines := []Line{
		{Kind: LineTitle, Text: "One"},
		{Kind: LineSeparator},
		{Kind: LineText, Text: "Two"},
	}
	s := New(solidBG(1), nil, lines, 0xff, 0x04)
	if s.done {
		t.Fatal("fresh screen is done")
	}
	for s.Advance(100) {
	}
	if !s.done {
		t.Fatal("screen did not finish")
	}
	if s.offset < columnH+len(lines)*lineSpacing {
		t.Fatalf("finish offset = %d, want at least %d", s.offset, columnH+len(lines)*lineSpacing)
	}
	if s.Advance(100) {
		t.Fatal("Advance returned true after Done")
	}
}

func TestDrawSyntheticNoFontGolden(t *testing.T) {
	bg := solidBG(3)
	lines := []Line{{Kind: LineTitle, Text: "Title"}, {Kind: LineText, Text: "Name"}}
	s := New(bg, nil, lines, 0xff, 0x04)
	s.offset = 117
	dst := gfx.New(screenW, screenH)
	dst.Fill(9)
	s.Draw(dst)
	got := hash(dst.Pix)
	const want = "bc84f5f29be0736c633aa5dacef6b2bd060b8012f2c5b60a0a82ca2cbbf23df1"
	if got != want {
		t.Fatalf("draw hash = %s, want %s", got, want)
	}
}

func TestDrawWithFontWhenDataAvailable(t *testing.T) {
	dir := testutil.DataDir(t)
	font, err := fonts.LoadFile(filepath.Join(dir, "FONT1.FON"))
	if err != nil {
		t.Fatal(err)
	}
	bg := solidBG(7)
	s := New(bg, font, []Line{
		{Kind: LineTitle, Text: "CREDITS"},
		{Kind: LineSeparator},
		{Kind: LineText, Text: "Ali Atabek"},
	}, 0xff, 0x04)
	s.offset = 230
	dst := gfx.New(screenW, screenH)
	s.Draw(dst)
	got := hash(dst.Pix)
	const want = "af01dfd65c87da86a4a1abf010f602db635285e7db9ab3ca64bd3b5c977afba9"
	if got != want {
		t.Fatalf("font draw hash = %s, want %s", got, want)
	}
}

func solidBG(color uint8) *gfx.Buffer {
	bg := gfx.New(screenW, screenH)
	bg.Fill(color)
	return bg
}

func hash(pix []uint8) string {
	sum := sha256.Sum256(pix)
	return hex.EncodeToString(sum[:])
}
