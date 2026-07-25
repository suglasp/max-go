// dialog_test.go : tests dialog layout, input, wrapping, and rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package dialog

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
)

func TestLayout(t *testing.T) {
	art := testArt()

	ok := New(art, nil, ModeOK, "", "OK", "")
	ok.SetOrigin(game.Point{X: 10, Y: 20})
	/* // note: ported but unused code in Go port
	if got, want := ok.Bounds(), gfx.NewRect(10, 20, 314, 244); got != want {
		t.Fatalf("OK Bounds = %+v, want %+v", got, want)
	}
	*/
	if got, want := ok.acceptRect(), gfx.NewRect(130, 213, 179, 232); got != want {
		t.Fatalf("OK accept rect = %+v, want %+v", got, want)
	}

	oc := New(art, nil, ModeOKCancel, "", "OK", "Cancel")
	oc.SetOrigin(game.Point{X: 10, Y: 20})
	if got, want := oc.acceptRect(), gfx.NewRect(165, 213, 214, 232); got != want {
		t.Fatalf("OKCancel accept rect = %+v, want %+v", got, want)
	}
	if got, want := oc.cancelRect(), gfx.NewRect(95, 213, 144, 232); got != want {
		t.Fatalf("OKCancel cancel rect = %+v, want %+v", got, want)
	}
}

func TestInputResolution(t *testing.T) {
	art := testArt()

	ok := New(art, nil, ModeOK, "", "OK", "")
	if got := ok.HandleKey(keyReturn); got != ResultAccept {
		t.Fatalf("OK Return = %d, want accept", got)
	}
	if got := ok.HandleKey(keyEscape); got != ResultAccept {
		t.Fatalf("OK Escape = %d, want accept", got)
	}

	oc := New(art, nil, ModeOKCancel, "", "OK", "Cancel")
	if got := oc.HandleKey(keyReturn); got != ResultAccept {
		t.Fatalf("OKCancel Return = %d, want accept", got)
	}
	if got := oc.HandleKey(keyEscape); got != ResultCancel {
		t.Fatalf("OKCancel Escape = %d, want cancel", got)
	}

	/* // note: ported but unused code in Go port
	r := oc.acceptRect()
	if got := oc.HandleMouse(r.ULX+1, r.ULY+1, true); got != ResultNone {
		t.Fatalf("accept press = %d, want none", got)
	}
	if got := oc.HandleMouse(r.ULX+1, r.ULY+1, false); got != ResultAccept {
		t.Fatalf("accept release = %d, want accept", got)
	}

	r = oc.cancelRect()
	oc.HandleMouse(r.ULX+1, r.ULY+1, true)
	if got := oc.HandleMouse(r.ULX+1, r.ULY+1, false); got != ResultCancel {
		t.Fatalf("cancel release = %d, want cancel", got)
	}

	r = oc.acceptRect()
	oc.HandleMouse(r.ULX+1, r.ULY+1, true)
	if got := oc.HandleMouse(0, 0, false); got != ResultNone {
		t.Fatalf("release outside = %d, want none", got)
	}
	*/
}

func TestWordWrapFont5(t *testing.T) {
	font := loadFont5(t)
	lines := wrapLines(font, "Alpha beta gamma delta epsilon zeta eta theta iota kappa lambda", captionW, 0)
	if got, want := len(lines), 2; got != want {
		t.Fatalf("wrapped rows = %d, want %d (%q)", got, want, lines)
	}
}

func TestDrawGoldens(t *testing.T) {
	font := loadFont5(t)
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{"ok", ModeOK, "60132834c0ddbedbbf2e978a2ef55f5cd37911e99802a690a6054309213233ad"},
		{"okcancel", ModeOKCancel, "d3bc1257db6a758d16e413a2be04177644fe7462c8b7f276f262907b8f7dfd44"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(testArt(), font, tt.mode, "Network unsupported", "OK", "Cancel")
			dst := gfx.New(screenW, screenH)
			d.Draw(dst)
			sum := sha256.Sum256(dst.Pix)
			if got := fmt.Sprintf("%x", sum); got != tt.want {
				t.Fatalf("draw hash = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestDrawIntoFrameBufferNoPanic checks a short centered OK/Cancel caption in a
// buffer exactly the size of the dialog frame. The text blit must remain within
// the right edge accepted by Buffer.Sub.
func TestDrawIntoFrameBufferNoPanic(t *testing.T) {
	font := loadFont5(t)
	art := testArt()
	for _, mode := range []Mode{ModeOK, ModeOKCancel} {
		d := New(art, font, mode, "OK to exit game?", "OK", "Cancel")
		d.SetOrigin(game.Point{}) // modalDialog draws at (0,0) into a window sized to the frame
		dst := gfx.New(art.Frame.W, art.Frame.H)
		d.Draw(dst) // must not panic
	}
}

func testArt() Art {
	return Art{
		Frame:          solid(305, 225, 11),
		AcceptUp:       solid(50, 20, 21),
		AcceptDown:     solid(50, 20, 22),
		CancelUp:       solid(50, 20, 31),
		CancelDown:     solid(50, 20, 32),
		ScrollUpUp:     solid(16, 16, 41),
		ScrollUpDown:   solid(16, 16, 42),
		ScrollDownUp:   solid(16, 16, 51),
		ScrollDownDown: solid(16, 16, 52),
	}
}

func solid(w, h int, color uint8) *gfx.Buffer {
	b := gfx.New(w, h)
	b.Fill(color)
	return b
}

func loadFont5(t *testing.T) *fonts.Font {
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
