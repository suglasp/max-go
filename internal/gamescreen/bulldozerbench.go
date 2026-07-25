// bulldozerbench.go : provides the bulldozer emulation bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// bulldozerBenchZoom shows the rubble clearing bench clearly.
const bulldozerBenchZoom = 48

// DeployBulldozerBench lays out the -emulate bulldozer test bench on snow: two
// player BULLDOZERS, each parked on a fresh rubble pile (a destroyed building's
// large pile and a destroyed tank's small one). Select a bulldozer, long press for
// its popup, and pick Remove to clear the pile under it - the building pile takes 4
// turns, the small one 1. While clearing it shows Stop instead (cancels and frees
// it). The rubble is real sim state (recorded by DestroyUnit), so it is the same
// debris combat leaves. C++: n/a (a debug layout helper for ORDER_CLEAR).
func (s *Screen) DeployBulldozerBench() {
	originX, originY, ok := s.snowBlock(9, 9)

	if !ok {
		originX, originY = 4, 4
	}

	const player = 0
	mobile := game.FlagMobileLandUnit | game.FlagSelectable
	building := game.FlagStationary | game.FlagBuilding | game.FlagSelectable

	// Large pile: deploy a building and blow it up in place to leave LRGRUBLE.
	bx, by := originX+2, originY+4
	bld := s.deployUnit(assets.PowerStn, building, player, 0, bx, by)
	s.game.DestroyUnit(bld)

	// Small pile: deploy a tank and destroy it to leave SMLRUBLE.
	sx, sy := originX+6, originY+4
	tank := s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, player, 0, sx, sy)
	s.game.DestroyUnit(tank)

	// A bulldozer parked on each pile, ready to clear (CanClearRubble = on a pile).
	dozerLarge := s.deployUnit(assets.Bulldozr, mobile, player, 0, bx, by)
	s.deployUnit(assets.Bulldozr, mobile, player, 0, sx, sy)

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = dozerLarge
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(bulldozerBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(originX+4, originY+4, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Bulldozer bench: select a dozer on a pile, popup -> Remove to clear it (building pile 4 turns, small pile 1); end turn to advance, Stop to cancel.")
	dbg.Printf("bulldozer bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
