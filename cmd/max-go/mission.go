// mission.go : loads and launches M.A.X. missions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gamescreen"
	"github.com/suglasp/max-go/internal/maxsave"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/winloss"
)

// loadMissionState parses scripted units and the cargo resource map from a V70
// mission save. w and h are the map dimensions because the file does not store
// map_cell_count.
func loadMissionState(savePath string, w, h int, info *missionfile.Info) ([]*game.UnitInfo, *economy.ResourceMap, gamescreen.MissionRules, error) {
	data, err := os.ReadFile(savePath)
	if err != nil {
		return nil, nil, gamescreen.MissionRules{}, err
	}
	gs, err := maxsave.LoadFormatV70(data, w, h)
	if err != nil {
		return nil, nil, gamescreen.MissionRules{}, err
	}
	units := make([]*game.UnitInfo, 0, len(gs.Units))
	pools := make(map[*game.UnitInfo]gamescreen.ComplexPool)
	for _, mu := range gs.Units {
		u := missionUnit(mu)
		units = append(units, u)
		// Carry the unit's saved complex pool so MissionStart can reapply it to the
		// rebuilt complex (else a loaded base's stockpile is lost and factories stall).
		if mu.Complex != nil {
			pools[u] = gamescreen.ComplexPool{
				Material: int(mu.Complex.Material),
				Fuel:     int(mu.Complex.Fuel),
				Gold:     int(mu.Complex.Gold),
			}
		}
	}
	rules := missionRules(gs)
	rules.ComplexPools = pools
	// Carry the mission's scripted clans (game convention 0 random, 1..8 a clan) so
	// each team's designed clan bonuses apply instead of a random draw.
	for team := 0; team < game.TeamCount && team < len(info.TeamClan); team++ {
		rules.TeamClan[team] = int(info.TeamClan[team])
	}
	return units, missionResources(data, w, h), rules, nil
}

// missionRules reads match options, team treasuries, research progress and the
// per type gold upgrades from a parsed V70 game state.
func missionRules(gs *maxsave.GameState) gamescreen.MissionRules {
	opt := gs.Options
	vt := winloss.VictoryDuration
	if opt.VictoryType == 1 { // VICTORY_TYPE_SCORE
		vt = winloss.VictoryScore
	}
	rules := gamescreen.MissionRules{
		VictoryType:  vt,
		VictoryLimit: opt.VictoryLimit,
		TurnTimer:    opt.Timer,
	}
	for team := 0; team < game.TeamCount && team < len(gs.Gold); team++ {
		rules.TeamGold[team] = gs.Gold[team]
		rules.TeamUpgradeGold[team] = gs.UpgradeGoldSpent[team]
		for topic := 0; topic < game.ResearchTopicCount && topic < len(gs.Research[team]); topic++ {
			rules.TeamResearch[team][topic] = game.ResearchTopic{
				Level:           gs.Research[team][topic].Level,
				TurnsToComplete: gs.Research[team][topic].TurnsToComplete,
				Allocation:      gs.Research[team][topic].Allocation,
			}
		}
		rules.TeamUpgrades[team] = missionUpgradeOffsets(gs, team)
	}
	return rules
}

// missionUpgradeOffsets separates gold upgrades from a file's per-type value table. researchValues derives the research portion at runtime, so only the remainder becomes a TeamInfo offset.
func missionUpgradeOffsets(gs *maxsave.GameState, team int) map[assets.UnitType][game.AttribCount]int {
	var out map[assets.UnitType][game.AttribCount]int

	for t := 0; t < len(gs.CurrentValues[team]); t++ {
		base, cur := gs.BaseValues[team][t], gs.CurrentValues[team][t]
		if base == nil || cur == nil {
			continue
		}

		var off [game.AttribCount]int
		changed := false
		diff := func(a game.Attrib, curV, baseV uint16) {
			topic := int(a) // research topics 0..7 share the first eight attrib indices
			if topic >= game.ResearchTopicCount {
				topic = -1 // ammo has no research topic
			}
			level := 0
			if topic >= 0 {
				level = gs.Research[team][topic].Level
			}
			if d := int(curV) - researchedValue(a, int(baseV), level); d != 0 {
				off[a] = d
				changed = true
			}
		}
		diff(game.AttribAttack, cur.Attack, base.Attack)
		diff(game.AttribRounds, cur.Rounds, base.Rounds)
		diff(game.AttribRange, cur.Range, base.Range)
		diff(game.AttribArmor, cur.Armor, base.Armor)
		diff(game.AttribHits, cur.Hits, base.Hits)
		diff(game.AttribSpeed, cur.Speed, base.Speed)
		diff(game.AttribScan, cur.Scan, base.Scan)
		diff(game.AttribTurns, cur.Turns, base.Turns)
		diff(game.AttribAmmo, cur.Ammo, base.Ammo)

		if changed {
			if out == nil {
				out = map[assets.UnitType][game.AttribCount]int{}
			}
			out[assets.UnitType(t)] = off
		}
	}

	return out
}

