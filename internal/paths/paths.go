// paths.go : the M.A.X. pathfinding package - finds a unit's optimal movement route using a single pass A* on the 8 way direction grid.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : searcher.cpp - bidirectional A* path search
//
// Package paths converts a start cell, destination, and movement-cost grid into ordered single-cell steps with facing directions and a total cost.
//
// The C++ implementation runs bidirectional A*. This package uses forward A* with an admissible heuristic while preserving its cell-cost model:
// a step costs the entered cell's value, multiplied by 3/2 for diagonals (searcher.cpp, EvaluateCost and ForwardSearch).
package paths

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/game"
)

// Find statistics accumulate call count and elapsed time only while tracing is enabled. Disabled tracing performs one atomic load per call.
var (
	findCalls atomic.Int64
	findNanos atomic.Int64
	statsOn   atomic.Bool
)

// EnableStats enables or disables Find timing.
func EnableStats(on bool) {
	statsOn.Store(on)
}

// StatsSnapshot returns the cumulative Find call count and elapsed nanoseconds. Subtract snapshots to measure a specific interval.
func StatsSnapshot() (calls, nanos int64) {
	return findCalls.Load(), findNanos.Load()
}

// dirOffsets are the 8 movement directions indexed by unit angle: 0 is north
// and the angle increases clockwise, matching the game's facing convention
// (and SetAngle). Odd indices are diagonals.
//
// C++: DIRECTION_OFFSETS (searcher.cpp)
var dirOffsets = [8]game.Point{
	{X: 0, Y: -1},  // 0 N
	{X: 1, Y: -1},  // 1 NE
	{X: 1, Y: 0},   // 2 E
	{X: 1, Y: 1},   // 3 SE
	{X: 0, Y: 1},   // 4 S
	{X: -1, Y: 1},  // 5 SW
	{X: -1, Y: 0},  // 6 W
	{X: -1, Y: -1}, // 7 NW
}

// StepOffset is the delta of one cell for a facing direction (0..7), the
// vector a unit travels for one step. It exposes dirOffsets so movement and
// pathfinding use the same eight direction convention.
//
// C++: DIRECTION_OFFSETS[angle] (searcher.cpp)
func StepOffset(dir int) game.Point {
	return dirOffsets[dir&7]
}

// minStepCost is the cheapest a single orthogonal step can cost (the Reduced
// road/slab class). It bounds the heuristic so the search stays admissible.
const minStepCost = int(access.Reduced)

// CostFunc gives the movement cost of entering cell (x, y): 0 means the cell
// is impassable, any positive value is the cost class (access.Cost values).
//
// C++: the low 5 bits of an AccessMap cell (searcher.cpp EvaluateCost)
type CostFunc func(x, y int) int

// Step is one cell of a path: the cell stepped into and the facing angle
// (0-7) of the move that reached it.
type Step struct {
	To  game.Point
	Dir int
}

// Result is a found path: the destination, the ordered steps from the start
// (exclusive) to the destination (inclusive), and the summed movement cost.
//
// C++: PathResult (searcher.hpp)
type Result struct {
	Dest  game.Point
	Steps []Step
	Cost  int
}

// GridCost builds a CostFunc from an access.Grid for a given moving unit:
// each cell's cost is access.IsAccessible, so impassable terrain and blocked
// cells return 0 and the pathfinder routes around them.
func GridCost(g *access.Grid, d access.UnitDesc, team int, mod access.Modifier) CostFunc {
	return func(x, y int) int { return int(g.IsAccessible(d, team, x, y, mod)) }
}

// stepCost returns the cost of entering cell c from a step in direction dir,
// or 0 if c is impassable. Diagonal steps cost (cost*3)/2.
func stepCost(cost CostFunc, c game.Point, dir int) int {
	v := cost(c.X, c.Y)
	if v == 0 {
		return 0
	}
	if dir&1 != 0 {
		v = (v * 3) / 2
	}
	return v
}

// node is an entry in the open set; the heap orders by f = g + heuristic.
type node struct {
	p game.Point
	f int
}

// nodeHeap is a typed binary min-heap ordered by f. Its pooled backing slice avoids the per-node interface allocations of container/heap.
type nodeHeap []node

// push adds n and sifts it up to restore the min heap order.
func (h *nodeHeap) push(n node) {
	s := append(*h, n)
	i := len(s) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if s[parent].f <= s[i].f {
			break
		}
		s[parent], s[i] = s[i], s[parent]
		i = parent
	}
	*h = s
}

// pop removes and returns the node with the lowest f, sifting the moved last element down.
func (h *nodeHeap) pop() node {
	s := *h
	top := s[0]
	n := len(s) - 1
	s[0] = s[n]
	s = s[:n]
	i := 0
	for {
		left, right, smallest := 2*i+1, 2*i+2, i
		if left < n && s[left].f < s[smallest].f {
			smallest = left
		}
		if right < n && s[right].f < s[smallest].f {
			smallest = right
		}
		if smallest == i {
			break
		}
		s[i], s[smallest] = s[smallest], s[i]
		i = smallest
	}
	*h = s
	return top
}

