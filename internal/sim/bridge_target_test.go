// bridge_target_test.go : tests a bridge not being a weapon target.
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

// TestBridgeIsNotAttackTarget verifies that an alien BRIDGE remains drivable terrain rather than an attack target.
// Bridges carry FlagStationary, so target resolution must exclude them separately from FlagGroundCover units.
func TestBridgeIsNotAttackTarget(t *testing.T) {
	g := newGame(t, 16, 16)

	attacker := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	attacker.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(attacker, 5, 5)

	bridge := game.NewUnitInfo(assets.Bridge, game.AlienTeam, 2, 0)
	bridge.Flags = game.FlagStationary
	g.Units.Deploy(bridge, 6, 5)
	if g.topTarget(attacker, 6, 5) != nil {
		t.Error("a bridge must not be an attack target (topTarget)")
	}
	if g.topAnyTarget(attacker, 6, 5) != nil {
		t.Error("a bridge must not be a force-attack target (topAnyTarget)")
	}

	// A real enemy unit on a like cell IS a target.
	enemy := game.NewUnitInfo(assets.Tank, game.AlienTeam, 3, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(enemy, 7, 5)
	if g.topTarget(attacker, 7, 5) != enemy {
		t.Error("an enemy tank should be a valid attack target")
	}
}
