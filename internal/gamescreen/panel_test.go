// panel_test.go : tests panel.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// TestCycleNext checks the unit cycle order helper: wrapping forward/back and
// the empty / nothing yet selected cases.
func TestCycleNext(t *testing.T) {
	a := &game.UnitInfo{Type: assets.Tank}
	b := &game.UnitInfo{Type: assets.Scout}
	c := &game.UnitInfo{Type: assets.Constrct}
	us := []*game.UnitInfo{a, b, c}

	cases := []struct {
		cur  *game.UnitInfo
		dir  int
		want *game.UnitInfo
	}{
		{a, 1, b},    // forward
		{c, 1, a},    // forward wraps
		{b, -1, a},   // back
		{a, -1, c},   // back wraps
		{nil, 1, a},  // nothing selected -> first
		{nil, -1, c}, // nothing selected, back -> last
	}
	for _, tc := range cases {
		if got := cycleNext(us, tc.cur, tc.dir); got != tc.want {
			t.Errorf("cycleNext(cur=%v dir=%d) = %v, want %v", tc.cur, tc.dir, got, tc.want)
		}
	}
	if got := cycleNext(nil, a, 1); got != a {
		t.Errorf("cycleNext(empty) = %v, want the current unit back", got)
	}
}

// TestPanelCycleLockedDuringAiTurn verifies the spectator input rule: while a
// computer turn animates (s.turnFlow.aiTurn), the panel's unit cycle actions (Prev/Done/
// Next/Lock) are inert but still consume the click, so the player cannot cycle
// or order during the opponent's turn. The display toggles staying live is the
// separate unlocked path. The locked branch returns before any renderer use,
// so this runs headless.
func TestPanelCycleLockedDuringAiTurn(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkUnit(g, assets.Tank, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	mkUnit(g, assets.Tank, 2, 6, 6, game.FlagMobileLandUnit|game.FlagSelectable)
	s.selected = a
	s.turnFlow.aiTurn = true

	// A release click on NEXT: consumed (returns true) but the selection is held.
	click := platform.MouseState{
		X:       (NextButton.ULX + NextButton.LRX) / 2,
		Y:       (NextButton.ULY + NextButton.LRY) / 2,
		Buttons: platform.MouseReleaseLeft,
	}
	if !s.handlePanelButtons(click) {
		t.Error("a NEXT click during the AI turn should be consumed")
	}
	if s.selected != a {
		t.Errorf("NEXT must not cycle the selection during the AI turn, got %v", s.selected)
	}
}