// heuristic is an admissible estimate of the remaining cost from a to dest:
// the Chebyshev distance times the cheapest step cost. Diagonals never cost
// less than minStepCost, so this never overestimates.
func heuristic(a, dest game.Point) int {
	dx := a.X - dest.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - dest.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx * minStepCost
	}
	return dy * minStepCost
}

// searchBuf stores A* state in flat, pooled slices indexed by y*w+x. A generation stamp invalidates old entries without clearing every cell between searches.
type searchBuf struct {
	gScore  []int    // Best entry cost when seen[i] equals gen.
	cameDir []uint8  // Arrival direction when seen[i] equals gen.
	seen    []uint32 // Per-cell generation stamp.
	gen     uint32   // Current search generation.
	open    nodeHeap // Reused A* open set.
}

// bufPool recycles search buffers between Find calls.
var bufPool sync.Pool

// getBuf returns a buffer for n cells with a fresh generation. It reuses existing slices and clears stamps only when the uint32 generation wraps.
func getBuf(n int) *searchBuf {
	b, _ := bufPool.Get().(*searchBuf)
	if b == nil {
		b = &searchBuf{}
	}
	if cap(b.gScore) < n {
		b.gScore = make([]int, n)
		b.cameDir = make([]uint8, n)
		b.seen = make([]uint32, n)
		b.gen = 0
	} else {
		b.gScore = b.gScore[:n]
		b.cameDir = b.cameDir[:n]
		b.seen = b.seen[:n]
	}
	b.gen++
	if b.gen == 0 { // wrapped: every stamp must be made stale again
		for i := range b.seen {
			b.seen[i] = 0
		}
		b.gen = 1
	}
	b.open = b.open[:0] // empty the open set, keep its backing array
	return b
}

// Find searches for a path at least cost from start to dest on a wxh grid whose
// entry cost per cell comes from cost. maxCost caps the accepted path cost
// (0 means unlimited). It returns the path and true on success, or nil and
// false when start/dest are off the grid, dest is impassable, no route exists, or
// the cheapest route exceeds maxCost.
//
// C++: PathSearchContext::RunSearch (searcher.hpp) over Searcher (searcher.cpp)
func Find(cost CostFunc, w, h int, start, dest game.Point, maxCost int) (*Result, bool) {
	if statsOn.Load() {
		findCalls.Add(1)
		t0 := time.Now()
		defer func() { findNanos.Add(int64(time.Since(t0))) }()
	}
	inBounds := func(p game.Point) bool { return p.X >= 0 && p.X < w && p.Y >= 0 && p.Y < h }
	if !inBounds(start) || !inBounds(dest) {
		return nil, false
	}
	if start.Equal(dest) {
		return &Result{Dest: dest}, true
	}
	// The destination itself must be enterable.
	if cost(dest.X, dest.Y) == 0 {
		return nil, false
	}

	b := getBuf(w * h)
	defer bufPool.Put(b)
	si := start.Y*w + start.X
	b.gScore[si] = 0
	b.seen[si] = b.gen

	b.open.push(node{p: start, f: heuristic(start, dest)})

	for len(b.open) > 0 {
		cur := b.open.pop().p
		if cur.Equal(dest) {
			di := dest.Y*w + dest.X
			if maxCost > 0 && b.gScore[di] > maxCost {
				return nil, false
			}
			return reconstruct(start, dest, b, w, b.gScore[di]), true
		}
		cg := b.gScore[cur.Y*w+cur.X]
		for dir := 0; dir < 8; dir++ {
			nb := cur.Add(dirOffsets[dir])
			if !inBounds(nb) {
				continue
			}
			sc := stepCost(cost, nb, dir)
			if sc == 0 {
				continue
			}
			tentative := cg + sc
			ni := nb.Y*w + nb.X
			if b.seen[ni] == b.gen && tentative >= b.gScore[ni] {
				continue
			}
			b.gScore[ni] = tentative
			b.cameDir[ni] = uint8(dir)
			b.seen[ni] = b.gen
			b.open.push(node{p: nb, f: tentative + heuristic(nb, dest)})
		}
	}
	return nil, false
}

// reconstruct walks the direction pointers back from dest to start and emits
// the steps in forward order. cameDir lives in the search buffer indexed by grid position.
//
// C++: Searcher::DeterminePath (searcher.cpp)
func reconstruct(start, dest game.Point, b *searchBuf, w, total int) *Result {
	var rev []Step
	cur := dest
	for !cur.Equal(start) {
		dir := int(b.cameDir[cur.Y*w+cur.X])
		rev = append(rev, Step{To: cur, Dir: dir})
		cur = cur.Sub(dirOffsets[dir])
	}
	// Reverse into forward order.
	steps := make([]Step, len(rev))
	for i, s := range rev {
		steps[len(rev)-1-i] = s
	}
	return &Result{Dest: dest, Steps: steps, Cost: total}
}
