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

// TestParkedBuildBench runs the -emulate parkedbuild bench end to end: both builders start parked on their
// still hidden buildings, loading the Constructor unfolds its Depot (H13) and killing the Engineer loses
// its Gun Turret with no slab left (H14).
func TestParkedBuildBench(t *testing.T) {
	s, g := newClickScreen(t)
	s.DeployParkedBuildBench()

	constructor, engineer := benchUnit(s, assets.Constrct), benchUnit(s, assets.Engineer)
	air := benchUnit(s, assets.AirTrans)
	if constructor == nil || engineer == nil || air == nil {
		t.Fatal("the bench did not deploy its Constructor, Engineer and Air Transport")
	}
	depot := g.BuildReadyInfo(constructor)
	turret := g.BuildReadyInfo(engineer)
	if depot == nil || turret == nil || !depot.Building.HiddenUntilExpand || !turret.Building.HiddenUntilExpand {
		t.Fatal("both builders must start parked on a hidden, finished building")
	}

	// H13: the Air Transport loads the Constructor; the Depot unfolds at once.
	if air.Values.Get(game.AttribStorage) == 0 {
		air.Values.Set(game.AttribStorage, 3) // headless placeholder stats carry no cargo room
	}
	if !g.LoadUnit(air, constructor) {
		t.Fatal("the Air Transport refused the parked Constructor")
	}
	if depot.Building.HiddenUntilExpand || g.IsBuildReady(constructor) || constructor.Building {
		t.Error("loading the Constructor did not release its Depot and end its build pose")
	}

	// H14: the Engineer dies parked; its Gun Turret goes with it, no slab left on the site.
	site := turret.Building
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
