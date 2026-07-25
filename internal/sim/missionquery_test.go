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
