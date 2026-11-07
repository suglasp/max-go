// menu.go : boots the main menu, selects the audio backend, and configures quick start benches.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/audio"
	miniaudioaudio "github.com/suglasp/max-go/internal/audio/miniaudio"
	soloudaudio "github.com/suglasp/max-go/internal/audio/soloud"
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gamescreen"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/intro"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/winloss"
)

// teamClanList pulls each team's chosen clan out of the setup params into a slice
// for gamescreen.SetTeamClans (game convention: 0 ClanRandom, 1..8 a clan).
func teamClanList(p *game.StartParams) []int {
	clans := make([]int, game.TeamCount)
	for i := 0; i < game.TeamCount && i < len(p.Teams); i++ {
		clans[i] = p.Teams[i].Clan
	}
	return clans
}

// installGlobalCursor sets the shared GNW system's Hand cursor for every menu.
// The game screen overrides it per frame and restores it on exit.
// C++: Cursor_Init + Cursor_SetCursor(CURSOR_HAND).
func installGlobalCursor(sys *gnw.System, lib cursors.Reader) {
	cat, err := cursors.LoadCatalog(lib)

	if err != nil {
		dbg.Printf("cursors: %v", err)
	}

	if c := cat.Get(cursors.Hand); c != nil {
		sys.SetCursor(c.Frame(0), c.HotX, c.HotY, c.Transparent)
	}
}

// menuBoot holds a prepared GNW system, its menu loop, and resources.
// run starts a new platform loop, while attach transfers an existing loop from the data bootstrap phase.
type menuBoot struct {
	sys     *gnw.System
	logic   func() error
	closers []func()
}

// close releases the menu's resources (resource library, audio backend).
func (b *menuBoot) close() {
	for _, fn := range b.closers {
		fn()
	}
}

// run starts the platform loop with the menu attached and closes the menu's
// resources when the loop ends.
func (b *menuBoot) run(p *platform.Platform) error {
	defer b.close()
	return b.sys.Run(p, b.logic)
}

// attach connects the menu to a running platform loop from inside its frame
// hook. The caller must close the menu when the loop ends.
func (b *menuBoot) attach(p *platform.Platform) {
	b.sys.Attach(p, b.logic)
}

// runMainMenu loads the resource library and starts the main-menu loop. Unavailable selections show a dialog and return to the menu.
func runMainMenu(p *platform.Platform, cfg *settings.Settings, cfgPath string, fontSet *fonts.Set, jumpMenu string, skipIntro, noMovie, noSplash bool) error {
	boot, err := prepareMainMenu(p, cfg, cfgPath, fontSet, jumpMenu, skipIntro, noMovie, noSplash)
	if err != nil {
		return err
	}
	return boot.run(p)
}

// prepareMainMenu loads the resources, assets, audio, and GNW state needed by the menu.
// It avoids Ebitengine calls, so a worker goroutine may run it while the platform loop is active.
func prepareMainMenu(p *platform.Platform, cfg *settings.Settings, cfgPath string, fontSet *fonts.Set, jumpMenu string, skipIntro, noMovie, noSplash bool) (*menuBoot, error) {
	dir, err := cfg.ResolveDataDir()
	if err != nil {
		return nil, err
	}

	lib, err := resfile.OpenDir(dir)
	if err != nil {
		return nil, err
	}

	gameAssets, err := assets.Load(lib, dir, cfg.Language)
	if err != nil {
		lib.Close()
		return nil, err
	}

	dbg.Printf("assets: %d unit types, %d clans, %d strings", len(gameAssets.Attribs), len(gameAssets.Clans), len(gameAssets.Strings))

	sound := audio.New(newAudioBackend(*cfg), lib, dir, cfg.Audio)

	sys := gnw.New(p.Screen(), fontSet)

	installGlobalCursor(sys, lib)

	ctx := &menus.Context{
		Plat:    p,
		Sys:     sys,
		Lib:     lib,
		Assets:  gameAssets,
		Audio:   sound,
		DataDir: dir,
		Rand:    rand.New(rand.NewSource(time.Now().UnixNano())),

		Settings:     cfg,
		SettingsPath: cfgPath,
	}

	flow := &menuFlow{
		ctx:       ctx,
		cfg:       cfg,
		cfgPath:   cfgPath,
		jumpMenu:  jumpMenu,
		skipIntro: skipIntro,
		noMovie:   noMovie,
		noSplash:  noSplash,
	}

	return &menuBoot{
		sys:   sys,
		logic: flow.menuLoop,
		closers: []func(){
			sound.Close,
			func() { lib.Close() },
		},
	}, nil
}

