// debugmenu.go : implements the in game debug menu.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/winloss"
)

// debugMenuState holds the F2 developer menu, click to place, and endless mode state.
type debugMenuState struct {
	dbgMenuOpen       bool   // F2 developer flag menu is open
	dbgMenuSel        int    // highlighted row in the F2 menu
	dbgMenuOpenedTick uint64 // freezes the turn clock while the F2 menu is open
	dbgPlaceMode      int    // armed click to place drop mode
	dbgUnitSel        int    // F2 menu: selected unit type to place
	dbgBuildSel       int    // F2 menu: selected building type to place
	endless           bool   // debug menu: suppress win/loss check (open ended testing)
}

// The F2 developer flag menu: a pure debug overlay for poking the live game:
// toggle the AI, fog and win/loss, roll the turn timer / turn cap, and arm a
// click to place "add unit" / "add building" drop. It is gated by Screen.debug
// (the DEBUG_GAME build) and has no retail counterpart. While it is open the
// turn clock is paused, the same way the Files / Preferences screens hold the
// game. Flags whose backing system does not exist yet are shown as "(n/a)".

// debug placement modes for the Add unit / Add building rows.
const (
	dbgPlaceNone     = 0
	dbgPlaceUnit     = 1
	dbgPlaceBuilding = 2
)

// dbgTimerSteps are the turn timer values (seconds) the "Time per turn" row
// rolls through; 0 means no limit. dbgTurnCapSteps are the "Max turns" values;
// 0 means no cap.
var (
	dbgTimerSteps   = []int{0, 30, 45, 60, 90, 120, 180, 240, 300}
	dbgTurnCapSteps = []int{0, 50, 100, 200, 400, 800}
)

// dbgBuilding is one "Add building" roster entry: the structure type and its
// footprint (large = 2x2).
type dbgBuilding struct {
	t     assets.UnitType
	large bool
}

// dbgBuildingRoster is the set of structures the "Add building" row cycles.
var dbgBuildingRoster = []dbgBuilding{
	{assets.MiningSt, true},
	{assets.PowGen, false},
	{assets.LandPlt, true},
	{assets.AirPlt, true},
	{assets.Depot, true},
	{assets.Research, true},
	{assets.Radar, false},
	{assets.Cnct4W, false},
}

// dbgFlag is one row of the menu: a label, the current value string, and an
// action invoked on toggle/roll (dir is -1 for left, +1 for right/enter). na
// rows have no action and render dimmed.
type dbgFlag struct {
	label string
	value func(s *Screen) string
	act   func(s *Screen, dir int)
	na    bool
}

