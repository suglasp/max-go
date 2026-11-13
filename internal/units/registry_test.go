// registry_test.go : testing unit for registry.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package units

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

func TestMapHashAddAtRemove(t *testing.T) {
	h := NewMapHash()
	a := game.NewUnitInfo(0, 0, 1, 0)
	b := game.NewUnitInfo(0, 1, 2, 0)
	a.SetPosition(4, 4)
	b.SetPosition(4, 4)
	h.Add(a)
	h.Add(b)

	if got := h.At(4, 4); len(got) != 2 {
		t.Fatalf("cell (4,4) has %d units, want 2", len(got))
	}
	if got := h.At(0, 0); got != nil {
		t.Errorf("empty cell returned %d units, want nil", len(got))
	}

	h.Remove(a)
	got := h.At(4, 4)
	if len(got) != 1 || got[0] != b {
		t.Fatalf("after removing a, cell = %v, want [b]", got)
	}
	h.Remove(b)
	if got := h.At(4, 4); got != nil {
		t.Errorf("cleared cell still has %d units", len(got))
	}
}

// TestMapHashBuildingFootprint checks a building is indexed at all four cells
// of its 2x2 footprint, so a unit query at any of them finds it (the collision
// that let buildings overlap on three of four cells).
func TestMapHashBuildingFootprint(t *testing.T) {
	h := NewMapHash()
	b := game.NewUnitInfo(0, 0, 1, 0)
	b.Flags = game.FlagBuilding | game.FlagStationary
	b.SetPosition(5, 5)
	h.Add(b)

	for _, c := range [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		got := h.At(c[0], c[1])
		if len(got) != 1 || got[0] != b {
			t.Errorf("building not indexed at footprint cell (%d,%d): %v", c[0], c[1], got)
		}
	}
	// A cell just outside the footprint must be clear.
	if got := h.At(7, 5); got != nil {
		t.Errorf("cell (7,5) outside the footprint has %d units, want 0", len(got))
	}

	h.Remove(b)
	for _, c := range [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		if got := h.At(c[0], c[1]); got != nil {
			t.Errorf("footprint cell (%d,%d) still occupied after remove: %v", c[0], c[1], got)
		}
	}
}

// TestMapHashBuildSiteFootprint checks that a mobile builder reserves its full
// construction site. Passing units cannot cross the other cells in a 2x2 work
// area.
func TestMapHashBuildSiteFootprint(t *testing.T) {
	h := NewMapHash()
	con := game.NewUnitInfo(0, 0, 1, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit
	con.SetPosition(5, 5) // the builder physically stands at one corner

	// Enter the build pose for a 2x2 site anchored at (5,5), then (re-)bucket it
	// the way StartBuild does.
	con.StartBuildPose(5, 5, 2)
	h.Add(con)

	for _, c := range [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		got := h.At(c[0], c[1])
		if len(got) != 1 || got[0] != con {
			t.Errorf("build site not reserved at cell (%d,%d): %v", c[0], c[1], got)
		}
	}
	if got := h.At(7, 5); got != nil {
		t.Errorf("cell (7,5) outside the site reserved: %v", got)
	}

	// Leaving the pose then bucketing it again collapses it back to the single cell.
	h.Remove(con)
	con.EndBuildPose()
	h.Add(con)
	if got := h.At(5, 5); len(got) != 1 || got[0] != con {
		t.Errorf("after build the builder should occupy only its own cell, got %v", got)
	}
	for _, c := range [][2]int{{6, 5}, {5, 6}, {6, 6}} {
		if got := h.At(c[0], c[1]); got != nil {
			t.Errorf("site cell (%d,%d) still reserved after build: %v", c[0], c[1], got)
		}
	}
}

func TestUnitHashGet(t *testing.T) {
	h := NewUnitHash()
	u := game.NewUnitInfo(0, 0, 42, 0)
	h.Add(u)
	if h.Get(42) != u {
		t.Error("Get(42) did not return the unit")
	}
	if h.Get(7) != nil {
		t.Error("Get of unknown id should be nil")
	}
	h.Remove(u)
	if h.Get(42) != nil {
		t.Error("Get after Remove should be nil")
	}
}

func TestManagerDeployMoveRemove(t *testing.T) {
	m := NewManager()
	u := game.NewUnitInfo(0, 0, 1, 0)
	m.Deploy(u, 2, 3)

	if u.GridX != 2 || u.GridY != 3 {
		t.Fatalf("deployed at %d,%d, want 2,3", u.GridX, u.GridY)
	}
	if m.ByID.Get(1) != u || len(m.Cells.At(2, 3)) != 1 {
		t.Fatal("deploy did not index in both hashes")
	}

	m.Move(u, 5, 6)
	if len(m.Cells.At(2, 3)) != 0 {
		t.Error("old cell still holds the unit after Move")
	}
	if got := m.Cells.At(5, 6); len(got) != 1 || got[0] != u {
		t.Error("new cell missing the unit after Move")
	}

	m.Remove(u)
	if m.ByID.Get(1) != nil || len(m.Cells.At(5, 6)) != 0 {
		t.Error("Remove left the unit indexed")
	}
}

// TestUnitHashAddLogsCollision checks that adding a different unit under a taken id writes a collision line, and re adding the same unit does not.
func TestUnitHashAddLogsCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.log")
	if err := dbg.Init(true, path, 1); err != nil {
		t.Fatal(err)
	}
	defer dbg.Close()

	h := NewUnitHash()
	tank := game.NewUnitInfo(assets.Tank, 0, 65, 0)
	mine := game.NewUnitInfo(assets.LandMine, 1, 65, 0)

	h.Add(tank)
	h.Add(tank) // the same unit again is no collision
	dbg.Flush()
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "unit id collision") {
		t.Fatalf("re adding the same unit logged a collision:\n%s", data)
	}

	h.Add(mine)
	dbg.Flush()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "unit id collision: id 65") {
		t.Errorf("a second unit under id 65 did not log a collision:\n%s", data)
	}
	if h.Get(65) != mine {
		t.Error("Add must still register the newest unit under the id")
	}
}
