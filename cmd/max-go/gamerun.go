// gamerun.go : builds and runs the game screen, including world loading, audio, and save handling.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gamescreen"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/winloss"
	"github.com/suglasp/max-go/internal/world"
)

// runGameScreen loads a world and enters the game screen with its FRAMEPIC panel, map, and minimap. It
// returns to the menu loop on Escape.
func runGameScreen(ctx *menus.Context, worldIdx int, mode gameScreenMode, noMovie bool) error {
	name, err := ctx.Lib.ReadString(menus.WorldTag(worldIdx))
	if err != nil {
		return err
	}

	scr, err := buildGameScreen(ctx, strings.TrimSpace(name))
	if err != nil {
		return err
	}

	scr.SetNoMovie(noMovie) // honour -nomovie for the landing cinematic

	applyGameScreenMode(scr, ctx, mode)

	// Start the terrain background music for actual gameplay (not the gallery /
	// inspection views). The win/lose jingle takes over at game over.
	switch mode {
	/* // note: unused code in Go port
	// no caller runs gameScreenPlain; every caller passes a named mode.
	case gameScreenPlain, gameScreenDemo, gameScreenRandom, gameScreenRandom2, gameScreenCustom:
	*/
	case gameScreenDemo, gameScreenRandom, gameScreenRandom2, gameScreenCustom:
		startGameMusic(ctx, gameMusicTag(worldIdx))
	}

	if err := scr.Run(); err != nil {
		return err
	}

	// When the game ends, show the game-over screen unless ESC returns to the menu.
	// as a standalone state with no game world underneath.
	if o := scr.Outcome(); o != winloss.Continue {
		// The game over snapshot is a screendata DTO the menu takes as is.
		return menus.RunGameOver(ctx, scr.GameOverData())
	}

	return nil
}

