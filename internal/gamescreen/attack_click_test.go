// attack_click_test.go : tests attackorder click handling on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// clickTank is the armed tank of the click tests (hits 80 survive a single 8 damage shot).
var clickTank = tankStats{attack: 8, rng: 4, rounds: 1, ammo: 6, speed: 12, hits: 80}

/* // note: unused code in Go port
// replaced by the shared mkTank(mgr, team, id, gx, gy, clickTank) in fixture_test.go
func clickTestUnit(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribAttack, 8)
	v.Set(game.AttribRange, 4)
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribAmmo, 6)
	v.Set(game.AttribSpeed, 12)
	v.Set(game.AttribHits, 80) // survives a single 8-damage shot
	u.Values = v
	u.InitState()
	mgr.Deploy(u, gx, gy)
	return u
}
*/

// TestLeftClickAttacksInRangeEnemy: a left click (clickCell) on an attackable
// enemy within range fires on it (and keeps the attacker selected); out of range
// it issues a move to attack (the unit pursues, keeping the attacker selected)
// rather than firing or selecting the enemy.
func TestLeftClickAttacksInRangeEnemy(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := mkTank(mgr, 0, 1, 5, 5, clickTank)  // player attacker, active team
	enemy := mkTank(mgr, 1, 2, 7, 5, clickTank) // enemy 2 cells away (in range 4)

	s := &Screen{game: g}
	s.selected = tank
	startHits := enemy.Hits

	s.clickCell(7, 5)
	if enemy.Hits >= startHits {
		t.Errorf("left click on an in range enemy did not fire (hits %d, start %d)", enemy.Hits, startHits)
	}
	if s.selected != tank {
		t.Error("attacking should keep the attacker selected, not select the enemy")
	}

	// An out of range enemy: left click issues a move to attack pursuit - it must not
	// fire now, must keep the attacker selected, and must mark the far enemy as the
	// pursued target.
	far := mkTank(mgr, 1, 3, 20, 20, clickTank)
	farHits := far.Hits
	s.clickCell(20, 20)
	if far.Hits != farHits {
		t.Error("left click on an out of range enemy must not fire immediately")
	}
	if s.selected != tank {
		t.Error("move to attack should keep the attacker selected, not select the enemy")
	}
	if tank.AttackTarget != far {
		t.Error("out of range click should set the attacker pursuing the far enemy")
	}
}

// TestForceAttackClickHitsFriendly: with the SHIFT/CTRL force modifier held, a
// left click on a FRIENDLY unit in range fires on it WITHOUT arming Attack from the
// popup (the vanilla click-the-target flow). Without the modifier, the same click selects the friendly instead of firing.
func TestForceAttackClickHitsFriendly(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	attacker := mkTank(mgr, 0, 1, 5, 5, clickTank) // player attacker, active team
	friend := mkTank(mgr, 0, 2, 7, 5, clickTank)   // SAME team, 2 cells away (in range 4)

	s := &Screen{game: g}
	s.selected = attacker

	// Without the modifier: clicking the friendly selects it, never fires.
	startHits := friend.Hits
	s.orderInput.forceMod = false
	s.clickCell(7, 5)
	if friend.Hits != startHits {
		t.Errorf("a plain click on a friendly must not fire (hits %d, start %d)", friend.Hits, startHits)
	}
	if s.selected != friend {
		t.Error("a plain click on a friendly should SELECT it")
	}

	// With the force modifier: the same click fires on the friendly and keeps the
	// attacker selected (an attack click never selects the target).
	s.selected = attacker
	s.orderInput.forceMod = true
	s.clickCell(7, 5)
	if friend.Hits >= startHits {
		t.Errorf("force-attack did not damage the friendly (hits %d, start %d)", friend.Hits, startHits)
	}
	if s.selected != attacker {
		t.Error("force-attacking should keep the attacker selected, not select the target")
	}
}

// airClickAttacker deploys an airborne (FlagHovering) mobile air unit (Bomber) set
// up to run a fly over when ordered to attack via a player click: attack 12, range 4,
// MOVE_AND_FIRE so firing leaves the move budget for the overflight pass.
func airClickAttacker(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Bomber, team, id, 2) // facing east
	u.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	v := &game.UnitValues{}
	v.Set(game.AttribAttack, 12)
	v.Set(game.AttribRange, 4)
	v.Set(game.AttribAmmo, 4)
	v.Set(game.AttribRounds, 1)
	v.Set(game.AttribSpeed, 8)
	v.Set(game.AttribMoveAndFire, 1)
	v.Set(game.AttribHits, 20)
	u.Values = v
	u.Hits = 20
	u.Ammo = 4
	u.Shots = 1
	u.Speed = 8
	mgr.Deploy(u, gx, gy)
	return u
}

// TestAirUnitClickFiresInPlace: a left click ordering an AIRBORNE air unit to fire on
// an in range enemy fires from its current cell and holds position - M.A.X. air units
// hover to attack, they do not run a strafing pass over the target. It launches its
// ordnance but issues no overflight move and stays put.
func TestAirUnitClickFiresInPlace(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	bomber := airClickAttacker(0, 1, 4, 5, mgr) // player flyer at (4,5), facing east
	mkTank(mgr, 1, 2, 7, 5, clickTank)          // enemy 3 cells east, within range 4

	s := &Screen{game: g}
	s.selected = bomber

	s.clickCell(7, 5)
	// A flyer launches a ROCKET that explodes on arrival, so the shot shows as a
	// projectile in flight rather than an instant hit.
	if n := len(g.ActiveProjectiles()); n != 1 {
		t.Fatalf("the air attack click launched no ordnance: %d projectiles, want 1", n)
	}
	// The air unit fires in place: it must NOT issue an overflight/drift move.
	if g.HasOrder(bomber) {
		t.Fatal("the bomber issued a move after firing; an air unit fires in place, it must not drift")
	}
	// The attacker stays selected, the enemy is never selected by an attack click.
	if s.selected != bomber {
		t.Error("attacking should keep the attacker selected, not select the enemy")
	}
	// Nothing to resolve, but confirm the bomber never left its firing cell.
	g.ResolveMovement()
	if bomber.GridX != 4 || bomber.GridY != 5 {
		t.Errorf("the bomber drifted off its firing cell to (%d,%d); want (4,5)", bomber.GridX, bomber.GridY)
	}
}

// TestGroundUnitClickFiresInPlace: a ground unit ordered to attack an in range enemy
// fires in place - it does NOT issue a move; the same in place shot air units now use.
func TestGroundUnitClickFiresInPlace(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	tank := mkTank(mgr, 0, 1, 5, 5, clickTank)  // player ground attacker
	enemy := mkTank(mgr, 1, 2, 7, 5, clickTank) // enemy in range
	startHits := enemy.Hits

	s := &Screen{game: g}
	s.selected = tank
	s.clickCell(7, 5)

	// A ground unit's gun is an instant hit, no overflight - it never moves.
	if g.HasOrder(tank) {
		t.Error("a ground unit fires in place; it must not issue an overflight move")
	}
	if enemy.Hits >= startHits {
		t.Errorf("the ground attack did not damage the enemy (hits %d, start %d)", enemy.Hits, startHits)
	}
}
