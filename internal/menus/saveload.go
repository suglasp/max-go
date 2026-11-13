// saveload.go : implements the save-slot browser for saving and loading games.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: saveloadmenu.cpp - SaveLoadMenu
package menus

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/flic"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// Save/Load uses the LOADPIC brain-jar file menu with ten radio slots per page, paging controls, and Save, Quit, Return, Help, and Load actions. SaveLoadOps provides file I/O.
//
// The FILESFLC brain jar animation floats over the static LOADPIC jar, advanced
// at ~12.5fps like the original.
//
// DEVIATIONS: Quit exits the dialog (quitting to the main menu needs a game to menu
// signaling). Help is unavailable.

// brainFlicMS is the brain animation's frame interval (12.5fps).
// C++: TIMER_FPS_TO_MS(12.5) (saveloadmenu.cpp).
const brainFlicMS = 80

// brainFlicX/Y is the FILESFLC origin in the window (WindowManager_ScaleU* at
// scale 1). C++: flicsmgr_construct(FILESFLC, ..., 256, 104) (saveloadmenu.cpp).
const (
	brainFlicX = 256
	brainFlicY = 104
)

// SaveLoadOps is the file side backend the Save/Load screen drives. Slot numbers
// are 1-based and absolute. Implemented by the game screen.
type SaveLoadOps interface {
	// SlotInfo returns the display fields for an absolute slot.
	SlotInfo(slot int) (fileName, saveName, typeName string, inUse bool)
	// Save writes the current game to the slot under the given name.
	Save(slot int, name string) error
	// Load replaces the current game from the slot.
	Load(slot int) error
}

// turnTimerProvider supplies the frozen top-right turn timer for in-game Save/Load. The main-menu load path does not provide one.
type turnTimerProvider interface {
	// TurnTimerSnapshot returns the seconds left in the turn and whether to draw it.
	TurnTimerSnapshot() (seconds int, show bool)
}

const (
	slMaxName   = 30 // save name character limit
	slPerPage   = 10
	slNameRed   = 0x01 // COLOR_RED: save name on the up (unselected) face
	slNameWhite = 0xFF // COLOR_WHITE: save name on the down (selected) face - the GNW list highlight
	slGreen     = colorGreen
	slBottomY   = 438
	slNameOffX  = 45 // save name field, slot relative
	slNameOffY  = 39
	slNameWidth = 143
	// slTimerNormal is the turn timer readout's normal colour: the original's literal
	// index 0xA2, which renders WHITE in the LOADPIC palette (it is NOT
	// COLOR_GREEN). C++: GameManager_DrawTimer's 0xA2.
	slTimerNormal = 0xA2
)

// slotWidget holds a built slot's button + geometry for the page.
type slotWidget struct {
	abs      int // absolute slot number
	ulx, uly int
	inUse    bool
	saveName string
	/* // note: unused code in Go port
	// bid, w and h were written and never read; the page finds a slot by its position.
	bid  gnw.ButtonID
	w, h int
	*/
}

// saveLoadMenu is the Save/Load screen state: the paging and selection, the
// composed slot widgets, the in slot name editor, and the brain jar animation
// with its loose file handle. C++: saveloadmenu.cpp - SaveLoadMenu.
type saveLoadMenu struct {
	ctx    *Context
	ops    SaveLoadOps
	saving bool

	firstSlot int // absolute number of the page's first slot
	sel       int // selected slot index on the page, -1 = none
	slots     []slotWidget

	slotBtns   []gnw.ButtonID
	bottomBtns []gnw.ButtonID
	textEdit   *gnw.TextEdit

	pic              *sprites.Big
	fnameUp, fnameDn *sprites.Simple
	ftypeUp, ftypeDn *sprites.Simple
	slotUps, slotDns [slPerPage]*sprites.Simple
	f5               *fonts.Font

	timerSecs int // frozen turn timer readout (in game only)
	showTimer bool

	brainName  string
	brain      *flic.Player
	brainFile  *os.File
	brainStamp uint64

	dbl menuDoubleClick
}

