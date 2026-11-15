// movetick_test.go : tests the order tick gate of tickMovementSfx in sfx.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestMovementTickGate: the sim runs one order tick per moveTickMS, as the reference does.
// Two calls within the gate run one sim tick, a call after it runs the next, and a zero
// stamp always fires. A tank facing north turns one eighth per order tick toward its
// eastern route, so its angle counts the sim ticks that ran.
func TestMovementTickGate(t *testing.T) {
	s, g := newTestScreen(t, 16)
	tank := mkTank(g.Units, 0, 1, 5, 5, tankStats{speed: 12, hits: 30})
	if !g.OrderMove(tank, 9, 5) {
		t.Fatal("the move order was refused")
	}

	var now uint64 = 1000
	s.turnFlow.moveClock = func() uint64 { return now }

	step := func(at uint64) uint8 {
		now = at
		s.tickMovementSfx()
		return tank.Angle
	}

	if got := step(1000); got != 1 {
		t.Fatalf("first call (zero stamp): angle = %d, want 1 (one sim tick)", got)
	}
	if got := step(1000 + moveTickMS - 1); got != 1 {
		t.Errorf("a call inside the gate ran a sim tick: angle = %d, want 1", got)
	}
	if got := step(1000 + moveTickMS); got != 2 {
		t.Errorf("a call after the gate: angle = %d, want 2 (the second sim tick)", got)
	}
	if got := step(1000 + moveTickMS + 10); got != 2 {
		t.Errorf("the gate restarts at the last tick: angle = %d, want 2", got)
	}
}

// TestMovementTickGateZeroStampFires: a freshly reset turn flow always fires, whatever the clock says.
func TestMovementTickGateZeroStampFires(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.turnFlow.moveClock = func() uint64 { return 5 }
	if !s.moveTickDue() {
		t.Fatal("a zero stamp must always fire")
	}
	if s.moveTickDue() {
		t.Error("a second call at the same time must not fire")
	}
}