// menuFlow holds the menu context, live settings, save path, and boot flags
// shared by the main menu flows. Its start methods run New Game entries, while
// menuLoop drives the main menu.
type menuFlow struct {
	ctx       *menus.Context
	cfg       *settings.Settings
	cfgPath   string
	jumpMenu  string
	skipIntro bool
	noMovie   bool
	noSplash  bool
}

// playGameMovie plays a full screen cinematic unless -nomovie is set.
// MAXMVE1 is the custom game intro, and MAXMVE2 is the campaign story intro.
// C++: game_manager.cpp Movie_Play.
func (f *menuFlow) playGameMovie(tag string) {
	if f.noMovie {
		return
	}

	if err := menus.RunMVE(f.ctx, tag); err != nil {
		dbg.Printf("game movie %q: %v", tag, err)
	}
}

// startCustomGame runs options, planet and clan selection, then buys a loadout
// and lands it. It reports whether the player launched a game or cancelled.
// C++: menu_custom_game_menu / GameManager_PlayerMissionSetup.
func (f *menuFlow) startCustomGame(isSinglePlayer bool) (bool, error) {
	params, err := menus.CustomGameFlow(f.ctx, f.cfg, isSinglePlayer)
	if err != nil || params == nil {
		return false, err // CustomGameFlow already steps back through its own screens
	}

	// The original persists the chosen options in its ini file.
	if err := f.cfg.Save(f.cfgPath); err != nil {
		dbg.Printf("settings save: %v", err)
	}

	dbg.Printf("start params: world=%d options=%+v", params.World, params.Options)

	// Purchase starting units with the team's gold budget. This path applies the
	// C++ start_gold value without clan credits. The loadout begins with the free
	// Constructor, Engineer, and Surveyor, the two builders already carrying their
	// default raw materials.
	var lo *menus.CargoLoadout

	for {
		lo = menus.NewCargoLoadout(0, f.cfg.Game.StartGold)
		if err := menus.RunCargo(f.ctx, lo); err != nil {
			if !errors.Is(err, menus.ErrCancelled) {
				return false, err
			}

			// Cancel/ESC on the purchase screen steps back to Choose Players, like
			// every other screen in this flow, instead of abandoning the whole setup
			// back to New Game.
			ok, err := menus.ChoosePlayer(f.ctx, params, isSinglePlayer, f.cfg.Player.Name)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil // Cancel on Choose Players itself: step back further, to New Game
			}

			continue
		}

		break
	}

	// Load the chosen planet and land with the purchased loadout.
	name, err := f.ctx.Lib.ReadString(menus.WorldTag(params.World))
	if err != nil {
		return false, err
	}

	scr, err := buildGameScreen(f.ctx, strings.TrimSpace(name))
	if err != nil {
		return false, err
	}

	scr.SetNoMovie(f.noMovie)         // honour -nomovie for the landing cinematic
	scr.SetLandingMovieTag("MAXMVE1") // custom game intro, played when the player CONFIRMS the landing site

	// Drive the AI at the difficulty the player chose on the setup screen
	// (settings.Game.Opponent). A -difficulty launch pre seeds that panel at boot
	// (the main.go fold into cfg.Game.Opponent), so the flag shows there and a
	// LATER panel edit wins - the raw flag must not silently override what the
	// player just picked. No banner: the menu game already shows the tier.
	scr.SetAIDifficulty(f.cfg.Game.Opponent)

	scr.SetAlienDerelicts(f.cfg.Game.AlienDerelicts)                                                // neutral alien derelict density (setup screen)
	scr.SetResourceLevels(f.cfg.Game.RawResource, f.cfg.Game.FuelResource, f.cfg.Game.GoldResource) // raw/fuel/gold map richness (setup screen / -raw -fuel -gold)
	scr.SetPlayMode(params.Options.PlayMode)                                                        // turn based or simultaneous, per the setup screen / -simultane

	// Carry the rest of the setup screen choices into the launch: the win condition,
	// the per turn clock, and each team's chosen clan (a random 0 stays random).
	// C++: GameManager_GameSetup.
	//
	// The opponent COUNT is not one of them. The setup screen's "Computer Opponent"
	// panel is the DIFFICULTY tier (Clueless..God, applied above); the original picks
	// the team types on its separate player setup screen, which is not ported yet. So a
	// custom game lands ONE computer opponent unless -npc asks for more.
	scr.SetCustomOpponents(npcCount())
	scr.SetVictory(winloss.VictoryType(params.Options.VictoryType), params.Options.VictoryLimit)

	if params.Options.TurnTimer > 0 {
		scr.SetTurnTimer(params.Options.TurnTimer)
	}

	// The End Turn grace: in simultaneous play, the seconds the player still has once the
	// opponents are done acting. The setup screen has always offered it; nothing read it.
	scr.SetEndTurnTimer(params.Options.EndTurn)

	scr.SetTeamClans(teamClanList(params))
	// The landing credit every team gets. The player already spent theirs on the
	// purchase screen (CustomStart takes the leftover below); a computer team pays for
	// its landing force out of the same budget and spends the rest on upgrades, so it
	// no longer fights the whole game on base attributes. C++: the start_gold setting
	// seeding each team's mission supplies.
	scr.SetStartGold(f.cfg.Game.StartGold)
	// The upgrades bought on the purchase screen, applied to the player's team BEFORE it
	// lands, so the units the gold went into deploy improved (and so does everything the
	// team builds later). C++: CargoMenu::CommitUpgradeChanges.
	scr.SetTeamUpgrades(0, lo.Upgrades, lo.UpgradeGold)
	scr.CustomStart(lo.Units, lo.Cargos, lo.TeamGold, time.Now().UnixNano())
	scr.SetRandomGameGoal() // GOAL button info dialog (single player; built from the resolved state)
	startGameMusic(f.ctx, gameMusicTag(params.World))

	// A win or loss shows the end of game score screen, the same as a mission or a
	// quick start; an ESC to menu abandon skips it (Outcome stays Continue).
	// C++ ported : menu.cpp - menu_wrap_up_game after a custom skirmish.
	return true, runGameOverAfter(f.ctx, scr)
}

