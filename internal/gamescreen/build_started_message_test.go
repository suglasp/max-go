// build_started_message_test.go : tests the completion-time message for a new mobile construction order.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// mkBuilder returns a constructor of the given team.
func mkBuilder(team int) *game.UnitInfo {
	return game.NewUnitInfo(assets.Constrct, team, 1, 0)
}

// TestAnnounceBuildStarted: a human player's mobile builder beginning a structure posts a
// routine dark banner plus a Messages row naming the structure and its turns to complete;
// singular for a one turn build. A computer team's build is silent.
func TestAnnounceBuildStarted(t *testing.T) {
	// A human build posts the banner + a message with the turn count.
	s := newDamageScreen(t)
	s.announceBuildStarted(mkBuilder(0), assets.Tank, 5, 3, 4)
	text, ok, tint := s.bannerText()
	if !ok || tint != bannerTintDark {
		t.Errorf("build banner = (ok=%v, tint=%v), want (true, dark)", ok, tint)
	}
	if !strings.Contains(text, "under construction") || !strings.Contains(text, "5 turns") {
		t.Errorf("build banner text = %q, want it to mention under construction + 5 turns", text)
	}
	if len(s.banner.msgLog) != 1 {
		t.Fatalf("msgLog len = %d, want 1", len(s.banner.msgLog))
	}
	if got := s.banner.msgLog[0].coords; got != "3,4" {
		t.Errorf("message coords = %q, want %q", got, "3,4")
	}

	// A one turn build reads "1 turn", not "1 turns".
	s = newDamageScreen(t)
	s.announceBuildStarted(mkBuilder(0), assets.Tank, 1, 0, 0)
	if text, _, _ := s.bannerText(); !strings.Contains(text, "1 turn.") || strings.Contains(text, "1 turns") {
		t.Errorf("one turn banner = %q, want singular \"1 turn.\"", text)
	}

	// A build with a sub-one turn count is clamped to one turn (never "0 turns").
	s = newDamageScreen(t)
	s.announceBuildStarted(mkBuilder(0), assets.Tank, 0, 0, 0)
	if text, _, _ := s.bannerText(); !strings.Contains(text, "1 turn.") {
		t.Errorf("zero turn banner = %q, want it clamped to \"1 turn.\"", text)
	}

	// A computer team's build is not the player's concern: no banner, no log.
	s = newDamageScreen(t)
	s.announceBuildStarted(mkBuilder(2), assets.Tank, 5, 3, 4)
	if _, ok, _ := s.bannerText(); ok {
		t.Error("a computer team build should post no banner")
	}
	if len(s.banner.msgLog) != 0 {
		t.Errorf("msgLog len = %d after a computer build, want 0", len(s.banner.msgLog))
	}
}
