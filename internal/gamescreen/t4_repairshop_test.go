// t4_repairshop_test.go : testing unit for repairshop.go service rules
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// t4TankRow is the base tank stats the shop compares stored units against.
var t4TankRow = assets.UnitAttributes{TurnsToBuild: 8, HitPoints: 40, Attack: 10, Rounds: 1, Speed: 8, Ammo: 10, Range: 4}

// t4Shop is a depot joined to a mining station, so the base pool feeds the shop.
type t4Shop struct {
	s       *Screen
	g       *sim.Game
	depot   *game.UnitInfo
	complex *economy.Complex
	ops     *screenRepairOps
}

// t4NewShop builds a depot beside a mining station holding raw material in the pool.
func t4NewShop(t *testing.T, raw int) *t4Shop {
	t.Helper()

	s, g := newClickScreen(t)
	t4ArtlessScreen(s)

	attribs := make([]assets.UnitAttributes, assets.UnitCount)
	attribs[assets.Tank] = t4TankRow
	s.SetAttribs(attribs)

	depot := &game.UnitInfo{
		Type: assets.Depot, Team: 0, ID: 40,
		Flags:  game.FlagBuilding | game.FlagStationary | game.FlagSelectable,
		Values: &game.UnitValues{},
	}
	g.PlaceBuilding(depot, 5, 5)

	_, c := t4MiningStation(t, g, 7, 5, raw)

	if g.ComplexOf(depot) != c {
		t.Fatal("the depot and the mining station should share one complex")
	}

	sh := &t4Shop{s: s, g: g, depot: depot, complex: c}
	sh.ops = &screenRepairOps{s: s, building: depot, team: 0, rawCapacity: 50}

	return sh
}

// store puts a tank into the depot bay with the given hits and ammo (full strength stats).
func (sh *t4Shop) store(id, hits, ammo int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, 0, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = game.NewUnitValues(t4TankRow)
	u.Hits = hits
	u.Ammo = ammo
	u.Parent = sh.depot
	sh.depot.Carried = append(sh.depot.Carried, u)
	sh.ops.units = sh.depot.Carried

	return u
}

// TestT4ComplexRaw pins that the shop reads the pooled raw, not the member gauges.
func TestT4ComplexRaw(t *testing.T) {
	t.Run("no complex", func(t *testing.T) {
		s, g := newClickScreen(t)
		lone := mkUnit(g, assets.Tank, 1, 5, 5, game.FlagMobileLandUnit)

		if raw, ok := s.complexRaw(lone); raw != 0 || ok {
			t.Errorf("complexRaw = (%d,%v), want (0,false)", raw, ok)
		}
	})

	t.Run("pool wins over gauges", func(t *testing.T) {
		sh := t4NewShop(t, 40)
		sh.complex.Material = 20 // the member gauge still reads 40

		if raw, ok := sh.s.complexRaw(sh.depot); raw != 20 || !ok {
			t.Errorf("complexRaw = (%d,%v), want the pool (20,true)", raw, ok)
		}
	})
}

// TestT4Snapshot pins the shop header and the per unit flags.
func TestT4Snapshot(t *testing.T) {
	sh := t4NewShop(t, 30)
	sh.store(1, 20, 4)
	info := sh.ops.Snapshot()

	if !info.IsShop || info.IsHangar {
		t.Errorf("IsShop=%v IsHangar=%v, want a depot shop", info.IsShop, info.IsHangar)
	}

	if !info.ShowMaterials || !info.ShowActivateAll {
		t.Errorf("ShowMaterials=%v ShowActivateAll=%v, want both", info.ShowMaterials, info.ShowActivateAll)
	}

	if info.RawMaterials != 30 || info.RawCapacity != 50 {
		t.Errorf("raw = %d/%d, want 30/50", info.RawMaterials, info.RawCapacity)
	}

	if len(info.Units) != 1 {
		t.Fatalf("units = %d, want 1", len(info.Units))
	}

	u := info.Units[0]

	if u.Hits != 20 || u.MaxHits != 40 || u.Ammo != 4 || u.MaxAmmo != 10 {
		t.Errorf("gauges hits %d/%d ammo %d/%d, want 20/40 and 4/10", u.Hits, u.MaxHits, u.Ammo, u.MaxAmmo)
	}

	if !u.CanReload || !u.CanRepair || !u.CanActivate {
		t.Errorf("CanReload=%v CanRepair=%v CanActivate=%v, want all true", u.CanReload, u.CanRepair, u.CanActivate)
	}

	if u.CanUpgrade {
		t.Error("a tank at current stats has nothing to upgrade")
	}
}

