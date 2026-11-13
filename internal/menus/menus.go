// menus.go : provides shared helpers and entry points for the GNW-based menu screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: menu.cpp - main_menu and shared menu helpers
//
// Package menus implements the game's menu screens on top of the gnw layer.
package menus

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/audio"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dialog"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/help"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/sprites"
)

// Context bundles what every menu screen needs.
type Context struct {
	Plat    *platform.Platform
	Sys     *gnw.System
	Lib     *resfile.Library
	Assets  *assets.Assets
	Audio   *audio.Manager
	DataDir string
	Rand    *rand.Rand

	Settings     *settings.Settings // live game configuration, editable by in game Options menus
	SettingsPath string             // settings.json path it persists to

	// HelpFixup maps a live pointer position into the 640x480 space the help database's
	// rectangles are written in. Menu screens are already that space, so only the in game
	// screen installs one (its widgets move in widescreen). Nil means identity.
	HelpFixup func(x, y int) (int, int)

	buttonSfx map[gnw.ButtonID]string // press sound per button
	sfxHooked bool

	help      *help.Database   // the context help database, parsed on first use
	cursorCat *cursors.Catalog // cursor art, loaded on first use (help mode swaps the pointer)

	statIcons map[string]*gfx.Buffer           // keyed gauge icons, loaded on first use
	portraits map[assets.UnitType]*sprites.Big // unit portraits, loaded on first use
}

// registerButtonSfx assigns a press sound to a button and installs
// the gnw sound hook on first use.
//
// C++: MenuButton.sfx + Button sound delivery (menu.cpp)
func (ctx *Context) registerButtonSfx(bid gnw.ButtonID, tag string) {
	if ctx.Audio == nil || bid < 0 {
		return
	}
	if ctx.buttonSfx == nil {
		ctx.buttonSfx = map[gnw.ButtonID]string{}
	}
	ctx.buttonSfx[bid] = tag
	if !ctx.sfxHooked {
		ctx.sfxHooked = true
		ctx.Sys.SetButtonSoundFunc(func(b gnw.ButtonID, down bool) {
			if down {
				if sfx, ok := ctx.buttonSfx[b]; ok {
					ctx.Audio.PlaySfx(sfx)
				}
			}
		})
	}
}

// playSfx plays the KCARG0 list row click, for the places that answer a click themselves instead of through a
// button (the list rows).
func (ctx *Context) playSfx() {
	if ctx.Audio == nil {
		return
	}
	ctx.Audio.PlaySfx("KCARG0")
}

// text looks up a localized string by its lang_english.ini key.
func (ctx *Context) text(key, fallback string) string {
	if ctx.Assets == nil {
		return fallback
	}
	return ctx.Assets.Strings.Get(key, fallback)
}

// Window 1 is the full screen background window created by gnw.New.
const mainWindow gnw.WinID = 1

// colorWhite is the plain white palette index used for dialog text.
const colorWhite = 255

// GNW color indices (color.h): COLOR_GREEN is the lime green of the
// menu title displays, COLOR_RED the selection boxes.
const (
	colorGreen = 2
	colorRed   = 1
)

// drawMenuTitle prints the caption centered on the black display
// below the M.A.X. logo (the 172x15 box at 236,145).
//
// C++: draw_menu_title (menu.cpp)
func (ctx *Context) drawMenuTitle(caption string) {
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	f := ctx.Sys.Fonts().Font()
	x := 236
	if w := f.Width(caption); w < 172 {
		x += (172 - w) / 2
	}
	y := 145 + (15-f.Height)/2
	ctx.Sys.Print(mainWindow, caption, 172, x, y, colorGreen)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
}

// setHandCursor installs the game's blue hand pointer.
//
// C++: Cursor_SetCursor(CURSOR_HAND): sprite HANDPTR (cursor.cpp)
func (ctx *Context) setHandCursor() {
	img, err := ctx.loadSimple("HANDPTR")
	if err != nil {
		return // keep the built in arrow
	}
	ctx.Sys.SetCursor(img.Buffer(), img.ULX, img.ULY, 0)
}

