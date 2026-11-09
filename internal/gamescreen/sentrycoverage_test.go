// sentrycoverage_test.go : covers the F2 "Sentry coverage" overlay: which units it
// rings and which it leaves alone.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
)

// greenPixels counts what the overlay drew: it is the only thing painting green into
// the bare buffer these tests hand it.
func greenPixels(buf *gfx.Buffer) int {
	n := 0

	for y := 0; y < buf.H; y++ {
		for x := 0; x < buf.W; x++ {
			if buf.At(x, y) == gfx.ColorGreen {
				n++
			}
		}
	}

	return n
}

// TestSentryCoverageRingsOwnArmedSentries is the heart of the overlay: a sentry of
// the player's carrying a weapon gets a ring, and nothing else does.
func TestSentryCoverageRingsOwnArmedSentries(t *testing.T) {
	cases := []struct {
		name  string
		team  int
		order game.Order
		rng   int
		want  bool
	}{
		{"own armed sentry", 0, game.OrderSentry, 6, true},
		// Not on guard: the ring claims "this ground is watched", so a unit merely
		// parked on the same spot must not claim it.
		{"own armed unit awaiting orders", 0, game.OrderAwait, 6, false},
		// An unarmed sentry guards nothing, because reaction fire is what the ring
		// is really predicting.
		{"own unarmed sentry", 0, game.OrderSentry, 0, false},
		// The overlay means "my coverage". Ringing an enemy sentry would leak where
		// the opponent is dug in.
		{"enemy armed sentry", 1, game.OrderSentry, 6, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, g := newTestScreen(t, 16)
			g.State.ActiveTeam = 0
			g.State.PlayerTeam = 0

			u := mkUnit(g, assets.GunTurrt, 0, 51, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
			u.Team = tc.team
			u.Order = tc.order
			u.Values.Set(game.AttribRange, tc.rng)

			s.showSentryCoverage = true
			buf := gfx.New(448, 448)
			s.drawSentryCoverage(buf)

			if got := greenPixels(buf) > 0; got != tc.want {
				t.Errorf("ringed = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSentryCoverageOffDrawsNothing checks the F2 row actually gates it: switched
// off, even the player's own armed sentry paints nothing.
func TestSentryCoverageOffDrawsNothing(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	u := mkUnit(g, assets.GunTurrt, 0, 51, 4, 4, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	u.Order = game.OrderSentry
	u.Values.Set(game.AttribRange, 6)

	buf := gfx.New(448, 448)
	s.drawSentryCoverage(buf) // showSentryCoverage defaults off

	if n := greenPixels(buf); n != 0 {
		t.Errorf("the overlay drew %d pixels while switched off", n)
	}
}
