// headrotatebench.go : provides the head rotate emulation bench.
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

const headRotateBenchZoom = 40

// DeployHeadRotateBench lays out the -emulate headrotate visual bench: player
// tanks/turrets face east, with static enemies north/south/west/east so a shot
// visibly turns only the turret head. Rocket Launcher and Scout are present as
// body rotation checks; move the tank east afterward to see the turret realign
// with the hull through SetAngle.
// C++: n/a (debug harness for UnitInfo turret_angle / StepTurret rendering).
func (s *Screen) DeployHeadRotateBench() {
	originX, originY, ok := s.snowBlock(12, 10)

	if !ok {
		originX, originY = 5, 5
	}

	const (
		player = 0
		enemy  = 1
	)
	headRotateLand := game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite | game.FlagCanHeadRotate
	bodyRotateLand := game.FlagMobileLandUnit | game.FlagSelectable
	staticTarget := game.FlagMobileLandUnit | game.FlagSelectable
	buildingHead := game.FlagStationary | game.FlagBuilding | game.FlagSelectable | game.FlagTurretSprite | game.FlagCanHeadRotate
	air := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	centerX, centerY := originX+5, originY+4

	tank := s.deployUnit(assets.Tank, headRotateLand, player, 2, centerX, centerY)
	tank.Values.Set(game.AttribRange, 7)
	s.selected = tank

	rocket := s.deployUnit(assets.RocktLch, bodyRotateLand, player, 2, centerX, centerY+2)
	rocket.Values.Set(game.AttribRange, 8)

	scout := s.deployUnit(assets.Scout, bodyRotateLand, player, 2, centerX+3, centerY+2)
	scout.Values.Set(game.AttribRange, 7)

	gun := s.deployUnit(assets.GunTurrt, buildingHead, player, 2, centerX-3, centerY)
	gun.Values.Set(game.AttribRange, 7)

	artillery := s.deployUnit(assets.ArtyTrrt, buildingHead, player, 2, centerX-3, centerY+2)
	artillery.Values.Set(game.AttribRange, 8)

	missile := s.deployUnit(assets.AntiMssl, buildingHead, player, 2, centerX+3, centerY)
	missile.Values.Set(game.AttribRange, 8)

	antiAir := s.deployUnit(assets.AntiAir, buildingHead, player, 2, centerX+3, centerY+4)
	antiAir.Values.Set(game.AttribRange, 7)

	type benchTarget struct {
		t      assets.UnitType
		flags  game.Flags
		x, y   int
		angle  uint8
		airHit bool
	}
	targets := []benchTarget{
		{assets.Tank, staticTarget, centerX, centerY - 3, 4, false},
		{assets.Tank, staticTarget, centerX - 4, centerY, 2, false},
		{assets.Tank, staticTarget, centerX, centerY + 5, 0, false},
		{assets.Tank, staticTarget, centerX + 4, centerY, 6, false},
		{assets.Fighter, air, centerX + 1, centerY + 4, 6, true},
		{assets.Bomber, air, centerX + 4, centerY + 4, 6, true},
	}

	for _, target := range targets {
		u := s.deployUnit(target.t, target.flags, enemy, target.angle, target.x, target.y)
		u.Values.Set(game.AttribSpeed, 0)
		u.Speed = 0
		u.Order = game.OrderAwait
		u.State = game.OrderStateExecutingOrder

		if target.airHit {
			u.AirAlt = game.AirAltMax
		}
	}

	// Keep the opposing side inert; this bench is for manual attack clicks, not AI turns.
	s.freezeBenchTeam(enemy)
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.fogShow = false
	s.showNames = false
	s.r.SetZoom(headRotateBenchZoom)
	s.startSingleBench("Headrotate bench: attack an enemy - the Tank and the turrets turn their heads toward it; the Rocket Launcher and the Scout turn their body.", game.Point{X: centerX, Y: centerY + 1}, tank)
	dbg.Printf("headrotate bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
