// saveload.go : connects the in-game save and load menu flows.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"os"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/world"
)

// SetSaveInfo sets the save directory and .WRL map filename recorded in snapshots. Both are required before F5 and F9 work.
// C++: smart_file_name / FileManager save path logic (game_manager.cpp)
func (s *Screen) SetSaveInfo(baseDir, mapName string) {
	s.baseDir = baseDir
	s.mapName = strings.TrimSpace(mapName)
}

// SetSaveLoadFunc installs the F9/F5 Save/Load hook. It returns quit when the player abandons the game for the main menu.
func (s *Screen) SetSaveLoadFunc(fn func(saving bool) (quit bool, err error)) {
	s.hooks.saveLoadFn = fn
}

// slotName converts a 1-indexed slot number to its save file stem ("save1").
func slotName(i int) string {
	return fmt.Sprintf("save%d", i)
}

// SlotInfo reports the save/load screen display fields for slot (1-based): the
// file stem, the entered save name, the game type and whether it is in use.
// It implements menus.SaveLoadOps. An unreadable slot reports not in use.
func (s *Screen) SlotInfo(slot int) (fileName, saveName, typeName string, inUse bool) {
	fileName = slotName(slot)

	if s.baseDir == "" {
		return fileName, "", "", false
	}

	path, ok := saveload.FindSlot(s.baseDir, slot) // save<N>, or legacy slot%02d

	if !ok {
		return fileName, "", "", false
	}

	f, err := os.Open(path)

	if err != nil {
		return fileName, "", "", false
	}

	defer f.Close()
	snap, err := saveload.Load(f)

	if err != nil {
		return fileName, "", "", false
	}

	name := snap.Name

	if name == "" { // older saves without a name: fall back to turn/map
		m := strings.TrimSuffix(strings.ToUpper(snap.MapName), ".WRL")
		name = fmt.Sprintf("Turn %d %s", snap.Turn, m)
	}

	return fileName, name, "Custom", true
}

// Save writes the current game to slot (1-based) under the given save name.
// It implements menus.SaveLoadOps.
func (s *Screen) Save(slot int, name string) error {
	return s.saveToSlot(slot, name, true)
}

// saveToSlot writes a snapshot to a slot and optionally plays the Game saved voice.
func (s *Screen) saveToSlot(slot int, name string, announce bool) error {
	if s.baseDir == "" || s.mapName == "" {
		return fmt.Errorf("save: no save info configured")
	}

	path, err := saveload.Path(s.baseDir, slotName(slot))

	if err != nil {
		return err
	}

	snap := saveload.Capture(s.game, s.mapName)
	snap.Name = strings.TrimSpace(name)
	snap.Bookmarks = s.captureCameraBookmarks() // screen state the sim capture cannot see (v15)
	snap.Groups = s.captureUnitGroups()         // the 1..9 control groups (v16)
	snap.Displays = s.captureDisplayToggles()   // the nine display overlays (v20)
	snap.TurnSeconds = s.captureTurnClock()     // seconds left on the running turn (v22)
	snap.MissionTitle = s.missionTitle          // a scripted mission's briefing, so a reload keeps its
	snap.MissionBrief = s.missionBriefing       // objective instead of reading as a skirmish (v22)
	f, err := os.Create(path)

	if err != nil {
		return err
	}

	defer f.Close()

	if err := saveload.Save(f, snap); err != nil {
		return err
	}

	if announce {
		s.playVoice(vsGameSaved) // "Game saved!"
	}

	dbg.Printf("saved slot %d to %s (turn %d, %q)", slot, path, snap.Turn, snap.Name)
	return nil
}

// autoSaveSlot is the fixed slot the Auto Save preference writes to each turn
// (the original always saves automatically to slot 10). C++: the auto_save preference.
const autoSaveSlot = 10

// maybeAutoSave writes a silent save to the auto save slot when the Auto Save
// preference is on. Called at the player's end of turn.
func (s *Screen) maybeAutoSave() {
	if !s.autoSave || s.baseDir == "" || s.mapName == "" {
		return
	}

	if err := s.saveToSlot(autoSaveSlot, "Auto Save", false); err != nil {
		// Report auto-save failure so the player does not rely on a stale slot.
		dbg.Printf("auto-save: %v", err)
		s.showAlertBanner("Auto Save FAILED.")
		s.logMessage("Auto Save failed: the game was not saved to slot 10.")
	}
}