// loadBig reads and decodes a BIG sprite resource from the menu library.
func (ctx *Context) loadBig(tag string) (*sprites.Big, error) {
	data, err := ctx.Lib.Read(tag)
	if err != nil {
		return nil, err
	}
	img, err := sprites.DecodeBig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return img, nil
}

// statIcon returns a gauge icon (I_HITS, I_ARMOR, ...) with its fuchsia top left keyed to transparent, loaded once
// per session and shared by every menu; nil when the tag is empty or does not load.
func (ctx *Context) statIcon(tag string) *gfx.Buffer {
	if b, ok := ctx.statIcons[tag]; ok {
		return b
	}
	var b *gfx.Buffer
	if tag != "" {
		if img, e := ctx.loadSimple(tag); e == nil {
			c := img.Buffer().Clone()
			c.KeyToZero(c.At(0, 0))
			b = c
		}
	}
	if ctx.statIcons == nil {
		ctx.statIcons = map[string]*gfx.Buffer{}
	}
	ctx.statIcons[tag] = b
	return b
}

// loadPortrait returns a unit's large portrait, loaded once per session and shared by every menu; nil when the unit has
// none or it does not load. Callers that draw into it must clone it first.
func (ctx *Context) loadPortrait(t assets.UnitType) *sprites.Big {
	if b, ok := ctx.portraits[t]; ok {
		return b
	}
	var b *sprites.Big
	if tag := unitPortrait[t]; tag != "" {
		if img, e := ctx.loadBig(tag); e == nil {
			b = img
		}
	}
	if ctx.portraits == nil {
		ctx.portraits = map[assets.UnitType]*sprites.Big{}
	}
	ctx.portraits[t] = b
	return b
}

// loadSimple reads and decodes a SIMPLE sprite resource from the menu library.
func (ctx *Context) loadSimple(tag string) (*sprites.Simple, error) {
	data, err := ctx.Lib.Read(tag)
	if err != nil {
		return nil, err
	}
	img, err := sprites.DecodeSimple(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tag, err)
	}
	return img, nil
}

// drawBigAt decodes nothing; it blits a decoded big image into the
// window buffer at (x+img.ULX, y+img.ULY), the position rule of
// WindowManager_LoadBigImage with ulx/uly == -1.
func (ctx *Context) drawBigAt(img *sprites.Big, x, y int) {
	buf := ctx.Sys.Buf(mainWindow)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	x += img.ULX
	y += img.ULY
	w := min(img.W, buf.W-x)
	h := min(img.H, buf.H-y)
	if w <= 0 || h <= 0 {
		return
	}
	buf.Sub(x, y, w, h).Copy(img.Buffer().Sub(0, 0, w, h))
}

// drawSimpleAt blits a simple image into the window buffer at (x, y),
// honoring the image hotspot like WindowManager_LoadSimpleImage.
func (ctx *Context) drawSimpleAt(win gnw.WinID, img *sprites.Simple, x, y int, transparent bool) {
	buf := ctx.Sys.Buf(win)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	img.Draw(buf, x, y, transparent)
}

// fuchsiaKey reports whether a palette index is one of MAX's transparent (fuchsia) slots
// in COLOR.PAL (ranges 2-3, 9-31, 64-159). An image whose transparent_color (top left
// pixel) is one of these blits with that colour keyed out; solid plate buttons use
// stable bevel colours (>=160) and stay opaque. See [[max-image-transparent-color]].
func fuchsiaKey(idx uint8) bool {
	return idx == 2 || idx == 3 || (idx >= 9 && idx <= 31) || (idx >= 64 && idx <= 159)
}

// keyButtonCorner makes buf transparent on its corner colour when that colour is
// a fuchsia slot (so beveled/cut corners reveal the panel), reporting whether it
// applied. C++: the per image transparent_color + button transparent blit.
func keyButtonCorner(buf *gfx.Buffer) bool {
	if k := buf.At(0, 0); fuchsiaKey(k) {
		buf.KeyToZero(k)
		return true
	}
	return false
}