// dbgFlags is the menu layout, top to bottom.
var dbgFlags = []dbgFlag{
	{
		label: "AI reasoning",
		value: func(s *Screen) string { return dbgOnOff(s.ai != nil && s.ai.IsEnabled()) },
		act: func(s *Screen, _ int) {
			if s.ai != nil {
				s.ai.SetEnabled(!s.ai.IsEnabled())
			}
		},
	},
	{
		label: "AI difficulty",
		value: func(s *Screen) string {
			if s.ai == nil {
				return "(no AI)"
			}

			return fmt.Sprintf("%d %s", s.ai.Difficulty(), ai.DifficultyName(s.ai.Difficulty()))
		},
		act: func(s *Screen, dir int) {
			if s.ai == nil {
				return
			}

			if dir == 0 {
				dir = 1 // Enter cycles to the next tier
			}

			s.SetAIDifficulty(wrapIndex(s.ai.Difficulty()+dir, ai.DifficultyCount))
		},
	},
	{
		label: "AI combat",
		value: func(s *Screen) string {
			if s.ai == nil {
				return "(no AI)"
			}

			return ai.CombatPosture()
		},
	},
	{
		label: "Fog of war",
		value: func(s *Screen) string { return dbgOnOff(s.fogShow) },
		act:   func(s *Screen, _ int) { s.fogShow = !s.fogShow },
	},
	{
		label: "Endless mode",
		value: func(s *Screen) string { return dbgOnOff(s.debugMenu.endless) },
		act:   func(s *Screen, _ int) { s.debugMenu.endless = !s.debugMenu.endless },
	},
	{
		label: "Show AI paths",
		value: func(s *Screen) string { return dbgOnOff(s.showAIPaths) },
		act:   func(s *Screen, _ int) { s.showAIPaths = !s.showAIPaths },
	},
	{
		// Rings the ground the player's sentries guard, so the holes in a defensive
		// line show up. It lives here rather than on the left panel because the nine
		// panel toggles are vanilla exact and the original has no such overlay.
		label: "Sentry coverage",
		value: func(s *Screen) string { return dbgOnOff(s.showSentryCoverage) },
		act:   func(s *Screen, _ int) { s.showSentryCoverage = !s.showSentryCoverage },
	},
	{
		// Marks the whole resource map surveyed for the player team, so an
		// Auto Survey surveyor sees nothing left and switches itself off - the
		// manual way to exercise the path that turns it off automatically. Enter triggers the reveal.
		label: "Unlock resource map",
		value: func(s *Screen) string {
			if s.game.Resources == nil {
				return "(no map)"
			}

			if s.resourceMapFullySurveyed(s.game.State.PlayerTeam) {
				return "(already done)"
			}

			return "(ENTER to reveal)"
		},
		act: func(s *Screen, _ int) { s.revealResourceMap() },
	},
	{
		// Cheat [MAXSTORAGE]: fill every one of the player's units' cargo holds.
		label: "Max storage",
		value: func(*Screen) string { return "(ENTER to fill)" },
		act: func(s *Screen, _ int) {
			s.game.CheatFillStorage(s.game.State.PlayerTeam)
			s.showMessage("Cheat: all units' cargo filled.")
		},
	},
	{
		// Cheat [MAXAMMO]: refill every one of the player's units' ammo.
		label: "Max ammo",
		value: func(*Screen) string { return "(ENTER to refill)" },
		act: func(s *Screen, _ int) {
			s.game.CheatFillAmmo(s.game.State.PlayerTeam)
			s.showMessage("Cheat: all units' ammo refilled.")
		},
	},
	{
		// Cheat [MAXSUPER]: super charge the selected unit's combat stats.
		label: "Super unit",
		value: func(s *Screen) string {
			if s.selected == nil {
				return "(select a unit)"
			}

			return "(ENTER to boost)"
		},
		act: func(s *Screen, _ int) {
			if s.selected == nil {
				s.showMessage("Select a unit to super charge.")
				return
			}

			s.game.CheatSuperUnit(s.selected)
			s.showMessage("Cheat: unit super charged.")
		},
	},
	{
		label: "Game mode",
		value: func(s *Screen) string {
			if s.simMode() {
				return "simultaneous"
			}

			return "turn based"
		},
	},
	{
		label: "Max turns",
		value: func(s *Screen) string {
			if s.game.VictoryLimit <= 0 {
				return "none"
			}

			return fmt.Sprintf("%d", s.game.VictoryLimit)
		},
		act: func(s *Screen, dir int) {
			s.game.VictoryType = winloss.VictoryDuration
			s.game.VictoryLimit = rollStep(dbgTurnCapSteps, s.game.VictoryLimit, dir)
		},
	},
	{
		label: "Time per turn",
		value: func(s *Screen) string {
			if s.turnFlow.turnSeconds <= 0 {
				return "off"
			}

			return fmt.Sprintf("%ds", s.turnFlow.turnSeconds)
		},
		act: func(s *Screen, dir int) {
			s.turnFlow.turnSeconds = rollStep(dbgTimerSteps, s.turnFlow.turnSeconds, dir)
			s.turnFlow.turnStart = platform.Ticks() // restart the clock against the new budget
			s.logTurnClock("debug menu timer step")
		},
	},
	{
		label: "Add unit",
		value: func(s *Screen) string { return displayName(debugUnitTypes[s.debugMenu.dbgUnitSel].t) },
		act: func(s *Screen, dir int) {
			if dir == 0 { // Enter: arm the drop and step out of the menu
				s.armDebugPlace(dbgPlaceUnit)
				return
			}

			s.debugMenu.dbgUnitSel = wrapIndex(s.debugMenu.dbgUnitSel+dir, len(debugUnitTypes))
		},
	},
	{
		label: "Add building",
		value: func(s *Screen) string { return displayName(dbgBuildingRoster[s.debugMenu.dbgBuildSel].t) },
		act: func(s *Screen, dir int) {
			if dir == 0 {
				s.armDebugPlace(dbgPlaceBuilding)
				return
			}

			s.debugMenu.dbgBuildSel = wrapIndex(s.debugMenu.dbgBuildSel+dir, len(dbgBuildingRoster))
		},
	},
}

