// economy_test.go : tests cargo rates, complexes, and resource maps.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// building makes a stationary unit of a type/team at a cell with the given
// per turn order and an optional storage capacity.
func building(t assets.UnitType, team, gx, gy int, order game.Order, storage int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, 0, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary
	u.GridX, u.GridY = gx, gy
	u.Order = order
	u.State = game.OrderStateExecutingOrder // an established building, not mid init
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, storage)
	return u
}

func TestConsumptionRates(t *testing.T) {
	// Raw consumption scales with the build speed multiplier (2->4, 4->12).
	cases := []struct {
		typ     assets.UnitType
		mult    int
		wantRaw int
		fuel    int
		power   int
		life    int
		gold    int
	}{
		{assets.Constrct, 2, 8, 0, 0, 0, 0},  // 4*2
		{assets.LandPlt, 4, 36, 0, 1, 0, 0},  // 12*3, power draw 1
		{assets.LandPlt, 1, 3, 0, 1, 0, 0},   // 1*3
		{assets.TrainHal, 1, 1, 0, 1, 1, 0},  // raw 1, power 1, life 1
		{assets.PowerStn, 1, 0, 6, -6, 0, 0}, // burns 6 fuel, makes 6 power
		{assets.PowGen, 1, 0, 2, -1, 0, 0},   // burns 2 fuel, makes 1 power
		{assets.CommTwr, 1, 0, 0, 1, 0, 5},   // power 1, gold upkeep 5
		{assets.Habitat, 1, 0, 0, 0, -3, 0},  // houses 3 workers
		{assets.MiningSt, 1, 0, 0, 1, 0, 0},  // power draw 1
	}
	for _, c := range cases {
		if got := RawConsumptionRate(c.typ, c.mult); got != c.wantRaw {
			t.Errorf("%v raw@%d = %d, want %d", c.typ, c.mult, got, c.wantRaw)
		}
		if got := FuelConsumptionRate(c.typ); got != c.fuel {
			t.Errorf("%v fuel = %d, want %d", c.typ, got, c.fuel)
		}
		if got := PowerConsumptionRate(c.typ); got != c.power {
			t.Errorf("%v power = %d, want %d", c.typ, got, c.power)
		}
		if got := LifeConsumptionRate(c.typ); got != c.life {
			t.Errorf("%v life = %d, want %d", c.typ, got, c.life)
		}
		if got := GoldConsumptionRate(c.typ); got != c.gold {
			t.Errorf("%v gold = %d, want %d", c.typ, got, c.gold)
		}
	}
}

func TestCargoTypeOf(t *testing.T) {
	if CargoTypeOf(assets.ADump) != CargoRaw || CargoTypeOf(assets.MiningSt) != CargoRaw {
		t.Error("raw storage units misclassified")
	}
	if CargoTypeOf(assets.FDump) != CargoFuel || CargoTypeOf(assets.FuelTrck) != CargoFuel {
		t.Error("fuel storage units misclassified")
	}
	if CargoTypeOf(assets.GoldSm) != CargoGold || CargoTypeOf(assets.GoldTrck) != CargoGold {
		t.Error("gold storage units misclassified")
	}
	// Mine layers carry mines made of raw material, so they transfer raw (reloadable).
	if CargoTypeOf(assets.MineLayr) != CargoRaw || CargoTypeOf(assets.SeaMnLyr) != CargoRaw {
		t.Error("mine layers should carry raw cargo (their mine store)")
	}
	if CargoTypeOf(assets.Tank) != CargoNone {
		t.Error("a tank should hold no cargo")
	}
	// A power building CONSUMES fuel from the complex pool; only the dedicated
	// Fuel Truck moves fuel to or from a Fuel Tank.
	if CargoTypeOf(assets.PowerStn) != CargoNone || CargoTypeOf(assets.PowGen) != CargoNone {
		t.Error("power buildings consume fuel but do not X-Fer it")
	}
}

func TestCargoArithmetic(t *testing.T) {
	a := Cargo{Raw: 10, Fuel: 5, Gold: 2}
	a.Add(Cargo{Raw: 3, Power: 4})
	if a.Raw != 13 || a.Power != 4 {
		t.Errorf("Add wrong: %+v", a)
	}
	a.Fuel -= 5
	if a.Get(CargoRaw) != 13 || a.Get(CargoGold) != 2 || a.Get(CargoFuel) != 0 {
		t.Error("Get returned wrong bucket")
	}
}