// bottomPlate is one plate button on the bottom row several setup screens
// share (Random / Cancel / ? / Done at y=438): the x position, the up art tag
// (the down tag swaps the trailing U for a D), the caption, the key value and
// the click sfx.
type bottomPlate struct {
	x     int
	tag   string
	label string
	value int
	sfx   string
}

// addBottomPlates creates a row of bottom plate buttons on the main window and
// appends their ids to *buttons (so the caller's cleanup releases whatever was
// created even when a later plate fails to load).
func (ctx *Context) addBottomPlates(defs []bottomPlate, buttons *[]gnw.ButtonID) error {
	for _, d := range defs {
		bid, err := ctx.plateButton(mainWindow, d.x, 438, d.tag, d.tag[:len(d.tag)-1]+"D", d.label, d.value)
		if err != nil {
			return err
		}
		ctx.registerButtonSfx(bid, d.sfx)
		*buttons = append(*buttons, bid)
	}
	return nil
}

// plateButton creates a button from up/down plate images with a centered caption, the
// way menu_draw_main_menu_buttons builds the main menu buttons from BLANK_UP/BLANK_DN.
// C++: Button::SetCaption + menu_draw_main_menu_buttons (menu.cpp).
func (ctx *Context) plateButton(win gnw.WinID, x, y int, upTag, downTag, label string, value int) (gnw.ButtonID, error) {
	return ctx.plateButtonOff(win, x, y, upTag, downTag, label, value, 0)
}

// plateButtonOff is plateButton with an extra caption x-offset (logical pixels,
// added to the centred position) for the few arts whose recessed face is not
// centred in the image, e.g. MNUBTN5U nudges "?" a couple pixels left. Default
// 0 (plateButton) leaves the caption image centred.
func (ctx *Context) plateButtonOff(win gnw.WinID, x, y int, upTag, downTag, label string, value, capOffX int) (gnw.ButtonID, error) {
	upImg, err := ctx.loadSimple(upTag)
	if err != nil {
		return -1, err
	}
	downImg, err := ctx.loadSimple(downTag)
	if err != nil {
		return -1, err
	}

	up := upImg.Buffer().Clone()
	down := downImg.Buffer().Clone()
	var flags uint32
	// Auto detect a fuchsia transparent corner (e.g. DPTBAYUP/BLDONE_U = idx 9);
	// solid plates (MNUBTN/DPTHP = idx 217/218) are left opaque.
	if keyButtonCorner(up) {
		keyButtonCorner(down)
		flags = gnw.ButtonTransparent
	}

	// Captions use the bevel and shadow rendering of
	// Button::SetCaption / Text_TextLine: gold at rest, dark orange
	// ("etched") while pressed, both at the same position.
	f := ctx.Sys.Fonts().Font()
	tw := f.Width(label)
	textX := (up.W-tw)/2 + capOffX
	textY := (up.H - f.Height) / 2
	if textX < 0 {
		textX = 0
	}
	if textX > up.W-1 {
		textX = up.W - 1
	}
	f.BlitShadowed(up.Sub(textX, textY, up.W-textX, up.H-textY), label, 0, fonts.ColorGold)
	f.BlitShadowed(down.Sub(textX, textY, down.W-textX, down.H-textY), label, 0, fonts.ColorDarkOrange)

	bid := ctx.Sys.AddButton(win, x, y, up.W, up.H,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, value, up, down, nil, flags)
	if bid < 0 {
		return -1, fmt.Errorf("menus: AddButton failed for %q", label)
	}
	return bid, nil
}

// dataDir returns the game data directory.
func (ctx *Context) dataDir() (string, error) {
	if ctx.DataDir == "" {
		return "", fmt.Errorf("menus: no data dir configured")
	}
	return ctx.DataDir, nil
}

// snapshotArea copies the current content of a main window region,
// so it can be restored before repainting text over background art.
//
// C++: Image::Copy(WindowInfo*) (image.cpp)
func (ctx *Context) snapshotArea(r gfx.Rect) *gfx.Buffer {
	buf := ctx.Sys.Buf(mainWindow)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	return buf.SubRect(r).Clone()
}