// attachGameServices wires the AI and unit factory onto the live sim.Game. It runs at screen setup and after loading a restored game.
// C++: GameManager_LoadGame reinstalls the AI + deploy hooks on the restored state.
func (s *Screen) attachGameServices() {
	s.game.AI = s.ai
	s.game.MakeUnit = s.makeUnit

	// Mission objectives are functions, so rebuild them from the saved category and index.
	s.applyMissionObjective()

	// Apply the game's stored opponent tier to the AI now. A load restored the tier
	// onto the game state, but the AI here is freshly built at its default and would
	// keep that default (wrong Goal / Reports readout, wrong knobs) until the first
	// computer turn runs its own init. A fresh game leaves the tier unset here and gets
	// it through SetAIDifficulty at launch instead.
	if s.ai != nil && s.game.State != nil && s.game.State.Difficulty != game.DifficultyUnset {
		s.ai.SetDifficulty(s.game.State.Difficulty)
	}
}

// SetWorldReloader installs the loader used to swap to a save's map when it differs from the current world. Without it, loading keeps the current map.
func (s *Screen) SetWorldReloader(fn func(mapFile string) (*world.Map, error)) {
	s.hooks.worldReloader = fn
}

// reloadWorld returns the map used to restore a save. A different map reloads its terrain palette and renderer state; unavailable reloads keep the current map.
func (s *Screen) reloadWorld(mapFile string) *world.Map {
	cur := strings.TrimSpace(s.mapName)

	// Same map (or no name to compare): nothing to reload.
	if mapFile == "" || cur == "" || strings.EqualFold(mapFile, cur) {
		return s.wmap
	}

	if s.hooks.worldReloader == nil {
		dbg.Printf("  NOTE: save world %q differs from %q but no reloader is wired; keeping the current world", mapFile, cur)
		return s.wmap
	}

	m, err := s.hooks.worldReloader(mapFile)

	if err != nil || m == nil {
		dbg.Printf("  NOTE: could not reload save world %q (%v); keeping the current world %q", mapFile, err, cur)
		return s.wmap
	}

	m.ApplyTerrainPalette(&s.pal) // swap only the terrain slots to the new world's colours

	// Rebuild sprite shadow colours for the new terrain palette.
	if s.sprites != nil {
		s.sprites.SetPalette(&s.pal)
	}

	s.r.SetMap(m) // point the renderer at the new map, dropping its tile cache
	s.wmap = m
	s.mapName = mapFile
	dbg.Printf("  reloaded save world %q (was %q) for the in game load", mapFile, cur)
	return m
}