func TestResourceMapDepositAndSurvey(t *testing.T) {
	m := NewResourceMap(8, 8)
	m.SetDeposit(2, 2, CargoRaw, 20) // amount stored as 20, read capped at 16
	if typ, amt := m.DepositAt(2, 2); typ != CargoRaw || amt != 16 {
		t.Errorf("deposit = (%v,%d), want (Raw,16) read-capped", typ, amt)
	}
	m.SetDeposit(3, 2, CargoGold, 8)

	// Unsurveyed: a team gated query sees nothing; an all=true query sees it.
	raw, gold, _ := m.ResourcesInArea(2, 2, 1, 255, 0, false)
	if raw != 0 || gold != 0 {
		t.Errorf("unsurveyed gated query = raw %d gold %d, want 0/0", raw, gold)
	}
	raw, gold, _ = m.ResourcesInArea(2, 2, 1, 255, 0, true)
	if raw != 16 || gold != 8 {
		t.Errorf("all query = raw %d gold %d, want 16/8", raw, gold)
	}

	// Survey reveals it to team 0 only.
	m.Survey(2, 2, 1, 0)
	if !m.IsSurveyed(2, 2, 0) || m.IsSurveyed(2, 2, 1) {
		t.Error("survey should mark team 0 only")
	}
	raw, gold, _ = m.ResourcesInArea(2, 2, 1, 255, 0, false)
	if raw != 16 || gold != 8 {
		t.Errorf("surveyed gated query = raw %d gold %d, want 16/8", raw, gold)
	}
}

func TestResourceLimitCap(t *testing.T) {
	m := NewResourceMap(8, 8)
	m.SetDeposit(0, 0, CargoRaw, 16)
	m.SetDeposit(1, 0, CargoRaw, 16)
	m.SetDeposit(0, 1, CargoRaw, 16) // three cells of 16 = 48 raw in the area
	raw, _, _ := m.ResourcesInArea(0, 0, 1, 16, 0, true)
	if raw != 16 {
		t.Errorf("raw = %d, want 16 (capped at limit)", raw)
	}
}

func TestSetInitialMining(t *testing.T) {
	m := NewResourceMap(16, 16)
	// Footprint of a station at (5,5) is the 2x2 block (5,5)..(6,6).
	m.SetDeposit(5, 5, CargoRaw, 10)
	m.SetDeposit(6, 5, CargoRaw, 10) // 20 raw total
	m.SetDeposit(5, 6, CargoFuel, 6)
	m.SetDeposit(6, 6, CargoGold, 8)

	st := building(assets.MiningSt, 0, 5, 5, game.OrderPowerOn, 0)
	m.SetInitialMining(st)

	// Fuel up to a power generator's burn (2), then raw fills to the 16 cap,
	// leaving no room for more fuel or gold.
	if st.FuelMining != 2 || st.RawMining != 14 || st.GoldMining != 0 {
		t.Errorf("mining = raw %d fuel %d gold %d, want 14/2/0", st.RawMining, st.FuelMining, st.GoldMining)
	}
	if st.TotalMining != 16 {
		t.Errorf("total mining = %d, want 16 (capped)", st.TotalMining)
	}
	if st.RawMiningMax != 20 || st.FuelMiningMax != 6 || st.GoldMiningMax != 8 {
		t.Errorf("mining max = %d/%d/%d, want 20/6/8", st.RawMiningMax, st.FuelMiningMax, st.GoldMiningMax)
	}
}

// TestCanPower checks the power part of the live power on gate (StartShortage).
func TestCanPower(t *testing.T) {
	c := NewComplex(1)
	factory := &game.UnitInfo{Type: assets.LandPlt} // draws power
	c.Grow(factory)

	if c.StartShortage(factory) != ShortagePower {
		t.Error("isolated factory with no generator should NOT power")
	}
	gen := &game.UnitInfo{Type: assets.PowGen}
	c.Grow(gen)
	c.Fuel = 10
	if c.StartShortage(factory) == ShortagePower {
		t.Error("factory with a fuelled generator should power")
	}
	c.Fuel = 0
	if c.StartShortage(factory) != ShortagePower {
		t.Error("a generator with no fuel cannot power the factory")
	}
	if c.StartShortage(gen) == ShortagePower {
		t.Error("a generator always powers itself")
	}
	if c.StartShortage(&game.UnitInfo{Type: assets.ADump}) == ShortagePower {
		t.Error("a building that needs no power always powers")
	}
	// A base that mines its own fuel sustains itself: it powers a consumer even
	// with an empty pool (c.Fuel is still 0 here) - the generator runs regardless of
	// fuel STOCK and the per turn balance refills the pool from mining.
	c.Grow(&game.UnitInfo{Type: assets.MiningSt, FuelMining: 4})
	if c.StartShortage(factory) == ShortagePower {
		t.Error("a fuel-mining base should power a consumer even at 0 fuel stock")
	}
}

