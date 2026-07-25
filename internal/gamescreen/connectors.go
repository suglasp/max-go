// connectors.go : implements the connector network.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - connector adjacency and network wiring
package gamescreen

// connectors.go - building interconnection ("collectors"). Edge adjacent
// structures (and a Cnct4W built by an engineer bridging a gap) auto rig: a short
// connector tube is drawn between them, and they pool into one complex (the
// complex/network side is build.Network). Here we compute each structure's
// connection direction bitmask from its neighbours so DrawUnit can overlay the
// connector arm frames. C++: UnitsManager_UpdateConnectors + RenderWithConnectors.

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// Connector direction bits. The bit index equals the connector overlay frame
// offset the original draws (UnitInfo::RenderWithConnectors, unitinfo.cpp):
// a 1x1 structure uses only the four single point edges (bits 0-3); a 2x2 uses
// all eight (a left/right or top/bottom point per edge).
const (
	connNorthLeft  = 0
	connEastTop    = 1
	connSouthLeft  = 2
	connWestTop    = 3
	connNorthRight = 4
	connEastBottom = 5
	connSouthRight = 6
	connWestBottom = 7
)

// connectable reports whether a unit is a structure that joins the connector
// network: any team's stationary building / 1x1 building / connector, excluding
// ground cover (slab, road, rubble) and the stationary props (bridge, mines).
// C++: the BUILDING | CONNECTOR_UNIT | STANDALONE flag test.
func connectable(u *game.UnitInfo) bool {
	if u == nil || !u.Flags.Has(game.FlagStationary) || u.Flags.Has(game.FlagGroundCover) {
		return false
	}

	switch u.Type {
	case assets.Bridge, assets.LandMine, assets.SeaMine, assets.Waldo:
		return false
	}

	return true
}

// refreshConnectors recomputes every connectable structure's connection bitmask
// from the current unit grid. Cheap for base sized counts; called from the draw
// path so the auto rig tracks placement and removal without per site hooks.
func (s *Screen) refreshConnectors() {
	if s.game == nil {
		return
	}

	for _, u := range s.game.Units.Units() {
		if connectable(u) {
			u.Connectors = s.connectorBits(u)
		}
	}
}

// connectorBits returns u's connection direction bitmask: a bit is set for each
// edge point that abuts a same team connectable structure.
func (s *Screen) connectorBits(u *game.UnitInfo) uint8 {
	x, y := u.GridX, u.GridY

	// connProbe is one edge point to test: the cell probed and the mask bit it
	// sets when a same team connectable structure sits there.
	type connProbe struct {
		px, py, bit int
	}
	var probes []connProbe

	if u.Flags.Has(game.FlagBuilding) { // 2x2: two connection points per edge
		probes = []connProbe{
			{x, y - 1, connNorthLeft}, {x + 1, y - 1, connNorthRight},
			{x + 2, y, connEastTop}, {x + 2, y + 1, connEastBottom},
			{x, y + 2, connSouthLeft}, {x + 1, y + 2, connSouthRight},
			{x - 1, y, connWestTop}, {x - 1, y + 1, connWestBottom},
		}
	} else { // 1x1: one point per cardinal edge
		probes = []connProbe{
			{x, y - 1, connNorthLeft}, {x + 1, y, connEastTop},
			{x, y + 1, connSouthLeft}, {x - 1, y, connWestTop},
		}
	}

	var bits uint8

	for _, p := range probes {
		if s.connectedAt(u, p.px, p.py) {
			bits |= 1 << p.bit
		}
	}

	return bits
}

// connectedAt reports whether a same team connectable structure (other than u)
// occupies cell (x, y) - a building registers at all of its footprint cells, so
// an abutting 2x2 neighbour is found at the probed edge cell.
func (s *Screen) connectedAt(u *game.UnitInfo, x, y int) bool {
	for _, o := range s.game.Units.Cells.At(x, y) {
		if o != u && o.Team == u.Team && connectable(o) {
			return true
		}
	}

	return false
}
