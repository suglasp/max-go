// helpcoords_test.go : tests the help pointer fixup for the widescreen game frame.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestHelpCoordsFourThree: in the 4:3 frame the widgets sit exactly where the help data
// says they do, so the pointer needs almost no fixup - only the instruments snap to their
// probe points, as they do in the original.
func TestHelpCoordsFourThree(t *testing.T) {
	ApplyWidescreen(false)

	// The left panel is never touched.
	if x, y := HelpCoords(60, 200); x != 60 || y != 200 {
		t.Errorf("left panel (60,200) -> (%d,%d), want it untouched", x, y)
	}

	// The End Turn button snaps to its probe point.
	if x, y := HelpCoords(EndTurnButton.ULX+5, 10); x != helpEndTurnX || y != helpEndTurnY {
		t.Errorf("End Turn -> (%d,%d), want (%d,%d)", x, y, helpEndTurnX, helpEndTurnY)
	}

	// A click in the middle of the map stays in the map.
	x, y := HelpCoords(400, 240)
	if x < helpMapULX || x > helpMapLRX || y < helpMapULY || y > helpMapLRY {
		t.Errorf("map click -> (%d,%d), want it inside the 4:3 map frame", x, y)
	}
}

// TestHelpCoordsWidescreen: with the wide frame the instruments have MOVED, so a raw
// pointer position would miss every rectangle in the help data. The fixup puts the click
// back where the data expects it: the instruments snap, and the wider map is squeezed back
// into the old map frame.
func TestHelpCoordsWidescreen(t *testing.T) {
	ApplyWidescreen(true)
	defer ApplyWidescreen(false)

	// The moved End Turn button still resolves to the original's End Turn probe point.
	if x, y := HelpCoords(EndTurnButton.ULX+5, 10); x != helpEndTurnX || y != helpEndTurnY {
		t.Errorf("wide End Turn -> (%d,%d), want (%d,%d)", x, y, helpEndTurnX, helpEndTurnY)
	}

	// So do the turn counter and the turn timer.
	if x, _ := HelpCoords(TurnCounter.ULX+5, 10); x != helpTurnCountX {
		t.Errorf("wide turn counter -> x=%d, want %d", x, helpTurnCountX)
	}

	if x, _ := HelpCoords(TurnTimer.ULX+5, 10); x != helpTurnTimerX {
		t.Errorf("wide turn timer -> x=%d, want %d", x, helpTurnTimerX)
	}

	// The right hand edge of the wide map still lands inside the 4:3 map frame, not past it.
	x, y := HelpCoords(MainMap.LRX-1, MainMap.LRY-1)
	if x < helpMapULX || x > helpMapLRX || y < helpMapULY || y > helpMapLRY {
		t.Errorf("wide map corner -> (%d,%d), want it inside the 640x480 map frame", x, y)
	}

	// The left panel does not move in widescreen either.
	if x, y := HelpCoords(60, 200); x != 60 || y != 200 {
		t.Errorf("wide left panel (60,200) -> (%d,%d), want it untouched", x, y)
	}
}
