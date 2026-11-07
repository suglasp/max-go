// gameconfig.go : implements the difficulty, resource, turn-limit, and opponent settings screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: gameconfigmenu.cpp - GameConfigMenu
package menus

import (
	"strconv"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/settings"
)

// Game config menu modes; only the custom game mode is used so far,
// the others arrive with the mission menus and the in game options.
//
// C++: GAME_CONFIG_MENU_GAME_MODE_* (gameconfigmenu.hpp)
const (
	GameConfigModeCustom   = 1 // new custom/hot seat game: all panels
	GameConfigModeMission  = 2 // before a mission: the difficulty, the timers and the play mode
	GameConfigModeTraining = 3 // before a TRAINING mission: timers and play mode only, no difficulty
)

// showsOpponentPanel reports whether the screen offers the Computer Opponent difficulty panel. Training missions use a locked passive tier.
func showsOpponentPanel(mode int) bool {
	return mode == GameConfigModeCustom || mode == GameConfigModeMission
}

// The selectable values per panel; the C++ reads them back from the
// option labels with strtol.
var (
	gcTimerValues   = [7]int{0, 60, 120, 180, 240, 300, 360}
	gcEndturnValues = [7]int{0, 15, 30, 45, 60, 75, 90}
	gcCreditValues  = [6]int{0, 50, 100, 150, 200, 250}
)

// gcOption is one selectable caption rect.
type gcOption struct {
	r     gfx.Rect
	label string
}

// gameConfigMenu carries the screen state.
//
// C++: class GameConfigMenu (gameconfigmenu.hpp)
type gameConfigMenu struct {
	ctx  *Context
	mode int
	g    settings.Game // the values being edited

	panels     [6]*gfx.Buffer // pristine panel backgrounds
	panelRects [6]gfx.Rect
	titles     [6]string

	textEdit   *gnw.TextEdit
	activeEdit int // 0 none, else 46 (turns) or 47 (points)
}

// GameConfig edits g when confirmed and leaves it unchanged on cancel. It returns done=true on confirm.
//
// C++: GameConfigMenu + menu_options_menu_loop (menu.cpp)
func GameConfig(ctx *Context, g *settings.Game, mode int) (bool, error) {
	m := &gameConfigMenu{ctx: ctx, mode: mode, g: *g}

	pic, err := ctx.loadBig("OPTNFRM")
	if err != nil {
		return false, err
	}
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	buttons, err := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()
	if err != nil {
		return false, err
	}

	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if done, confirmed := m.handleKey(key, g); done {
			return confirmed, nil
		}
	}
}

// buildWidgets lays out the click regions of every panel, snapshots the pristine
// panel art, paints the titles, and adds the bottom plates. It returns the button
// ids for the caller to delete.
//
// C++: GameConfigMenu::Init - the control table (gameconfigmenu.cpp)
func (m *gameConfigMenu) buildWidgets() ([]gnw.ButtonID, error) {
	ctx, mode := m.ctx, m.mode

	var buttons []gnw.ButtonID
	region := func(r gfx.Rect, value int) {
		bid := ctx.Sys.AddButton(mainWindow, r.ULX, r.ULY, r.Width()+1, r.Height()+1, gnw.NoValue, gnw.NoValue, gnw.NoValue, value, nil, nil, nil, 0)
		if bid >= 0 {
			ctx.registerButtonSfx(bid, "KCARG0")
			buttons = append(buttons, bid)
		}
	}

	// Click regions per control index (the C++ control table). The
	// difficulty panel only exists for games against the computer; the
	// lower panels only when starting a custom game.
	if showsOpponentPanel(mode) {
		for i, o := range gcDifficultyOptions() {
			region(o.r, 1000+i)
		}
	}
	for i, o := range gcTimerOptions() {
		region(o.r, 1000+6+i)
	}
	for i, o := range gcEndturnOptions() {
		region(o.r, 1000+13+i)
	}
	for i, o := range gcPlayModeOptions() {
		region(o.r, 1000+20+i)
	}
	if mode == GameConfigModeCustom {
		for i, o := range gcCreditOptions() {
			region(o.r, 1000+22+i)
		}
		for i, o := range gcResourceOptions() {
			region(o.r, 1000+28+i)
		}
		for i, o := range gcVictoryOptions() {
			region(o.r, 1000+40+i)
		}
		// Victory limit edit fields, with the PREFEDIT box art.
		for i, r := range gcEditRects() {
			if img, err := ctx.loadSimple("PREFEDIT"); err == nil {
				ctx.drawSimpleAt(mainWindow, img, r.ULX, r.ULY, true)
			}
			region(r, 1000+46+i)
		}
	}

	// Pristine panel snapshots, taken after the static art is in
	// place. C++: bg_panels[6] (Image::Copy)
	m.panelRects = [6]gfx.Rect{
		gfx.NewRect(6, 38, 205, 227), gfx.NewRect(216, 38, 415, 227), gfx.NewRect(426, 38, 625, 227),
		gfx.NewRect(6, 233, 205, 422), gfx.NewRect(216, 233, 415, 422), gfx.NewRect(426, 233, 625, 422),
	}
	for i, r := range m.panelRects {
		m.panels[i] = ctx.snapshotArea(r)
	}
	m.titles = [6]string{
		ctx.text("de1b", "Computer Opponent"), ctx.text("e96a", "Turn Timers"),
		ctx.text("4599", "Play Mode"), ctx.text("1d15", "Starting Credit"),
		ctx.text("e1e4", "Resource Levels"), ctx.text("3b7b", "Victory Condition"),
	}

	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.printCentered(ctx.text("8448", "Options Menu"),
		gfx.NewRect(230, 5, 409, 25), colorGreen|fonts.FlagOutline)
	m.drawPanels()

	ctx.Sys.Fonts().SetFont(fonts.Font1)
	if err := ctx.addBottomPlates([]bottomPlate{
		{354, "MNUBTN4U", ctx.text("a80a", "Cancel"), 1000 + 48, "NCANC0"},
		{465, "MNUBTN5U", ctx.text("8610", "?"), 1000 + 49, "NHELP0"},
		{514, "MNUBTN6U", ctx.text("da62", "Done"), 1000 + 50, "NDONE0"},
	}, &buttons); err != nil {
		return buttons, err
	}

	return buttons, nil
}