// researchedValue applies the port's research formula to a file baseline value
// (the same math as gamescreen's researchValues): combat topics gain
// level*base/10, the Cost topic reduces build turns.
func researchedValue(a game.Attrib, base, level int) int {
	if level <= 0 {
		return base
	}

	if a == game.AttribTurns {
		turns := ((level+10)/2 + base*10) / (level + 10)
		if turns < 1 {
			turns = 1
		}
		return turns
	}

	return base + level*base/10
}

// emulateMission parses and deploys a mission through MissionStart. It falls
// back to a header only landing if the V70 body fails to parse, and teamTypesFn
// assigns the controllers for the mission type.
func emulateMission(ctx *menus.Context, m missionfile.Mission, teamTypesFn func(*missionfile.Info) []game.TeamType) error {
	world := int(m.Info.WorldIndex)
	name, err := ctx.Lib.ReadString(menus.WorldTag(world))
	if err != nil {
		return err
	}
	scr, err := buildGameScreen(ctx, strings.TrimSpace(name))
	if err != nil {
		return err
	}

	// The mission's kind comes from the file it was loaded from (.TRA / .SCE / .CAM /
	// .MPS), so a -mission launch of a training file behaves like one: locked AI tier,
	// trimmed build roster, the build prompt voice.
	category := missionCategoryOf(m.SavePath)
	scr.SetMissionCategory(category)
	scr.SetMissionIndex(int(m.Info.MissionIndex)) // its scripted objective is keyed on this
	scr.SetAIDifficulty(missionDifficulty(category, ctx.Settings.Game.Opponent))
	// Mirror the menu launch path (launchMission): the saved play mode + the End
	// Turn grace ride a direct -mission launch too, so -simultane and game.end_turn
	// are not silently dropped.
	scr.SetPlayMode(ctx.Settings.Game.PlayMode)
	scr.SetEndTurnTimer(ctx.Settings.Game.EndTurn)

	seed := int64(m.Info.RNGSeed)
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	w, h := scr.MapSize()
	if units, res, rules, err := loadMissionState(m.SavePath, w, h, m.Info); err == nil {
		dbg.Printf("emulate mission %q: world=%d units=%d victory=%d/%d teams=%v (scripted)", m.Title, world, len(units), rules.VictoryType, rules.VictoryLimit, teamTypesFn(m.Info))
		scr.MissionStart(units, teamTypesFn(m.Info), res, rules, seed)
	} else {
		dbg.Printf("emulate mission %q: world=%d (header-only, body parse failed: %v)", m.Title, world, err)
		scr.CustomStart(nil, nil, 200, seed) // header only fallback: a modest default gold reserve
	}
	scr.SetMissionGoal(m.Title, m.Description) // light the GOAL button with the mission briefing

	return runGameOverAfter(ctx, scr)
}

// missionCategoryOf reads a mission's kind off the file it lives in: the retail
// formats are one per category (.TRA training, .SCE scenario, .CAM campaign, .MPS
// multiplayer scenario, .DMO demo). The menu flows know their own category and pass it
// directly; this is for the paths that only have a path (-mission, the demo player).
func missionCategoryOf(path string) game.MissionCategory {
	switch strings.ToUpper(filepath.Ext(path)) {
	case ".TRA":
		return game.MissionTraining

	case ".SCE":
		return game.MissionScenario

	case ".CAM":
		return game.MissionCampaign

	case ".MPS":
		return game.MissionMultiScenario

	case ".DMO":
		return game.MissionDemo

	default:
		return game.MissionNone
	}
}

// runGameOverAfter runs the game screen and, once a win or loss ends it, shows the
// end of game score screen. An ESC to menu abandon leaves the outcome at Continue and
// returns without a score screen. Shared by the launch paths that own their own return.
// C++ ported : menu.cpp - menu_wrap_up_game after scr.Run.
func runGameOverAfter(ctx *menus.Context, scr *gamescreen.Screen) error {
	if err := scr.Run(); err != nil {
		return err
	}

	if o := scr.Outcome(); o != winloss.Continue {
		return menus.RunGameOver(ctx, scr.GameOverData())
	}

	return nil
}

