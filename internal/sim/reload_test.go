// reload_test.go : tests reload.go.
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

// addSupplyTruck deploys a Supply Truck with the given material stock at (gx, gy).
func addSupplyTruck(g *Game, id, gx, gy, team, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.SplyTrck, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, 50)
	u.Values.Set(game.AttribHits, 16)
	u.Hits = 16
	u.Storage = storage
	g.Units.Deploy(u, gx, gy)
	return u
}

// addDrainedShooter deploys an armed land unit whose ammo store is empty (out of ammo).
func addDrainedShooter(g *Game, id, gx, gy, team int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 12)
	u.Values.Set(game.AttribAmmo, 4)
	u.Values.Set(game.AttribHits, 30)
	u.Hits = 30
	u.Ammo = 0 // drained
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestSupplyTruckReloadsAdjacentFriendly: a truck spends one material to fully refill
// an adjacent same team unit's ammo.
func TestSupplyTruckReloadsAdjacentFriendly(t *testing.T) {
	g := newGame(t, 16, 8)
	truck := addSupplyTruck(g, 1, 4, 4, 0, 50)
	tank := addDrainedShooter(g, 2, 5, 4, 0)

	if !g.CanReload(truck, tank) {
		t.Fatal("CanReload false for an adjacent drained friendly with stock")
	}
	if !g.Reload(truck, tank) {
		t.Fatal("Reload returned false on a valid reload")
	}
	if tank.Ammo != 4 {
		t.Errorf("ammo not refilled to max: Ammo=%d, want 4", tank.Ammo)
	}
	if truck.Storage != 49 {
		t.Errorf("material not spent: Storage=%d, want 49", truck.Storage)
	}
}

// TestReloadRejectsInvalid: CanReload is false when not adjacent, out of stock, target
// full, or the target is an enemy.
func TestReloadRejectsInvalid(t *testing.T) {
	g := newGame(t, 16, 8)

	t.Run("not adjacent", func(t *testing.T) {
		truck := addSupplyTruck(g, 10, 1, 1, 0, 50)
		tank := addDrainedShooter(g, 11, 8, 1, 0)
		if g.CanReload(truck, tank) {
			t.Error("reloaded a non-adjacent target")
		}
	})
	t.Run("no stock", func(t *testing.T) {
		truck := addSupplyTruck(g, 12, 3, 3, 0, 0)
		tank := addDrainedShooter(g, 13, 4, 3, 0)
		if g.CanReload(truck, tank) {
			t.Error("reloaded with empty cargo")
		}
	})
	t.Run("target full", func(t *testing.T) {
		truck := addSupplyTruck(g, 14, 6, 6, 0, 50)
		tank := addDrainedShooter(g, 15, 7, 6, 0)
		tank.Ammo = 4 // already full
		if g.CanReload(truck, tank) {
			t.Error("reloaded an already-full target")
		}
	})
	t.Run("enemy target", func(t *testing.T) {
		truck := addSupplyTruck(g, 16, 9, 6, 0, 50)
		enemy := addDrainedShooter(g, 17, 10, 6, 1)
		if g.CanReload(truck, enemy) {
			t.Error("reloaded an enemy unit")
		}
	})
}

// TestReloadAirUnitMustLand: a flying (hovering) air unit cannot be reloaded; once it
// lands (FlagHovering cleared) the same truck can service it.
func TestReloadAirUnitMustLand(t *testing.T) {
	g := newGame(t, 16, 8)
	truck := addSupplyTruck(g, 1, 4, 4, 0, 50)
	fighter := addDrainedShooter(g, 2, 5, 4, 0)
	fighter.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	if g.CanReload(truck, fighter) {
		t.Error("a flying air unit must NOT be reloadable (it has to land first)")
	}
	fighter.Flags &^= game.FlagHovering // it lands
	if !g.CanReload(truck, fighter) {
		t.Error("a landed air unit should be reloadable")
	}
}
