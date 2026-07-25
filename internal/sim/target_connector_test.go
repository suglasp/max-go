// target_connector_test.go : tests target resolution when a connector shares a cell with another unit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestTopTargetPrefersStructureOverConnector verifies that topTarget and topAnyTarget select a real unit or building before a connector sharing the same cell.
func TestTopTargetPrefersStructureOverConnector(t *testing.T) {
	g := newGame(t, 24, 24)
	attacker := game.NewUnitInfo(assets.Bomber, 0, 1, 0) // team 0

	// The connector is deployed first, so a naive "first valid enemy" pick returns it.
	conn := game.NewUnitInfo(assets.Cnct4W, 1, 2, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable
	conn.Values = &game.UnitValues{}
	conn.Values.Set(game.AttribHits, 10)
	g.Units.Deploy(conn, 5, 5)

	bld := game.NewUnitInfo(assets.MiningSt, 1, 3, 0)
	bld.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	bld.Values = &game.UnitValues{}
	bld.Values.Set(game.AttribHits, 50)
	g.Units.Deploy(bld, 5, 5)

	if got := g.topTarget(attacker, 5, 5); got != bld {
		t.Errorf("topTarget returned the connector instead of the building (isConnector=%v)", got == conn)
	}
	if got := g.topAnyTarget(attacker, 5, 5); got != bld {
		t.Errorf("topAnyTarget returned the connector instead of the building (isConnector=%v)", got == conn)
	}

	// A lone connector is still a valid target when nothing else shares its cell.
	lone := game.NewUnitInfo(assets.Cnct4W, 1, 4, 0)
	lone.Flags = game.FlagStationary | game.FlagSelectable
	lone.Values = &game.UnitValues{}
	lone.Values.Set(game.AttribHits, 10)
	g.Units.Deploy(lone, 8, 8)
	if got := g.topTarget(attacker, 8, 8); got != lone {
		t.Error("topTarget on a lone connector should still return the connector")
	}
}
