// layout.go : implements the game screen layout constants.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : window_manager.cpp - windows[] layout table
//
// Note: Package gamescreen is the in game screen shell: it lays out the main
// game window the way window_manager.cpp does, renders the tactical map
// (via internal/drawmap) into the map viewport with scrolling and zoom,
// draws the minimap with a click to jump viewport box, and animates the
// terrain palette. The side panel gauges, unit sprites and FLC portraits
// build on this frame.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// Window rectangles from the static layout table in window_manager.cpp
// (640x480 logical screen, GNW inclusive corner rectangles).
//
// C++: windows[] (window_manager.cpp)
var (
	// MainWindow is the whole screen (WINDOW_MAIN_WINDOW).
	MainWindow = gfx.Rect{ULX: 0, ULY: 0, LRX: 639, LRY: 479}
	// MainMap is the tactical map viewport, 7 tiles square at 1:1 (WINDOW_MAIN_MAP: 180,18 .. 7*64+180-1, 7*64+18-1).
	MainMap = gfx.Rect{ULX: 180, ULY: 18, LRX: 627, LRY: 465}
	// Minimap is the overview box (WINDOW_MINIMAP), 112x112.
	Minimap = gfx.Rect{ULX: 15, ULY: 356, LRX: 126, LRY: 467}
	// CornerFlic is the top left portrait/animation panel (WINDOW_CORNER_FLIC).
	CornerFlic = gfx.Rect{ULX: 10, ULY: 29, LRX: 137, LRY: 156}
	// PlayButton / PauseButton are the small icon buttons just right of the
	// corner portrait: Play resumes the unit's rotating FLIC animation, Pause
	// freezes it. C++: WINDOW_PLAY_BUTTON (146,123,164,140) / WINDOW_PAUSE_BUTTON
	// (146,143,164,161) in window_manager.cpp.
	PlayButton  = gfx.Rect{ULX: 146, ULY: 123, LRX: 164, LRY: 140}
	PauseButton = gfx.Rect{ULX: 146, ULY: 143, LRX: 164, LRY: 161}
	// HelpButton / ReportsButton are lower panel strip buttons. C++:
	// WINDOW_HELP_BUTTON (21,250,45,273) / WINDOW_REPORTS_BUTTON (51,252,98,270).
	HelpButton    = gfx.Rect{ULX: 21, ULY: 250, LRX: 45, LRY: 273}
	ReportsButton = gfx.Rect{ULX: 51, ULY: 252, LRX: 98, LRY: 270}
	// FilesButton / PrefsButton are the top left menu buttons: FILES opens the
	// Save/Load screen, PREFERENCES the in game Options. C++: WINDOW_FILES_BUTTON (17,4,83,21) / WINDOW_PREFS_BUTTON (86,4,152,21) in window_manager.cpp.
	FilesButton = gfx.Rect{ULX: 17, ULY: 4, LRX: 83, LRY: 21}
	PrefsButton = gfx.Rect{ULX: 86, ULY: 4, LRX: 152, LRY: 21}

	// EndTurnButton, TurnCounter and TurnTimer are the live readouts in the
	// top instruments strip (WINDOW_ENDTURN_BUTTON / WINDOW_TURN_COUNTER_DISPLAY / WINDOW_TURN_TIMER_DISPLAY).
	// The button carries the ENDTRN art; the two displays show centred gold FONT5 text over their FRAMEPIC slot.
	EndTurnButton = gfx.Rect{ULX: 390, ULY: 2, LRX: 461, LRY: 22}
	TurnCounter   = gfx.Rect{ULX: 468, ULY: 2, LRX: 528, LRY: 22}
	TurnTimer     = gfx.Rect{ULX: 534, ULY: 2, LRX: 594, LRY: 22}
	// StatWindow is the selected unit stat panel under the corner portrait (WINDOW_STAT_WINDOW)
	//  StatRow1..4 are its four readout rows (WINDOW_STAT_ROW_1..4): hits, ammo/cargo, speed, shots.
	StatWindow = gfx.Rect{ULX: 10, ULY: 173, LRX: 162, LRY: 220}
	StatRow1   = gfx.Rect{ULX: 10, ULY: 173, LRX: 162, LRY: 184}
	StatRow2   = gfx.Rect{ULX: 10, ULY: 185, LRX: 162, LRY: 196}
	StatRow3   = gfx.Rect{ULX: 10, ULY: 197, LRX: 162, LRY: 208}
	StatRow4   = gfx.Rect{ULX: 10, ULY: 209, LRX: 162, LRY: 220}
	// CoordsDisplay and UnitNameDisplay are the bottom instruments strip's readouts (WINDOW_COORDINATES_DISPLAY / WINDOW_UNIT_DESCRIPTION_DISPLAY):
	// the map cell under the cursor and the selected unit's name.
	CoordsDisplay   = gfx.Rect{ULX: 242, ULY: 456, LRX: 331, LRY: 476}
	UnitNameDisplay = gfx.Rect{ULX: 338, ULY: 456, LRX: 559, LRY: 476}

	// Minimap control buttons to the right of the minimap: 2X toggles a
	// zoomed minimap, TNT shows only combat units. C++: WINDOW_2X_MINIMAP /
	// WINDOW_TNT_MINIMAP (window_manager.cpp).
	MinimapX2  = gfx.Rect{ULX: 136, ULY: 387, LRX: 162, LRY: 412}
	MinimapTnt = gfx.Rect{ULX: 136, ULY: 413, LRX: 162, LRY: 439}

	// Zoom slider in the left panel: a "-"/track/"+" trio. The knob (ZOOMPTR)
	// rides the ZOOMPNL1 track; clicking/dragging the track sets the zoom.
	// C++: WINDOW_ZOOM_PLUS/SLIDER/MINUS_BUTTON + WINDOW_ZOOM_SLIDER_WINDOW.
	ZoomPlus   = gfx.Rect{ULX: 2, ULY: 275, LRX: 19, LRY: 288}
	ZoomSlider = gfx.Rect{ULX: 20, ULY: 275, LRX: 149, LRY: 288}
	ZoomMinus  = gfx.Rect{ULX: 150, ULY: 275, LRX: 169, LRY: 288}
	ZoomTrack  = gfx.Rect{ULX: 20, ULY: 275, LRX: 149, LRY: 293}

	// Unit control row above the Help/Reports strip: Center (jump the view to the
	// selected unit), Lock (follow it), and the Prev/Done/Next unit cycle. C++:
	// WINDOW_CENTER/LOCK/PRE/DONE/NXT_BUTTON (window_manager.cpp).
	CenterButton = gfx.Rect{ULX: 4, ULY: 227, LRX: 23, LRY: 249}
	LockButton   = gfx.Rect{ULX: 32, ULY: 227, LRX: 51, LRY: 248}
	PrevButton   = gfx.Rect{ULX: 60, ULY: 228, LRX: 97, LRY: 249}
	DoneButton   = gfx.Rect{ULX: 99, ULY: 228, LRX: 124, LRY: 249}
	NextButton   = gfx.Rect{ULX: 125, ULY: 228, LRX: 162, LRY: 249}

	// Chat button (out of scope: networking). C++: WINDOW_CHAT_BUTTON.
	ChatButton = gfx.Rect{ULX: 102, ULY: 252, LRX: 149, LRY: 270}

	// Display overlay toggle grid (3 columns x 3 rows) below the zoom slider:
	// each toggles a map overlay. C++: WINDOW_SURVEY/STATUS/COLORS/HITS/AMMO/
	// RANGE/SCAN/GRID/NAME_BUTTON (window_manager.cpp).
	SurveyButton = gfx.Rect{ULX: 4, ULY: 298, LRX: 58, LRY: 313}
	StatusButton = gfx.Rect{ULX: 4, ULY: 314, LRX: 58, LRY: 329}
	ColorsButton = gfx.Rect{ULX: 4, ULY: 330, LRX: 58, LRY: 347}
	HitsButton   = gfx.Rect{ULX: 59, ULY: 298, LRX: 113, LRY: 313}
	AmmoButton   = gfx.Rect{ULX: 59, ULY: 314, LRX: 113, LRY: 329}
	RangeButton  = gfx.Rect{ULX: 59, ULY: 330, LRX: 113, LRY: 347}
	ScanButton   = gfx.Rect{ULX: 114, ULY: 298, LRX: 168, LRY: 313}
	GridButton   = gfx.Rect{ULX: 114, ULY: 314, LRX: 168, LRY: 329}
	NameButton   = gfx.Rect{ULX: 114, ULY: 330, LRX: 168, LRY: 347}
)

