// chrome.go : renders the map editor toolbar, minimap, file buttons, and edge scrolling.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package mapeditor

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// editorAction is a file button press the draw pass reports back to the loop.
// Some open a modal, so the loop runs them after the frame is composed.
type editorAction int

const (
	actNone editorAction = iota
	actNew
	actOpen
	actSave
	actSaveAs
	actPlayTest
	actSettings
	actExit
)

const (
	barWidth = 180 // width of the floating left tool bar when unfolded
	uiMargin = 4
	statusH  = 16 // bottom strip kept clear for the status line
	clusterW = 54 // width of a button in the top right file cluster
)

// editorTabs lists the vertical tabs from top to bottom, matching mapeditor-UX.png. Tabs without an editor body display a one-line placeholder.
var editorTabs = []string{"TOOLS", "TERRAIN", "BUILDINGS", "UNITS", "PROPS"}

var editorTabStubs = []string{
	"Brush / Fill / Spray / Picker / Delete / Select",
	"Terrain + Sea tiles",
	"Place buildings per team",
	"Place units per team",
	"Raw / fuel / gold brush",
}

// fileAction is one small top right cluster button: its label and the editor
// action it fires.
type fileAction struct {
	label string
	act   editorAction
}

// fileActions are the small top right cluster buttons, top to bottom. Settings
// opens the scenario + mission goal screen (no longer a tab).
var fileActions = []fileAction{
	{"New", actNew},
	{"Open", actOpen},
	{"Save", actSave},
	{"Save As", actSaveAs},
	{"Play", actPlayTest},
	{"Setup", actSettings},
	{"Exit", actExit},
}

/* // note: unused code in Go port
// One caller and it only returned e.font; drawChrome reads e.font like the rest of the chrome.
// chromeFont is the font every tool-bar label uses (cached at open).
func (e *Editor) chromeFont() *fonts.Font {
	return e.font
}
*/

// overUI reports whether a screen point lies on the toolbar. The open toolbar occupies the left strip; when folded, only the unfold button remains.
func (e *Editor) overUI(x, y int) bool {
	if e.barFolded {
		return x < uiMargin+e.unfoldW && y < uiMargin+e.rowH+uiMargin
	}
	return x < barWidth
}

// minimapRect is the bottom-left minimap box, shown only while the bar is open.
func (e *Editor) minimapRect(buf *gfx.Buffer) gfx.Rect {
	x := uiMargin
	y := buf.H - statusH - MINIMAP_SIZE - uiMargin
	return gfx.NewRect(x, y, x+MINIMAP_SIZE-1, y+MINIMAP_SIZE-1)
}

/* // note: unused code in Go port
// The same per pixel lookup as gfx Buffer.Recolor; drawChrome calls buf.Sub(...).Recolor(lut).
// dimBand blends a rect's pixels toward black through the recolor LUT, the
// translucent panel look shared with the F2 debug overlay.
func dimBand(buf *gfx.Buffer, x, y, w, h int, lut *[256]uint8) {
	band := buf.Sub(x, y, w, h)
	for yy := 0; yy < band.H; yy++ {
		row := band.Row(yy)
		for xx := 0; xx < band.W; xx++ {
			row[xx] = lut[row[xx]]
		}
	}
}
*/

// boxButton draws a yellow box outline (white while hovered or active) and
// reports a click, the left release frame over the box, which plays KBUY0.
func (e *Editor) boxButton(buf *gfx.Buffer, r gfx.Rect, mouse platform.MouseState, active bool) bool {
	hover := r.Contains(mouse.X, mouse.Y)
	box := uint8(gfx.ColorYellow)
	if hover || active {
		box = 255
	}
	buf.DrawBox(r.ULX, r.ULY, r.LRX, r.LRY, box)

	clicked := hover && mouse.Buttons&platform.MouseReleaseLeft != 0
	if clicked && e.ctx != nil && e.ctx.Audio != nil {
		e.ctx.Audio.PlaySfx("KBUY0")
	}
	return clicked
}

