// loadformat_test.go : testing unit for loadformat.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package maxsave

import (
	"os"
	"path/filepath"
	"testing"
)

// Retail M.A.X. maps are 112x112 cells.
const mapW, mapH = 112, 112

func dataDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("MAX_DATA_DIR")
	if dir == "" {
		t.Skip("game data not found (set MAX_DATA_DIR)")
	}
	return dir
}

// loadSave reads and parses one retail save file.
func loadSave(t *testing.T, path string) *GameState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gs, err := LoadFormatV70(data, mapW, mapH)
	if err != nil {
		t.Fatalf("%s: %v", filepath.Base(path), err)
	}
	return gs
}

func TestLoadCampaign1Units(t *testing.T) {
	dir := dataDir(t)
	gs := loadSave(t, filepath.Join(dir, "SAVE1.CAM"))

	if len(gs.Units) == 0 {
		t.Fatal("no units parsed")
	}
	// Misaligned unit records quickly produce values outside these ranges.
	teams := map[uint8]int{}
	for i, u := range gs.Units {
		if u.Type >= unitEnd {
			t.Fatalf("unit[%d] type %d >= %d (misaligned?)", i, u.Type, unitEnd)
		}
		if u.GridX < 0 || u.GridX >= mapW || u.GridY < 0 || u.GridY >= mapH {
			t.Fatalf("unit[%d] grid (%d,%d) off-map (misaligned?)", i, u.GridX, u.GridY)
		}
		if u.Team > 4 {
			t.Fatalf("unit[%d] team %d invalid (misaligned?)", i, u.Team)
		}
		teams[u.Team]++
	}
	t.Logf("SAVE1.CAM: %d units across teams %v", len(gs.Units), teams)
}

// TestLoadOptions checks the options block against the ranges used by retail scenarios, catching an offset error before it shifts later records.
func TestLoadOptions(t *testing.T) {
	dir := dataDir(t)
	gs := loadSave(t, filepath.Join(dir, "SAVE1.SCE"))
	o := gs.Options
	if o.VictoryType != 0 && o.VictoryType != 1 {
		t.Errorf("victory type %d not duration/score (misaligned?)", o.VictoryType)
	}
	if o.VictoryLimit <= 0 || o.VictoryLimit > 1000 {
		t.Errorf("victory limit %d out of range (misaligned?)", o.VictoryLimit)
	}
	if o.Timer < 0 || o.Timer > 3600 {
		t.Errorf("turn timer %d out of range (misaligned?)", o.Timer)
	}
	if o.StartGold < 0 || o.StartGold > 100000 {
		t.Errorf("start gold %d out of range (misaligned?)", o.StartGold)
	}
	t.Logf("SAVE1.SCE options: %+v", o)

	// Gold and research values also expose a misaligned team header.
	for team, g := range gs.Gold {
		if g < 0 || g > 1_000_000 {
			t.Errorf("team %d gold %d out of range (misaligned?)", team, g)
		}
		for topic, rs := range gs.Research[team] {
			if rs.Level < 0 || rs.Level > 1000 {
				t.Errorf("team %d research topic %d level %d out of range (misaligned?)", team, topic, rs.Level)
			}
			if rs.Allocation < 0 || rs.Allocation > 1000 || rs.TurnsToComplete < 0 {
				t.Errorf("team %d research topic %d remainder %+v out of range (misaligned?)", team, topic, rs)
			}
		}
		if gs.UpgradeGoldSpent[team] < 0 || gs.UpgradeGoldSpent[team] > 100000 {
			t.Errorf("team %d upgrade gold spent %d out of range (misaligned?)", team, gs.UpgradeGoldSpent[team])
		}
	}
	t.Logf("SAVE1.SCE gold=%v research=%v", gs.Gold, gs.Research)
}

// TestLoadAllMissions checks record alignment from the header through the unit lists across every retail campaign, scenario, training, and multiplayer save.
func TestLoadAllMissions(t *testing.T) {
	dir := dataDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	saves := 0
	for _, e := range entries {
		name := e.Name()
		if len(name) < 5 || name[:4] != "SAVE" {
			continue
		}
		ext := filepath.Ext(name)
		if ext != ".CAM" && ext != ".SCE" && ext != ".TRA" && ext != ".MPS" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			gs := loadSave(t, filepath.Join(dir, name))
			if len(gs.Units) == 0 {
				t.Fatal("no units")
			}
			for i, u := range gs.Units {
				if u.Type >= unitEnd || u.Team > 4 ||
					u.GridX < 0 || u.GridX >= mapW || u.GridY < 0 || u.GridY >= mapH {
					t.Fatalf("unit[%d] implausible: type=%d team=%d grid=(%d,%d)", i, u.Type, u.Team, u.GridX, u.GridY)
				}
			}
		})
		saves++
	}
	if saves == 0 {
		t.Fatal("no save files found")
	}
}