// RunSaveLoad runs the Save/Load screen. saving=true shows the Save Game menu
// (editable names + Save/Quit); false shows the Load Game menu. Returns when the
// player leaves (Return/Cancel/ESC, or a successful Load). quit is true when the
// player pressed Quit, which the in game caller treats as "abandon to main menu".
//
// C++: SaveLoadMenu_MenuLoop (saveloadmenu.cpp)
func RunSaveLoad(ctx *Context, ops SaveLoadOps, saving bool) (quit bool, err error) {
	pic, err := ctx.loadBig("LOADPIC")
	if err != nil {
		return false, err
	}
	fnameUp, _ := ctx.loadSimple("FNAME_UP")
	fnameDn, _ := ctx.loadSimple("FNAME_DN")
	ftypeUp, _ := ctx.loadSimple("FTYPE_UP")
	ftypeDn, _ := ctx.loadSimple("FTYPE_DN")
	// Each page position has its OWN plate art: the ten plates are pre rendered
	// over their patch of the LOADPIC background (the two columns differ in edge
	// shading and corner cuts) and GNW blits them opaque, so sharing one art
	// leaves mismatched corners on the right column.
	// C++: SaveLoadMenu_Init (FILE1_UP + i / FILE1_DN + i).
	var slotUps, slotDns [slPerPage]*sprites.Simple
	for i := 0; i < slPerPage; i++ {
		upTag, dnTag := slotArtTags(i)
		if slotUps[i], err = ctx.loadSimple(upTag); err != nil {
			return false, err
		}
		if slotDns[i], err = ctx.loadSimple(dnTag); err != nil {
			return false, err
		}
	}

	ctx.Plat.SetPalette(&pic.Palette) // LOADPIC carries its own palette

	// Pin Font1 for the plate button captions (Save/Quit/Cancel/?/Load); they use
	// the current font, which in game is Font5 (too large). Restore on exit.
	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &saveLoadMenu{
		ctx:       ctx,
		ops:       ops,
		saving:    saving,
		firstSlot: 1,
		sel:       -1,
		pic:       pic,
		fnameUp:   fnameUp,
		fnameDn:   fnameDn,
		ftypeUp:   ftypeUp,
		ftypeDn:   ftypeDn,
		slotUps:   slotUps,
		slotDns:   slotDns,
		f5:        ctx.Sys.Fonts().Get(fonts.Font5),
	}

	// Delete every registered button on exit, or they keep drawing (and flashing)
	// over whatever screen comes next. C++: SaveLoadMenu_MenuLoop button cleanup.
	defer m.deleteAllButtons()

	// The FILESFLC resource is not the FLC itself but a NUL terminated loose
	// filename (saveload.flc); the real animation is that file in the data dir
	// (same indirection as the corner portrait F_<tag> resources).
	m.brainName = looseFlicName(ctx.Lib, "FILESFLC")
	defer m.closeBrain()

	// Frozen turn timer readout (in game only). Read ONCE on entry so it does not
	// tick while the screen is up - the game clock is paused for the modal. The
	// main menu Load path's ops does not implement turnTimerProvider, so showTimer
	// stays false there and nothing draws. C++: GameManager_DrawTimer / the paused
	// GameManager_TurnTimerValue (saveloadmenu.cpp draws it each loop iteration).
	if provider, ok := ops.(turnTimerProvider); ok {
		m.timerSecs, m.showTimer = provider.TurnTimerSnapshot()
	}

	m.buildScreen()
	m.newBrain()
	ctx.Sys.MouseShow()

	m.brainStamp = platform.Ticks()
	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			m.tickIdle()
			continue
		}
		if done, quit := m.handleKey(key); done {
			return quit, nil
		}
	}
}

