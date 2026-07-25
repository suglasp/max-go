// button.go : implements GNW buttons, sticky states, and groups.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : button.c - GNW button creation, input handling, and rendering
package gnw

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// fontsOutline aliases the text outline flag for the generated text
// button labels.
const fontsOutline = fonts.FlagOutline

// Buttons, ported from button.c. A button is a window relative
// rectangle with up/down/hover images; mouse interaction emits the
// button's input codes into the same queue GetInput reads, exactly
// like GNW delivering button presses through get_input.

// ButtonID identifies a button across all windows.
type ButtonID int

// Button flags (raw hex in button.c).
const (
	ButtonSticky      = 0x01 // stays down when clicked (checkbox/toggle)
	ButtonPressToggle = 0x02 // sticky toggles on press instead of release
	ButtonNoUncheck   = 0x04 // clicking a checked button does not uncheck it (radio)
	buttonDisabled    = 0x08
	ButtonTransparent = 0x10000 // images blit with color 0 transparency
	buttonChecked     = 0x020000
	ButtonRightActive = 0x080000 // also reacts to the right mouse button
)

// NoValue marks an unused input code slot.
const NoValue = -1

// Button is one GNW button.
//
// C++: struct GNW_buttondata (button.h)
type Button struct {
	id   ButtonID
	win  *Window
	rect gfx.Rect // window relative

	onValue, offValue int // hover enter/leave
	pValue, rValue    int // left press/release
	rpValue, rrValue  int // right press/release

	up, down, hover          *gfx.Buffer
	disUp, disDown, disHover *gfx.Buffer

	flags uint32
	group *ButtonGroup
}

// ButtonGroup limits how many buttons of a set may be checked at
// once; radio groups have a maximum of one.
//
// C++: struct GNW_ButtonGroup_s (button.h)
type ButtonGroup struct {
	maxChecked  int
	currChecked int
	buttons     []*Button
	onChange    func(ButtonID)
}

// Checked reports whether a sticky button is currently down.
func (b *Button) Checked() bool {
	return b.flags&buttonChecked != 0
}

// ID returns the button id.
func (b *Button) ID() ButtonID {
	return b.id
}

// AddButton registers a button on a window. Image buffers may be nil
// (down/hover default to up). Values are input codes emitted on the
// respective events, NoValue for none.
//
// C++: win_register_button (button.c)
func (s *System) AddButton(win WinID, ulx, uly, width, height int,
	onValue, offValue, pValue, rValue int,
	up, down, hover *gfx.Buffer, flags uint32) ButtonID {

	s.mu.Lock()
	defer s.mu.Unlock()

	w := s.find(win)
	if w == nil {
		return -1
	}
	b := &Button{
		id:       s.nextButton,
		win:      w,
		rect:     gfx.NewRect(ulx, uly, ulx+width-1, uly+height-1),
		onValue:  onValue,
		offValue: offValue,
		pValue:   pValue,
		rValue:   rValue,
		rpValue:  NoValue,
		rrValue:  NoValue,
		up:       up,
		down:     down,
		hover:    hover,
		flags:    flags | ButtonTransparent,
	}
	s.nextButton++
	s.buttons = append(s.buttons, b)
	s.drawButton(b, b.up)
	return b.id
}

// AddTextButton builds a classic GNW beveled text button and
// registers it.
//
// C++: win_register_text_button (button.c)
func (s *System) AddTextButton(win WinID, ulx, uly int,
	onValue, offValue, pValue, rValue int, label string, flags uint32) ButtonID {

	s.mu.Lock()
	w := s.find(win)
	if w == nil {
		s.mu.Unlock()
		return -1
	}
	f := s.fonts.Font()
	width := f.Width(label) + 16
	height := f.Height + 6

	up := gfx.New(width, height)
	down := gfx.New(width, height)
	up.Fill(w.color)
	down.Fill(w.color)

	textColor := int(s.WColor[3]) | fontsOutline
	f.Blit(up.Sub(8, 3, width-8, f.Height), label, width, textColor)
	up.DrawShadedBox(2, 2, width-3, height-3, s.WColor[1], s.WColor[2])
	up.DrawShadedBox(1, 1, width-2, height-2, s.WColor[1], s.WColor[2])
	up.DrawBox(0, 0, width-1, height-1, 0)

	f.Blit(down.Sub(9, 4, width-9, f.Height), label, width, textColor)
	down.DrawShadedBox(2, 2, width-3, height-3, s.WColor[2], s.WColor[1])
	down.DrawShadedBox(1, 1, width-2, height-2, s.WColor[2], s.WColor[1])
	down.DrawBox(0, 0, width-1, height-1, 0)
	s.mu.Unlock()

	return s.AddButton(win, ulx, uly, width, height,
		onValue, offValue, pValue, rValue, up, down, nil, flags&^ButtonTransparent)
}

