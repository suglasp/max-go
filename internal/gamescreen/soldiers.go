// soldiers.go : implements the soldier/infantry unit rendering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - infantry/commando sprite frame selection
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// soldierZoom is the pixels per tile the soldier walk view opens at: bigger than
// the asset gallery so the small infantry/commando sprites (and their stride)
// read clearly while all eight facings still fit across the map viewport.
const soldierZoom = 48

// soldierCols/soldierRowGap lay the demo out as two rows (infantry, commando) of
// eight facings each: one cell between facings (so all eight fit on screen) and
// a three cell gap between the rows (room for the facing labels).
const (
	soldierCols   = game.AngleCount
	soldierRowGap = 3
)

// DeploySoldiers lays out the two marching soldier types, infantry and commando
// in all eight facings on solid snow for the -emulate soldier view, and sets
// walkAnimAll so every one cycles its walk frames in place (in normal play a
// soldier only steps while it is moving). Each row is one type; each column is
// one facing (0 = north, clockwise). C++: n/a (a debug layout helper).
func (s *Screen) DeploySoldiers() {
	// A fully snow block keeps every soldier off the water/coast (the snow_1 map
	// has a river through it; a corner only probe can straddle it).
	originX, originY, ok := s.snowBlock(soldierCols, soldierRowGap+1)

	if !ok {
		originX, originY = 4, 4
	}

	flags := game.FlagMobileLandUnit | game.FlagSelectable

	for row, t := range []assets.UnitType{assets.Infantry, assets.Commando} {
		gy := originY + row*soldierRowGap

		for a := 0; a < soldierCols; a++ {
			gx := originX + a
			s.deployUnit(t, flags, galleryTeam, uint8(a), gx, gy)
			// Label the first column with the type name, the rest with the facing
			// index, so the row reads "Infantry 1 2 3 ...".
			name := fmt.Sprintf("%d", a)

			if a == 0 {
				name = displayName(t)
			}

			s.galleryLabels = append(s.galleryLabels, galleryLabel{name: name, gx: gx, gy: gy})
		}
	}

	dbg.Printf("soldiers: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))

	s.walkAnimAll = true
	s.fogShow = false
	s.r.SetZoom(soldierZoom)
	vw, vh := viewSize(MainMap)
	// Centre the two row block so all eight facings of both rows fit on screen.
	s.camera.scrollX, s.camera.scrollY = centerScroll(originX+soldierCols/2, originY+soldierRowGap/2, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// snowBlock finds the top left of a w x h cell block whose every cell is land
// (snow), scanning outward from the map origin, robust against the river that
// cuts through the snow map (unlike a corner only probe). C++: n/a.
func (s *Screen) snowBlock(w, h int) (int, int, bool) {
	allSnow := func(x, y int) bool {
		for fy := y; fy < y+h; fy++ {
			for fx := x; fx < x+w; fx++ {
				if s.wmap.SurfaceAt(fx, fy)&world.SurfaceLand == 0 {
					return false
				}
			}
		}

		return true
	}

	for y := 2; y+h < s.wmap.H; y += 2 {
		for x := 2; x+w < s.wmap.W; x += 2 {
			if allSnow(x, y) {
				return x, y, true
			}
		}
	}

	return 0, 0, false
}

// walkStepMS is how often a marching soldier advances one walk cycle frame; a
// touch slower than the dish spin so the stride reads as a walk rather than a
// blur. C++: the per move offset image_index+8 advance (unitinfo.cpp).
const walkStepMS = 96

// advanceWalkAnims cycles each marching soldier's walk frame while it is
// ACTIVELY sliding (live in g.orders); a paused/queued route rests on the
// standing frame. -emulate soldier forces every soldier via walkAnimAll.
// C++ ported : unitinfo.cpp - UnitInfo::Move advances image_index+8 only while driven.
func (s *Screen) advanceWalkAnims() {
	moving := s.activeMovers()

	for _, u := range s.game.Units.Units() {
		if !s.walkAnimAll && !moving[u] { // rest on the standing frame (a no op for non soldiers)
			u.WalkFrame = 0
			continue
		}

		if cnt := s.sprites.WalkPhaseCount(u.Type); cnt > 0 { // actively marching or the demo view
			u.AdvanceWalk(cnt)
		}
	}
}

// activeMovers is the set of units whose move order is live in g.orders (being
// ticked / sliding this frame). A unit with only a queued route (g.pending, out
// of move points) is NOT here, so its soldier walk cycle stops.
func (s *Screen) activeMovers() map[*game.UnitInfo]bool {
	orders := s.game.MoveOrders()

	if len(orders) == 0 {
		return nil
	}

	moving := make(map[*game.UnitInfo]bool, len(orders))

	for _, o := range orders {
		moving[o.Unit] = true
	}

	return moving
}
