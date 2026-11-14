// missionstart.go : implements the mission start handler.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// MissionRules are the scripted match settings a mission carries in its save
// body (parsed by internal/maxsave from SaveLoad_LoadOptions). They override the
// generic skirmish defaults so a mission ends on ITS win condition with ITS
// clock and starting gold. TeamGold and TeamResearch are authoritative mission values, including zero gold and the
// full per topic research state (level + mid research turns/allocation). TeamUpgrades carries the file's per type
// gold bought attribute offsets and TeamUpgradeGold the matching score screen stat, so a save that ships upgraded
// types restores them. TeamClan uses 0 for random and 1..8 for a fixed clan. ComplexPools restores each rebuilt
// building network's saved material, fuel, and gold.
type MissionRules struct {
	VictoryType     winloss.VictoryType                                         // duration (turns) or score
	VictoryLimit    int                                                         // the duration/score threshold (0 = none)
	TurnTimer       int                                                         // per turn seconds (0 = untimed)
	EndTurn         int                                                         // the End Turn Time in seconds (0 = none)
	PlayMode        int                                                         // 0 turn based, 1 simultaneous
	Opponent        int                                                         // the computer opponent tier
	TeamGold        [game.TeamCount]int                                         // scripted treasury
	TeamResearch    [game.TeamCount][game.ResearchTopicCount]game.ResearchTopic // scripted research progress (level + mid research remainder)
	TeamUpgrades    [game.TeamCount]map[assets.UnitType][game.AttribCount]int   // per type gold bought attribute offsets (nil = none)
	TeamUpgradeGold [game.TeamCount]int                                         // gold already spent on upgrades (the score screen stat)
	TeamClan        [game.TeamCount]int                                         // scripted clan IDs
	ComplexPools    map[*game.UnitInfo]ComplexPool                              // saved network stores
}

// ComplexPool is one building network's saved pooled stores, restored onto the
// rebuilt complex so a loaded mission keeps its stockpile.
type ComplexPool struct {
	Material, Fuel, Gold int
}

// MissionStart launches a scripted single player mission: the units parsed from
// the retail save body are deployed at their stored grid positions, the per team
// types come from the mission header, the resource map is taken from the
// mission's cargo layer, and the win condition / turn timer / starting gold come
// from its options block. Unlike CustomStart there is NO interactive landing site
// selection: a predefined mission dictates placement; the player only watches
// the descent/landing animation before play begins.
//
// res may be nil (an empty resource map is used). teamTypes maps team index to
// its TeamType (player/computer/none); short slices leave later teams untouched.
//
// C++: GameManager_GameSetup after SaveLoadMenu loads a mission save
// (MissionManager::LoadMission -> the post load fixups in saveload.cpp).
func (s *Screen) MissionStart(units []*game.UnitInfo, teamTypes []game.TeamType, res *economy.ResourceMap, rules MissionRules, seed int64) {
	for i := range s.game.State.Teams {
		if i < len(teamTypes) {
			s.game.State.Teams[i].Type = teamTypes[i]
		}
	}

	// Apply the mission's scripted rules.
	s.game.VictoryType = rules.VictoryType
	s.game.VictoryLimit = rules.VictoryLimit

	// And its OBJECTIVE, which is the thing that actually makes a mission a mission:
	// Campaign 1 is won by razing the enemy's eco spheres with their research centre left
	// standing, not by wiping them out. Without this the mission would end on the generic
	// rules, as every mission in the port did before.
	s.applyMissionObjective()

	// The mission's own timers rule, an untimed mission included (N13). The player's settings
	// are never written: max-go.json keeps them for the next custom game.
	/* if rules.TurnTimer > 0 {: note: unused code in Go port, an untimed mission kept the player's turn timer; a mission plays with the options fixed in its file (N13, the user's rule of 2026-11-07, kept 2026-11-14) */
	s.SetTurnTimer(rules.TurnTimer)
	s.SetEndTurnTimer(rules.EndTurn)

	s.applyMissionEconomy(rules)

	if res == nil {
		res = economy.NewResourceMap(s.wmap.W, s.wmap.H)
	}

	res.Surf = s.wmap
	s.game.Resources = res

	s.applyMissionClans(rules, seed)

	// Give every unit its team's attribute set (base ATTRIBS + research + clan); the
	// save body doesn't carry a usable UnitValues per unit, and without it a
	// unit's scan range reads 0, so nothing reveals the fog. Must happen before
	// Restore, whose visibility rebuild reads each unit's scan.
	for _, u := range units {
		if u.Values == nil {
			u.Values = s.teamValues(u.Team, u.Type)
		}
	}

	// Seed each unit's combat load. The V70 save body does not store ammo or
	// shots (internal/maxsave skips both), so without this every mission unit
	// deploys with Ammo == 0 and Shots == 0 and cannot fire. Must run after the
	// Values loop above (it reads ATTRIB_AMMO / ATTRIB_ROUNDS).
	seedMissionCombat(units)

	// Deploy the scripted units as they are (rejoins complexes + refreshes visibility).
	s.game.Restore(units)

	s.restoreMissionProduction(units, rules)

	if s.game.MiningDifficulty == 0 {
		s.game.MiningDifficulty = 4 // normal mining scale (mirrors CustomStart)
	}

	s.game.State.ActiveTeam = s.game.State.FirstActiveTeam()
	s.game.State.PlayerTeam = s.game.State.ActiveTeam

	// Run the player's opening turn initialization, exactly as every later turn
	// does (C++ runs ManageEconomy at the start of turn one too). A skirmish start
	// hides the need for this because deployUnit fully inits each unit; a mission's
	// units come from the file, so without it their per turn budgets and the base
	// power are never set up: ResetMovement refills Speed (the tanks read 1/6 -> 6/6),
	// ResetCombat refills Shots, and RunEconomy powers the base so the mining station
	// becomes active (it shows its stats and hums when selected). It does NOT touch
	// Hits, so a mission that scripts a damaged unit keeps it. C++: GameManager_
	// ManageEconomy on turn one.
	s.game.BeginTeamTurn(s.game.State.PlayerTeam)

	// Select the player's first BUILDER (constructor / engineer) up front so the map
	// opens on a live selection with its info panel and idle hum, matching a new
	// skirmish game; fall back to the first team unit when none is present.
	s.selected = firstTeamBuilder(units, s.game.State.PlayerTeam)
	s.fogShow = true

	// Descend onto the planet (camera zoom in) and unfold the bases to full size.
	s.StartLanding(s.r.Zoom() * 2)
}