// handleKey answers one input code. It reports the screen closing and whether the
// player confirmed, writing the edited values back into g on a confirm.
//
// C++: menu_options_menu_loop - the input switch (menu.cpp)
func (m *gameConfigMenu) handleKey(key int, g *settings.Game) (done, confirmed bool) {
	ctx := m.ctx

	// An open victory limit edit field consumes input first.
	// C++: menu_options_menu_loop text_edit block
	if m.textEdit != nil {
		switch {
		case key == platform.KeyReturn:
			m.commitEdit(true)
			return false, false
		case key == platform.KeyEscape:
			m.commitEdit(false)
			return false, false
		case key >= '0' && key <= '9', key == platform.KeyBackspace:
			m.textEdit.HandleKey(key)
			return false, false
		case key >= 1000:
			m.commitEdit(true) // clicking elsewhere commits
		default:
			return false, false
		}
	}

	switch {
	case key >= 1000 && key <= 1047:
		m.event(key - 1000)
	case key == 1000+48, key == platform.KeyEscape:
		return true, false
	case key == 1000+50, key == platform.KeyReturn:
		*g = m.g
		return true, true
	case key == 1000+49, key == '?':
		Help(ctx, "OPTIONS_SETUP")
	}

	return false, false
}

// event dispatches a control index to its panel update.
//
// C++: GameConfigMenu::Event* handlers
// victoryPreset is one preset victory condition button: the victory type
// (0 points, 1 turns) and its limit.
type victoryPreset struct{ vtype, limit int }

func (m *gameConfigMenu) event(i int) {
	switch {
	case i < 6:
		m.g.Opponent = i
		m.drawPanelOpponent()
	case i < 13:
		m.g.TurnTimer = gcTimerValues[i-6]
		m.drawPanelTimers()
	case i < 20:
		m.g.EndTurn = gcEndturnValues[i-13]
		m.drawPanelTimers()
	case i < 22:
		m.g.PlayMode = i - 20
		m.drawPanelPlayMode()
	case i < 28:
		m.g.StartGold = gcCreditValues[i-22]
		m.drawPanelCredit()
	case i < 31:
		m.g.RawResource = i - 28
		m.drawPanelResources()
	case i < 34:
		m.g.FuelResource = i - 31
		m.drawPanelResources()
	case i < 37:
		m.g.GoldResource = i - 34
		m.drawPanelResources()
	case i < 40:
		m.g.AlienDerelicts = i - 37
		m.drawPanelResources()
	case i < 46:
		// Preset victory conditions.
		// C++: EventVictoryCondition
		presets := [6]victoryPreset{
			{0, 100}, {0, 200}, {0, 400}, {1, 200}, {1, 400}, {1, 800},
		}
		m.g.VictoryType = presets[i-40].vtype
		m.g.VictoryLimit = presets[i-40].limit
		m.drawPanelVictory()
	case i < 48:
		m.openEdit(i)
	}
}

