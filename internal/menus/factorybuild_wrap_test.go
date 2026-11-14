// factorybuild_wrap_test.go : tests that factory build list names stay clear of the turns number.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestFactoryListNamesClearTurns wraps every factory unit name the way both lists
// draw it and checks that no line reaches the left edge of a two digit turns number. A single word
// wider than the wrap width cannot be broken and is exempt.
func TestFactoryListNamesClearTurns(t *testing.T) {
	set, err := fonts.LoadSet(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	f := set.Get(fonts.Font5)
	lists := []struct {
		name           string
		nameX, turnsRX int
	}{
		{"available", fbNameX, fbTurnsRX},
		{"queue", fbQNameX, fbQTurnsRX},
	}
	for _, l := range lists {
		wrapW := fbNameWrapW(f, l.nameX, l.turnsRX)
		turnsLeft := l.turnsRX - f.Width("88")
		for _, factory := range []assets.UnitType{assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.Shipyard, assets.TrainHal} {
			for _, e := range factoryRoster(factory) {
				for _, ln := range wrapLines(f, e.name, wrapW, 0) {
					// A single word wider than the wrap width cannot be broken; only multi word lines must clear.
					if strings.Contains(ln, " ") || f.Width(ln) <= wrapW {
						if right := l.nameX + f.Width(ln); right >= turnsLeft {
							t.Errorf("%s list: %q line %q ends at x=%d, turns number starts at x=%d", l.name, e.name, ln, right, turnsLeft)
						}
					}
				}
			}
		}
	}
}

// TestFactoryQueueRepairUnitWraps checks that "Repair Unit" no longer fits one line
// in the queue list, so it wraps instead of running over its turns number.
func TestFactoryQueueRepairUnitWraps(t *testing.T) {
	set, err := fonts.LoadSet(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	f := set.Get(fonts.Font5)
	lines := wrapLines(f, "Repair Unit", fbNameWrapW(f, fbQNameX, fbQTurnsRX), 0)
	if len(lines) < 2 {
		t.Errorf("queue wrap of %q = %q, want it wrapped onto two lines", "Repair Unit", lines)
	}
}