// launchMission deploys units at their scripted positions through MissionStart.
// If the retail save body fails to parse, it uses a header only custom landing.
// teamTypesFn maps the mission header to the appropriate team controllers.
func (f *menuFlow) launchMission(m missionfile.Mission, teamTypesFn func(*missionfile.Info) []game.TeamType, category game.MissionCategory) (winloss.Outcome, error) {
	f.ctx.Sys.FlushInput() // drop the menu's confirm key/click so it can't leak into the game
	world := int(m.Info.WorldIndex)
	name, err := f.ctx.Lib.ReadString(menus.WorldTag(world))

	if err != nil {
		return winloss.Continue, err
	}

	scr, err := buildGameScreen(f.ctx, strings.TrimSpace(name))
	if err != nil {
		return winloss.Continue, err
	}

	scr.SetNoMovie(f.noMovie)               // honour -nomovie for mission win/lose movies
	scr.SetPlayMode(f.cfg.Game.PlayMode)    // turn based or simultaneous, per the campaign/mission setup screen / -simultane
	scr.SetEndTurnTimer(f.cfg.Game.EndTurn) // the simultaneous End Turn grace period
	scr.SetMissionCategory(category)        // training trims the build roster, locks the AI and speaks the build prompt
	// WHICH mission of that kind: the header's mission_index is what its scripted win and
	// loss conditions are keyed on (campaign 1 is not won the way campaign 8 is).
	scr.SetMissionIndex(int(m.Info.MissionIndex))
	scr.SetAIDifficulty(missionDifficulty(category, f.cfg.Game.Opponent))
	seed := int64(m.Info.RNGSeed)
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	w, h := scr.MapSize()
	if units, res, rules, err := loadMissionState(m.SavePath, w, h, m.Info); err == nil {
		dbg.Printf("start mission %q: world=%d seed=%d units=%d victory=%d/%d (scripted)", m.Title, world, seed, len(units), rules.VictoryType, rules.VictoryLimit)
		scr.MissionStart(units, teamTypesFn(m.Info), res, rules, seed)
	} else {
		dbg.Printf("start mission %q: world=%d seed=%d (header-only, body parse failed: %v)", m.Title, world, seed, err)
		scr.CustomStart(nil, nil, 200, seed) // header only fallback: a modest default gold reserve
	}

	scr.SetMissionGoal(m.Title, m.Description) // light the GOAL button with the mission briefing (retail DESCR file)
	startGameMusic(f.ctx, gameMusicTag(world))

	if err := scr.Run(); err != nil {
		return winloss.Continue, err
	}

	o := scr.Outcome()

	if o != winloss.Continue {
		if err := menus.RunGameOver(f.ctx, scr.GameOverData()); err != nil {
			return o, err
		}
	}

	return o, nil
}