func TestNetProductionMiningStation(t *testing.T) {
	st := building(assets.MiningSt, 0, 5, 5, game.OrderPowerOn, 0)
	st.RawMining, st.FuelMining, st.GoldMining = 14, 2, 0
	st.TotalMining = 16

	net := NetProduction(st, 4) // human difficulty
	// raw 14*4/4=14, fuel 2*4/4=2, power -1 (upkeep), free 16-16=0.
	if net.Raw != 14 || net.Fuel != 2 || net.Power != -1 || net.FreeCapacity != 0 {
		t.Errorf("net@diff4 = %+v, want raw14 fuel2 power-1", net)
	}
	// God difficulty (6) scales mining up.
	god := NetProduction(st, 6)
	if god.Raw != 21 || god.Fuel != 3 {
		t.Errorf("net@diff6 = raw %d fuel %d, want 21/3", god.Raw, god.Fuel)
	}
	// Powered off, a station produces nothing.
	st.Order = game.OrderPowerOff
	if off := NetProduction(st, 4); off != (Cargo{}) {
		t.Errorf("powered-off net = %+v, want zero", off)
	}
}

func TestNetProductionPowerBuildings(t *testing.T) {
	ps := building(assets.PowerStn, 0, 0, 0, game.OrderPowerOn, 0)
	if net := NetProduction(ps, 4); net.Fuel != -6 || net.Power != 6 {
		t.Errorf("power station net = %+v, want fuel-6 power6", net)
	}
	hab := building(assets.Habitat, 0, 0, 0, game.OrderPowerOn, 0)
	if net := NetProduction(hab, 4); net.Life != 3 {
		t.Errorf("habitat net = %+v, want life 3", net)
	}
}

func TestComplexBalanceTurn(t *testing.T) {
	c := NewComplex(1)
	st := building(assets.MiningSt, 0, 5, 5, game.OrderPowerOn, 25) // a mine holds 25 raw
	st.RawMining, st.FuelMining, st.TotalMining = 14, 2, 16
	ps := building(assets.PowerStn, 0, 1, 1, game.OrderPowerOn, 0)
	tower := building(assets.CommTwr, 0, 2, 2, game.OrderPowerOn, 0)
	c.Grow(st)
	c.Grow(ps)
	c.Grow(tower)
	// The base needs somewhere to KEEP what it mines and burns: a Storage Unit (50 raw)
	// and two Fuel Tanks (50 each). Without them the surplus spills - see CapStores.
	dump := building(assets.ADump, 0, 6, 5, game.OrderPowerOn, 50)
	tank1 := building(assets.FDump, 0, 7, 5, game.OrderPowerOn, 50)
	tank2 := building(assets.FDump, 0, 8, 5, game.OrderPowerOn, 50)
	c.Grow(dump)
	c.Grow(tank1)
	c.Grow(tank2)
	c.Fuel = 100 // fuel buffer so the station can run (it needs fuel to burn)
	if c.Buildings != 6 {
		t.Fatalf("buildings = %d, want 6", c.Buildings)
	}

	net := c.BalanceTurn(4)
	// raw 14; fuel 2-6=-4; power -1+6-1=4; gold -5; life 0. The net RATE still shows
	// the gold deficit (-5), but the STORE clamps at zero (no negative resources).
	if net.Raw != 14 || net.Fuel != -4 || net.Power != 4 || net.Gold != -5 {
		t.Errorf("net = %+v, want raw14 fuel-4 power4 gold-5", net)
	}
	if c.Material != 14 || c.Fuel != 96 || c.Gold != 0 {
		t.Errorf("stores = mat%d fuel%d gold%d, want 14/96/0 (gold clamps at zero)", c.Material, c.Fuel, c.Gold)
	}
	// Stores accumulate over turns; power stays a live balance.
	c.BalanceTurn(4)
	if c.Material != 28 {
		t.Errorf("material after 2 turns = %d, want 28 (accumulated)", c.Material)
	}
	if p := c.activeGeneratorOutput() - c.consumerPowerDemand(); p != 4 {
		t.Errorf("spare power = %d, want 4 (recomputed, not accumulated)", p)
	}

	// Shrinking the last member empties the complex.
	c.Shrink(st)
	c.Shrink(ps)
	c.Shrink(dump)
	c.Shrink(tank1)
	c.Shrink(tank2)
	if empty := c.Shrink(tower); !empty {
		t.Error("complex should report empty after removing all buildings")
	}
}

