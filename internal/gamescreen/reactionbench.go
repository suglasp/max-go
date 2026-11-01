// reactionbench.go : provides the reaction fire bench for the reaction voice hand check (H1).
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

// reactionBenchZoom frames one pair: the turret, the approaching Tank and the land between.
const reactionBenchZoom = 32

// The cells beyond the turret's range each enemy Tank starts at: the near pair fights first, the far pair a few
// seconds later, so the two fights are told apart.
const (
	reactionBenchNearLead = 4
	reactionBenchFarLead  = 12
)

// reactionBenchRadius is how far from its compass anchor a pair's long land strip may lie.
const reactionBenchRadius = 24

// DeployReactionFireBench lays out the -emulate reactionfire bench for hand check H1, the reaction fire voice:
// two pairs, each a player Gun Turret on Sentry and a frozen enemy Tank that drives in to attack it. Driving
// spends the Tank's shot (moving and firing share one budget), so it fires back after End Turn.
//   - NEAR (stop 1, under the camera): the turret fires back on screen: the red banner shows, no voice.
//   - FAR (stop 2, out of view): the turret fires back off screen and unselected: banner and the voice.
//
// The far Tank starts further out, so its fight comes a few seconds after the near one.
//
// C++: n/a (debug helper).
func (s *Screen) DeployReactionFireBench() {
	const player, enemy = 0, 1
	tankFlags := game.FlagMobileLandUnit | game.FlagSelectable
	turretFlags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable | game.FlagTurretSprite | game.FlagCanHeadRotate

	turretRange := s.deployValues(player, assets.GunTurrt).Get(game.AttribRange)
	if turretRange < 1 {
		turretRange = 5
	}

	pairs := []struct {
		lead  int
		title string
		text  string
	}{
		{reactionBenchNearLead, "H1 on screen",
			"the enemy Tank drives in; as it enters range the Gun Turret fires back: red banner, NO reaction voice. Stay on this view and select nothing"},
		{reactionBenchFarLead, "H1 off screen",
			"this pair fights a few seconds after the near one, out of view: its red banner shows AND you hear the reaction voice. The Tanks shoot back after End Turn"},
	}

	var turrets, tanks []*game.UnitInfo

	for _, p := range pairs {
		distance := turretRange + p.lead
		var blockX, blockY int

		compass, ok := s.claimAnchor(func(anchor game.Point) bool {
			x, y, fits := s.landBlockNear(anchor, distance+3, 3, reactionBenchRadius)
			blockX, blockY = x, y
			return fits
		})
		if !ok {
			dbg.Printf("reactionfire bench: no free compass anchor has a %dx3 land strip for %s", distance+3, p.title)
			continue
		}

		// The turret faces east, on guard so it answers in simultaneous play too, and it must see the Tank to fire.
		turret := s.benchPlaceTurret(assets.GunTurrt, turretFlags, player, 2, blockX+1, blockY+1)
		turret.Order = game.OrderSentry
		turret.State = game.OrderStateExecutingOrder
		if turret.Values.Get(game.AttribScan) <= turretRange {
			turret.Values.Set(game.AttribScan, turretRange+2)
		}

		// The Tank faces west, reaches its range in one turn and sees the turret it drives at.
		tank := s.deployUnit(assets.Tank, tankFlags, enemy, 6, blockX+1+distance, blockY+1)
		tank.Values.Set(game.AttribSpeed, distance+4)
		tank.Values.Set(game.AttribScan, distance+2)
		tank.InitState()

		turrets = append(turrets, turret)
		tanks = append(tanks, tank)
		s.addBenchStop(benchStop{Compass: compass, Title: p.title, Text: p.text, Cell: game.Point{X: blockX + 1 + distance/2, Y: blockY + 1}})
		dbg.Printf("reactionfire bench: %s %s turret (%d,%d) range %d, tank (%d,%d) %d cells out",
			compass, p.title, turret.GridX, turret.GridY, turretRange, tank.GridX, tank.GridY, distance)
	}

	s.freezeBenchTeam(enemy) // no AI: only the scripted attack below moves the Tanks
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.showNames = true
	s.fogShow = false
	s.game.RefreshVisibility()

	for i, tank := range tanks {
		if !s.game.OrderMoveToAttack(tank, turrets[i].GridX, turrets[i].GridY) {
			dbg.Printf("reactionfire bench: the Tank at (%d,%d) could not be ordered to attack", tank.GridX, tank.GridY)
		}
	}

	s.r.SetZoom(reactionBenchZoom)
	s.startBench("Reaction fire bench (H1). Stay on this view and select nothing until both fights are done; a selected turret stays silent too. Tab afterwards to see the far pair.")
}
