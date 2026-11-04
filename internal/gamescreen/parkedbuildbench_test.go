// parkedbuildbench_test.go : testing unit for parkedbuildbench.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
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
// hidden buildings (H13, H14, H2), the idle builders (H15) and the running Habitat (H17) are there, the Air
// Transport's Load only selects the parked Constructor (H13), and killing the Gun Turret's Engineer loses the
// turret (H14).
func TestParkedBuildBench(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 112*112)}
	m.W, m.H = 112, 112
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}
	s.DeployParkedBuildBench()

	if n := len(s.bench.stops); n != 6 {
		t.Fatalf("%d stops, want 6 (H13, H14, H2, H15, H17, N4)", n)
	}
	checkBenchStopsApart(t, s.bench.stops)

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
	tank := benchUnit(s, assets.Tank)
	if constructor == nil || engineer == nil || air == nil || !depot.HiddenUntilExpand || !site.HiddenUntilExpand {
		t.Fatal("the H13 / H14 builders must start parked on a hidden, finished building, with the Air Transport")
	}

	idle := 0
	for _, u := range g.Units.Units() {
		if u.Team == 0 && (u.Type == assets.Constrct || u.Type == assets.Engineer) && !g.IsBuildReady(u) {
			idle++
		}
	}
	if idle != 2 {
		t.Errorf("%d idle builders for the H15 menus, want 2", idle)
	}
	if habitat := benchUnit(s, assets.Habitat); habitat == nil || habitat.Order != game.OrderPowerOn {
		t.Error("the H17 Habitat is missing or not running")
	}

	// N4: two enemy Constructors, one posed on the lower right corner of a 2x2 site, and a player Rocket
	// Launcher out of range of both.
	var launcher *game.UnitInfo
	var targets []*game.UnitInfo
	for _, u := range g.Units.Units() {
		switch {
		case u.Type == assets.RocktLch && u.Team == 0:
			launcher = u
		case u.Type == assets.Constrct && u.Team == 1:
			targets = append(targets, u)
		}
	}
	if launcher == nil || len(targets) != 2 {
		t.Fatalf("N4: launcher %v, %d enemy Constructors, want a launcher and 2", launcher != nil, len(targets))
	}
	posed := 0
	for _, c := range targets {
		dx, dy := c.GridX-launcher.GridX, c.GridY-launcher.GridY
		if r := launcher.Values.Get(game.AttribRange); dx*dx+dy*dy <= r*r {
			t.Errorf("N4: the launcher starts in range of the Constructor at (%d,%d)", c.GridX, c.GridY)
		}
		if c.Building {
			posed++
			if c.BuildFootprint != 2 || c.GridX != c.BuildSiteX+1 || c.GridY != c.BuildSiteY+1 {
				t.Errorf("N4: working Constructor at (%d,%d) site (%d,%d) n=%d, want the lower right corner of a 2x2",
					c.GridX, c.GridY, c.BuildSiteX, c.BuildSiteY, c.BuildFootprint)
			}
		}
	}
	if posed != 1 {
		t.Errorf("N4: %d posed enemy Constructors, want 1", posed)
	}
	if launcher.Shots != 4 || launcher.Ammo != 40 {
		t.Errorf("N4: launcher shots %d ammo %d, want the upgraded 4 and 40", launcher.Shots, launcher.Ammo)
	}
	// The unarmed enemy team stays in play: an End Turn does not win the bench. Headless placeholder stats give
	// every unit ammo, so empty the Constructors as the real ATTRIBS do.
	for _, c := range targets {
		c.Ammo = 0
	}
	g.UpdateEliminations()
	if out := g.Outcome(); out != winloss.Continue {
		t.Errorf("N4: outcome %v with both enemy Constructors alive, want the game to go on", out)
	}

	// H13: the Air Transport's Load refuses the parked Constructor; a click on it only selects it.
	if air.Values.Get(game.AttribStorage) == 0 {
		air.Values.Set(game.AttribStorage, 3) // headless placeholder stats carry no cargo room
	}
	air.Order = game.OrderAwait // a receiving carrier is not idle
	if g.CanContain(air, constructor) || g.LoadUnit(air, constructor) {
		t.Fatal("the Air Transport took the parked Constructor")
	}
	s.effects.loadInto = air // the popup Load armed
	if id, _ := s.cursorForArmedOrder(constructor.GridX, constructor.GridY); id != cursors.Friend {
		t.Errorf("Load pointer over the parked Constructor = %v, want the select pointer FRND_PTR", id)
	}
	if id, _ := s.cursorForArmedOrder(tank.GridX, tank.GridY); id != cursors.Load {
		t.Errorf("Load pointer over the idle Tank = %v, want the load pointer", id)
	}
	if !s.tryTransportTarget(constructor.GridX, constructor.GridY) || s.selected != constructor {
		t.Errorf("a click on the parked Constructor did not just select it (selected %v)", s.selected)
	}
	if !depot.HiddenUntilExpand || !g.IsBuildReady(constructor) || len(air.Carried) != 0 {
		t.Error("the refused load changed the parked Constructor or its Depot")
	}

	// H14: the Tank's forced shot at the cell hits the Engineer, never the hidden Gun Turret; one shot kills
	// it, the turret and its slab go with it, and the Engineer leaves rubble where it stood.
	siteX, siteY := site.GridX, site.GridY
	if g.CanForceAttack(tank, siteX, siteY) == false {
		t.Fatal("the Tank cannot force fire at the parked Engineer")
	}
	res, ok := g.ForceAttack(tank, siteX, siteY)
	if !ok || !res.Destroyed || g.Units.ByID.Get(engineer.ID) != nil {
		t.Fatalf("the forced shot did not kill the Engineer (ok %v, destroyed %v)", ok, res.Destroyed)
	}
	if g.Units.ByID.Get(site.ID) != nil {
		t.Error("the hidden Gun Turret survived its Engineer")
	}
	for _, u := range g.Units.Cells.At(siteX, siteY) {
		if u.Flags.Has(game.FlagGroundCover) || u == site {
			t.Errorf("%v left on the lost turret site", u.Type)
		}
	}
	if _, rubble := g.RubbleAt(siteX, siteY); !rubble {
		t.Error("no rubble where the Engineer died")
	}
	if kills := g.DrainCasualties(); len(kills) != 1 || kills[0].Victim != engineer {
		t.Errorf("%d casualties, want one explosion: the Engineer", len(kills))
	}
}

// TestParkedBuildBenchSnow places the five scenarios on the bench's real map, SNOW_1: each on its own land
// block at its own compass anchor, far enough apart to never share the screen.
func TestParkedBuildBenchSnow(t *testing.T) {
	s := benchScreen(t, "SNOW_1.WRL")
	s.DeployParkedBuildBench()

	if n := len(s.bench.stops); n != 6 {
		t.Fatalf("%d stops on SNOW_1, want 6", n)
	}
	checkBenchStopsApart(t, s.bench.stops)
	if n := len(s.game.BuildReadyUnits(0)); n != 4 {
		t.Errorf("%d parked builders on SNOW_1, want 4", n)
	}
	for _, stop := range s.bench.stops {
		t.Logf("%s %s at %v", stop.Compass, stop.Title, stop.Cell)
	}
}