// runMissionFlow moves from the Game Setup list to mission setup and launch.
// Cancelling setup preserves the list selection, while cancelling the list
// returns to New Game. C++: GameSetupMenu_Menu loop.
func (f *menuFlow) runMissionFlow(title string, missions []missionfile.Mission, teamTypesFn func(*missionfile.Info) []game.TeamType, category game.MissionCategory) (bool, error) {
	sel := 0
	for {
		idx, start, err := menus.RunGameSetup(f.ctx, title, missions, sel, 0, helpSectionFor(category)) // 0 = all unlocked

		if err != nil {
			return false, err
		}

		if !start {
			return false, nil // Cancel on the list -> back to the New Game menu
		}
		sel = idx // remember the choice so a step back restores it

		// Flush the Start key/click so a held Return (or its key repeat) cannot
		// leak into the setup screen and instantly confirm it.
		f.ctx.Sys.FlushInput()

		// A training mission hides the Computer Opponent panel: its AI tier is locked to
		// the passive one, so offering the picker there would be a lie (missionDifficulty).
		mode := menus.GameConfigModeMission
		if category == game.MissionTraining {
			mode = menus.GameConfigModeTraining
		}

		done, err := menus.GameConfig(f.ctx, &f.cfg.Game, mode)

		if err != nil {
			return false, err
		}

		if !done {
			continue // Cancel on the setup screen -> back to the list
		}

		if err := f.cfg.Save(f.cfgPath); err != nil {
			dbg.Printf("settings save: %v", err)
		}
		_, err = f.launchMission(missions[idx], teamTypesFn, category) // scenarios/training don't progress on the outcome

		return true, err
	}
}

// startScenario lists retail SCE scenarios, shows the selected briefing and
// setup options, then launches the mission as a standalone game.
// C++: GameSetupMenu -> menu_options_menu.
func (f *menuFlow) startScenario(title string) (bool, error) {
	missions, err := missionfile.ListScenarios(f.ctx.DataDir)

	if err != nil {
		return false, err
	}

	if len(missions) == 0 {
		menus.Message(f.ctx, "No scenarios found in the game data.")

		return false, nil
	}

	return f.runMissionFlow(title, missions, missionTeamTypes, game.MissionScenario)
}