// restoreArea writes a snapshot back.
//
// C++: Image::Write(WindowInfo*) (image.cpp)
func (ctx *Context) restoreArea(r gfx.Rect, saved *gfx.Buffer) {
	if saved == nil {
		return
	}
	buf := ctx.Sys.Buf(mainWindow)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	buf.SubRect(r).Copy(saved)
}

// printCentered prints one line centered in the rect (corners
// inclusive) with the current font.
//
// C++: menu_draw_menu_title (menu.cpp)
func (ctx *Context) printCentered(text string, r gfx.Rect, color int) {
	if text == "" {
		return
	}
	f := ctx.Sys.Fonts().Font()
	x := r.ULX
	if w := f.Width(text); w < r.Width()+1 {
		x += (r.Width() + 1 - w) / 2
	}
	y := r.ULY + (r.Height()+1-f.Height)/2
	ctx.Sys.Print(mainWindow, text, r.Width()+1, x, y, color)
}

// printWrapped renders word wrapped text top left aligned in the
// rect of the main window, with the current font.
func (ctx *Context) printWrapped(text string, r gfx.Rect, color int) {
	ctx.printWrappedIn(mainWindow, text, r, color)
}

// printWrappedIn is printWrapped into an arbitrary window.
//
// C++: Text_TextBox (text.cpp), reduced to left aligned wrapping
func (ctx *Context) printWrappedIn(win gnw.WinID, text string, r gfx.Rect, color int) {
	if text == "" {
		return
	}
	f := ctx.Sys.Fonts().Font()
	maxW := r.Width() + 1

	var line string
	y := r.ULY
	flush := func() {
		if line != "" && y+f.Height <= r.LRY+1 {
			ctx.Sys.Print(win, line, maxW, r.ULX, y, color)
			y += f.Height + 1
		}
		line = ""
	}
	for _, word := range strings.Fields(text) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if f.Width(candidate) > maxW && line != "" {
			flush()
			line = word
		} else {
			line = candidate
		}
	}
	flush()
}

// wrapLines splits text into at most maxRows word wrapped lines for
// the given pixel width; maxRows <= 0 means no limit.
//
// C++: Text_SplitText (text.cpp)
func wrapLines(f *fonts.Font, text string, maxW, maxRows int) []string {
	var lines []string
	var line string
	for _, word := range strings.Fields(text) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if f.Width(candidate) > maxW && line != "" {
			lines = append(lines, line)
			if maxRows > 0 && len(lines) == maxRows {
				return lines
			}
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" && (maxRows <= 0 || len(lines) < maxRows) {
		lines = append(lines, line)
	}
	return lines
}

// printShadowed renders a bevel and shadow caption in the rect of the
// main window: word wrapped, the block vertically centered, each row
// optionally horizontally centered.
//
// C++: Text_TextBox with a FontColor (text.cpp)
func (ctx *Context) printShadowed(text string, r gfx.Rect, color fonts.Color, centered bool) {
	if text == "" {
		return
	}
	f := ctx.Sys.Fonts().Font()
	rowH := f.Height + 1
	boxH := r.Height() + 1
	maxRows := max(boxH/rowH, 1)
	lines := wrapLines(f, text, r.Width()+1, maxRows)
	y := r.ULY + (boxH+1-rowH*len(lines))/2

	buf := ctx.Sys.Buf(mainWindow)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	for _, line := range lines {
		x := r.ULX
		if centered {
			if w := f.Width(line); w < r.Width()+1 {
				x += (r.Width() + 1 - w) / 2
			}
		}
		w := min(r.LRX+1, buf.W) - x
		h := buf.H - y
		if w <= 0 || h <= 0 || y < 0 {
			return
		}
		f.BlitShadowed(buf.Sub(x, y, w, h), line, w, color)
		y += rowH
	}
}

