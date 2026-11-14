// hud.go : implements the instrument bar chrome: the frame, the top and bottom bars and the readouts.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
	/* "github.com/suglasp/max-go/internal/world": note: unused code in Go port, animatePalette uses Screen.cycler */
	"github.com/suglasp/max-go/internal/units"
)

// animatePalette advances terrain colour cycles on their own clocks and reinstalls the palette when one changes.
//
// C++: World_UpdateColorAnimations driven from the game tick.
func (s *Screen) animatePalette() {
	if s.cycler.Advance(&s.pal, platform.Ticks()) {
		s.plat.SetPalette(&s.pal)
	}
	/* // note: unused code in Go port
	// The loop moved into world.ColorCycler.Advance, shared with the map editor and the mapview debug screen.
	now := platform.Ticks()
	changed := false

	for i, c := range world.ColorCycles {
		if now-s.cycleStamps[i] >= uint64(1000/c.FPS) {
			s.cycleStamps[i] = now
			world.CycleStep(&s.pal, c, &s.pulse)
			changed = true
		}
	}

	if changed {
		s.plat.SetPalette(&s.pal)
	}
	*/
}

const colorTurnText = 0xA2 // gold palette index for the turn counter/timer. C++: GameManager_DrawTurnCounter/Timer

const colorTurnTimerWarn = 0x01 // COLOR_RED, shown at <=20s remaining. C++: GameManager_DrawTurnTimer

// drawTopBar paints the End Turn button, turn counter, and turn timer over the static FRAMEPIC slots.
// C++: GameManager_DrawTurnCounter / GameManager_DrawTurnTimer + WINDOW_ENDTURN_BUTTON.
func (s *Screen) drawTopBar(buf *gfx.Buffer) {
	s.drawEndTurnButton(buf)
	s.drawReadout(buf, TurnCounter, s.hudArt.turnCounterBg, 0, fmt.Sprintf("%d", s.game.State.TurnCounter), colorTurnText)

	remaining := s.turnFlow.turnSeconds
	/* remaining, color := s.turnFlow.turnSeconds, colorTurnText: note: unused code in Go port, the colour is turnClockColor (N25) */

	if s.turnFlow.turnSeconds > 0 && s.turnFlow.handover == -1 && !s.gameOver {
		remaining = s.turnRemaining()
	}

	/* // note: unused code in Go port
	// The colour rule moved into turnClockColor, which adds the yellow full round clock (N25).
	if s.turnFlow.turnSeconds > 0 && remaining <= 20 {
		color = colorTurnTimerWarn
	}
	*/

	s.drawReadout(buf, TurnTimer, s.hudArt.turnTimerBg, 0, formatClock(remaining), s.turnClockColor(remaining))
}

// turnClockColor picks the turn timer colour: red at 20 seconds or less, yellow while a simultaneous round still
// runs on its full Turn Time (no team has ended yet, so it reads apart from the End Turn countdown), else gold.
// The yellow is a port addition (user 2026-11-14); the reference draws gold and red only.
func (s *Screen) turnClockColor(remaining int) int {
	switch {
	case s.turnFlow.turnSeconds > 0 && remaining <= 20:
		return colorTurnTimerWarn

	case s.simMode() && s.turnFlow.turnSeconds > 0 && !s.turnFlow.graceArmed:
		return int(gfx.ColorYellow)
	}

	return colorTurnText
}

// drawBottomBar paints the map cell under the cursor and the selected unit's name. Empty values remain blank.
// C++: GameManager_DrawMouseCoordinates / GameManager_DrawDisplayPanel.
func (s *Screen) drawBottomBar(buf *gfx.Buffer) {
	coords := ""
	mouse := s.plat.Mouse()

	if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
		coords = fmt.Sprintf("%03d-%03d", cellX+1, cellY+1)
	}

	s.drawReadout(buf, CoordsDisplay, s.hudArt.coordsBg, 21, coords, colorTurnText)

	name := ""

	if s.selected != nil {
		name = displayName(s.selected.Type)

		// A builder under construction shows its product and completed turns.
		if o := s.game.BuildOrderFor(s.selected); o != nil {
			name = fmt.Sprintf("Building %s %d/%d", displayName(o.Product.Type), o.Progress, o.TotalTurns)
		} else if rank := s.commandoRankLabel(s.selected); rank != "" {
			// Commandos show their experience rank, such as "Veteran (+3)".
			name = name + " " + rank
		}
	}

	s.drawReadout(buf, UnitNameDisplay, s.hudArt.unitNameBg, 0, name, colorTurnText)
}

