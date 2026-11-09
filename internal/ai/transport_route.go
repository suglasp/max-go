// transport_route.go : plans where a transporter picks up a unit with no land route and where it lands it.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : taskmove.cpp - AttemptTransport, TranscribeTransportPath; paths_manager.cpp - BuildAccessMap
package ai

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// transportLegWeight multiplies the cost of a cell only the transporter can enter, so the
// mixed route keeps the passenger on its own terrain for as long as it can.
const transportLegWeight = 3

// ferryRoute is where a ferry starts and ends: the passenger waits at pickup, the last cell
// it can reach on its own, and goes ashore at landing, the first cell past the water.
type ferryRoute struct {
	pickup  game.Point
	landing game.Point
}

// chooseTransporter picks the transporter type for a passenger: none for a flier or a unit
// that can cross water itself, an Air Transport when the team fields one, else a Sea Transport.
// C++ ported : taskmove.cpp - TaskMove::AttemptTransport
func chooseTransporter(g *sim.Game, team int, passenger *game.UnitInfo) (assets.UnitType, bool) {
	// A flier never needs a ride.
	if passenger.Flags.Has(game.FlagMobileAirUnit) {
		return 0, false
	}

	// A sea unit or an amphibian crosses the water on its own.
	if access.LandTypeForUnit(passenger.Type, passenger.Flags)&world.SurfaceWater != 0 {
		return 0, false
	}

	// An Air Transport already on the map wins over the Sea Transport.
	for _, unit := range g.Units.Units() {
		if unit.Team == team && unit.Type == assets.AirTrans && unit.Hits > 0 {
			return assets.AirTrans, true
		}
	}

	return assets.SeaTrans, true
}

// transporterDesc describes a transporter type for a passability query, the way the
// factory deploys it.
func transporterDesc(transporter assets.UnitType) access.UnitDesc {
	flags := flagSeaUnit

	// The Air Transport flies; every other ferry here sails.
	if transporter == assets.AirTrans {
		flags = flagAirUnit
	}

	return access.UnitDesc{Type: transporter, Flags: flags, LandType: access.LandTypeForUnit(transporter, flags)}
}

// planFerry finds the mixed route from the passenger to dest, where a cell only the
// transporter can enter costs transportLegWeight times its own cost, and cuts it at the
// shore into a pickup and a landing cell. It reports false when there is no route, when
// the route never leaves the passenger's own terrain (no ferry needed), or when the route
// ends before it reaches ground the passenger can stand on again.
// C++ ported : taskmove.cpp - TaskMove::TranscribeTransportPath (with the transporter
// overlay of paths_manager.cpp - PathsManager::BuildAccessMap)
func planFerry(g *sim.Game, passenger *game.UnitInfo, dest game.Point, transporter assets.UnitType) (ferryRoute, bool) {
	passengerDesc := access.UnitDesc{Type: passenger.Type, Flags: passenger.Flags, LandType: access.LandTypeForUnit(passenger.Type, passenger.Flags)}
	passengerCost := paths.GridCost(g.Grid, passengerDesc, passenger.Team, access.NoModifiers)
	transporterCost := paths.GridCost(g.Grid, transporterDesc(transporter), passenger.Team, access.NoModifiers)

	// The passenger's own cells keep their cost; a transporter only cell costs extra.
	mixedCost := func(x, y int) int {
		if cost := passengerCost(x, y); cost > 0 {
			return cost
		}

		return transporterCost(x, y) * transportLegWeight
	}

	result, ok := paths.Find(mixedCost, g.W, g.H, passenger.Position(), dest, 0)
	if !ok {
		return ferryRoute{}, false
	}

	route := ferryRoute{}
	onBoard := false
	previous := passenger.Position()

	for _, step := range result.Steps {
		ownGround := passengerCost(step.To.X, step.To.Y) > 0

		// The first cell back on the passenger's own ground after the ride is the landing.
		if ownGround && onBoard {
			route.landing = step.To
			return route, true
		}

		// The first cell the passenger cannot enter starts the ride; it waits one cell before.
		if !ownGround && !onBoard {
			route.pickup = previous
			onBoard = true
		}

		previous = step.To
	}

	// Either the route never needed a ride, or it ended out on the water.
	return ferryRoute{}, false
}