// tickIdle runs one idle frame of the file screen: it advances the brain
// animation and repaints the frozen turn timer and any open name field.
func (m *saveLoadMenu) tickIdle() {
	if now := platform.Ticks(); now-m.brainStamp >= brainFlicMS {
		m.brainStamp = now
		m.advanceBrain()
	}
	m.drawTurnTimer() // repaint the frozen readout (idempotent; survives page rebuilds)
	if m.textEdit != nil {
		// Keep the edited save name painted. The sticky slot button redraws its
		// blank down face after openNameEdit runs, which would wipe the name until
		// the first keystroke; repainting the field each idle frame keeps it visible.
		m.textEdit.Draw()
	}
}

// handleKey runs one input event of the file screen loop. done reports that the
// screen is finished, and quit that the player chose to abandon the game.
func (m *saveLoadMenu) handleKey(key int) (done, quit bool) {
	ctx, ops, saving := m.ctx, m.ops, m.saving

	// While editing a name, feed keys to the field first.
	if m.textEdit != nil {
		switch {
		case key == platform.KeyReturn:
			m.commitName()
			key = 1022 // Enter commits AND saves, like vanilla
		case key == platform.KeyEscape:
			m.commitName()
			key = -2
		case key >= 32 && key < 127, key == platform.KeyBackspace:
			m.textEdit.HandleKey(key)
			key = -2
		}
		if key == -2 {
			return false, false
		}
	}

	// Slot selection (1001..1010).
	if key >= 1001 && key <= 1010 {
		m.sel = key - 1001
		if saving && m.sel < len(m.slots) {
			m.openNameEdit()
		} else if m.sel < len(m.slots) && m.dbl.Click(key, m.slots[m.sel].abs, platform.Ticks()) && m.loadSelected() {
			return true, false
		}
		return false, false
	}

	switch key {
	case 1100: // page up
		if m.firstSlot > slPerPage {
			m.firstSlot -= slPerPage
			m.buildScreen()
			m.newBrain()
		}
	case 1101: // page down
		m.firstSlot += slPerPage
		m.buildScreen()
		m.newBrain()
	case 1022: // Save
		if saving && m.sel >= 0 && m.sel < len(m.slots) {
			m.commitName()
			name := m.slots[m.sel].saveName
			if err := ops.Save(m.slots[m.sel].abs, name); err != nil {
				Message(ctx, "Save failed: "+err.Error())
			}
			m.buildScreen() // refresh the slot to show it now in use
			m.newBrain()
		}
	case 1023: // Load - active in BOTH modes (vanilla's file screen Saves OR Loads);
		// the in game FILES button opens this in save mode, where Load must still work.
		if m.loadSelected() {
			return true, false
		}
	case 1021, '?': // Help
		// The file screen is one screen in two moods, and the help database has a
		// section for each. C++: saveloadmenu.cpp:454.
		if m.saving {
			Help(ctx, "SAVELOAD_SETUP")
		} else {
			Help(ctx, "LOAD_SETUP")
		}
	case 1024: // Quit: abandon the game to the main menu
		return true, true
	case 1000, platform.KeyEscape: // Cancel / Return
		return true, false
	}
	return false, false
}

// deleteAllButtons removes the page's slot + bottom row buttons, or they keep
// drawing (and flashing) over whatever screen comes next.
func (m *saveLoadMenu) deleteAllButtons() {
	for _, b := range m.slotBtns {
		m.ctx.Sys.DeleteButton(b)
	}
	for _, b := range m.bottomBtns {
		m.ctx.Sys.DeleteButton(b)
	}
}

