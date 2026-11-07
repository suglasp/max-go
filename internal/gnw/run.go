// run.go : connects GNW logic and input to the platform loop.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"errors"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// The threading model (PORTING.md D14) runs game logic and menu loops in a dedicated goroutine, preserving the original code's blocking loops.
// Each platform frame
// drains the keyboard queue into a channel and signals a tick; the
// logic goroutine's GetInput blocks until input arrives or the next
// frame begins, returning -1 on an idle frame exactly like GNW's
// get_input pump.

// ErrQuit ends the logic goroutine and closes the game cleanly.
var ErrQuit = errors.New("gnw: quit")

// GetInput returns the next queued input code, blocking until the
// next frame tick when the queue is empty (then -1). The mouse is
// polled separately via Mouse.
//
// C++: get_input (input.cpp)
func (s *System) GetInput() int {
	select {
	case code := <-s.events:
		return code
	case <-s.ticks:
		return -1
	}
}

/* // note: unused code in Go port
// GetInputNonBlocking was only called by a test; the menus read input through GetInput.
// GetInputNonBlocking returns the next queued input code, or -1
// immediately when the queue is empty.
func (s *System) GetInputNonBlocking() int {
	select {
	case code := <-s.events:
		return code
	default:
		return -1
	}
}
*/

// FlushInput discards any queued input codes. Call it on a menu to menu
// transition so a confirm key (Return) or a stray click consumed by one screen
// cannot leak into the next screen and immediately trigger its default action.
//
// C++: the GNW_kb_clear / flushall input flush done between menu screens.
func (s *System) FlushInput() {
	for {
		select {
		case <-s.events:
		default:
			return
		}
	}
}

// pushEvent queues an input code for the logic goroutine, dropping it
// when the queue is full (GNW's buffer is fixed size too).
func (s *System) pushEvent(code int) {
	select {
	case s.events <- code:
	default:
	}
}

// ResizeScreen switches the logical resolution (640x480 menus <-> 854x480
// wide game view) and resizes the full screen background window (WinID 1)
// to match. Holding the system lock keeps the compositor out while the
// platform swaps the screen buffer underneath it; the caller repaints.
//
// C++: the GNW screen reinit on a mode change (gnw.c - win_reinit path)
func (s *System) ResizeScreen(plat *platform.Platform, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plat.SetLogicalSize(width, height) // resizes s.screen in place
	// windows[0] is the background window New created over the whole screen.
	// Its buffer resizes in place too, so callers that cached Buf(1) across
	// the switch keep a valid pointer; the content resets, the caller repaints.
	background := s.windows[0]
	background.buf.W = width
	background.buf.H = height
	background.buf.Stride = width
	background.buf.Pix = make([]uint8, width*height)
	background.buf.Fill(background.color)
	background.rect = gfx.NewRect(0, 0, width-1, height-1)
	// Keep every other window on the resized screen.
	for _, w := range s.windows[1:] {
		w.rect = clampRect(w.rect.ULX, w.rect.ULY, w.buf.W, w.buf.H, s.screen)
	}
}

// Run wires the system to the platform and starts logic in its own
// goroutine, then starts the platform loop. It returns when logic
// returns (ErrQuit and nil both quit cleanly) or the window is closed.
func (s *System) Run(plat *platform.Platform, logic func() error) error {
	s.Attach(plat, logic)
	return plat.Run()
}

// Attach wires the system to the platform and starts logic in its own
// goroutine WITHOUT starting the platform loop. Use it when the loop is
// already running (the data bootstrap phase hands over to the menu this
// way); in that case call it from inside the running frame hook, since
// swapping the hook is only safe from the update goroutine.
func (s *System) Attach(plat *platform.Platform, logic func() error) {
	done := make(chan error, 1)
	go func() { done <- logic() }()

	var mouse platform.MouseState

	plat.SetFrameHook(func(p *platform.Platform) error {
		select {
		case err := <-done:
			if err == nil || errors.Is(err, ErrQuit) {
				return ebiten.Termination
			}
			return err
		default:
		}

		// Drain this frame's keyboard input into the logic channel.
		for {
			code := p.GetInput()
			if code < 0 {
				break
			}
			s.pushEvent(code)
		}

		// Mouse interaction with buttons emits their input codes.
		mouse = p.Mouse()
		for _, code := range s.CheckButtons(mouse) {
			s.pushEvent(code)
		}

		// Wake the logic goroutine for an idle frame.
		select {
		case s.ticks <- struct{}{}:
		default:
		}

		s.Composite(mouse.X, mouse.Y)
		return nil
	})
}