// startMultiScenario launches a retail MPS scenario as a single player game.
// The local player takes the first team, and missionTeamTypesSolo assigns the
// remaining teams to the computer.
func (f *menuFlow) startMultiScenario() (bool, error) {
	missions, err := missionfile.ListMultiScenarios(f.ctx.DataDir)
	if err != nil {
		return false, err
	}

	if len(missions) == 0 {
		menus.Message(f.ctx, "No custom scenarios found in the game data.")

		return false, nil
	}

	return f.runMissionFlow("Custom Scenarios", missions, missionTeamTypesSolo, game.MissionMultiScenario)
}

// startCampaign lists unlocked missions, opens setup, and launches the chosen
// mission. A win unlocks the next mission before the campaign ends at the main
// menu. It reports whether a mission was played. C++: menu_campaign.
func (f *menuFlow) startCampaign() (bool, error) {
	missions, err := missionfile.ListCampaign(f.ctx.DataDir)

	if err != nil {
		return false, err
	}

	if len(missions) == 0 {
		menus.Message(f.ctx, "No campaign missions found in the game data.")

		return false, nil
	}

	sel := 0
	if f.cfg.Game.LastCampaign >= 1 {
		sel = f.cfg.Game.LastCampaign - 1 // resume at the furthest unlocked mission
	}

	for {
		unlocked := f.cfg.Game.LastCampaign
		if unlocked < 1 {
			unlocked = 1 // an old settings.json without the field: only mission 1
		}

		idx, start, err := menus.RunGameSetup(f.ctx, "Campaign Game", missions, sel, unlocked, helpSectionFor(game.MissionCampaign))
		if err != nil {
			return false, err
		}

		if !start {
			return false, nil // Cancel on the list -> back to the New Game menu
		}

		sel = idx
		f.ctx.Sys.FlushInput()
		done, err := menus.GameConfig(f.ctx, &f.cfg.Game, menus.GameConfigModeMission)

		if err != nil {
			return false, err
		}

		if !done {
			continue // Cancel on the setup screen -> back to the list
		}

		if err := f.cfg.Save(f.cfgPath); err != nil {
			dbg.Printf("settings save: %v", err)
		}

		// Only the first campaign mission has a briefing story. SC_C0001 shows the
		// Secret Council text and MAXMVE2.MVE. Both replay on each launch of mission
		// one, and later missions skip them.
		if idx == 0 {
			if err := menus.RunCampaignIntro(f.ctx); err != nil {
				dbg.Printf("campaign briefing: %v", err)
			}

			f.playGameMovie("MAXMVE2")
		}

		outcome, err := f.launchMission(missions[idx], missionTeamTypes, game.MissionCampaign)
		if err != nil {
			return true, err
		}

		if outcome == winloss.PlayerWon {
			// The ending story follows the final mission because WIN9.CAM is the only
			// ending file. The score screen already ran
			// inside launchMission. C++: menu.cpp menu_wrap_up_game victory branch.
			if err := menus.RunCampaignVictory(f.ctx, f.ctx.DataDir, missions[idx].Index); err != nil {
				dbg.Printf("campaign victory: %v", err)
			}

			// Unlock the next mission (last_campaign = next index + 1, 1-based). It is
			// preselected on the next visit to the campaign list, which reopens at the
			// furthest unlocked mission. C++: menu.cpp game over handler.
			if next := idx + 2; next <= len(missions) && f.cfg.Game.LastCampaign < next {
				f.cfg.Game.LastCampaign = next
				if err := f.cfg.Save(f.cfgPath); err != nil {
					dbg.Printf("settings save: %v", err)
				}
				dbg.Printf("campaign: unlocked mission %d (last_campaign=%d)", next, f.cfg.Game.LastCampaign)
			}
		}

		// The mission is over (won, lost, or abandoned), so the campaign ends here and
		// the caller returns to the main menu. The story mode is no exception: the
		// retail game leaves the campaign the same way it leaves any other game.
		// C++ ported : menu.cpp - menu_wrap_up_game sets GAME_STATE_3_MAIN_MENU for
		// MISSION_CATEGORY_CAMPAIGN too, and only unlocks the next mission on a win.
		return true, nil
	}
}

