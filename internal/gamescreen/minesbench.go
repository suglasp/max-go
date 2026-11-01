// minesbench.go : provides the mines emulation bench.
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

// minesBenchZoom shows the mine laying bench clearly.
const minesBenchZoom = 48

// DeployMinesBench lays out the -emulate mines test bench on snow: a player MINE
// LAYER (loaded with its full mine store) in open ground, flanked by two enemy
// tanks. Select the layer, long press for its popup, and pick Place Mine to drop a
// LandMine on its own cell (no target pick - it mines where it stands); move it a
// cell and lay again to build a minefield. Vanilla mines are STEALTH and PASSABLE:
// an enemy drives onto a hidden mine and it DETONATES (internal/access leaves the
// cell passable, internal/vis hides the mine from a team without a land detector);
// the layer can pick its own back up with Remove Mine (store refills, capped at
// capacity). C++: n/a (a debug layout helper).
func (s *Screen) DeployMinesBench() {
	originX, originY, ok := s.snowBlock(9, 9)

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	// Player mine layer in the centre (deployUnit -> InitState loads its mine store).
	lx, ly := originX+4, originY+4
	layer := s.deployUnit(assets.MineLayr, mobile, player, 2, lx, ly)
	layer.Values.Set(game.AttribSpeed, 16) // roam to lay a line of mines

	// Two enemy tanks a few cells away so a laid mine visibly blocks their path.
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, enemy, 6, originX+1, originY+4)
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, enemy, 6, originX+7, originY+4)

	s.game.State.Teams[enemy].Type = game.TeamPlayer
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = layer
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(minesBenchZoom)
	s.startSingleBench("Mines bench: select the layer, popup -> Place Mine (lays on its own cell); move + lay to build a minefield; Remove Mine picks it back up.", game.Point{X: originX + 4, Y: originY + 4}, layer)
	dbg.Printf("mines bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
