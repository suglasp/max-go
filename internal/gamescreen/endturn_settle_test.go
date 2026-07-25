// endturn_settle_test.go : tests that End Turn waits for active movement to settle.
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

// TestEndTurnWaitsForActiveMove: pressing End Turn while a unit is still sliding must
// NOT commit at once - the commit runs EndTeamTurn -> ResolveMovement, which fast
// forwards the move budget and teleports the unit. Instead End Turn enters the settle
// state and leaves the move running so the idle loop can animate it out. This covers
// the "units jump to position on End Turn" report.
func TestEndTurnWaitsForActiveMove(t *testing.T) {
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
	s := &Screen{game: g, wmap: m, turnFlow: turnFlowState{handover: -1}}

	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 4)
	tank.Speed = 4
	g.Units.Deploy(tank, 2, 2)

	const destX, destY = 12, 2
	if !g.OrderMove(tank, destX, destY) {
		t.Fatal("OrderMove should start on open land")
	}
	g.Tick() // one frame in: the tank is sliding, not yet at the destination

	if g.ActiveOrders() == 0 {
		t.Fatal("the tank should be an active mover after issuing the move")
	}

	s.endTurn()

	if !s.turnFlow.endTurnSettling {
		t.Error("End Turn with a move in flight should enter the settle state, not commit immediately")
	}
	if g.ActiveOrders() == 0 {
		t.Error("End Turn must not drain the in-flight move (draining snaps the unit to its destination)")
	}
	if tank.GridX == destX && tank.GridY == destY {
		t.Error("the tank jumped straight to its destination on End Turn instead of animating there")
	}

	// Once the move settles (the idle loop would tick it out), ActiveOrders drops to 0,
	// which is the condition the Update loop uses to run the deferred commit.
	for i := 0; i < 100000 && g.ActiveOrders() > 0; i++ {
		g.Tick()
	}
	if g.ActiveOrders() != 0 {
		t.Fatal("the move never settled; the deferred commit would never fire")
	}
}