// openDebugMenu shows the F2 menu and freezes the turn clock. C++: none (dev).
func (s *Screen) openDebugMenu() {
	s.debugMenu.dbgMenuOpen = true
	s.debugMenu.dbgMenuOpenedTick = platform.Ticks()
}

// closeDebugMenu hides the menu and resumes the clock, crediting back the time
// the game spent paused so the active team does not lose its turn budget.
func (s *Screen) closeDebugMenu() {
	if s.debugMenu.dbgMenuOpen {
		s.turnFlow.turnStart += platform.Ticks() - s.debugMenu.dbgMenuOpenedTick
	}

	s.debugMenu.dbgMenuOpen = false
}

// revealResourceMap (F2 debug) marks every cell of the resource map surveyed for
// the player team, then runs the automatic survey driver again so any Surveyor in Auto
// mode immediately sees nothing left to map and switches itself off. The manual
// way to confirm the Auto Survey path that turns it off automatically without driving the whole map. It
// does nothing (aside from a message) when the game has no resource map.
func (s *Screen) revealResourceMap() {
	if s.game.Resources == nil {
		s.showMessage("No resource map to reveal.")
		return
	}

	team := s.game.State.PlayerTeam

	// Mark the whole grid surveyed for the player team.
	for y := 0; y < s.game.H; y++ {
		for x := 0; x < s.game.W; x++ {
			s.game.Resources.MarkSurveyed(x, y, team)
		}
	}

	// Any surveyor with automatic survey on now finds nothing unsurveyed and turns Auto off.
	s.tickAutoSurvey(team)
	s.showMessage("Resource map revealed - Auto-Survey switches off.")
}

// resourceMapFullySurveyed reports whether every cell of the resource map is
// already surveyed for the given team (used by the F2 row to show "already
// done"). False when there is no resource map.
func (s *Screen) resourceMapFullySurveyed(team int) bool {
	if s.game.Resources == nil {
		return false
	}

	for y := 0; y < s.game.H; y++ {
		for x := 0; x < s.game.W; x++ {
			if !s.game.Resources.IsSurveyed(x, y, team) {
				return false
			}
		}
	}

	return true
}

// SetAIDifficulty sets the computer opponent tier (0..5 Clueless..God) for this
// game's AI. Called from the launch path (the custom game setting and the
// -opponent flag) and the F2 debug menu's "AI difficulty" row. It does nothing when no AI
// is wired or the tier is negative (leaves the AI at its default tier).
func (s *Screen) SetAIDifficulty(d int) {
	if d < 0 || s.ai == nil {
		return
	}

	s.ai.SetDifficulty(d)

	// Capture the tier onto the game state right away so a save carries it, even one
	// taken before the first computer turn runs the AI's own capture. Difficulty()
	// returns the clamped tier just applied.
	if s.game != nil && s.game.State != nil {
		s.game.State.Difficulty = s.ai.Difficulty()
	}
}

// AnnounceAIDifficulty raises the on screen banner "AI Difficulty : <n> [<desc>]"
// for the current tier. The quick start benches (-random / -random2) call it so the
// player sees which difficulty they are facing; a menu game does not need it since
// the player picked the tier on the setup screen.
func (s *Screen) AnnounceAIDifficulty() {
	if s.ai == nil {
		return
	}

	d := s.ai.Difficulty()
	s.showMessage(fmt.Sprintf("AI Difficulty : %d [%s]", d, ai.DifficultyName(d)))
}

