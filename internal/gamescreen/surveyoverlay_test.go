// surveyoverlay_test.go : tests the SURVEY panel toggle and the map wide resource overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Tests for the SURVEY panel toggle driving the map wide resource overlay.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSurveyOverlayTogglesResourceVisibility verifies the SURVEY panel toggle
// makes the resource overlay draw map wide independent of selection, while the
// surveyor selected and maxSurvey debug paths still work on their own.
func TestSurveyOverlayTogglesResourceVisibility(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	// Nothing on: no selection, no toggle -> overlay hidden.
	if s.resourceOverlayVisible() {
		t.Error("overlay should be hidden with no selection and SURVEY off")
	}

	// SURVEY toggle on -> overlay visible map wide, with nothing selected.
	s.surveyOverlay = true
	if !s.resourceOverlayVisible() {
		t.Error("overlay should be visible when the SURVEY toggle is on")
	}
	s.surveyOverlay = false

	// The maxSurvey debug reveal independently shows it.
	s.maxSurvey = true
	if !s.resourceOverlayVisible() {
		t.Error("overlay should be visible under the maxSurvey debug")
	}
	s.maxSurvey = false

	// A selected surveyor still shows it without the toggle.
	sur := mkUnit(s.game, assets.Surveyor, 0, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	s.selected = sur
	if !s.resourceOverlayVisible() {
		t.Error("overlay should be visible when a surveyor is selected")
	}
}

// TestSurveyButtonActiveProbeReflectsToggle checks the SURVEY bar button shows
// its pressed (_ON) face exactly when the overlay toggle is on.
func TestSurveyButtonActiveProbeReflectsToggle(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	// Find the SURVEY button in the bar list.
	var survey *panelButton
	for i, b := range s.panelButtons() {
		if b.rect == SurveyButton {
			survey = &s.panelButtons()[i]
			break
		}
	}
	if survey == nil || survey.active == nil {
		t.Fatal("SURVEY button should be present with a toggle-state probe")
	}

	// Off by default, on after toggling.
	if survey.active() {
		t.Error("SURVEY probe should be false when the overlay is off")
	}
	s.surveyOverlay = true
	if !survey.active() {
		t.Error("SURVEY probe should be true when the overlay is on")
	}
}
