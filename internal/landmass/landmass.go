// landmass.go : divides map land cells into connected components for AI navigation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package landmass finds connected components of world-map land cells.
package landmass

import (
	"slices"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// Component is one connected landmass.
type Component struct {
	/* // note: unused code in Go port
	// ID and the bounding box were only read by tests; callers use the slice index (Find returns the largest first).
	ID                     int
	*/
	Cells []game.Point
	/* // note: unused code in Go port
	MinX, MinY, MaxX, MaxY int
	*/
}

// Size is the number of land cells in the component.
func (c *Component) Size() int {
	return len(c.Cells)
}

// Find flood fills m's land into components connected on the 4 sides, largest first.
func Find(m *world.Map) []Component {
	if m == nil || m.W <= 0 || m.H <= 0 {
		return nil
	}

	total := m.W * m.H
	visited := make([]bool, total)
	queue := make([]int, 0, total)
	var comps []Component

	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			start := y*m.W + x
			if visited[start] || !isLand(m.SurfaceAt(x, y)) {
				continue
			}

			visited[start] = true
			queue = append(queue[:0], start)
			cells := make([]int, 0, 64)
			/* // note: unused code in Go port
			// the bounding box of the component (see the Component fields).
			minX, maxX := x, x
			minY, maxY := y, y
			*/

			for head := 0; head < len(queue); head++ {
				idx := queue[head]
				cells = append(cells, idx)
				cx := idx % m.W
				cy := idx / m.W
				/* // note: unused code in Go port
				if cx < minX {
					minX = cx
				}
				if cx > maxX {
					maxX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cy > maxY {
					maxY = cy
				}
				*/
				if cx > 0 {
					addIfLand(m, visited, &queue, idx-1, cx-1, cy)
				}
				if cx+1 < m.W {
					addIfLand(m, visited, &queue, idx+1, cx+1, cy)
				}
				if cy > 0 {
					addIfLand(m, visited, &queue, idx-m.W, cx, cy-1)
				}
				if cy+1 < m.H {
					addIfLand(m, visited, &queue, idx+m.W, cx, cy+1)
				}
			}

			slices.Sort(cells)
			points := make([]game.Point, len(cells))
			for i, idx := range cells {
				points[i] = game.Point{X: idx % m.W, Y: idx / m.W}
			}

			comps = append(comps, Component{
				Cells: points,
				/* // note: unused code in Go port
				MinX:  minX, MinY: minY,
				MaxX: maxX, MaxY: maxY,
				*/
			})
		}
	}

	slices.SortStableFunc(comps, func(a, b Component) int { return b.Size() - a.Size() }) // largest first, ties keep scan order
	/* // note: unused code in Go port
	for i := range comps {
		comps[i].ID = i
	}
	*/
	return comps
}

/* // note: unused code in Go port
// LargeEnough is only read by its own test; no caller filters components by size.
// LargeEnough returns components with Size >= minCells, preserving order.
func LargeEnough(comps []Component, minCells int) []Component {
	out := make([]Component, 0, len(comps))
	for _, c := range comps {
		if c.Size() >= minCells {
			out = append(out, c)
		}
	}
	return out
}
*/

// isLand reports whether a surface byte counts as land for component search.
func isLand(surface uint8) bool {
	return surface&(world.SurfaceLand|world.SurfaceCoast) != 0
}

// addIfLand enqueues an unvisited neighboring land tile.
func addIfLand(m *world.Map, visited []bool, queue *[]int, idx, x, y int) {
	if visited[idx] || !isLand(m.SurfaceAt(x, y)) {
		return
	}
	visited[idx] = true
	*queue = append(*queue, idx)
}

/* // note: unused code in Go port
// stableSortBySizeDesc repeated slices.SortStableFunc, which Find now calls.
// stableSortBySizeDesc orders components from largest to smallest.
func stableSortBySizeDesc(comps []Component) {
	for i := 1; i < len(comps); i++ {
		c := comps[i]
		j := i - 1
		for ; j >= 0 && comps[j].Size() < c.Size(); j-- {
			comps[j+1] = comps[j]
		}
		comps[j+1] = c
	}
}
*/

/* // note: unused code in Go port
// sortInts repeated slices.Sort, which Find now calls.
// sortInts sorts values in ascending order.
func sortInts(values []int) {
	if len(values) < 2 {
		return
	}
	type span struct {
		lo, hi int
	}
	stack := []span{{lo: 0, hi: len(values) - 1}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.lo >= s.hi {
			continue
		}
		p := partition(values, s.lo, s.hi)
		if p-s.lo > s.hi-p {
			stack = append(stack, span{lo: s.lo, hi: p - 1}, span{lo: p + 1, hi: s.hi})
		} else {
			stack = append(stack, span{lo: p + 1, hi: s.hi}, span{lo: s.lo, hi: p - 1})
		}
	}
}
*/

/* // note: unused code in Go port
// partition was only called by sortInts.
// partition partitions values for quicksort and returns the pivot index.
func partition(values []int, lo, hi int) int {
	mid := lo + (hi-lo)/2
	if values[mid] < values[lo] {
		values[mid], values[lo] = values[lo], values[mid]
	}
	if values[hi] < values[mid] {
		values[hi], values[mid] = values[mid], values[hi]
	}
	if values[mid] < values[lo] {
		values[mid], values[lo] = values[lo], values[mid]
	}

	pivot := values[mid]
	values[mid], values[hi] = values[hi], values[mid]
	store := lo
	for i := lo; i < hi; i++ {
		if values[i] < pivot {
			values[i], values[store] = values[store], values[i]
			store++
		}
	}
	values[store], values[hi] = values[hi], values[store]
	return store
}
*/
