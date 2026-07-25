// connector_replace_test.go : tests a building replacing a connector it is built on.
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

// TestPlaceBuildingRemovesConnectorUnderneath verifies that PlaceBuilding consumes friendly connectors in its footprint and joins their complex.
func TestPlaceBuildingRemovesConnectorUnderneath(t *testing.T) {
	g := newGame(t, 16, 16)

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 1, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable
	conn.Values = &game.UnitValues{}
	g.PlaceBuilding(conn, 5, 5)

	turret := game.NewUnitInfo(assets.GunTurrt, 0, 2, 0)
	turret.Flags = game.FlagStationary | game.FlagSelectable
	turret.Values = &game.UnitValues{}
	g.PlaceBuilding(turret, 5, 5) // built on the connector's cell

	for _, u := range g.Units.Units() {
		if u.Type == assets.Cnct4W {
			t.Error("connector was not removed when a building was placed on its cell")
		}
	}
	if g.ComplexOf(turret) == nil {
		t.Error("the turret did not join a complex after replacing the connector")
	}
}
