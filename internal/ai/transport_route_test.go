// transport_route_test.go : testing unit for transport_route.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// ferryTestUnit deploys a unit with some hits, so the ferry code counts it as alive.
func ferryTestUnit(team, id int, unitType assets.UnitType, flags game.Flags, x, y int, mgr *units.Manager) *game.UnitInfo {
	unit := game.NewUnitInfo(unitType, team, id, 0)
	unit.Flags = flags
	values := &game.UnitValues{}
	values.Set(game.AttribHits, 20)
	values.Set(game.AttribSpeed, 10)
	unit.Values = values
	unit.InitState()
	unit.Hits = 20
	mgr.Deploy(unit, x, y)
	return unit
}

// straitGame is two land blocks with a three cell strait between them (x 4..6).
func straitGame() *sim.Game {
	rows := []string{
		"####~~~####",
		"####~~~####",
		"####~~~####",
	}
	return gameFromSurf(newPatternSurf(rows), 11, 3)
}

// TestPlanFerryCutsTheRouteAtTheShore checks the pickup is the last land cell before the
// strait and the landing the first land cell after it.
func TestPlanFerryCutsTheRouteAtTheShore(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)

	route, ok := planFerry(g, tank, game.Point{X: 9, Y: 1}, assets.SeaTrans)
	if !ok {
		t.Fatal("planFerry found no ferry across the strait")
	}

	if route.pickup != (game.Point{X: 3, Y: 1}) {
		t.Errorf("pickup = %v, want (3,1), the last land cell before the water", route.pickup)
	}

	if route.landing != (game.Point{X: 7, Y: 1}) {
		t.Errorf("landing = %v, want (7,1), the first land cell past the water", route.landing)
	}
}

// TestPlanFerryRefusesWhenNoRideIsNeededOrPossible checks the three refusals: a target on
// the same island, a target out on the water and a target no route reaches.
func TestPlanFerryRefusesWhenNoRideIsNeededOrPossible(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)

	cases := []struct {
		name string
		dest game.Point
	}{
		{"same island", game.Point{X: 2, Y: 2}},
		{"open water", game.Point{X: 5, Y: 1}},
		{"off the map", game.Point{X: 40, Y: 1}},
	}

	for _, tc := range cases {
		if route, ok := planFerry(g, tank, tc.dest, assets.SeaTrans); ok {
			t.Errorf("%s: planFerry = %+v, want no ferry", tc.name, route)
		}
	}
}

// TestPlanFerryWeighsTheWaterLeg checks the water leg costs three times its own cost: a
// short land detour beats a one cell hop, a longer detour loses to it.
func TestPlanFerryWeighsTheWaterLeg(t *testing.T) {
	// A one cell water hop (12) costs more than the two diagonal land steps around it (12 total
	// against 16 for the hop and the next cell), so the tank drives round and needs no ride.
	shortDetour := gameFromSurf(newPatternSurf([]string{
		"#~#",
		"###",
	}), 3, 2)
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 0, 0, shortDetour.Units)

	if route, ok := planFerry(shortDetour, tank, game.Point{X: 2, Y: 0}, assets.SeaTrans); ok {
		t.Errorf("short detour: planFerry = %+v, want no ferry (the land detour is cheaper)", route)
	}

	// The detour round a two cell deep inlet costs 20 against 16 for the hop: ferry.
	longDetour := gameFromSurf(newPatternSurf([]string{
		"#~#",
		"#~#",
		"###",
	}), 3, 3)
	tank = ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 0, 0, longDetour.Units)

	route, ok := planFerry(longDetour, tank, game.Point{X: 2, Y: 0}, assets.SeaTrans)
	if !ok {
		t.Fatal("long detour: planFerry found no ferry, want the hop across the inlet")
	}

	if route.pickup != (game.Point{X: 0, Y: 0}) || route.landing != (game.Point{X: 2, Y: 0}) {
		t.Errorf("long detour: route = %+v, want pickup (0,0) and landing (2,0)", route)
	}
}

// TestChooseTransporter checks who asks for a ride and which transporter it gets.
func TestChooseTransporter(t *testing.T) {
	g := straitGame()
	tank := ferryTestUnit(1, 1, assets.Tank, flagLandUnit, 1, 1, g.Units)

	// A tank of a team without an Air Transport goes by sea.
	if got, ok := chooseTransporter(g, 1, tank); !ok || got != assets.SeaTrans {
		t.Errorf("tank, no Air Transport: chooseTransporter = %v %v, want Sea Transport", got.Tag(), ok)
	}

	// Another team's Air Transport does not count.
	ferryTestUnit(2, 2, assets.AirTrans, flagAirUnit, 9, 0, g.Units)
	if got, _ := chooseTransporter(g, 1, tank); got != assets.SeaTrans {
		t.Errorf("tank, enemy Air Transport: chooseTransporter = %v, want Sea Transport", got.Tag())
	}

	// The team's own Air Transport wins.
	ferryTestUnit(1, 3, assets.AirTrans, flagAirUnit, 2, 0, g.Units)
	if got, ok := chooseTransporter(g, 1, tank); !ok || got != assets.AirTrans {
		t.Errorf("tank, own Air Transport: chooseTransporter = %v %v, want Air Transport", got.Tag(), ok)
	}

	// Amphibians, sea units and fliers never ask.
	riders := []struct {
		unitType assets.UnitType
		flags    game.Flags
	}{
		{assets.Scout, flagLandUnit},
		{assets.Constrct, flagLandUnit},
		{assets.Surveyor, flagLandUnit},
		{assets.Corvette, flagSeaUnit},
		{assets.Fighter, flagAirUnit},
	}

	for i, rider := range riders {
		unit := ferryTestUnit(1, 10+i, rider.unitType, rider.flags, 0, 2, units.NewManager())
		if got, ok := chooseTransporter(g, 1, unit); ok {
			t.Errorf("%s: chooseTransporter = %v, want no ride", rider.unitType.Tag(), got.Tag())
		}
	}
}
