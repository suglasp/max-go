// mining_test.go : checks that a mission's mining stations load with their scripted extraction.
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

// TestMissionMiningStationsCarryTheirAllocation checks that V70 mining bytes restore raw, fuel, gold, and deposit-cap allocations for mission stations.
func TestMissionMiningStationsCarryTheirAllocation(t *testing.T) {
	dir := os.Getenv("MAX_DATA_DIR")
	if dir == "" {
		t.Skip("MAX_DATA_DIR not set")
	}

	// SAVE1.TRA is Mining and Storage - the lesson that is ABOUT the mining station.
	data, err := os.ReadFile(filepath.Join(dir, "SAVE1.TRA"))
	if err != nil {
		t.Skipf("SAVE1.TRA not available: %v", err)
	}

	gs, err := LoadFormatV70(data, 112, 112)
	if err != nil {
		t.Fatalf("LoadFormatV70: %v", err)
	}

	const miningStation = 40 // ResourceID MININGST (assets.MiningSt; maxsave stays free of the assets package)

	stations := 0

	for _, u := range gs.Units {
		if u.Type != miningStation {
			continue
		}

		stations++

		if u.RawMining == 0 && u.FuelMining == 0 && u.GoldMining == 0 {
			t.Errorf("mining station %d extracts nothing at all (raw/fuel/gold all 0)", u.ID)
		}

		if u.RawMiningMax == 0 && u.FuelMiningMax == 0 && u.GoldMiningMax == 0 {
			t.Errorf("mining station %d sits on no deposit at all (every cap 0)", u.ID)
		}

		total := int(u.RawMining) + int(u.FuelMining) + int(u.GoldMining)
		if total > 16 {
			t.Errorf("mining station %d extracts %d, over the 16 a station can pull", u.ID, total)
		}

		// The caps must bound what the station is set to pull, or the bytes were read in
		// the wrong order (the file lists the caps as raw, GOLD, fuel - not raw, fuel, gold).
		if int(u.RawMining) > int(u.RawMiningMax) {
			t.Errorf("station %d mines %d raw but its raw deposit caps at %d: the mining bytes are misread",
				u.ID, u.RawMining, u.RawMiningMax)
		}

		if int(u.FuelMining) > int(u.FuelMiningMax) {
			t.Errorf("station %d mines %d fuel but its fuel deposit caps at %d: the mining bytes are misread",
				u.ID, u.FuelMining, u.FuelMiningMax)
		}

		if int(u.GoldMining) > int(u.GoldMiningMax) {
			t.Errorf("station %d mines %d gold but its gold deposit caps at %d: the mining bytes are misread",
				u.ID, u.GoldMining, u.GoldMiningMax)
		}
	}

	if stations == 0 {
		t.Fatal("the Mining and Storage lesson has no mining station in it")
	}
}
