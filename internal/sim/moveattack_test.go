// moveattack_test.go : tests moveattack.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// addMoveAttacker deploys a mobile gun unit ready to move and fire: full ammo and
// shots, the given speed / weapon range / rounds.
func addMoveAttacker(g *Game, id, team, gx, gy, speed, rng, rounds int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 2) // facing east
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 12)
	u.Values.Set(game.AttribHits, 30)
	u.Values.Set(game.AttribRounds, rounds)
	u.Values.Set(game.AttribSpeed, speed)
	u.Values.Set(game.AttribRange, rng)
	u.Values.Set(game.AttribAmmo, 6)
	u.InitState() // Shots=rounds, Speed=speed, Ammo=6, Hits=30
	g.Units.Deploy(u, gx, gy)
	return u
}

// pursueToFire drives the move to attack order across up to maxTurns turns: each
// turn it resolves movement (advancing the chase / firing on arrival) then refills
// the team's budget so a pursuit across turns continues. Stops once the unit fired
// (AttackTarget cleared).
func pursueToFire(g *Game, atk *game.UnitInfo, maxTurns int) {
	for turn := 0; turn < maxTurns && atk.AttackTarget != nil; turn++ {
		g.ResolveMovement()
		if atk.AttackTarget == nil {
			break
		}
		g.BeginTeamTurn(atk.Team) // refill speed + shots; resumePursuit continues the chase
	}
}

// TestMoveToAttackIssuesPursuit: clicking an out of range enemy sets the attacker
// pursuing it and starts a move, but does not fire on the issuing turn.
func TestMoveToAttackIssuesPursuit(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1) // 18 cells east, well out of range 4

	if !g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY) {
		t.Fatal("move-to-attack rejected for a valid out-of-range enemy")
	}
	if atk.AttackTarget != enemy {
		t.Errorf("attacker should be pursuing the enemy, got %v", atk.AttackTarget)
	}
	if enemy.Hits != 30 {
		t.Errorf("enemy must not be hit on the issuing turn: hits=%d", enemy.Hits)
	}
	if g.ActiveOrders() == 0 {
		t.Error("expected a move order driving the attacker toward the enemy")
	}
}

// TestMoveToAttackDrivesIntoRangeAndFires: the pursuit closes to weapon range and
// fires (across however many turns the shared move/fire budget needs), damaging the
// enemy and ending the order.
func TestMoveToAttackDrivesIntoRangeAndFires(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1)
	start := enemy.Hits

	g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY)
	pursueToFire(g, atk, 10)

	if enemy.Hits >= start {
		t.Errorf("pursuit never fired on the enemy (hits=%d, start=%d)", enemy.Hits, start)
	}
	if atk.AttackTarget != nil {
		t.Error("AttackTarget should be cleared once the unit has fired")
	}
	// The unit ended within weapon range of the target (a standoff firing cell).
	if dx, dy := atk.GridX-enemy.GridX, atk.GridY-enemy.GridY; dx*dx+dy*dy > 4*4 {
		t.Errorf("attacker stopped out of range at (%d,%d), enemy at (%d,%d)", atk.GridX, atk.GridY, enemy.GridX, enemy.GridY)
	}
}

// TestMoveToAttackPursuesMovingEnemy: if the enemy relocates after the order, the
// unit paths again to its new position and still fires.
func TestMoveToAttackPursuesMovingEnemy(t *testing.T) {
	g := newGame(t, 40, 12)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1)
	start := enemy.Hits

	g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY)
	g.ResolveMovement() // close part of the way

	// The enemy moves to a fresh cell; the pursuit must follow it there.
	g.Units.Move(enemy, 20, 9) // buckets again and sets the enemy's new position

	g.BeginTeamTurn(0) // resumePursuit targets the enemy's new position again
	pursueToFire(g, atk, 12)

	if enemy.Hits >= start {
		t.Errorf("pursuit of the relocated enemy never fired (hits=%d, start=%d)", enemy.Hits, start)
	}
	if dx, dy := atk.GridX-enemy.GridX, atk.GridY-enemy.GridY; dx*dx+dy*dy > 4*4 {
		t.Errorf("attacker did not close on the relocated enemy: at (%d,%d), enemy (%d,%d)", atk.GridX, atk.GridY, enemy.GridX, enemy.GridY)
	}
}

