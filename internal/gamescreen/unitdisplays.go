// unitdisplays.go : implements the per unit display overlays.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: The per unit display overlays toggled from the left panel's 3x3 button grid:
// COLORS (a team colour bracket), STATUS (speed/shots/disabled icons), HITS and
// AMMO (proportional bars), and the RANGE/SCAN range rings. Each draws an extra
// layer over the on screen units, matching the vanilla display button overlays.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"sort"
)

// drawUnitDisplays paints the COLORS / STATUS / HITS / AMMO overlays over every
// on screen selectable unit when their panel toggles are on. It mirrors the
// vanilla DrawMap_RenderUnitDisplays pass: a team colour bracket, the speed/shots
// status icons, and the stacked hit point / ammo bars.
//
// C++ ported : drawmap.cpp - DrawMap_RenderUnitDisplays
func (s *Screen) drawUnitDisplays(view *gfx.Buffer) {
	// Nothing to do unless at least one of the four bar/icon toggles is on.
	if !s.showColors && !s.showStatus && !s.showHits && !s.showAmmo {
		return
	}

	z := s.r.Zoom()
	player := s.game.State.PlayerTeam

	for _, u := range s.unitDrawOrder() {
		// Ground cover (roads, rubble) never carries a unit display.
		if u.Flags.Has(game.FlagGroundCover) || !u.Flags.Has(game.FlagSelectable) {
			continue
		}

		// Only a unit the player can see gets a display: not a building hidden behind its parked builder, not an
		// undetected stealth unit (a submerged Submarine), not an enemy in the fog. The sprite pass skips them too.
		if u.HiddenUntilExpand || s.stealthHidden(u) || (s.fogShow && !s.unitVisibleToPlayer(u)) {
			continue
		}

		// Footprint: a 2x2 building, else a single cell. Screen rect at the zoom.
		n := 1

		if u.Flags.Has(game.FlagBuilding) {
			n = 2
		}

		sx := u.GridX*z - s.camera.scrollX
		sy := u.GridY*z - s.camera.scrollY
		fp := n * z

		// Skip a unit fully off the viewport (cheap bounds reject).
		if sx+fp <= 0 || sy+fp <= 0 || sx >= view.W || sy >= view.H {
			continue
		}

		// COLORS: a hollow team colour box around a mobile unit (stationary
		// buildings keep the plain frame, matching the original's !STATIONARY gate).
		// Use the same per team palette index the minimap dots use (red/green/
		// blue/grey, else yellow for aliens) - reportTeamColor mapped team 2 to
		// ColorYellow (0x04) instead of ColorBlue (0x03), so blue units boxed yellow.
		if s.showColors && !u.Flags.Has(game.FlagStationary) {
			col := s.dotColor(u.Team)
			boxClipped(view, sx, sy, fp, fp, col)
		}

		// STATUS: the speed/shots/disabled icons centred in the footprint.
		if s.showStatus {
			s.drawUnitStatusIcons(view, u, sx, sy, fp)
		}

		// HITS / AMMO: the stacked proportional bars, only at the closer zooms
		// (the original gates the bars on Gfx_ZoomLevel > 8).
		if z > 8 && (s.showHits || s.showAmmo) {
			s.drawUnitBars(view, u, player, sx, sy, fp)
		}
	}
}

// drawUnitBars stacks the hit point bar (and, for a player unit, the ammo bar)
// near the bottom of the unit footprint. Each bar is a black trough with a
// fill proportional to the value, coloured green/yellow/red by the fraction.
//
// C++ ported : drawmap.cpp - DrawMap_RenderUnitDisplays (the i=0/1 bar loop)
func (s *Screen) drawUnitBars(view *gfx.Buffer, u *game.UnitInfo, player, sx, sy, fp int) {
	if u.Values == nil {
		return
	}

	// Bar geometry: full width inset 1px, height a fraction of the footprint
	// (min 3px), the pair anchored just above the footprint's bottom edge.
	barH := fp / 8

	if barH < 3 {
		barH = 3
	}

	barW := fp - 2

	if barW < 3 {
		return
	}

	barX := sx + 1
	barY := sy + fp - 2*barH - 1

	// Two passes: 0 = hits (any team), 1 = ammo (player team only).
	for pass := 0; pass < 2; pass++ {
		valueMax := 0
		valueActual := 0

		if pass == 0 && s.showHits {
			// Hit points: shown for every visible unit.
			valueMax = u.Values.Get(game.AttribHits)
			valueActual = min(u.Hits, valueMax)
		} else if pass == 1 && s.showAmmo && u.Team == player {
			// Ammo: the original only reveals the player's own ammunition.
			valueMax = u.Values.Get(game.AttribAmmo)
			valueActual = min(u.Ammo, valueMax)
		}

		if valueMax > 0 {
			// The black trough, then the proportional coloured fill.
			fillRectClipped(view, barX, barY, barW, barH, gfx.ColorBlack)
			fillW := (barW - 2) * valueActual / valueMax

			if fillW > 0 {
				fillRectClipped(view, barX+1, barY+1, fillW, barH-2, barFillColor(valueActual, valueMax))
			}

			// Stack the next bar directly under this one (one row overlap, as in the original).
			barY += barH - 1
		}
	}
}

