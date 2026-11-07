// gnw.go : manages GNW windows, composition, and input.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : gnw.c, interface.c - GNW window management, input, and screen composition
//
// Package gnw manages z-ordered windows with private 8-bit buffers, a blocking input pump for the game-logic goroutine, and a software cursor.
// The compositor redraws every visible window back to front each frame instead of splitting rectangles around occluded regions.
package gnw

import (
	"fmt"
	"sync"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
)

// WinID identifies a window; 0 is invalid, window IDs start at 1.
type WinID int

// Window flags (interface.h).
const (
	WindowDontMoveTop = 0x02
	WindowMoveOnTop   = 0x04
	WindowModal       = 0x10
	WindowTransparent = 0x20
	/* // note: ported but unused code in Go port
	// no window asks for the defaults, and only the commented out Hide set WindowHidden.
	WindowUseDefaults = 0x01
	WindowHidden      = 0x08
	*/
)

// Window is one GNW window: a screen rectangle backed by a private
// 8 bit buffer.
//
// C++: struct GNW_Window_s (gnw.h)
type Window struct {
	id    WinID
	flags uint32
	rect  gfx.Rect // screen position, corners inclusive
	buf   *gfx.Buffer
	color uint8
}

/* // note: ported but unused code in Go port
// no caller asks a window for its id; a *Window never leaves the package.
// ID returns the window's id.
func (w *Window) ID() WinID {
	return w.id
}
*/

/* // note: ported but unused code in Go port
// callers read a window's rect through System.WinRect.
// Rect returns the window's screen rectangle (corners inclusive).
func (w *Window) Rect() gfx.Rect {
	return w.rect
}
*/

// System owns the window list and the screen they composite onto. All exported methods lock.
// Callers that work on a window buffer directly via Buf must wrap the access in Lock/Unlock because the compositor reads the buffers from the render goroutine.
// WColor maps GNW_wcolor from gnw.c: entries [1]/[2] are the bevel light/dark colors, and [3] is the text color.
type System struct {
	mu      sync.Mutex
	screen  *gfx.Buffer
	windows []*Window // z order, index 0 = bottom (screen window)
	nextID  WinID

	fonts *fonts.Set

	cursorVisible bool
	cursorImage   *gfx.Buffer
	cursorHotX    int
	cursorHotY    int
	cursorTrans   uint8

	buttons     []*Button
	nextButton  ButtonID
	hoverButton *Button
	clickButton *Button
	/* clickRight  bool: note: ported but unused code in Go port, no right button */
	soundFunc func(ButtonID, bool)

	WColor [6]uint8 // generated widget colors

	events chan int
	ticks  chan struct{}
}

// New creates the window system targeting the given screen buffer and
// creates window 1 covering the whole screen as the background
// (matching the screen window WindowManager_Init adds).
func New(screen *gfx.Buffer, fontSet *fonts.Set) *System {
	s := &System{
		screen:     screen,
		nextID:     1,
		nextButton: 1,
		fonts:      fontSet,
		// Usable defaults on the game palette: light/dark bevel grays
		// and white text; the game overrides them later.
		WColor: [6]uint8{0, 146, 148, 255, 0, 0},
		events: make(chan int, 128),
		ticks:  make(chan struct{}, 1),
	}
	s.cursorImage, s.cursorHotX, s.cursorHotY, s.cursorTrans = defaultCursor()
	s.AddWindow(0, 0, screen.W, screen.H, 0, WindowDontMoveTop)
	return s
}

// ScreenSize returns the current composite target dimensions: 640x480 in
// the menus, 854x480 while the wide game view is active.
func (s *System) ScreenSize() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.screen.W, s.screen.H
}

// Lock/Unlock guard direct window buffer access against the
// compositor.
func (s *System) Lock() {
	s.mu.Lock()
}

// Unlock releases the GNW system lock acquired with Lock.
func (s *System) Unlock() {
	s.mu.Unlock()
}

// Fonts returns the font set used by Print.
func (s *System) Fonts() *fonts.Set {
	return s.fonts
}

