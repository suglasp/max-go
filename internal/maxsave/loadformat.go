// loadformat.go : the M.A.X. V70 format driver - orchestrates per class FileLoad calls to restore a full game state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : smartfile.cpp - LoadFormatV70 and per-class FileLoad methods
package maxsave

import "fmt"

// LoadFormatV70 reads a retail SmartFile V70 mission or save through its five unit lists. It consumes unneeded fields to preserve alignment and stops before
// the trailing hash tables and heat maps. The map dimensions come from the mission's .WRL file.
//
// C++ ported : saveload.cpp - SaveLoad_LoadFormatV70 (up to the unit lists)
const (
	teamCount          = 4  // PLAYER_TEAM_MAX-1: red, green, blue, gray
	unitEnd            = 93 // UNIT_END (resourcetable.hpp)
	researchTopicCount = 8  // RESEARCH_TOPIC_COUNT (enums.hpp): attack..scan, cost
)

// Options contains the V70 mission rules used by the launcher. LoadFormatV70 consumes the remaining SaveLoad_LoadOptions fields only for alignment.
type Options struct {
	Timer        int // time budget for each turn in seconds (0 = no limit)
	EndTurn      int // forced game end turn (0 = none)
	StartGold    int // each team's starting gold reserve
	PlayMode     int // 0 = turn based, 1 = simultaneous moves
	VictoryType  int // VICTORY_TYPE_* : 0 = duration (turns), 1 = score
	VictoryLimit int // the duration/score threshold (0 = none)
	Opponent     int // AI difficulty
}

// ResearchState is one topic's scripted progress: the reached level plus the mid
// research remainder a save carries (turns still to complete and the research
// centres allocated to the topic).
type ResearchState struct {
	Level, TurnsToComplete, Allocation int
}

// GameState contains the mission rules, team economy, and units needed by the launcher.
type GameState struct {
	Options          Options
	Gold             [teamCount]int                               // TeamUnits.gold by team.
	Research         [teamCount][researchTopicCount]ResearchState // Research progress by team and RESEARCH_TOPIC_* index.
	UpgradeGoldSpent [teamCount]int                               // stats_gold_spent_on_upgrades by team.
	BaseValues       [teamCount][unitEnd]*UnitValues              // per unit type baseline attribute set (index = the file's unit type id).
	CurrentValues    [teamCount][unitEnd]*UnitValues              // per unit type CURRENT set (baseline + research + gold upgrades).
	Units            []*Unit                                      // Units from all five lists, in file order.
}

// LoadFormatV70 parses a complete SAVE*.SCE, SAVE*.CAM, or SAVE*.TRA file.
func LoadFormatV70(data []byte, w, h int) (*GameState, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("maxsave: invalid map size %dx%d", w, h)
	}
	r := NewReader(data, classRegistry())
	if r.Format() != FormatV70 {
		return nil, fmt.Errorf("maxsave: not a V70 file (format %d)", r.Format())
	}

	// Fixed header (170 bytes), see internal/missionfile/savefile.md.
	if v := r.U16(); v != 70 {
		return nil, fmt.Errorf("maxsave: bad version %d", v)
	}
	r.Skip(1)      // save_game_type
	r.Skip(30)     // save_name
	r.Skip(1)      // world_index
	r.Skip(2)      // mission_index
	r.Skip(4 * 30) // team_names[4][30]
	r.Skip(5)      // team_type[5]
	r.Skip(5)      // team_clan[5]
	r.Skip(4)      // rng_seed

	// Options prefix (read by SaveLoadMenu before LoadOptions): opponent u8,
	// turn_timer u16, endturn u16, play_mode i8, duplicated by LoadOptions below.
	r.Skip(1 + 2 + 2 + 1)

	// SaveLoad_LoadOptions: 12 * int32. Capture the mission's scripted rules.
	var opt Options
	r.Skip(4)                  // world
	opt.Timer = int(r.I32())   // timer (per turn seconds)
	opt.EndTurn = int(r.I32()) // endturn
	opt.StartGold = int(r.I32())
	opt.PlayMode = int(r.I32())
	opt.VictoryType = int(r.I32())
	opt.VictoryLimit = int(r.I32())
	opt.Opponent = int(r.I32())
	r.Skip(4 * 4) // raw_resource, fuel_resource, gold_resource, alien_derelicts

	// Terrain surface map + resource (cargo) map.
	r.Skip(w * h)     // surface_map: u8 per cell
	r.Skip(w * h * 2) // cargo_map: u16 per cell

	gs := &GameState{Options: opt}
	for team := 0; team < teamCount; team++ {
		gs.Research[team], gs.UpgradeGoldSpent[team] = readCTInfoV70(r)
	}

	r.Skip(1)     // GameManager_ActiveTurnTeam (u8)
	r.Skip(1)     // GameManager_PlayerTeam (u8)
	r.Skip(4)     // GameManager_TurnCounter (i32)
	r.Skip(2)     // game_state (u16)
	r.Skip(2)     // SaveLoadMenu_TurnTimer (u16)
	r.Skip(7 * 4) // effects, click_scroll, quick_scroll, fast_movement, follow_unit, auto_select, enemy_halt (int32)

	for team := 0; team < teamCount; team++ {
		gs.Gold[team], gs.BaseValues[team], gs.CurrentValues[team] = readTeamUnitsV70(r)
	}

	for list := 0; list < 5; list++ { // GroundCover, MobileLandSea, Stationary, MobileAir, Particle
		for n := r.Count(); n > 0; n-- {
			if u, ok := r.ReadObjectAs(allocUnit).(*Unit); ok && u != nil {
				gs.Units = append(gs.Units, u)
			}
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("maxsave: unit list %d: %w", list, r.Err())
		}
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return gs, nil
}

