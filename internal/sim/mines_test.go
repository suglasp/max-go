// mines_test.go : tests mines.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"os"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/resfile"
)

// addMineLayer deploys a mine layer of the given type loaded with `capacity` mines.
func addMineLayer(g *Game, typ assets.UnitType, team, gx, gy, capacity int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribStorage, capacity)
	u.InitState() // Storage = capacity for a mine layer
	g.Units.Deploy(u, gx, gy)
	return u
}

func TestLayAndRemoveMine(t *testing.T) {
	g := newGame(t, 16, 16)
	layer := addMineLayer(g, assets.MineLayr, 0, 5, 5, 3)
	if layer.Storage != 3 {
		t.Fatalf("mine layer Storage = %d, want 3 (loaded by InitState)", layer.Storage)
	}

	// Lay one: a LandMine appears on the cell, store drops to 2.
	if !g.CanLayMine(layer) {
		t.Fatal("CanLayMine = false for a loaded layer on an empty cell")
	}
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}
	if layer.Storage != 2 {
		t.Errorf("Storage after laying = %d, want 2", layer.Storage)
	}
	if g.mineOnCell(assets.LandMine, 0, 5, 5) == nil {
		t.Error("no LandMine on the layer's cell after laying")
	}

	// Cannot lay a second mine on the same cell.
	if g.CanLayMine(layer) || g.LayMine(layer) {
		t.Error("laid a second mine on a cell that already has one")
	}

	// An enemy cannot enter the mined cell; the owner can.
	enemy := addMineLayer(g, assets.MineLayr, 1, 0, 0, 0) // a team 1 land unit
	if g.CanEnter(enemy, 5, 5) {
		t.Error("enemy unit can enter a cell with an enemy mine (should be blocked)")
	}

	// Remove it: store returns to 3, the mine is gone.
	if !g.CanRemoveMine(layer) {
		t.Fatal("CanRemoveMine = false with own mine on the cell and room in store")
	}
	if !g.RemoveMine(layer) {
		t.Fatal("RemoveMine returned false")
	}
	if layer.Storage != 3 {
		t.Errorf("Storage after removing = %d, want 3", layer.Storage)
	}
	if g.mineOnCell(assets.LandMine, 0, 5, 5) != nil {
		t.Error("mine still on the cell after removal")
	}

	// Nothing to remove now, and a full layer cannot remove.
	if g.CanRemoveMine(layer) {
		t.Error("CanRemoveMine = true with no mine present / store full")
	}
}

func TestMineDetonatesOnEnemy(t *testing.T) {
	g := newGame(t, 12, 12)
	layer := addMineLayer(g, assets.MineLayr, 0, 5, 5, 3)
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}
	if g.mineOnCell(assets.LandMine, 0, 5, 5) == nil {
		t.Fatal("no mine on the cell after laying")
	}

	// A friendly unit on the mine does NOT set it off.
	friend := addTank(g, 7, 1, 5, 5) // team 0
	g.detonateMines()
	if g.mineOnCell(assets.LandMine, 0, 5, 5) == nil {
		t.Error("a friendly unit detonated the mine")
	}
	g.Units.Remove(friend)

	// An enemy land unit driving onto the cell triggers the mine: it takes damage,
	// the mine is consumed.
	enemy := game.NewUnitInfo(assets.Tank, 1, 50, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Values.Set(game.AttribHits, 30)
	enemy.Values.Set(game.AttribArmor, 4)
	enemy.Hits = 30
	g.Units.Deploy(enemy, 5, 5)

	g.detonateMines()
	if g.mineOnCell(assets.LandMine, 0, 5, 5) != nil {
		t.Error("mine not consumed after an enemy stepped on it")
	}
	if enemy.Hits >= 30 {
		t.Errorf("enemy took no mine damage: hits=%d, want < 30", enemy.Hits)
	}
}

func TestMineDestroysWeakEnemy(t *testing.T) {
	g := newGame(t, 12, 12)
	layer := addMineLayer(g, assets.MineLayr, 0, 3, 3, 1)
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}
	weak := game.NewUnitInfo(assets.Scout, 1, 60, 0)
	weak.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	weak.Values = &game.UnitValues{}
	weak.Values.Set(game.AttribHits, 8) // less than the mine's attack minus armor
	weak.Hits = 8
	g.Units.Deploy(weak, 3, 3)

	g.detonateMines()
	if g.Units.ByID.Get(weak.ID) != nil {
		t.Error("a weak enemy survived a mine blast (should be destroyed)")
	}
}