// launchMissionFile resolves a -mission path or data directory name, decodes
// its header, and launches it with the controller mapping for its type.
func launchMissionFile(ctx *menus.Context, ref string) error {
	path := ref
	if _, err := os.Stat(path); err != nil {
		path = resfile.DataPath(ctx.DataDir, ref) // not a direct path: look in the data dir
	}
	m, err := missionfile.LoadMission(path)
	if err != nil {
		return fmt.Errorf("-mission %q: %w", ref, err)
	}
	teamTypes := missionTeamTypes
	if strings.EqualFold(filepath.Ext(path), ".MPS") {
		teamTypes = missionTeamTypesSolo
	}
	return emulateMission(ctx, m, teamTypes)
}

// launchSaveSlot loads a saved game directly for the -save flag: it restores the
// numbered save slot (the same slot path the main menu Load Game uses), bypassing
// the file picker.
func launchSaveSlot(ctx *menus.Context, slotRef string) error {
	slot, err := strconv.Atoi(strings.TrimSpace(slotRef))
	if err != nil {
		return fmt.Errorf("-save: slot must be a number, got %q", slotRef)
	}
	baseDir := ctx.Plat.BaseDir()
	mapFile, err := loadedSaveMap(baseDir, slot)
	if err != nil {
		return fmt.Errorf("-save %d: %w", slot, err)
	}
	scr, err := buildGameScreen(ctx, mapFile)
	if err != nil {
		return err
	}
	if err := scr.Load(slot); err != nil {
		return fmt.Errorf("-save %d: %w", slot, err)
	}
	startGameMusic(ctx, gameMusicForMap(mapFile))

	return runGameOverAfter(ctx, scr)
}

// missionUnit converts one parsed save body unit into a live game.UnitInfo,
// keeping its type, team, position, facing, hits, speed and order. Mirrors
// saveload.restoreUnit (the file Flags/Orders/State share the original encoding).
func missionUnit(mu *maxsave.Unit) *game.UnitInfo {
	u := game.NewUnitInfo(assets.UnitType(mu.Type), int(mu.Team), int(mu.ID), mu.Angle)
	u.Flags = game.Flags(mu.Flags)
	u.GridX, u.GridY = int(mu.GridX), int(mu.GridY)
	u.Order = game.Order(mu.Orders)
	u.State = game.OrderState(mu.State)
	u.Angle = mu.Angle
	u.Hits = int(mu.Hits)
	u.Speed = int(mu.Speed)
	u.Storage = int(mu.Storage) // the loaded cargo (a builder's raw, a truck's supply, a mine layer's mines)
	// Restore scripted mining values and deposit caps. Normal building placement derives them from the map, but mission units arrive with their saved state.
	u.TotalMining = int(mu.TotalMining)
	u.RawMining, u.FuelMining, u.GoldMining = int(mu.RawMining), int(mu.FuelMining), int(mu.GoldMining)
	u.RawMiningMax, u.FuelMiningMax, u.GoldMiningMax = int(mu.RawMiningMax), int(mu.FuelMiningMax), int(mu.GoldMiningMax)
	// Carry the factory production state so a scripted factory loads producing (the
	// build_list + rate + repeat from the file). MissionStart calls SetFactoryQueue
	// to power the factory on and start the head. The original clamps a 0 build_rate to 1.
	if len(mu.BuildList) > 0 {
		u.BuildQueue = make([]assets.UnitType, len(mu.BuildList))
		for i, id := range mu.BuildList {
			u.BuildQueue[i] = assets.UnitType(id)
		}
		u.RepeatBuild = mu.RepeatBuild != 0
		u.BuildRate = int(mu.BuildRate)
		if u.BuildRate < 1 {
			u.BuildRate = 1
		}
	}
	return u
}

// missionResources decodes the mission's cargo map into a live ResourceMap, so
// mining stations land on their scripted deposits. Returns an empty map if the
// cargo layer can't be read (the mission is still playable).
func missionResources(data []byte, w, h int) *economy.ResourceMap {
	res := economy.NewResourceMap(w, h)
	mm, err := missionfile.ReadMissionMap(bytes.NewReader(data), w, h)
	if err != nil {
		dbg.Printf("mission cargo map: %v", err)
		return res
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if kind, amt := mm.DepositAt(x, y); kind != missionfile.CargoNone {
				res.SetDeposit(x, y, cargoKindToType(kind), amt)
			}
		}
	}
	return res
}

