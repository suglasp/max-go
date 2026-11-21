// teamstats_test.go : tests teamstats.go.
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
)

// liveBuilding is a 2x2 stationary structure with the flags a real building
// carries in play (Building + Stationary + Selectable), so casualty
// classification matches gameplay (a building is both BUILDING and SELECTABLE).
func liveBuilding(t assets.UnitType, team, id int) *game.UnitInfo {
	u := game.NewUnitInfo(t, team, id, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribHits, 50)
	return u
}

// TestTeamStatsBuildingCounts: PlaceBuilding tallies any BUILDING into
// BuildingsBuilt, a factory (Light/Heavy/Air/Shipyard) also into FactoriesBuilt,
// and a mining station also into MinesBuilt. A plain building bumps only the
// buildings total.
func TestTeamStatsBuildingCounts(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	g.PlaceBuilding(liveBuilding(assets.LightPlt, 0, 1), 4, 4)   // a factory
	g.PlaceBuilding(liveBuilding(assets.MiningSt, 0, 2), 10, 10) // a mining station
	g.PlaceBuilding(liveBuilding(assets.PowerStn, 0, 3), 16, 16) // a plain building

	ts := g.TeamStats[0]
	if ts.FactoriesBuilt != 1 {
		t.Errorf("FactoriesBuilt = %d, want 1 (only the LightPlt counts)", ts.FactoriesBuilt)
	}
	if ts.MinesBuilt != 1 {
		t.Errorf("MinesBuilt = %d, want 1", ts.MinesBuilt)
	}
	if ts.BuildingsBuilt != 3 {
		t.Errorf("BuildingsBuilt = %d, want 3", ts.BuildingsBuilt)
	}
}

// TestTeamStatsFactoryUnitBuilt: a factory producing a mobile unit bumps
// UnitsBuilt (via completeFactoryOutput), and the factory's own placement is
// counted as a built factory.
func TestTeamStatsFactoryUnitBuilt(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2)
	f := placeFactory(g, 4, 2) // a Heavy Vehicle Plant
	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)

	buildUntilReady(t, g, f) // the produced Tank parks build ready -> completeFactoryOutput ran

	ts := g.TeamStats[0]
	if ts.UnitsBuilt != 1 {
		t.Errorf("UnitsBuilt = %d, want 1 after the factory produced a unit", ts.UnitsBuilt)
	}
	if ts.FactoriesBuilt != 1 || ts.BuildingsBuilt != 1 {
		t.Errorf("factory placement tally: factories=%d buildings=%d, want 1/1",
			ts.FactoriesBuilt, ts.BuildingsBuilt)
	}
}

// TestTeamStatsCasualties: DestroyUnit tallies losses. A building counts only in
// BuildingsLost; a mobile unit counts only in UnitsLost.
func TestTeamStatsCasualties(t *testing.T) {
	g := newGame(t, 24, 24)

	b := liveBuilding(assets.PowerStn, 0, 1)
	g.Units.Deploy(b, 5, 5)
	g.DestroyUnit(b)

	tank := addTank(g, 2, 6, 12, 12)
	g.DestroyUnit(tank)

	ts := g.TeamStats[0]
	if ts.BuildingsLost != 1 {
		t.Errorf("BuildingsLost = %d, want 1", ts.BuildingsLost)
	}
	if ts.UnitsLost != 1 { // the tank only
		t.Errorf("UnitsLost = %d, want 1 (a destroyed building is not a unit lost)", ts.UnitsLost)
	}
}