// applyGameScreenMode prepares one -emulate bench by placing units, setting debug state, or opening the
// requested game-over screen. Plain mode does nothing.
func applyGameScreenMode(scr *gamescreen.Screen, ctx *menus.Context, mode gameScreenMode) {
	switch mode {
	case gameScreenDeath:
		scr.DebugGameOver() // -emulate death: jump straight to the game over screen (won)

	case gameScreenWin:
		scr.DebugGameOverOutcome(winloss.PlayerWon) // -emulate win: score screen with VICTORY

	case gameScreenLose:
		scr.DebugGameOverOutcome(winloss.PlayerLost) // -emulate lose: score screen with DEFEAT

	case gameScreenDemo:
		scr.DebugSpawn() // -emulate game: seed a few units so the view isn't empty

	case gameScreenPopup:
		scr.DebugPopup() // -emulate popup: open a unit's order popup for inspection

	case gameScreenRandom:
		scr.SetAlienDerelicts(derelictsLevel)              // -derelicts: scatter neutral aliens (default Rare)
		applyBenchCustomFlags(scr, ctx)                    // explicit -raw/-fuel/-gold/-startcredit/-score/-duration
		scr.SetAIDifficulty(benchDifficulty())             // before starting, the AI rolls its strategy and buys upgrades
		scr.RandomStart(time.Now().UnixNano(), npcCount()) // -random: player + 1..3 opponents on separate islands
		scr.SetPlayMode(ctx.Settings.Game.PlayMode)        // -simultane / -turnbased selects the quick-start mode
		scr.SetTurnTimer(ctx.Settings.Game.TurnTimer)      // sets the per-turn time budget
		scr.SetEndTurnTimer(ctx.Settings.Game.EndTurn)     // sets the simultaneous End Turn grace period
		scr.AnnounceAIDifficulty()                         // banner: the quick start has no setup screen to show it
		scr.SetRandomGameGoal()                            // GOAL button info dialog (built from the resolved state)

	case gameScreenRandom2:
		scr.SetAlienDerelicts(derelictsLevel)               // -derelicts: scatter neutral aliens (default Rare)
		applyBenchCustomFlags(scr, ctx)                     // explicit -raw/-fuel/-gold/-startcredit/-score/-duration
		scr.SetAIDifficulty(benchDifficulty())              // before starting, the AI rolls its strategy
		scr.RandomStart2(time.Now().UnixNano(), npcCount()) // -emulate random2: player + 1..3 large turn 100 bases (test bed)
		scr.SetPlayMode(ctx.Settings.Game.PlayMode)         // -simultane / -turnbased selects the random2 mode
		scr.SetTurnTimer(ctx.Settings.Game.TurnTimer)       // sets the per-turn time budget
		scr.SetEndTurnTimer(ctx.Settings.Game.EndTurn)      // sets the simultaneous End Turn grace period
		scr.AnnounceAIDifficulty()
		scr.SetRandomGameGoal() // GOAL button info dialog (built from the resolved state)

	case gameScreenCustom:
		// -emulate custom starts with a purchased loadout like a custom game. The two
		// builders carry the raw materials from the purchase screen. The tier goes
		// first, so the AI rolls its strategy and buys its upgrades during setup.
		scr.SetAIDifficulty(benchDifficulty())
		scr.CustomStart([]assets.UnitType{
			assets.Constrct, assets.Engineer, assets.Tank, assets.Scout, assets.Scanner,
		}, []int{
			economy.StartConstructorCargo, economy.StartEngineerCargo, 0, 0, 0,
		}, 500, time.Now().UnixNano()) // a testable starting gold reserve for the bench
		scr.SetRandomGameGoal() // GOAL button info dialog (built from the resolved state)

	case gameScreenAssets:
		scr.DeployGallery() // -emulate assets: every building + unit laid out and animating

	case gameScreenSoldier:
		scr.DeploySoldiers() // -emulate soldier: infantry + commando in all 8 facings, walking

	case gameScreenAPCUnit:
		scr.DeployAPCs() // -emulate apcunit: a sound test bench (select/drive/stop/fire)

	case gameScreenHeadRotate:
		runHeadrotateDebug(scr) // -emulate headrotate: tank/turret heads rotate toward fired at targets

	case gameScreenTransport:
		scr.DeployTransport() // -emulate transport: load a transport, Activate to unload via the Repair Shop screen

	case gameScreenCommando:
		scr.DeployCommandoBench() // -emulate commando: disable/steal infiltration bench

	case gameScreenMines:
		scr.DeployMinesBench() // -emulate mines: mine laying bench

	case gameScreenBulldozer:
		scr.DeployBulldozerBench() // -emulate bulldozer: rubble clearing bench

	case gameScreenAir:
		scr.DeployAirBench() // -emulate air: take off / land bench

	case gameScreenMissile:
		scr.DeployMissileBench() // -emulate missile: all missile units vs static enemies

	case gameScreenSpotted:
		scr.DeploySpottedBench() // -emulate spotted: AI pursues an enemy's last seen cell

	case gameScreenAliens:
		scr.DeployAliensBench() // -emulate aliens: neutral alien derelicts (inert, attackable)

	case gameScreenFactoryRun:
		scr.DebugFactoryRun() // -emulate factoryrun: a factory producing its build queue

	case gameScreenLevelUp:
		scr.DeployLevelUpBench() // -emulate levelup: Cost build turn reduction + HITS HP scaling

	case gameScreenSubmarine:
		scr.DeploySubmarineBench() // -emulate submarine: a friendly AWAC + Corvette reveal an enemy sub

	case gameScreenNavalFire:
		scr.DeployNavalFireBench() // -emulate navalfire: ships cannot fire across land

	case gameScreenParkedBuild:
		scr.DeployParkedBuildBench() // -emulate parkedbuild: a parked builder loaded or killed releases / loses its building

	case gameScreenReactionFire:
		scr.DeployReactionFireBench() // -emulate reactionfire: the reaction fire voice on and off screen (H1)

	}
}

// loadGameScreenArt returns the game UI palette and panel art from FRAMEPIC. If FRAMEPIC is unavailable, it
// falls back to the COLOR.PAL palette with no panel art (a plain border).
func loadGameScreenArt(ctx *menus.Context) (gfx.Palette, *gfx.Buffer) {
	var (
		basePal gfx.Palette
		bg      *gfx.Buffer
	)

	if data, err := ctx.Lib.Read("FRAMEPIC"); err == nil {
		if img, err := sprites.DecodeBig(data); err == nil {
			basePal = img.Palette
			bg = img.Buffer()
		} else {
			dbg.Printf("FRAMEPIC decode: %v", err)
		}
	}

	if basePal == (gfx.Palette{}) {
		// A missing or invalid file leaves the zero palette.
		basePal, _ = gfx.LoadBasePalette(ctx.DataDir)
		/* // note: unused code in Go port
		// The read and parse block is gfx.LoadBasePalette.
		if data, err := os.ReadFile(resfile.DataPath(ctx.DataDir, "COLOR.PAL")); err == nil {
			if pf, err := gfx.ParsePalFile(data); err == nil {
				basePal = pf.Palette
			}
		}
		*/
	}

	return basePal, bg
}