// TestT4SnapshotAirTransportHasNoActivateAll pins the header of a non shop carrier.
func TestT4SnapshotAirTransportHasNoActivateAll(t *testing.T) {
	s, g := newClickScreen(t)
	air := mkUnit(g, assets.AirTrans, 1, 5, 5, game.FlagMobileAirUnit|game.FlagSelectable)
	ops := &screenRepairOps{s: s, building: air, team: 0}
	info := ops.Snapshot()

	if info.IsShop || info.ShowActivateAll || info.ShowMaterials {
		t.Errorf("air transport header = %+v, want no shop, no materials, no Activate All", info)
	}
}

// TestT4UnitInfoFlags pins when each service button is offered.
func TestT4UnitInfoFlags(t *testing.T) {
	cases := []struct {
		name                                string
		raw, hits, ammo                     int
		mutate                              func(u *game.UnitInfo)
		wantReload, wantRepair, wantUpgrade bool
		wantActivate                        bool
	}{
		{"damaged and empty with raw", 30, 20, 0, nil, true, true, false, true},
		{"no raw blocks reload and repair", 0, 20, 0, nil, false, false, false, true},
		{"full strength offers neither", 30, 40, 10, nil, false, false, false, true},
		{"old stats can upgrade", 30, 40, 10, func(u *game.UnitInfo) { u.Values.Set(game.AttribHits, 30) }, false, false, true, true},
		{"too little raw to upgrade", 5, 40, 10, func(u *game.UnitInfo) { u.Values.Set(game.AttribHits, 30) }, false, false, false, true},
		{"regenerating cannot upgrade", 30, 40, 10, func(u *game.UnitInfo) {
			u.Values.Set(game.AttribHits, 30)
			u.Flags |= game.FlagRegenerating
		}, false, false, false, true},
		{"air unit without speed cannot activate", 30, 40, 10, func(u *game.UnitInfo) {
			u.Flags = game.FlagMobileAirUnit | game.FlagSelectable
			u.Speed = 0
		}, false, false, false, false},
		{"air unit with speed can activate", 30, 40, 10, func(u *game.UnitInfo) {
			u.Flags = game.FlagMobileAirUnit | game.FlagSelectable
			u.Speed = 4
		}, false, false, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := t4NewShop(t, c.raw)
			u := sh.store(1, c.hits, c.ammo)

			if c.mutate != nil {
				c.mutate(u)
			}

			got := sh.ops.unitInfo(u, true, c.raw)

			if got.CanReload != c.wantReload || got.CanRepair != c.wantRepair ||
				got.CanUpgrade != c.wantUpgrade || got.CanActivate != c.wantActivate {
				t.Errorf("reload/repair/upgrade/activate = %v/%v/%v/%v, want %v/%v/%v/%v",
					got.CanReload, got.CanRepair, got.CanUpgrade, got.CanActivate,
					c.wantReload, c.wantRepair, c.wantUpgrade, c.wantActivate)
			}
		})
	}

	t.Run("a non shop carrier offers no service", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		u := sh.store(1, 20, 0)
		got := sh.ops.unitInfo(u, false, 30)

		if got.CanReload || got.CanRepair || got.CanUpgrade {
			t.Error("only a shop reloads, repairs and upgrades")
		}
	})
}

// TestT4UpgradeCostAndViable pins the quarter of build cost price and the nothing changes rule.
func TestT4UpgradeCostAndViable(t *testing.T) {
	sh := t4NewShop(t, 30)
	old := sh.store(1, 30, 10)
	old.Values.Set(game.AttribHits, 30)

	if got := sh.ops.upgradeCost(old); got != 6 {
		t.Errorf("upgradeCost = %d, want 6 (3 raw per turn * 8 turns / 4)", got)
	}

	if !sh.ops.upgradeViable(old, 6) {
		t.Error("old stats with exactly the cost in raw should be viable")
	}

	if sh.ops.upgradeViable(old, 5) {
		t.Error("one raw short of the cost must not be viable")
	}

	current := sh.store(2, 40, 10)

	if sh.ops.upgradeViable(current, 30) {
		t.Error("a unit at current stats has nothing to gain")
	}
}