// SetRightValues enables right mouse press/release codes.
//
// C++: win_register_right_button (button.c)
func (s *System) SetRightValues(bid ButtonID, rpValue, rrValue int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil {
		b.rpValue = rpValue
		b.rrValue = rrValue
		b.flags |= ButtonRightActive
	}
}

// SetDisabledImages installs the images used while disabled.
//
// C++: win_register_button_disable (button.c)
func (s *System) SetDisabledImages(bid ButtonID, up, down, hover *gfx.Buffer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil {
		b.disUp, b.disDown, b.disHover = up, down, hover
	}
}

// DeleteButton removes a button and restores nothing (the caller
// repaints if needed).
//
// C++: win_delete_button (button.c)
func (s *System) DeleteButton(bid ButtonID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.buttons {
		if b.id == bid {
			s.buttons = append(s.buttons[:i], s.buttons[i+1:]...)
			if s.hoverButton == b {
				s.hoverButton = nil
			}
			if s.clickButton == b {
				s.clickButton = nil
			}
			return
		}
	}
}

// EnableButton enables again a disabled button.
//
// C++: win_enable_button (button.c)
func (s *System) EnableButton(bid ButtonID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil && b.flags&buttonDisabled != 0 {
		b.flags &^= buttonDisabled
		s.drawButton(b, b.stateImage())
	}
}

// DisableButton makes a button inert (drawn with its disabled
// images when present).
//
// C++: win_disable_button (button.c)
func (s *System) DisableButton(bid ButtonID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil && b.flags&buttonDisabled == 0 {
		b.flags |= buttonDisabled
		s.drawButton(b, b.stateImage())
	}
}

// SetRestState forces a sticky button up or down without emitting
// events.
//
// C++: win_set_button_rest_state (button.c)
func (s *System) SetRestState(bid ButtonID, down bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.findButton(bid)
	if b == nil {
		return
	}
	was := b.flags&buttonChecked != 0
	if down == was {
		return
	}
	if down {
		b.flags |= buttonChecked
		if b.group != nil {
			b.group.currChecked++
		}
	} else {
		b.flags &^= buttonChecked
		if b.group != nil {
			b.group.currChecked--
		}
	}
	s.drawButton(b, b.stateImage())
}

// ButtonChecked reports a sticky button's state.
func (s *System) ButtonChecked(bid ButtonID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil {
		return b.Checked()
	}
	return false
}

// SetButtonToggle makes a sticky button uncheckable; a real toggle checkbox
// rather than a radio member. It clears the ButtonNoUncheck flag that the
// sticky button helpers default on, so a second click releases it.
// C++: the Description checkbox in AbstractUpgradeMenu.
func (s *System) SetButtonToggle(bid ButtonID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := s.findButton(bid); b != nil {
		b.flags &^= ButtonNoUncheck
	}
}

// GroupCheckButtons puts buttons into a group allowing at most
// maxChecked of them down at once.
//
// C++: win_group_check_buttons (button.c)
func (s *System) GroupCheckButtons(ids []ButtonID, maxChecked int, onChange func(ButtonID)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := &ButtonGroup{maxChecked: maxChecked, onChange: onChange}
	for _, id := range ids {
		if b := s.findButton(id); b != nil {
			b.group = g
			g.buttons = append(g.buttons, b)
			if b.Checked() {
				g.currChecked++
			}
		}
	}
}

// GroupRadioButtons makes the buttons a radio set: exactly one down,
// clicking the checked one does not release it.
//
// C++: win_group_radio_buttons (button.c)
func (s *System) GroupRadioButtons(ids []ButtonID, onChange func(ButtonID)) {
	s.mu.Lock()
	for _, id := range ids {
		if b := s.findButton(id); b != nil {
			b.flags |= ButtonSticky | ButtonNoUncheck
		}
	}
	s.mu.Unlock()
	s.GroupCheckButtons(ids, 1, onChange)
}