// Load replaces the current game from slot (1-based) and resets view state.
// It implements menus.SaveLoadOps.
func (s *Screen) Load(slot int) error {
	if s.baseDir == "" {
		return fmt.Errorf("load: no base dir configured")
	}

	path, ok := saveload.FindSlot(s.baseDir, slot) // save<N>, or legacy slot%02d

	if !ok {
		return fmt.Errorf("load: no save in slot %d", slot)
	}

	f, err := os.Open(path)

	if err != nil {
		return err
	}

	defer f.Close()
	snap, err := saveload.Load(f)

	if err != nil {
		return err
	}

	dbg.Printf("loaded savegame : %s (world=%s, turn=%d, ver=%d)",
		path, strings.TrimSpace(snap.MapName), snap.Turn, snap.Version)

	// A save carries the .WRL it was made on. When that differs from the world now on
	// screen (an in game load of a game saved on another map), reload the save's own
	// terrain first, so the restored units sit on the ground they were saved on. Without
	// this a base saved on land would land in water on the current map. reloadWorld
	// returns the current world unchanged when the maps match or no reloader is wired.
	worldMap := s.reloadWorld(strings.TrimSpace(snap.MapName))

	s.game = snap.Restore(worldMap)
	s.attachGameServices() // Restore builds a fresh game: rewire the AI + unit factory
	s.replayPanelAnim()    // a load rebuilds the bars, so they unfold again (C++: GameManager_LoadGame)
	// Reset UI state that referred to the old game instance.
	s.clearSelected()
	s.turnFlow.handover = -1
	s.gameOver = false
	s.resumeActiveTurn()
	s.turnFlow.handBackTeam = -1    // drop any pending end turn voice hand back from the old game
	s.turnFlow.simPlanned = false   // a loaded simultaneous game reopens its plan phase...
	s.turnFlow.simLoadResume = true // ...as a RESUME: AI replan only, no turn start re-run (beginSimRound)
	s.turnFlow.graceArmed = false   // the old game's armed grace must not suppress this round's
	s.buildUI.buildIdx = 0
	s.buildUI.buildMenuOpen = false
	s.orderInput.popupOpen = false
	s.banner.msg = ""
	s.banner.msgLog = nil     // an in game load must not carry the PREVIOUS game's messages into the Reports log
	s.resetAnnouncerForLoad() // drop the old game's voice queue + rebase every one shot latch on the restored board
	// Transient animation jobs hold OLD game unit pointers: a job left running
	// across a load would tick against units that no longer exist on the board.
	s.effects.airUnload = nil
	s.effects.airLoad = nil
	s.effects.loading = nil
	s.effects.bulbUnit = nil // a deferred planning bulb move must not fire into the loaded game
	s.clearPortrait()
	s.resetDebugFlags()                    // a loaded game starts at the vanilla state - F2 cheats off (fog ON)
	s.applyCameraBookmarks(snap.Bookmarks) // the save's F5..F8 camera pins replace the current ones (v15)
	s.applyUnitGroups(snap.Groups)         // the save's 1..9 control groups replace the current ones (v16)
	s.applyDisplayToggles(snap.Displays)   // the save's display overlays come back with it (v20)
	s.applyTurnClock(snap.TurnSeconds)     // resume the countdown where the save left it (v22)
	s.focusOnLoadedBase()                  // centre + zoom partly in to the base (vanilla zooms in on load)
	s.selectFirstBuilder()                 // pre select the first builder, matching new game map entry (design choice)
	s.restoreGoalBlock(snap)               // the mission's own briefing, or the random game info block
	return nil
}

// restoreGoalBlock puts the right text behind the GOAL button. A load used to call
// SetRandomGameGoal for everything, so a campaign or scenario reloaded showing the
// "Random game:" info block a skirmish gets, with its objective gone. A save that carries
// a mission briefing (v22) gets it back; everything else rebuilds the info block from the
// restored state, as before.
//
// NOTE: a mission's scripted WIN/LOSS conditions are a different matter and are NOT a save
// gap - nothing in the port ever installs them (they need the Lua scripter layer), so a
// mission runs the default elimination rule whether it is freshly launched or reloaded.
func (s *Screen) restoreGoalBlock(snap *saveload.Snapshot) {
	if game.MissionCategory(snap.Mission).IsMission() && snap.MissionBrief != "" {
		s.SetMissionGoal(snap.MissionTitle, snap.MissionBrief)

		return
	}

	s.SetRandomGameGoal()
}

// resumeActiveTurn reopens the turn slot the save was taken in. The FILES button is live
// while an opponent moves, so a save can carry a COMPUTER team as the active one; a load
// used to hand the map straight back to the human, who was then playing inside the
// computer's slot and whose End Turn closed that computer's turn. The team's in flight
// moves are transient and were never saved, so its order queue drains on the first frame
// and the turn rolls on after the usual dwell. Its economy and its AI orders both ran
// before the save, so BeginTeamTurn must NOT run again for it.
func (s *Screen) resumeActiveTurn() {
	team := s.game.State.ActiveTeam
	computer := team >= 0 && team < game.TeamCount && s.game.State.Teams[team].Type == game.TeamComputer

	s.turnFlow.aiTurn = computer
	if computer {
		s.turnFlow.aiTurnStartMS = platform.Ticks() // start this turn's pace dwell now
	}
}

// selectFirstBuilder preselects the player's lowest ID builder (constructor/engineer)
// on a load, matching new game map entry; selects nothing if the team has
// none. It does NOT restore the saved selection (the save format does not persist it),
// and sets it directly (no "Ready!" voice/banner) like the new game deploy.
func (s *Screen) selectFirstBuilder() {
	team := s.game.State.PlayerTeam
	var best *game.UnitInfo

	for _, u := range s.game.Units.Units() {
		if u.Team != team || (u.Type != assets.Constrct && u.Type != assets.Engineer) {
			continue
		}

		if best == nil || u.ID < best.ID {
			best = u
		}
	}

	s.setSelected(best) // nil when the team has no builder left
}

