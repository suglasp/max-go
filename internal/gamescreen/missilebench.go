// missilebench.go : provides the missile emulation bench.
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

// missileBenchZoom is a little zoomed out so a missile's whole flight is visible.
const missileBenchZoom = 36

// missileBenchRange is the reach given to every bench launcher so the missile
// flies a long, visible arc (the generic deploy range is only 5).
const missileBenchRange = 9

// missileBenchAttack overpowers the bench launchers so a single missile destroys
// any enemy (24 hits) outright - the point of the bench is to watch the DEATH blast
// (LNDEXPLD on a unit, BLDEXPLD on a building), not to whittle a target down over
// three turns (which only shows the small HITEXPLD hit bloom).
const missileBenchAttack = 80

// DeployMissileBench lays out the -emulate missile test bench: every missile firing
// unit class with static enemy targets to shoot (the enemy team's AI is LOCKED OFF,
// so they sit still). LAND launchers (Rocket Launcher + Missile Crawler) + an AIR
// Bomber fire rockets east at a column of enemy tanks; SEA units (Submarine,
// Corvette, Missile Boat) sit in the nearest open water and torpedo/rocket a pair of
// enemy boats. Select any one and attack -> watch the missile fly + trail + explode.
// C++: n/a (a debug layout helper for the FIRES_MISSILES units).
func (s *Screen) DeployMissileBench() {
	const (
		player = 0
		enemy  = 1
	)
	turret := game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	air := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	originX, originY, ok := s.snowBlock(16, 16)

	if !ok {
		originX, originY = 6, 6
	}

	cx, cy := originX+3, originY+6

	// Player LAND launchers + AIR bomber, firing east; a column of enemy tanks in range.
	rl := s.deployUnit(assets.RocktLch, turret, player, 2, cx, cy-2)
	ml := s.deployUnit(assets.MisslLch, turret, player, 2, cx, cy)
	bomber := s.deployUnit(assets.Bomber, air, player, 2, cx, cy+2)

	for _, u := range []*game.UnitInfo{rl, ml, bomber} {
		u.Values.Set(game.AttribRange, missileBenchRange)
		u.Values.Set(game.AttribAttack, missileBenchAttack) // kill any bench enemy in one shot so the DEATH blast always plays
	}

	// The Rocket Launcher is the ONLY retail unit with a blast radius (ATTRIBS Area=2);
	// deployUnit builds blank values, so set it here to demonstrate SPLASH/collateral -
	// a rocket on one building also damages the neighbours (friend or foe) in radius.
	rl.Values.Set(game.AttribAttackRadius, 2)

	for i := 0; i < 3; i++ {
		s.deployUnit(assets.Tank, turret, enemy, 6, cx+6, cy-2+2*i)
	}

	// A Supply Truck parked beside the Rocket Launcher to demo field RELOAD: fire the
	// launcher dry (Attack vanishes when Ammo hits 0), then select the truck -> Reload
	// -> click the launcher and its ammo bar refills (one material per reload).
	truck := s.deployUnit(assets.SplyTrck, game.FlagMobileLandUnit|game.FlagSelectable, player, 2, cx-1, cy-2)
	truck.Storage = 5 // a few materials to hand out

	// A tight cluster of enemy BUILDINGS (mining station + factory + small generator)
	// so a missile can be fired AT structures - and, once splash/area damage lands, to
	// watch one impact damage the adjacent buildings. Placed adjacent on purpose.
	// Use PlaceBuilding so each structure receives its slab and joins the enemy complex.
	s.benchPlaceBuilding(assets.MiningSt, enemy, cx+6, cy+4, true) // 2x2 mining station
	s.benchPlaceBuilding(assets.LandPlt, enemy, cx+8, cy+4, true)  // 2x2 factory (vehicle plant)
	s.benchPlaceBuilding(assets.PowGen, enemy, cx+6, cy+6, false)  // 1x1 small generator

	// Player SEA missile units + enemy boats, in the nearest open water (best effort:
	// skipped on a landlocked map).
	s.deployMissileSea(player, enemy, cx, cy)

	s.freezeBenchTeam(enemy) // LOCK the AI: enemies are static targets
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.selected = rl
	s.showNames = true
	s.fogShow = false
	s.r.SetZoom(missileBenchZoom)
	s.startSingleBench("Missile bench (AI off): select a launcher - Rocket/Missile (land), Sub/Corvette/Boat (sea, scroll to the water), Bomber (air) - and attack an enemy tank or building; the missile flies + explodes. Fire the Rocket Launcher dry, then use the Supply Truck's Reload to refill it.", game.Point{X: cx + 3, Y: cy}, rl)
	dbg.Printf("missile bench: land block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}

// deployMissileSea places the player's sea missile units (Submarine, Corvette,
// Missile Boat) in the nearest open water near (fromX, fromY) and two enemy boats a
// few cells away as torpedo/missile targets. Does nothing when no open water is in range
// (a landlocked map) - the land + air part still works.
func (s *Screen) deployMissileSea(player, enemy, fromX, fromY int) {
	sea := game.FlagMobileSeaUnit | game.FlagSelectable
	taken := map[game.Point]bool{}
	nextWater := func(from game.Point, maxR int) (int, int, bool) {
		return s.nearestCell(from, maxR, func(x, y int) bool {
			return !taken[game.Point{X: x, Y: y}] && s.isOpenWater(x, y)
		})
	}

	ax, ay, ok := nextWater(game.Point{X: fromX, Y: fromY}, 60)

	if !ok {
		return
	}

	for _, t := range []assets.UnitType{assets.Submarne, assets.Corvette, assets.MsslBoat} {
		x, y, found := nextWater(game.Point{X: ax, Y: ay}, 10)

		if !found {
			break
		}

		taken[game.Point{X: x, Y: y}] = true
		u := s.deployUnit(t, sea, player, 2, x, y)
		u.Values.Set(game.AttribRange, missileBenchRange)
		u.Values.Set(game.AttribAttack, missileBenchAttack) // one shot kill -> the death blast always plays
	}

	for i := 0; i < 2; i++ {
		x, y, found := nextWater(game.Point{X: ax + 5, Y: ay}, 14)

		if !found {
			break
		}

		taken[game.Point{X: x, Y: y}] = true
		s.deployUnit(assets.Corvette, sea, enemy, 6, x, y)
	}
}