// commandoRankLabel returns a commando's experience rank and level suffix, or an empty string for other unit types.
// C++ ported : game_manager.cpp - GameManager_GetUnitStatusMessage.
func (s *Screen) commandoRankLabel(u *game.UnitInfo) string {
	if u == nil || u.Type != assets.Commando {
		return ""
	}

	rank := u.GetExperience()
	var key, fallback string

	switch {
	case rank > 10:
		key, fallback = "1d69", "Elite"
	case rank > 5:
		key, fallback = "13d9", "Crack"
	case rank > 2:
		key, fallback = "abcf", "Veteran"
	case rank > 0:
		key, fallback = "b38a", "Average"
	default:
		key, fallback = "ea0b", "Rookie"
	}

	label := s.lang.Get(key, fallback)

	if rank > 0 {
		label += fmt.Sprintf(" (+%d)", rank)
	}

	return label
}

// unitDisplayNames gives readable names for unit types shown by the game screen. Tag is the fallback.
var unitDisplayNames = map[assets.UnitType]string{
	// Mobile land units.
	assets.Tank: "Tank", assets.Scout: "Scout Car", assets.Constrct: "Constructor",
	assets.Engineer: "Engineer", assets.Bulldozr: "Bulldozer",
	assets.Surveyor: "Surveyor", assets.Scanner: "Scanner",
	assets.MisslLch: "Missile Launcher", assets.Artillry: "Assault Gun",
	assets.RocktLch: "Rocket Launcher", assets.SpFlak: "SP Flak",
	assets.MineLayr: "Mine Layer", assets.SplyTrck: "Supply Truck",
	assets.GoldTrck: "Gold Truck", assets.FuelTrck: "Fuel Truck",
	assets.Repair: "Repair Unit", assets.ClnTrans: "Personnel Carrier",
	assets.Commando: "Commando", assets.Infantry: "Infantry",
	// Air units.
	assets.Awac: "AWAC", assets.Bomber: "Bomber", assets.Fighter: "Fighter",
	assets.AirTrans: "Air Transport",
	// Sea units.
	assets.FastBoat: "Escort", assets.Corvette: "Corvette",
	assets.Battlshp: "Battleship", assets.Submarne: "Submarine",
	assets.SeaTrans: "Sea Transport", assets.MsslBoat: "Missile Cruiser",
	assets.SeaMnLyr: "Sea Mine Layer", assets.CargoShp: "Cargo Ship",
	// Structures.
	assets.MiningSt: "Mining Station", assets.PowGen: "Power Generator",
	assets.PowerStn: "Power Station", assets.LandPlt: "Heavy Vehicle Plant",
	assets.LightPlt: "Light Vehicle Plant", assets.AirPlt: "Air Units Plant",
	assets.Shipyard: "Shipyard", assets.Radar: "Radar",
	assets.Barracks: "Barracks", assets.Depot: "Depot",
	assets.Habitat: "Habitat", assets.Research: "Research Centre",
	assets.ShieldGn: "Shield Generator", assets.GunTurrt: "Gun Turret",
	assets.AntiAir: "Anti Aircraft", assets.ArtyTrrt: "Artillery",
	assets.AntiMssl: "Missile Turret",
	assets.SuprtPlt: "Life Support", assets.Greenhse: "Eco-sphere",
	assets.RecCentr: "Recreation Center", assets.Hangar: "Hangar",
	assets.Dock: "Dock", assets.TrainHal: "Training Hall",
	assets.CommTwr: "Gold Refinery", assets.ADump: "Storage Unit",
	assets.FDump: "Fuel Tank", assets.GoldSm: "Gold Vault",
	assets.Cnct4W: "Connector", assets.WtrPltfm: "Water Platform",
	assets.Bridge: "Bridge", assets.Road: "Road",
	assets.LandPad: "Landing Pad", assets.Block: "Block",
}

// displayName returns a readable name for a unit type, falling back to its
// resource tag for types not yet in the table.
func displayName(t assets.UnitType) string {
	if n, ok := unitDisplayNames[t]; ok {
		return n
	}

	return t.Tag()
}

