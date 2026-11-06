// intro_test.go : tests splash timing, palette fading, and movie skipping.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package intro

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

func TestLogoPathUsesFakeClock(t *testing.T) {
	stage := newFakeStage()
	lib := memReader{
		"MLOGO": syntheticBig(4, 2, []uint8{1, 2, 3, 4, 5, 5, 5, 5}),
	}
	err := Run(stage, lib, Options{ShowLogos: true, LogoHoldMS: 30})
	if err != nil {
		t.Fatal(err)
	}
	// The logo dissolves in via a palette ramp (many SetPalette steps over the
	// fade), then holds: so the palette is set far more than once and the fade
	// runs its full duration before the hold.
	if stage.palettes < 2 {
		t.Fatalf("palettes = %d, want a multi-step fade-in", stage.palettes)
	}
	if stage.now < logoFadeMS {
		t.Fatalf("fade ended at %d, want it to run the full %d ms", stage.now, logoFadeMS)
	}
	// The ramp ends on the logo's real palette: syntheticBig writes 252, which
	// DecodeBig scales to 6-bit (252/4 = 63).
	if stage.lastPal[0] != 63 {
		t.Fatalf("final palette entry 0 = %d, want 63 (target palette)", stage.lastPal[0])
	}
	if stage.frame.At((640-4)/2, (480-2)/2) != 1 {
		t.Fatal("logo was not centered into the frame")
	}
}

func TestSkipReturnsPromptly(t *testing.T) {
	stage := newFakeStage()
	stage.skipAfter = 1
	lib := memReader{
		"MLOGO": syntheticBig(4, 2, []uint8{1, 2, 3, 4, 5, 5, 5, 5}),
	}
	err := Run(stage, lib, Options{ShowLogos: true, LogoHoldMS: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if stage.presents != 0 {
		t.Fatalf("presents = %d, want 0 for immediate skip", stage.presents)
	}
}

func TestMissingResourceReturnsError(t *testing.T) {
	err := Run(newFakeStage(), memReader{}, Options{ShowLogos: true})
	if err == nil {
		t.Fatal("missing MLOGO returned nil error")
	}
}

func TestRealIntroMovieWhenDataAvailable(t *testing.T) {
	dir := testutil.DataDir(t)
	stage := newFakeStage()
	stage.presentStep = 1000
	err := Run(stage, dataReader{dir: dir, t: t}, Options{PlayMovie: true})
	if err != nil {
		t.Fatal(err)
	}
	if stage.presents == 0 {
		t.Fatal("movie presented zero frames")
	}
	if stage.audio == 0 {
		t.Fatal("movie queued no audio")
	}
}

type fakeStage struct {
	frame       *gfx.Buffer
	now         uint64
	presentStep uint64
	presents    int
	palettes    int
	lastPal     gfx.Palette
	audio       int
	skipAfter   int
	skipCalls   int
}

func newFakeStage() *fakeStage {
	return &fakeStage{frame: gfx.New(640, 480), presentStep: 10}
}

func (s *fakeStage) Frame() *gfx.Buffer { return s.frame }

func (s *fakeStage) Present() {
	s.presents++
	s.now += s.presentStep
}

func (s *fakeStage) SetPalette(p *gfx.Palette) {
	if p != nil {
		s.palettes++
		s.lastPal = *p
	}
}

func (s *fakeStage) QueueAudio(pcm []int16) { s.audio += len(pcm) }

func (s *fakeStage) Skipped() bool {
	s.skipCalls++
	return s.skipAfter > 0 && s.skipCalls >= s.skipAfter
}

func (s *fakeStage) NowMS() uint64 { return s.now }

func (s *fakeStage) ConsumeSkip() { s.skipAfter, s.skipCalls = 0, 0 }

type memReader map[string][]byte

func (m memReader) Read(tag string) ([]byte, error) {
	if data, ok := m[tag]; ok {
		return append([]byte(nil), data...), nil
	}
	return nil, errors.New("not found")
}

type dataReader struct {
	dir string
	t   *testing.T
}

func (r dataReader) Read(tag string) ([]byte, error) {
	switch tag {
	case "MAXINT", "MAXINT.MVE", "INTROFLC":
		return os.ReadFile(filepath.Join(r.dir, "MAXINT.MVE"))
	default:
		lib, err := resfile.OpenDir(r.dir)
		if err != nil {
			return nil, err
		}
		defer lib.Close()
		return lib.Read(tag)
	}
}

func syntheticBig(w, h int, pix []uint8) []byte {
	data := make([]byte, 8+gfx.PaletteSize*gfx.PaletteStride)
	binary.LittleEndian.PutUint16(data[4:6], uint16(w))
	binary.LittleEndian.PutUint16(data[6:8], uint16(h))
	data[8] = 252
	for y := 0; y < h; y++ {
		row := pix[y*w : (y+1)*w]
		data = append(data, byte(w), 0)
		data = append(data, row...)
	}
	return data
}
