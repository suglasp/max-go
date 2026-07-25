// buildmenu_test.go : tests buildmenu.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestBuilderRosters verifies the engineer/constructor split: the constructor
// builds only large (2x2) structures, the engineer only small (1x1) ones, a
// a unit that is not a builder has no roster, and the two rosters do not overlap.
func TestBuilderRosters(t *testing.T) {
	if got := rosterFor(assets.Tank); got != nil {
		t.Errorf("rosterFor(Tank) = %v, want nil (not a builder)", got)
	}
	con := rosterFor(assets.Constrct)
	eng := rosterFor(assets.Engineer)
	if len(con) == 0 || len(eng) == 0 {
		t.Fatalf("empty roster: constructor=%d engineer=%d", len(con), len(eng))
	}
	for _, b := range con {
		if !b.large {
			t.Errorf("constructor roster has a non-large entry %s", b.t.Tag())
		}
	}
	for _, b := range eng {
		if b.large {
			t.Errorf("engineer roster has a large entry %s", b.t.Tag())
		}
	}
	// No type appears in both rosters.
	seen := map[assets.UnitType]bool{}
	for _, b := range append(append([]buildableType{}, con...), eng...) {
		if seen[b.t] {
			t.Errorf("type %s is in both rosters", b.t.Tag())
		}
		seen[b.t] = true
	}
	// The combined buildableTypes is exactly the two rosters.
	if len(buildableTypes) != len(con)+len(eng) {
		t.Errorf("buildableTypes has %d entries, want %d", len(buildableTypes), len(con)+len(eng))
	}
	// The Gold Refinery (COMMTWR) is a constructor building, so it must be in the
	// constructor roster so the player can place it and grow the gold economy.
	hasRefinery := false
	for _, b := range con {
		if b.t == assets.CommTwr {
			hasRefinery = true
		}
	}
	if !hasRefinery {
		t.Error("constructor roster is missing the Gold Refinery (COMMTWR)")
	}
}