// printTextBox renders plain color word wrapped text with the block
// vertically centered in the rect (the Text_TextBox default), unlike
// printWrapped which is top aligned.
//
// C++: Text_TextBox(buffer, ...) with vertical_align (text.cpp)
func (ctx *Context) printTextBox(text string, r gfx.Rect, color int, centered bool) {
	if text == "" {
		return
	}
	f := ctx.Sys.Fonts().Font()
	rowH := f.Height
	if color&fonts.FlagOutline != 0 {
		rowH++
	}
	boxH := r.Height() + 1
	maxRows := max(boxH/rowH, 1)
	lines := wrapLines(f, text, r.Width()+1, maxRows)
	y := r.ULY + (boxH+1-rowH*len(lines))/2

	for _, line := range lines {
		x := r.ULX
		if centered {
			if w := f.Width(line); w < r.Width()+1 {
				x += (r.Width() + 1 - w) / 2
			}
		}
		ctx.Sys.Print(mainWindow, line, r.Width()+1, x, y, color)
		y += rowH
	}
}

// imageButton builds a button from up/down image resources, optionally blending an
// overlay into both states (C++ Button::Copy, used for the clan logos). Sticky buttons
// emit their value on press and stay down (the SetFlags(5)+SetPValue pattern of the C++
// pre game menus); plain buttons emit on release.
func (ctx *Context) imageButton(win gnw.WinID, x, y int, upTag, downTag, overlayTag string,
	overlayX, overlayY int, sticky bool, value int) (gnw.ButtonID, error) {

	upImg, err := ctx.loadSimple(upTag)
	if err != nil {
		return -1, err
	}
	downImg, err := ctx.loadSimple(downTag)
	if err != nil {
		return -1, err
	}
	up := upImg.Buffer().Clone()
	down := downImg.Buffer().Clone()
	// Auto detect a fuchsia transparent corner (e.g. UPGLFT_U/UPGRGT_U = idx 9);
	// solid arrows/toggles (LFARO/SEL*/BLDUP = idx 182/217/218) stay opaque.
	transparent := keyButtonCorner(up)
	if transparent {
		keyButtonCorner(down)
	}
	if overlayTag != "" {
		ov, err := ctx.loadSimple(overlayTag)
		if err != nil {
			return -1, err
		}
		ov.Draw(up, overlayX, overlayY, true)
		// The pressed state nudges the overlay one pixel down.
		// C++: Button::Copy(ResourceID, ulx, uly) (button.cpp)
		ov.Draw(down, overlayX, overlayY+1, true)
	}

	pv, rv := gnw.NoValue, value
	var flags uint32
	if sticky {
		flags = gnw.ButtonSticky | gnw.ButtonNoUncheck
		pv, rv = value, gnw.NoValue
	}
	if transparent {
		flags |= gnw.ButtonTransparent
	}
	bid := ctx.Sys.AddButton(win, x, y, up.W, up.H,
		gnw.NoValue, gnw.NoValue, pv, rv, up, down, nil, flags)
	if bid < 0 {
		return -1, fmt.Errorf("menus: AddButton failed for %s", upTag)
	}
	return bid, nil
}

// toUpper folds lowercase letter input codes, like the menu loops'
// toupper(get_input()).
func toUpper(key int) int {
	if key >= 'a' && key <= 'z' {
		return key - 'a' + 'A'
	}
	return key
}

// Message shows a MAX style OK modal with caller provided text.
func Message(ctx *Context, text string) {
	messageBox(ctx, text)
}

// Confirm shows a MAX style OK/Cancel modal and reports whether OK won.
func Confirm(ctx *Context, text string) bool {
	return modalDialog(ctx, text, dialog.ModeOKCancel) == dialog.ResultAccept
}

// messageBox shows a modal text dialog with an OK button.
//
// C++: DialogMenu (dialogmenu.cpp)
func messageBox(ctx *Context, text string) {
	if modalDialog(ctx, text, dialog.ModeOK) != dialog.ResultNone {
		return
	}
	fallbackMessageBox(ctx, text)
}