// composeSlot bakes the slot number, file name, type and save name into the
// up/down button faces of the given page position. In save mode the down face
// leaves the name area blank so the in slot text edit draws cleanly over it.
func (m *saveLoadMenu) composeSlot(pos, num int, fileName, saveName, typeName string, inUse bool) (*gfx.Buffer, *gfx.Buffer) {
	up := m.slotUps[pos].Buffer().Clone()
	dn := m.slotDns[pos].Buffer().Clone()
	f1 := m.ctx.Sys.Fonts().Get(fonts.Font1)
	f2 := m.ctx.Sys.Fonts().Get(fonts.Font2)
	numStr := strconv.Itoa(num)
	for _, img := range []*gfx.Buffer{up, dn} {
		blitInBox(img, f1, numStr, 0, 0, 40, img.H, 0xA2, true) // slot number
	}
	if inUse {
		blitField(up, m.fnameUp, fileName, m.f5, false)
		blitField(dn, m.fnameDn, fileName, m.f5, false)
		blitField(up, m.ftypeUp, typeName, f2, true)
		blitField(dn, m.ftypeDn, typeName, f2, true)
		blitInBox(up, m.f5, saveName, slNameOffX, slNameOffY, slNameWidth, m.f5.Height+2, slNameRed, false)
		if !m.saving { // load mode bakes the name on the down (selected) face too, in WHITE:
			// the standard GNW list highlight (red normally, white when that slot is selected).
			blitInBox(dn, m.f5, saveName, slNameOffX, slNameOffY, slNameWidth, m.f5.Height+2, slNameWhite, false)
		}
	} else {
		blitField(up, m.fnameUp, "", m.f5, false)
		blitField(dn, m.fnameDn, "", m.f5, false)
	}
	return up, dn
}

// addImg / addCap add one bottom row button (arrow art, or a labelled
// plate with a press sound); buildScreen calls them on every page rebuild.
func (m *saveLoadMenu) addImg(x int, up, down string, value int) {
	if b, e := m.ctx.imageButton(mainWindow, x, slBottomY, up, down, "", 0, 0, false, value); e == nil {
		m.bottomBtns = append(m.bottomBtns, b)
	}
}

// addCap adds one labelled bottom plate button with a press sound.
func (m *saveLoadMenu) addCap(x int, up, down, label string, value int, sfx string) {
	if b, e := m.ctx.plateButton(mainWindow, x, slBottomY, up, down, label, value); e == nil {
		m.ctx.registerButtonSfx(b, sfx)
		m.bottomBtns = append(m.bottomBtns, b)
	}
}

// buildScreen (re)draws the whole page: background, title, slot buttons and
// the bottom button row. C++: SaveLoadMenu_Init.
func (m *saveLoadMenu) buildScreen() {
	m.deleteAllButtons()
	m.slotBtns = m.slotBtns[:0]
	m.bottomBtns = m.bottomBtns[:0]
	m.slots = m.slots[:0]
	m.sel = -1
	m.textEdit = nil

	m.ctx.drawBigAt(m.pic, 0, 0)

	// Title.
	title := m.ctx.text("63e7", "Load Game Menu")
	if m.saving {
		title = m.ctx.text("5426", "Save Game Menu")
	}
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	// Plain flat green like every menu title. C++: Text_TextBox(..., COLOR_GREEN).
	blitInBox(buf, m.f5, title, 229, 5, 181, 21, slGreen, true)
	m.ctx.Sys.Unlock()

	// 10 slots: 5 rows x 2 columns.
	for i := 0; i < slPerPage; i++ {
		abs := m.firstSlot + i
		fileName, saveName, typeName, inUse := m.ops.SlotInfo(abs)
		up, dn := m.composeSlot(i, abs, fileName, saveName, typeName, inUse)
		ulx := 402*(i/5) + 16
		uly := 76*(i%5) + 44
		bid := m.ctx.Sys.AddButton(mainWindow, ulx, uly, up.W, up.H,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, 1001+i, up, dn, nil, gnw.ButtonSticky)
		if bid < 0 {
			continue
		}
		m.ctx.registerButtonSfx(bid, "KCARG0")
		m.slotBtns = append(m.slotBtns, bid)
		m.slots = append(m.slots, slotWidget{ /* bid: bid: note: unused code in Go port, see slotWidget */ abs: abs, ulx: ulx, uly: uly /* , w: up.W, h: up.H: note: unused code in Go port, see slotWidget */, inUse: inUse, saveName: saveName})
	}
	m.ctx.Sys.GroupRadioButtons(m.slotBtns, nil)

	// Bottom buttons. Arrows first, then the labelled row.
	m.addImg(33, "MNUUAROU", "MNUUAROD", 1100) // page up
	m.addImg(63, "MNUDAROU", "MNUDAROD", 1101) // page down
	m.addCap(514, "MNUBTN6U", "MNUBTN6D", m.ctx.text("f846", "Load"), 1023, "FLOAD")
	m.addCap(465, "MNUBTN5U", "MNUBTN5D", m.ctx.text("278f", "?"), 1021, "FHELP")
	// Opened in game (Save mode) the leftmost button reads "Return"; the main menu
	// load only path reads "Cancel". C++: saveloadmenu.cpp:261 - the button
	// caption is is_saving_allowed ? _(6b01 "Return") : _(f752 "Cancel").
	cancelLabel := m.ctx.text("f752", "Cancel")
	if m.saving {
		cancelLabel = m.ctx.text("6b01", "Return")
	}
	m.addCap(354, "MNUBTN4U", "MNUBTN4D", cancelLabel, 1000, "FCANC")
	if m.saving {
		m.addCap(243, "MNUBTN3U", "MNUBTN3D", m.ctx.text("d4ac", "Quit"), 1024, "FQUIT")
		m.addCap(132, "MNUBTN2U", "MNUBTN2D", m.ctx.text("1509", "Save"), 1022, "FSAVE")
	}
}

