// options.go : implements audio, display, player-name, and input settings screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: optionsmenu.cpp - OptionsMenu
package menus

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/sprites"
)

// PREFSPIC uses the original panel, slider, checkbox, edit-box, and button art in a centred game window. It binds player_name to one Player.Name and omits networking, per-team names, and Help.

// optKind is the control type, mirroring OptionsType (optionsmenu.hpp).
type optKind int

const (
	optSection optKind = iota
	optSlider
	optCheckbox
	optEditInt
	optEditStr
	optLabel
)

// optGold is the gold text color (the C++ 0xA2 used for option captions).
const optGold = 0xA2

// optControl is one row of the Preferences table (OptionsButton in the C++),
// with its resolved layout box and value binding into the working copy config.
type optControl struct {
	kind     optKind
	key      string // setting key (drives behavior + ordering)
	label    string // resolved caption string
	ulx, uly int    // window relative position (uly filled in by layout)
	rmin     int
	rmax     int

	getV func() int    // value for slider/int/checkbox(0/1)
	setV func(int)     // store value
	getS func() string // string for str edit / label text

	// Runtime widget state filled while building the screen.
	bid          gnw.ButtonID
	slitX, slitW int // slider track origin/width (window coords)
	editX, editW int // edit box origin/width

	bg       *gfx.Buffer // snapshot of the panel under the track/box (art, not knob/value), restored before redraw so it does not ghost. C++: OptionsButton::image
	bgX, bgY int
}

// RunOptions runs the in-game Preferences dialog. Done writes settings and applies audio live; Cancel restores edited values. The game remains visible behind it.
// Requires ctx.Settings. C++: OptionsMenu(team, PREFSPIC).Run (optionsmenu.cpp).
func RunOptions(ctx *Context, center gfx.Rect) error {
	return runOptionsScreen(ctx, center, "PREFSPIC", 383, 20)
}

// RunSetup is the standalone main menu Setup screen (SETUPPIC): the same controls
// as the in game Preferences, but on its own full screen background, with the
// setup button row (y=141) and the higher layout origin, and no drawn title (the
// SETUPPIC art carries its own). C++: OptionsMenu(team, SETUPPIC).Run().
func RunSetup(ctx *Context, center gfx.Rect) error {
	return runOptionsScreen(ctx, center, "SETUPPIC", 141, -20)
}

// optionsMenu is the shared Preferences/Setup screen state: the working copy
// settings the control bindings edit, the resolved control rows, and the open
// edit field. wc is bound by pointer into the optionsControls getV/setV
// closures, so it must never be copied after construction.
// C++: optionsmenu.cpp - OptionsMenu.
type optionsMenu struct {
	ctx       *Context
	win       gnw.WinID
	ox        int // window left edge, to map slider clicks
	showTitle bool

	orig settings.Settings // for Cancel (revert) + live volume restore
	wc   settings.Settings // working copy edited by the controls

	controls []optControl
	slide    *sprites.Simple
	f5       *fonts.Font

	textEdit *gnw.TextEdit
	editIdx  int
}