const (
	// edgeMargin is how close to a screen edge the mouse must be to
	// trigger edge scrolling.
	edgeMargin = 4
	// scrollStep is the per frame edge scroll distance in screen pixels.
	scrollStep = 8
)

// ApplyWidescreen switches the layout between the 640-wide 4:3 frame and the 854-wide 16:9 frame. The left panel stays fixed, top instruments remain right-aligned, and bottom readouts remain centred.
//
// C++ ported : window_manager.cpp - WindowManager_ScaleWindows
func ApplyWidescreen(wide bool) {
	extra := 0

	if wide {
		extra = platform.WideWidth - platform.ScreenWidth
	}

	centered := extra / 2
	MainWindow = gfx.Rect{ULX: 0, ULY: 0, LRX: 639 + extra, LRY: 479}
	MainMap = gfx.Rect{ULX: 180, ULY: 18, LRX: 627 + extra, LRY: 465}
	EndTurnButton = gfx.Rect{ULX: 390 + extra, ULY: 2, LRX: 461 + extra, LRY: 22}
	TurnCounter = gfx.Rect{ULX: 468 + extra, ULY: 2, LRX: 528 + extra, LRY: 22}
	TurnTimer = gfx.Rect{ULX: 534 + extra, ULY: 2, LRX: 594 + extra, LRY: 22}
	CoordsDisplay = gfx.Rect{ULX: 242 + centered, ULY: 456, LRX: 331 + centered, LRY: 476}
	UnitNameDisplay = gfx.Rect{ULX: 338 + centered, ULY: 456, LRX: 559 + centered, LRY: 476}
}