// PressAndRelease performs a programmatic click (used for keyboard
// shortcuts driving a button).
//
// C++: win_button_press_and_release (button.c)
func (s *System) PressAndRelease(bid ButtonID) {
	s.mu.Lock()
	b := s.findButton(bid)
	if b == nil || b.flags&buttonDisabled != 0 {
		s.mu.Unlock()
		return
	}
	s.drawButton(b, b.imageOr(b.down))
	emit := s.handleRelease(b, false)
	s.mu.Unlock()
	for _, code := range emit {
		s.pushEvent(code)
	}
}

// SetButtonSoundFunc installs a hook called on press (down=true) and
// release; the audio layer uses it for click sounds.
//
// C++: win_register_button_sound_func (button.c)
func (s *System) SetButtonSoundFunc(f func(b ButtonID, down bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.soundFunc = f
}

// findButton returns the button with bid from any window, if it exists.
func (s *System) findButton(bid ButtonID) *Button {
	for _, b := range s.buttons {
		if b.id == bid {
			return b
		}
	}
	return nil
}

// stateImage picks the image matching the button's current state.
func (b *Button) stateImage() *gfx.Buffer {
	disabled := b.flags&buttonDisabled != 0
	if b.Checked() {
		if disabled && b.disDown != nil {
			return b.disDown
		}
		return b.imageOr(b.down)
	}
	if disabled && b.disUp != nil {
		return b.disUp
	}
	return b.up
}

// imageOr returns img when present, otherwise the button's up image.
func (b *Button) imageOr(img *gfx.Buffer) *gfx.Buffer {
	if img != nil {
		return img
	}
	return b.up
}

// hoverImage is the image shown while the mouse hovers the button: the
// explicit hover image if any, otherwise the button's current rest state; so
// a checked sticky button (a set checkbox) stays drawn checked on hover instead
// of reverting to its unchecked "up" face.
func (b *Button) hoverImage() *gfx.Buffer {
	if b.hover != nil {
		return b.hover
	}
	return b.stateImage()
}

// drawButton paints img into the button's window region, then redraws overlapping buttons that come after it to preserve their z-order.
// This method refreshes one button rather than a whole RECT. Callers hold s.mu.
//
// C++ ported : button.c/gnw.c - button_draw + GNW_win_refresh.
func (s *System) drawButton(b *Button, img *gfx.Buffer) {
	s.blitButton(b, img)
	idx := -1
	for i, x := range s.buttons {
		if x == b {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	for j := idx + 1; j < len(s.buttons); j++ {
		o := s.buttons[j]
		if o.win == b.win {
			if _, ok := o.rect.Intersect(b.rect); ok {
				s.blitButton(o, o.stateImage()) // raw blit (no recursion)
			}
		}
	}
}

// blitButton paints img into b's window region with no overlap handling.
//
// C++: button_draw (button.c)
func (s *System) blitButton(b *Button, img *gfx.Buffer) {
	if img == nil {
		return
	}
	dstRect, ok := b.rect.Intersect(gfx.NewRect(0, 0, b.win.buf.W-1, b.win.buf.H-1))
	if !ok {
		return
	}
	src := img.Sub(dstRect.ULX-b.rect.ULX, dstRect.ULY-b.rect.ULY,
		min(img.W, dstRect.Width()+1), min(img.H, dstRect.Height()+1))
	dst := b.win.buf.Sub(dstRect.ULX, dstRect.ULY, src.W, src.H)
	if b.flags&ButtonTransparent != 0 {
		dst.TransCopy(src)
	} else {
		dst.Copy(src)
	}
}

// checkGroup enforces the group limit before checking a button:
// for radio groups (max 1) the currently checked sibling is released,
// otherwise exceeding the limit rejects the click.
//
// C++: button_check_group (button.c)
func (s *System) checkGroup(b *Button) bool {
	g := b.group
	if g == nil {
		return true
	}
	if g.currChecked < g.maxChecked {
		g.currChecked++
		return true
	}
	if g.maxChecked == 1 {
		for _, other := range g.buttons {
			if other != b && other.Checked() {
				other.flags &^= buttonChecked
				s.drawButton(other, other.stateImage())
				if g.onChange != nil {
					g.onChange(other.id)
				}
			}
		}
		return true
	}
	return false
}

// under reports whether the screen point lies on the button.
func (b *Button) under(x, y int) bool {
	return b.rect.Contains(x-b.win.rect.ULX, y-b.win.rect.ULY)
}

// topWindowAt returns the topmost visible window containing the
// screen point; callers hold s.mu.
func (s *System) topWindowAt(x, y int) *Window {
	for i := len(s.windows) - 1; i >= 0; i-- {
		w := s.windows[i]
		if w.flags&WindowHidden == 0 && w.rect.Contains(x, y) {
			return w
		}
	}
	return nil
}

// CheckButtons runs one frame of mouse interaction and returns the
// input codes to emit. Called from the frame hook with the current
// mouse state.
//
// C++: win_check_all_buttons + GNW_check_buttons (button.c)
func (s *System) CheckButtons(m platform.MouseState) []int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var emit []int
	push := func(code int) {
		if code != NoValue {
			emit = append(emit, code)
		}
	}

	topWin := s.topWindowAt(m.X, m.Y)

	// Locate the button under the mouse in the topmost window.
	var over *Button
	if topWin != nil {
		for _, b := range s.buttons {
			if b.win == topWin && b.flags&buttonDisabled == 0 && b.under(m.X, m.Y) {
				over = b
				break
			}
		}
	}

	// Hover transitions.
	if s.hoverButton != nil && s.hoverButton != over {
		h := s.hoverButton
		// Blit again on hover leave only when the button has a distinct hover image;
		// repainting the same rest face clobbers overlapping neighbours. C++:
		// button.c hover redraw only when button->hover is set.
		if s.clickButton != h && h.hover != nil { // keep a held button drawn down
			s.drawButton(h, h.stateImage())
		}
		push(h.offValue)
		s.hoverButton = nil
	}
	if over != nil && s.hoverButton == nil && s.clickButton == nil && m.Buttons == 0 {
		s.hoverButton = over
		if over.hover != nil { // a plain rest image redraw would clobber neighbours
			s.drawButton(over, over.hoverImage())
		}
		push(over.onValue)
	}

	left := m.Buttons&platform.MousePressLeft != 0
	right := m.Buttons&platform.MousePressRight != 0
	releasedLeft := m.Buttons&platform.MouseReleaseLeft != 0
	releasedRight := m.Buttons&platform.MouseReleaseRight != 0

	// Press.
	if over != nil && s.clickButton == nil {
		switch {
		case left:
			s.clickButton = over
			s.clickRight = false
			s.drawButton(over, over.imageOr(over.down))
			if s.soundFunc != nil {
				s.soundFunc(over.id, true)
			}
			push(over.pValue)
		case right && over.flags&ButtonRightActive != 0:
			s.clickButton = over
			s.clickRight = true
			s.drawButton(over, over.imageOr(over.down))
			push(over.rpValue)
		}
	}

	// Release.
	if s.clickButton != nil &&
		((!s.clickRight && releasedLeft) || (s.clickRight && releasedRight)) {
		b := s.clickButton
		s.clickButton = nil
		if b.under(m.X, m.Y) {
			emit = append(emit, s.handleRelease(b, s.clickRight)...)
		} else {
			s.drawButton(b, b.stateImage())
		}
	}

	return emit
}

// handleRelease finishes a click on b and returns codes to emit;
// callers hold s.mu.
func (s *System) handleRelease(b *Button, rightClick bool) []int {
	var emit []int
	push := func(code int) {
		if code != NoValue {
			emit = append(emit, code)
		}
	}

	if b.flags&ButtonSticky != 0 {
		if b.Checked() {
			if b.flags&ButtonNoUncheck == 0 {
				if b.group != nil {
					b.group.currChecked--
				}
				b.flags &^= buttonChecked
				if rightClick {
					push(b.rrValue)
				} else {
					push(b.rValue)
				}
			}
		} else if s.checkGroup(b) {
			b.flags |= buttonChecked
			if rightClick {
				push(b.rpValue)
			} else {
				push(b.rValue)
			}
			if b.group != nil && b.group.onChange != nil {
				b.group.onChange(b.id)
			}
		}
	} else {
		if rightClick {
			push(b.rrValue)
		} else {
			push(b.rValue)
		}
	}

	if s.soundFunc != nil {
		s.soundFunc(b.id, false)
	}
	s.drawButton(b, b.stateImage())
	return emit
}