// runOptionsScreen renders the shared options screen: bg is the background image
// (PREFSPIC in game / SETUPPIC at the menu), btnY the Done/Cancel/Help row, and
// startY the layout origin. C++: OptionsMenu(team, bg_image).Run (optionsmenu.cpp).
func runOptionsScreen(ctx *Context, center gfx.Rect, bg string, btnY, startY int) error {
	showTitle := bg == "PREFSPIC"
	cfg := ctx.Settings
	if cfg == nil {
		return nil
	}
	m := &optionsMenu{
		ctx:       ctx,
		showTitle: showTitle,
		orig:      *cfg, // for Cancel (revert) + live volume restore
		wc:        *cfg, // working copy edited by the controls
		editIdx:   -1,
	}
	m.wc.Validate() // sane ranges before display

	pic, err := ctx.loadBig(bg)
	if err != nil {
		return err
	}
	slit, err := ctx.loadSimple("PRFSLIT")
	if err != nil {
		return err
	}
	slide, err := ctx.loadSimple("PRFSLIDE")
	if err != nil {
		return err
	}

	// plateButton bakes the Done/Cancel/Help captions in the current font. The menu default
	// Font1 overflows the small PRFDNE/PRFCAN/PRFHLP buttons, so pin the body's Font5 and
	// restore on exit. (Vanilla uses FONT_1; our FONT1 renders taller, so FONT5 fits, DEVIATION.)
	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	// Centre the panel in the given window (C++ Window(id, win_id) geometry).
	ox := max(center.ULX+(center.Width()+1-pic.W)/2, 0)
	oy := max(center.ULY+(center.Height()+1-pic.H)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, pic.W, pic.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("menus: options window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)
	m.win = win
	m.ox = ox
	m.slide = slide

	// Paint the panel background into the dialog window. Resolve the window
	// buffer BEFORE locking: Buf() locks the same mutex (D60 deadlock lesson).
	winBuf := ctx.Sys.Buf(win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, pic.W, pic.H).Copy(pic.Buffer().Sub(0, 0, pic.W, pic.H))
	ctx.Sys.Unlock()

	m.controls = optionsControls(ctx, &m.wc)
	if !showTitle { // SETUPPIC: only the first ("setup") section, audio, auto-save,
		m.controls = setupSectionOnly(m.controls) // player name; preferences/options are in game only (C++ Init)
	}
	layoutOptions(ctx, m.controls, startY)

	m.f5 = ctx.Sys.Fonts().Get(fonts.Font5)
	controls := m.controls

	buttons, err := m.buildControls(slit, btnY)
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()
	if err != nil {
		return err
	}

	drawOptionsLabels(ctx, win, controls, showTitle)
	for i := range controls {
		if controls[i].kind == optSlider {
			drawSlider(ctx, win, &controls[i], slide)
		}
	}
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if m.handleKey(key) {
			return nil
		}
	}
}

// buildControls creates the checkbox, slider and edit field hit widgets plus the
// bottom Cancel/Help/Done row, returning the ids so the caller can delete them on
// exit. slit is the slider track art and btnY the bottom row's window y.
func (m *optionsMenu) buildControls(slit *sprites.Simple, btnY int) ([]gnw.ButtonID, error) {
	ctx, win, f5 := m.ctx, m.win, m.f5
	controls := m.controls
	var buttons []gnw.ButtonID

	// Build the checkbox / slider / edit hit widgets.
	for i := range controls {
		c := &controls[i]
		switch c.kind {
		case optCheckbox:
			up, e1 := ctx.loadSimple("UNCHKED")
			down, e2 := ctx.loadSimple("CHECKED")
			if e1 != nil || e2 != nil {
				continue
			}
			bid := ctx.Sys.AddButton(win, c.ulx, c.uly, up.Buffer().W, up.Buffer().H,
				gnw.NoValue, gnw.NoValue, gnw.NoValue, 1002+i,
				up.Buffer().Clone(), down.Buffer().Clone(), nil, gnw.ButtonSticky)
			if bid >= 0 {
				ctx.Sys.SetRestState(bid, c.getV() != 0)
				ctx.registerButtonSfx(bid, "MBUTT0")
				c.bid = bid
				buttons = append(buttons, bid)
			}
		case optSlider:
			c.slitW = slit.Buffer().W
			c.slitX = c.ulx + f5.Width(c.label) + 10
			region := ctx.Sys.AddButton(win, c.slitX-10, c.uly, c.slitW+20, 20,
				gnw.NoValue, gnw.NoValue, gnw.NoValue, 1002+i, nil, nil, nil, 0)
			if region >= 0 {
				ctx.registerButtonSfx(region, "MBUTT0")
				c.bid = region
				buttons = append(buttons, region)
			}
			// Draw the slit track, then snapshot it so the knob can be erased.
			ctx.drawSimpleAt(win, slit, c.slitX, c.uly+(20-slit.Buffer().H)/2, true)
			c.bgX, c.bgY = c.slitX-10, c.uly
			c.bg = snapBox(ctx, win, c.bgX, c.bgY, c.slitW+20, 20)
		case optEditInt, optEditStr:
			art := "PREFEDIT"
			if c.kind == optEditStr {
				art = "PREFNAME"
			}
			img, e := ctx.loadSimple(art)
			if e != nil {
				continue
			}
			c.editX = c.ulx + f5.Width(c.label) + 10
			c.editW = img.Buffer().W
			ctx.drawSimpleAt(win, img, c.editX, c.uly+(20-img.Buffer().H)/2, true)
			region := ctx.Sys.AddButton(win, c.editX, c.uly, c.editW, 20,
				gnw.NoValue, gnw.NoValue, gnw.NoValue, 1002+i, nil, nil, nil, 0)
			if region >= 0 {
				c.bid = region
				buttons = append(buttons, region)
			}
			// Snapshot the box art so the value can be erased before each redraw.
			c.bgX, c.bgY = c.editX, c.uly
			c.bg = snapBox(ctx, win, c.bgX, c.bgY, c.editW, 20)
		}
	}

	// Bottom Cancel / Help / Done buttons (window y from the caller: 383 in game,
	// 141 for the main menu setup screen).
	mkBtn := func(x int, up, down, label string, value int, sfx string) error {
		bid, e := ctx.plateButton(win, x, btnY, up, down, label, value)
		if e != nil {
			return e
		}
		ctx.registerButtonSfx(bid, sfx)
		buttons = append(buttons, bid)
		return nil
	}
	if err := mkBtn(125, "PRFCAN_U", "PRFCAN_D", ctx.text("3fda", "Cancel"), 1001, "NCANC0"); err != nil {
		return buttons, err
	}
	if err := mkBtn(188, "PRFHLP_U", "PRFHLP_D", "", 1049, "NHELP0"); err != nil {
		return buttons, err
	}
	if err := mkBtn(215, "PRFDNE_U", "PRFDNE_D", ctx.text("6907", "Done"), 1000, "NDONE0"); err != nil {
		return buttons, err
	}
	return buttons, nil
}

// handleKey runs one input event of the options loop, returning true when the
// screen is finished (Done has already written and applied the settings).
func (m *optionsMenu) handleKey(key int) bool {
	ctx := m.ctx
	cfg := ctx.Settings
	controls := m.controls

	// An open edit field consumes input first.
	if m.textEdit != nil {
		c := &controls[m.editIdx]
		switch {
		case key == platform.KeyReturn || key == platform.KeyEscape:
			m.commitEdit()
		case c.kind == optEditInt && key >= '0' && key <= '9':
			m.textEdit.HandleKey(key)
		case c.kind == optEditStr && key >= 32 && key < 127:
			m.textEdit.HandleKey(key)
		case key == platform.KeyBackspace:
			m.textEdit.HandleKey(key)
		case key >= 1000:
			m.commitEdit() // clicking elsewhere commits
		}
		if key < 1000 {
			return false
		}
	}

	switch {
	case key == 1000 || key == platform.KeyReturn:
		m.commitEdit()
		commitOptions(ctx, controls, &m.wc)
		*cfg = m.wc
		cfg.Validate()
		if ctx.SettingsPath != "" {
			_ = cfg.Save(ctx.SettingsPath)
		}
		if ctx.Audio != nil {
			ctx.Audio.SetVolumes(cfg.Audio)
		}
		return true
	case key == 1001 || key == platform.KeyEscape:
		if ctx.Audio != nil {
			ctx.Audio.SetVolumes(m.orig.Audio) // revert live volume changes
		}
		return true
	case key == 1049 || key == '?':
		// The same controls serve the main menu Setup screen and the in game
		// Preferences, and the help database has a section for each.
		// C++: optionsmenu.cpp:478.
		if m.showTitle {
			Help(ctx, "PREFS_MENU_SETUP")
		} else {
			Help(ctx, "SETUP_MENU_SETUP")
		}
	case key >= 1002 && key-1002 < len(controls):
		i := key - 1002
		c := &controls[i]
		switch c.kind {
		case optCheckbox:
			c.setV(boolToInt(ctx.Sys.ButtonChecked(c.bid)))
			applyAudioLive(ctx, &m.wc)
		case optSlider:
			v := sliderValue(ctx.Plat.Mouse().X-m.ox, c.slitX, c.slitW, c.rmax)
			if v < c.rmin {
				v = c.rmin
			}
			c.setV(v)
			drawSlider(ctx, m.win, c, m.slide)
			applyAudioLive(ctx, &m.wc)
		case optEditInt, optEditStr:
			m.commitEdit()
			m.openEdit(i)
		}
	}
	return false
}

// commitEdit closes the open edit field, clamping and storing an int value into
// its control; a string field parks its text for the Done commit (player_name).
func (m *optionsMenu) commitEdit() {
	if m.textEdit == nil {
		return
	}
	c := &m.controls[m.editIdx]
	if c.kind == optEditInt {
		v, _ := strconv.Atoi(m.textEdit.Text())
		if v < c.rmin {
			v = c.rmin
		}
		if v > c.rmax {
			v = c.rmax
		}
		c.setV(v)
	}
	// optEditStr writes its setting at commit time on Done (player_name).
	if c.kind == optEditStr {
		editStrText[c.key] = m.textEdit.Text()
	}
	m.textEdit.SetActive(false)
	m.textEdit = nil
	m.editIdx = -1
	drawOptionsLabels(m.ctx, m.win, m.controls, m.showTitle)
}

// openEdit opens the text edit field for control i at the exact spot
// drawOptionsLabels centers the static value, so clicking does not move it.
func (m *optionsMenu) openEdit(i int) {
	c := &m.controls[i]
	m.editIdx = i
	val := strconv.Itoa(c.getV())
	if c.kind == optEditStr {
		val = editStrText[c.key]
	}
	restoreBox(m.ctx, m.win, c.bg, c.bgX, c.bgY) // clean box art under the field
	tx := c.editX
	if w := m.f5.Width(val); w < c.editW {
		tx += (c.editW - w) / 2
	}
	ty := c.uly + (20-m.f5.Height)/2
	te := m.ctx.Sys.NewTextEdit(m.win, tx, ty, c.editW-(tx-c.editX), 28, optGold, fonts.Font5)
	te.SetText(val)
	te.SetActive(true) // active => highlights white (gnw TextEdit)
	m.textEdit = te
}

// editStrText holds the in progress text for string edit fields by key, so the
// player name survives between opening the field and committing it.
var editStrText = map[string]string{}

// boolToInt maps true/false to 1/0.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sliderValue maps a window relative mouse x on a slider track to its value,
// the inverse of DrawSlider's positioning (optionsmenu.cpp UpdateSlider).
func sliderValue(mouseX, slitX, slitW, rmax int) int {
	if slitW <= 0 {
		return 0
	}
	v := (mouseX - slitX) * rmax / slitW
	if v < 0 {
		v = 0
	}
	if v > rmax {
		v = rmax
	}
	return v
}

// optionsControls builds the in game Preferences control table bound to the
// working copy settings wc, in the order and with the keys of the C++ table
// (optionsmenu.hpp), up to the "invalid" section where the in game menu stops.
func optionsControls(ctx *Context, wc *settings.Settings) []optControl {
	t := func(k, fb string) string { return ctx.text(k, fb) }
	boolV := func(p *bool) (func() int, func(int)) {
		return func() int { return boolToInt(*p) }, func(v int) { *p = v != 0 }
	}
	// disable_* checkboxes are checked when the channel is OFF.
	disV := func(p *bool) (func() int, func(int)) {
		return func() int { return boolToInt(!*p) }, func(v int) { *p = v == 0 }
	}
	intV := func(p *int) (func() int, func(int)) {
		return func() int { return *p }, func(v int) { *p = v }
	}
	mk := func(kind optKind, key, label string, ulx, rmin, rmax int, g func() int, s func(int)) optControl {
		return optControl{kind: kind, key: key, label: label, ulx: ulx, rmin: rmin, rmax: rmax, getV: g, setV: s}
	}

	gMusic, sMusic := intV(&wc.Audio.MusicVolume)
	gFx, sFx := intV(&wc.Audio.SfxVolume)
	gVoice, sVoice := intV(&wc.Audio.VoiceVolume)
	gDM, sDM := disV(&wc.Audio.MusicEnabled)
	gDF, sDF := disV(&wc.Audio.SfxEnabled)
	gDV, sDV := disV(&wc.Audio.VoiceEnabled)
	gAuto, sAuto := boolV(&wc.Game.AutoSave)
	gEff, sEff := boolV(&wc.Prefs.Effects)
	gClick, sClick := boolV(&wc.Prefs.ClickScroll)
	gFast, sFast := boolV(&wc.Prefs.FastMovement)
	gFollow, sFollow := boolV(&wc.Prefs.FollowUnit)
	gHalt, sHalt := boolV(&wc.Prefs.EnemyHalt)
	gSel, sSel := boolV(&wc.Prefs.AutoSelect)
	gScroll, sScroll := intV(&wc.Prefs.QuickScroll)
	gTimer, sTimer := intV(&wc.Game.TurnTimer)
	gEnd, sEnd := intV(&wc.Game.EndTurn)

	editStrText["player_name"] = wc.Player.Name

	controls := []optControl{
		{kind: optSection, key: "setup"},
		{kind: optLabel, key: "music_level", label: t("b2c7", "Volume:"), ulx: 25},
		// enhanced_graphics (id 2) is SETUPPIC only, omitted in game.
		mk(optSlider, "music_level", t("6eb0", "Music"), 25, 0, 100, gMusic, sMusic),
		mk(optCheckbox, "disable_music", t("c2e5", "Disable Music"), 210, 0, 1, gDM, sDM),
		mk(optSlider, "fx_sound_level", t("adf0", "FX"), 25, 0, 100, gFx, sFx),
		mk(optCheckbox, "disable_fx", t("2eeb", "FX Disabled"), 210, 0, 1, gDF, sDF),
		mk(optSlider, "voice_level", t("87bf", "Voice"), 25, 0, 100, gVoice, sVoice),
		mk(optCheckbox, "disable_voice", t("e9db", "Voice Disabled"), 210, 0, 1, gDV, sDV),
		mk(optCheckbox, "auto_save", t("64fb", "Auto-Save Enabled"), 25, 0, 1, gAuto, sAuto),
		// ipx_socket (id 10) is networking, omitted (PROJECT.txt).
		{kind: optEditStr, key: "player_name", label: t("f11f", "Player Name"), ulx: 25,
			getV: func() int { return 0 }, setV: func(int) {}},
		{kind: optSection, key: "preferences"},
		mk(optCheckbox, "effects", t("238c", "Animate Effects"), 25, 0, 1, gEff, sEff),
		mk(optCheckbox, "click_scroll", t("bf3b", "Click to Scroll"), 210, 0, 1, gClick, sClick),
		mk(optCheckbox, "fast_movement", t("2349", "Double Unit Steps"), 25, 0, 1, gFast, sFast),
		mk(optCheckbox, "follow_unit", t("09e9", "Track Selected Unit"), 210, 0, 1, gFollow, sFollow),
		mk(optCheckbox, "enemy_halt", t("f99e", "Halt movement when enemy is detected"), 25, 0, 1, gHalt, sHalt),
		mk(optCheckbox, "auto_select", t("c8b2", "Auto-Select Next Unit"), 210, 0, 1, gSel, sSel),
		mk(optSlider, "quick_scroll", t("24e6", "Scroll Speed"), 25, 4, 128, gScroll, sScroll),
		{kind: optSection, key: "options"},
		mk(optEditInt, "timer", t("86df", "Turn Time:"), 25, 0, 32767, gTimer, sTimer),
		mk(optEditInt, "endturn", t("de3f", "End Turn Time:"), 210, 0, 32767, gEnd, sEnd),
	}

	// The play_mode / opponent / victory labels are read only displays.
	playMode := []string{t("d0a7", "Turn Based"), t("ee25", "Simultaneous Moves")}
	opponents := []string{t("a24d", "Clueless"), t("37b7", "Apprentice"), t("bb85", "Average"),
		t("b663", "Expert"), t("ab89", "Master"), t("c7e3", "God")}
	vtype := []string{t("b542", "turns"), t("c303", "points")}
	controls = append(controls,
		optControl{kind: optLabel, key: "play_mode", ulx: 25,
			getS: func() string { return fmt.Sprintf(t("f019", "Play Mode: %s"), pick(playMode, wc.Game.PlayMode)) }},
		optControl{kind: optLabel, key: "opponent", ulx: 25,
			getS: func() string {
				return fmt.Sprintf(t("d5b2", "Computer Player is: %s"), pick(opponents, wc.Game.Opponent))
			}},
		optControl{kind: optLabel, key: "victory_limit", ulx: 25,
			getS: func() string {
				// The lang format uses C's %i for the integer; Go's fmt needs %d.
				format := strings.ReplaceAll(t("e791", "Game ends at %d %s"), "%i", "%d")
				return fmt.Sprintf(format, wc.Game.VictoryLimit, pick(vtype, wc.Game.VictoryType))
			}},
	)
	return controls
}

// pick returns s[i] or "" if out of range.
func pick(s []string, i int) string {
	if i >= 0 && i < len(s) {
		return s[i]
	}
	return ""
}

// layoutOptions assigns each control its window relative uly, reproducing the row stepping
// of OptionsMenu::Init for PREFSPIC (left column ulx==25 advances a row by 20, right column
// shares it; sections add a 20px gap; labels take less vertical space).
// setupSectionOnly returns the controls up to the second section (only the first "setup"
// section). The main menu Setup screen (SETUPPIC) stops there; the preferences/options
// sections are in game (PREFSPIC) only. C++: OptionsMenu::Init.
func setupSectionOnly(controls []optControl) []optControl {
	sections := 0
	for i, c := range controls {
		if c.kind == optSection {
			if sections++; sections == 2 {
				return controls[:i]
			}
		}
	}
	return controls
}

func layoutOptions(ctx *Context, controls []optControl, startY int) {
	f5 := ctx.Sys.Fonts().Get(fonts.Font5)
	uly := startY
	for i := range controls {
		c := &controls[i]
		if c.kind == optSection {
			uly += 20
			continue
		}
		if c.ulx == 25 {
			uly += 20
		}
		c.uly = uly
		if c.kind == optLabel {
			uly -= 20 - (f5.Height + 3)
		}
	}
}

// drawOptionsLabels repaints every control's caption/value (the parts not owned
// by gnw button art). Called after the static art is in place and whenever a
// label or edit value changes.
func drawOptionsLabels(ctx *Context, win gnw.WinID, controls []optControl, showTitle bool) {
	buf := ctx.Sys.Buf(win)
	f5 := ctx.Sys.Fonts().Get(fonts.Font5)

	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()

	// Title "Preferences" (window box 108,12,184,17). PREFSPIC only; the SETUPPIC
	// background carries its own heading. Plain flat green like every menu title.
	// C++: Text_TextBox(..., COLOR_GREEN, true, true) (optionsmenu.cpp).
	if showTitle {
		blitInBox(buf, f5, ctx.text("f70f", "Preferences"), 108, 12, 184, 17, colorGreen, true)
	}

	// Control captions use the gold shadowed style (the WindowInfo Text_TextBox
	// default Fonts_GoldColor); the read only label rows use BRIGHT YELLOW, except
	// the "Volume:" header (key music_level) which stays gold. Edit VALUES stay
	// flat 0xA2 (the buffer Text_TextBox variant). C++: OptionsMenu::Init*Control.
	for i := range controls {
		c := &controls[i]
		switch c.kind {
		case optLabel:
			text := c.label
			if c.getS != nil {
				text = c.getS()
			}
			color := fonts.ColorBrightYellow
			if c.key == "music_level" {
				color = fonts.ColorGold
			}
			blitTextBoxShadowed(buf, f5, text, c.ulx, c.uly, buf.W-c.ulx-4, 20, color, false)
		case optSlider:
			blitTextBoxShadowed(buf, f5, c.label, c.ulx, c.uly, 185, 20, fonts.ColorGold, false)
		case optCheckbox:
			// 155x24 box holds up to two wrapped lines (e.g. the long "Halt
			// movement when enemy is detected"), matching the C++ Text_TextBox.
			blitTextBoxShadowed(buf, f5, c.label, c.ulx+25, c.uly-2, 155, 24, fonts.ColorGold, false)
		case optEditInt:
			blitTextBoxShadowed(buf, f5, c.label, c.ulx, c.uly, 185, 20, fonts.ColorGold, false)
			if c.editW > 0 && c.bg != nil {
				buf.Sub(c.bgX, c.bgY, c.bg.W, c.bg.H).Copy(c.bg) // clean box, no ghost
				blitInBox(buf, f5, strconv.Itoa(c.getV()), c.editX, c.uly, c.editW, 20, optGold, true)
			}
		case optEditStr:
			blitTextBoxShadowed(buf, f5, c.label, c.ulx, c.uly, 185, 20, fonts.ColorGold, false)
			if c.editW > 0 && c.bg != nil {
				buf.Sub(c.bgX, c.bgY, c.bg.W, c.bg.H).Copy(c.bg) // clean box, no ghost
				blitInBox(buf, f5, editStrText[c.key], c.editX, c.uly, c.editW, 20, optGold, true)
			}
		}
	}
}

// blitInBox draws one line of text into a window buffer box (x,y,w,h), the
// block vertically centered and optionally horizontally centered. The caller
// holds the gnw lock. It clamps to the buffer so Sub never goes out of bounds.
func blitInBox(buf *gfx.Buffer, f *fonts.Font, s string, x, y, w, h, color int, centered bool) {
	if s == "" {
		return
	}
	tx := x
	if centered {
		if tw := f.Width(s); tw < w {
			tx += (w - tw) / 2
		}
	}
	ty := y + (h-f.Height)/2
	if ty < y {
		ty = y
	}
	aw := w - (tx - x)
	if tx+aw > buf.W {
		aw = buf.W - tx
	}
	ah := buf.H - ty
	if aw <= 0 || ah <= 0 || tx < 0 || ty < 0 {
		return
	}
	f.Blit(buf.Sub(tx, ty, aw, ah), s, aw, color)
}

// drawSlider repaints a slider for its current value: restore the snapshotted
// track (erasing the previous knob), then draw the knob at the value position.
// Port of OptionsMenu::DrawSlider (image->Write then load PRFSLIDE). Caller must
// not hold the gnw lock.
func drawSlider(ctx *Context, win gnw.WinID, c *optControl, slide *sprites.Simple) {
	buf := ctx.Sys.Buf(win)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	if c.bg != nil {
		buf.Sub(c.bgX, c.bgY, c.bg.W, c.bg.H).Copy(c.bg)
	}
	v := c.getV()
	if v > c.rmax {
		v = c.rmax
	}
	rmax := c.rmax
	if rmax < 1 {
		rmax = 1
	}
	kx := c.slitX + v*c.slitW/rmax - slide.Buffer().W/2
	slide.Draw(buf, kx, c.uly+(20-slide.Buffer().H)/2, true)
}

// snapBox clones the window buffer region (x,y,w,h) for later restore, or nil
// if out of bounds. Takes the gnw lock.
func snapBox(ctx *Context, win gnw.WinID, x, y, w, h int) *gfx.Buffer {
	buf := ctx.Sys.Buf(win)
	if buf == nil {
		return nil
	}
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	if x < 0 || y < 0 || w <= 0 || h <= 0 || x+w > buf.W || y+h > buf.H {
		return nil
	}
	return buf.Sub(x, y, w, h).Clone()
}

// restoreBox copies a saved region back at (x,y). Takes the gnw lock; must not
// be called while already holding it.
func restoreBox(ctx *Context, win gnw.WinID, bg *gfx.Buffer, x, y int) {
	if bg == nil {
		return
	}
	buf := ctx.Sys.Buf(win)
	if buf == nil {
		return
	}
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	if x < 0 || y < 0 || x+bg.W > buf.W || y+bg.H > buf.H {
		return
	}
	buf.Sub(x, y, bg.W, bg.H).Copy(bg)
}

// commitOptions copies the gnw checkbox states back into the working copy at
// Done (sliders/int edits already wrote through their setters live); the
// player name string field writes here too.
func commitOptions(ctx *Context, controls []optControl, wc *settings.Settings) {
	for i := range controls {
		c := &controls[i]
		switch {
		case c.kind == optCheckbox && c.bid >= 0:
			c.setV(boolToInt(ctx.Sys.ButtonChecked(c.bid)))
		case c.kind == optEditStr && c.key == "player_name":
			if name, ok := editStrText["player_name"]; ok {
				wc.Player.Name = name
			}
		}
	}
}

// applyAudioLive pushes the working copy's volumes to the running mixer so the
// player hears slider/disable changes immediately.
func applyAudioLive(ctx *Context, wc *settings.Settings) {
	if ctx.Audio != nil {
		ctx.Audio.SetVolumes(wc.Audio)
	}
}
