// bridge_select_test.go : tests a bridge being terrain that cannot be selected or attacked.
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

// TestBridgeNotSelectableTarget verifies that a BRIDGE
// owned by the alien team showed a red attack cursor and absorbed shots (no damage). A bridge is drivable
// terrain, not a selectable/attackable unit - so selectableOccupant (the shared cursor +
// click rule) must reject it, letting a unit drive across instead.
func TestBridgeNotSelectableTarget(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.fogTeam = 0

	bridge := mkUnit(g, assets.Bridge, 0, 1, 5, 5, game.FlagStationary)
	if s.selectableOccupant(bridge) {
		t.Error("a bridge must not be a selectable/attackable target (it is drivable terrain)")
	}

	// A real structure remains a valid target.
	turret := mkUnit(g, assets.GunTurrt, 0, 2, 6, 6, game.FlagStationary|game.FlagSelectable)
	if !s.selectableOccupant(turret) {
		t.Error("a turret should remain a selectable target")
	}
}