// handleDebugMenuKey drives the menu while it is open: arrows navigate / adjust,
// Enter activates, F2 / Escape close.
func (s *Screen) handleDebugMenuKey(key int) {
	switch key {
	case platform.KeyF2, platform.KeyEscape:
		s.closeDebugMenu()

	case platform.KeyUp:
		s.debugMenu.dbgMenuSel = wrapIndex(s.debugMenu.dbgMenuSel-1, len(dbgFlags))

	case platform.KeyDown:
		s.debugMenu.dbgMenuSel = wrapIndex(s.debugMenu.dbgMenuSel+1, len(dbgFlags))

	case platform.KeyLeft:
		s.applyDebugFlag(-1)

	case platform.KeyRight:
		s.applyDebugFlag(+1)

	case platform.KeyReturn:
		s.applyDebugFlag(0)
	}
}

// applyDebugFlag runs the selected row's action (does nothing on an n/a row) and logs
// the change to the debug log as "alter dbg : <label> = <value>".
func (s *Screen) applyDebugFlag(dir int) {
	f := &dbgFlags[s.debugMenu.dbgMenuSel]

	if f.na || f.act == nil {
		return
	}

	f.act(s, dir)

	dbg.Printf("alter dbg : %s = %s", f.label, f.value(s))
}

// armDebugPlace closes the menu and arms a click to place drop; the next
// left click on the map places the selected unit / building for the active team.
func (s *Screen) armDebugPlace(mode int) {
	s.debugMenu.dbgPlaceMode = mode
	s.closeDebugMenu()

	switch mode {
	case dbgPlaceUnit:
		s.showMessage("Click to place " + displayName(debugUnitTypes[s.debugMenu.dbgUnitSel].t) + " (ESC cancels)")

	case dbgPlaceBuilding:
		s.showMessage("Click to place " + displayName(dbgBuildingRoster[s.debugMenu.dbgBuildSel].t) + " (ESC cancels)")
	}
}

// placeDebugAt drops the armed unit / building at the clicked cell for the active
// team, then disarms. Returns whether a placement happened. C++: none (dev).
func (s *Screen) placeDebugAt(cellX, cellY int) bool {
	team := s.game.State.ActiveTeam

	switch s.debugMenu.dbgPlaceMode {
	case dbgPlaceUnit:
		def := debugUnitTypes[s.debugMenu.dbgUnitSel]
		s.setSelected(s.deployUnit(def.t, def.flags, team, 0, cellX, cellY))

	case dbgPlaceBuilding:
		spec := dbgBuildingRoster[s.debugMenu.dbgBuildSel]
		flags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

		if spec.large {
			flags |= game.FlagBuilding
		}

		u := game.NewUnitInfo(spec.t, team, s.spawnSeq+1, 0)
		u.Flags = flags
		u.Values = s.teamValues(team, spec.t)
		u.InitState()
		s.spawnSeq++
		s.game.PlaceBuilding(u, cellX, cellY)
		s.setSelected(u)

	default:
		return false
	}

	if s.selected != nil {
		dbg.Printf("alter dbg : placed %s for team %d at (%d,%d)", displayName(s.selected.Type), team, cellX, cellY)
	}

	s.debugMenu.dbgPlaceMode = dbgPlaceNone

	return true
}

// debugPlaceArmed reports whether a click should drop a debug unit/building.
func (s *Screen) debugPlaceArmed() bool {
	return s.debugMenu.dbgPlaceMode != dbgPlaceNone
}

// cancelDebugPlace disarms a pending drop (Escape).
func (s *Screen) cancelDebugPlace() {
	if s.debugMenu.dbgPlaceMode != dbgPlaceNone {
		s.debugMenu.dbgPlaceMode = dbgPlaceNone

		s.showMessage("Placement cancelled.")
	}
}

// debugSelfDestructSelected mirrors the original's DEBUG_GAME Alt+D shortcut: the
// selected live unit is forced through the explosion/removal path. It is not a
// retail popup action; normal self destruct still goes through SELFDSTR.
//
// C++: game_manager.cpp - GNW_KB_KEY_LALT_D.
func (s *Screen) debugSelfDestructSelected() {
	if s.game == nil || s.selected == nil {
		return
	}

	u := s.selected

	if s.game.Units.ByID.Get(u.ID) != u {
		return
	}

	dbg.Printf("alter dbg : self-destruct %s id=%d", displayName(u.Type), u.ID)

	s.destroyUnit(u)
}