// applyMissionEconomy restores the per team economy from the save body: the scripted
// treasury and research levels (header fidelity). Gold falls back to the options
// start_gold when a team carries none. Research must be set BEFORE the teamValues loop
// in MissionStart so each unit inherits its team's researched attributes (researchValues
// reads Teams[team].Research[topic].Level).
func (s *Screen) applyMissionEconomy(rules MissionRules) {
	for i := range s.game.State.Teams {
		// The alien slot (TeamAlien) has no economy; skip it AND the array bounds
		// guard: TeamGold / TeamResearch / sim.Gold are all [TeamCount] (not [TeamSlots]).
		if s.game.State.Teams[i].Type == game.TeamNone || game.IsAlien(i) {
			continue
		}

		s.game.Gold[i] = rules.TeamGold[i] // the persisted, spendable team gold balance

		// The whole topic state applies, not only the level: a save exists to
		// restore it, so mid research progress (turns to complete + allocation)
		// resumes instead of resetting. syncResearchOrders re-caps the allocation
		// against the mission's actual centres on the first turn.
		for topic := 0; topic < game.ResearchTopicCount; topic++ {
			s.game.State.Teams[i].Research[topic] = rules.TeamResearch[i][topic]
		}

		// The file's per type gold upgrades land as offsets, like the purchase
		// screen carry, so every deployed AND later built unit derives them via
		// teamValues. Must run BEFORE the unit values loop in MissionStart.
		for t, off := range rules.TeamUpgrades[i] {
			for a := game.Attrib(0); a < game.AttribCount; a++ {
				if off[a] != 0 {
					s.game.State.Teams[i].SetUpgradeOffset(t, a, off[a])
				}
			}
		}

		if rules.TeamUpgradeGold[i] > 0 {
			s.game.RecordUpgradeGold(i, rules.TeamUpgradeGold[i]) // the score screen stat resumes too
		}
	}
}

// applyMissionClans applies the mission's scripted clans (0 leaves the team random), then
// resolves any team still left ClanRandom to a concrete one. Both must run before the unit
// values are derived, so each mission unit carries its clan stat bonuses.
func (s *Screen) applyMissionClans(rules MissionRules, seed int64) {
	for i := range s.game.State.Teams {
		if s.game.State.Teams[i].Type == game.TeamNone || game.IsAlien(i) {
			continue
		}

		if i < len(rules.TeamClan) && rules.TeamClan[i] != game.ClanRandom {
			s.game.State.Teams[i].Clan = rules.TeamClan[i]
		}
	}

	// A mission that scripts an explicit clan is left untouched: resolveTeamClans only
	// fills the rest.
	s.resolveTeamClans(game.NewRandomizer(uint32(seed)))
}

