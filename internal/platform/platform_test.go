// platform_test.go : testing unit for platform.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package platform

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/suglasp/max-go/internal/gfx"
)

func TestTranslateKey(t *testing.T) {
	tests := []struct {
		key   ebiten.Key
		shift bool
		alt   bool
		want  int
		ok    bool
	}{
		{ebiten.KeyA, false, false, 'a', true},
		{ebiten.KeyA, true, false, 'A', true},
		{ebiten.KeyDigit1, false, false, '1', true},
		{ebiten.KeyDigit1, true, false, '!', true},
		{ebiten.KeySlash, true, false, '?', true},
		{ebiten.KeyEscape, false, false, KeyEscape, true},
		{ebiten.KeyEnter, false, false, KeyReturn, true},
		{ebiten.KeyF1, false, false, 315, true},
		{ebiten.KeyF1, true, false, 340, true},  // SHIFT_F1
		{ebiten.KeyF12, true, false, 392, true}, // SHIFT_F12
		{ebiten.KeyArrowUp, false, false, 328, true},
		{ebiten.KeyArrowUp, true, false, 328, true},         // shift does not change nav keys
		{ebiten.KeyShift, false, false, 0, false},           // modifier has no code
		{ebiten.KeyS, false, true, AltMod | 's', true},      // Alt+S
		{ebiten.KeyDigit2, false, true, AltMod | '2', true}, // Alt+2
		{ebiten.KeyA, true, true, AltMod | 'a', true},       // alt takes precedence over shift
		{ebiten.KeyF5, false, true, AltMod | KeyF5, true},   // Alt+F5 (camera bookmark set)
		{ebiten.KeyF8, false, true, AltMod | KeyF8, true},   // Alt+F8
		{ebiten.KeyF5, false, false, KeyF5, true},           // plain F5 stays plain
		{ebiten.KeyArrowLeft, false, true, KeyLeft, true},   // Alt+arrow keeps its plain code (scroll)
	}
	for _, tt := range tests {
		got, ok := translateKey(tt.key, tt.shift, tt.alt)
		if got != tt.want || ok != tt.ok {
			t.Errorf("translateKey(%v, shift=%v, alt=%v) = %d,%v want %d,%v",
				tt.key, tt.shift, tt.alt, got, ok, tt.want, tt.ok)
		}
	}
}

func TestInputQueue(t *testing.T) {
	p := New(Config{})
	if got := p.GetInput(); got != -1 {
		t.Errorf("empty queue GetInput = %d, want -1", got)
	}
	p.inputs = append(p.inputs, 'a', KeyF1)
	if got := p.GetInput(); got != 'a' {
		t.Errorf("GetInput = %d, want 'a'", got)
	}
	if got := p.GetInput(); got != KeyF1 {
		t.Errorf("GetInput = %d, want KeyF1", got)
	}
	if got := p.GetInput(); got != -1 {
		t.Errorf("drained queue GetInput = %d, want -1", got)
	}
}

func TestBlitRGBA(t *testing.T) {
	src := gfx.New(2, 2)
	copy(src.Pix, []uint8{0, 1, 2, 1})
	var lut [256][4]uint8
	lut[0] = [4]uint8{0, 0, 0, 255}
	lut[1] = [4]uint8{252, 0, 0, 255}
	lut[2] = [4]uint8{0, 252, 0, 255}

	dst := make([]uint8, 2*2*4)
	blitRGBA(src, &lut, dst)

	want := []uint8{
		0, 0, 0, 255, 252, 0, 0, 255,
		0, 252, 0, 255, 252, 0, 0, 255,
	}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("dst = %v, want %v", dst, want)
		}
	}
}

func TestSetPaletteBuildsLUT(t *testing.T) {
	p := New(Config{})
	var pal gfx.Palette
	pal.SetRGB(5, 63, 31, 0)
	p.SetPalette(&pal)
	if p.lut[5] != [4]uint8{252, 124, 0, 255} {
		t.Errorf("lut[5] = %v", p.lut[5])
	}
	p.SetPaletteColor(5, 0, 0, 63)
	if p.lut[5] != [4]uint8{0, 0, 252, 255} {
		t.Errorf("lut[5] after SetPaletteColor = %v", p.lut[5])
	}
}

