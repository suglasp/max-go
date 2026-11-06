// missionquery_test.go : tests the board questions a mission objective asks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// queryUnit deploys a unit of a type on a team at a cell, with the flags and values a
// real one of that type would carry.
func queryUnit(g *Game, team int, t assets.UnitType, id, x, y int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.InitState()
	g.Units.Deploy(u, x, y)

	return u
}

// TestHasAttackPowerTestsAmmoNotAttack pins the original's rule, which is not what the
// name says: a unit counts as attack power when it has AMMO, and the submarine and the
// commando never count at all.
func TestHasAttackPowerTestsAmmoNotAttack(t *testing.T) {
	g := newGame(t, 16, 16)

	tank := queryUnit(g, 0, assets.Tank, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Values.Set(game.AttribAttack, 20)
	tank.Ammo = 0 // out of ammo: no attack power, whatever its gun says

	if g.HasAttackPower(0, game.ClassMobileLandSea) {
		t.Error("a tank with no ammo counts as attack power")
	}

	tank.Ammo = 4

	if !g.HasAttackPower(0, game.ClassMobileLandSea) {
		t.Error("a loaded tank does not count as attack power")
	}

	// A commando and a submarine are loaded, and still do not count.
	g2 := newGame(t, 16, 16)
	cmd := queryUnit(g2, 0, assets.Commando, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	cmd.Ammo = 4
	sub := queryUnit(g2, 0, assets.Submarne, 2, 6, 6, game.FlagMobileSeaUnit|game.FlagSelectable)
	sub.Ammo = 4

	if g2.HasAttackPower(0, game.ClassMobileLandSea) {
		t.Error("a team down to a commando and a submarine counts as armed")
	}
}

// TestCountReadyUnitsSkipsWhatIsStillInTheFactory checks that a unit still sitting inside
// the factory that built it does not count yet.
func TestCountReadyUnitsSkipsWhatIsStillInTheFactory(t *testing.T) {
	g := newGame(t, 16, 16)

	queryUnit(g, 0, assets.Tank, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	fresh := queryUnit(g, 0, assets.Tank, 2, 6, 6, game.FlagMobileLandUnit|game.FlagSelectable)
	fresh.State = game.OrderStateBuildingReady

	if got := g.CountReadyUnits(0, assets.Tank); got != 1 {
		t.Fatalf("ready tanks = %d, want 1 (the second is still in the factory)", got)
	}
}

// TestStoredCargoSumsTheUnitsThatCarryIt checks the original's cargo rule: it sums the
// storage of the units whose TYPE carries that cargo, not a complex pool.
func TestStoredCargoSumsTheUnitsThatCarryIt(t *testing.T) {
	g := newGame(t, 16, 16)

	store := queryUnit(g, 0, assets.ADump, 1, 4, 4, game.FlagStationary|game.FlagBuilding)
	store.Storage = 30 // a storage unit holds RAW

	fuel := queryUnit(g, 0, assets.FDump, 2, 6, 6, game.FlagStationary|game.FlagBuilding)
	fuel.Storage = 12 // a fuel tank holds FUEL

	if got := g.StoredCargo(0, economy.CargoRaw); got != 30 {
		t.Errorf("raw in store = %d, want 30", got)
	}

	if got := g.StoredCargo(0, economy.CargoFuel); got != 12 {
		t.Errorf("fuel in store = %d, want 12", got)
	}

	if got := g.StoredCargo(0, economy.CargoGold); got != 0 {
		t.Errorf("gold in store = %d, want 0", got)
	}

	if got := g.StoredCargo(1, economy.CargoRaw); got != 0 {
		t.Errorf("the other team's raw = %d, want 0", got)
	}
}

// TestDamagedAndReducedCounts covers the two gauges the Repairing and Reloading lessons
// watch.
func TestDamagedAndReducedCounts(t *testing.T) {
	g := newGame(t, 16, 16)

	for i := 0; i < 3; i++ {
		u := queryUnit(g, 0, assets.Tank, i+1, 4+i, 4, game.FlagMobileLandUnit|game.FlagSelectable)
		u.Values.Set(game.AttribHits, 40)
		u.Values.Set(game.AttribAmmo, 4)
		u.Hits = 40
		u.Ammo = 4
	}

	if got := g.CountDamagedUnits(0, assets.Tank); got != 0 {
		t.Fatalf("damaged tanks = %d, want 0", got)
	}

	if got := g.CountUnitsWithReducedAttrib(0, assets.Tank, game.AttribAmmo); got != 0 {
		t.Fatalf("tanks short of ammo = %d, want 0", got)
	}

	hurt := g.Units.Units()[0]
	hurt.Hits = 12
	hurt.Ammo = 1

	if got := g.CountDamagedUnits(0, assets.Tank); got != 1 {
		t.Errorf("damaged tanks = %d, want 1", got)
	}

	if got := g.CountUnitsWithReducedAttrib(0, assets.Tank, game.AttribAmmo); got != 1 {
		t.Errorf("tanks short of ammo = %d, want 1", got)
	}
}

// TestTeamIsAliveIsTheDefaultRule checks that a team with no attack power, plant, or builder is beaten regardless of its remaining unit count.
func TestTeamIsAliveIsTheDefaultRule(t *testing.T) {
	g := newGame(t, 16, 16)
	g.State.Teams[0].Type = game.TeamPlayer

	// A surveyor and a mining station: units on the board, but nothing to fight, build or
	// produce with.
	queryUnit(g, 0, assets.Surveyor, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	queryUnit(g, 0, assets.MiningSt, 2, 8, 8, game.FlagStationary|game.FlagBuilding)

	if g.TeamIsAlive(0) {
		t.Fatal("a team with no gun, no plant and no builder still counts as alive")
	}

	status, ok := g.defaultCondition(0)
	if !ok || status != winloss.StatusLost {
		t.Fatalf("default condition = %v/%v, want Lost/true", status, ok)
	}

	// Give it a loaded tank and it is back in the fight.
	tank := queryUnit(g, 0, assets.Tank, 3, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Ammo = 4

	if !g.TeamIsAlive(0) {
		t.Fatal("a team with a loaded tank counts as beaten")
	}

	if status, _ := g.defaultCondition(0); status != winloss.StatusGeneric {
		t.Fatalf("default condition = %v, want Generic", status)
	}
}

// queryBuilding deploys a stationary building of a type on a team with an order.
func queryBuilding(g *Game, team int, t assets.UnitType, id, x, y int, order game.Order) *game.UnitInfo {
	u := queryUnit(g, team, t, id, x, y, game.FlagStationary|game.FlagSelectable)
	u.Order = order
	u.State = game.OrderStateExecutingOrder

	return u
}

// TestCountCasualties checks the per type loss tally and that a team out of range reads 0.
func TestCountCasualties(t *testing.T) {
	g := newGame(t, 16, 16)
	g.TeamStats[0].Casualties = map[assets.UnitType]int{assets.Tank: 3}
	g.TeamStats[1].Casualties = map[assets.UnitType]int{assets.Tank: 1, assets.Scout: 2}

	tests := []struct {
		name string
		team int
		unit assets.UnitType
		want int
	}{
		{"team 0 tanks", 0, assets.Tank, 3},
		{"team 1 tanks, not team 0", 1, assets.Tank, 1},
		{"team 1 scouts", 1, assets.Scout, 2},
		{"a type nobody lost", 0, assets.Scout, 0},
		{"team below range", -1, assets.Tank, 0},
		{"team above range", game.TeamCount, assets.Tank, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.CountCasualties(tc.team, tc.unit); got != tc.want {
				t.Errorf("CountCasualties(%d) = %d, want %d", tc.team, got, tc.want)
			}
		})
	}
}

// TestRawMiningCapacity checks that only powered mining stations of the team add their raw capacity.
func TestRawMiningCapacity(t *testing.T) {
	g := newGame(t, 16, 16)

	on := queryBuilding(g, 0, assets.MiningSt, 1, 2, 2, game.OrderPowerOn)
	on.RawMiningMax = 12
	off := queryBuilding(g, 0, assets.MiningSt, 2, 5, 2, game.OrderPowerOff)
	off.RawMiningMax = 9
	other := queryBuilding(g, 1, assets.MiningSt, 3, 8, 2, game.OrderPowerOn)
	other.RawMiningMax = 7
	notMine := queryBuilding(g, 0, assets.PowerStn, 4, 11, 2, game.OrderPowerOn)
	notMine.RawMiningMax = 5

	if got := g.RawMiningCapacity(0); got != 12 {
		t.Errorf("team 0 capacity = %d, want 12 (powered station only)", got)
	}

	if got := g.RawMiningCapacity(1); got != 7 {
		t.Errorf("team 1 capacity = %d, want 7", got)
	}

	off.Order = game.OrderPowerOn

	if got := g.RawMiningCapacity(0); got != 21 {
		t.Errorf("capacity with both stations on = %d, want 21", got)
	}

	for _, team := range []int{-1, game.TeamCount} {
		if got := g.RawMiningCapacity(team); got != 0 {
			t.Errorf("RawMiningCapacity(%d) = %d, want 0", team, got)
		}
	}
}

// TestTotalMining checks the per cargo allocation of powered stations, split by team.
func TestTotalMining(t *testing.T) {
	g := newGame(t, 16, 16)

	a := queryBuilding(g, 0, assets.MiningSt, 1, 2, 2, game.OrderPowerOn)
	a.RawMining, a.FuelMining, a.GoldMining = 10, 4, 2
	b := queryBuilding(g, 0, assets.MiningSt, 2, 5, 2, game.OrderPowerOn)
	b.RawMining, b.FuelMining, b.GoldMining = 3, 1, 0
	off := queryBuilding(g, 0, assets.MiningSt, 3, 8, 2, game.OrderPowerOff)
	off.RawMining, off.FuelMining, off.GoldMining = 50, 50, 50
	enemy := queryBuilding(g, 1, assets.MiningSt, 4, 11, 2, game.OrderPowerOn)
	enemy.RawMining, enemy.FuelMining, enemy.GoldMining = 6, 7, 8

	tests := []struct {
		name  string
		team  int
		cargo economy.CargoType
		want  int
	}{
		{"raw", 0, economy.CargoRaw, 13},
		{"fuel", 0, economy.CargoFuel, 5},
		{"gold", 0, economy.CargoGold, 2},
		{"other team raw", 1, economy.CargoRaw, 6},
		{"other team gold", 1, economy.CargoGold, 8},
		{"no cargo", 0, economy.CargoNone, 0},
		{"team below range", -1, economy.CargoRaw, 0},
		{"team above range", game.TeamCount, economy.CargoRaw, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.TotalMining(tc.team, tc.cargo); got != tc.want {
				t.Errorf("TotalMining = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestTotalConsumption checks that only the negative net of stationary units counts, returned positive.
func TestTotalConsumption(t *testing.T) {
	g := newGame(t, 16, 16)

	// A powered power station burns 6 fuel a turn.
	queryBuilding(g, 0, assets.PowerStn, 1, 2, 2, game.OrderPowerOn)
	queryBuilding(g, 0, assets.PowerStn, 2, 5, 2, game.OrderPowerOn)
	// An unpowered one burns nothing.
	queryBuilding(g, 0, assets.PowerStn, 3, 8, 2, game.OrderPowerOff)
	// A powered mining station has a positive fuel net: not consumption.
	mine := queryBuilding(g, 0, assets.MiningSt, 4, 11, 2, game.OrderPowerOn)
	mine.FuelMining = 5
	// A mobile unit never counts, whatever its type would burn.
	mobile := queryUnit(g, 0, assets.PowerStn, 5, 3, 8, game.FlagMobileLandUnit)
	mobile.Order = game.OrderPowerOn
	// The other team's power station is not ours.
	queryBuilding(g, 1, assets.PowerStn, 6, 6, 8, game.OrderPowerOn)

	if got := g.TotalConsumption(0, economy.CargoFuel); got != 12 {
		t.Errorf("fuel consumption = %d, want 12 (two powered stations)", got)
	}

	if got := g.TotalConsumption(1, economy.CargoFuel); got != 6 {
		t.Errorf("team 1 fuel consumption = %d, want 6", got)
	}

	if got := g.TotalConsumption(0, economy.CargoRaw); got != 0 {
		t.Errorf("raw consumption = %d, want 0 (the mine produces raw)", got)
	}

	if got := g.TotalConsumption(0, economy.CargoGold); got != 0 {
		t.Errorf("gold consumption = %d, want 0", got)
	}

	if got := g.TotalConsumption(-1, economy.CargoFuel); got != 0 {
		t.Errorf("team -1 consumption = %d, want 0", got)
	}
}

// TestRunningBuildings checks that only stationary buildings of the type that are on or going up count.
func TestRunningBuildings(t *testing.T) {
	g := newGame(t, 16, 16)

	queryBuilding(g, 0, assets.LightPlt, 1, 2, 2, game.OrderPowerOn)
	queryBuilding(g, 0, assets.LightPlt, 2, 5, 2, game.OrderBuild)
	queryBuilding(g, 0, assets.LightPlt, 3, 8, 2, game.OrderPowerOff)
	queryBuilding(g, 0, assets.LightPlt, 4, 11, 2, game.OrderAwait)
	queryBuilding(g, 0, assets.LandPlt, 5, 2, 6, game.OrderPowerOn)
	queryBuilding(g, 1, assets.LightPlt, 6, 5, 6, game.OrderPowerOn)
	mobile := queryUnit(g, 0, assets.LightPlt, 7, 8, 6, game.FlagMobileLandUnit)
	mobile.Order = game.OrderPowerOn

	tests := []struct {
		name string
		team int
		unit assets.UnitType
		want int
	}{
		{"light plants on or building", 0, assets.LightPlt, 2},
		{"land plants", 0, assets.LandPlt, 1},
		{"a type with none", 0, assets.AirPlt, 0},
		{"other team", 1, assets.LightPlt, 1},
		{"team below range", -1, assets.LightPlt, 0},
		{"team above range", game.TeamCount, assets.LightPlt, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.RunningBuildings(tc.team, tc.unit); got != tc.want {
				t.Errorf("RunningBuildings = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestUnitsBeingConstructed checks that a builder's order and a producing factory's queue head count, nothing else.
func TestUnitsBeingConstructed(t *testing.T) {
	g := newGame(t, 16, 16)

	if got := g.UnitsBeingConstructed(0, assets.GunTurrt); got != 0 {
		t.Fatalf("empty board = %d, want 0", got)
	}

	// A builder part way through a turret.
	engineer := queryUnit(g, 0, assets.Engineer, 1, 2, 2, game.FlagMobileLandUnit|game.FlagConstructorUnit)
	turret := game.NewUnitInfo(assets.GunTurrt, 0, 2, 0)
	turret.Flags = game.FlagStationary
	g.StartBuild(engineer, turret, 3, 2, 1)

	if got := g.UnitsBeingConstructed(0, assets.GunTurrt); got != 1 {
		t.Errorf("one turret on order = %d, want 1", got)
	}

	if got := g.UnitsBeingConstructed(1, assets.GunTurrt); got != 0 {
		t.Errorf("other team = %d, want 0", got)
	}

	if got := g.UnitsBeingConstructed(0, assets.Tank); got != 0 {
		t.Errorf("another type = %d, want 0", got)
	}

	// A factory producing a tank then a scout: only the head counts.
	plant := queryBuilding(g, 0, assets.LightPlt, 3, 8, 8, game.OrderPowerOn)
	g.SetFactoryQueue(plant, []assets.UnitType{assets.Tank, assets.Scout}, 1, false)

	if got := g.UnitsBeingConstructed(0, assets.Tank); got != 1 {
		t.Errorf("queue head tank = %d, want 1", got)
	}

	if got := g.UnitsBeingConstructed(0, assets.Scout); got != 0 {
		t.Errorf("later queue entry scout = %d, want 0", got)
	}

	// A stopped factory builds nothing.
	g.PauseFactory(plant)

	if got := g.UnitsBeingConstructed(0, assets.Tank); got != 0 {
		t.Errorf("stopped factory tank = %d, want 0", got)
	}

	for _, team := range []int{-1, game.TeamCount} {
		if got := g.UnitsBeingConstructed(team, assets.GunTurrt); got != 0 {
			t.Errorf("UnitsBeingConstructed(%d) = %d, want 0", team, got)
		}
	}
}

// TestHasUnitAboveMarkLevel checks the strict greater than rule and the mobile land and sea restriction.
func TestHasUnitAboveMarkLevel(t *testing.T) {
	g := newGame(t, 16, 16)

	tank := queryUnit(g, 0, assets.Tank, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Values.SetVersion(1)

	if g.HasUnitAboveMarkLevel(0, assets.Tank, 1) {
		t.Error("a Mk I tank is above Mk 1")
	}

	if !g.HasUnitAboveMarkLevel(0, assets.Tank, 0) {
		t.Error("a Mk I tank is not above Mk 0")
	}

	tank.Values.SetVersion(2)

	if !g.HasUnitAboveMarkLevel(0, assets.Tank, 1) {
		t.Error("an upgraded tank is not above Mk 1")
	}

	if g.HasUnitAboveMarkLevel(1, assets.Tank, 1) {
		t.Error("another team's query sees our tank")
	}

	if g.HasUnitAboveMarkLevel(0, assets.Scout, 1) {
		t.Error("a type we do not field counts")
	}

	// A stationary or air unit of a high version does not count.
	building := queryBuilding(g, 0, assets.GunTurrt, 2, 6, 2, game.OrderPowerOn)
	building.Values.SetVersion(5)

	if g.HasUnitAboveMarkLevel(0, assets.GunTurrt, 1) {
		t.Error("an upgraded stationary turret counts")
	}

	air := queryUnit(g, 0, assets.Fighter, 3, 9, 2, game.FlagMobileAirUnit|game.FlagSelectable)
	air.Values.SetVersion(5)

	if g.HasUnitAboveMarkLevel(0, assets.Fighter, 1) {
		t.Error("an upgraded air unit counts")
	}

	for _, team := range []int{-1, game.TeamCount} {
		if g.HasUnitAboveMarkLevel(team, assets.Tank, 0) {
			t.Errorf("HasUnitAboveMarkLevel(%d) = true, want false", team)
		}
	}
}

// TestHasUnitExperience checks that a unit of the type needs Experience above 0, and the team must match.
func TestHasUnitExperience(t *testing.T) {
	g := newGame(t, 16, 16)

	cmd := queryUnit(g, 0, assets.Commando, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)

	if g.HasUnitExperience(0, assets.Commando) {
		t.Error("a green commando is experienced")
	}

	cmd.Experience = 1

	if !g.HasUnitExperience(0, assets.Commando) {
		t.Error("a blooded commando is not experienced")
	}

	if g.HasUnitExperience(0, assets.Tank) {
		t.Error("another type counts")
	}

	if g.HasUnitExperience(1, assets.Commando) {
		t.Error("another team sees our commando")
	}

	for _, team := range []int{-1, game.TeamCount} {
		if g.HasUnitExperience(team, assets.Commando) {
			t.Errorf("HasUnitExperience(%d) = true, want false", team)
		}
	}
}

// TestResearchLevel checks the level read per team and topic, and that out of range arguments give 0.
func TestResearchLevel(t *testing.T) {
	g := newGame(t, 16, 16)
	g.State.Teams[0].Research[game.ResearchArmor].Level = 3
	g.State.Teams[1].Research[game.ResearchArmor].Level = 1

	tests := []struct {
		name  string
		team  int
		topic int
		want  int
	}{
		{"team 0 armor", 0, game.ResearchArmor, 3},
		{"team 1 armor", 1, game.ResearchArmor, 1},
		{"an untouched topic", 0, 0, 0},
		{"topic below range", 0, -1, 0},
		{"topic above range", 0, game.ResearchTopicCount, 0},
		{"team below range", -1, game.ResearchArmor, 0},
		{"team above range", game.TeamCount, game.ResearchArmor, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.ResearchLevel(tc.team, tc.topic); got != tc.want {
				t.Errorf("ResearchLevel = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestPoints checks that Points reads the live team score and guards the team index.
func TestPoints(t *testing.T) {
	g := newGame(t, 16, 16)
	g.TeamPoints[0] = 7
	g.TeamPoints[1] = 2

	tests := []struct {
		team int
		want int
	}{{0, 7}, {1, 2}, {2, 0}, {-1, 0}, {game.TeamCount, 0}}

	for _, tc := range tests {
		if got := g.Points(tc.team); got != tc.want {
			t.Errorf("Points(%d) = %d, want %d", tc.team, got, tc.want)
		}
	}
}

// TestTurnValue checks that TurnValue follows the turn counter.
func TestTurnValue(t *testing.T) {
	g := newGame(t, 16, 16)

	for _, turn := range []int{0, 1, 42} {
		g.State.TurnCounter = turn

		if got := g.TurnValue(); got != turn {
			t.Errorf("TurnValue = %d, want %d", got, turn)
		}
	}
}

// TestTurnVictoryLimit checks that the limit is 0 unless the game is a Duration game.
func TestTurnVictoryLimit(t *testing.T) {
	g := newGame(t, 16, 16)
	g.VictoryLimit = 30

	for _, vt := range []winloss.VictoryType{winloss.VictoryScore, winloss.VictoryDuration} {
		g.VictoryType = vt
		want := 0

		if vt == winloss.VictoryDuration {
			want = 30
		}

		if got := g.TurnVictoryLimit(); got != want {
			t.Errorf("TurnVictoryLimit with victory type %v = %d, want %d", vt, got, want)
		}
	}
}