// TestLaidMineCarriesItsRealAttribs pins the fix for a placeholder: a laid mine used to be
// hand built with an invented attack of 24, because the ATTRIBS table never reached the sim.
// It is minted through the unit factory now, like any other unit, so it hits for the real
// LANDMINE row (28 in the retail data) and the team's upgrades ride along.
func TestLaidMineCarriesItsRealAttribs(t *testing.T) {
	dir := os.Getenv("MAX_DATA_DIR")
	if dir == "" {
		t.Skip("MAX_DATA_DIR not set: needs the real ATTRIBS table")
	}

	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := lib.Read("ATTRIBS")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := assets.ParseAttribs(raw)
	if err != nil {
		t.Fatal(err)
	}

	const factoryID = 100

	g := newGame(t, 16, 16)
	g.MakeUnit = func(team int, ut assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(ut, team, factoryID, 0)
		u.Flags = flags
		u.Values = game.NewUnitValues(rows[ut])
		u.InitState()

		return u
	}

	layer := addMineLayer(g, assets.MineLayr, 0, 5, 5, 3)
	if !g.LayMine(layer) {
		t.Fatal("LayMine returned false")
	}

	mine := g.mineOnCell(assets.LandMine, 0, 5, 5)
	if mine == nil {
		t.Fatal("no mine on the layer's cell")
	}

	if mine.ID != factoryID {
		t.Errorf("the mine was hand built, not minted by the factory: id = %d", mine.ID)
	}

	if got, want := mine.Values.Get(game.AttribAttack), rows[assets.LandMine].Attack; got != want {
		t.Errorf("mine attack = %d, want the ATTRIBS value %d", got, want)
	}

	if mine.Hits <= 0 {
		t.Error("the laid mine has no hit points, so it could never be destroyed")
	}
}

func TestLayMineGating(t *testing.T) {
	g := newGame(t, 8, 8)

	// An empty layer cannot lay.
	empty := addMineLayer(g, assets.MineLayr, 0, 2, 2, 0)
	if g.CanLayMine(empty) || g.LayMine(empty) {
		t.Error("an out-of-mines layer laid a mine")
	}

	// A unit that is not a layer never lays/removes.
	tank := addTank(g, 9, 1, 4, 4)
	if g.CanLayMine(tank) || g.CanRemoveMine(tank) || IsMineLayer(tank) {
		t.Error("a non-layer unit reports mine-laying ability")
	}

	// A sea layer lays a SeaMine, not a LandMine.
	sea := addMineLayer(g, assets.SeaMnLyr, 0, 6, 6, 2)
	if !g.LayMine(sea) {
		t.Fatal("sea layer LayMine returned false")
	}
	if g.mineOnCell(assets.SeaMine, 0, 6, 6) == nil {
		t.Error("sea layer did not lay a SeaMine on its cell")
	}
	if g.mineOnCell(assets.LandMine, 0, 6, 6) != nil {
		t.Error("sea layer wrongly laid a LandMine")
	}
}

// addDetector deploys a mobile unit of the given type: a mine detector when the
// type is one of the four (surveyor, both layers, commando), else a plain unit.
func addDetector(g *Game, typ assets.UnitType, id, team, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(typ, team, id, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribScan, 8) // a wide radar sweep, which must not matter for mines
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestMineDetectorsMatchTheOriginal pins the detector roster: a surveyor, both mine
// layers and a commando uncover an enemy mine they stand next to; a scanner and an
// infantry never do, however far they can see.
func TestMineDetectorsMatchTheOriginal(t *testing.T) {
	detectors := []assets.UnitType{assets.Surveyor, assets.MineLayr, assets.SeaMnLyr, assets.Commando}
	blind := []assets.UnitType{assets.Scanner, assets.Infantry, assets.Tank, assets.Scout}

	for _, typ := range detectors {
		g := newGame(t, 16, 16)
		mine := g.newMine(assets.LandMine, 1)
		g.Units.Deploy(mine, 8, 8)
		addDetector(g, typ, 42, 0, 7, 8) // one cell away
		g.RefreshVisibility()

		if !mine.SpottedByTeam(0) {
			t.Errorf("%v stood beside the mine and did not uncover it", typ)
		}
	}

	for _, typ := range blind {
		g := newGame(t, 16, 16)
		mine := g.newMine(assets.LandMine, 1)
		g.Units.Deploy(mine, 8, 8)
		addDetector(g, typ, 42, 0, 8, 8) // standing right on it
		g.RefreshVisibility()

		if mine.SpottedByTeam(0) {
			t.Errorf("%v is not a mine detector but uncovered the mine", typ)
		}
	}
}

// TestMineDetectionIsStickyAndRangeOne: a detector two cells off finds nothing, one
// cell away uncovers the mine, and the mark survives the detector driving off. Its own
// team's mines were never hidden in the first place.
func TestMineDetectionIsStickyAndRangeOne(t *testing.T) {
	g := newGame(t, 16, 16)
	mine := g.newMine(assets.LandMine, 1)
	g.Units.Deploy(mine, 8, 8)

	surveyor := addDetector(g, assets.Surveyor, 42, 0, 10, 8) // two cells away
	g.RefreshVisibility()

	if mine.SpottedByTeam(0) {
		t.Fatal("a surveyor two cells away uncovered the mine; the reach is one cell")
	}

	g.Units.Move(surveyor, 9, 8) // now adjacent
	g.RefreshVisibility()

	if !mine.SpottedByTeam(0) {
		t.Fatal("a surveyor one cell away did not uncover the mine")
	}

	g.Units.Move(surveyor, 0, 0) // drive away again
	g.RefreshVisibility()

	if !mine.SpottedByTeam(0) {
		t.Error("the mine went hidden again when the surveyor left; the mark must stick")
	}

	if mine.SpottedByTeam(2) {
		t.Error("uncovering a mine for one team handed it to another")
	}
}