// find returns the window with id, or nil when it is not registered.
func (s *System) find(id WinID) *Window {
	for _, w := range s.windows {
		if w.id == id {
			return w
		}
	}
	return nil
}

// AddWindow creates a window at (ulx, uly) of the given size, filled
// with color. Unless flags contains WindowMoveOnTop, it is inserted
// below the topmost stratum of WindowMoveOnTop windows.
//
// C++: win_add (gnw.c)
func (s *System) AddWindow(ulx, uly, width, height int, color uint8, flags uint32) WinID {
	s.mu.Lock()
	defer s.mu.Unlock()

	if width <= 0 || height <= 0 || width > s.screen.W || height > s.screen.H {
		return -1
	}
	w := &Window{
		id:    s.nextID,
		flags: flags,
		buf:   gfx.New(width, height),
		color: color,
	}
	s.nextID++
	w.buf.Fill(color)
	w.rect = clampRect(ulx, uly, width, height, s.screen)

	pos := len(s.windows)
	if flags&WindowMoveOnTop == 0 {
		for pos > 0 && s.windows[pos-1].flags&WindowMoveOnTop != 0 {
			pos--
		}
	}
	s.windows = append(s.windows, nil)
	copy(s.windows[pos+1:], s.windows[pos:])
	s.windows[pos] = w
	return w.id
}

// clampRect keeps a window fully on screen, as win_move does.
//
// C++: win_move (gnw.c)
func clampRect(ulx, uly, width, height int, screen *gfx.Buffer) gfx.Rect {
	if ulx < 0 {
		ulx = 0
	}
	if uly < 0 {
		uly = 0
	}
	if ulx+width > screen.W {
		ulx = screen.W - width
	}
	if uly+height > screen.H {
		uly = screen.H - height
	}
	return gfx.NewRect(ulx, uly, ulx+width-1, uly+height-1)
}

// DeleteWindow removes a window and its buttons.
//
// C++: win_delete (gnw.c)
func (s *System) DeleteWindow(id WinID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.windows {
		if w.id == id {
			s.windows = append(s.windows[:i], s.windows[i+1:]...)
			kept := s.buttons[:0]
			for _, b := range s.buttons {
				if b.win != w {
					kept = append(kept, b)
				} else {
					if s.hoverButton == b {
						s.hoverButton = nil
					}
					if s.clickButton == b {
						s.clickButton = nil
					}
				}
			}
			s.buttons = kept
			return
		}
	}
}

// Buf returns the window's buffer, or nil. Hold Lock/Unlock while
// drawing into it.
//
// C++: win_get_buf (gnw.c)
func (s *System) Buf(id WinID) *gfx.Buffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		return w.buf
	}
	return nil
}

// WinRect returns the window's screen rectangle.
//
// C++: win_get_rect (gnw.c)
func (s *System) WinRect(id WinID) (gfx.Rect, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		return w.rect, true
	}
	return gfx.Rect{}, false
}

/* // note: ported but unused code in Go port
// nothing shows a hidden window; the reference calls win_show only from win_add.
// Show makes a window visible and raises it to the top of its
// stratum unless it has WindowDontMoveTop.
//
// C++: win_show (gnw.c)
func (s *System) Show(id WinID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.find(id)
	if w == nil {
		return
	}
	w.flags &^= WindowHidden
	if w.flags&WindowDontMoveTop != 0 {
		return
	}
	s.raise(w)
}
*/

/* // note: ported but unused code in Go port
// raise was only called by Show.
// raise moves a window to the front of the z order.
func (s *System) raise(w *Window) {
	for i, win := range s.windows {
		if win == w {
			s.windows = append(s.windows[:i], s.windows[i+1:]...)
			break
		}
	}
	pos := len(s.windows)
	if w.flags&WindowMoveOnTop == 0 {
		for pos > 0 && s.windows[pos-1].flags&WindowMoveOnTop != 0 {
			pos--
		}
	}
	s.windows = append(s.windows, nil)
	copy(s.windows[pos+1:], s.windows[pos:])
	s.windows[pos] = w
}
*/

