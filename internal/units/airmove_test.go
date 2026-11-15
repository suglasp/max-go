// airmove_test.go : testing unit for airmove.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// newPlane builds a placed bomber with a known speed budget. A flying plane carries the hovering flag.
func newPlane(t *testing.T, mgr *Manager, speed, gx, gy int, angle uint8, flying bool) *game.UnitInfo {
	t.Helper()
	u := game.NewUnitInfo(assets.Bomber, 0, 1, angle)
	u.Flags = game.FlagMobileAirUnit | game.FlagSelectable
	if flying {
		u.Flags |= game.FlagHovering
	}
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, speed)
	u.Speed = speed
	if mgr != nil {
		mgr.Deploy(u, gx, gy)
	} else {
		u.SetPosition(gx, gy)
	}
	return u
}

// TestAirTickTotals locks the tick count of a straight flight: 4 ticks per cell (2 fast) along
// the line, and the last stride halves, which adds 4 ticks (5 fast) over a stride of 16 px (32 fast). A diagonal flies the
// Euclidean length, not the grid steps, and the plane is on its end cell centre afterwards.
func TestAirTickTotals(t *testing.T) {
	cases := []struct {
		name         string
		fast         bool
		destX, destY int
		angle        uint8
		want         int
	}{
		{"east 10", false, 11, 1, 2, 4*10 + 4},
		{"east 4", false, 5, 1, 2, 4*4 + 4},
		{"east 10 fast", true, 11, 1, 2, 2*10 + 5},
		{"diagonal 7 (Euclid 9.9 cells, 10 points, the stride 11.2 px halves 3 times)", false, 8, 8, 3, 4*10 + 3},
	}
	for _, c := range cases {
		mgr := NewManager()
		u := newPlane(t, mgr, 200, 1, 1, c.angle, true)
		m := NewAirMoveOrder(mgr, u, c.destX, c.destY, c.fast)
		if m == nil {
			t.Fatalf("%s: no order", c.name)
		}
		status, ticks := run(t, m)
		if status != MoveDone {
			t.Fatalf("%s: status = %v, want MoveDone", c.name, status)
		}
		if ticks != c.want {
			t.Errorf("%s: ticks = %d, want %d", c.name, ticks, c.want)
		}
		if u.GridX != c.destX || u.GridY != c.destY {
			t.Errorf("%s: ended at (%d,%d), want (%d,%d)", c.name, u.GridX, u.GridY, c.destX, c.destY)
		}
		if u.X != c.destX*tileSize+tileSize/2 || u.Y != c.destY*tileSize+tileSize/2 {
			t.Errorf("%s: pixel (%d,%d) not snapped to the cell centre", c.name, u.X, u.Y)
		}
		if got := mgr.Cells.At(c.destX, c.destY); len(got) != 1 || got[0] != u {
			t.Errorf("%s: plane not in the destination cell hash", c.name)
		}
		if got := mgr.Cells.At(1, 1); len(got) != 0 {
			t.Errorf("%s: plane still in the start cell hash", c.name)
		}
		if u.Velocity != 0 {
			t.Errorf("%s: velocity = %d after the flight, want 0", c.name, u.Velocity)
		}
	}
}

// TestAirFlightSpendsOnePointPerPeriod: a 10 cell flight east costs 10 speed points, one every 4
// ticks (every 2 ticks fast), the same total as the ground route over Clear cells.
func TestAirFlightSpendsOnePointPerPeriod(t *testing.T) {
	for _, fast := range []bool{false, true} {
		period := 4
		if fast {
			period = 2
		}
		u := newPlane(t, nil, 50, 1, 1, 2, true)
		m := NewAirMoveOrder(nil, u, 11, 1, fast)
		firstPointTick := 0
		for tick := 1; tick < 1000; tick++ {
			before := u.Speed
			status := m.Tick()
			if firstPointTick == 0 && u.Speed < before {
				firstPointTick = tick
			}
			if status == MoveDone {
				break
			}
		}
		if spent := 50 - u.Speed; spent != 10 {
			t.Errorf("fast=%v: spent %d speed points over 10 cells, want 10", fast, spent)
		}
		if firstPointTick != period {
			t.Errorf("fast=%v: first speed point spent on tick %d, want %d", fast, firstPointTick, period)
		}
	}
}

// TestAirTakeOffTakesFiveTicks: a landed plane climbs for 5 ticks (hovering on the fifth) before it
// flies, and the climb costs no movement points.
func TestAirTakeOffTakesFiveTicks(t *testing.T) {
	mgr := NewManager()
	u := newPlane(t, mgr, 20, 1, 1, 2, false)
	m := NewAirMoveOrder(mgr, u, 6, 1, false)
	startX := u.X
	for i := 1; i <= 5; i++ {
		if s := m.Tick(); s != MoveTurning {
			t.Fatalf("take off tick %d: status = %v, want MoveTurning", i, s)
		}
		if hover := u.Flags.Has(game.FlagHovering); hover != (i == 5) {
			t.Errorf("take off tick %d: hovering = %v, want %v", i, hover, i == 5)
		}
		if u.X != startX || u.Speed != 20 {
			t.Errorf("take off tick %d: the plane moved or paid speed (x %d, speed %d)", i, u.X, u.Speed)
		}
	}
	if s := m.Tick(); s != MoveDriving || u.X == startX {
		t.Errorf("tick 6: status = %v, x = %d, want the flight to start", s, u.X)
	}
}