// buildGameScreen loads a .WRL map and wires its panel art, portraits, attributes, and menu hooks. New games
// and loaded games share this setup.
func buildGameScreen(ctx *menus.Context, mapFile string) (*gamescreen.Screen, error) {
	m, err := world.LoadMap(resfile.DataPath(ctx.DataDir, mapFile))

	if err != nil {
		return nil, err
	}

	dbg.Printf("world loaded: %s %dx%d cells", mapFile, m.W, m.H)

	basePal, bg := loadGameScreenArt(ctx)
	scr := gamescreen.New(ctx.Sys, ctx.Plat, m, basePal, bg, ctx.Lib, ctx.Assets.Strings)
	scr.SetPortraitDir(ctx.DataDir)              // loose .FLC corner portraits
	scr.SetAttribs(ctx.Assets.Attribs)           // base ATTRIBS rows for build costs
	scr.SetClans(ctx.Assets.Clans)               // per team clan stat bonuses
	scr.SetSaveInfo(ctx.Plat.BaseDir(), mapFile) // F9/F5 save load

	/* // note: unused code in Go port
	// ctx.Settings is never nil: the only menus.Context is built with Settings set; the call below runs unguarded.
	if ctx.Settings != nil {
		scr.SetPopupVanillaLook(ctx.Settings.Video.PopupVanillaLook) // order popup: retail face or bevel art
	}
	*/
	scr.SetPopupVanillaLook(ctx.Settings.Video.PopupVanillaLook) // order popup: retail face or bevel art

	// The in game debug UI (F2 menu, developer keys U/A/V/B, key hint) is on in a DEBUG_GAME dev build,
	// and otherwise opt in at runtime via the max-go.json debug.menu flag. (-emulate stays DEBUG_GAME only;
	// the custom game launch flags register in every build.)
	/* // note: unused code in Go port
	// ctx.Settings is never nil, so the debug flag is read unguarded below.
	debugUI := DEBUG_GAME
	if ctx.Settings != nil {
		debugUI = debugUI || ctx.Settings.Debug.Menu
	}
	*/
	debugUI := DEBUG_GAME || ctx.Settings.Debug.Menu

	scr.SetDebug(debugUI)

	// Apply remappable action keys when building the game. Changes from max-settings take effect on
	// the next run because the Options menu does not edit key bindings.
	/* // note: unused code in Go port
	// ctx.Settings is never nil; the call below runs unguarded.
	if ctx.Settings != nil {
		scr.SetKeybinds(ctx.Settings.Keybinds)
	}
	*/
	scr.SetKeybinds(ctx.Settings.Keybinds)

	scr.SetShowAStarPath(showAStarPath) // -set-a-star debug: draw the white A* path

	// Let an in-game load swap to a save's own world when its map differs (otherwise the restored
	// units land on the wrong terrain).
	scr.SetWorldReloader(func(name string) (*world.Map, error) {
		return world.LoadMap(resfile.DataPath(ctx.DataDir, strings.TrimSpace(name)))
	})

	applyLivePrefs(scr, ctx)

	wireMenuCallbacks(scr, ctx)
	wireAudioCallbacks(scr, ctx)

	return scr, nil
}

// applyLivePrefs pushes the changeable game prefs into a running screen, so the in game
// Options menu takes effect at once and not only on the next load.
func applyLivePrefs(scr *gamescreen.Screen, ctx *menus.Context) {
	scr.ApplyPrefs(gamescreen.LivePrefs{
		QuickScroll:    ctx.Settings.Prefs.QuickScroll,
		TurnTimer:      ctx.Settings.Game.TurnTimer,
		EndTurn:        ctx.Settings.Game.EndTurn,
		AnimateEffects: ctx.Settings.Prefs.Effects,
		DoubleSteps:    ctx.Settings.Prefs.FastMovement,
		FollowUnit:     ctx.Settings.Prefs.FollowUnit,
		ClickScroll:    ctx.Settings.Prefs.ClickScroll,
		AutoSelect:     ctx.Settings.Prefs.AutoSelect,
		EnemyHalt:      ctx.Settings.Prefs.EnemyHalt,
		AutoSave:       ctx.Settings.Game.AutoSave,
	})
}

