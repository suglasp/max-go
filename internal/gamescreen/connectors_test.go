// connectors_test.go : tests connectors.go.
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
	"github.com/suglasp/max-go/internal/units"
)

/* // note: unused code in Go port
// replaced by the shared mkBuilding(t, team, id, -1, -1, flags, game.OrderPowerOn, 0)
func connBld(t assets.UnitType, team int, id uint16, large bool) *game.UnitInfo {
	flags := game.FlagStationary | game.FlagSelectable

	if large {
		flags |= game.FlagBuilding
	}

	return &game.UnitInfo{ID: id, Type: t, Team: team, Flags: flags, Order: game.OrderPowerOn}
}
*/

// TestConnectorBitsAutoRig checks that two abutting 2x2 buildings each set their
// connection bits toward the other (the auto rig), and that a 1x1 connector next
// to a building connects both ways.
func TestConnectorBitsAutoRig(t *testing.T) {
	s := &Screen{game: &sim.Game{Units: units.NewManager()}}

	// Two 2x2 buildings sharing the A east / B west edge: A(4,4), B(6,4).
	a := mkBuilding(assets.PowerStn, 0, 1, -1, -1, game.FlagStationary|game.FlagSelectable|game.FlagBuilding, game.OrderPowerOn, 0)
	b := mkBuilding(assets.LandPlt, 0, 2, -1, -1, game.FlagStationary|game.FlagSelectable|game.FlagBuilding, game.OrderPowerOn, 0)
	s.game.Units.Deploy(a, 4, 4)
	s.game.Units.Deploy(b, 6, 4)
	s.refreshConnectors()

	if a.Connectors&(1<<connEastTop) == 0 || a.Connectors&(1<<connEastBottom) == 0 {
		t.Errorf("A east bits = %08b, want E-top + E-bottom set", a.Connectors)
	}

	if b.Connectors&(1<<connWestTop) == 0 || b.Connectors&(1<<connWestBottom) == 0 {
		t.Errorf("B west bits = %08b, want W-top + W-bottom set", b.Connectors)
	}

	// No spurious north/south connections.
	if a.Connectors&(1<<connNorthLeft) != 0 || a.Connectors&(1<<connSouthLeft) != 0 {
		t.Errorf("A has a spurious N/S bit: %08b", a.Connectors)
	}

	// A 1x1 connector immediately east of B (B covers x=6..7; connector at x=8).
	conn := mkBuilding(assets.Cnct4W, 0, 3, -1, -1, game.FlagStationary|game.FlagSelectable, game.OrderPowerOn, 0)
	s.game.Units.Deploy(conn, 8, 4)
	s.refreshConnectors()

	if conn.Connectors&(1<<connWestTop) == 0 {
		t.Errorf("connector west bit not set: %08b", conn.Connectors)
	}

	if b.Connectors&(1<<connEastTop) == 0 {
		t.Errorf("B should now connect east to the connector: %08b", b.Connectors)
	}
}

// TestConnectableExcludesProps confirms ground cover and stationary props are not
// part of the connector network.
func TestConnectableExcludesProps(t *testing.T) {
	if connectable(&game.UnitInfo{Type: assets.Road, Flags: game.FlagStationary | game.FlagGroundCover}) {
		t.Error("a road is ground cover, not connectable")
	}

	if connectable(&game.UnitInfo{Type: assets.Bridge, Flags: game.FlagStationary}) {
		t.Error("a bridge is not connectable")
	}

	if !connectable(&game.UnitInfo{Type: assets.Cnct4W, Flags: game.FlagStationary}) {
		t.Error("a connector is connectable")
	}

	if !connectable(&game.UnitInfo{Type: assets.PowGen, Flags: game.FlagStationary}) {
		t.Error("a 1x1 building is connectable")
	}
}