// TestAirFlightCutToBudget: a plane with a budget of 3 points flying 10 cells covers 3, spends all
// its points, stops blocked on a centre and keeps the destination as its remaining route.
func TestAirFlightCutToBudget(t *testing.T) {
	mgr := NewManager()
	u := newPlane(t, mgr, 3, 1, 1, 2, true)
	m := NewAirMoveOrder(mgr, u, 11, 1, false)
	status, _ := run(t, m)
	if status != MoveBlocked {
		t.Fatalf("status = %v, want MoveBlocked", status)
	}
	if u.GridX != 4 || u.GridY != 1 || u.X != 4*tileSize+tileSize/2 {
		t.Errorf("stopped at cell (%d,%d) pixel x %d, want cell (4,1) on its centre", u.GridX, u.GridY, u.X)
	}
	if u.Speed != 0 {
		t.Errorf("speed = %d, want 0", u.Speed)
	}
	if rest := m.RemainingSteps(); len(rest) != 1 || rest[0].To.X != 11 || rest[0].To.Y != 1 {
		t.Errorf("remaining route = %v, want the destination (11,1)", rest)
	}
	if !m.NextCellPassable() {
		t.Error("a plane's route is always passable, so the cut flight can be queued for next turn")
	}
}

// TestAirFlightNoBudgetStopsAtOnce: a plane with no movement left does not move, it reports blocked.
func TestAirFlightNoBudgetStopsAtOnce(t *testing.T) {
	u := newPlane(t, nil, 0, 1, 1, 2, true)
	m := NewAirMoveOrder(nil, u, 6, 1, false)
	if s := m.Tick(); s != MoveBlocked {
		t.Errorf("status = %v, want MoveBlocked", s)
	}
	if u.GridX != 1 {
		t.Errorf("the plane moved to cell %d with no budget", u.GridX)
	}
}

// TestAirOrderOnOwnCell: there is nothing to fly when the destination is the plane's own cell.
func TestAirOrderOnOwnCell(t *testing.T) {
	u := newPlane(t, nil, 10, 3, 3, 2, true)
	if m := NewAirMoveOrder(nil, u, 3, 3, false); m != nil {
		t.Error("an air order on the plane's own cell must not start")
	}
}

// TestAirStopAtNextCell: an emergency stop mid flight aims the plane at the centre ahead; it parks
// there, and the original destination comes back as the route to hold.
func TestAirStopAtNextCell(t *testing.T) {
	mgr := NewManager()
	u := newPlane(t, mgr, 50, 1, 1, 2, true)
	m := NewAirMoveOrder(mgr, u, 11, 1, false)
	for i := 0; i < 6; i++ {
		m.Tick()
	}
	if !m.MidCell() {
		t.Fatal("the plane should be between cell centres after 6 ticks")
	}
	rest := m.StopAtNextCell()
	if len(rest) != 1 || rest[0].To.X != 11 {
		t.Fatalf("held route = %v, want the destination (11,1)", rest)
	}
	status, _ := run(t, m)
	if status != MoveDone {
		t.Fatalf("status = %v, want MoveDone on the stop cell", status)
	}
	if m.MidCell() || u.X%tileSize != tileSize/2 {
		t.Errorf("the plane is not parked on a cell centre (x %d)", u.X)
	}
	if u.GridX >= 11 {
		t.Errorf("the plane flew on to cell %d instead of stopping", u.GridX)
	}
}

// TestAirRouteMarkers: the path arrows put one marker per movement point along the line, the last
// one on the destination.
func TestAirRouteMarkers(t *testing.T) {
	east := AirRouteMarkers(1, 1, 5, 1)
	if len(east) != 4 || east[3].To.X != 5 || east[3].To.Y != 1 || east[0].Dir != 2 {
		t.Errorf("east markers = %v, want 4 cells ending on (5,1) heading 2", east)
	}
	diag := AirRouteMarkers(1, 1, 8, 8)
	if len(diag) != 10 || diag[9].To.X != 8 || diag[9].To.Y != 8 || diag[0].Dir != 3 {
		t.Errorf("diagonal markers = %v, want 10 ending on (8,8) heading 3", diag)
	}
	if AirRouteMarkers(2, 2, 2, 2) != nil {
		t.Error("a zero length flight has no markers")
	}
}

// TestAirFlightCutToBudgetOblique: an oblique flight too long for the budget is cut to a whole cell
// vector (the target is rounded down to cells), so the plane parks on a cell centre with its points spent.
func TestAirFlightCutToBudgetOblique(t *testing.T) {
	mgr := NewManager()
	u := newPlane(t, mgr, 5, 1, 1, 3, true)
	m := NewAirMoveOrder(mgr, u, 22, 12, false)
	status, _ := run(t, m)
	if status != MoveBlocked {
		t.Fatalf("status = %v, want MoveBlocked", status)
	}
	if u.GridX != 5 || u.GridY != 3 {
		t.Errorf("stopped at cell (%d,%d), want (5,3)", u.GridX, u.GridY)
	}
	if u.X != 5*tileSize+tileSize/2 || u.Y != 3*tileSize+tileSize/2 {
		t.Errorf("stopped at pixel (%d,%d), want the centre of the cell", u.X, u.Y)
	}
	if u.Speed != 0 {
		t.Errorf("speed = %d, want 0", u.Speed)
	}
}
