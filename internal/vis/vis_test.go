// vis_test.go : testing unit for vis.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package vis

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// allWater is a synthetic SurfaceSource where every cell is water, so a colony
// transport counts as submerged.
type allWater struct{}

func (allWater) SurfaceAt(x, y int) uint8 { return world.SurfaceWater }

// unit builds a selectable unit of a type/team at a cell with a scan range.
func unit(typ assets.UnitType, team, gx, gy, scan int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, 0, 2)
	u.Flags = game.FlagSelectable
	u.GridX, u.GridY = gx, gy
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribScan, scan)
	return u
}

// TestScanCoverageIsCircular checks the reveal shape: a cell exactly scan
// tiles away on an axis is covered, but the diagonal corner at the same tile
// offset is out of the pixel radius.
func TestScanCoverageIsCircular(t *testing.T) {
	v := NewView(16, 16)
	scout := unit(assets.Tank, 0, 5, 5, 2)
	v.Rebuild([]*game.UnitInfo{scout})
	m := v.Team(0)

	if m.Complete(5, 5) == 0 {
		t.Error("own cell should be covered")
	}
	if m.Complete(7, 5) == 0 { // dx = 2 tiles, on the radius
		t.Error("cell two tiles east (on the radius) should be covered")
	}
	if m.Complete(7, 7) != 0 { // diagonal corner, sqrt(8) tiles > 2
		t.Error("diagonal corner two tiles out should be beyond the circular radius")
	}
	if m.Complete(5, 8) != 0 { // dy = 3 tiles, out of range
		t.Error("cell three tiles away should not be covered")
	}
}

// TestFogMemoryPersists checks that a cell scanned then abandoned downgrades
// from Visible to Explored (not back to Unseen).
func TestFogMemoryPersists(t *testing.T) {
	v := NewView(16, 4)
	scout := unit(assets.Tank, 0, 2, 1, 1)
	v.Rebuild([]*game.UnitInfo{scout})
	m := v.Team(0)
	if got := m.FogAt(2, 1); got != FogVisible {
		t.Errorf("scanned cell fog = %d, want FogVisible", got)
	}
	if got := m.FogAt(10, 1); got != FogUnseen {
		t.Errorf("far cell fog = %d, want FogUnseen", got)
	}

	scout.GridX = 10 // unit walks away
	v.Rebuild([]*game.UnitInfo{scout})
	if got := m.FogAt(2, 1); got != FogExplored {
		t.Errorf("abandoned cell fog = %d, want FogExplored", got)
	}
	if got := m.FogAt(10, 1); got != FogVisible {
		t.Errorf("new cell fog = %d, want FogVisible", got)
	}
}

// TestEnemyVisibilityFollowsCoverage checks an enemy unit is visible only
// while a team unit covers its cell, and own units are always visible.
func TestEnemyVisibilityFollowsCoverage(t *testing.T) {
	v := NewView(16, 4)
	scout := unit(assets.Tank, 0, 2, 1, 2)
	enemyNear := unit(assets.Tank, 1, 3, 1, 0) // within scout's range
	enemyFar := unit(assets.Tank, 1, 12, 1, 0) // out of range
	v.Rebuild([]*game.UnitInfo{scout, enemyNear, enemyFar})

	if !v.VisibleToTeam(enemyNear, 0) {
		t.Error("nearby enemy should be visible to team 0")
	}
	if v.VisibleToTeam(enemyFar, 0) {
		t.Error("distant enemy should not be visible to team 0")
	}
	if !v.VisibleToTeam(scout, 0) {
		t.Error("own unit should always be visible to its team")
	}
}

// TestSubmarineNeedsSonar checks a submerged unit is hidden under plain scan
// coverage and revealed only by a corvette's sonar (stealthSea).
func TestSubmarineNeedsSonar(t *testing.T) {
	sub := unit(assets.Submarne, 1, 4, 1, 0)

	// Plain tank coverage: complete > 0 but no sonar.
	v := NewView(16, 4)
	tank := unit(assets.Tank, 0, 4, 1, 3)
	v.Rebuild([]*game.UnitInfo{tank, sub})
	if v.Team(0).Complete(4, 1) == 0 {
		t.Fatal("test setup: sub's cell should have complete coverage")
	}
	if v.VisibleToTeam(sub, 0) {
		t.Error("submarine should stay hidden under plain scan coverage")
	}

	// Corvette adds sonar coverage.
	v2 := NewView(16, 4)
	corvette := unit(assets.Corvette, 0, 4, 1, 3)
	v2.Rebuild([]*game.UnitInfo{corvette, sub})
	if !v2.VisibleToTeam(sub, 0) {
		t.Error("submarine should be spotted within sonar (corvette) range")
	}
}