func TestComplexTransferDistributesByType(t *testing.T) {
	c := NewComplex(1)
	dump := building(assets.ADump, 0, 0, 0, game.OrderPowerOn, 50) // raw storage
	fuel := building(assets.FDump, 0, 1, 0, game.OrderPowerOn, 30) // fuel storage
	c.Grow(dump)
	c.Grow(fuel)

	c.Transfer(40, 20, 0) // 40 raw, 20 fuel
	if dump.Storage != 40 {
		t.Errorf("raw dump storage = %d, want 40", dump.Storage)
	}
	if fuel.Storage != 20 {
		t.Errorf("fuel dump storage = %d, want 20", fuel.Storage)
	}
	if c.Material != 40 || c.Fuel != 20 {
		t.Errorf("complex stores = mat%d fuel%d, want 40/20", c.Material, c.Fuel)
	}

	// Inventory and capacity roll up across members.
	inv, cap := c.CargoInfo()
	if inv.Raw != 40 || inv.Fuel != 20 {
		t.Errorf("inventory = %+v, want raw40 fuel20", inv)
	}
	if cap.Raw != 50 || cap.Fuel != 30 {
		t.Errorf("capacity = %+v, want raw50 fuel30", cap)
	}
}

func TestComplexCargoMining(t *testing.T) {
	m := NewResourceMap(16, 16)
	m.SetDeposit(5, 5, CargoRaw, 10)
	m.SetDeposit(6, 5, CargoRaw, 10)

	c := NewComplex(1)
	st := building(assets.MiningSt, 0, 5, 5, game.OrderPowerOn, 0)
	st.RawMining, st.FuelMining, st.GoldMining, st.TotalMining = 14, 0, 0, 14
	c.Grow(st)

	materials, capacity := c.CargoMining(m)
	if materials.Raw != 14 {
		t.Errorf("mined raw = %d, want 14", materials.Raw)
	}
	if materials.FreeCapacity != 2 { // 16 - 14
		t.Errorf("free capacity = %d, want 2", materials.FreeCapacity)
	}
	if capacity.Raw != 16 { // 20 deposit, capped at 16 by the area limit
		t.Errorf("deposit capacity = %d, want 16", capacity.Raw)
	}
}

// TestCapStoresWastesWhatTheBaseCannotHold pins the storage cap. The port used to keep an
// unbounded pool: a mine kept filling past the base's capacity, the surplus stayed in the
// pool unshown, and the gauge read a full 25/25 while the base secretly held more. The
// original has no pool at all - the stock lives in the buildings - so anything over
// capacity is simply lost, and the cure is to build storage.
func TestCapStoresWastesWhatTheBaseCannotHold(t *testing.T) {
	c := NewComplex(1)
	c.Grow(building(assets.MiningSt, 0, 5, 5, game.OrderPowerOn, 25)) // 25 raw, no fuel/gold

	c.Material, c.Fuel, c.Gold = 40, 12, 9

	c.CapStores()

	if c.Material != 25 {
		t.Errorf("material = %d, want 25 (the mine's capacity)", c.Material)
	}
	// A mine stores no fuel and no gold: with no Fuel Tank and no Gold Vault, both spill.
	if c.Fuel != 0 || c.Gold != 0 {
		t.Errorf("fuel/gold = %d/%d, want 0/0 (nowhere to keep them)", c.Fuel, c.Gold)
	}

	// Raise a Storage Unit and the base keeps what it mines.
	c.Grow(building(assets.ADump, 0, 6, 5, game.OrderPowerOn, 50))
	c.Material = 40

	if c.CapStores(); c.Material != 40 {
		t.Errorf("with a Storage Unit: material = %d, want 40 kept", c.Material)
	}
}