// openEdit opens the victory limit text edit over field 46 (turns) or
// 47 (points).
//
// C++: GameConfigMenu::EventVictoryConditionPrefs
func (m *gameConfigMenu) openEdit(i int) {
	m.activeEdit = i
	m.drawPanelVictory()

	limit := m.g.VictoryLimit
	if (i == 46 && m.g.VictoryType != 0) || (i == 47 && m.g.VictoryType != 1) {
		limit = 0
	}

	// The field owns Font5 and snapshots the PREFEDIT art under it, so
	// repaints stay correct whatever font the menu loop has current.
	// C++: TextEdit(..., 0xA2, GNW_TEXT_FONT_5)
	r := gcEditRects()[i-46]
	// 0xA2 is the metal value color; the gnw TextEdit highlights it white while
	// active (C++ TextEdit::DrawFullText), so it reads metal idle, white editing.
	te := m.ctx.Sys.NewTextEdit(mainWindow, r.ULX+10, r.ULY+5, r.Width()-20, 5, 0xA2, fonts.Font5)
	te.SetText(strconv.Itoa(limit))
	te.SetActive(true)
	m.textEdit = te
}

// commitEdit closes the edit field, applying the typed limit when
// apply is set.
//
// C++: GameConfigMenu::UpdateTextEdit
func (m *gameConfigMenu) commitEdit(apply bool) {
	if m.textEdit == nil {
		return
	}
	if apply {
		m.g.VictoryType = m.activeEdit - 46
		m.g.VictoryLimit, _ = strconv.Atoi(m.textEdit.Text())
	}
	m.textEdit.SetActive(false)
	m.textEdit = nil
	m.activeEdit = 0
	m.drawPanelVictory()
}

// drawPanels paints every panel applicable to the menu mode.
//
// C++: GameConfigMenu::DrawPanels
func (m *gameConfigMenu) drawPanels() {
	if showsOpponentPanel(m.mode) {
		m.drawPanelOpponent()
	}
	m.drawPanelTimers()
	m.drawPanelPlayMode()
	if m.mode == GameConfigModeCustom {
		m.drawPanelCredit()
		m.drawPanelResources()
		m.drawPanelVictory()
	}
}

// drawPanelBg restores a panel background and reprints its header.
//
// C++: GameConfigMenu::DrawBgPanel
func (m *gameConfigMenu) drawPanelBg(i int) {
	ctx := m.ctx
	ctx.restoreArea(m.panelRects[i], m.panels[i])
	headers := [6]gfx.Rect{
		gfx.NewRect(19, 50, 199, 70), gfx.NewRect(230, 50, 409, 70), gfx.NewRect(439, 50, 619, 70),
		gfx.NewRect(19, 245, 199, 265), gfx.NewRect(230, 245, 409, 265), gfx.NewRect(439, 245, 619, 265),
	}
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.printCentered(m.titles[i], headers[i], colorGreen|fonts.FlagOutline)
}

// drawOptions paints selectable captions and boxes the selected one.
//
// C++: GameConfigMenu::DrawRadioButtons
func (m *gameConfigMenu) drawOptions(opts []gcOption, selected int) {
	ctx := m.ctx
	for i, o := range opts {
		ctx.printShadowed(o.label, o.r, fonts.ColorBrightBrown, true)
		if i == selected {
			ctx.Sys.Box(mainWindow, o.r.ULX, o.r.ULY, o.r.LRX, o.r.LRY, colorWhite)
		}
	}
}

// gcIndexFor picks the highest option whose value the setting still
// reaches (the C++ reverse strtol scan).
func gcIndexFor(setting int, values []int) int {
	for i := len(values) - 1; i > 0; i-- {
		if setting >= values[i] {
			return i
		}
	}
	return 0
}

// gcDifficultyOptions returns the selectable computer/opponent difficulty values.
func gcDifficultyOptions() []gcOption {
	labels := [6]string{"fde2|Clueless", "e644|Apprentice", "2d88|Average", "8b31|Expert", "a2bb|Master", "9c54|God"}
	out := make([]gcOption, 6)
	for i := range out {
		out[i] = gcOption{gfx.NewRect(56, 85+15*i, 156, 100+15*i), labels[i]}
	}
	return out
}

// gcTimerOptions returns the selectable turn timer values.
func gcTimerOptions() []gcOption {
	labels := [7]string{"366c|No Limit", "03d2|60 seconds", "|120", "|180", "|240", "|300", "|360"}
	out := make([]gcOption, 7)
	out[0] = gcOption{gfx.NewRect(227, 105, 307, 120), labels[0]}
	for i := 1; i < 7; i++ {
		out[i] = gcOption{gfx.NewRect(227, 105+15*i, 307, 120+15*i), labels[i]}
	}
	return out
}