// wireMenuCallbacks installs the hooks that open popups, full-screen menus, and help mode over a running
// game. The setters only store functions, so their order does not matter.
func wireMenuCallbacks(scr *gamescreen.Screen, ctx *menus.Context) {
	scr.SetOptionsFunc(func() error { // O / PREFERENCES (centred in the viewport)
		err := menus.RunOptions(ctx, gamescreen.MainMap)
		/* // note: unused code in Go port
		// ctx.Settings is never nil and settings.ResolvePath never returns an empty SettingsPath; both run unguarded below.
		if ctx.Settings != nil {
			applyLivePrefs(scr, ctx) // apply prefs changes to the running game
			if ctx.SettingsPath != "" {
				if e := ctx.Settings.Save(ctx.SettingsPath); e != nil { // persist so settings.json matches
					dbg.Printf("settings save: %v", e)
				}
			}
		}
		*/
		applyLivePrefs(scr, ctx)                                // apply prefs changes to the running game
		if e := ctx.Settings.Save(ctx.SettingsPath); e != nil { // persist so settings.json matches
			dbg.Printf("settings save: %v", e)
		}

		return err
	})

	scr.SetSaveLoadFunc(func(saving bool) (bool, error) { return menus.RunSaveLoad(ctx, scr, saving) }) // F9/F5 / FILES
	scr.SetConfirmFunc(func(text string) bool { return menus.Confirm(ctx, text) })                      // ESC exit prompt
	scr.SetMessageFunc(func(text string) { menus.Message(ctx, text) })                                  // GOAL info + Alt+P pause dialog
	scr.SetResearchFunc(func(r *[game.ResearchTopicCount]game.ResearchTopic, c int) error {             // research building popup
		return menus.RunResearch(ctx, r, c, gamescreen.MainMap)
	})

	scr.SetAllocFunc(func(c *economy.Complex, rmap *economy.ResourceMap, diff int) error { // complex Allocate popup
		return menus.RunAllocate(ctx, c, rmap, diff, gamescreen.MainMap)
	})

	scr.SetUpgradeFunc(func(units []*game.UpgradeUnit, gold *int) (bool, error) { // building Upgrade popup
		return menus.RunUpgrade(ctx, units, gold, gamescreen.MainMap)
	})

	scr.SetSelfDestructFunc(func() (bool, error) { return menus.RunSelfDestruct(ctx, gamescreen.MainMap) })      // building popup Remove -> arm/detonate
	scr.SetBuildFunc(func(builder assets.UnitType, team, storage int) (screendata.BuildSelection, bool, error) { // B / popup Build -> CONBUILD Construction Menu
		return menus.RunConstruction(ctx, builder, team, storage)
	})

	scr.SetFactoryBuildFunc(func(factory assets.UnitType, team, storage int, queue []assets.UnitType, repeat bool) (screendata.FactorySelection, bool, error) { // factory popup Build -> FACBUILD Production Menu
		return menus.RunFactoryBuild(ctx, factory, team, storage, queue, repeat)
	})

	scr.SetStatusFunc(func(u *game.UnitInfo, name string) error { return menus.RunUnitStatus(ctx, u, name) }) // right click unit -> STATS Unit Status Screen

	scr.SetTransferFunc(func(info screendata.TransferInfo) (int, error) { // X-Fer cargo exchange popup
		return menus.RunTransfer(ctx, info, gamescreen.MainMap)
	})

	scr.SetRepairFunc(func(ops gamescreen.RepairShopOps) error { // depot/hangar/dock/barracks Repair Shop
		// gamescreen.RepairShopOps and menus.RepairShopOps both alias
		// repairshopui.RepairShopOps, so the backend passes straight through.
		return menus.RunRepairShop(ctx, ops)
	})

	scr.SetReportsFunc(func(data screendata.ReportData) (uint16, error) { // REPORTS button -> Reports screen
		return menus.RunReports(ctx, data)
	})

	// HELP button / ? key -> help mode: the pointer becomes a question mark and the next
	// thing clicked is explained. HelpFixup maps the click back into the 640x480 layout
	// the help data is written in (the in game widgets move in widescreen).
	ctx.HelpFixup = gamescreen.HelpCoords
	scr.SetHelpFunc(func(section string) { menus.Help(ctx, section) })
}