// TestSurfacedSubmarineVisibleToGeneralCoverage checks that a submarine that
// has surfaced (u.Surfaced) is visible to plain scan coverage, not gated
// behind Corvette sonar the way a submerged one is.
func TestSurfacedSubmarineVisibleToGeneralCoverage(t *testing.T) {
	sub := unit(assets.Submarne, 1, 4, 1, 0)
	tank := unit(assets.Tank, 0, 4, 1, 3)

	v := NewView(16, 4)
	v.Rebuild([]*game.UnitInfo{tank, sub})
	if v.VisibleToTeam(sub, 0) {
		t.Fatal("test setup: a submerged sub should stay hidden under plain scan coverage")
	}

	sub.Surfaced = true
	v.Rebuild([]*game.UnitInfo{tank, sub})
	if !v.VisibleToTeam(sub, 0) {
		t.Error("a surfaced submarine should be visible to plain scan coverage, like a normal unit")
	}
}

// TestColonyTransportSubmergedOverWater checks the stealth that depends on
// surface for a colony transport: hidden over water without sonar.
func TestColonyTransportSubmergedOverWater(t *testing.T) {
	v := NewView(16, 4)
	v.Surf = allWater{}
	tank := unit(assets.Tank, 0, 4, 1, 3)
	cln := unit(assets.ClnTrans, 1, 4, 1, 0)
	v.Rebuild([]*game.UnitInfo{tank, cln})
	if v.VisibleToTeam(cln, 0) {
		t.Error("colony transport over water should be hidden from plain scan")
	}

	// Without a water surface it is treated as on land, plain coverage spots it.
	dry := NewView(16, 4)
	dry.Rebuild([]*game.UnitInfo{tank, cln})
	if !dry.VisibleToTeam(cln, 0) {
		t.Error("colony transport on land should be visible under plain coverage")
	}
}

// TestCommandoNeedsLandDetector checks a commando is hidden under plain scan
// and revealed by an infantry/commando's land stealth coverage.
func TestCommandoNeedsLandDetector(t *testing.T) {
	enemyCmd := unit(assets.Commando, 1, 4, 1, 0)

	v := NewView(16, 4)
	tank := unit(assets.Tank, 0, 4, 1, 3)
	v.Rebuild([]*game.UnitInfo{tank, enemyCmd})
	if v.VisibleToTeam(enemyCmd, 0) {
		t.Error("commando should stay hidden under plain scan coverage")
	}

	v2 := NewView(16, 4)
	scout := unit(assets.Infantry, 0, 4, 1, 3)
	v2.Rebuild([]*game.UnitInfo{scout, enemyCmd})
	if !v2.VisibleToTeam(enemyCmd, 0) {
		t.Error("commando should be spotted within an infantry's land-stealth range")
	}
}

// TestMineHiddenFromEnemy checks a laid enemy land mine is off the heat map
// entirely: no amount of scan coverage finds it, not even an infantry's land
// stealth (that layer only uncovers a commando). It shows up when a mine detector
// has marked it - sim.sweepMineDetectors sets that mark - so until then the enemy
// drives straight onto it.
func TestMineHiddenFromEnemy(t *testing.T) {
	enemyMine := unit(assets.LandMine, 1, 4, 1, 0)

	v := NewView(16, 4)
	tank := unit(assets.Tank, 0, 4, 1, 3)
	v.Rebuild([]*game.UnitInfo{tank, enemyMine})
	if v.VisibleToTeam(enemyMine, 0) {
		t.Error("an enemy mine should stay hidden under plain scan coverage")
	}

	v2 := NewView(16, 4)
	infantry := unit(assets.Infantry, 0, 4, 1, 3) // land stealth coverage, but no mine detection
	v2.Rebuild([]*game.UnitInfo{infantry, enemyMine})
	if v2.VisibleToTeam(enemyMine, 0) {
		t.Error("infantry uncovers commandos, not mines; the mine must stay hidden")
	}

	enemyMine.SpotBy(0) // a surveyor / mine layer / commando walked up to it
	if !v2.VisibleToTeam(enemyMine, 0) {
		t.Error("an uncovered mine should be visible to the team that found it")
	}
}

