// sprites_test.go : testing unit for sprites.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sprites

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

var (
	libOnce sync.Once
	lib     *resfile.Library
	libErr  error
)

func openLib(t *testing.T) *resfile.Library {
	t.Helper()
	dir := testutil.DataDir(t)
	libOnce.Do(func() { lib, libErr = resfile.OpenDir(dir) })
	if libErr != nil {
		t.Fatal(libErr)
	}
	return lib
}

func hashPix(pix []uint8) string {
	sum := sha256.Sum256(pix)
	return hex.EncodeToString(sum[:])
}

func TestDecodeSimpleSynthetic(t *testing.T) {
	// 3x2 image, hotspot (1, 1).
	data := []byte{3, 0, 2, 0, 1, 0, 1, 0, 9, 0, 2, 3, 4, 5}
	s, err := DecodeSimple(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.W != 3 || s.H != 2 || s.ULX != 1 || s.ULY != 1 {
		t.Fatalf("header = %+v", s)
	}
	if s.TransparentColor() != 9 {
		t.Errorf("TransparentColor = %d, want 9 (first pixel)", s.TransparentColor())
	}

	dst := gfx.New(4, 4)
	dst.Fill(7)
	s.Draw(dst, 1, 1, false) // hotspot puts the image at (0, 0)
	if dst.At(0, 0) != 9 || dst.At(2, 1) != 5 {
		t.Errorf("Draw opaque wrong: %v", dst.Pix)
	}
	if dst.At(3, 0) != 7 {
		t.Error("Draw wrote outside image area")
	}

	dst = gfx.New(4, 4)
	dst.Fill(7)
	s.Draw(dst, 1, 1, true)
	if dst.At(1, 0) != 7 { // pixel value 0 is transparent
		t.Error("Draw transparent copied color 0")
	}

	// Exercise clipping against each edge without allowing an out-of-bounds write.
	for _, p := range [][2]int{{-2, -2}, {5, 5}, {-2, 3}, {3, -2}} {
		s.Draw(dst, p[0], p[1], false)
	}
	// Fully off screen.
	s.Draw(dst, -10, -10, false)

	if _, err := DecodeSimple(data[:10]); err == nil {
		t.Error("truncated pixel data should fail")
	}
	if _, err := DecodeSimple(data[:4]); err == nil {
		t.Error("truncated header should fail")
	}
}

func TestDecodeBigSynthetic(t *testing.T) {
	// 4x2 image: row 0 = literal [1 2 3 4], row 1 = repeat 5 x4.
	data := make([]byte, 776)
	data[4] = 4   // width
	data[6] = 2   // height
	data[8] = 252 // palette entry 0 red, 8-bit
	stream := []byte{
		4, 0, 1, 2, 3, 4, // literal run of 4
		0xFC, 0xFF, 5, // -4: repeat 5
	}
	b, err := DecodeBig(append(data, stream...))
	if err != nil {
		t.Fatal(err)
	}
	want := []uint8{1, 2, 3, 4, 5, 5, 5, 5}
	for i, c := range want {
		if b.Pix[i] != c {
			t.Fatalf("Pix = %v, want %v", b.Pix, want)
		}
	}
	if r, _, _ := b.Palette.RGB(0); r != 63 {
		t.Errorf("palette not converted to 6-bit: r=%d, want 63", r)
	}

	// Error cases.
	if _, err := DecodeBig(append(data, 0, 0)); err == nil {
		t.Error("zero-length run should fail")
	}
	if _, err := DecodeBig(append(data, 8, 0, 1, 2)); err == nil {
		t.Error("literal overflow should fail")
	}
	if _, err := DecodeBig(data[:500]); err == nil {
		t.Error("truncated header should fail")
	}
}

func TestDecodeMainPic(t *testing.T) {
	l := openLib(t)
	data, err := l.Read("MAINPIC")
	if err != nil {
		t.Fatal(err)
	}
	img, err := DecodeBig(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.W != 640 || img.H != 480 {
		t.Errorf("MAINPIC is %dx%d, want 640x480", img.W, img.H)
	}
	// These golden values come from the retail 1.0.4 resources.
	if h := hashPix(img.Pix); h != "6daadc50965e1ed99a4148ccca02a833d258a42a1cb3027626acb786639dbea8" {
		t.Errorf("MAINPIC pixel hash = %s", h)
	}
	if h := hashPix(img.Palette[:]); h != "1f0fd13b756e2097d3e8545dbc6415b7f88ba765acdd3cc99f8c93b3c17f4282" {
		t.Errorf("MAINPIC palette hash = %s", h)
	}
}

func TestDecodeLogoFromPatches(t *testing.T) {
	l := openLib(t)
	data, err := l.Read("ILOGO") // PATCHES.RES override
	if err != nil {
		t.Fatal(err)
	}
	img, err := DecodeBig(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.W != 640 || img.H != 480 {
		t.Errorf("ILOGO is %dx%d, want 640x480", img.W, img.H)
	}
	// The PATCHES.RES hash confirms that the override takes precedence.
	if h := hashPix(img.Pix); h != "bc840f3347aaceafd5f7b2f93f848cc1eefb0c4be6b86d635d9e808b80179f58" {
		t.Errorf("ILOGO pixel hash = %s", h)
	}
}

func TestDecodeSimpleButtons(t *testing.T) {
	l := openLib(t)
	goldens := map[string]string{
		"BLANK_UP": "49a417913886420dec72aa0079f4122b314b0cea636f770bd6a295aa269efed4",
		"BLANK_DN": "622eb5569ff7758ee7c48c5ff58f986f1c1c0d6f939cdd57b59629938be4b760",
	}
	for tag, want := range goldens {
		data, err := l.Read(tag)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		s, err := DecodeSimple(data)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if s.W != 200 || s.H != 29 {
			t.Errorf("%s is %dx%d, want 200x29", tag, s.W, s.H)
		}
		// The resource contains exactly the header and pixel payload, confirming the decoded layout.
		if len(data) != 8+s.W*s.H {
			t.Errorf("%s: resource %dB != header+pixels %dB", tag, len(data), 8+s.W*s.H)
		}
		if h := hashPix(s.Pix); h != want {
			t.Errorf("%s pixel hash = %s, want %s", tag, h, want)
		}
	}
}
