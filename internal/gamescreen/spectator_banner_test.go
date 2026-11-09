// spectator_banner_test.go : tests the opponent-turn banner and inspection clicks during computer turns.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestBannerShowsComputerTurn checks that an animating computer turn shows the
// "<Colour> Computer Turn." banner. Before, the banner only appeared during a hot
// seat handover, so the player saw no "whose turn is it" line while an NPC played.
func TestBannerShowsComputerTurn(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.turnFlow.handover = -1 // not a hot seat pass
	s.banner.msg = ""
	s.turnFlow.aiTurn = true
	s.turnFlow.aiTurnTeam = 1

	text, show, tint := s.bannerText()
	if !show {
		t.Fatal("a computer turn should show a turn banner")
	}
	if tint != bannerTintDark {
		t.Errorf("turn banner tint = %v, want dark", tint)
	}
	if want := fmt.Sprintf("%s Computer Turn.", s.teamName(1)); text != want {
		t.Errorf("banner = %q, want %q", text, want)
	}

	// The solo human's own turn has no vanilla banner (input is simply unlocked).
	s.turnFlow.aiTurn = false
	if _, show, _ := s.bannerText(); show {
		t.Error("a solo human turn should show no banner")
	}
}

// TestSpectatorLeftClickSelectsNoOrder checks that left clicking a unit during the
// opponent's turn selects it for inspection but issues no order, and that clicking
// empty terrain keeps the current selection (it is never dropped mid opponent turn).
func TestSpectatorLeftClickSelectsNoOrder(t *testing.T) {
	s, g := newTestScreen(t, 16)
	s.turnFlow.aiTurn = true

	tank := mkUnit(g, assets.Tank, 0, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	bld := mkUnit(g, assets.PowerStn, 0, 2, 6, 6, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	s.selected = tank

	s.selectForInspect(6, 6) // inspect the building
	if s.selected != bld {
		t.Fatalf("left click should select the unit under the cursor, got %v", s.selected == tank)
	}
	if n := len(g.MoveOrders()); n != 0 {
		t.Errorf("inspecting during the AI turn must issue no move order, got %d", n)
	}
	if n := len(g.PendingRoutes()); n != 0 {
		t.Errorf("inspecting during the AI turn must queue no route, got %d", n)
	}

	// Empty terrain: the selection is kept, not cleared.
	s.selectForInspect(0, 0)
	if s.selected != bld {
		t.Error("clicking empty terrain during the AI turn should keep the selection")
	}
}