// openNameEdit starts editing the selected slot's save name (save mode).
func (m *saveLoadMenu) openNameEdit() {
	if m.sel < 0 || m.sel >= len(m.slots) {
		return
	}
	sw := m.slots[m.sel]
	te := m.ctx.Sys.NewTextEdit(mainWindow, sw.ulx+slNameOffX, sw.uly+slNameOffY,
		slNameWidth, slMaxName, slNameRed, fonts.Font5)
	te.SetText(sw.saveName)
	te.SetActive(true) // white while editing (gnw TextEdit)
	m.textEdit = te
}

// commitName stores the edited save name back into its slot and closes the field.
func (m *saveLoadMenu) commitName() {
	if m.textEdit != nil {
		m.slots[m.sel].saveName = m.textEdit.Text()
		m.textEdit.SetActive(false)
		m.textEdit = nil
	}
}

// closeBrain stops the brain jar animation and closes its loose file handle.
func (m *saveLoadMenu) closeBrain() {
	m.brain = nil
	if m.brainFile != nil {
		m.brainFile.Close()
		m.brainFile = nil
	}
}

// newBrain (re)opens the brain jar animation floating over the static LOADPIC
// jar. The original reconstructs the flic inside SaveLoadMenu_Init (per page),
// so newBrain is called after each buildScreen; a missing file just leaves the
// static jar.
func (m *saveLoadMenu) newBrain() {
	m.closeBrain()
	f := openLooseFile(m.ctx.DataDir, m.brainName)
	if f == nil {
		return
	}
	a, err := flic.Open(f) // seeks to 0 + reads header for the native size
	if err != nil {
		f.Close()
		return
	}
	rect := gfx.NewRect(brainFlicX, brainFlicY, brainFlicX+a.W()-1, brainFlicY+a.H()-1)
	p, e := flic.NewPlayer(f, rect, true) // reset() Opens again (seeks to 0)
	if e != nil {
		f.Close()
		return
	}
	m.brainFile = f
	m.brain = p
}

// advanceBrain draws the brain jar animation's next frame.
func (m *saveLoadMenu) advanceBrain() {
	if m.brain == nil {
		return
	}
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	_ = m.brain.DrawNext(buf)
	m.ctx.Sys.Unlock()
}