// barFillColor picks the bar fill shade: green above half, yellow above a
// quarter, red below. C++: the value_max/2 and value_max/4 thresholds.
func barFillColor(actual, max int) uint8 {
	if actual > max/2 {
		return gfx.ColorGreen
	}

	if actual > max/4 {
		return gfx.ColorYellow
	}

	return gfx.ColorRed
}

// drawUnitStatusIcons blits the speed (IL_SPEED) and shots (IL_SHOTS) icons, or
// the disabled (IL_DSBLD) icon, centred in the unit footprint. They show, at a
// glance, whether the unit still has moves or shots this turn.
//
// C++ ported : drawmap.cpp - DrawMap_RenderStatusDisplay
func (s *Screen) drawUnitStatusIcons(view *gfx.Buffer, u *game.UnitInfo, sx, sy, fp int) {
	// A disabled unit shows only the single "disabled" icon, centred.
	if u.Order == game.OrderDisable {
		if img := s.statusIcon("IL_DSBLD"); img != nil {
			blitTransClipped(view, img, sx+(fp-img.W)/2, sy+(fp-img.H)/2)
		}

		return
	}

	// Otherwise gather the icons the unit currently warrants: it can still move
	// (speed) and/or still fire (shots).
	var icons []*gfx.Buffer

	if u.Speed > 0 {
		if img := s.statusIcon("IL_SPEED"); img != nil {
			icons = append(icons, img)
		}
	}

	if u.Shots > 0 {
		if img := s.statusIcon("IL_SHOTS"); img != nil {
			icons = append(icons, img)
		}
	}

	if len(icons) == 0 {
		return
	}

	// Lay the icons out in a centred row along the bottom of the footprint.
	totalW := 0
	maxH := 0

	for _, img := range icons {
		totalW += img.W + 1

		if img.H > maxH {
			maxH = img.H
		}
	}

	totalW--
	iconX := sx + (fp-totalW)/2
	iconY := sy + fp - maxH - 1

	for _, img := range icons {
		blitTransClipped(view, img, iconX, iconY)
		iconX += img.W + 1
	}
}

// statusIcon loads and caches a STATUS overlay icon (IL_SPEED/IL_SHOTS/IL_DSBLD)
// with its fuchsia key made transparent, returning nil when the tag is absent.
func (s *Screen) statusIcon(tag string) *gfx.Buffer {
	if s.mapArt.statusIcons == nil {
		s.mapArt.statusIcons = map[string]*gfx.Buffer{}
	}

	if img, ok := s.mapArt.statusIcons[tag]; ok {
		return img
	}

	img := loadSimpleArt(s.lib, tag)

	if img != nil {
		// MAX stores an image's transparent colour as its top left pixel; key it
		// to 0 so the transparent blit drops it (same as the resource markers).
		img.KeyToZero(img.At(0, 0))
	}

	s.mapArt.statusIcons[tag] = img
	return img
}

// drawUnitRangeRings outlines each player unit's weapon range (RANGE, red) and
// scan range (SCAN, yellow) when the matching panel toggles are on. Only the
// player's own units are ringed, so the overlay reads as "my coverage".
//
// C++ ported : unitinfo.cpp - the DisplayButtonRange / DisplayButtonScan rings
func (s *Screen) drawUnitRangeRings(view *gfx.Buffer) {
	if !s.showRange && !s.showScan {
		return
	}

	z := s.r.Zoom()
	player := s.game.State.PlayerTeam
	const tile = 64

	for _, u := range s.game.Units.Units() {
		// Friendly, selectable units with attribute data only.
		if u.Team != player || u.Values == nil || !u.Flags.Has(game.FlagSelectable) {
			continue
		}

		// The ring is centred on the unit's pixel position (world px -> screen px).
		centerX := u.X*z/tile - s.camera.scrollX
		centerY := u.Y*z/tile - s.camera.scrollY

		if s.showRange {
			// Weapon range ring (red): only a unit that actually has a weapon.
			if rng := u.Values.Get(game.AttribRange); rng > 0 {
				view.DrawCircleClip(centerX, centerY, rng*z, gfx.ColorRed)
			}
		}

		if s.showScan {
			// Sensor range ring (yellow): how far the unit reveals the map.
			if scan := u.Values.Get(game.AttribScan); scan > 0 {
				view.DrawCircleClip(centerX, centerY, scan*z, gfx.ColorYellow)
			}
		}
	}
}