// TestT4Reload pins the ammo refill and its raw cost.
func TestT4Reload(t *testing.T) {
	cases := []struct {
		name         string
		raw, ammo    int
		wantAmmo     int
		wantRawAfter int
	}{
		{"full refill", 30, 2, 10, 22},
		{"limited by raw", 3, 2, 5, 0},
		{"no raw refuses", 0, 2, 2, 0},
		{"already full", 30, 10, 10, 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := t4NewShop(t, c.raw)
			u := sh.store(1, 40, c.ammo)
			sh.ops.Reload(0)

			if u.Ammo != c.wantAmmo {
				t.Errorf("ammo = %d, want %d", u.Ammo, c.wantAmmo)
			}

			if sh.complex.Material != c.wantRawAfter {
				t.Errorf("pool raw = %d, want %d", sh.complex.Material, c.wantRawAfter)
			}
		})
	}

	t.Run("bad index is ignored", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		u := sh.store(1, 40, 2)
		sh.ops.Reload(-1)
		sh.ops.Reload(5)

		if u.Ammo != 2 || sh.complex.Material != 30 {
			t.Error("an index outside the bay must change nothing")
		}
	})
}

// TestT4Repair pins the hit point repair and its raw cost of one per four hits.
func TestT4Repair(t *testing.T) {
	cases := []struct {
		name         string
		raw, hits    int
		wantHits     int
		wantRawAfter int
	}{
		{"full repair", 30, 20, 40, 25},
		{"limited by raw", 2, 20, 28, 0},
		{"no raw refuses", 0, 20, 20, 0},
		{"already healthy", 30, 40, 40, 30},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := t4NewShop(t, c.raw)
			u := sh.store(1, c.hits, 10)
			sh.ops.Repair(0)

			if u.Hits != c.wantHits {
				t.Errorf("hits = %d, want %d", u.Hits, c.wantHits)
			}

			if sh.complex.Material != c.wantRawAfter {
				t.Errorf("pool raw = %d, want %d", sh.complex.Material, c.wantRawAfter)
			}
		})
	}

	t.Run("bad index is ignored", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		u := sh.store(1, 20, 10)
		sh.ops.Repair(-1)
		sh.ops.Repair(5)

		if u.Hits != 20 || sh.complex.Material != 30 {
			t.Error("an index outside the bay must change nothing")
		}
	})
}

// TestT4Upgrade pins the upgrade to the team baseline, its price and its refusals.
func TestT4Upgrade(t *testing.T) {
	t.Run("upgrades and pays a quarter of the build cost", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		u := sh.store(1, 30, 10)
		u.Values.Set(game.AttribHits, 30)
		sh.ops.Upgrade(0)

		if got := u.Values.Get(game.AttribHits); got != 40 {
			t.Errorf("max hits = %d, want the team baseline 40", got)
		}

		if u.Hits != 40 {
			t.Errorf("hits = %d, want a full unit to stay full (40)", u.Hits)
		}

		if sh.complex.Material != 24 {
			t.Errorf("pool raw = %d, want 24 after the 6 raw fee", sh.complex.Material)
		}
	})

	t.Run("too little raw refuses", func(t *testing.T) {
		sh := t4NewShop(t, 5)
		u := sh.store(1, 30, 10)
		u.Values.Set(game.AttribHits, 30)
		sh.ops.Upgrade(0)

		if u.Values.Get(game.AttribHits) != 30 || sh.complex.Material != 5 {
			t.Error("an unaffordable upgrade must change nothing")
		}
	})

	t.Run("nothing to gain refuses", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		sh.store(1, 40, 10)
		sh.ops.Upgrade(0)

		if sh.complex.Material != 30 {
			t.Errorf("pool raw = %d, a useless upgrade must cost nothing", sh.complex.Material)
		}
	})

	t.Run("regenerating unit refuses", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		u := sh.store(1, 30, 10)
		u.Values.Set(game.AttribHits, 30)
		u.Flags |= game.FlagRegenerating
		sh.ops.Upgrade(0)

		if u.Values.Get(game.AttribHits) != 30 || sh.complex.Material != 30 {
			t.Error("a regenerating unit is not upgraded")
		}
	})

	t.Run("bad index is ignored", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		sh.store(1, 30, 10)
		sh.ops.Upgrade(-1)
		sh.ops.Upgrade(9)

		if sh.complex.Material != 30 {
			t.Error("an index outside the bay must change nothing")
		}
	})
}