// wireAudioCallbacks installs the sound hooks: the one shot and held effects, the
// announcer voice, the music and the mission cut scenes.
func wireAudioCallbacks(scr *gamescreen.Screen, ctx *menus.Context) {
	scr.SetSfxFunc(func(primary, fallback string, pan float64) { ctx.Audio.PlaySfxPannedFallback(primary, fallback, pan) }) // in game sfx (combat, movement); unit specific clip else generic default

	// Start movement sounds once and idle sounds in a loop. The returned handle
	// lets the screen stop either sound.
	scr.SetSfxHeldFuncs(
		func(primary, fallback string, pan, volScale float64, loop bool) int {
			return ctx.Audio.PlaySfxHeld(primary, fallback, pan, volScale, loop)
		},
		func(id int) { ctx.Audio.StopSfxHeld(id) },
		func(id int, pan float64) { ctx.Audio.SetSfxHeldPan(id, pan) })
	scr.SetSfxIntroLoopFunc(func(primary, fallback string, pan, volScale float64) int {
		return ctx.Audio.PlaySfxIntroThenLoop(primary, fallback, pan, volScale)
	})
	// Positional sound: a unit's one shots and its held sounds fade with distance from the view centre and zoom.
	scr.SetSfxPositionalFunc(ctx.Audio.PlaySfxPositional)
	scr.SetSfxHeldVolumeFunc(ctx.Audio.SetSfxHeldVolume)
	scr.SetChannelsReversed(ctx.Settings.Audio.ChannelsReversed)

	scr.SetVoiceFunc(ctx.Audio.PlayVoice)                                      // announcer voice (e.g. colony started)
	scr.SetVoicePlayingFunc(ctx.Audio.VoicePlaying)                            // lets the voice queue space back to back lines (team end of turn voices)
	scr.SetMusicFunc(ctx.Audio.PlayMusic)                                      // win/lose jingle at game over
	scr.SetMovieFunc(func(tag string) error { return menus.RunMVE(ctx, tag) }) // the landing cinematic
}

// gameMusicTag picks a planet's background track by terrain: the 24 worlds run in four blocks
// of six (snow, crater, green, desert), so world/6 selects the .MSC track.
// C++ ported : game_manager.cpp - PlayMusic(world/6 + SNOW_MSC, true)
func gameMusicTag(worldIdx int) string {
	tracks := [4]string{"SNOW_MSC", "CRTR_MSC", "GREN_MSC", "DSRT_MSC"}
	i := worldIdx / 6

	if i < 0 || i >= len(tracks) {
		i = 0
	}

	return tracks[i]
}

// startGameMusic begins the looping in game background track and logs it.
func startGameMusic(ctx *menus.Context, tag string) {
	dbg.Printf("in-game music: %s", tag)
	ctx.Audio.PlayMusic(tag, true)
}

// gameMusicForMap chooses a background track from a WRL terrain prefix when no
// world index is available. Unknown prefixes use the snow track.
func gameMusicForMap(mapFile string) string {
	switch u := strings.ToUpper(mapFile); {
	case strings.HasPrefix(u, "CRATER"):
		return "CRTR_MSC"

	case strings.HasPrefix(u, "GREEN"):
		return "GREN_MSC"

	case strings.HasPrefix(u, "DESERT"):
		return "DSRT_MSC"

	default:
		return "SNOW_MSC"
	}
}

// loadGameFromMainMenu opens the Load Game screen and, on a chosen slot, loads that save's map,
// restores the game into it and runs it. C++: menu.cpp load path -> SaveLoadMenu_MenuLoop(.., false) ->
// GameManager_LoadGame.
func loadGameFromMainMenu(ctx *menus.Context) error {
	baseDir := ctx.Plat.BaseDir()
	chosen := -1
	ops := &menuLoadOps{debugSaveLoadOps: debugSaveLoadOps{baseDir: baseDir}, onLoad: func(slot int) { chosen = slot }}

	if _, err := menus.RunSaveLoad(ctx, ops, false); err != nil {
		return err
	}

	if chosen < 0 {
		return nil // cancelled
	}

	mapFile, err := loadedSaveMap(baseDir, chosen)
	if err != nil {
		menus.Message(ctx, "Load failed: "+err.Error())
		return nil
	}

	scr, err := buildGameScreen(ctx, mapFile)
	if err != nil {
		return err
	}

	if err := scr.Load(chosen); err != nil {
		menus.Message(ctx, "Load failed: "+err.Error())
		return nil
	}

	startGameMusic(ctx, gameMusicForMap(mapFile))
	if err := scr.Run(); err != nil {
		return err
	}

	if o := scr.Outcome(); o != winloss.Continue {
		return menus.RunGameOver(ctx, scr.GameOverData())
	}

	return nil
}