// drawSentryCoverage rings the ground the player's Sentry units are guarding: one
// green circle per sentry unit at its weapon range, so the holes in a defensive line
// show up at a glance. It is toggled from the F2 developer menu.
//
// The original has no such overlay, only the per unit Range and Scan rings. This is a
// port addition borrowed from MaxR's sentry map, and it matters more now that games
// run in simultaneous mode, where a sentry line does its work while the player is
// busy elsewhere. Reaction fire is what the ring actually predicts, so a unit with no
// weapon is skipped: it is standing guard over nothing.
func (s *Screen) drawSentryCoverage(view *gfx.Buffer) {
	if !s.showSentryCoverage {
		return
	}

	z := s.r.Zoom()
	player := s.game.State.PlayerTeam
	const tile = 64

	for _, u := range s.game.Units.Units() {
		if u.Team != player || u.Values == nil || u.Order != game.OrderSentry {
			continue
		}

		rng := u.Values.Get(game.AttribRange)

		if rng <= 0 {
			continue
		}

		centerX := u.X*z/tile - s.camera.scrollX
		centerY := u.Y*z/tile - s.camera.scrollY

		view.DrawCircleClip(centerX, centerY, rng*z, gfx.ColorGreen)
	}
}

// boxClipped draws a hollow rectangle (top left at x,y, size w x h) clipped to the
// buffer, so an edge crossing the viewport border is safe. GNW's DrawBox does not
// clip, so the overlays use this guarded variant.
func boxClipped(view *gfx.Buffer, x, y, w, h int, col uint8) {
	if w <= 0 || h <= 0 {
		return
	}

	// Each edge is a one pixel thick clipped fill, so corners stay square.
	fillRectClipped(view, x, y, w, 1, col)     // top
	fillRectClipped(view, x, y+h-1, w, 1, col) // bottom
	fillRectClipped(view, x, y, 1, h, col)     // left
	fillRectClipped(view, x+w-1, y, 1, h, col) // right
}

// fillRectClipped fills the rectangle (top left x,y, size w x h) with col,
// clipped to the buffer bounds. C++: buf_fill after the DrawMap edge clamps.
func fillRectClipped(view *gfx.Buffer, x, y, w, h int, col uint8) {
	// Clamp the rectangle to the buffer; bail when it falls entirely outside.
	if x < 0 {
		w += x
		x = 0
	}

	if y < 0 {
		h += y
		y = 0
	}

	if x+w > view.W {
		w = view.W - x
	}

	if y+h > view.H {
		h = view.H - y
	}

	if w <= 0 || h <= 0 {
		return
	}

	view.Sub(x, y, w, h).Fill(col)
}

// unitDrawOrder returns every unit in a stable painter's order: back to front by
// cell (GridY then GridX), ground cover under whatever shares its cell. Units()
// is map backed random order, so both the sprite pass and name overlay sort
// through this to keep overlapping draws from flickering between frames.
func (s *Screen) unitDrawOrder() []*game.UnitInfo {
	order := s.game.Units.Units()
	sort.SliceStable(order, func(i, j int) bool {
		// Ground cover (slabs/roads/rubble, flat bridge) draws first, under EVERY
		// unit, not just at the same cell, else a unit crossing it flickers above/below.
		// A RAISED bridge is excluded (draws over the unit passing under).
		gi := groundCoverLayer(order[i])
		gj := groundCoverLayer(order[j])

		if gi != gj {
			return gi // ground cover before any non ground cover unit
		}

		// A RAISED bridge is a TOP layer: draws over every other unit regardless of row.
		// C++: the elevated_bridge draw list runs after sea_land_units.
		ebi := isElevatedBridge(order[i])
		ebj := isElevatedBridge(order[j])

		if ebi != ebj {
			return ebj // the raised bridge sorts last (drawn on top of the other unit)
		}

		if order[i].GridY != order[j].GridY {
			return order[i].GridY < order[j].GridY
		}

		if order[i].GridX != order[j].GridX {
			return order[i].GridX < order[j].GridX
		}

		// Same cell: paint by stacking rank (a mine under the unit on it; a ground/sea
		// unit under a connector; an aircraft over it). ID tie break avoids flicker.
		ri, rj := sameCellRank(order[i]), sameCellRank(order[j])

		if ri != rj {
			return ri < rj
		}

		return order[i].ID < order[j].ID
	})
	return order
}

