// textedit.go : implements single-line GNW input fields.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// TextEdit is a minimal single line input field bound to a window
// region. It consumes key codes from GetInput, maintains the text, and draws the cursor.
// It supports player names, save names, and victory limits, but omits textedit.cpp's mark and clipboard behavior.
//
// Like the C++ TextEdit it owns its font (so repaints don't depend on
// the caller's current font) and restores a snapshot of the pixels
// under the field before every draw instead of filling a background
// color.
//
// C++: textedit.cpp, reduced
type TextEdit struct {
	sys    *System
	win    WinID
	x, y   int
	width  int
	max    int // max characters
	color  int
	font   int         // font number, fixed at creation (C++: font_num)
	bg     *gfx.Buffer // pixels under the field (C++: bg_image)
	buf    []byte
	active bool
}

// NewTextEdit creates an input field at (x, y) in the window, width
// pixels wide, holding at most maxLen characters, rendered with the
// given font (fonts.Font*). The pixels under the field are captured
// as the background restored on every repaint.
//
// C++: TextEdit::TextEdit + LoadBgImage (textedit.cpp)
func (s *System) NewTextEdit(win WinID, x, y, width, maxLen int, color int, font int) *TextEdit {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &TextEdit{
		sys: s, win: win, x: x, y: y,
		width: width, max: maxLen, color: color, font: font,
	}
	if w := s.find(win); w != nil {
		t.bg = w.buf.Sub(x, y, width, t.fontRef().Height).Clone()
	}
	return t
}

// Text returns the current content.
func (t *TextEdit) Text() string {
	return string(t.buf)
}

// SetText replaces the content.
func (t *TextEdit) SetText(str string) {
	if len(str) > t.max {
		str = str[:t.max]
	}
	t.buf = []byte(str)
	t.Draw()
}

// SetActive toggles the cursor.
func (t *TextEdit) SetActive(active bool) {
	t.active = active
	t.Draw()
}

// HandleKey processes one input code, returning true when it was
// consumed (printable characters, backspace).
func (t *TextEdit) HandleKey(code int) bool {
	switch {
	case code == platform.KeyBackspace:
		if len(t.buf) > 0 {
			t.buf = t.buf[:len(t.buf)-1]
		}
	case code >= 32 && code < 127:
		if len(t.buf) >= t.max {
			return true
		}
		f := t.fontRef()
		if f.Width(string(append(t.buf, byte(code))))+cursorBarWidth > t.width {
			return true
		}
		t.buf = append(t.buf, byte(code))
	default:
		return false
	}
	t.Draw()
	return true
}

const cursorBarWidth = 2

// fontRef resolves the field's own font, falling back to the current
// font for an out of range number.
func (t *TextEdit) fontRef() *fonts.Font {
	if f := t.sys.fonts.Get(t.font); f != nil {
		return f
	}
	return t.sys.fonts.Font()
}

// Draw repaints the field into its window: background snapshot back
// first, then the text, then the cursor bar.
//
// C++: TextEdit::DrawFullText (textedit.cpp)
func (t *TextEdit) Draw() {
	s := t.sys
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.find(t.win)
	if w == nil {
		return
	}
	f := t.fontRef()
	area := w.buf.Sub(t.x, t.y, t.width, f.Height)
	if t.bg != nil {
		area.Copy(t.bg)
	}
	// While the field is actively being edited the text highlights white
	// (0xFF); otherwise it draws in its configured color (e.g. 0xA2 metal).
	// C++: TextEdit::DrawFullText: is_being_edited && is_selected ? 0xFF : color.
	col := t.color
	if t.active {
		col = (t.color &^ fonts.ColorMask) | colorWhite
	}
	f.Blit(area, string(t.buf), t.width, col)
	if t.active {
		cx := f.Width(string(t.buf))
		if cx+cursorBarWidth <= t.width {
			area.Sub(cx, 0, cursorBarWidth, f.Height).Fill(uint8(col & fonts.ColorMask))
		}
	}
}

// colorWhite is the highlight color index for an actively edited field.
const colorWhite = 0xFF
