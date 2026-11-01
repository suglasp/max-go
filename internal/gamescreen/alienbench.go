// alienbench.go : provides the alien unit emulation bench.
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

// alienBenchZoom shows the whole alien derelict roster (4 units + 3 buildings) at once.
const alienBenchZoom = 32

// DeployAliensBench lays out the -emulate aliens bench: the full neutral alien derelict
// roster (owned by the fixed game.AlienTeam) - the four alien units (tank, assault gun,
// juggernaut, plane) on a top row and ALL THREE alien artifact buildings (Shield Generator,
// Life Support [SUPRTPLT], Recreation Center) on a bottom row, next to a player tank. These
// are the original's GameManager_AlienUnits + GameManager_AlienBuildings. The derelicts render in the
// dedicated DERELICT colour ramp, never move/fire/take a turn (inert until attacked), and a
// player can drive up and attack or salvage one. C++: n/a (debug layout).
func (s *Screen) DeployAliensBench() {
	originX, originY, ok := s.snowBlock(14, 11)

	if !ok {
		originX, originY = 4, 4
	}

	const player = 0
	mobile := game.FlagMobileLandUnit | game.FlagSelectable
	air := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	building := game.FlagStationary | game.FlagBuilding | game.FlagSelectable

	// The four alien mobile derelicts (top row), owned by the fixed alien slot.
	s.deployUnit(assets.AlnTank, mobile, game.AlienTeam, 2, originX+4, originY+2)
	s.deployUnit(assets.AlnAsGun, mobile, game.AlienTeam, 2, originX+6, originY+2)
	s.deployUnit(assets.Juggrnt, mobile, game.AlienTeam, 2, originX+8, originY+2)
	s.deployUnit(assets.AlnPlane, air, game.AlienTeam, 2, originX+10, originY+2)

	// All three alien artifact buildings (bottom row, each 2x2).
	s.placeStartBuilding(game.AlienTeam, assets.ShieldGn, building, originX+2, originY+6)
	s.placeStartBuilding(game.AlienTeam, assets.SuprtPlt, building, originX+6, originY+6)
	s.placeStartBuilding(game.AlienTeam, assets.RecCentr, building, originX+10, originY+6)

	// A player tank so the derelicts can be inspected and attacked.
	tank := s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, player, 6, originX, originY+4)

	s.game.State.Teams[player].Type = game.TeamPlayer
	s.game.State.Teams[game.AlienTeam].Type = game.TeamAlien
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = tank
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(alienBenchZoom)
	s.startSingleBench("Aliens bench: the neutral derelicts (alien colour) - 4 units + 3 artifact buildings - never move or fire; drive the player tank up and attack one to salvage it.", game.Point{X: originX + 6, Y: originY + 4}, tank)
	dbg.Printf("aliens bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