// TestMoveToAttackDropsDeadTarget: if the pursued enemy is destroyed, the pursuer
// drops the dangling target (no fire, no panic).
func TestMoveToAttackDropsDeadTarget(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1)

	g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY)
	if atk.AttackTarget != enemy {
		t.Fatal("expected the attacker to be pursuing the enemy")
	}
	g.DestroyUnit(enemy)
	if atk.AttackTarget != nil {
		t.Error("destroying the target must clear every pursuer's AttackTarget")
	}
	// Resuming the (now targetless) unit must not panic or fire.
	g.BeginTeamTurn(0)
	g.ResolveMovement()
}

// TestMoveToAttackRefusedWhenUnarmedOrOutOfAmmo: an out of ammo (or unarmed) unit
// cannot accept a move to attack - it could never fire even after closing.
func TestMoveToAttackRefusedWhenOutOfAmmo(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1)
	atk.Ammo = 0

	if g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY) {
		t.Error("an out-of-ammo unit should not accept a move-to-attack")
	}
	if atk.AttackTarget != nil {
		t.Error("a refused order must leave no pursuit target")
	}
}

// TestMoveToAttackInRangeFiresNow: an enemy already within weapon range fires on the
// spot (no pursuit), matching the in range enemy click.
func TestMoveToAttackInRangeFiresNow(t *testing.T) {
	g := newGame(t, 16, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 4, 2, 1) // 2 cells east, within range 4
	start := enemy.Hits

	if !g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY) {
		t.Fatal("in-range move-to-attack should be accepted (and fire)")
	}
	if enemy.Hits >= start {
		t.Errorf("an in-range target should be hit immediately: hits=%d, start=%d", enemy.Hits, start)
	}
	if atk.AttackTarget != nil {
		t.Error("an immediate in-range shot leaves no pursuit")
	}
}

// TestFreshMoveCancelsPursuit: a plain move order issued after a move to attack
// cancels the pursuit (the unit obeys the new manual destination).
func TestFreshMoveCancelsPursuit(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 1)
	enemy := addTankTarget(g, 2, 20, 2, 1)

	g.OrderMoveToAttack(atk, enemy.GridX, enemy.GridY)
	if atk.AttackTarget == nil {
		t.Fatal("expected an active pursuit before the manual move")
	}
	g.OrderMove(atk, 2, 6)
	if atk.AttackTarget != nil {
		t.Error("a fresh manual move must cancel the move-to-attack pursuit")
	}
}

// TestArrivalShotFiresWhenTheMoverParks pins the fix for the computer opponent firing its
// whole turn's shots at PLANNING time, before a single unit had driven anywhere: on screen
// the shots landed and only then did the units drive to where they had shot from. An
// advancing attacker now queues its shot for the moment it parks, so the attack is the END
// of the drive. Nothing may fire early, and a unit whose target dies on the way in holds fire.
func TestArrivalShotFiresWhenTheMoverParks(t *testing.T) {
	g := newGame(t, 40, 8)
	// Speed 12, rounds 2: a SHORT approach (3 cells) leaves the shared move/fire budget
	// with a shot in hand. Drive further and the clamp eats it, exactly as for the player.
	atk := addMoveAttacker(g, 1, 0, 6, 2, 12, 4, 2)
	enemy := addTankTarget(g, 2, 12, 2, 1)

	// The caller does its own routing (the AI routes around threat), then arms the shot.
	if !g.OrderMove(atk, 9, 2) { // a cell inside weapon range 4 of the enemy
		t.Fatal("the approach move was rejected")
	}

	g.AttackOnArrival(atk, enemy)

	if enemy.Hits != 30 {
		t.Fatalf("the shot went off at planning time, before the drive: enemy hits=%d", enemy.Hits)
	}

	g.ResolveMovement() // drive the approach out

	if atk.GridX != 9 {
		t.Fatalf("the attacker never reached its firing cell: at (%d,%d)", atk.GridX, atk.GridY)
	}

	if enemy.Hits >= 30 {
		t.Errorf("the attacker parked in range and did not take its shot: enemy hits=%d", enemy.Hits)
	}
}

// TestArrivalShotIsDroppedAtTheNextTurn: a queued shot belongs to the turn that planned it.
// If the unit never got there, the team re plans from scratch and the stale shot must not
// go off later.
func TestArrivalShotIsDroppedAtTheNextTurn(t *testing.T) {
	g := newGame(t, 40, 8)
	atk := addMoveAttacker(g, 1, 0, 2, 2, 12, 4, 2)
	enemy := addTankTarget(g, 2, 12, 2, 1)

	g.AttackOnArrival(atk, enemy)
	g.BeginTeamTurn(0) // a new turn: last turn's unfired shot is void

	if len(g.arrivalShots) != 0 {
		t.Errorf("a stale arrival shot survived the turn start: %d queued", len(g.arrivalShots))
	}
}
