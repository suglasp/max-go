// easteregg_target_test.go : tests the Waldo / Harvster gag props not being weapon targets.
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

// TestEasterEggPropsAreNotTargets: Waldo and Harvster are inert 1x1 gag props, never
// weapon targets (no red attack cursor, no shots), just like a bridge.
func TestEasterEggPropsAreNotTargets(t *testing.T) {
	g := newGame(t, 16, 16)

	attacker := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	attacker.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(attacker, 5, 5)

	for i, et := range []assets.UnitType{assets.Waldo, assets.Harvster} {
		prop := game.NewUnitInfo(et, game.AlienTeam, 10+i, 0)
		prop.Flags = game.FlagStationary
		g.Units.Deploy(prop, 6, 5)
		if g.topTarget(attacker, 6, 5) != nil {
			t.Errorf("%s must not be an attack target (topTarget)", et.Tag())
		}
		if g.topAnyTarget(attacker, 6, 5) != nil {
			t.Errorf("%s must not be a force-attack target (topAnyTarget)", et.Tag())
		}
		g.Units.Remove(prop)
	}
}