// restoreMissionProduction brings the loaded base back up to speed: each complex's saved
// pool, then each factory's saved production. Must run BEFORE BeginTeamTurn's construction
// pass so the first build turn has material.
func (s *Screen) restoreMissionProduction(units []*game.UnitInfo, rules MissionRules) {
	// Reapply each complex's saved material/fuel/gold pool onto the rebuilt complexes
	// (Restore rebuilds them EMPTY). Without this a loaded base has a zero pool: its
	// factories can't fund a build and stall at 0 progress, so production never runs.
	// Match each saved pool to the rebuilt complex that contains its building (mirrors
	// saveload.restore's byAnchor pool reapply).
	if len(rules.ComplexPools) > 0 {
		for _, c := range s.game.Complexes {
			for _, member := range c.Members {
				if pool, ok := rules.ComplexPools[member]; ok {
					c.Material, c.Fuel, c.Gold = pool.Material, pool.Fuel, pool.Gold
					break
				}
			}
		}
	}

	// Kick off each factory's restored production. missionUnit carried the file's
	// build_list + rate + repeat onto the factory; SetFactoryQueue powers it on and
	// starts the head unit, so a mission's factories come up PRODUCING what the file
	// scripted (e.g. a light plant building Scouts on repeat, a heavy plant Tanks)
	// instead of loading idle.
	// C++: UnitInfo::FileLoad restores build_list/build_rate/repeat_build.
	for _, u := range units {
		if isFactory(u.Type) && len(u.BuildQueue) > 0 {
			// Resume the factory's saved production silently (powers it on + starts the
			// head unit, no "production started" banner - it was already running when the
			// mission was saved). missionUnit carried the build_list/rate/repeat.
			s.game.RestoreFactoryProduction(u, u.BuildQueue, u.BuildRate, u.RepeatBuild)
		}
	}

	// Idle an EMPTY factory that loaded on a stale retail "active" order, so it draws
	// no power / shows no "Usage 1/1" with nothing to build (it powers back on when the
	// player queues a unit).
	idleEmptyFactories(units)
}

// seedMissionCombat gives each scripted mission unit its full ammo stock and
// per turn shot budget. The retail V70 save body does not carry ammo/shots
// (internal/maxsave.Unit.FileLoad skips both), so a mission/scenario/campaign/
// training unit arrives with Ammo == 0 and Shots == 0 and cannot fire. Only the
// combat load is seeded: Hits, Speed and the file's order/state are kept (a
// mission may script a damaged unit), so this does NOT call the full InitState
// (which would clobber Hits). An unarmed unit (ATTRIB_AMMO/ROUNDS == 0) stays at
// 0. The == 0 guard preserves ammunition when the mission data already supplies it.
//
// C++ ported : unitinfo.cpp - UnitInfo::Init (ammo/shots portion)
func seedMissionCombat(units []*game.UnitInfo) {
	for _, u := range units {
		if u.Values == nil {
			continue
		}

		if u.Ammo == 0 {
			u.Ammo = u.Values.Get(game.AttribAmmo)
		}

		if u.Shots == 0 {
			u.Shots = u.Values.Get(game.AttribRounds)
		}
	}
}

// idleEmptyFactories powers OFF a mission factory that loaded with an empty build
// queue but a stale "active" retail order. The retail ORDER_BUILD (0x04) reads as
// active (economy.IsActive) yet is not OrderPowerOn, so an idle plant kept drawing
// power and the info panel showed "Usage 1/1" with nothing to build. Match the port's
// idle factory state (OrderPowerOff, like a freshly built one); SetFactoryQueue
// powers it back on when the player queues a unit.
func idleEmptyFactories(units []*game.UnitInfo) {
	for _, u := range units {
		if isFactory(u.Type) && len(u.BuildQueue) == 0 && economy.IsActive(u) {
			u.Order = game.OrderPowerOff
			u.State = game.OrderStateExecutingOrder
		}
	}
}

// MapSize returns the loaded world map's cell dimensions, so the caller can
// decode a mission save body (whose map_cell_count is W*H, not stored in the file).
func (s *Screen) MapSize() (w, h int) {
	return s.wmap.W, s.wmap.H
}

// firstTeamUnit returns the first unit belonging to team, or the first unit of
// any team if none match (so a selection always exists for the panels).
func firstTeamUnit(units []*game.UnitInfo, team int) *game.UnitInfo {
	var any *game.UnitInfo

	for _, u := range units {
		if any == nil {
			any = u
		}

		if u.Team == team {
			return u
		}
	}

	return any
}

// firstTeamBuilder returns team's first constructor / engineer (the unit a player
// most likely acts with first), falling back to firstTeamUnit when the team fields
// no builder. Mirrors the skirmish map entry selection (deployStartBase returns the
// first builder).
func firstTeamBuilder(units []*game.UnitInfo, team int) *game.UnitInfo {
	for _, u := range units {
		if u.Team == team && (u.Type == assets.Constrct || u.Type == assets.Engineer) {
			return u
		}
	}

	return firstTeamUnit(units, team)
}
