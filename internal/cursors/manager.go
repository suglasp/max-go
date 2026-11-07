// manager.go : tracks the active cursor and its animation frame.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package cursors

import "github.com/suglasp/max-go/internal/gfx"

// FrameDurationMS is the time each frame stays up for animated cursors.
//
// C++: HardwareCursor.frame_duration default (cursor.cpp), 200ms.
const FrameDurationMS = 200

// Manager tracks the selected cursor and its animation frame. The host
// switches cursors with Set (driven by whatever is under the pointer),
// advances animation with Tick, and reads Frame/Hotspot/Transparent when
// it draws: typically feeding them straight into the gnw software cursor:
//
//	m.Set(cursors.UnitGo)              // pointer is over passable terrain
//	m.Tick(elapsedMS)                  // once per frame
//	if buf := m.Frame(); buf != nil {  // on redraw
//	    hx, hy := m.Hotspot()
//	    sys.SetCursor(buf, hx, hy, m.Transparent())
//	}
//
// C++: the Cursor_ActiveCursorIndex / Cursor_AnimationTick state in
// cursor.cpp, minus the SDL hardware cursor cache.
type Manager struct {
	cat     *Catalog
	current ID
	frame   int
	accMS   int
}

// NewManager returns a manager over cat with the Hand cursor selected.
func NewManager(cat *Catalog) *Manager {
	return &Manager{cat: cat, current: Hand}
}

// Set selects cursor id. Changing the selection resets the animation to
// the first frame (matching Cursor_SetCursor); selecting the same id does
// nothing so an animation in progress keeps running.
func (m *Manager) Set(id ID) {
	if id == m.current {
		return
	}
	m.current = id
	m.frame = 0
	m.accMS = 0
}

// Current returns the selected cursor ID.
func (m *Manager) Current() ID {
	return m.current
}

/* // note: unused code in Go port
// no caller reads the selected Cursor; the screen draws through Frame.
// Cursor returns the selected Cursor, or nil when it did not load.
func (m *Manager) Cursor() *Cursor {
	return m.cat.Get(m.current)
}
*/

// FrameIndex returns the current animation frame index.
func (m *Manager) FrameIndex() int {
	return m.frame
}

// Tick advances the animation by elapsedMS milliseconds. Static cursors
// and missing cursors are unaffected.
//
// C++: Cursor_AnimationTick (cursor.cpp).
func (m *Manager) Tick(elapsedMS int) {
	c := m.cat.Get(m.current)
	if c == nil || !c.Animated() || elapsedMS <= 0 {
		return
	}
	m.accMS += elapsedMS
	for m.accMS >= FrameDurationMS {
		m.accMS -= FrameDurationMS
		m.frame = (m.frame + 1) % c.FrameCount
	}
}

// Frame returns the current frame of the selected cursor as a buffer ready
// to draw, or nil when the cursor did not load.
func (m *Manager) Frame() *gfx.Buffer {
	c := m.cat.Get(m.current)
	if c == nil {
		return nil
	}
	return c.Frame(m.frame)
}

// Hotspot returns the selected cursor's click point, or (0, 0) when it did
// not load.
func (m *Manager) Hotspot() (x, y int) {
	if c := m.cat.Get(m.current); c != nil {
		return c.HotX, c.HotY
	}
	return 0, 0
}

// Transparent returns the selected cursor's transparent colour index, or 0
// when it did not load.
func (m *Manager) Transparent() uint8 {
	if c := m.cat.Get(m.current); c != nil {
		return c.Transparent
	}
	return 0
}
