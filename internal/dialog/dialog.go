// dialog.go : lays out, renders, and handles input for M.A.X. modal dialogs.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package dialog composites stock modal frames into 8-bit buffers.
package dialog

import (
	"strings"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
)

const (
	screenW = 640
	screenH = 480

	captionX = 20
	captionY = 14
	captionW = 265
	captionH = 175

	okOnlyAcceptX = 120
	acceptX       = 155
	cancelX       = 85
	buttonY       = 193
	scrollUpX     = 20
	scrollDownX   = 40
	scrollY       = 197

	keyReturn   = 13
	keyEscape   = 27
	keyPageUp   = 329
	keyPageDown = 337

	textColor = fonts.FlagOutline | 0xFF
)

// Mode selects the button set and text layout style.
type Mode int

const (
	ModeOK Mode = iota
	ModeOKCancel
)

// Result is the resolved dialog action.
type Result int

const (
	ResultNone Result = iota
	ResultAccept
	ResultCancel
)

// Art is the decoded 8 bit image set the dialog composites.
type Art struct {
	Frame                        *gfx.Buffer
	AcceptUp, AcceptDown         *gfx.Buffer
	CancelUp, CancelDown         *gfx.Buffer
	ScrollUpUp, ScrollUpDown     *gfx.Buffer
	ScrollDownUp, ScrollDownDown *gfx.Buffer
}

// Dialog is a laid out dialog instance.
type Dialog struct {
	art                      Art
	font                     *fonts.Font
	mode                     Mode
	caption                  string
	acceptLabel, cancelLabel string
	origin                   game.Point
	lines                    []string
	rowOffset                int
	maxRows                  int
	pressed                  buttonKind
}

type buttonKind int

const (
	buttonNone buttonKind = iota
	buttonAccept
	buttonCancel
	buttonScrollUp
	buttonScrollDown
)

// New lays out a dialog. The labels are expected to be localized by
// the caller.
//
// C++: DialogMenu::DialogMenu (dialogmenu.cpp),
// OKCancelMenu::OKCancelMenu (okcancelmenu.cpp)
func New(art Art, font *fonts.Font, mode Mode, caption, acceptLabel, cancelLabel string) *Dialog {
	d := &Dialog{
		art:         art,
		font:        font,
		mode:        mode,
		caption:     caption,
		acceptLabel: acceptLabel,
		cancelLabel: cancelLabel,
	}
	if art.Frame != nil {
		d.origin = game.Point{X: (screenW - art.Frame.W) / 2, Y: (screenH - art.Frame.H) / 2}
	}
	d.rewrap()
	return d
}

// SetOrigin overrides the default centered frame origin.
func (d *Dialog) SetOrigin(p game.Point) {
	d.origin = p
}

// Scrollable reports whether the OK dialog caption needs scroll controls.
func (d *Dialog) Scrollable() bool {
	return d.scrollable()
}

// Draw composites the current dialog state into dst.
func (d *Dialog) Draw(dst *gfx.Buffer) {
	if d.art.Frame == nil {
		return
	}
	blit(dst, d.art.Frame, d.origin.X, d.origin.Y, false)
	d.drawText(dst)
	d.drawButton(dst, buttonAccept, d.acceptRect(), d.art.AcceptUp, d.art.AcceptDown, d.acceptLabel)
	if d.mode == ModeOKCancel {
		d.drawButton(dst, buttonCancel, d.cancelRect(), d.art.CancelUp, d.art.CancelDown, d.cancelLabel)
	}
	if d.scrollable() {
		d.drawButton(dst, buttonScrollUp, d.scrollUpRect(), d.art.ScrollUpUp, d.art.ScrollUpDown, "")
		d.drawButton(dst, buttonScrollDown, d.scrollDownRect(), d.art.ScrollDownUp, d.art.ScrollDownDown, "")
	}
}

// HandleKey resolves one GNW key code.
func (d *Dialog) HandleKey(key int) Result {
	switch key {
	case keyReturn:
		return ResultAccept
	case keyEscape:
		if d.mode == ModeOK {
			return ResultAccept
		}
		return ResultCancel
	case keyPageUp:
		d.pageUp()
	case keyPageDown:
		d.pageDown()
	}
	return ResultNone
}

// rewrap rebuilds wrapped message lines for the current dialog width.
func (d *Dialog) rewrap() {
	if d.font == nil {
		d.maxRows = captionH
		d.lines = nil
		return
	}
	d.maxRows = captionH / d.font.Height
	if d.maxRows < 1 {
		d.maxRows = 1
	}
	limit := 0
	if d.mode == ModeOK {
		limit = d.maxRows * 3
	}
	d.lines = wrapLines(d.font, d.caption, captionW, limit)
}

// drawText renders the visible message rows into the dialog body.
func (d *Dialog) drawText(dst *gfx.Buffer) {
	if d.font == nil || len(d.lines) == 0 {
		return
	}
	first := d.rowOffset
	last := min(first+d.maxRows, len(d.lines))
	rowCount := last - first

	y := d.origin.Y + captionY
	if d.mode == ModeOK && rowCount*d.font.Height < captionH {
		y += (captionH - rowCount*d.font.Height) / 2
	}
	if d.mode == ModeOKCancel {
		y += (captionH + 1 - rowCount*d.font.Height) / 2
	}

	for i := first; i < last; i++ {
		line := d.lines[i]
		x := d.origin.X + captionX
		if d.mode == ModeOKCancel {
			w := d.font.Width(line) + 1
			if w > captionW {
				w = captionW
			}
			x += (captionW - w) / 2
		}
		// Centering moves x to the right, so clip the caption width to both its
		// layout area and the remaining buffer before calling Sub.
		yy := y + (i-first)*d.font.Height
		bw := d.origin.X + captionX + captionW - x // remaining caption width
		if x+bw > dst.W {
			bw = dst.W - x
		}
		bh := d.font.Height
		if yy+bh > dst.H {
			bh = dst.H - yy
		}
		if x < 0 || yy < 0 || bw <= 0 || bh <= 0 {
			continue
		}
		d.font.Blit(dst.Sub(x, yy, bw, bh), line, bw, textColor)
	}
}

