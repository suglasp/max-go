// landmass_test.go : tests connected land components and size filtering.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package landmass

import (
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

func testMap(w, h int, rows ...string) *world.Map {
	surface := make([]uint8, w*h)
	for y, row := range rows {
		for x, r := range row {
			switch r {
			case 'L':
				surface[y*w+x] = world.SurfaceLand
			case 'C':
				surface[y*w+x] = world.SurfaceCoast
			case 'W':
				surface[y*w+x] = world.SurfaceWater
			case 'A':
				surface[y*w+x] = world.SurfaceAir
			}
		}
	}
	return &world.Map{
		Preview: world.Preview{W: w, H: h},
		Surface: surface,
	}
}

func TestFindSyntheticComponents(t *testing.T) {
	m := testMap(6, 4,
		"LLWWWW",
		"LWWWCC",
		"WWWWWW",
		"WWWLLL",
	)

	comps := Find(m)
	if len(comps) != 3 {
		t.Fatalf("component count = %d, want 3", len(comps))
	}

	want := []struct {
		size  int
		cells []game.Point
	}{
		{
			size:  3,
			cells: []game.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}},
		},
		{
			size:  3,
			cells: []game.Point{{X: 3, Y: 3}, {X: 4, Y: 3}, {X: 5, Y: 3}},
		},
		{
			size:  2,
			cells: []game.Point{{X: 4, Y: 1}, {X: 5, Y: 1}},
		},
	}
	for i, w := range want {
		c := comps[i]
		if c.Size() != w.size {
			t.Errorf("component %d size = %d, want %d", i, c.Size(), w.size)
		}
		if len(c.Cells) != len(w.cells) {
			t.Fatalf("component %d cell count = %d, want %d", i, len(c.Cells), len(w.cells))
		}
		for j, wantCell := range w.cells {
			if c.Cells[j] != wantCell {
				t.Errorf("component %d cell %d = %+v, want %+v", i, j, c.Cells[j], wantCell)
			}
		}
	}
}

func TestFindAllWater(t *testing.T) {
	m := testMap(3, 2,
		"WWW",
		"WAW",
	)
	if comps := Find(m); len(comps) != 0 {
		t.Fatalf("component count = %d, want 0", len(comps))
	}
}

func TestFindSolidLand(t *testing.T) {
	m := testMap(3, 2,
		"LLL",
		"LLL",
	)
	comps := Find(m)
	if len(comps) != 1 {
		t.Fatalf("component count = %d, want 1", len(comps))
	}
	if comps[0].Size() != 6 {
		t.Errorf("solid component size = %d, want 6", comps[0].Size())
	}
}

func TestFindUsesFourWayConnectivity(t *testing.T) {
	m := testMap(3, 3,
		"LWW",
		"WWW",
		"WWL",
	)
	if comps := Find(m); len(comps) != 2 {
		t.Fatalf("diagonal land component count = %d, want 2", len(comps))
	}
}

func TestStableSortForEqualSizes(t *testing.T) {
	m := testMap(5, 1, "LWLWL")
	comps := Find(m)
	if len(comps) != 3 {
		t.Fatalf("component count = %d, want 3", len(comps))
	}
	for i, c := range comps {
		if gotX := c.Cells[0].X; gotX != i*2 {
			t.Errorf("component %d starts at x=%d, want %d", i, gotX, i*2)
		}
	}
}

/* // note: unused code in Go port
// the test of LargeEnough, commented out with it.
func TestLargeEnough(t *testing.T) {
	comps := []Component{
		{ID: 0, Cells: make([]game.Point, 5)},
		{ID: 1, Cells: make([]game.Point, 2)},
		{ID: 2, Cells: make([]game.Point, 4)},
	}
	got := LargeEnough(comps, 4)
	if len(got) != 2 {
		t.Fatalf("LargeEnough count = %d, want 2", len(got))
	}
	if got[0].ID != 0 || got[1].ID != 2 {
		t.Fatalf("LargeEnough IDs = %d,%d, want 0,2", got[0].ID, got[1].ID)
	}
}
*/

func TestFindOnRetailMaps(t *testing.T) {
	dir := testutil.DataDir(t)
	for _, name := range []string{"GREEN_1.WRL", "SNOW_1.WRL"} {
		m, err := world.LoadMap(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		comps := Find(m)
		if len(comps) == 0 {
			t.Fatalf("%s: no components", name)
		}
		total := 0
		for _, c := range comps {
			total += c.Size()
		}
		want := 0
		for y := 0; y < m.H; y++ {
			for x := 0; x < m.W; x++ {
				if isLand(m.SurfaceAt(x, y)) {
					want++
				}
			}
		}
		if total != want {
			t.Errorf("%s component cell total = %d, want %d", name, total, want)
		}
		if comps[0].Size() <= 0 {
			t.Errorf("%s largest component size = %d, want > 0", name, comps[0].Size())
		}
	}
}