// loadFocusZoom is the zoom a freshly loaded game settles at: partly zoomed IN to the
// base (one preset step in from the 16 in game default, the same level the new game
// landing climb settles at). Vanilla zooms in to the base on load rather than opening
// at the wider default. C++: GameManager_GameLoop's post load zoom to the team.
const loadFocusZoom = 32

// focusOnLoadedBase centres on the player's base and, when the view is wider than the
// focus zoom, plays a brief zoom IN to it. It reuses the landing camera climb (phase()
// reads phaseLanding while landingStep>0, so the run loop drives tickLanding), but does
// NOT touch Unfold - a loaded base is already built, so nothing unfolds again.
func (s *Screen) focusOnLoadedBase() {
	s.centerOnPlayerBase() // position on the base at the current zoom first

	if s.r == nil || s.r.Zoom() >= loadFocusZoom {
		return // already at or closer than the focus zoom: stay centred, no climb
	}

	s.camera.landingZoom = loadFocusZoom
	s.camera.landingSmooth = true // a gentle LINEAR ramp (tickLanding), not the orbit doubling
	steps := 1                    // a final settle step so the full size frame shows

	for z := s.r.Zoom(); z < loadFocusZoom; z += loadZoomInc {
		steps++
	}

	s.camera.landingStep = steps
	s.camera.landingStamp = platform.Ticks()
}

// resetDebugFlags returns the F2 developer cheats to their vanilla start defaults (fog
// ON, win/loss active, no enemy path overlay). They are volatile UI state, never saved,
// so a load must not inherit a stale toggle (a fresh Screen would default fogShow false
// and reveal the whole map). AI reasoning is turned back on by attachGameServices.
func (s *Screen) resetDebugFlags() {
	s.fogShow = true            // fog of war ON (the vanilla state; F2 reveals it)
	s.debugMenu.endless = false // win/loss conditions active
	s.showAIPaths = false       // vanilla never draws enemy move paths
	s.maxSurvey = false         // resource reveal off (the [MAXSURVEY] flag is never saved)
	s.cheatInput = ""           // drop a half typed cheat code from the old game
}

// centerOnPlayerBase scrolls the main view (and minimap viewport) to the player's
// base so a loaded game opens on the player's units. It centres on the centroid of the
// team's buildings, falling back to the lowest ID unit. Both are ORDER INDEPENDENT
// (centroid is a sum; fallback keys on ID), so re centring every frame during the load
// zoom is STABLE (Units() is random order; a "first unit" fallback jumped the view).
// C++: GameManager centres on the team's units after a load.
func (s *Screen) centerOnPlayerBase() {
	if s.game == nil {
		return
	}

	team := s.game.State.PlayerTeam
	sumX, sumY, n := 0, 0, 0
	var fallback *game.UnitInfo

	for _, u := range s.game.Units.Units() {
		if u.Team != team {
			continue
		}

		if fallback == nil || u.ID < fallback.ID {
			fallback = u
		}

		if u.Flags.Has(game.FlagStationary) { // the base = the team's buildings
			sumX, sumY, n = sumX+u.GridX, sumY+u.GridY, n+1
		}
	}

	var cx, cy int

	switch {
	case n > 0:
		cx, cy = sumX/n, sumY/n
	case fallback != nil:
		cx, cy = fallback.GridX, fallback.GridY
	default:
		return // no player units to centre on
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cx, cy, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// openSaveMenu / openLoadMenu run the Save/Load screen via the installed hook.
func (s *Screen) openSaveMenu() {
	s.runSaveLoad(true)
}
func (s *Screen) openLoadMenu() {
	s.runSaveLoad(false)
}

// runSaveLoad invokes the Save/Load screen hook if wired. A Quit choice plays
// the planet leaving animation (panels slide closed), which then returns to the
// main menu - the same exit sequence as the ESC "OK to exit game?" path.
func (s *Screen) runSaveLoad(saving bool) {
	if s.hooks.saveLoadFn == nil {
		return
	}

	quit, err := s.hooks.saveLoadFn(saving)

	if err != nil {
		dbg.Printf("save/load: %v", err)
	}

	if quit {
		s.startLeaving()
	}
}