// gcEndturnOptions returns the selectable end turn behavior values.
func gcEndturnOptions() []gcOption {
	labels := [7]string{"8f89|None", "218f|15 seconds", "|30", "|45", "|60", "|75", "|90"}
	out := make([]gcOption, 7)
	for i := range out {
		out[i] = gcOption{gfx.NewRect(331, 105+15*i, 411, 120+15*i), labels[i]}
	}
	return out
}

// gcPlayModeOptions returns the selectable game mode values.
func gcPlayModeOptions() []gcOption {
	return []gcOption{
		{gfx.NewRect(480, 90, 580, 110), "f7e4|Turn Based"},
		{gfx.NewRect(480, 115, 580, 145), "0ba9|Simultaneous Moves"},
	}
}

// gcCreditOptions returns the selectable starting credit values.
func gcCreditOptions() []gcOption {
	out := make([]gcOption, 6)
	for i := range out {
		out[i] = gcOption{gfx.NewRect(144, 285+20*i, 194, 300+20*i), "|" + strconv.Itoa(gcCreditValues[i])}
	}
	return out
}

// gcResourceOptions: 4 rows (raw/fuel/gold/alien) of 3 columns.
func gcResourceOptions() []gcOption {
	rows := [4]int{292, 327, 362, 397}
	labels := [4][3]string{
		{"94df|Poor", "8e58|Medium", "dca9|Rich"},
		{"8c28|Poor", "ccfe|Medium", "eac9|Rich"},
		{"ec35|Poor", "9c1d|Medium", "0c4a|Rich"},
		{"2484|None", "291d|Rare", "3099|Common"},
	}
	cols := [3][2]int{{225, 287}, {288, 352}, {351, 413}}
	var out []gcOption
	for row := 0; row < 4; row++ {
		for col := 0; col < 3; col++ {
			out = append(out, gcOption{
				gfx.NewRect(cols[col][0], rows[row], cols[col][1], rows[row]+12),
				labels[row][col],
			})
		}
	}
	return out
}

// gcVictoryOptions returns the selectable victory condition values.
func gcVictoryOptions() []gcOption {
	return []gcOption{
		{gfx.NewRect(447, 305, 507, 320), "f5d6|Short"},
		{gfx.NewRect(447, 325, 507, 340), "9427|Medium"},
		{gfx.NewRect(447, 345, 507, 360), "d1d8|Long"},
		{gfx.NewRect(551, 305, 611, 320), "76b7|Low"},
		{gfx.NewRect(551, 325, 611, 340), "b459|Medium"},
		{gfx.NewRect(551, 345, 611, 360), "4065|High"},
	}
}

// gcEditRects returns the text entry hit boxes for the custom game's numeric fields.
func gcEditRects() [2]gfx.Rect {
	return [2]gfx.Rect{gfx.NewRect(447, 370, 507, 390), gfx.NewRect(551, 370, 611, 390)}
}

// label resolves a "langkey|fallback" option label via the strings
// table.
func (m *gameConfigMenu) label(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			if i == 0 {
				return s[1:]
			}
			return m.ctx.text(s[:i], s[i+1:])
		}
	}
	return s
}

// resolve maps the raw option labels through the language table.
func (m *gameConfigMenu) resolve(opts []gcOption) []gcOption {
	out := make([]gcOption, len(opts))
	for i, o := range opts {
		out[i] = gcOption{o.r, m.label(o.label)}
	}
	return out
}

// drawPanelOpponent: difficulty list plus its description text.
//
// C++: GameConfigMenu::DrawPanelComputerOpponent
func (m *gameConfigMenu) drawPanelOpponent() {
	ctx := m.ctx
	m.drawPanelBg(0)
	m.drawOptions(m.resolve(gcDifficultyOptions()), m.g.Opponent)

	descKeys := [6]string{"1dc3", "eb1c", "0e00", "2de0", "e22c", "7cb8"}
	desc := ""
	if m.g.Opponent >= 0 && m.g.Opponent < 6 {
		desc = ctx.text(descKeys[m.g.Opponent], "")
	}
	const colorYellow = 0x04
	ctx.printTextBox(desc, gfx.NewRect(19, 170, 198, 229), colorYellow|fonts.FlagOutline, false)
}

