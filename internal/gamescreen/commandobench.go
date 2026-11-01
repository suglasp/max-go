// commandobench.go : provides the commando emulation bench.
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

// commandoBenchZoom shows the small infiltration bench clearly.
const commandoBenchZoom = 48

// DeployCommandoBench lays out the -emulate commando test bench on snow: a player
// COMMANDO in open ground with three enemy units a few cells away, so the
// disable/steal mechanic can be exercised by hand. Select the commando, walk it
// next to a target, long press for its popup, pick Disable or Steal, then click
// the adjacent enemy:
//   - ENEMY TANK (mobile electronic), north  -> Disable OR Steal (steal flips its
//     team colour; disable greys it out for a few turns),
//   - ENEMY RADAR (2x2 powered building), southeast -> Disable only (a building
//     cannot be stolen; disabling it drops it out of production),
//   - ENEMY INFANTRY (organic), west          -> neither (the no go cursor shows;
//     it is not an electronic unit).
//
// The enemies start a few cells away so the commando must MOVE adjacent first
// (proving movement and the adjacency rule). The commando carries extra shots,
// high experience and a generous move budget so it can roam the bench and land
// repeated attempts. Press Return to end the turn and watch a disabled unit count
// down and recover. C++: n/a (a debug layout helper).
func (s *Screen) DeployCommandoBench() {
	originX, originY, ok := s.snowBlock(9, 9)

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	// Commando in the centre with every neighbour cell free so it moves freely.
	cx, cy := originX+4, originY+4
	commando := s.deployUnit(assets.Commando, mobile, player, 2, cx, cy)
	commando.Values.Set(game.AttribRounds, 4)
	commando.Values.Set(game.AttribSpeed, 24) // roam the whole bench in one turn
	commando.Shots = 4
	commando.Speed = 24
	commando.Experience = 50 // high success chance so the effect is easy to see

	// Enemies a few cells away in three directions (open lanes between).
	s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite|game.FlagElectronicUnit, enemy, 4, originX+4, originY+1) // N: disable/steal
	s.deployUnit(assets.Infantry, mobile, enemy, 2, originX+1, originY+4)                                           // W: immune (organic)
	s.placeElectronicBuilding(assets.Radar, enemy, originX+6, originY+6)                                            // SE: disable only

	// Team 1 is a live opponent so its units are valid enemy targets and Return
	// hands the turn over (watch a disabled unit recover on its turn).
	s.freezeBenchTeam(enemy)
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = commando
	s.showNames = true // label the units (the NAMES panel toggle reads checked)
	s.fogShow = false
	s.r.SetZoom(commandoBenchZoom)
	s.startSingleBench("Commando bench: move next to an enemy, then popup -> Disable/Steal (tank: both, radar: disable, infantry: immune).", game.Point{X: originX + 4, Y: originY + 4}, commando)
	dbg.Printf("commando bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}

// placeElectronicBuilding deploys a powered 2x2 electronic building for the
// commando bench (a valid disable target). C++: n/a (debug helper).
func (s *Screen) placeElectronicBuilding(t assets.UnitType, team, gx, gy int) {
	u := game.NewUnitInfo(t, team, s.spawnSeq+1, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable | game.FlagElectronicUnit
	u.Values = s.teamValues(team, t)
	u.InitState()
	u.Order = game.OrderPowerOn
	u.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(u, gx, gy)
}
