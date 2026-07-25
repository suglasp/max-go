// easteregg_save_test.go : testing unit for Waldo/Harvster gag props surviving a save/load round trip.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestEasterEggPropsRoundTrip checks that stationary Waldo and Harvster props use ordinary unit serialization and retain their type, team, position, and flags.
// Retail SAVE1.CAM stores its Harvster through the same stationary-unit representation.
func TestEasterEggPropsRoundTrip(t *testing.T) {
	g := newTestGame(t) // tank ID 1, gen ID 2 on an all land 16x16

	waldo := game.NewUnitInfo(assets.Waldo, game.AlienTeam, 20, 0)
	waldo.Flags = game.FlagStationary
	waldo.Values = &game.UnitValues{}
	waldo.InitState()
	g.Units.Deploy(waldo, 7, 9)

	harvster := game.NewUnitInfo(assets.Harvster, game.AlienTeam, 21, 0)
	harvster.Flags = game.FlagStationary
	harvster.Values = &game.UnitValues{}
	harvster.InitState()
	g.Units.Deploy(harvster, 11, 4)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	for _, tc := range []struct {
		id           uint16
		want         assets.UnitType
		gridX, gridY int
	}{
		{20, assets.Waldo, 7, 9},
		{21, assets.Harvster, 11, 4},
	} {
		u := g2.Units.ByID.Get(tc.id)
		if u == nil {
			t.Errorf("%s (ID %d) lost on reload", tc.want.Tag(), tc.id)
			continue
		}
		if u.Type != tc.want {
			t.Errorf("ID %d type = %s, want %s", tc.id, u.Type.Tag(), tc.want.Tag())
		}
		if u.Team != game.AlienTeam {
			t.Errorf("%s team = %d, want the neutral alien slot", tc.want.Tag(), u.Team)
		}
		if u.GridX != tc.gridX || u.GridY != tc.gridY {
			t.Errorf("%s pos = %d,%d, want %d,%d", tc.want.Tag(), u.GridX, u.GridY, tc.gridX, tc.gridY)
		}
		if !u.Flags.Has(game.FlagStationary) {
			t.Errorf("%s lost FlagStationary on reload (flags=%v)", tc.want.Tag(), u.Flags)
		}
	}
}