// drawPanelTimers: turn limit and end turn columns.
//
// C++: GameConfigMenu::DrawPanelTurnTimers
func (m *gameConfigMenu) drawPanelTimers() {
	ctx := m.ctx
	m.drawPanelBg(1)
	ctx.printShadowed(ctx.text("779f", "Turn Limit:"), gfx.NewRect(227, 85, 307, 105), fonts.ColorBrightYellow, true)
	ctx.printShadowed(ctx.text("99b8", "End Turn:"), gfx.NewRect(331, 85, 411, 105), fonts.ColorBrightYellow, true)
	m.drawOptions(m.resolve(gcTimerOptions()), gcIndexFor(m.g.TurnTimer, gcTimerValues[:]))
	m.drawOptions(m.resolve(gcEndturnOptions()), gcIndexFor(m.g.EndTurn, gcEndturnValues[:]))
}

// C++: GameConfigMenu::DrawPanelPlayMode
func (m *gameConfigMenu) drawPanelPlayMode() {
	m.drawPanelBg(2)
	m.drawOptions(m.resolve(gcPlayModeOptions()), m.g.PlayMode)
}

// C++: GameConfigMenu::DrawPanelStartingCredit
func (m *gameConfigMenu) drawPanelCredit() {
	ctx := m.ctx
	m.drawPanelBg(3)
	labels := [6]string{"dbc7|None", "f793|Low", "e512|Limited", "a724|Normal", "c4eb|High", "3ade|Too Much!"}
	for i, l := range labels {
		ctx.printShadowed(m.label(l), gfx.NewRect(24, 285+20*i, 114, 300+20*i), fonts.ColorBrightYellow, false)
	}
	m.drawOptions(m.resolve(gcCreditOptions()), gcIndexFor(m.g.StartGold, gcCreditValues[:]))
}

// C++: GameConfigMenu::DrawPanelResourceLevels
func (m *gameConfigMenu) drawPanelResources() {
	ctx := m.ctx
	m.drawPanelBg(4)
	ctx.printShadowed(ctx.text("d558", "Raw Materials:"), gfx.NewRect(229, 280, 409, 292), fonts.ColorBrightYellow, false)
	ctx.printShadowed(ctx.text("10c4", "Fuel:"), gfx.NewRect(229, 315, 409, 327), fonts.ColorBrightYellow, false)
	ctx.printShadowed(ctx.text("4ef9", "Gold:"), gfx.NewRect(229, 350, 409, 362), fonts.ColorBrightYellow, false)
	ctx.printShadowed(ctx.text("48bd", "Alien Derelicts:"), gfx.NewRect(229, 385, 409, 397), fonts.ColorBrightYellow, false)

	opts := m.resolve(gcResourceOptions())
	m.drawOptions(opts[0:3], m.g.RawResource)
	m.drawOptions(opts[3:6], m.g.FuelResource)
	m.drawOptions(opts[6:9], m.g.GoldResource)
	m.drawOptions(opts[9:12], m.g.AlienDerelicts)
}

// C++: GameConfigMenu::DrawPanelVictoryCondition
func (m *gameConfigMenu) drawPanelVictory() {
	ctx := m.ctx
	m.drawPanelBg(5)
	ctx.printShadowed(ctx.text("298d", "Duration:"), gfx.NewRect(427, 285, 529, 305), fonts.ColorBrightYellow, true)
	ctx.printShadowed(ctx.text("9689", "Score:"), gfx.NewRect(529, 285, 633, 305), fonts.ColorBrightYellow, true)

	// Selected preset: nearest preset at or below the limit.
	sel := 0
	if m.g.VictoryType == 0 {
		switch {
		case m.g.VictoryLimit > 200:
			sel = 2
		case m.g.VictoryLimit > 100:
			sel = 1
		}
	} else {
		sel = 3
		switch {
		case m.g.VictoryLimit > 400:
			sel = 5
		case m.g.VictoryLimit > 200:
			sel = 4
		}
	}
	m.drawOptions(m.resolve(gcVictoryOptions()), sel)

	// Current limit in white inside the matching edit box (hidden
	// while it is being edited). The PREFEDIT box art is part of the
	// panel snapshot.
	if m.activeEdit == 0 {
		box := gcEditRects()[m.g.VictoryType&1]
		// Metal value color (0xA2), matching the edit field's idle color; it
		// only turns white while being edited (vanilla convention).
		ctx.printCentered(strconv.Itoa(m.g.VictoryLimit), box, 0xA2)
	}
	ctx.printShadowed(ctx.text("a07a", "turns"), gfx.NewRect(447, 390, 507, 410), fonts.ColorBrightYellow, true)
	ctx.printShadowed(ctx.text("7015", "points"), gfx.NewRect(551, 390, 611, 410), fonts.ColorBrightYellow, true)
}