// drawDebugMenu renders the flag overlay near the top left of the map area: a
// darkened band with one row per flag, the selected row marked and n/a rows
// dimmed.
func (s *Screen) drawDebugMenu(buf *gfx.Buffer) {
	if !s.debugMenu.dbgMenuOpen {
		return
	}

	f := s.sys.Fonts().Get(fonts.Font2)

	// padX widened 8 -> 11 (+3px each side) so the dark band is a touch wider than
	// the text and the rows do not sit flush against the panel edge.
	const padX, padY, rowH = 11, 6, 0
	lineH := f.Height + 2
	title := "-- DEBUG FLAGS (F2) --"

	// Width: the longest "label : value" plus padding.
	maxW := f.Width(title)

	for i := range dbgFlags {
		if w := f.Width(dbgFlags[i].label + " : " + dbgFlags[i].value(s)); w > maxW {
			maxW = w
		}
	}

	panelW := maxW + 2*padX
	panelH := 2*padY + lineH*(len(dbgFlags)+2)
	x0 := MainMap.ULX + 6
	y0 := MainMap.ULY + 6
	panelW = min(panelW, buf.W-x0)
	panelH = min(panelH, buf.H-y0)

	// Blend the terrain under the panel toward black at 60% (40% of the terrain
	// shows through) so the flags read clearly while the map stays a little visible.
	lut := s.pal.IntensityTable1(0, 0, 0, 60, 40)
	band := buf.Sub(x0, y0, panelW, panelH)

	for y := 0; y < band.H; y++ {
		row := band.Row(y)

		for x := 0; x < band.W; x++ {
			row[x] = lut[row[x]]
		}
	}

	white := messageColor
	sel := fonts.Color{Base: s.colorOrange, Outline: s.colorOrange, Shadow: 199}
	dim := fonts.Color{Base: s.effects.colRubble, Outline: s.effects.colRubble, Shadow: 199}

	tx := x0 + padX
	ty := y0 + padY
	f.BlitShadowed(buf.Sub(tx, ty, buf.W-tx, buf.H-ty), title, f.Width(title)+1, white)
	ty += lineH * 2

	for i := range dbgFlags {
		text := dbgFlags[i].label + " : " + dbgFlags[i].value(s)
		col := white

		if dbgFlags[i].na {
			col = dim
		}

		if i == s.debugMenu.dbgMenuSel {
			col = sel
			text = "> " + text
		} else {
			text = "  " + text
		}

		f.BlitShadowed(buf.Sub(tx, ty, buf.W-tx, buf.H-ty), text, f.Width(text)+1, col)
		ty += lineH
	}
}

// onOff renders a bool as ON / OFF.
func dbgOnOff(b bool) string {
	if b {
		return "ON"
	}

	return "OFF"
}

// wrapIndex wraps i into [0, n).
func wrapIndex(i, n int) int {
	if n <= 0 {
		return 0
	}

	return ((i % n) + n) % n
}

// rollStep returns the next/previous value in steps relative to cur (dir +1/-1),
// landing on the nearest step if cur is not exactly one of them.
func rollStep(steps []int, cur, dir int) int {
	idx := 0

	for i, v := range steps {
		if v == cur {
			idx = i
			break
		}

		if v <= cur {
			idx = i
		}
	}

	return steps[wrapIndex(idx+dir, len(steps))]
}

// debugUnit is one U key roster entry: the unit type and its deploy flags.
type debugUnit struct {
	t     assets.UnitType // unit type offered in the debug menu
	flags game.Flags      // flags applied when the unit spawns
}

// debugUnitTypes is the U key roster: turreted tank, recon car, spinning radar
// air unit, bomber, constructor - enough to exercise every draw path.
var debugUnitTypes = []debugUnit{
	{assets.Tank, game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite | game.FlagCanHeadRotate},
	{assets.Scout, game.FlagMobileLandUnit | game.FlagSelectable},
	{assets.Scanner, game.FlagMobileLandUnit | game.FlagSelectable | game.FlagSpinningTurret},
	{assets.Awac, game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable | game.FlagSpinningTurret},
	{assets.Bomber, game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable},
	{assets.Constrct, game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable},
}
