// power.go : the M.A.X. AI switching its own buildings on.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// aiFuelReserve is the fuel a team must have banked before the AI switches on the
// buildings it can live without. The mines come first; gold, science and colonisation
// wait until there is fuel to burn on them.
// C++ ported : aiplayer.cpp - AiPlayer::PlanMinimalistMining (fuel_reserves >= 10)
const aiFuelReserve = 10

// powerOnBuildings activates the team's newly completed buildings. Idle plants draw no power, so activating one costs nothing until it has work.
//
// A building the team switched off deliberately (Idle, Disabled) is left alone, and a
// complex that cannot carry the draw refuses - the shortfall then shows up as power
// demand and the building manager raises a generator for it.
//
// C++ ported : aiplayer.cpp - the per turn sweep that re issues ORDER_POWER_ON over the
// team's stationary units (extended to the plants, which the original runs under
// ORDER_BUILD rather than a power flag).
func powerOnBuildings(g *sim.Game, team int) {
	if g == nil {
		return
	}

	fuel := 0

	for _, u := range g.Units.Units() {
		if u.Team == team && u.Type == assets.FDump {
			fuel += u.Storage
		}
	}

	for _, u := range g.Units.Units() {
		if u.Team != team || !u.Flags.Has(game.FlagStationary) || u.Building {
			continue
		}

		// Only activate buildings construction left off. Idle and Disabled buildings keep their existing state.
		if u.Order != game.OrderPowerOff || !aiWantsPowered(u.Type, fuel) {
			continue
		}

		if !g.PowerOnConsumer(u) {
			dbg.Logf(dbg.AI, "team %d cannot power %s at (%d,%d): complex will not carry the draw", team, u.Type.Tag(), u.GridX, u.GridY)
		}
	}
}

// aiWantsPowered reports whether the AI runs a building type. The mines run always;
// the plants run always too because an idle one draws nothing (economy.PowerOnDraw)
// and a dark one can never build; gold, research and the Eco Spheres wait for a fuel
// reserve, so an early base spends its first fuel on mining and armour instead.
//
// C++ ported : aiplayer.cpp - MININGST unconditionally, then COMMTWR / GREENHSE /
// RESEARCH once fuel_reserves >= 10.
func aiWantsPowered(t assets.UnitType, fuelReserve int) bool {
	switch t {
	case assets.MiningSt:
		return true
	case assets.CommTwr, assets.Greenhse, assets.Research:
		return fuelReserve >= aiFuelReserve
	}

	return isFactory(t) // the plants the team actually produces from (buildunits.go)
}