// readCTInfoV70 consumes one team's CTInfo block (no FileObjects, all inline)
// and returns its per topic research progress plus the gold spent on upgrades
// stat. A save exists to restore state, so the mid research remainder
// (turns_to_complete + allocation) is read, not skipped.
// C++ ported : saveload.cpp - the per-team CTInfo reads in SaveLoad_LoadFormatV70
func readCTInfoV70(r *Reader) ([researchTopicCount]ResearchState, int) {
	r.Skip(10 * 4) // markers Point[10] (i16,i16)
	r.Skip(1)      // team_type (u8)
	r.Skip(1)      // finished_turn (bool)
	r.Skip(1)      // team_clan (u8)
	// research_topics ResearchTopic[8] = {research_level, turns_to_complete,
	// allocation} (3 * i32).
	var research [researchTopicCount]ResearchState
	for i := 0; i < researchTopicCount; i++ {
		research[i].Level = int(r.I32())
		research[i].TurnsToComplete = int(r.I32())
		research[i].Allocation = int(r.I32())
	}
	r.Skip(4)                 // team_points (u32)
	r.Skip(2)                 // number_of_objects_created (u16)
	r.Skip(unitEnd)           // unit_counters u8[93] (V70)
	r.Skip(6 * 2)             // screen_locations ScreenLocation[6] (i8,i8): reads 6, uses 4
	r.Skip(50 * 2)            // score_graph i16[50]
	r.Skip(2)                 // selected_unit_ids (u16)
	r.Skip(2)                 // zoom_level (u16)
	r.Skip(4)                 // camera_position Point (i16,i16)
	r.Skip(11)                // display_button_* (11 * i8)
	r.Skip(4 * 2)             // stats_factories/mines/buildings/units_built (i16)
	r.Skip(unitEnd * 2)       // casualties u16[93] (V70)
	goldSpent := int(r.I16()) // stats_gold_spent_on_upgrades (i16, V70)
	return research, goldSpent
}

// readTeamUnitsV70 consumes one team's TeamUnits table: a gold word, then the
// base and current UnitValues for each unit type (shared objects), then complexes.
// It returns the team's starting gold plus both per type value tables - a file
// that ships pre upgraded types carries the delta in current vs base.
// C++ ported : teamunits.cpp - TeamUnits::FileLoad
func readTeamUnitsV70(r *Reader) (int, [unitEnd]*UnitValues, [unitEnd]*UnitValues) {
	gold := int(r.U16()) // gold (u16, V70)
	var base, current [unitEnd]*UnitValues
	for i := 0; i < unitEnd; i++ {
		base[i], _ = r.ReadObjectAs(allocUnitValues).(*UnitValues)
	}
	for i := 0; i < unitEnd; i++ {
		current[i], _ = r.ReadObjectAs(allocUnitValues).(*UnitValues)
	}
	for n := r.Count(); n > 0; n-- {
		r.ReadObjectAs(allocComplex) // complexes
	}
	return gold, base, current
}