// TestT4AllVariants pins that the All buttons act on every eligible unit in bay order until raw runs out.
func TestT4AllVariants(t *testing.T) {
	t.Run("ReloadAll serves every unit", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		a := sh.store(1, 40, 0)
		b := sh.store(2, 40, 5)
		sh.ops.ReloadAll()

		if a.Ammo != 10 || b.Ammo != 10 || sh.complex.Material != 15 {
			t.Errorf("ammo %d and %d, pool %d; want 10, 10, 15", a.Ammo, b.Ammo, sh.complex.Material)
		}
	})

	t.Run("ReloadAll stops when raw runs out", func(t *testing.T) {
		sh := t4NewShop(t, 12)
		a := sh.store(1, 40, 0)
		b := sh.store(2, 40, 0)
		sh.ops.ReloadAll()

		if a.Ammo != 10 || b.Ammo != 2 || sh.complex.Material != 0 {
			t.Errorf("ammo %d and %d, pool %d; want 10, 2, 0", a.Ammo, b.Ammo, sh.complex.Material)
		}
	})

	t.Run("RepairAll serves every unit", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		a := sh.store(1, 20, 10)
		b := sh.store(2, 36, 10)
		sh.ops.RepairAll()

		if a.Hits != 40 || b.Hits != 40 || sh.complex.Material != 24 {
			t.Errorf("hits %d and %d, pool %d; want 40, 40, 24", a.Hits, b.Hits, sh.complex.Material)
		}
	})

	t.Run("UpgradeAll skips units with nothing to gain", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		old := sh.store(1, 30, 10)
		old.Values.Set(game.AttribHits, 30)
		current := sh.store(2, 40, 10)
		sh.ops.UpgradeAll()

		if old.Values.Get(game.AttribHits) != 40 || current.Values.Get(game.AttribHits) != 40 {
			t.Error("the old unit should reach 40 and the current one stay at 40")
		}

		if sh.complex.Material != 24 {
			t.Errorf("pool raw = %d, want 24 (one upgrade fee)", sh.complex.Material)
		}
	})

	t.Run("ActivateAll ejects only the first unit", func(t *testing.T) {
		const w, h = 12, 12
		mgr := units.NewManager()
		grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
		p := &game.StartParams{}
		p.Teams[0].Type = game.TeamPlayer
		g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

		apc := game.NewUnitInfo(assets.ClnTrans, 0, 1, 0)
		apc.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		apc.Values = &game.UnitValues{}
		apc.Values.Set(game.AttribStorage, 2)
		apc.InitState()
		mgr.Deploy(apc, 5, 5)

		var squads []*game.UnitInfo

		for id := 2; id <= 3; id++ {
			sq := game.NewUnitInfo(assets.Infantry, 0, id, 0)
			sq.Flags = game.FlagMobileLandUnit | game.FlagSelectable
			sq.Values = &game.UnitValues{}
			sq.InitState()
			sq.Parent = apc
			apc.Carried = append(apc.Carried, sq)
			squads = append(squads, sq)
		}

		s := &Screen{game: g}
		ops := &screenRepairOps{s: s, building: apc, team: 0, units: apc.Carried}
		ops.ActivateAll()

		if s.orderInput.unloadUnit != squads[0] {
			t.Errorf("armed unit = %v, want the first stored unit", s.orderInput.unloadUnit)
		}

		if len(apc.Carried) != 2 {
			t.Error("the pick only arms: both units stay aboard until a square is chosen")
		}
	})

	t.Run("ActivateAll on an empty bay does nothing", func(t *testing.T) {
		sh := t4NewShop(t, 30)
		sh.ops.ActivateAll()

		if sh.s.transportArmed() {
			t.Error("an empty bay arms nothing")
		}
	})
}
