// v70body_test.go : testing unit for v70body.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/testutil"
)

// Retail M.A.X. maps are 112x112 cells.
const testMapW, testMapH = 112, 112

func TestReadMissionMap(t *testing.T) {
	dir := testutil.DataDir(t)
	f, err := os.Open(filepath.Join(dir, "SAVE1.SCE"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := ReadMissionMap(f, testMapW, testMapH)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Surface) != testMapW*testMapH || len(m.Cargo) != testMapW*testMapH {
		t.Fatalf("map sizes: surface=%d cargo=%d, want %d", len(m.Surface), len(m.Cargo), testMapW*testMapH)
	}

	// SURFACE_TYPE_* uses 1, 2, 4, or 8. A correctly aligned map is almost entirely valid, while reading from the wrong offset produces mostly random bytes.
	if r := surfaceValidRatio(m.Surface); r < 0.98 {
		t.Fatalf("only %.1f%% of surface cells are valid types (misaligned read?)", r*100)
	}

	// A scenario has some resource deposits, but not one in every cell, and each amount stays within 0..16.
	deposits, gold, fuel, raw := 0, 0, 0, 0
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			kind, amt := m.DepositAt(x, y)
			if kind == CargoNone {
				continue
			}
			if amt < 0 || amt > 16 {
				t.Fatalf("deposit at (%d,%d) amount %d out of range", x, y, amt)
			}
			deposits++
			switch kind {
			case CargoGold:
				gold++
			case CargoFuel:
				fuel++
			case CargoRaw:
				raw++
			}
		}
	}
	if deposits == 0 {
		t.Fatal("no resource deposits decoded (misaligned cargo map?)")
	}
	if deposits >= m.W*m.H {
		t.Fatalf("every cell is a deposit (%d): cargo map misread", deposits)
	}
	t.Logf("SAVE1.SCE: %d deposit cells (gold=%d fuel=%d raw=%d)", deposits, gold, fuel, raw)
}

// TestReadMissionMapAllScenarios checks the 224-byte prefix and surface-map alignment across every retail scenario.
func TestReadMissionMapAllScenarios(t *testing.T) {
	dir := testutil.DataDir(t)
	missions, err := ListScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, mis := range missions {
		f, err := os.Open(mis.SavePath)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(mis.SavePath), err)
			continue
		}
		m, err := ReadMissionMap(f, testMapW, testMapH)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(mis.SavePath), err)
			continue
		}
		if r := surfaceValidRatio(m.Surface); r < 0.98 {
			t.Errorf("%s: only %.1f%% valid surface cells (misaligned)", filepath.Base(mis.SavePath), r*100)
		}
	}
}

// surfaceValidRatio is the fraction of cells holding a SURFACE_TYPE_* bit
// (1/2/4/8): ~1.0 when the surface map is correctly located, ~0.06 when not.
func surfaceValidRatio(surface []uint8) float64 {
	if len(surface) == 0 {
		return 0
	}
	ok := 0
	for _, s := range surface {
		if s == 0x1 || s == 0x2 || s == 0x4 || s == 0x8 {
			ok++
		}
	}
	return float64(ok) / float64(len(surface))
}
