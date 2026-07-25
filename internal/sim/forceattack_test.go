// forceattack_test.go : tests forced attack orders against own or neutral units.
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

// TestForceAttackHitsFriendly: a normal attack refuses a same team target, but a
// FORCE ATTACK (the player's SHIFT/CTRL force fire) fires on a friendly and damages
// it. The normal Attack path still does nothing to a friendly. C++: the CURSOR_ATTACK_SHIFT
// force target path skips the enemy team gate.
func TestForceAttackHitsFriendly(t *testing.T) {
	g := newGame(t, 20, 12)

	attacker := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	attacker.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagTurretSprite
	attacker.Values = &game.UnitValues{}
	attacker.Values.Set(game.AttribAttack, 12)
	attacker.Values.Set(game.AttribAmmo, 4)
	attacker.Values.Set(game.AttribRange, 4)
	attacker.Values.Set(game.AttribHits, 30)
	attacker.Hits = 30
	attacker.Ammo = 4
	attacker.Shots = 1
	g.Units.Deploy(attacker, 5, 5)

	friendly := addTankTarget(g, 2, 6, 5, 0) // same team, one tile east -> in range

	// A normal attack refuses a same team target.
	if g.CanAttack(attacker, 6, 5) {
		t.Error("CanAttack should refuse a same-team target")
	}
	// Force attack allows it.
	if !g.CanForceAttack(attacker, 6, 5) {
		t.Fatal("CanForceAttack should allow firing on a friendly")
	}

	before := friendly.Hits
	if _, ok := g.ForceAttack(attacker, 6, 5); !ok {
		t.Fatal("ForceAttack should fire on the friendly")
	}
	if friendly.Hits >= before {
		t.Errorf("the friendly should have taken damage (hits %d -> %d)", before, friendly.Hits)
	}

	// The normal Attack path still does nothing to a friendly even with a shot ready.
	attacker.Shots = 1
	if _, ok := g.Attack(attacker, 6, 5); ok {
		t.Error("normal Attack should not fire on a friendly")
	}
}
