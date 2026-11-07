// zonewalker.go : M.A.X. AI zone walker
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/game"
)

// zoneWalker enumerates every map cell inside the Euclidean disc of a given radius
// around a centre point, row by row, clamped to the map bounds. It is the iterator
// the AI uses to sweep the area where it last saw a target it has lost. The first
// cell is ready right after construction; findNext advances to the next, returning
// false when the disc is exhausted.
//
// C++ ported : zonewalker.cpp - ZoneWalker (InitXY seeds the top row, InitX finds the
// leftmost in disc x of the current row and mirrors it as the right limit, FindNext
// walks x then steps down a row).
type zoneWalker struct {
	startX, startY int
	radius         int
	distance       int // radius squared, the disc test threshold
	x, y           int // the current cell
	limitX, limitY int
	mapW           int
}

// newZoneWalker positions the walker at the first cell of the disc.
func newZoneWalker(center game.Point, radius, mapW, mapH int) *zoneWalker {
	w := &zoneWalker{
		startX:   center.X,
		startY:   center.Y,
		radius:   radius,
		distance: radius * radius,
		mapW:     mapW,
	}

	w.limitY = w.startY + radius
	if w.limitY > mapH-1 {
		w.limitY = mapH - 1
	}

	w.initXY()

	return w
}

// initXY seeds the top row of the disc.
func (w *zoneWalker) initXY() {
	w.y = w.startY - w.radius

	if w.y < 0 {
		w.y = 0
	}

	w.initX()
}

// initX finds the leftmost x of the current row that falls inside the disc and the
// mirrored right limit, clamped to the map.
func (w *zoneWalker) initX() {
	distanceY := (w.y - w.startY) * (w.y - w.startY)
	ix := w.startX - w.radius

	for ; ix <= w.startX; ix++ {
		distanceX := (ix - w.startX) * (ix - w.startX)

		if distanceX+distanceY <= w.distance {
			break
		}
	}

	w.x = ix
	w.limitX = w.startX*2 - ix
	if w.x < 0 {
		w.x = 0
	}

	if w.limitX > w.mapW-1 {
		w.limitX = w.mapW - 1
	}
}

// findNext steps to the next disc cell, returning false once the disc is exhausted.
func (w *zoneWalker) findNext() bool {
	w.x++
	if w.x <= w.limitX {
		return true
	}

	w.y++
	if w.y <= w.limitY {
		w.initX()
		return true
	}

	return false
}