// drawEndTurnButton draws the ENDTRN bevel and caption. It is white during the local turn, red while held, and green during another team's turn.
// C++ ported : game_manager.cpp - GameManager_MenuClickEndTurnButton.
func (s *Screen) drawEndTurnButton(buf *gfx.Buffer) {
	s.restoreFrame(buf, EndTurnButton)

	// Held while an opponent acts, and while the human's own End Turn settles an in
	// flight move (input is locked until the mover stops, so show the button pressed).
	opponentTurn := s.turnFlow.aiTurn || s.turnFlow.handover != -1 || s.turnFlow.endTurnSettling
	face := s.hudArt.endTurnUp

	if opponentTurn {
		team := s.game.State.ActiveTeam

		if s.turnFlow.handBackTeam != -1 {
			team = s.turnFlow.aiTurnTeam // settling: keep the last opponent's colour while its end turn voice plays
		}

		if s.turnFlow.handover != -1 {
			team = s.turnFlow.handover // the team about to play (its colour)
		}

		if team >= 0 && team < len(s.hudArt.endTurnDown) && s.hudArt.endTurnDown[team] != nil {
			face = s.hudArt.endTurnDown[team]
		}
	}

	if face != nil {
		w, h := viewSize(EndTurnButton)
		cw, ch := min(face.W, w), min(face.H, h)
		buf.Sub(EndTurnButton.ULX, EndTurnButton.ULY, cw, ch).TransCopy(face.Sub(0, 0, cw, ch))
	} else {
		buf.DrawBox(EndTurnButton.ULX, EndTurnButton.ULY, EndTurnButton.LRX, EndTurnButton.LRY, 0xFF)
	}

	cap := s.hudArt.capEnabled

	switch {
	case opponentTurn || s.gameOver:
		cap = s.hudArt.capDisabled // green while it is someone else's turn / the game is over
	case s.hudArt.endTurnPressed:
		cap = s.hudArt.capPressed // red while the local player holds the button
	}

	w, h := viewSize(EndTurnButton)
	label := s.lang.Get("6af6", "End Turn")
	f := s.sys.Fonts().Get(fonts.Font5)
	tx := EndTurnButton.ULX + (w-f.Width(label))/2
	ty := EndTurnButton.ULY + (h-f.Height)/2

	f.BlitShadowed(buf.Sub(tx, ty, buf.W-tx, buf.H-ty), label, f.Width(label)+1, cap)
}

// drawReadout restores a text slot's background before centring its gold FONT5 value. C++: GameManager_DrawDisplayPanel.
func (s *Screen) drawReadout(buf *gfx.Buffer, r gfx.Rect, slot *gfx.Buffer, leftInset int, text string, color int) {
	w, h := viewSize(r)
	s.restoreFrame(buf, r)

	if slot != nil {
		cw, ch := min(slot.W, w), min(slot.H, h)
		buf.Sub(r.ULX, r.ULY, cw, ch).TransCopy(slot.Sub(0, 0, cw, ch))
	}

	if text == "" {
		return
	}

	f := s.sys.Fonts().Get(fonts.Font5) // centres right of any baked in label (leftInset)
	tx := r.ULX + leftInset + (w-leftInset-f.Width(text))/2
	ty := r.ULY + (h-f.Height)/2

	f.Blit(buf.Sub(tx, ty, buf.W-tx, buf.H-ty), text, f.Width(text)+1, color)
}

// restoreFrame repaints the static FRAMEPIC background below a readout. Rows within the map viewport remain untouched so the map shows through.
func (s *Screen) restoreFrame(buf *gfx.Buffer, r gfx.Rect) {
	if s.bg == nil {
		return
	}

	overMap := r.ULX <= MainMap.LRX && r.LRX >= MainMap.ULX

	if !overMap {
		s.restoreRows(buf, r, r.ULY, r.LRY)
		return
	}

	if r.ULY < MainMap.ULY {
		s.restoreRows(buf, r, r.ULY, min(r.LRY, MainMap.ULY-1))
	}

	if r.LRY > MainMap.LRY {
		s.restoreRows(buf, r, max(r.ULY, MainMap.LRY+1), r.LRY)
	}
}

// restoreRows copies the FRAMEPIC backdrop for rect r's columns over the
// inclusive row span [startY, endY], clipped to the background bounds.
func (s *Screen) restoreRows(buf *gfx.Buffer, r gfx.Rect, startY, endY int) {
	cw := min(r.LRX+1, s.bg.W) - r.ULX
	ch := min(endY+1, s.bg.H) - startY

	if cw > 0 && ch > 0 {
		buf.Sub(r.ULX, startY, cw, ch).Copy(s.bg.Sub(r.ULX, startY, cw, ch))
	}
}

