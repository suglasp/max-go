// dropped_unit_select_test.go : tests selection after a produced mobile unit exits its factory.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestDroppedUnitAtPrefersMobile: a unit driven out of a build site onto a cell that
// also holds a passable structure (a connector / collector it landed on) is the one
// selected - never the structure, even when the structure is listed first on the cell.
func TestDroppedUnitAtPrefersMobile(t *testing.T) {
	s := newDamageScreen(t)

	// A connector on the cell FIRST (so a naive "first match" would pick it), then the
	// freshly dropped mobile unit on the same cell.
	connector := game.NewUnitInfo(assets.Cnct4W, 0, 1, 0)
	connector.Flags = game.FlagStationary | game.FlagSelectable
	s.game.Units.Deploy(connector, 5, 5)

	dropped := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	dropped.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	s.game.Units.Deploy(dropped, 5, 5)

	if got := s.droppedUnitAt(5, 5); got != dropped {
		t.Fatalf("droppedUnitAt = %v, want the mobile unit that exited, not the structure it landed on", got)
	}

	// A cell with only the connector yields no dropped unit (nothing exited here).
	if got := s.droppedUnitAt(5, 5); got == connector {
		t.Fatal("the connector must never be treated as the dropped unit")
	}
}

// TestReportMessagesNewestFirst: the Messages tab lists the most recent message first,
// so a just posted event is visible at the top without scrolling a long history.
func TestReportMessagesNewestFirst(t *testing.T) {
	s := newDamageScreen(t)
	s.logMessage("oldest")
	s.logMessage("middle")
	s.logMessage("newest")

	msgs := s.reportData().Messages
	if len(msgs) != 3 {
		t.Fatalf("Messages len = %d, want 3", len(msgs))
	}
	if msgs[0].Text != "newest" || msgs[2].Text != "oldest" {
		t.Errorf("Messages order = [%q ... %q], want newest first (newest ... oldest)", msgs[0].Text, msgs[2].Text)
	}
}