/* // note: ported but unused code in Go port
// nothing hides a window; the reference has no caller of win_hide outside gnw.c.
// Hide makes a window invisible.
//
// C++: win_hide (gnw.c)
func (s *System) Hide(id WinID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.flags |= WindowHidden
	}
}
*/

/* // note: ported but unused code in Go port
// nothing moves a window; the reference has no caller of win_move outside gnw.c.
// Move repositions a window, keeping it fully on screen.
//
// C++: win_move (gnw.c)
func (s *System) Move(id WinID, ulx, uly int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.rect = clampRect(ulx, uly, w.buf.W, w.buf.H, s.screen)
	}
}
*/

/* // note: unused code in Go port
// a no op stub with no caller: Composite redraws every frame.
// Draw is a compatibility no-op because Composite redraws every visible window each frame. It preserves win_draw call sites in ported code.
//
// C++: win_draw (gnw.c)
func (s *System) Draw(id WinID) {}
*/

/* // note: unused code in Go port
// a no op stub with no caller: Composite redraws every frame.
// DrawRect is Draw for a sub rectangle; equally a no op.
//
// C++: win_draw_rect (gnw.c)
func (s *System) DrawRect(id WinID, r gfx.Rect) {}
*/

// Fill fills a region of the window with color.
//
// C++: win_fill (gnw.c)
func (s *System) Fill(id WinID, ulx, uly, width, height int, color uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.buf.Sub(ulx, uly, width, height).Fill(color)
	}
}

/* // note: ported but unused code in Go port
// no window draws a line; the reference has no caller of win_line either.
// Line draws a line in window coordinates.
//
// C++: win_line (gnw.c)
func (s *System) Line(id WinID, x1, y1, x2, y2 int, color uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.buf.DrawLine(x1, y1, x2, y2, color)
	}
}
*/

// Box draws a hollow box in window coordinates.
//
// C++: win_box (gnw.c)
func (s *System) Box(id WinID, ulx, uly, lrx, lry int, color uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.buf.DrawBox(ulx, uly, lrx, lry, color)
	}
}

// ShadedBox draws a beveled box in window coordinates.
//
// C++: win_shaded_box (gnw.c)
func (s *System) ShadedBox(id WinID, ulx, uly, lrx, lry int, c1, c2 uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.find(id); w != nil {
		w.buf.DrawShadedBox(ulx, uly, lrx, lry, c1, c2)
	}
}

// Print renders text into the window at (x, y) with the current font.
// fieldWidth limits the text width (0 = window width).
//
// C++: win_print (gnw.c)
func (s *System) Print(id WinID, str string, fieldWidth, x, y int, color int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.find(id)
	if w == nil {
		return fmt.Errorf("gnw: no window %d", id)
	}
	f := s.fonts.Font()
	if y+f.Height > w.buf.H || x >= w.buf.W {
		return fmt.Errorf("gnw: text outside window %d", id)
	}
	view := w.buf.Sub(x, y, w.buf.W-x, f.Height)
	f.Blit(view, str, fieldWidth, color)
	return nil
}

// Composite paints all visible windows back to front onto the screen
// buffer, then the mouse cursor at (mx, my).
//
// C++: refresh_all + GNW_win_refresh (gnw.c), simplified
func (s *System) Composite(mx, my int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range s.windows {
		/* // note: ported but unused code in Go port
		// a hidden window was skipped; nothing hides a window.
		if w.flags&WindowHidden != 0 {
			continue
		}
		*/
		clipped, ok := w.rect.Intersect(gfx.NewRect(0, 0, s.screen.W-1, s.screen.H-1))
		if !ok {
			continue
		}
		src := w.buf.Sub(clipped.ULX-w.rect.ULX, clipped.ULY-w.rect.ULY,
			clipped.Width()+1, clipped.Height()+1)
		dst := s.screen.SubRect(clipped)
		if w.flags&WindowTransparent != 0 {
			dst.TransCopy(src)
		} else {
			dst.Copy(src)
		}
	}

	if s.cursorVisible {
		s.drawCursor(mx, my)
	}
}
