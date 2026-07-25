// fartarget_test.go : tests far target fire ordering (move to attack beyond current range).
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

// TestCanFarTarget: a mobile armed attacker shows the far target affordance over a
// valid enemy that is OUT of weapon range, but not over an enemy that is in range, and not
// at all when the attacker is out of ammo or stationary (cannot close the distance).
func TestCanFarTarget(t *testing.T) {
	g := newGame(t, 32, 8)

	atk := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	atk.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	atk.Values = &game.UnitValues{}
	atk.Values.Set(game.AttribAttack, 12)
	atk.Values.Set(game.AttribAmmo, 4)
	atk.Values.Set(game.AttribRange, 3)
	atk.Values.Set(game.AttribHits, 30)
	atk.Hits = 30
	atk.Ammo = 4
	atk.Shots = 1
	g.Units.Deploy(atk, 2, 2)

	far := addTankTarget(g, 2, 20, 2, 1) // enemy, well beyond range 3
	near := addTankTarget(g, 3, 4, 2, 1) // enemy, 2 tiles -> within range 3

	if !g.CanFarTarget(atk, far.GridX, far.GridY) {
		t.Error("expected a far-target over an out-of-range valid enemy")
	}
	if g.CanAttack(atk, far.GridX, far.GridY) {
		t.Error("a far target must not also be in attack range")
	}
	if g.CanFarTarget(atk, near.GridX, near.GridY) {
		t.Error("an in-range enemy is the normal attack cursor, not a far target")
	}

	// Out of ammo: no far target (closing in would not let it fire).
	atk.Ammo = 0
	if g.CanFarTarget(atk, far.GridX, far.GridY) {
		t.Error("an out-of-ammo attacker has no far target")
	}
	atk.Ammo = 4

	// A stationary weapon cannot close on the target -> not viable, not a far target.
	atk.Flags = game.FlagStationary | game.FlagSelectable
	if g.CanFarTarget(atk, far.GridX, far.GridY) {
		t.Error("a stationary attacker has no far-target cursor")
	}
}