// cargoKindToType maps a mission file cargo kind to the economy resource type.
func cargoKindToType(k missionfile.CargoKind) economy.CargoType {
	switch k {
	case missionfile.CargoFuel:
		return economy.CargoFuel
	case missionfile.CargoGold:
		return economy.CargoGold
	default:
		return economy.CargoRaw
	}
}

// missionTeamTypes maps retail TEAM_TYPE values to playable controllers. Remote
// teams use computer control because this port has no networking.
func missionTeamTypes(info *missionfile.Info) []game.TeamType {
	types := make([]game.TeamType, game.TeamCount)
	for i := 0; i < game.TeamCount && i < len(info.TeamType); i++ {
		switch info.TeamType[i] {
		case 1: // TEAM_TYPE_PLAYER
			types[i] = game.TeamPlayer
		case 2, 3: // TEAM_TYPE_COMPUTER / TEAM_TYPE_REMOTE (no net -> computer)
			types[i] = game.TeamComputer
		default:
			types[i] = game.TeamNone
		}
	}
	return types
}

// missionTeamTypesSolo assigns the first active MPS team to the local player and
// the rest to the computer because retail MPS files mark every team as human.
// C++: GameSetupMenu_Menu(MISSION_CATEGORY_MULTI_PLAYER_SCENARIO, is_single_player=true).
func missionTeamTypesSolo(info *missionfile.Info) []game.TeamType {
	types := make([]game.TeamType, game.TeamCount)
	humanSeen := false
	for i := 0; i < game.TeamCount && i < len(info.TeamType); i++ {
		switch info.TeamType[i] {
		case 1, 2, 3: // any active team
			if !humanSeen {
				types[i] = game.TeamPlayer // the local player takes the first active team
				humanSeen = true
			} else {
				types[i] = game.TeamComputer
			}
		default:
			types[i] = game.TeamNone
		}
	}
	return types
}

// demoTeamTypes gives every active team computer control because attract mode
// has no local player. C++: GameManager_DemoMode in game_manager.cpp.
func demoTeamTypes(info *missionfile.Info) []game.TeamType {
	types := make([]game.TeamType, game.TeamCount)
	for i := 0; i < game.TeamCount && i < len(info.TeamType); i++ {
		switch info.TeamType[i] {
		case 1, 2, 3: // player / computer / remote: all AI driven in a demo
			types[i] = game.TeamComputer
		default:
			types[i] = game.TeamNone
		}
	}
	return types
}

// runAttractDemo rotates through SAVE<N>.DMO files while the menu is idle. Input
// returns to the menu, demoTurnLimit caps the match, and invalid demos return
// quietly. C++: menu.cpp play_attract_demo.
func runAttractDemo(ctx *menus.Context, idx *int, noMovie bool) error {
	demos, err := missionfile.ListDemos(ctx.DataDir)
	if err != nil || len(demos) == 0 {
		dbg.Printf("attract demo: none available (%v)", err)
		return nil
	}

	m := demos[*idx%len(demos)]
	*idx++

	world := int(m.Info.WorldIndex)
	name, err := ctx.Lib.ReadString(menus.WorldTag(world))
	if err != nil {
		return err
	}

	scr, err := buildGameScreen(ctx, strings.TrimSpace(name))
	if err != nil {
		return err
	}

	scr.SetNoMovie(noMovie)
	// Turn based regardless of the configured play mode: the per team turn
	// cycle IS the attract show (the original plays the save's own ini options).
	scr.SetPlayMode(int(game.PlayTurnBased))

	seed := int64(m.Info.RNGSeed)
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	w, h := scr.MapSize()
	units, res, rules, err := loadMissionState(m.SavePath, w, h, m.Info)
	if err != nil {
		// An unattended demo never falls back to an interactive landing.
		dbg.Printf("attract demo %q: body parse failed: %v", m.Title, err)
		return nil
	}

	dbg.Printf("attract demo %q: world=%d units=%d", m.Title, world, len(units))
	scr.MissionStart(units, demoTeamTypes(m.Info), res, rules, seed)
	scr.SetDemoMode(true) // after MissionStart, so the turn cap measures from the loaded counter
	startGameMusic(ctx, gameMusicTag(world))

	// The outcome is ignored: win, lose or timeout, a demo just returns to the
	// menu (no game over screen, no mission goal).
	return scr.Run()
}