// viewSize returns a rectangle's pixel dimensions. GNW Width and Height use inclusive coordinates.
func viewSize(r gfx.Rect) (int, int) {
	return r.Width() + 1, r.Height() + 1
}

// centerScroll returns the scaled top-left map position that centres a cell in the viewport. The caller clamps it to the map.
func centerScroll(cellX, cellY, zoom, vw, vh int) (int, int) {
	return cellX*zoom + zoom/2 - vw/2, cellY*zoom + zoom/2 - vh/2
}

// minimapCell maps a screen-space point inside the minimap to a map cell. The bool is false outside the minimap.
func minimapCell(mx, my, mapW, mapH int) (int, int, bool) {
	if mx < Minimap.ULX || mx > Minimap.LRX || my < Minimap.ULY || my > Minimap.LRY {
		return 0, 0, false
	}

	mw, mh := viewSize(Minimap)
	cellX := (mx - Minimap.ULX) * mapW / mw
	cellY := (my - Minimap.ULY) * mapH / mh
	return max(0, min(cellX, mapW-1)), max(0, min(cellY, mapH-1)), true
}

// edgeScroll returns the per-frame scroll delta for a mouse near a screen edge. It simplifies the original's map-edge scroll windows to the screen border.
func edgeScroll(mx, my, step int) (int, int) {
	var dx, dy int

	switch {
	case mx <= MainWindow.ULX+edgeMargin:
		dx = -step
	case mx >= MainWindow.LRX-edgeMargin:
		dx = step
	}

	switch {
	case my <= MainWindow.ULY+edgeMargin:
		dy = -step
	case my >= MainWindow.LRY-edgeMargin:
		dy = step
	}

	return dx, dy
}

/* // note: unused code in Go port
// clamp repeated the builtins; callers write max(lo, min(v, hi)).
// clamp constrains v to the inclusive lo..hi range.
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}
*/

// inRect reports whether (x, y) lies inside a GNW inclusive corner rect.
func inRect(r gfx.Rect, x, y int) bool {
	return x >= r.ULX && x <= r.LRX && y >= r.ULY && y <= r.LRY
}

/* // note: unused code in Go port
// clampInt repeated clamp and the builtins; callers write max(lo, min(v, hi)).
// clampInt clamps v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}
*/