// TestTeamStatsConnectorLossIsNoUnit: a destroyed non mobile selectable building
// (a connector is not BUILDING, a power station is) never adds a unit lost.
func TestTeamStatsConnectorLossIsNoUnit(t *testing.T) {
	g := newGame(t, 24, 24)

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 1, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable | game.FlagConnectorUnit
	conn.Values = &game.UnitValues{}
	g.Units.Deploy(conn, 3, 3)
	g.recordCasualty(conn)

	ts := g.TeamStats[0]
	if ts.UnitsLost != 0 || ts.BuildingsLost != 0 {
		t.Errorf("connector loss: units %d buildings %d, want 0/0", ts.UnitsLost, ts.BuildingsLost)
	}
	if ts.Casualties[assets.Cnct4W] != 1 {
		t.Errorf("connector Casualties = %d, want 1", ts.Casualties[assets.Cnct4W])
	}

	g.recordCasualty(liveBuilding(assets.PowerStn, 0, 2))
	ts = g.TeamStats[0]
	if ts.UnitsLost != 0 || ts.BuildingsLost != 1 {
		t.Errorf("building loss: units %d buildings %d, want 0/1", ts.UnitsLost, ts.BuildingsLost)
	}
}

// TestTeamStatsCasualtyGate: a unit that is exploding or a missile is not tallied
// at all; a non selectable unit (a laid mine) is.
func TestTeamStatsCasualtyGate(t *testing.T) {
	g := newGame(t, 24, 24)
	for i, fl := range []game.Flags{
		game.FlagMobileLandUnit | game.FlagSelectable | game.FlagExploding,
		game.FlagMobileLandUnit | game.FlagSelectable | game.FlagMissileUnit,
	} {
		u := game.NewUnitInfo(assets.Tank, 0, 1+i, 0)
		u.Flags = fl
		g.recordCasualty(u)
	}
	if ts := g.TeamStats[0]; ts.UnitsLost != 0 || len(ts.Casualties) != 0 {
		t.Errorf("gated units tallied: lost %d, casualties %v", ts.UnitsLost, ts.Casualties)
	}

	mine := game.NewUnitInfo(assets.LandMine, 0, 9, 0)
	mine.Flags = game.FlagStationary
	g.recordCasualty(mine)
	if n := g.TeamStats[0].Casualties[assets.LandMine]; n != 1 {
		t.Errorf("non selectable mine Casualties = %d, want 1", n)
	}
}

// TestMineCasualtyTally: the live mine trigger consumes the mine and tallies it
// once in Casualties; a Go mine is neither mobile nor BUILDING, so no unit or
// building lost.
func TestMineCasualtyTally(t *testing.T) {
	g := newGame(t, 12, 12)
	layer := addMineLayer(g, assets.MineLayr, 0, 5, 5, 3)
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}
	addTankTarget(g, 50, 5, 5, 1)
	g.detonateMines()
	if g.mineOnCell(assets.LandMine, 0, 5, 5) != nil {
		t.Fatal("mine not consumed")
	}
	ts := g.TeamStats[0]
	if n := ts.Casualties[assets.LandMine]; n != 1 {
		t.Errorf("LandMine Casualties = %d, want 1", n)
	}
	if ts.UnitsLost != 0 || ts.BuildingsLost != 0 {
		t.Errorf("mine loss: units %d buildings %d, want 0/0", ts.UnitsLost, ts.BuildingsLost)
	}
}

