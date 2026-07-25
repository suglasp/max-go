// transport_test.go : tests transport.go.
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

// TestTransportLoadUnload covers the core transport interaction: an adjacent
// friendly passenger boards (and leaves the map), a far one cannot, capacity is
// respected, and unloading returns the unit to a free adjacent cell. The carrier
// is the APC, so the passengers are INFANTRY - klei's receiver table gives the
// APC infantry and commandos only (a tank rides a Sea Transport or a Depot).
func TestTransportLoadUnload(t *testing.T) {
	g := newGame(t, 16, 16)

	transport := func(id, cap int) *game.UnitInfo {
		tr := game.NewUnitInfo(assets.ClnTrans, 0, id, 0)
		tr.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		tr.Values = &game.UnitValues{}
		tr.Values.Set(game.AttribStorage, cap)
		return tr
	}
	squad := func(id int) *game.UnitInfo {
		u := game.NewUnitInfo(assets.Infantry, 0, id, 0)
		u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
		return u
	}

	tr := transport(1, 3)
	g.Units.Deploy(tr, 5, 5)
	near := squad(2)
	g.Units.Deploy(near, 6, 5) // adjacent
	far := squad(3)
	g.Units.Deploy(far, 10, 10) // not adjacent

	if g.CanLoad(tr, far) {
		t.Error("a far unit must not be loadable")
	}
	if !g.CanLoad(tr, near) {
		t.Fatal("an adjacent friendly infantry squad should be loadable")
	}

	// Board the adjacent squad: aboard, off the map, out of the registry.
	if !g.LoadUnit(tr, near) {
		t.Fatal("LoadUnit failed for an adjacent squad")
	}
	if near.Parent != tr || len(tr.Carried) != 1 {
		t.Errorf("squad not aboard: parent=%p carried=%d", near.Parent, len(tr.Carried))
	}
	if g.Units.ByID.Get(near.ID) != nil {
		t.Error("a loaded unit must leave the registry")
	}
	if g.CanLoad(tr, near) {
		t.Error("a unit already aboard must not reload")
	}

	// Unload onto a free adjacent cell: back on the map, slot freed.
	x, y, ok := g.UnloadSpot(tr, near)
	if !ok {
		t.Fatal("no unload spot found around the transport")
	}
	if !g.UnloadUnit(tr, near, x, y) {
		t.Fatal("UnloadUnit failed for a valid spot")
	}
	if near.Parent != nil || len(tr.Carried) != 0 {
		t.Error("unit still aboard after unload")
	}
	if near.GridX != x || near.GridY != y || g.Units.ByID.Get(near.ID) != near {
		t.Errorf("unloaded unit not redeployed at (%d,%d)", x, y)
	}
	if !adjacent(tr.GridX, tr.GridY, x, y) {
		t.Errorf("unload spot (%d,%d) not adjacent to the transport", x, y)
	}

	// Capacity: a 1-slot transport refuses a second passenger.
	cap1 := transport(4, 1)
	g.Units.Deploy(cap1, 2, 2)
	a, b := squad(5), squad(6)
	g.Units.Deploy(a, 3, 2)
	g.Units.Deploy(b, 1, 2)
	if !g.LoadUnit(cap1, a) {
		t.Fatal("first load into a 1-slot transport should succeed")
	}
	if g.CanLoad(cap1, b) || g.LoadUnit(cap1, b) {
		t.Error("a full transport must refuse a further passenger")
	}
}
