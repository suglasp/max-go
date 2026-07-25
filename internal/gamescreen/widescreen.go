// widescreen.go : switches the game view to the wide 16:9 layout.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : window_manager.cpp - WindowManager_ScaleWindows / WindowManager_CustomSpriteScaler
package gamescreen

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// The FRAMEPIC vertical bands the widening preserves: the left panel columns
// are copied pixel for pixel, the right border columns move to the new right
// edge, and everything between stretches horizontally.
const (
	frameLeftBand  = 180
	frameRightBand = 12
)

// widenFrame builds the wide FRAMEPIC variant: left band verbatim, middle
// band stretched horizontally (nearest neighbour), right band moved to the
// new right edge. Height is unchanged. A nil or already wide input returns
// as is.
//
// C++ ported : window_manager.cpp - WindowManager_CustomSpriteScaler
func widenFrame(bg *gfx.Buffer, wideWidth int) *gfx.Buffer {
	if bg == nil || bg.W >= wideWidth || bg.W <= frameLeftBand+frameRightBand {
		return bg
	}

	out := gfx.New(wideWidth, bg.H)
	srcMiddle := bg.W - frameLeftBand - frameRightBand
	dstMiddle := wideWidth - frameLeftBand - frameRightBand

	for y := 0; y < bg.H; y++ {
		srcRow := bg.Row(y)
		dstRow := out.Row(y)
		// Left band: the whole left panel, unscaled.
		copy(dstRow[:frameLeftBand], srcRow[:frameLeftBand])

		// Middle band: the top/bottom border strips stretch across the wider run.
		for x := 0; x < dstMiddle; x++ {
			dstRow[frameLeftBand+x] = srcRow[frameLeftBand+x*srcMiddle/dstMiddle]
		}

		// Right band: the outer border, moved to the new edge.
		copy(dstRow[wideWidth-frameRightBand:], srcRow[bg.W-frameRightBand:])
	}

	return out
}

// setWideMode switches the in game shell between the 4:3 and wide 16:9
// frame: the logical resolution (through the GNW resize), the layout rect
// table, the FRAMEPIC variant behind s.bg, and the renderer's viewport
// floor. The caller repaints (repaintBackground + draw). A call in the
// current mode is a no op.
func (s *Screen) setWideMode(wide bool) {
	if s.wideMode == wide {
		return
	}

	s.wideMode = wide
	width := platform.ScreenWidth

	if wide {
		width = platform.WideWidth

		if s.bgWide == nil {
			// Built once on first use; nil stays nil (paintFrame's box fallback).
			s.bgWide = widenFrame(s.bgNarrow, platform.WideWidth)
		}

		s.bg = s.bgWide
	} else {
		s.bg = s.bgNarrow
	}

	s.sys.ResizeScreen(s.plat, width, platform.ScreenHeight)
	ApplyWidescreen(wide)
	// The viewport width changed: lift/relax the zoom floor and keep the
	// camera on the map.
	vw, vh := viewSize(MainMap)
	s.r.SetViewport(vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}
