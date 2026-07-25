// helpcoords.go : maps a live pointer position into the coordinate space the help data uses.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : helpmenu.cpp - HelpMenu_ScaleCursor
package gamescreen

// The help database stores clickable rectangles in the 640x480 layout. Widescreen moves the instrument strips and widens the map, so this maps those areas back into that layout.
// The left panel keeps its position and needs no adjustment.

// Probe points select help rectangles when the pointer is over an instrument. C++: helpmenu.cpp:412 - HelpMenu_ScaleCursor.
const (
	helpEndTurnX, helpEndTurnY     = 426, 11
	helpTurnCountX, helpTurnCountY = 498, 11
	helpTurnTimerX, helpTurnTimerY = 565, 11
	helpTopRightX, helpTopRightY   = 610, 11
	helpCoordsX, helpCoordsY       = 250, 473
	helpUnitNameX, helpUnitNameY   = 448, 473
)

// The 4:3 game frame the help rectangles are written against.
const (
	helpMapULX = 180
	helpMapULY = 18
	helpMapLRX = 627
	helpMapLRY = 465
)

// HelpCoords maps a live pointer position on the game screen into the 640x480 space the
// help database is written in. In 4:3 it is very nearly the identity; in widescreen it
// snaps the moved instruments back to where the data expects them and squeezes the wider
// map back into the old map frame. The caller installs it as menus.Context.HelpFixup.
func HelpCoords(x, y int) (int, int) {
	// The left panel is the same pixels at the same place in both aspect ratios.
	if x < helpMapULX {
		return x, y
	}

	// The top instruments strip: End Turn, the turn counter, the turn timer, and the
	// strip to their right.
	if y <= EndTurnButton.LRY {
		switch {
		case inRect(EndTurnButton, x, y):
			return helpEndTurnX, helpEndTurnY

		case inRect(TurnCounter, x, y):
			return helpTurnCountX, helpTurnCountY

		case inRect(TurnTimer, x, y):
			return helpTurnTimerX, helpTurnTimerY

		case x > TurnTimer.LRX:
			return helpTopRightX, helpTopRightY
		}

		return x, y
	}

	// The bottom readouts: the map coordinates and the selected unit's name.
	if inRect(CoordsDisplay, x, y) {
		return helpCoordsX, helpCoordsY
	}

	if inRect(UnitNameDisplay, x, y) {
		return helpUnitNameX, helpUnitNameY
	}

	// The map itself: scale the (possibly wider) view back into the 4:3 map frame, so a
	// click anywhere on it lands inside the data's map rectangle.
	if inRect(MainMap, x, y) {
		liveW := MainMap.LRX - MainMap.ULX
		liveH := MainMap.LRY - MainMap.ULY

		if liveW <= 0 || liveH <= 0 {
			return x, y
		}

		mx := helpMapULX + (x-MainMap.ULX)*(helpMapLRX-helpMapULX)/liveW
		my := helpMapULY + (y-MainMap.ULY)*(helpMapLRY-helpMapULY)/liveH

		return mx, my
	}

	return x, y
}
