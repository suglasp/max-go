// connector_zorder_test.go : tests connector draw z order (above ground, below aircraft).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestConnectorDrawsOverUnitUnderAir is the z order verified against vanilla:
// a connector ("collector") draws OVER a ground/sea/bipedal unit on its cell (units
// drive UNDER it), but UNDER an aircraft (a plane flies OVER it). C++: unitinfogroup.cpp
// RenderLists draws GroundCover < MobileLandSea < Stationary (connector) < MobileAir.
func TestConnectorDrawsOverUnitUnderAir(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	conn := game.NewUnitInfo(assets.Cnct4W, 0, 1, 0)
	conn.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(conn, 4, 4)

	// A ground unit on the connector's cell drives UNDER it (drawn first / lower index).
	tank := game.NewUnitInfo(assets.Tank, 0, 2, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	mgr.Deploy(tank, 4, 4)

	order := s.unitDrawOrder()
	if ti, ci := drawIndexOf(order, tank), drawIndexOf(order, conn); ti > ci {
		t.Errorf("a ground unit (index %d) must draw before the connector (%d) - it drives under it", ti, ci)
	}

	// A plane on the connector's cell flies OVER it (drawn after / higher index).
	plane := game.NewUnitInfo(assets.Fighter, 0, 3, 0)
	plane.Flags = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	mgr.Deploy(plane, 4, 4)

	order = s.unitDrawOrder()
	if ci, pi := drawIndexOf(order, conn), drawIndexOf(order, plane); ci > pi {
		t.Errorf("the connector (index %d) must draw before the plane (%d) - the plane flies over it", ci, pi)
	}
}
