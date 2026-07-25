// setmap_test.go : checks map replacement and scaled tile cache invalidation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package drawmap

import (
	"testing"

	"github.com/suglasp/max-go/internal/world"
)

// TestSetMapSwapsMapAndClearsCache checks that SetMap drops cached tiles before
// rendering the replacement world.
func TestSetMapSwapsMapAndClearsCache(t *testing.T) {
	first := &world.Map{}
	first.W, first.H = 30, 30
	r := New(first)
	r.SetZoom(8)
	r.cache[1] = nil // pretend a tile was cached for the old world

	second := &world.Map{}
	second.W, second.H = 40, 40
	r.SetMap(second)

	if r.m != second {
		t.Error("SetMap did not swap the map")
	}
	if len(r.cache) != 0 {
		t.Errorf("SetMap left %d cache entries, want 0", len(r.cache))
	}
}