// modalDialog runs the shared modal dialog loop and returns the player choice.
func modalDialog(ctx *Context, text string, mode dialog.Mode) dialog.Result {
	art, err := loadDialogArt(ctx)
	if err != nil {
		return dialog.ResultNone
	}
	fontsSet := ctx.Sys.Fonts()
	oldFont := fontsSet.GetFont()
	_ = fontsSet.SetFont(fonts.Font5)
	defer fontsSet.SetFont(oldFont)

	d := dialog.New(art, fontsSet.Font(), mode, text,
		ctx.text("ae0b", "OK"), ctx.text("2879", "Cancel"))
	if mode == dialog.ModeOKCancel {
		d = dialog.New(art, fontsSet.Font(), mode, text,
			ctx.text("755f", "OK"), ctx.text("2879", "Cancel"))
	}
	d.SetOrigin(game.Point{})

	// Center on the live screen size: 640 in the menus, 854 when the dialog
	// (e.g. the ESC exit confirm) opens over the wide game view.
	screenW, screenH := ctx.Sys.ScreenSize()
	win := ctx.Sys.AddWindow((screenW-art.Frame.W)/2, (screenH-art.Frame.H)/2,
		art.Frame.W, art.Frame.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return dialog.ResultNone
	}
	defer ctx.Sys.DeleteWindow(win)

	redraw := func() {
		buf := ctx.Sys.Buf(win)
		if buf == nil {
			return
		}
		ctx.Sys.Lock()
		d.Draw(buf)
		ctx.Sys.Unlock()
	}
	redraw()

	acceptUp, acceptDown := captionDialogButton(art.AcceptUp, art.AcceptDown, dAcceptLabel(ctx, mode), fontsSet.Font())
	acceptX := 120
	if mode == dialog.ModeOKCancel {
		acceptX = 155
	}
	accept := ctx.Sys.AddButton(win, acceptX, 193, acceptUp.W, acceptUp.H,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyReturn, acceptUp, acceptDown, nil, 0)
	ctx.registerButtonSfx(accept, "NDONE0")

	if mode == dialog.ModeOKCancel {
		cancelUp, cancelDown := captionDialogButton(art.CancelUp, art.CancelDown, ctx.text("2879", "Cancel"), fontsSet.Font())
		cancel := ctx.Sys.AddButton(win, 85, 193, cancelUp.W, cancelUp.H,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyEscape, cancelUp, cancelDown, nil, 0)
		ctx.registerButtonSfx(cancel, "NCANC0")
	}

	if d.Scrollable() {
		up := ctx.Sys.AddButton(win, 20, 197, art.ScrollUpUp.W, art.ScrollUpUp.H,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyPageUp, art.ScrollUpUp, art.ScrollUpDown, nil, 0)
		down := ctx.Sys.AddButton(win, 40, 197, art.ScrollDownUp.W, art.ScrollDownUp.H,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyPageDown, art.ScrollDownUp, art.ScrollDownDown, nil, 0)
		ctx.registerButtonSfx(up, "KCARG0")
		ctx.registerButtonSfx(down, "KCARG0")
	}

	ctx.Sys.MouseShow() // the in game caller hides the cursor handing off to a modal

	for {
		key := ctx.Sys.GetInput()
		if key < 0 {
			continue
		}
		result := d.HandleKey(key)
		if result != dialog.ResultNone {
			return result
		}
		if key == platform.KeySpace && mode == dialog.ModeOK {
			return dialog.ResultAccept
		}
		if key == platform.KeyPageUp || key == platform.KeyPageDown {
			redraw()
		}
	}
}

type dialogArtBuffers = dialog.Art

// loadDialogArt loads the art needed by the vanilla style dialog renderer.
func loadDialogArt(ctx *Context) (dialogArtBuffers, error) {
	load := func(tag string) (*gfx.Buffer, error) {
		img, err := ctx.loadSimple(tag)
		if err != nil {
			return nil, err
		}
		return img.Buffer().Clone(), nil
	}
	var art dialogArtBuffers
	var err error
	frame, err := ctx.loadBig("HELPFRAM")
	if err != nil {
		return art, err
	}
	art.Frame = frame.Buffer().Clone()
	if art.AcceptUp, err = load("HELPOK_U"); err != nil {
		return art, err
	}
	if art.AcceptDown, err = load("HELPOK_D"); err != nil {
		return art, err
	}
	if art.CancelUp, err = load("XFRCAN_U"); err != nil {
		return art, err
	}
	if art.CancelDown, err = load("XFRCAN_D"); err != nil {
		return art, err
	}
	if art.ScrollUpUp, err = load("BLDUP__U"); err != nil {
		return art, err
	}
	if art.ScrollUpDown, err = load("BLDUP__D"); err != nil {
		return art, err
	}
	if art.ScrollDownUp, err = load("BLDDWN_U"); err != nil {
		return art, err
	}
	if art.ScrollDownDown, err = load("BLDDWN_D"); err != nil {
		return art, err
	}
	return art, nil
}