// chromeButton is a boxButton with a centered horizontal yellow label.
func (e *Editor) chromeButton(buf *gfx.Buffer, f *fonts.Font, r gfx.Rect, label string, mouse platform.MouseState, active bool) bool {
	clicked := e.boxButton(buf, r, mouse, active)
	blitCentered(buf, f, r, label)
	return clicked
}

// blitCentered blits a yellow label centered in r, clipped to the buffer.
func blitCentered(buf *gfx.Buffer, f *fonts.Font, r gfx.Rect, label string) {
	tw := f.Width(label)
	tx := r.ULX + (r.Width()+1-tw)/2
	if tx < r.ULX+1 {
		tx = r.ULX + 1
	}
	ty := r.ULY + (r.Height()+1-f.Height)/2
	if ty < r.ULY {
		ty = r.ULY
	}
	dw := tw
	if tx+dw > buf.W {
		dw = buf.W - tx
	}
	if dw > 0 {
		f.Blit(buf.Sub(tx, ty, dw, f.Height), label, tw, int(gfx.ColorYellow))
	}
}

// blitVertical draws a yellow label rotated a quarter turn so it reads bottom to
// top, its rotated box top left at (destX, destY). The label is first blitted
// flat into a scratch buffer, then rotateInto copies its set pixels.
func blitVertical(dst *gfx.Buffer, f *fonts.Font, label string, destX, destY, maxLen int) {
	tw := f.Width(label)
	if tw > maxLen {
		tw = maxLen
	}
	if tw <= 0 {
		return
	}
	scratch := gfx.New(tw, f.Height)
	f.Blit(scratch, label, tw, int(gfx.ColorYellow))
	rotateInto(dst, scratch, destX, destY)
}

// rotateInto copies src into dst rotated a quarter turn counter clockwise, so a
// left to right row of src reads bottom to top in dst, placing the rotated box
// top left at (destX, destY). Background (index 0) pixels are skipped so the bar
// shows through, and out of bounds writes are clipped.
func rotateInto(dst, src *gfx.Buffer, destX, destY int) {
	for ty := 0; ty < src.H; ty++ {
		row := src.Row(ty)
		for tx := 0; tx < src.W; tx++ {
			px := row[tx]
			if px == 0 {
				continue
			}
			rx := destX + ty
			ry := destY + (src.W - 1 - tx)
			if rx < 0 || rx >= dst.W || ry < 0 || ry >= dst.H {
				continue
			}
			dst.Row(ry)[rx] = px
		}
	}
}

