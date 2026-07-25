// bench_test.go : pathfinding benchmark tests for paths.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package paths

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// These benchmarks cover uncapped A* searches on a 112x112 retail-size grid: an unreachable flood, a long open route, and a short nearby route.

const benchW, benchH = 112, 112

// openCost is a flat all passable map (every cell costs 1).
func openCost(x, y int) int { return 1 }

// walledCost divides the grid with an impassable column, forcing a failed corner-to-corner search to flood the reachable half.
func walledCost(x, y int) int {
	if x == benchW/2 {
		return 0 // impassable wall column
	}
	return 1
}

// BenchmarkFindUnreachable measures the open-grid corner-to-corner baseline retained under its original benchmark name.
func BenchmarkFindUnreachable(b *testing.B) {
	start := game.Point{X: 0, Y: 0}
	dest := game.Point{X: benchW - 1, Y: benchH - 1} // across the wall: unreachable
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := Find(openCost, benchW, benchH, start, dest, 0); !ok {
			b.Fatal("openCost dest should be reachable")
		}
	}
}

// BenchmarkFindUnreachableWalled measures a failed search that expands the entire reachable half of the grid.
func BenchmarkFindUnreachableWalled(b *testing.B) {
	start := game.Point{X: 0, Y: 0}
	dest := game.Point{X: benchW - 1, Y: benchH - 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := Find(walledCost, benchW, benchH, start, dest, 0); ok {
			b.Fatal("walled dest must be unreachable from start")
		}
	}
}

// BenchmarkFindLongOpen measures a reachable corner-to-corner route on an open grid.
func BenchmarkFindLongOpen(b *testing.B) {
	start := game.Point{X: 0, Y: 0}
	dest := game.Point{X: benchW - 1, Y: benchH - 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Find(openCost, benchW, benchH, start, dest, 0)
	}
}

// BenchmarkFindShort measures a nearby route for comparison.
func BenchmarkFindShort(b *testing.B) {
	start := game.Point{X: 10, Y: 10}
	dest := game.Point{X: 16, Y: 14}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Find(openCost, benchW, benchH, start, dest, 0)
	}
}
