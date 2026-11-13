// unitid_test.go : testing unit for the unit counter a load restores and the in use guard of newUnit
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// boardPassenger puts a unit with the given id aboard carrier, off the registry like a real load.
func boardPassenger(carrier *game.UnitInfo, id int) *game.UnitInfo {
	p := game.NewUnitInfo(assets.Tank, carrier.Team, id, 0)
	p.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	p.Values = &game.UnitValues{}
	p.Parent = carrier
	carrier.Carried = append(carrier.Carried, p)

	return p
}

// saveUnitBoard builds a saver with units 1..60 on the board and unit 75 aboard depot 60,
// sets its counter and saves it to slot 1 of dir.
func saveUnitBoard(t *testing.T, dir string, counter int) {
	t.Helper()
	saver, g := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")

	for id := 1; id < 60; id++ {
		mkUnit(g, assets.Tank, 0, id, id%16, id/16, game.FlagMobileLandUnit|game.FlagSelectable)
	}

	depot := mkUnit(g, assets.Depot, 0, 60, 12, 12, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	boardPassenger(depot, 75)
	saver.spawnSeq = counter

	if err := saver.saveToSlot(1, "unit ids", false); err != nil {
		t.Fatalf("save: %v", err)
	}
}

// loadUnitBoard loads slot 1 of dir into a fresh screen whose counter starts at 0.
func loadUnitBoard(t *testing.T, dir string) (*Screen, *sim.Game) {
	t.Helper()
	loader, _ := newTestScreen(t, 16)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")

	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}

	return loader, loader.game
}

// TestLoadKeepsNewUnitIDsUnique checks that units made after a load never take the ID of a restored unit, the carried one included.
func TestLoadKeepsNewUnitIDsUnique(t *testing.T) {
	dir := t.TempDir()
	saveUnitBoard(t, dir, 60)
	loader, g := loadUnitBoard(t, dir)

	if loader.spawnSeq < 75 {
		t.Fatalf("unit counter after the load = %d, want at least 75 (the highest restored ID)", loader.spawnSeq)
	}

	for i := 0; i < 80; i++ {
		u := loader.newUnit(1, assets.Tank, game.FlagMobileLandUnit)

		if holder := loader.unitIDInUse(u.ID); holder != nil {
			t.Fatalf("new unit %d got id %d, already used by a %v of team %d", i+1, u.ID, holder.Type, holder.Team)
		}

		g.Units.Deploy(u, i%16, 15) // a placed unit, so the next call must step past it
	}
}

// TestLoadRestoresSavedUnitCounter checks that a v25 save brings its counter back, also when it is above every living unit's ID.
func TestLoadRestoresSavedUnitCounter(t *testing.T) {
	dir := t.TempDir()
	saveUnitBoard(t, dir, 90) // units 76..90 were made and have since died
	loader, _ := loadUnitBoard(t, dir)

	if loader.spawnSeq != 90 {
		t.Errorf("unit counter after the load = %d, want the saved 90", loader.spawnSeq)
	}
}

// TestLoadOldSaveDerivesUnitCounter checks that a save without a counter (pre v25) derives it from the highest restored ID.
func TestLoadOldSaveDerivesUnitCounter(t *testing.T) {
	dir := t.TempDir()
	saveUnitBoard(t, dir, 0) // an older save decodes SpawnSeq as 0
	loader, _ := loadUnitBoard(t, dir)

	if loader.spawnSeq != 75 {
		t.Errorf("unit counter after the load = %d, want 75 (the carried unit's ID)", loader.spawnSeq)
	}
}

// TestNewUnitSkipsIDInUse checks that newUnit steps past an ID held by a unit on the board or aboard a carrier.
func TestNewUnitSkipsIDInUse(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.spawnSeq = 4
	mkUnit(g, assets.Tank, 0, 5, 1, 1, game.FlagMobileLandUnit|game.FlagSelectable)
	depot := mkUnit(g, assets.Depot, 0, 7, 4, 4, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	boardPassenger(depot, 6)

	if u := s.newUnit(1, assets.Tank, game.FlagMobileLandUnit); u.ID != 8 {
		t.Errorf("newUnit id = %d, want 8 (5 on the board, 6 aboard, 7 the depot)", u.ID)
	}

	if u := s.newUnit(1, assets.Tank, game.FlagMobileLandUnit); u.ID != 9 {
		t.Errorf("next newUnit id = %d, want 9", u.ID)
	}
}