// loadSimpleArt decodes an ImageSimpleHeader resource into a buffer, or returns
// nil when the tag is missing or undecodable so the caller can degrade.
func loadSimpleArt(lib units.Reader, tag string) *gfx.Buffer {
	data, err := lib.Read(tag)

	if err != nil {
		return nil
	}

	img, err := sprites.DecodeSimple(data)

	if err != nil {
		return nil
	}

	return img.Buffer()
}

// bakeButtonLabel centres a caption on a button face with the GNW bevel/shadow
// (brighter, with depth) instead of a flat fill. C++: Button::SetCaption.
func bakeButtonLabel(f *fonts.Font, art *gfx.Buffer, text string, col fonts.Color) {
	if art == nil || f == nil || text == "" {
		return
	}

	x := max((art.W-f.Width(text))/2, 0)
	y := max((art.H-f.Height)/2, 0)
	f.BlitShadowed(art.Sub(x, y, art.W-x, art.H-y), text, art.W-x, col)
}

// repaintBackground restores the full static FRAMEPIC panel art after an overlay draws into bgWindow. The caller follows it with s.draw for dynamic regions.
func (s *Screen) repaintBackground(buf *gfx.Buffer) {
	s.sys.Lock()
	defer s.sys.Unlock()
	s.paintFrame(buf)
}

// paintFrame paints the full static FRAMEPIC frame + menu buttons + panel
// captions into buf. The caller must already hold the screen lock (repaint-
// Background wraps it with the lock; draw() calls it directly while it holds
// the lock, to refresh the frame as the landing panels slide off it).
func (s *Screen) paintFrame(buf *gfx.Buffer) {
	if s.bg != nil {
		w := min(s.bg.W, buf.W)
		h := min(s.bg.H, buf.H)
		buf.Sub(0, 0, w, h).Copy(s.bg.Sub(0, 0, w, h))
	} else {
		buf.Fill(0)
		buf.DrawBox(MainMap.ULX-1, MainMap.ULY-1, MainMap.LRX+1, MainMap.LRY+1, 255)
		buf.DrawBox(Minimap.ULX-1, Minimap.ULY-1, Minimap.LRX+1, Minimap.LRY+1, 255)
	}

	// Top left FILES / PREFERENCES menu buttons, blitted over the frame.
	blitButtonArt(buf, s.hudArt.filesArt, FilesButton)
	blitButtonArt(buf, s.hudArt.prefsArt, PrefsButton)
	// Corner flic play/pause icon buttons (right of the portrait).
	blitButtonArt(buf, s.hudArt.playArt, PlayButton)
	blitButtonArt(buf, s.hudArt.pauseArt, PauseButton)
	// Lower strip Reports / Help buttons.
	blitButtonArt(buf, s.hudArt.helpArt, HelpButton)
	blitButtonArt(buf, s.hudArt.reptArt, ReportsButton)
	// Captions over the control row + the display overlay toggle grid (the
	// FRAMEPIC art draws only the bevels, so the buttons would otherwise be blank).
	s.drawPanelCaptions(buf)
}

// setMenuButtonDown updates a top left menu button's pressed visual: when the
// held state changes it restores the frame under the button and blits the
// pressed (on) or released (off) face. Redraws only on change, so it is cheap
// to call every frame. C++: Button up/down image swap on press.
func (s *Screen) setMenuButtonDown(r gfx.Rect, off, on *gfx.Buffer, state *bool, down bool) {
	if down == *state {
		return
	}

	*state = down
	art := off

	if down {
		art = on
	}

	buf := s.sys.Buf(bgWindow)
	s.sys.Lock()
	defer s.sys.Unlock()

	if s.bg != nil { // restore the frame under the button before the new face
		w := min(r.Width()+1, buf.W-r.ULX)
		h := min(r.Height()+1, buf.H-r.ULY)

		if w > 0 && h > 0 {
			buf.Sub(r.ULX, r.ULY, w, h).Copy(s.bg.Sub(r.ULX, r.ULY, w, h))
		}
	}

	blitButtonArt(buf, art, r)
}

// blitButtonArt transparently blits a button face into the rect (its top left),
// clipped to the buffer. A nil image (resource missing) draws nothing.
func blitButtonArt(buf, art *gfx.Buffer, r gfx.Rect) {
	if art == nil {
		return
	}

	w := min(art.W, buf.W-r.ULX)
	h := min(art.H, buf.H-r.ULY)

	if w <= 0 || h <= 0 {
		return
	}

	buf.Sub(r.ULX, r.ULY, w, h).TransCopy(art.Sub(0, 0, w, h))
}
