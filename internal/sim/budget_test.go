// budget_test.go : tests move/fire shared budget deduction in sim.
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

// TestAttackSpendsSpeed verifies the shared movement and firing budget through Attack. A two-round tank with eight Speed retains four Speed after one shot.
func TestAttackSpendsSpeed(t *testing.T) {
	g := newGame(t, 8, 8)

	atk := game.NewUnitInfo(assets.Tank, 0, 1, 2)
	atk.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	atk.Values = &game.UnitValues{}
	atk.Values.Set(game.AttribAttack, 18)
	atk.Values.Set(game.AttribHits, 36)
	atk.Values.Set(game.AttribRounds, 2)
	atk.Values.Set(game.AttribSpeed, 8)
	atk.Values.Set(game.AttribRange, 4)
	atk.Values.Set(game.AttribAmmo, 6)
	atk.InitState() // Shots=2, Speed=8, Ammo=6
	g.Units.Deploy(atk, 2, 2)

	tgt := addGunTank(g, 2, 1, 3, 2)
	_ = tgt

	if atk.Speed != 8 || atk.Shots != 2 {
		t.Fatalf("pre-fire Speed=%d Shots=%d, want 8/2", atk.Speed, atk.Shots)
	}
	if _, ok := g.Attack(atk, 3, 2); !ok {
		t.Fatal("attack did not fire")
	}
	if atk.Shots != 1 {
		t.Errorf("Shots after one shot = %d, want 1", atk.Shots)
	}
	if atk.Speed != 4 {
		t.Errorf("Speed after one shot = %d, want 4 (half the budget spent firing)", atk.Speed)
	}
}