// startTraining runs the .TRA training missions through the same flow.
func (f *menuFlow) startTraining() (bool, error) {
	missions, err := missionfile.ListTraining(f.ctx.DataDir)
	if err != nil {
		return false, err
	}

	if len(missions) == 0 {
		menus.Message(f.ctx, "No training missions found in the game data.")
		return false, nil
	}

	return f.runMissionFlow("Training Missions", missions, missionTeamTypes, game.MissionTraining)
}

// menuLoop is the main menu itself: the boot intro sequence, then the menu
// selection loop until quit. It is the logic func the gnw system runs.
func (f *menuFlow) menuLoop() error {
	f.ctx.Sys.GetInput() // wait for the first frame (console handling)
	showConsoleOnce(*f.cfg)

	f.playBootIntro()

	if err := jumpToMenu(f.ctx, f.cfg, f.jumpMenu, f.noMovie); err != nil {
		return err
	}

	attractIndex := 0 // rotates over the SAVE<N>.DMO demos across idle timeouts

	for {
		sel, err := menus.MainMenu(f.ctx)
		if err != nil {
			return err
		}
		switch sel {
		case menus.MainExit:
			return gnw.ErrQuit

		case menus.MainDemo:
			// The menu sat idle for the attract timeout: play the next demo,
			// then fall back to the menu (its timer re arms on redraw).
			// C++: main_menu -> play_attract_demo (menu.cpp).
			if err := runAttractDemo(f.ctx, &attractIndex, f.noMovie); err != nil {
				return err
			}

		case menus.MainLoadGame:
			if err := loadGameFromMainMenu(f.ctx); err != nil {
				return err
			}

		case menus.MainSetup:
			// The standalone main menu Setup screen (SETUPPIC), distinct from the
			// in game Preferences (PREFSPIC). Persist changes so they apply next game.
			if err := menus.RunSetup(f.ctx, gamescreen.MainWindow); err != nil {
				return err
			}

			if err := f.cfg.Save(f.cfgPath); err != nil {
				dbg.Printf("settings save: %v", err)
			}

		case menus.MainCredits:
			if err := menus.RunCredits(f.ctx); err != nil {
				return err
			}

		case menus.MainIntro:
			// The replay button plays the MAXINT movie and nothing else. The boot
			// logos belong to the boot sequence, not to a deliberate replay.
			// C++ ported : menu.cpp - main_menu SHIFT+I calls Movie_PlayIntro alone.
			if err := menus.RunIntro(f.ctx, intro.Options{PlayMovie: true}); err != nil {
				dbg.Printf("intro replay: %v", err)
			}

		case menus.MainNewGame:
			if err := f.runNewGameMenu(); err != nil {
				return err
			}

		case menus.MainMultiplayer:
			// Multiplayer is postponed (PROJECT.txt). The submenu code is kept
			// (reachable via -emulate multiplayer) but the main button just say so.
			menus.Message(f.ctx, "No multiplayer support in this version.")

		default:
			menus.NotImplemented(f.ctx, sel.String())
		}
	}
}

// playBootIntro runs the boot sequence in front of the main menu.
//
// Normal boot plays the movie and splash logos. Quick start shows only the
// splash, while debug jumps skip both. Command line flags and the saved
// intro_movie preference can suppress either stage.
func (f *menuFlow) playBootIntro() {
	quickStart := f.jumpMenu == "random" || strings.HasPrefix(f.jumpMenu, "loadmission:") || strings.HasPrefix(f.jumpMenu, "loadsave:")
	if !f.skipIntro && (f.jumpMenu == "" || quickStart) {
		opts := intro.Options{
			PlayMovie: introMovieEnabled(f.skipIntro, f.noMovie, f.cfg.Game.IntroMovie, quickStart),
			ShowLogos: !f.noSplash,
		}

		if opts.PlayMovie || opts.ShowLogos {
			if err := menus.RunIntro(f.ctx, opts); err != nil {
				dbg.Printf("intro: %v", err)
			}
		}
	}
}

