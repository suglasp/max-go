// soldier_walk_test.go : tests soldier/infantry walking animation and movement on the game screen.
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
	"github.com/suglasp/max-go/internal/world"
)

// TestSoldierWalkAnimStopsWhenOutOfMoves verifies that
// a marching soldier that spends its last move point mid route kept cycling
// its walk animation in place. The walk advance must key on the unit ACTIVELY
// sliding (a live order in g.orders), not on the persistent u.Order value - a
// route paused out of moves keeps OrderMove but is queued in g.pending, so the
// soldier must stop marching and rest on its standing frame.
func TestSoldierWalkAnimStopsWhenOutOfMoves(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0
	s := &Screen{game: g, wmap: m}

	// An infantry with a small per turn budget, so a long route runs it dry mid path.
	inf := game.NewUnitInfo(assets.Infantry, 0, 1, 0)
	inf.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	inf.Values = &game.UnitValues{}
	inf.Values.Set(game.AttribSpeed, 2)
	inf.Speed = 2
	g.Units.Deploy(inf, 2, 2)

	if !g.OrderMove(inf, 12, 2) {
		t.Fatal("OrderMove should start on open land")
	}

	// Mid move (order live in g.orders) the soldier counts as an active mover -> walks.
	g.Tick()
	if !s.activeMovers()[inf] {
		t.Fatal("a soldier mid-move should be an active mover (cycling its walk frames)")
	}

	// Drive the route until the per turn budget is spent and it pauses.
	for i := 0; i < 100000 && g.ActiveOrders() > 0; i++ {
		g.Tick()
	}

	// The paused route still carries OrderMove (resumes next turn)...
	if inf.Order != game.OrderMove {
		t.Fatalf("a paused route keeps OrderMove, got %v", inf.Order)
	}
	// ...but it is NO LONGER an active mover, so it must stop walking.
	if s.activeMovers()[inf] {
		t.Error("a soldier out of moves (route only queued) must not be an active mover - it should stop marching")
	}

	// advanceWalkAnims snaps a stopped soldier back to the standing frame (phase 0).
	inf.WalkFrame = 2
	s.advanceWalkAnims()
	if inf.WalkFrame != 0 {
		t.Errorf("a stopped soldier should rest on the standing frame, got WalkFrame=%d", inf.WalkFrame)
	}
}
