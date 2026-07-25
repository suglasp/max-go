// banner_test.go : tests the ingame message and alert banner display.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
)

// loadFont5ForBanner loads the real FONT5 the banner uses (skips without data).
func loadFont5ForBanner(t *testing.T) *fonts.Font {
	t.Helper()
	dir := os.Getenv("MAX_DATA_DIR")
	if dir == "" {
		t.Skip("game data not found (set MAX_DATA_DIR)")
	}
	f, err := fonts.LoadFile(filepath.Join(dir, "FONT5.FON"))
	if err != nil {
		t.Skipf("FONT5.FON not available: %v", err)
	}
	return f
}

// TestWrapBannerTextStaysInWidth checks a long banner message wraps to several
// lines that each fit the cap, so it can never run off the map / over the side
// UI frame. A short message stays a single line.
func TestWrapBannerTextStaysInWidth(t *testing.T) {
	f := loadFont5ForBanner(t)
	width := MainMap.Width() - 2*bannerPad

	long := "The enemy gun turret was destroyed and your construction crew needs more raw material to continue building the new mining station."
	lines := wrapBannerText(f, long, width)
	if len(lines) < 2 {
		t.Fatalf("a long message must wrap to multiple lines, got %d: %q", len(lines), lines)
	}
	for i, ln := range lines {
		if w := f.Width(ln); w > width {
			t.Errorf("line %d %q width %d exceeds the cap %d", i, ln, w, width)
		}
	}

	short := wrapBannerText(f, "Enemy detected!", width)
	if len(short) != 1 {
		t.Errorf("a short message must stay one line, got %d: %q", len(short), short)
	}
}

// TestWrapBannerTextHonorsNewlines: explicit \n breaks become separate lines (one
// shut building per line in the "not enough power" banner), each still word wrapped.
func TestWrapBannerTextHonorsNewlines(t *testing.T) {
	f := loadFont5ForBanner(t)
	width := MainMap.Width() - 2*bannerPad

	lines := wrapBannerText(f, "Research Centre shut down.\nLight Vehicle Plant shut down.", width)
	if len(lines) != 2 {
		t.Fatalf("two short lines separated by \\n should stay 2 lines, got %d: %q", len(lines), lines)
	}
	if lines[0] != "Research Centre shut down." || lines[1] != "Light Vehicle Plant shut down." {
		t.Errorf("lines = %q, want the two building lines preserved", lines)
	}
}

// TestWrapBannerTextSplitsLongWord confirms a single word wider than the box is
// broken into chunks rather than overflowing.
func TestWrapBannerTextSplitsLongWord(t *testing.T) {
	f := loadFont5ForBanner(t)
	width := 40 // a deliberately narrow box

	lines := wrapBannerText(f, "Supercalifragilisticexpialidocious", width)
	if len(lines) < 2 {
		t.Fatalf("a word longer than the box must split, got %d: %q", len(lines), lines)
	}
	for i, ln := range lines {
		if w := f.Width(ln); w > width {
			t.Errorf("split chunk %d %q width %d exceeds the cap %d", i, ln, w, width)
		}
	}
}