// TestStealthHiddenFrom checks the render/click stealth gate: an enemy mine is
// hidden from a team that has not uncovered it but not from its own team, and a
// unit with no stealth is never reported as hidden by stealth (it relies on the fog
// overlay instead, so an idle enemy structure is not wrongly hidden).
func TestStealthHiddenFrom(t *testing.T) {
	enemyMine := unit(assets.LandMine, 1, 4, 1, 0)

	v := NewView(16, 4)
	tank := unit(assets.Tank, 0, 4, 1, 3) // plain scan coverage, no mine detection
	v.Rebuild([]*game.UnitInfo{tank, enemyMine})
	if !v.StealthHiddenFrom(enemyMine, 0) {
		t.Error("an enemy mine nobody has uncovered must be stealth-hidden")
	}
	if v.StealthHiddenFrom(enemyMine, 1) {
		t.Error("a team always sees its own mine")
	}
	if v.StealthHiddenFrom(tank, 1) {
		t.Error("a non-stealth enemy tank must never be reported stealth-hidden")
	}

	enemyMine.SpotBy(0)
	if v.StealthHiddenFrom(enemyMine, 0) {
		t.Error("an uncovered mine must be drawn for the team that found it")
	}
	if !v.StealthHiddenFrom(enemyMine, 2) {
		t.Error("uncovering a mine for one team must not hand it to another")
	}
}

// TestBuildingVisibleByAnyCorner checks a 2x2 building is visible when only
// one of its corners is within scan range.
func TestBuildingVisibleByAnyCorner(t *testing.T) {
	v := NewView(16, 8)
	// Scout covers only cell (6,6): the building's bottom right corner.
	scout := unit(assets.Tank, 0, 6, 6, 0)
	mine := game.NewUnitInfo(assets.Tank, 1, 0, 2)
	mine.Flags = game.FlagSelectable | game.FlagBuilding
	mine.GridX, mine.GridY = 5, 5 // occupies (5,5)(6,5)(5,6)(6,6)
	mine.Values = &game.UnitValues{}
	v.Rebuild([]*game.UnitInfo{scout, mine})

	if v.Team(0).Complete(5, 5) != 0 {
		t.Fatal("test setup: only the (6,6) corner should be covered")
	}
	if !v.VisibleToTeam(mine, 0) {
		t.Error("building should be visible when any of its four corners is covered")
	}
}

// TestIdleUnitNeitherScansNorIsSeen checks an idle unit projects no coverage
// and is not reported visible.
func TestIdleUnitNeitherScansNorIsSeen(t *testing.T) {
	v := NewView(16, 4)
	idle := unit(assets.Tank, 0, 2, 1, 3)
	idle.Order = game.OrderIdle
	enemy := unit(assets.Tank, 1, 3, 1, 0)
	enemy.Order = game.OrderIdle
	v.Rebuild([]*game.UnitInfo{idle, enemy})

	if v.Team(0).Complete(3, 1) != 0 {
		t.Error("idle unit should project no scan coverage")
	}
	if v.VisibleToTeam(enemy, 0) {
		t.Error("idle enemy should not be reported visible")
	}
}

// TestParkedUnitDoesNotScan checks that a unit still hidden in its factory (or a building not yet revealed behind its
// parked builder) projects no coverage: a finished Corvette parked in the Shipyard does not reveal a submarine until it
// leaves.
func TestParkedUnitDoesNotScan(t *testing.T) {
	sub := unit(assets.Submarne, 1, 4, 1, 0)
	corvette := unit(assets.Corvette, 0, 4, 1, 3)
	corvette.HiddenUntilExpand = true

	v := NewView(16, 4)
	v.Rebuild([]*game.UnitInfo{corvette, sub})
	if v.Team(0).Complete(4, 1) != 0 {
		t.Error("a parked unit gives scan coverage")
	}
	if v.VisibleToTeam(sub, 0) {
		t.Error("a Corvette parked in its factory reveals the submarine")
	}

	corvette.HiddenUntilExpand = false // it drives out
	v.Rebuild([]*game.UnitInfo{corvette, sub})
	if !v.VisibleToTeam(sub, 0) {
		t.Error("the Corvette that left its factory must reveal the submarine")
	}
}