// drawButton draws one dialog button in its current pressed or idle state.
func (d *Dialog) drawButton(dst *gfx.Buffer, kind buttonKind, r gfx.Rect, up, down *gfx.Buffer, label string) {
	img := up
	color := fonts.ColorGold
	if d.pressed == kind {
		img = down
		color = fonts.ColorDarkOrange
	}
	if img == nil {
		return
	}
	blit(dst, img, r.ULX, r.ULY, true)
	if d.font == nil || label == "" {
		return
	}
	tw := d.font.Width(label)
	x := r.ULX + (img.W-tw)/2
	y := r.ULY + (img.H-d.font.Height)/2
	if x < r.ULX {
		x = r.ULX
	}
	if y < r.ULY {
		y = r.ULY
	}
	d.font.BlitShadowed(dst.Sub(x, y, min(img.W, dst.W-x), min(d.font.Height+1, dst.H-y)), label, 0, color)
}

// acceptRect returns the hit box for the accept button.
func (d *Dialog) acceptRect() gfx.Rect {
	x := acceptX
	if d.mode == ModeOK {
		x = okOnlyAcceptX
	}
	return rectAt(d.origin, x, buttonY, d.art.AcceptUp)
}

// cancelRect returns the hit box for the cancel button.
func (d *Dialog) cancelRect() gfx.Rect {
	return rectAt(d.origin, cancelX, buttonY, d.art.CancelUp)
}

// scrollUpRect returns the hit box for the scroll up button.
func (d *Dialog) scrollUpRect() gfx.Rect {
	return rectAt(d.origin, scrollUpX, scrollY, d.art.ScrollUpUp)
}

// scrollDownRect returns the hit box for the scroll down button.
func (d *Dialog) scrollDownRect() gfx.Rect {
	return rectAt(d.origin, scrollDownX, scrollY, d.art.ScrollDownUp)
}

// scrollable reports whether the message has more rows than the visible area.
func (d *Dialog) scrollable() bool {
	return len(d.lines) > d.maxRows
}

// pageUp scrolls the message upward by one visible page.
func (d *Dialog) pageUp() {
	if !d.scrollable() {
		return
	}
	d.rowOffset -= d.maxRows
	if d.rowOffset < 0 {
		d.rowOffset = 0
	}
}

// pageDown scrolls the message downward by one visible page.
func (d *Dialog) pageDown() {
	if !d.scrollable() {
		return
	}
	d.rowOffset += d.maxRows
	maxOffset := len(d.lines) - d.maxRows
	if d.rowOffset > maxOffset {
		d.rowOffset = maxOffset
	}
}

// rectAt builds a rectangle sized to img relative to a dialog origin.
func rectAt(origin game.Point, x, y int, img *gfx.Buffer) gfx.Rect {
	if img == nil {
		return gfx.NewRect(origin.X+x, origin.Y+y, origin.X+x-1, origin.Y+y-1)
	}
	return gfx.NewRect(origin.X+x, origin.Y+y, origin.X+x+img.W-1, origin.Y+y+img.H-1)
}

// blit copies a dialog asset into dst, optionally skipping transparent pixels.
func blit(dst, src *gfx.Buffer, x, y int, transparent bool) {
	if src == nil {
		return
	}
	for sy := 0; sy < src.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.H {
			continue
		}
		srow := src.Row(sy)
		for sx, c := range srow {
			dx := x + sx
			if dx < 0 || dx >= dst.W || transparent && c == 0 {
				continue
			}
			dst.Set(dx, dy, c)
		}
	}
}

// C++: Text_SplitText (text.cpp)
func wrapLines(font *fonts.Font, text string, width, limit int) []string {
	if font == nil {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = appendLimited(lines, "", limit)
			continue
		}
		line := ""
		for _, word := range words {
			parts := splitLongWord(font, word, width)
			for _, part := range parts {
				if line == "" {
					line = part
				} else if font.Width(line+" "+part) <= width {
					line += " " + part
				} else {
					lines = appendLimited(lines, line, limit)
					line = part
				}
				if limit > 0 && len(lines) >= limit {
					return lines
				}
			}
		}
		if line != "" || len(words) == 0 {
			lines = appendLimited(lines, line, limit)
			if limit > 0 && len(lines) >= limit {
				return lines
			}
		}
	}
	return lines
}

// splitLongWord breaks a single word too wide to fit into drawable chunks.
func splitLongWord(font *fonts.Font, word string, width int) []string {
	if font.Width(word) <= width || len(word) < 4 {
		return []string{word}
	}
	var out []string
	rest := word
	for font.Width(rest) > width && len(rest) >= 4 {
		cut := len(rest) - 1
		for cut > 1 && font.Width(rest[:cut]+"-") > width {
			cut--
		}
		if cut <= 1 {
			break
		}
		out = append(out, rest[:cut]+"-")
		rest = rest[cut:]
	}
	if rest != "" {
		out = append(out, rest)
	}
	return out
}

// appendLimited appends text while respecting a maximum line count.
func appendLimited(lines []string, line string, limit int) []string {
	if limit > 0 && len(lines) >= limit {
		return lines
	}
	return append(lines, line)
}