// dAcceptLabel resolves the primary button label for a dialog mode.
func dAcceptLabel(ctx *Context, mode dialog.Mode) string {
	if mode == dialog.ModeOKCancel {
		return ctx.text("755f", "OK")
	}
	return ctx.text("ae0b", "OK")
}

// captionDialogButton draws a centered label on both dialog button states.
func captionDialogButton(up, down *gfx.Buffer, label string, f *fonts.Font) (*gfx.Buffer, *gfx.Buffer) {
	up = up.Clone()
	down = down.Clone()
	tw := f.Width(label)
	textX := (up.W - tw) / 2
	textY := (up.H - f.Height) / 2
	if textX < 0 {
		textX = 0
	}
	if textY < 0 {
		textY = 0
	}
	f.BlitShadowed(up.Sub(textX, textY, up.W-textX, up.H-textY), label, 0, fonts.ColorGold)
	f.BlitShadowed(down.Sub(textX, textY, down.W-textX, down.H-textY), label, 0, fonts.ColorDarkOrange)
	return up, down
}

// fallbackMessageBox shows a minimal message box when dialog art is unavailable.
func fallbackMessageBox(ctx *Context, text string) {
	const w, h = 340, 110
	screenW, screenH := ctx.Sys.ScreenSize()
	win := ctx.Sys.AddWindow((screenW-w)/2, (screenH-h)/2,
		w, h, 32, gnw.WindowMoveOnTop)
	defer ctx.Sys.DeleteWindow(win)

	ctx.Sys.ShadedBox(win, 0, 0, w-1, h-1, colorWhite, 0)
	ctx.Sys.ShadedBox(win, 1, 1, w-2, h-2, colorWhite, 0)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	ctx.printWrappedIn(win, text, gfx.NewRect(20, 16, w-21, h-44), colorWhite)
	ctx.Sys.AddTextButton(win, w/2-30, h-36,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyReturn, "OK", 0)

	for {
		switch ctx.Sys.GetInput() {
		case platform.KeyReturn, platform.KeyEscape, platform.KeySpace:
			return
		}
	}
}

// blitTextLine draws one line in the classic GNW shadowed caption style: the
// base color with its bevel outline and drop shadow, optionally centered in w.
// The caller holds the gnw lock.
// C++: Text_TextLine (text.cpp), default color Fonts_GoldColor.
func blitTextLine(buf *gfx.Buffer, f *fonts.Font, s string, x, y, w int, color fonts.Color, centered bool) {
	if s == "" {
		return
	}
	if centered {
		if tw := f.Width(s) + 1; tw < w {
			x += (w - tw) / 2
		}
	}
	aw := min(w, buf.W-x)
	ah := buf.H - y
	if aw <= 0 || ah <= 0 || x < 0 || y < 0 {
		return
	}
	f.BlitShadowed(buf.Sub(x, y, aw, ah), s, aw, color)
}

// blitTextBoxShadowed draws word wrapped shadowed text vertically centered in
// the (x,y,w,h) box, each line in the blitTextLine style. The caller holds the
// gnw lock. C++: Text_TextBox(WindowInfo*, ..., FontColor) (text.cpp).
func blitTextBoxShadowed(buf *gfx.Buffer, f *fonts.Font, s string, x, y, w, h int, color fonts.Color, centered bool) {
	if s == "" {
		return
	}
	lineH := f.Height + 1
	maxRows := h / lineH
	if maxRows < 1 {
		return
	}
	lines := wrapLines(f, s, w, maxRows)
	ty := y + (h-len(lines)*lineH)/2
	if ty < y {
		ty = y
	}
	for _, ln := range lines {
		blitTextLine(buf, f, ln, x, ty, w, color, centered)
		ty += lineH
	}
}