// drawTurnTimer paints the MM:SS readout in the top right corner (524,10) in
// FONT5, white normally and red under 20s, centred in a 62x24 box - the same
// corner as the in game readout. It is repainted each idle frame so a page
// change / save refresh (which redraws the whole LOADPIC) keeps it.
func (m *saveLoadMenu) drawTurnTimer() {
	if !m.showTimer {
		return
	}
	// The original draws index 0xA2 normally (white in the LOADPIC palette, NOT
	// COLOR_GREEN=0x02) and COLOR_RED under 20s. C++: GameManager_DrawTimer.
	color := slTimerNormal
	if m.timerSecs <= 20 { // COLOR_RED warning under 20s, matching the live readout
		color = slNameRed
	}
	text := fmt.Sprintf("%02d:%02d", m.timerSecs/60, m.timerSecs%60)
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	blitInBox(buf, m.f5, text, 524, 10, 62, 24, color, true)
	m.ctx.Sys.Unlock()
}

// loadSelected loads the selected in use slot; false when nothing loaded.
func (m *saveLoadMenu) loadSelected() bool {
	if m.sel < 0 || m.sel >= len(m.slots) || !m.slots[m.sel].inUse {
		return false
	}
	if m.textEdit != nil { // discard any save name edit still in progress
		m.textEdit.SetActive(false)
		m.textEdit = nil
	}
	if err := m.ops.Load(m.slots[m.sel].abs); err != nil {
		Message(m.ctx, "Load failed: "+err.Error())
		return false
	}
	return true
}

// slotArtTags returns the up/down plate art tags for a page position (0-9).
// Slot 10's tags drop the underscore: RES tags cap at 8 characters.
// C++: ResourceID FILE1_UP + i / FILE1_DN + i (saveloadmenu.cpp).
func slotArtTags(pos int) (up, dn string) {
	if pos == 9 {
		return "FILE10UP", "FILE10DN"
	}
	n := strconv.Itoa(pos + 1)
	return "FILE" + n + "_UP", "FILE" + n + "_DN"
}

// looseFlicName reads a RES resource that holds a NUL terminated loose .FLC
// filename (e.g. FILESFLC -> "saveload.flc") and returns the trimmed name, or ""
// when the resource is absent. C++: the loose flic ResourceID indirection.
func looseFlicName(lib interface{ Read(string) ([]byte, error) }, tag string) string {
	raw, err := lib.Read(tag)
	if err != nil {
		return ""
	}
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(string(raw))
}

// openLooseFile opens a loose data dir file by name, trying the name as is and
// upper/lower case (the data dir filenames are upper case on disk, but the
// resource stores lower case). Returns nil on any failure.
func openLooseFile(dir, name string) *os.File {
	if dir == "" || name == "" {
		return nil
	}
	for _, n := range []string{name, strings.ToUpper(name), strings.ToLower(name)} {
		if f, err := os.Open(filepath.Join(dir, n)); err == nil {
			return f
		}
	}
	return nil
}

// blitField copies a name/type field background image into the slot face at its
// own (ULX,ULY) hotspot, then blits the title text inside it (green). FONT2
// titles centre; FONT5 titles inset 5px. Caller does not hold the gnw lock.
// C++: SaveLoadMenu_DrawSaveSlotResource.
func blitField(dst *gfx.Buffer, field *sprites.Simple, title string, f *fonts.Font, centered bool) {
	if field == nil {
		return
	}
	fb := field.Buffer()
	x, y := field.ULX, field.ULY
	w := min(fb.W, dst.W-x)
	h := min(fb.H, dst.H-y)
	if w > 0 && h > 0 && x >= 0 && y >= 0 {
		dst.Sub(x, y, w, h).Copy(fb.Sub(0, 0, w, h))
	}
	if title == "" {
		return
	}
	if centered {
		blitInBox(dst, f, title, x, y, fb.W, fb.H, slGreen, true)
	} else {
		blitInBox(dst, f, title, x+5, y, fb.W-10, fb.H, slGreen, false)
	}
}
