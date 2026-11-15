// airarrows_test.go : tests the air route markers of patharrows.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
)

// TestAirRouteDrawsMarkersAlongTheLine: a plane's route is one synthetic step (its destination),
// but the arrows run along the straight line, one marker per movement point, with the blue
// boundary every speed points.
func TestAirRouteDrawsMarkersAlongTheLine(t *testing.T) {
	s, g := newTestScreen(t, 16)
	u := mkUnit(g, assets.Bomber, 0, 1, 1, 1, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable)
	u.Team = g.State.PlayerTeam
	u.Values.Set(game.AttribSpeed, 4)
	u.Speed = 4

	steps := []paths.Step{{Dir: 2, To: game.Point{X: 11, Y: 1}}}
	cells := map[game.Point]routeCell{}
	s.collectRoute(cells, u, steps)

	if len(cells) != 10 {
		t.Fatalf("markers = %d, want 10 (one per cell from x=2 to x=11)", len(cells))
	}
	var blue []int
	for x := 2; x <= 11; x++ {
		c, ok := cells[game.Point{X: x, Y: 1}]
		if !ok {
			t.Fatalf("no marker on cell (%d,1)", x)
		}
		if c.color == 0x03 {
			blue = append(blue, x)
		}
	}
	if !equalInts(blue, []int{5, 9}) {
		t.Errorf("blue boundary cells = %v, want [5 9] (every 4 points)", blue)
	}
}
