// cheats_test.go : tests the developer cheat effects.
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

// cheatUnit deploys a bare team unit (unique id) with fixed storage and ammo caps at
// (x, y).
func cheatUnit(g *Game, team, id int, t assets.UnitType, x, y int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 20)
	u.Values.Set(game.AttribStorage, 40)
	u.Values.Set(game.AttribAmmo, 6)
	u.InitState()
	g.Units.Deploy(u, x, y)
	return u
}

// TestCheatFillStorageAndAmmo: the storage and ammo cheats top the player's own units
// back to their caps and leave another team's units alone.
func TestCheatFillStorageAndAmmo(t *testing.T) {
	g := newGame(t, 16, 16)
	mine := cheatUnit(g, 0, 1, assets.Tank, 5, 5)
	enemy := cheatUnit(g, 1, 2, assets.Tank, 9, 9)

	// Drain both units.
	mine.Storage, mine.Ammo = 0, 0
	enemy.Storage, enemy.Ammo = 0, 0

	g.CheatFillStorage(0)
	g.CheatFillAmmo(0)

	if mine.Storage != 40 {
		t.Errorf("player storage = %d, want the 40 cap", mine.Storage)
	}
	if mine.Ammo != 6 {
		t.Errorf("player ammo = %d, want the 6 cap", mine.Ammo)
	}
	if enemy.Storage != 0 || enemy.Ammo != 0 {
		t.Errorf("enemy unit was touched: storage=%d ammo=%d, want 0/0", enemy.Storage, enemy.Ammo)
	}
}

// TestCheatSuperUnit: the super cheat at least doubles the unit's combat stats on its
// OWN cloned values (a sibling unit sharing the type values is untouched) and refreshes
// its live hit points to the new maximum.
func TestCheatSuperUnit(t *testing.T) {
	g := newGame(t, 16, 16)

	u := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 10)
	u.Values.Set(game.AttribArmor, 4)
	u.Values.Set(game.AttribHits, 20)
	u.Values.Set(game.AttribRange, 5)
	u.Values.Set(game.AttribRounds, 1)
	u.InitState()
	g.Units.Deploy(u, 5, 5)

	// A sibling unit shares the same value set: the clone must isolate the boost.
	sibling := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	sibling.Values = u.Values
	g.Units.Deploy(sibling, 6, 5)
	sharedBefore := sibling.Values

	g.CheatSuperUnit(u)

	if got := u.Values.Get(game.AttribAttack); got < 20 {
		t.Errorf("attack = %d, want >= 2x base (20)", got)
	}
	if got := u.Values.Get(game.AttribHits); got < 40 {
		t.Errorf("hits attribute = %d, want >= 2x base (40)", got)
	}
	if u.Hits != u.Values.Get(game.AttribHits) {
		t.Errorf("live hits = %d, want it refreshed to the new max %d", u.Hits, u.Values.Get(game.AttribHits))
	}
	if sibling.Values != sharedBefore || sibling.Values.Get(game.AttribAttack) != 10 {
		t.Errorf("sibling value set was mutated: attack=%d, want the untouched 10", sibling.Values.Get(game.AttribAttack))
	}
}
