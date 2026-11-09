// autosurvey_test.go : tests the Auto Survey popup toggle and the per turn survey driver.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Tests for the player's Auto Survey popup toggle and the per turn driver.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestAutoSurveyPopupItemAppearsForPlayerSurveyor verifies that the popup for a
// player Surveyor includes the "Auto" item and that toggling it flips AutoSurvey.
func TestAutoSurveyPopupItemAppearsForPlayerSurveyor(t *testing.T) {
	s, g := newTestScreen(t, 16)
	// A fresh resource map (every cell unsurveyed) so toggling Auto ON finds a
	// nearby cell to drive to and the mode stays on (it clears automatically only when
	// nothing remains to survey).
	g.Resources = economy.NewResourceMap(g.W, g.H)
	// Deploy a player Surveyor with a move budget.
	sur := mkUnit(g, assets.Surveyor, 0, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	sur.Speed = 6
	sur.Values.Set(game.AttribSpeed, 6)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	items := s.popupItemsFor(sur)

	// Find the Auto item.
	var autoItem *popupItem
	for i := range items {
		if items[i].label == "Auto" {
			autoItem = &items[i]
			break
		}
	}
	if autoItem == nil {
		t.Fatal("popup for a player Surveyor should contain an 'Auto' item")
	}

	// Initially AutoSurvey is off; the item's active flag reflects that.
	if sur.AutoSurvey {
		t.Error("AutoSurvey should default to false")
	}
	if autoItem.active {
		t.Error("Auto item should not be active when AutoSurvey is off")
	}

	// Invoke the action: AutoSurvey toggles on.
	autoItem.action()
	if !sur.AutoSurvey {
		t.Error("invoking Auto action should set AutoSurvey to true")
	}

	// Rebuild items: active flag should now reflect the on state.
	items = s.popupItemsFor(sur)
	for _, it := range items {
		if it.label == "Auto" {
			if !it.active {
				t.Error("Auto item should be active when AutoSurvey is on")
			}
			break
		}
	}
}

// TestAutoSurveyNotShownForNonSurveyorOrEnemy checks that the Auto item is
// absent for a tank (mobile but not a Surveyor) and for an enemy Surveyor.
func TestAutoSurveyNotShownForNonSurveyorOrEnemy(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	// Own tank: no Auto item.
	tank := mkUnit(g, assets.Tank, 0, 1, 2, 2, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Speed = 6
	tank.Values.Set(game.AttribSpeed, 6)
	for _, it := range s.popupItemsFor(tank) {
		if it.label == "Auto" {
			t.Error("tank popup should not include an Auto item")
		}
	}

	// Enemy Surveyor: no Auto item.
	enem := mkUnit(g, assets.Surveyor, 0, 2, 6, 6, game.FlagMobileLandUnit|game.FlagSelectable)
	enem.Team = 1
	enem.Speed = 6
	enem.Values.Set(game.AttribSpeed, 6)
	for _, it := range s.popupItemsFor(enem) {
		if it.label == "Auto" {
			t.Error("enemy Surveyor popup should not include an Auto item")
		}
	}
}

// TestTickAutoSurveyIssuesMoveOrder verifies that tickAutoSurvey drives a Surveyor
// with AutoSurvey on toward the nearest unsurveyed cell at turn start.
func TestTickAutoSurveyIssuesMoveOrder(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.Resources = economy.NewResourceMap(g.W, g.H)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	// Deploy a Surveyor with AutoSurvey on and a move budget.
	sur := mkUnit(g, assets.Surveyor, 0, 1, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable)
	sur.Speed = 6
	sur.Values.Set(game.AttribSpeed, 6)
	sur.AutoSurvey = true

	// Mark all cells surveyed except one far cell.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if x != g.W-1 || y != g.H-1 {
				g.Resources.MarkSurveyed(x, y, 0)
			}
		}
	}

	// tickAutoSurvey should issue a move toward the unsurveyed cell.
	s.tickAutoSurvey(0)
	if !g.HasOrder(sur) {
		t.Error("tickAutoSurvey should issue a move order for an AutoSurvey Surveyor")
	}
}

// TestTickAutoSurveyClearsFlagWhenDone confirms that AutoSurvey is turned off
// when the map is fully surveyed (nothing left to reveal).
func TestTickAutoSurveyClearsFlagWhenDone(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.Resources = economy.NewResourceMap(g.W, g.H)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	sur := mkUnit(g, assets.Surveyor, 0, 1, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable)
	sur.Speed = 6
	sur.Values.Set(game.AttribSpeed, 6)
	sur.AutoSurvey = true

	// Mark every cell surveyed: nothing to discover.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			g.Resources.MarkSurveyed(x, y, 0)
		}
	}

	s.tickAutoSurvey(0)

	// AutoSurvey should be cleared automatically.
	if sur.AutoSurvey {
		t.Error("tickAutoSurvey should clear AutoSurvey when the map is fully surveyed")
	}
}

// TestRevealResourceMapSwitchesOffAuto verifies the F2 "Unlock resource map" debug
// action: it marks the whole map surveyed for the player team and drives the
// automatic survey tick so a Surveyor in Auto mode immediately switches off.
func TestRevealResourceMapSwitchesOffAuto(t *testing.T) {
	s, g := newTestScreen(t, 16)
	g.Resources = economy.NewResourceMap(g.W, g.H)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	sur := mkUnit(g, assets.Surveyor, 0, 1, 0, 0, game.FlagMobileLandUnit|game.FlagSelectable)
	sur.Speed = 6
	sur.Values.Set(game.AttribSpeed, 6)
	sur.AutoSurvey = true

	// Before: the fresh map is not fully surveyed and Auto is on.
	if s.resourceMapFullySurveyed(0) {
		t.Fatal("a fresh resource map should not be fully surveyed")
	}

	s.revealResourceMap()

	// After: the map reads fully surveyed and the surveyor's Auto turned off.
	if !s.resourceMapFullySurveyed(0) {
		t.Error("revealResourceMap should mark every cell surveyed for the player team")
	}
	if sur.AutoSurvey {
		t.Error("revealResourceMap should switch the surveyor's Auto-Survey off")
	}
}
