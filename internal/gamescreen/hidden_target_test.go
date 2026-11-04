// hidden_target_test.go : tests that an enemy the player cannot see stays hidden from cursors, orders and overlays.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// hiddenSubScreen builds a fogged screen: land in columns 0 to 7, water from column 8, a player Bomber at (bomberX,5)
// and an enemy Submarine submerged at (9,5) with no Corvette near, so the player cannot see it.
func hiddenSubScreen(t *testing.T, bomberX int) (*Screen, *sim.Game, *game.UnitInfo, *game.UnitInfo) {
	t.Helper()
	m := &world.Map{Surface: make([]uint8, 16*16)}
	m.W, m.H = 16, 16
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			m.Surface[y*m.W+x] = world.SurfaceLand
			if x >= 8 {
				m.Surface[y*m.W+x] = world.SurfaceWater
			}
		}
	}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.New(p, m)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	bomber := game.NewUnitInfo(assets.Bomber, 0, 1, 0)
	bomber.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	bomber.Values = &game.UnitValues{}
	for a, v := range map[game.Attrib]int{game.AttribAttack: 20, game.AttribRange: 2, game.AttribAmmo: 4,
		game.AttribRounds: 1, game.AttribSpeed: 20, game.AttribScan: 6, game.AttribHits: 30} {
		bomber.Values.Set(a, v)
	}
	bomber.InitState()
	bomber.Order = game.OrderAwait
	g.Units.Deploy(bomber, bomberX, 5)

	sub := game.NewUnitInfo(assets.Submarne, 1, 2, 0)
	sub.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	sub.Values = &game.UnitValues{}
	sub.Values.Set(game.AttribHits, 30)
	sub.InitState()
	sub.Order = game.OrderAwait
	g.Units.Deploy(sub, 9, 5)
	g.Vis.Rebuild(g.Units.Units())

	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}
	s.fogShow = true
	s.fogTeam = 0
	s.selected = bomber
	return s, g, bomber, sub
}

// addSonar puts a player Corvette next to the sub, so its sonar reveals it.
func addSonar(g *sim.Game) {
	c := game.NewUnitInfo(assets.Corvette, 0, 3, 0)
	c.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribScan, 4)
	g.Units.Deploy(c, 10, 6)
	g.Vis.Rebuild(g.Units.Units())
}

// TestCursorHidesUnseenEnemy checks that the attack cursors never light over a submerged sub the player cannot see,
// and that they do once a Corvette's sonar reveals it.
func TestCursorHidesUnseenEnemy(t *testing.T) {
	s, g, bomber, _ := hiddenSubScreen(t, 8) // in range of the sub
	if !g.CanAttack(bomber, 9, 5) {
		t.Fatal("setup: the Bomber cannot attack the sub cell at all")
	}

	if got := s.cellTarget(bomber, 9, 5, true); got.canAttack || got.canFarTarget {
		t.Errorf("hover over the hidden sub: canAttack %v canFarTarget %v, want both false", got.canAttack, got.canFarTarget)
	}
	s.orderInput.attackArmed = true
	if id, _ := s.cursorForArmedOrder(9, 5); id == cursors.Enemy {
		t.Error("the armed Attack pick lights the enemy pointer over the hidden sub")
	}
	s.orderInput.attackArmed = false

	far, _, _, _ := hiddenSubScreen(t, 4) // out of range: the far target cursor
	if got := far.cellTarget(far.selected, 9, 5, true); got.canFarTarget {
		t.Error("the far target cursor lights over the hidden sub")
	}

	addSonar(g)
	if got := s.cellTarget(bomber, 9, 5, true); !got.canAttack {
		t.Error("once the sonar reveals the sub, the attack cursor must light")
	}
}

// TestClickDoesNotTargetUnseenEnemy checks that a click on the hidden sub's cell neither fires nor starts a move to
// attack: the click falls through to an ordinary move.
func TestClickDoesNotTargetUnseenEnemy(t *testing.T) {
	s, g, bomber, sub := hiddenSubScreen(t, 4)
	if !g.CanFarTarget(bomber, 9, 5) {
		t.Fatal("setup: the sub is not a far target for the Bomber")
	}

	if s.clickCombat(9, 5) {
		t.Error("a click on the hidden sub became an attack or a move to attack")
	}
	if bomber.AttackTarget == sub {
		t.Error("the Bomber pursues a sub the player cannot see")
	}
}

// TestUnitDisplaysSkipUnseenEnemy checks that the Colors frame is not drawn over the hidden sub, while it is drawn
// over the player's own Bomber.
func TestUnitDisplaysSkipUnseenEnemy(t *testing.T) {
	s, _, _, _ := hiddenSubScreen(t, 4)
	s.showColors = true
	z := s.r.Zoom()
	view := gfx.New(16*z, 16*z)

	s.drawUnitDisplays(view)

	if px := view.At(9*z, 5*z); px != 0 {
		t.Errorf("the Colors frame is drawn on the hidden sub's cell (pixel %d)", px)
	}
	if px := view.At(4*z, 5*z); px == 0 {
		t.Error("control: the player's own Bomber lost its Colors frame")
	}
}