// runNewGameMenu drives the New Game submenu.
//
// Keep New Game open while the player steps back out of a setup screen, so
// a cancel lands here rather than at the main menu. Once a game has actually
// been played, though, its end belongs to the main menu.
// C++: menu_new_game_menu_loop.
func (f *menuFlow) runNewGameMenu() error {
	for {
		ng, err := menus.NewGameMenu(f.ctx)
		if err != nil {
			return err
		}

		if ng == menus.NewGameCancel {
			return nil // Cancel/ESC on the New Game menu -> main menu
		}

		played := false

		switch ng {
		case menus.NewGameCustom:
			played, err = f.startCustomGame(true)

		case menus.NewGameTraining:
			played, err = f.startTraining()

		case menus.NewGameStandAlone:
			played, err = f.startScenario("Stand Alone Missions")

		case menus.NewGameScenarios:
			played, err = f.startMultiScenario()

		case menus.NewGameCampaign:
			played, err = f.startCampaign()

		case menus.NewGameTips:
			err = menus.RunTips(f.ctx)
		}

		if err != nil {
			return err
		}

		if played {
			// The game ran to a win, a loss, or an ESC abandon. Every ending
			// goes back to the main menu, never to the list it was launched from.
			// C++ ported : menu.cpp - menu_wrap_up_game sets GAME_STATE_3_MAIN_MENU.
			return nil
		}
	}
}

// introMovieEnabled applies command line and saved preferences to the boot
// movie. Quick start launches show only the splash, and command line options
// take precedence over game.intro_movie.
func introMovieEnabled(skipIntro, noMovie, introMovie, quickStart bool) bool {
	return !skipIntro && !noMovie && !quickStart && introMovie
}

// noSound disables all audio (set by -nosound)
var noSound bool

// showAStarPath draws the active move order's full A* route as white cells (set by -set-a-star)
var showAStarPath bool

// difficultyOverride returns the requested AI difficulty from 0 through 5.
// Without -difficulty, quick start uses AVERAGE and menu games use the saved
// value. See benchDifficulty.
var difficultyOverride = -1

// helpSectionFor names the help database section for a mission list: the original keeps
// one per flavour, so the ? button on the training list explains training and the one on
// the campaign list explains the campaign.
//
// C++ ported : gamesetupmenu.cpp:237 - GameSetupMenu::EventHelp
func helpSectionFor(category game.MissionCategory) string {
	switch category {
	case game.MissionTraining:
		return "TRAINING_MENU_SETUP"

	case game.MissionCampaign:
		return "CAMPAIGN_MENU"

	case game.MissionMultiScenario:
		return "MULTI_SCENARIO_MENU"

	default:
		return "STAND_ALONE_MENU_SETUP"
	}
}

// missionDifficulty resolves a mission's AI tier. Training missions use the passive Clueless tier; other missions use the requested or saved choice.
//
// C++: the training missions ship their own passive game rules; the difficulty setting
// does not reach them.
func missionDifficulty(category game.MissionCategory, chosen int) int {
	if category == game.MissionTraining {
		return ai.OpponentClueless
	}

	if difficultyOverride >= 0 {
		return difficultyOverride
	}

	return chosen
}

// benchDifficulty resolves the AI difficulty for the quick start benches (-random /
// -random2 / -emulate custom): the -difficulty override when set, else AVERAGE (2).
func benchDifficulty() int {
	if difficultyOverride >= 0 {
		return difficultyOverride
	}

	return ai.OpponentAverage
}

// npcOverride is the -npc value: how many computer opponents a game lands. See
// npcCount. The default is 1.
var npcOverride = 1