// menuLoadOps is the Load Game screen backend used from the main menu:
// it reads slot metadata (via debugSaveLoadOps) and records the slot the player loads,
// deferring the actual restore to loadGameFromMainMenu (which has the map).
type menuLoadOps struct {
	debugSaveLoadOps
	onLoad func(slot int)
}

// Load records the chosen slot and returns so RunSaveLoad exits.
func (o *menuLoadOps) Load(slot int) error {
	if o.onLoad != nil {
		o.onLoad(slot)
	}

	return nil
}

// loadedSaveMap reads a save slot's embedded .WRL map file name,
// resolving the slot via FindSlot so legacy (slot%02d) saves load too.
func loadedSaveMap(baseDir string, slot int) (string, error) {
	path, ok := saveload.FindSlot(baseDir, slot)
	if !ok {
		return "", fmt.Errorf("no save in slot %d", slot)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	snap, err := saveload.Load(f)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(snap.MapName), nil
}

// game screen debug modes for runGameScreen.
type gameScreenMode int

const (
	gameScreenPlain gameScreenMode = iota
	gameScreenDemo
	gameScreenDeath
	gameScreenWin
	gameScreenLose
	gameScreenRandom
	gameScreenRandom2
	gameScreenPopup
	gameScreenAssets
	gameScreenSoldier
	gameScreenAPCUnit
	gameScreenHeadRotate
	gameScreenCommando
	gameScreenMines
	gameScreenBulldozer
	gameScreenAir
	gameScreenMissile
	gameScreenSpotted
	gameScreenFactoryRun
	gameScreenTransport
	gameScreenCustom
	gameScreenAliens
	gameScreenLevelUp
	gameScreenSubmarine
	gameScreenNavalFire
	gameScreenParkedBuild
	gameScreenReactionFire
)

// runGameScreenShadow is the unit shadow inspection view (-emulate shadow):
// 1:1 zoom, one constructor on land, no fog, for visually checking the unit ground shadow render against the
// original (brightness level 5).
func runGameScreenShadow(ctx *menus.Context, worldIdx int) error {
	name, err := ctx.Lib.ReadString(menus.WorldTag(worldIdx))

	if err != nil {
		return err
	}

	m, err := world.LoadMap(resfile.DataPath(ctx.DataDir, strings.TrimSpace(name)))
	if err != nil {
		return err
	}

	basePal, bg := loadGameScreenArt(ctx)
	scr := gamescreen.New(ctx.Sys, ctx.Plat, m, basePal, bg, ctx.Lib, ctx.Assets.Strings)
	scr.SetPortraitDir(ctx.DataDir)
	scr.SetAttribs(ctx.Assets.Attribs)
	scr.SetClans(ctx.Assets.Clans)
	scr.DebugSpawnShadow()

	return scr.Run()
}

// These values record explicitly supplied Custom Game flags for the quick-start benches. Unset flags leave
// each bench at its default.
var (
	benchResourceLevels  *[3]int // -raw/-fuel/-gold as 0/1/2 levels; nil = default allocator params
	benchStartGold       = -1    // -startcredit gold amount; negative = no reserve
	benchVictoryProvided bool    // -score or -duration is set: a real victory outranks endless
)

// applyBenchCustomFlags applies explicit Custom Game flags to the random benches before RandomStart or
// RandomStart2 reads them. The custom emulate bench keeps its fixed setup.
func applyBenchCustomFlags(scr *gamescreen.Screen, ctx *menus.Context) {
	if benchResourceLevels != nil {
		scr.SetResourceLevels(benchResourceLevels[0], benchResourceLevels[1], benchResourceLevels[2])
	}

	if benchStartGold >= 0 {
		scr.SetStartGold(benchStartGold)
	}

	if benchVictoryProvided /* && ctx.Settings != nil: note: unused code in Go port, ctx.Settings is never nil */ {
		scr.SetVictory(winloss.VictoryType(ctx.Settings.Game.VictoryType), ctx.Settings.Game.VictoryLimit)
	}
}

// runGameScreenBuild is the construction view (-emulate build): lands a small base with a
// constructor and engineer mid build to check placed buildings, the green/red build cursor
// and construction sounds. Goes through the fully wired buildGameScreen so SFX/voice work.
func runGameScreenBuild(ctx *menus.Context, worldIdx int) error {
	name, err := ctx.Lib.ReadString(menus.WorldTag(worldIdx))

	if err != nil {
		return err
	}

	scr, err := buildGameScreen(ctx, strings.TrimSpace(name))
	if err != nil {
		return err
	}

	scr.DebugBuild()

	return scr.Run()
}