// TestRecountBuiltStatistics: the four built tallies are rebuilt from the units in
// play; upgrade gold, losses and casualties stay, ground cover and the alien team
// are skipped, and held cargo counts as built.
func TestRecountBuiltStatistics(t *testing.T) {
	g := newGame(t, 32, 32)
	g.Units.Deploy(liveBuilding(assets.LightPlt, 0, 1), 2, 2)
	g.Units.Deploy(liveBuilding(assets.MiningSt, 0, 2), 6, 6)

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 3, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable | game.FlagConnectorUnit
	g.Units.Deploy(conn, 10, 10)

	road := game.NewUnitInfo(assets.Road, 0, 4, 0)
	road.Flags = game.FlagStationary | game.FlagBuilding | game.FlagGroundCover
	g.Units.Deploy(road, 12, 12)

	tank := addTank(g, 5, 6, 14, 14)
	cargo := game.NewUnitInfo(assets.Scout, 0, 6, 0)
	cargo.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Carried = append(tank.Carried, cargo)

	alien := game.NewUnitInfo(assets.Tank, game.AlienTeam, 7, 0)
	alien.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(alien, 20, 20)

	g.TeamStats[0] = TeamStats{FactoriesBuilt: 9, MinesBuilt: 9, BuildingsBuilt: 9, UnitsBuilt: 9,
		UnitsLost: 4, BuildingsLost: 2, GoldOnUpgrades: 77, Casualties: map[assets.UnitType]int{assets.Tank: 3}}

	g.RecountBuiltStatistics()

	ts := g.TeamStats[0]
	if ts.FactoriesBuilt != 1 || ts.MinesBuilt != 1 || ts.BuildingsBuilt != 2 || ts.UnitsBuilt != 2 {
		t.Errorf("built: factories %d mines %d buildings %d units %d, want 1/1/2/2",
			ts.FactoriesBuilt, ts.MinesBuilt, ts.BuildingsBuilt, ts.UnitsBuilt)
	}
	if ts.GoldOnUpgrades != 77 || ts.UnitsLost != 4 || ts.BuildingsLost != 2 || ts.Casualties[assets.Tank] != 3 {
		t.Errorf("recount touched gold/losses/casualties: %+v", ts)
	}
	if other := g.TeamStats[1]; other.UnitsBuilt != 0 || other.BuildingsBuilt != 0 {
		t.Errorf("team 1 counted something: %+v", other)
	}
}

// TestTeamStatsCarriedCasualties: destroying a transport also tallies the units
// it was carrying (passengers live off the registry and are never destroyed
// individually).
func TestTeamStatsCarriedCasualties(t *testing.T) {
	g := newGame(t, 24, 24)

	transport := game.NewUnitInfo(assets.SeaTrans, 0, 1, 0)
	transport.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	transport.Values = &game.UnitValues{}
	g.Units.Deploy(transport, 8, 8)
	for i := 0; i < 2; i++ {
		p := game.NewUnitInfo(assets.Tank, 0, 2+i, 0)
		p.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		p.Values = &game.UnitValues{}
		transport.Carried = append(transport.Carried, p)
	}

	g.DestroyUnit(transport)

	if got := g.TeamStats[0].UnitsLost; got != 3 {
		t.Errorf("UnitsLost = %d, want 3 (the transport + 2 carried units)", got)
	}
}

// TestRecordUpgradeGold accumulates spent gold and ignores non positive deltas.
func TestRecordUpgradeGold(t *testing.T) {
	g := newGame(t, 8, 8)
	g.RecordUpgradeGold(0, 30)
	g.RecordUpgradeGold(0, 20)
	g.RecordUpgradeGold(0, -5) // ignored
	g.RecordUpgradeGold(0, 0)  // ignored
	if got := g.TeamStats[0].GoldOnUpgrades; got != 50 {
		t.Errorf("GoldOnUpgrades = %d, want 50", got)
	}
}

/* // note: unused code in Go port
// tested Land, commented out with it; TestDeployStartBaseCountsRoster (skipped, DC-F8) covers the live landing.
// TestTeamStatsLandingRoster: the landing colony counts its mine + power
// generator as buildings (the mine also as a mine) and its deployed loadout as
// units built.
func TestTeamStatsLandingRoster(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	mine := liveBuilding(assets.MiningSt, 0, 1)
	gen := liveBuilding(assets.PowerStn, 0, 2)
	mk := func(id int) *game.UnitInfo {
		u := game.NewUnitInfo(assets.Tank, 0, id, 2)
		u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribSpeed, 6)
		return u
	}
	loadout := []*game.UnitInfo{mk(3), mk(4)}

	g.Land(0, 10, 10, 100, mine, gen, loadout)

	ts := g.TeamStats[0]
	if ts.MinesBuilt != 1 {
		t.Errorf("MinesBuilt = %d, want 1", ts.MinesBuilt)
	}
	if ts.BuildingsBuilt != 2 { // mine + power generator
		t.Errorf("BuildingsBuilt = %d, want 2 (mine + generator)", ts.BuildingsBuilt)
	}
	if ts.UnitsBuilt != 2 {
		t.Errorf("UnitsBuilt = %d, want 2 (the landed roster)", ts.UnitsBuilt)
	}
}
*/