// derelictsLevel returns the alien derelict density for random benches. Custom
// games read cfg.Game.AlienDerelicts because the benches have no setup screen.
var derelictsLevel = 1

/* // note: unused code in Go port
// benchDerelictsLevel repeated resourceDensity (custom_flags.go) for every validated -derelicts value; main now calls resourceDensity.
// benchDerelictsLevel maps -derelicts to the density used by random benches.
// An empty value defaults to Rare, while None disables derelicts.
func benchDerelictsLevel(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "none":
		return 0

	case "common":
		return 2

	default: // "" (default) or "rare"
		return 1
	}
}
*/

// npcCount resolves how many computer opponents a game fields, clamped to 1..3
// (there are 4 team slots: one player plus up to three NPCs). It answers for a
// custom game as well as the -random / -random2 benches, since the setup screen
// carries no opponent COUNT panel: the only way to ask for more than one is -npc.
func npcCount() int {
	n := npcOverride
	if n < 1 {
		n = 1
	}

	if n > game.TeamCount-1 {
		n = game.TeamCount - 1
	}

	return n
}

// newAudioBackend creates the configured audio backend, falling back
// from SoLoud to Ebitengine/Oto, then to silence if no audio device is
// available.
func newAudioBackend(cfg settings.Settings) audio.Backend {
	if noSound {
		dbg.Printf("audio backend: silent (-nosound)")

		return audio.NewSilentBackend()
	}

	if cfg.Audio.Backend == settings.BackendMiniAudio {
		be, err := miniaudioaudio.New(miniAudioDLLPath())
		if err == nil {
			dbg.Printf("audio backend: miniaudio")

			return be
		}

		dbg.Printf("audio backend miniaudio unavailable, trying ebiten/oto: %v", err)
	}

	if cfg.Audio.Backend == settings.BackendSoLoud {
		be, err := soloudaudio.NewWithBackend(soLoudDLLPath(), cfg.Audio.SoLoudBackend)

		if err == nil {
			dbg.Printf("audio backend: soloud (device %q); movie audio via ebiten/oto", cfg.Audio.SoLoudBackend)
			return be
		}

		dbg.Printf("audio backend soloud (device %q) unavailable, trying ebiten/oto: %v", cfg.Audio.SoLoudBackend, err)
	}

	// "default" (and any unrecognized value normalized by Validate) lands here:
	// the native Ebitengine/Oto backend.
	be, err := audio.NewEbitenBackendChecked()
	if err == nil {
		dbg.Printf("audio backend: ebiten/oto")

		return be
	}

	dbg.Printf("audio backend ebiten/oto unavailable, disabling audio: %v", err)

	return audio.NewSilentBackend()
}

// Resolve the Soloud engine and libraries
func soLoudDLLPath() string {
	if path := os.Getenv("MAX_SOLOUD_DLL"); path != "" {
		return path
	}

	baseDir, err := settings.BaseDir()
	if err != nil {
		return ""
	}

	name := "libsoloud_x64.so"
	if runtime.GOOS == "windows" {
		name = "soloud_x64.dll"

		if runtime.GOARCH == "386" {
			name = "soloud_x86.dll"
		}
	} else if runtime.GOOS == "darwin" {
		name = "libsoloud_x64.dylib"
	}

	path := filepath.Join(baseDir, name)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

// Resolve the miniAudio wrapper engine and libraries
func miniAudioDLLPath() string {
	if path := os.Getenv("MAX_MINIAUDIO_DLL"); path != "" {
		return path
	}

	baseDir, err := settings.BaseDir()
	if err != nil {
		return ""
	}

	name := "libminiaudio_x64.so"
	if runtime.GOOS == "windows" {
		name = "miniaudio_x64.dll"
	} else if runtime.GOOS == "darwin" {
		name = "libminiaudio_x64.dylib"
	}

	path := filepath.Join(baseDir, name)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}
