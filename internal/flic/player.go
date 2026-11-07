// player.go : draws decoded FLI and FLC frames into game buffers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package flic

import (
	"errors"
	"fmt"
	"io"

	"github.com/suglasp/max-go/internal/gfx"
)

// Player streams a FLIC animation into an 8-bit destination rectangle.
type Player struct {
	r    io.ReadSeeker
	anim *Animation

	Rect gfx.Rect
	Loop bool
}

// NewPlayer opens a FLIC stream for repeated DrawNext calls.
//
// C++: flicsmgr_construct (flicsmgr.cpp)
func NewPlayer(r io.ReadSeeker, rect gfx.Rect, loop bool) (*Player, error) {
	if _, _, err := rectSize(rect); err != nil {
		return nil, err
	}
	p := &Player{r: r, Rect: rect, Loop: loop}
	if err := p.reset(); err != nil {
		return nil, err
	}
	return p, nil
}

// DrawNext decodes the next frame and draws it into dst. When Loop is false,
// io.EOF is returned after the final frame. When Loop is true, EOF rewinds the
// stream and draws the first animation frame again.
//
// C++: flicsmgr_advance_animation (flicsmgr.cpp)
func (p *Player) DrawNext(dst *gfx.Buffer) error {
	if p == nil || p.anim == nil {
		return errors.New("flic: nil player")
	}
	frame, err := p.anim.NextFrame()
	if errors.Is(err, io.EOF) && p.Loop {
		if err := p.reset(); err != nil {
			return err
		}
		frame, err = p.anim.NextFrame()
	}
	if err != nil {
		return err
	}
	return DrawFrame(dst, p.Rect, frame)
}

// reset rewinds the animation and prepares the first frame for playback.
func (p *Player) reset() error {
	anim, err := Open(p.r)
	if err != nil {
		return err
	}
	p.anim = anim
	return nil
}

// DrawFrame draws a decoded frame into dst at rect. If rect is not the frame's
// native size the pixels are scaled using nearest neighbor sampling by gfx.Buffer.Scale.
func DrawFrame(dst *gfx.Buffer, rect gfx.Rect, frame *Frame) error {
	if dst == nil {
		return errors.New("flic: nil destination")
	}
	if frame == nil {
		return errors.New("flic: nil frame")
	}
	w, h, err := rectSize(rect)
	if err != nil {
		return err
	}
	if frame.W <= 0 || frame.H <= 0 || len(frame.Pix) < frame.W*frame.H {
		return fmt.Errorf("flic: invalid frame geometry %dx%d len=%d", frame.W, frame.H, len(frame.Pix))
	}
	if rect.ULX < 0 || rect.ULY < 0 || rect.LRX >= dst.W || rect.LRY >= dst.H {
		return fmt.Errorf("flic: rect %+v outside destination %dx%d", rect, dst.W, dst.H)
	}

	src := gfx.FromPix(frame.Pix, frame.W, frame.H)
	view := dst.Sub(rect.ULX, rect.ULY, w, h)
	if w == frame.W && h == frame.H {
		view.Copy(src)
	} else {
		view.Scale(src)
	}
	return nil
}

// rectSize validates a destination rectangle and returns its dimensions.
func rectSize(r gfx.Rect) (int, int, error) {
	w := r.LRX - r.ULX + 1
	h := r.LRY - r.ULY + 1
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("flic: invalid rect %+v", r)
	}
	return w, h, nil
}