// drawChrome renders the tool bar over the map and returns any file action
// clicked this frame. Tab, fold and zoom clicks are pure state, settled here;
// file actions run in the loop (some open a modal).
func (e *Editor) drawChrome(buf *gfx.Buffer, mouse platform.MouseState) editorAction {
	f := e.font
	lut := e.palette.IntensityTable1(0, 0, 0, 60, 40)

	// The status band spans the full width in both states. Recolor through the
	// LUT blends it toward black, the translucent panel look of the F2 debug overlay.
	buf.Sub(0, buf.H-statusH, buf.W, statusH).Recolor(lut)

	if e.barFolded {
		// Only a small unfold button at the top left; the map is full screen.
		r := gfx.NewRect(uiMargin, uiMargin, uiMargin+e.unfoldW-1, uiMargin+e.rowH)
		if e.chromeButton(buf, f, r, "Unfold >>", mouse, false) {
			e.barFolded = false
		}
		return actNone
	}

	// Left bar: the translucent band from the top down to the status strip, with
	// the minimap drawn over it (after the dim, so it stays crisp).
	buf.Sub(0, 0, barWidth, buf.H-statusH).Recolor(lut)
	mm := e.minimapRect(buf)
	e.renderer.Minimap(buf.Sub(mm.ULX, mm.ULY, MINIMAP_SIZE, MINIMAP_SIZE), e.scrollX, e.scrollY, buf.W, buf.H, 0xFF)

	action := actNone

	// Vertical rotated tabs down the far left edge, in the narrow tab font. The
	// tab height fits the longest label but never runs into the minimap.
	tf := e.tabFont
	tabStripW := tf.Height + 8
	maxLabel := 0
	for _, name := range editorTabs {
		if w := tf.Width(name); w > maxLabel {
			maxLabel = w
		}
	}
	availH := mm.ULY - uiMargin - uiMargin
	tabH := maxLabel + 8
	if fit := (availH - (len(editorTabs)-1)*2) / len(editorTabs); tabH > fit {
		tabH = fit
	}
	ty := uiMargin
	for i, name := range editorTabs {
		if e.boxButton(buf, gfx.NewRect(0, ty, tabStripW-1, ty+tabH), mouse, i == e.tab) {
			e.tab = i
		}
		labelLen := tabH - 4
		rendered := min(tf.Width(name), labelLen)
		blitVertical(buf, tf, name, (tabStripW-tf.Height)/2, ty+(tabH-rendered)/2, labelLen)
		ty += tabH + 2
	}

	// Top right cluster: fold, zoom toggle, then the file actions.
	cx := barWidth - uiMargin - clusterW
	cy := uiMargin
	step := e.rowH + 2

	if e.chromeButton(buf, f, gfx.NewRect(cx, cy, cx+clusterW, cy+e.rowH), "<< Fold", mouse, false) {
		e.barFolded = true
	}
	cy += step
	if e.chromeButton(buf, f, gfx.NewRect(cx, cy, cx+clusterW, cy+e.rowH), "Zoom x2", mouse, false) {
		e.toggleZoom()
	}
	cy += step
	for _, fa := range fileActions {
		if e.chromeButton(buf, f, gfx.NewRect(cx, cy, cx+clusterW, cy+e.rowH), fa.label, mouse, false) {
			action = fa.act
		}
		cy += step
	}

	// The 'sel.' swatch: the current selection, top left of the content panel.
	swatchLeft := tabStripW + uiMargin
	swatchRight := cx - uiMargin
	if swatchRight > swatchLeft+8 {
		sr := gfx.NewRect(swatchLeft, uiMargin, swatchRight, uiMargin+e.rowH+8)
		buf.DrawBox(sr.ULX, sr.ULY, sr.LRX, sr.LRY, uint8(gfx.ColorYellow))
		blitCentered(buf, f, sr, "sel.")
	}

	// Draw the active tab's placeholder below the top row and clip it before the file-button cluster.
	stubY := uiMargin + max(e.rowH+8, 2*step) + 6
	sw := swatchRight - swatchLeft
	if sw > 0 && stubY+f.Height < mm.ULY {
		f.Blit(buf.Sub(swatchLeft, stubY, sw, f.Height), editorTabStubs[e.tab], sw, int(gfx.ColorYellow))
	}

	return action
}

// toggleZoom flips the view between the opening zoom and twice that, wiring the
// cluster's 'Zoom x2' button.
func (e *Editor) toggleZoom() {
	if e.renderer.Zoom() >= START_ZOOM*2 {
		e.renderer.SetZoom(START_ZOOM)
	} else {
		e.renderer.SetZoom(START_ZOOM * 2)
	}
}

// edgeScroll pans the viewport while the cursor rests against a screen edge,
// like the in-game screen. It skips the tool-bar overlays.
func (e *Editor) edgeScroll(buf *gfx.Buffer, mouse platform.MouseState) {
	const edge = 6
	const speed = 10
	if e.overUI(mouse.X, mouse.Y) {
		return
	}
	switch {
	case mouse.X < edge:
		e.scrollX -= speed
	case mouse.X >= buf.W-edge:
		e.scrollX += speed
	}
	switch {
	case mouse.Y < edge:
		e.scrollY -= speed
	case mouse.Y >= buf.H-edge:
		e.scrollY += speed
	}
}