// stealthHidden reports whether u must be hidden from the human view this frame:
// an enemy STEALTH unit (laid mine / submerged / commando) the viewing team
// (s.fogTeam, the team drawFog renders) cannot detect. Only applies while the fog
// overlay is active - a revealed map (benches / the F2 cheat) shows everything.
func (s *Screen) stealthHidden(u *game.UnitInfo) bool {
	return s.fogShow && s.game.Vis != nil && s.game.Vis.StealthHiddenFrom(u, s.fogTeam)
}

func (s *Screen) drawUnits(view *gfx.Buffer) {
	order := s.unitDrawOrder()
	s.refreshConnectors() // recompute each structure's connection bits for the auto rig overlay
	z := s.r.Zoom()
	// The construction overlay (slab/tape/cones) is a layer-3 floor effect: over the
	// layer-2 floor (water platform/flat bridge) but under units (layer 6). Injected
	// at the boundary between floor units and the rest. C++: UnitInfo::GetDrawLayer.
	overlaid := false

	for _, u := range order {
		if !overlaid && !groundCoverLayer(u) {
			s.drawConstruction(view)
			overlaid = true
		}

		if u.HiddenUntilExpand {
			continue // a parked but not yet exited factory/depot/builder output stays hidden until it materialises on its exit marker
		}

		if s.stealthHidden(u) {
			continue // an undetected enemy mine / sub / commando is invisible to this team
		}

		s.assignSprite(u)
		s.sprites.DrawUnit(view, u, z, s.camera.scrollX, s.camera.scrollY)
	}

	if !overlaid {
		s.drawConstruction(view) // only floor layer units (or none): the overlay sits on top of them
	}

	for _, u := range s.selectionSet() { // brackets on every selected unit (leader + group)
		s.drawSelection(view, u, z)
	}

	s.drawMarquee(view) // the yellow drag box while rubber banding a group
}

// markerBlinkMS is the selection marker blink half period: the original
// toggles GameManager_MarkerColor every TIMER_FPS_TO_MS(24)*5 ms (~208 ms).
const markerBlinkMS = 208

// markerColor blinks the selection brackets between white (0xFF) and black (0)
// on that cadence, the GameManager_MarkerColor ^= 0xFF animation.
func markerColor() uint8 {
	if (platform.Ticks()/markerBlinkMS)%2 == 1 {
		return 0
	}

	return 0xFF
}

// drawSelection draws the blinking corner bracket marker around the selected
// unit: four L shaped corner brackets in three nested sets, toggling white/black
// every ~208ms. One cell for a mobile unit, 2x2 for a building.
// C++: GameManager_DrawUnitSelector (double_marker=true), DrawMap call site.
func (s *Screen) drawSelection(view *gfx.Buffer, u *game.UnitInfo, z int) {
	n := 1

	if u.Flags.Has(game.FlagBuilding) {
		n = 2
	}

	// Centre the marker on the unit's pixel position (not its grid cell) so it
	// tracks a unit that renders off its cell centre; a moving unit between
	// cells, or a constructor sitting at the 2x2 build site centre. A mobile
	// unit keeps its 1x1 marker; a building gets 2x2. C++: GameManager_
	// DrawUnitSelector centres on the unit's x/y.
	const tile = 64
	cxp := u.X*z/tile - s.camera.scrollX
	cyp := u.Y*z/tile - s.camera.scrollY
	startX := cxp - n*z/2
	startY := cyp - n*z/2
	endX := startX + n*z - 1
	endY := startY + n*z - 1

	if endX < 0 || endY < 0 || startX >= view.W || startY >= view.H {
		return
	}

	arm := n * z / 4

	if arm < 2 {
		arm = 2
	}

	// double_marker: three nested bracket sets whose colour alternates each
	// level (the C++ `color ^= 0xFF` per loop), so the bracket reads as a
	// white+black double line; the whole pattern inverts on the blink.
	c := markerColor()

	for i := 0; i < 3; i++ {
		drawCornerMarker(view, startX+i, startY+i, endX-i, endY-i, arm-i, c)
		c ^= 0xFF
	}
}

// drawCornerMarker draws four L shaped corner brackets at the corners of the
// (startX,startY)-(endX,endY) box, each arm `arm` pixels long, clipped to the view.
func drawCornerMarker(view *gfx.Buffer, startX, startY, endX, endY, arm int, c uint8) {
	set := func(x, y int) {
		if x >= 0 && x < view.W && y >= 0 && y < view.H {
			view.Set(x, y, c)
		}
	}

	for i := 0; i <= arm; i++ {
		set(startX+i, startY) // top left: right + down arms
		set(startX, startY+i)
		set(endX-i, startY) // top right: left + down arms
		set(endX, startY+i)
		set(startX+i, endY) // bottom left: right + up arms
		set(startX, endY-i)
		set(endX-i, endY) // bottom right: left + up arms
		set(endX, endY-i)
	}
}
