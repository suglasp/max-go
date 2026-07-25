// repair_test.go : tests the mobile Repair unit's in world hit point repair.
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

// addRepairUnit deploys a mobile Repair unit carrying material.
func addRepairUnit(g *Game, id, gx, gy, team, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Repair, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, 50)
	u.Values.Set(game.AttribHits, 16)
	u.Hits = 16
	u.Storage = storage
	g.Units.Deploy(u, gx, gy)
	return u
}

// addDamagedTank deploys a same team land unit at a given (damaged) hit point level.
func addDamagedTank(g *Game, id, gx, gy, team, hits int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 30)
	u.Hits = hits
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestRepairUnitMendsAdjacentFriendly: a Repair unit spends material (one per four
// hits) to mend an adjacent same team damaged unit, bounded by its stored material.
func TestRepairUnitMendsAdjacentFriendly(t *testing.T) {
	g := newGame(t, 16, 8)
	rep := addRepairUnit(g, 1, 4, 4, 0, 50)
	tank := addDamagedTank(g, 2, 5, 4, 0, 10) // 10/30, missing 20 hits

	if !g.CanRepair(rep, tank) {
		t.Fatal("CanRepair false for an adjacent damaged friendly with material")
	}
	if !g.Repair(rep, tank) {
		t.Fatal("Repair returned false on a valid repair")
	}
	if tank.Hits != 30 {
		t.Errorf("hits not mended to max: Hits=%d, want 30", tank.Hits)
	}
	if rep.Storage != 45 { // 20 hits / 4 = 5 material
		t.Errorf("material not spent correctly: Storage=%d, want 45", rep.Storage)
	}
	// Fully mended now: nothing more to repair.
	if g.CanRepair(rep, tank) {
		t.Error("CanRepair true for an already-full target")
	}
}

// TestRepairBoundedByMaterial: a repairer with little material heals only what its
// cargo covers (four hits per material), not to full.
func TestRepairBoundedByMaterial(t *testing.T) {
	g := newGame(t, 16, 8)
	rep := addRepairUnit(g, 1, 4, 4, 0, 2) // 2 material -> at most 8 hits
	tank := addDamagedTank(g, 2, 5, 4, 0, 10)

	if !g.Repair(rep, tank) {
		t.Fatal("Repair returned false with material available")
	}
	if tank.Hits != 18 { // 10 + 2*4
		t.Errorf("hits = %d, want 18 (bounded by 2 material)", tank.Hits)
	}
	if rep.Storage != 0 {
		t.Errorf("material left = %d, want 0", rep.Storage)
	}
}

// TestRepairRejectsInvalid: CanRepair is false when not adjacent, out of material,
// the target is full, or the target is an enemy.
func TestRepairRejectsInvalid(t *testing.T) {
	g := newGame(t, 16, 8)

	t.Run("not adjacent", func(t *testing.T) {
		rep := addRepairUnit(g, 10, 1, 1, 0, 50)
		tank := addDamagedTank(g, 11, 8, 1, 0, 10)
		if g.CanRepair(rep, tank) {
			t.Error("repaired a non-adjacent target")
		}
	})
	t.Run("no material", func(t *testing.T) {
		rep := addRepairUnit(g, 12, 3, 3, 0, 0)
		tank := addDamagedTank(g, 13, 4, 3, 0, 10)
		if g.CanRepair(rep, tank) {
			t.Error("repaired with empty cargo")
		}
	})
	t.Run("target full", func(t *testing.T) {
		rep := addRepairUnit(g, 14, 6, 6, 0, 50)
		tank := addDamagedTank(g, 15, 7, 6, 0, 30) // already full
		if g.CanRepair(rep, tank) {
			t.Error("repaired an already-full target")
		}
	})
	t.Run("enemy target", func(t *testing.T) {
		rep := addRepairUnit(g, 16, 9, 6, 0, 50)
		enemy := addDamagedTank(g, 17, 10, 6, 1, 10)
		if g.CanRepair(rep, enemy) {
			t.Error("repaired an enemy unit")
		}
	})
}

// TestRepairAtResolvesCell: RepairAt mends the repairable friendly standing at a cell.
func TestRepairAtResolvesCell(t *testing.T) {
	g := newGame(t, 16, 8)
	rep := addRepairUnit(g, 1, 4, 4, 0, 50)
	addDamagedTank(g, 2, 5, 4, 0, 10)

	if !g.CanRepairAt(rep, 5, 4) {
		t.Fatal("CanRepairAt false over a damaged adjacent friendly")
	}
	if !g.RepairAt(rep, 5, 4) {
		t.Error("RepairAt returned false over a valid target")
	}
}