func TestSaveScreenshotCreatesFolder(t *testing.T) {
	base := t.TempDir()
	p := New(Config{BaseDir: base})
	var pal gfx.Palette
	pal.SetRGB(1, 63, 0, 0)
	p.SetPalette(&pal)
	p.Screen().Fill(1)

	path, err := p.SaveScreenshot()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(base, "screenshots") {
		t.Errorf("screenshot path %s not in screenshots folder", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != ScreenWidth || img.Bounds().Dy() != ScreenHeight {
		t.Errorf("screenshot is %v, want %dx%d", img.Bounds(), ScreenWidth, ScreenHeight)
	}
	r, _, _, _ := img.At(10, 10).RGBA()
	if r>>8 != 252 {
		t.Errorf("screenshot pixel red = %d, want 252", r>>8)
	}

	// Screenshots taken in the same second receive distinct names.
	path2, err := p.SaveScreenshot()
	if err != nil {
		t.Fatal(err)
	}
	if path2 == path {
		t.Error("second screenshot reused the same file name")
	}
}

func TestMonitorPrefersWide(t *testing.T) {
	cases := []struct {
		w, h int
		want bool
	}{
		{1920, 1080, true},  // 16:9
		{2560, 1440, true},  // 16:9
		{3440, 1440, true},  // 21:9 ultrawide
		{1680, 1050, true},  // 16:10
		{1280, 1024, false}, // 5:4
		{1024, 768, false},  // 4:3 (the old pentium CRT)
		{1500, 1000, false}, // 3:2
		{0, 0, false},       // unknown monitor = classic
	}
	for _, c := range cases {
		if got := monitorPrefersWide(c.w, c.h); got != c.want {
			t.Errorf("monitorPrefersWide(%dx%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

func TestSetLogicalSize(t *testing.T) {
	p := New(Config{})
	screen := p.Screen()

	p.SetLogicalSize(WideWidth, ScreenHeight)
	if p.Screen() != screen {
		t.Fatal("SetLogicalSize replaced the screen buffer pointer, want in place resize")
	}
	if screen.W != WideWidth || screen.H != ScreenHeight || screen.Stride != WideWidth {
		t.Fatalf("screen = %dx%d stride %d, want %dx%d stride %d", screen.W, screen.H, screen.Stride, WideWidth, ScreenHeight, WideWidth)
	}
	if len(screen.Pix) != WideWidth*ScreenHeight {
		t.Fatalf("screen pixels = %d, want %d", len(screen.Pix), WideWidth*ScreenHeight)
	}
	if len(p.rgba) != WideWidth*ScreenHeight*4 {
		t.Fatalf("rgba = %d bytes, want %d", len(p.rgba), WideWidth*ScreenHeight*4)
	}
	if w, h := p.Layout(0, 0); w != WideWidth || h != ScreenHeight {
		t.Fatalf("Layout = %dx%d, want %dx%d", w, h, WideWidth, ScreenHeight)
	}

	// Reapplying the current size preserves the buffers.
	pixBefore := &screen.Pix[0]
	p.SetLogicalSize(WideWidth, ScreenHeight)
	if &screen.Pix[0] != pixBefore {
		t.Error("no op SetLogicalSize reallocated the pixel slice")
	}

	// Switching back restores the 4:3 menu size.
	p.SetLogicalSize(ScreenWidth, ScreenHeight)
	if screen.W != ScreenWidth || len(p.rgba) != ScreenWidth*ScreenHeight*4 {
		t.Fatalf("restore: screen %dx%d rgba %d bytes", screen.W, screen.H, len(p.rgba))
	}
	if w, h := p.Layout(0, 0); w != ScreenWidth || h != ScreenHeight {
		t.Fatalf("restored Layout = %dx%d, want %dx%d", w, h, ScreenWidth, ScreenHeight)
	}
}

func TestElapsed(t *testing.T) {
	start := Ticks()
	time.Sleep(15 * time.Millisecond)
	if e := Elapsed(start); e < 10 {
		t.Errorf("Elapsed = %d ms, want >= 10", e)
	}
	if Elapsed(Ticks()+10000) != 0 {
		t.Error("future stamp should give 0")
	}
}

func TestScaleBright(t *testing.T) {
	cases := []struct {
		c    uint8
		f    float64
		want uint8
	}{
		{100, 1.0, 100}, {100, 0.7, 70}, {100, 1.3, 130}, {200, 1.6, 255}, // clamps
	}
	for _, c := range cases {
		if got := scaleBright(c.c, c.f); got != c.want {
			t.Errorf("scaleBright(%d,%v) = %d, want %d", c.c, c.f, got, c.want)
		}
	}
}

func TestBrightnessCycle(t *testing.T) {
	p := New(Config{})
	if p.bright != brightnessNormal {
		t.Fatalf("default brightness = %d, want %d", p.bright, brightnessNormal)
	}
	p.rawLUT[5] = [3]uint8{100, 50, 200}
	p.mu.Lock()
	p.rebuildLUT()
	p.mu.Unlock()
	if got := p.lut[5]; got != ([4]uint8{100, 50, 200, 0xFF}) {
		t.Fatalf("normal lut = %v, want unchanged", got)
	}
	// Normal (3) -> bright (4, 1.3x): 200*1.3=260 clamps to 255.
	if m := p.cycleBrightness(); m != 4 {
		t.Fatalf("cycle -> %d, want 4", m)
	}
	if got := p.lut[5]; got != ([4]uint8{130, 65, 255, 0xFF}) {
		t.Fatalf("bright lut = %v, want {130 65 255 255}", got)
	}
	// Wrap 5 -> 1.
	p.bright = 5
	if m := p.cycleBrightness(); m != 1 {
		t.Fatalf("cycle from 5 -> %d, want 1", m)
	}
}
