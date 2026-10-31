// buildexit.go : implements the builder exit overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

// buildexit.go implements the build-ready exit flow. Finished structures show a green label, and a selected parked builder shows BLDMRK markers on its accessible exit cells.
//
// C++ ported : drawmap.cpp - DrawMap_RenderBuildMarker / DrawMap_RedrawUnit +
// the "Finished building" / "New %s available" prompts (game_manager.cpp).

import (
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// loadMarkerFrame decodes a BLDMRK image into a MultiFrame for scaled, centred map drawing. Its first pixel becomes transparent. Missing tags return nil.
func loadMarkerFrame(lib units.Reader, tag string) *sprites.MultiFrame {
	data, err := lib.Read(tag)

	if err != nil {
		return nil
	}

	img, err := sprites.DecodeSimple(data)

	if err != nil {
		return nil
	}

	key := img.TransparentColor()
	f := &sprites.MultiFrame{
		W:      img.W,
		H:      img.H,
		HotX:   img.ULX,
		HotY:   img.ULY,
		Pix:    img.Pix,
		Opaque: make([]bool, len(img.Pix)),
	}

	for i, p := range img.Pix {
		f.Opaque[i] = p != key
	}

	return f
}

// buildMarkFrameMS is the BLDMRK frame duration. The original advances markers every three game ticks, 125 ms at 24 fps.
//
// C++ ported : drawmap.cpp - DrawMap_RenderBuildMarker (DrawMap_BuildMMarkDelayCounter = 3)
const buildMarkFrameMS = 125

// buildMarkFrame returns the current animation frame index for the cycling exit
// markers, or -1 when no marker art is loaded.
func (s *Screen) buildMarkFrame() int {
	if len(s.effects.bldMrk) == 0 {
		return -1
	}

	return int(platform.Ticks()/buildMarkFrameMS) % len(s.effects.bldMrk)
}

// drawBuildReady paints green completion labels and BLDMRK exit markers for the player's build-ready units.
//
// C++ ported : drawmap.cpp - DrawMap_RenderBuildMarker
func (s *Screen) drawBuildReady(view *gfx.Buffer) {
	team := s.game.State.PlayerTeam
	z := s.r.Zoom()

	// Only newly built structures and factory units receive the completion label. A unit leaving storage is not new.
	for _, b := range s.game.BuildReadyUnits(team) {
		br := s.game.BuildReadyInfo(b)

		if br == nil || br.FromStorage {
			continue
		}

		s.drawNewAvailable(view, br, z)
	}

	// Exit markers only show for the selected parked builder and blink with the marker clock.
	sel := s.selected

	if sel == nil || sel.Team != team || !s.game.IsBuildReady(sel) {
		return
	}

	frame := s.buildMarkFrame()

	if frame < 0 {
		return
	}

	mf := s.effects.bldMrk[frame]
	const tile = 64

	for _, c := range s.game.ExitMarkers(sel) {
		sx := c[0]*z + z/2 - s.camera.scrollX
		sy := c[1]*z + z/2 - s.camera.scrollY
		mf.DrawScaledCentered(view, sx, sy, z, tile, nil)
	}
}

// newAvailBlinkMS is the completion-label half-period, producing a roughly one-second blink cycle.
const newAvailBlinkMS = 470

// buildLabelFont chooses the completion-label font for each zoom level. Lower zoom uses a larger font so long product names remain readable.
// C++ ported : drawmap.cpp - DrawMap_RenderTextBox (zoom >= 42 -> FONT_1,
// >= 24 -> FONT_5, else FONT_2).
func buildLabelFont(z int) int {
	switch {
	case z >= 42:
		return fonts.Font1
	case z >= 24:
		return fonts.Font5
	default:
		return fonts.Font2
	}
}

// drawNewAvailable draws a blinking, outlined green completion label centred within the finished structure's footprint. C++: DrawMap_RenderTextBox(COLOR_GREEN | GNW_TEXT_OUTLINE).
func (s *Screen) drawNewAvailable(view *gfx.Buffer, br *sim.BuildReady, z int) {
	if (platform.Ticks()/newAvailBlinkMS)%2 == 1 {
		return // off phase of the slow blink
	}

	f := s.sys.Fonts().Get(buildLabelFont(z))

	if f == nil {
		return
	}

	label := fmt.Sprintf(s.lang.Get("d7d0", "New %s available"), displayName(br.Product))
	lines := wrapToWidth(f, label, br.Footprint*z)

	if len(lines) == 0 {
		return
	}

	lineH := f.Height + 1
	// Centre the stacked block on the footprint centre.
	cx := br.SiteX*z + br.Footprint*z/2 - s.camera.scrollX
	cy := br.SiteY*z + br.Footprint*z/2 - s.camera.scrollY
	startY := cy - len(lines)*lineH/2
	const colGreen = 0x02 // COLOR_GREEN (0x00FF00); FlagOutline outlines it in black

	for i, ln := range lines {
		tw := f.Width(ln)
		tx := cx - tw/2
		ty := startY + i*lineH

		if tx < 0 || ty < 0 || tx >= view.W || ty >= view.H {
			continue
		}

		f.Blit(view.Sub(tx, ty, view.W-tx, view.H-ty), ln, tw+1, colGreen|fonts.FlagOutline)
	}
}

// wrapToWidth greedily keeps words together on lines no wider than maxW pixels. A word wider than maxW occupies a line by itself.
func wrapToWidth(f *fonts.Font, text string, maxW int) []string {
	words := strings.Fields(text)

	if len(words) == 0 {
		return nil
	}

	var lines []string
	cur := words[0]

	for _, w := range words[1:] {
		if f.Width(cur+" "+w) <= maxW {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}

	return append(lines, cur)
}

// tryBuildExit moves a selected build-ready builder when the click lands on an exit marker. Other unit clicks fall through to normal selection. C++: BLDMRK exits the UNIT_READY constructor.
func (s *Screen) tryBuildExit(cellX, cellY int) bool {
	sel := s.selected

	if sel == nil || !s.game.IsBuildReady(sel) {
		return false
	}

	if s.game.ExitBuildSite(sel, cellX, cellY) {
		// tickBuildExpansions plays SfxExpand when it begins the release animation.
		dbg.Printf("build-exit %s to (%d,%d)", displayName(sel.Type), cellX, cellY)
		// Announce the drive-off before selecting the released unit so its lower-priority Ready acknowledgement cannot replace it.
		s.playVoice(vsUnitComplete)

		// Vanilla pops the dropped unit out already selected. Select whichever mobile unit
		// now stands on the marker cell - the factory output, the unloaded depot/hangar/dock
		// unit, OR the constructor that drove off its finished building - through the normal
		// setter so the panel/portrait/voice refresh consistently. A freshly dropped unit is
		// still growing in (Unfold>0), normally not selectable by click until tickBuildExpansions
		// finishes it; the programmatic select bypasses that gate (matching vanilla, which
		// shows the unit selected as it pops out). C++: ORDER_STATE_UNIT_READY exit selection.
		if u := s.droppedUnitAt(cellX, cellY); u != nil {
			s.selectUnit(u)
		}

		return true
	}

	// Not a marker: swallow a click on empty ground (the builder must leave via a
	// marker), but let a click on another unit fall through to select it again.
	return len(s.game.Units.Cells.At(cellX, cellY)) == 0
}

// droppedUnitAt returns the player's selectable mobile unit standing on (cellX, cellY)
// - the unit just driven out of a build site - or nil. The thing that exits a build
// site is ALWAYS mobile (a factory's output, an unloaded transport unit, or the
// constructor/engineer driving off its finished building), so a STATIONARY structure
// sharing the cell is skipped: a unit can land on a passable connector / collector /
// landing pad, and without this guard whichever occupant the cell lists first wins - so
// the exit would select the connector instead of the new unit. Ground cover and other
// teams' units are skipped too, so the exit never lands on a tile decoration.
func (s *Screen) droppedUnitAt(cellX, cellY int) *game.UnitInfo {
	for _, u := range s.game.Units.Cells.At(cellX, cellY) {
		if u.Flags.Has(game.FlagStationary) || u.Flags.Has(game.FlagGroundCover) {
			continue // a structure / connector / tile the unit merely stands on, not the dropped unit
		}

		if u.Team == s.game.State.PlayerTeam && u.Flags.Has(game.FlagSelectable) {
			return u
		}
	}

	return nil
}

// buildExpandFrameMS is the duration of one building unfold step.
const buildExpandFrameMS = 120

// tickBuildExpansions starts released buildings' expand sound and reduces Unfold to full size on the expand clock. C++: UnitsManager_ScaleUnit(EXPAND).
func (s *Screen) tickBuildExpansions() {
	for _, bld := range s.game.ReleasedExpansions() {
		if bld.Team == s.game.State.PlayerTeam {
			s.playSfxForUnit(bld, game.SfxExpand) // "building unfolding" sound
		}

		if s.effects.expanding == nil {
			s.effects.expanding = make(map[*game.UnitInfo]bool)
		}

		s.effects.expanding[bld] = true
	}

	if len(s.effects.expanding) == 0 {
		return
	}

	if now := platform.Ticks(); now-s.effects.expandStamp >= buildExpandFrameMS {
		s.effects.expandStamp = now

		for bld := range s.effects.expanding {
			if bld.Unfold > 0 {
				bld.Unfold--
			}

			if bld.Unfold == 0 {
				delete(s.effects.expanding, bld) // fully grown: it is in place and clickable
			}
		}
	}
}

// announceBuildReady shows the finished-building prompt for each human builder that became ready this turn. C++: menu_finished_building.
func (s *Screen) announceBuildReady(team int) {
	if !s.isHumanTeam(team) {
		return
	}

	for _, b := range s.game.BuildReadyUnits(team) {
		br := s.game.BuildReadyInfo(b)

		if br == nil || br.FromStorage {
			continue // a depot unload is not a "Finished building" event
		}

		// Name the unit the player clicks a marker to move out. A constructor/engineer
		// drives ITSELF off its finished building, but a factory ejects the PRODUCED
		// unit, not the plant, so name that instead.
		mover := b.Type

		if br.Factory {
			mover = br.Product
		}

		s.showMessage(fmt.Sprintf(
			s.lang.Get("276c", "Finished building %s.  To move the %s out of the construction site, click on a square with a symbol in it."),
			displayName(br.Product), displayName(mover)))
	}
}
