// parkedbuildbench_test.go : testing unit for parkedbuildbench.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// benchUnit returns the bench's first unit of a type, or nil.
func benchUnit(s *Screen, t assets.UnitType) *game.UnitInfo {
	for _, u := range s.game.Units.Units() {
		if u.Type == t {
			return u
		}
	}
	return nil
}

// parkedOn returns the parked builder whose hidden building is of type t, with that building, or nils.
func parkedOn(s *Screen, t assets.UnitType) (*game.UnitInfo, *game.UnitInfo) {
	for _, b := range s.game.BuildReadyUnits(0) {
		if br := s.game.BuildReadyInfo(b); br != nil && br.Building != nil && br.Building.Type == t {
			return b, br.Building
		}
	}
	return nil, nil
}

// TestParkedBuildBench runs the -emulate parkedbuild bench end to end: four builders start parked on their still
// hidden buildings (H13, H14, H2), the idle builders (H15) and the running Habitat (H17) are there, loading the
// Depot's Constructor unfolds the Depot (H13) and killing the Gun Turret's Engineer loses the turret (H14).
func TestParkedBuildBench(t *testing.T) {
	s, g := newClickScreen(t)
	s.DeployParkedBuildBench()

	if n := len(g.BuildReadyUnits(0)); n != 4 {
		t.Fatalf("%d parked builders, want 4 (H13, H14 and the H2 pair)", n)
	}
	constructor, depot := parkedOn(s, assets.Depot)
	engineer, site := parkedOn(s, assets.GunTurrt)
	if _, plant := parkedOn(s, assets.LightPlt); plant == nil || !plant.HiddenUntilExpand {
		t.Error("the H2 Constructor is not parked on a hidden Light Vehicle Plant")
	}
	if _, antiAir := parkedOn(s, assets.AntiAir); antiAir == nil || !antiAir.HiddenUntilExpand {
		t.Error("the H2 Engineer is not parked on a hidden Anti Aircraft gun")
	}
	air := benchUnit(s, assets.AirTrans)
	if constructor == nil || engineer == nil || air == nil || !depot.HiddenUntilExpand || !site.HiddenUntilExpand {
		t.Fatal("the H13 / H14 builders must start parked on a hidden, finished building, with the Air Transport")
	}

	idle := 0
	for _, u := range g.Units.Units() {
		if (u.Type == assets.Constrct || u.Type == assets.Engineer) && !g.IsBuildReady(u) {
			idle++
		}
	}
	if idle != 2 {
		t.Errorf("%d idle builders for the H15 menus, want 2", idle)
	}
	if habitat := benchUnit(s, assets.Habitat); habitat == nil || habitat.Order != game.OrderPowerOn {
		t.Error("the H17 Habitat is missing or not running")
	}

	// H13: the Air Transport loads the Constructor; the Depot unfolds at once.
	if air.Values.Get(game.AttribStorage) == 0 {
		air.Values.Set(game.AttribStorage, 3) // headless placeholder stats carry no cargo room
	}
	if !g.LoadUnit(air, constructor) {
		t.Fatal("the Air Transport refused the parked Constructor")
	}
	if depot.HiddenUntilExpand || g.IsBuildReady(constructor) || constructor.Building {
		t.Error("loading the Constructor did not release its Depot and end its build pose")
	}

	// H14: the Engineer dies parked; its Gun Turret goes with it, no slab left on the site.
	siteX, siteY := site.GridX, site.GridY
	g.DestroyUnit(engineer)
	if g.Units.ByID.Get(site.ID) != nil {
		t.Error("the hidden Gun Turret survived its Engineer")
	}
	for _, u := range g.Units.Cells.At(siteX, siteY) {
		if u.Flags.Has(game.FlagGroundCover) || u == site {
			t.Errorf("%v left on the lost turret site", u.Type)
		}
	}
}
