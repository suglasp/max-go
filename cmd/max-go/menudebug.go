// menudebug.go : provides debug screen jumps, sprite viewing, and save or load stubs.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gamescreen"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/intro"
	"github.com/suglasp/max-go/internal/mapeditor"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/saveload"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/settings"
	"github.com/suglasp/max-go/internal/sprites"
)

// debugSaveLoadOps reads slot metadata for save and load debug screens. It
// stubs mutation because no live game exists in these targets.
type debugSaveLoadOps struct {
	baseDir string
}

// SlotInfo reads slot metadata from the saves folder for the debug screen.
func (o debugSaveLoadOps) SlotInfo(slot int) (fileName, saveName, typeName string, inUse bool) {
	fileName = fmt.Sprintf("save%d", slot)
	path, ok := saveload.FindSlot(o.baseDir, slot) // save<N>, or legacy slot%02d
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
	if name == "" {
		name = fmt.Sprintf("Turn %d", snap.Turn)
	}

	return fileName, name, "Custom", true
}

// Save logs the request because the debug target has no game to capture.
func (o debugSaveLoadOps) Save(slot int, name string) error {
	dbg.Printf("debug save: slot %d name %q (no live game)", slot, name)

	return nil
}

// Load logs the request because the debug target has no game to restore.
func (o debugSaveLoadOps) Load(slot int) error {
	dbg.Printf("debug load: slot %d (no live game)", slot)

	return nil
}

// jumpToMenu implements the -emulate debug flag: run one screen directly
// so menu chunks can be verified with -screenshot.
func jumpToMenu(ctx *menus.Context, cfg *settings.Settings, name string, noMovie bool) error {
	var err error

	// Launch the path or slot attached to -mission or -save.
	if ref, ok := strings.CutPrefix(name, "loadmission:"); ok {
		return launchMissionFile(ctx, ref)
	}
	if slot, ok := strings.CutPrefix(name, "loadsave:"); ok {
		return launchSaveSlot(ctx, slot)
	}

	// Most targets are grouped by theme into their own dispatchers below, so this
	// switch keeps the one off screens.
	if handled, e := jumpToMenuScreen(ctx, cfg, name); handled {
		return e
	}

	if handled, e := jumpToGameState(ctx, cfg, name, noMovie); handled {
		return e
	}

	if handled, e := jumpToUnitBench(ctx, name, noMovie); handled {
		return e
	}

	switch name {
	case "":
		return nil

	case "mapview":
		err = mapViewDebug(ctx, cfg)

	case "drawmap":
		err = drawMapDebug(ctx, cfg)

	case "mapeditor":
		err = mapeditor.Run(ctx, cfg)

	case "ending":
		// The campaign victory story shown after the final mission is won: the
		// WIN9.CAM ending text typed over an ENDGAME backdrop with WINR_MSC, the
		// same screen the real campaign win path shows. Index 9 = the last mission.
		err = menus.RunCampaignVictory(ctx, ctx.DataDir, 9)

	case "bevelart":
		err = runBevelArtDebug(ctx)

	case "options":
		err = runOptionsDebug(ctx)

	case "setup":
		// The standalone main menu Setup screen (SETUPPIC), centred on the screen.
		err = menus.RunSetup(ctx, gamescreen.MainWindow)

	case "prefs-game", "loadgame", "research", "alloc", "upgrade", "transfer", "selfdestruct", "repairshop", "reports", "help":
		err = runInGameDialogDebug(ctx, cfg, name)

	case "saveload", "save":
		_, err = menus.RunSaveLoad(ctx, debugSaveLoadOps{baseDir: ctx.Plat.BaseDir()}, true)

	case "load":
		_, err = menus.RunSaveLoad(ctx, debugSaveLoadOps{baseDir: ctx.Plat.BaseDir()}, false)

	case "cargo":
		err = runCargoDebug(ctx)

	case "construct", "construct-eng":
		err = runConstructionDebug(ctx, name)

	case "factorybuild":
		err = runFactoryBuildDebug(ctx)

	case "scrollbar":
		err = runScrollbarDebug(ctx)

	case "listbox":
		err = runListboxDebug(ctx)

	case "status":
		err = runUnitStatusDebug(ctx)

	case "credits":
		err = menus.RunCredits(ctx)

	case "intro":
		// Play only the intro MVE, without either splash logo, to isolate movie
		// playback from the full startup sequence.
		err = menus.RunIntro(ctx, intro.Options{PlayMovie: true})

	case "splash":
		// Just the MLOGO splash with the palette fade in (no movie, no ILOGO).
		err = menus.RunIntro(ctx, intro.Options{PlayMovie: false, ShowLogos: true})

	default:
		if tag, ok := strings.CutPrefix(name, "sprite:"); ok {
			err = spriteViewDebug(ctx, cfg, tag)
		} else {
			dbg.Printf("unknown -emulate %q", name)
		}
	}

	return err
}

// jumpToMenuScreen runs the -emulate targets that open a front end menu screen
// on their own, and reports whether the name was one of them.
func jumpToMenuScreen(ctx *menus.Context, cfg *settings.Settings, name string) (bool, error) {
	var err error

	switch name {
	case "planetselect":
		var world int
		world, err = menus.PlanetSelect(ctx, 0)
		dbg.Printf("planetselect: %d", world)

	case "newgame":
		_, err = menus.NewGameMenu(ctx)

	case "gamesetup":
		var missions []missionfile.Mission
		if missions, err = missionfile.ListScenarios(ctx.DataDir); err == nil {
			_, _, err = menus.RunGameSetup(ctx, "Custom Scenarios", missions, 0, 0, "STAND_ALONE_MENU_SETUP")
		}

	case "training":
		var missions []missionfile.Mission
		if missions, err = missionfile.ListTraining(ctx.DataDir); err == nil {
			_, _, err = menus.RunGameSetup(ctx, "Training Missions", missions, 0, 0, "TRAINING_MENU_SETUP")
		}

	case "campaign":
		// Exercise the full campaign transition (list -> setup) so the screen 2
		// handoff can be verified in isolation, stopping before the game launch.
		// Unlock 2 so the list shows the gating (mission 1 live, the rest greyed).
		var missions []missionfile.Mission
		if missions, err = missionfile.ListCampaign(ctx.DataDir); err == nil {
			var idx int
			var start bool
			if idx, start, err = menus.RunGameSetup(ctx, "Campaign Game", missions, 0, 1, "CAMPAIGN_MENU"); err == nil && start {
				ctx.Sys.FlushInput()
				dbg.Printf("campaign setup: opening Options Menu for %q", missions[idx].Title)
				_, err = menus.GameConfig(ctx, &cfg.Game, menus.GameConfigModeMission)
			}
		}

	case "mission":
		// Launch the first scenario at its scripted unit positions: parse
		// the V70 body and deploy via MissionStart, skipping the menu screens.
		var missions []missionfile.Mission
		if missions, err = missionfile.ListScenarios(ctx.DataDir); err == nil && len(missions) > 0 {
			err = emulateMission(ctx, missions[0], missionTeamTypes)
		}

	case "customscenario":
		// Launch the first .MPS Custom Scenario single player (you vs AI) to verify
		// the solo team mapping deploys correctly.
		var missions []missionfile.Mission
		if missions, err = missionfile.ListMultiScenarios(ctx.DataDir); err == nil && len(missions) > 0 {
			err = emulateMission(ctx, missions[0], missionTeamTypesSolo)
		}

	case "tips":
		err = menus.RunTips(ctx) // New Game -> Tips: the paged help text panel

	case "multiplayer":
		_, err = menus.MultiplayerMenu(ctx)

	case "gameconfig":
		_, err = menus.GameConfig(ctx, &cfg.Game, menus.GameConfigModeCustom)

	case "gameconfig-mission":
		_, err = menus.GameConfig(ctx, &cfg.Game, menus.GameConfigModeMission)

	case "chooseplayer":
		params := &game.StartParams{}
		for i := range params.Teams {
			params.Teams[i].Clan = cfg.Player.Clan
		}
		_, err = menus.ChoosePlayer(ctx, params, false, cfg.Player.Name)

	case "clanselect":
		var clan int
		clan, err = menus.ClanSelect(ctx, 0, cfg.Player.Clan)
		dbg.Printf("clanselect: %d", clan)

	default:
		return false, nil
	}

	return true, err
}

// jumpToGameState runs the -emulate targets that boot a whole game screen in a
// given state, and reports whether the name was one of them.
func jumpToGameState(ctx *menus.Context, cfg *settings.Settings, name string, noMovie bool) (bool, error) {
	var err error

	switch name {
	case "random":
		err = runGameScreen(ctx, rand.Intn(menus.WorldCount), gameScreenRandom, noMovie)

	case "random2":
		err = runGameScreen(ctx, rand.Intn(menus.WorldCount), gameScreenRandom2, noMovie)

	case "custom":
		err = runGameScreen(ctx, rand.Intn(menus.WorldCount), gameScreenCustom, noMovie)

	case "game":
		err = runGameScreen(ctx, cfg.Game.World, gameScreenDemo, noMovie)

	case "shadow":
		err = runGameScreenShadow(ctx, cfg.Game.World)

	case "build":
		err = runGameScreenBuild(ctx, cfg.Game.World)

	case "blewup":
		if wname, e := ctx.Lib.ReadString(menus.WorldTag(cfg.Game.World)); e == nil {
			if scr, e2 := buildGameScreen(ctx, strings.TrimSpace(wname)); e2 != nil {
				err = e2
			} else {
				scr.DebugBlowUp()
				err = scr.Run()
			}
		}

	case "colony":
		if wname, e := ctx.Lib.ReadString(menus.WorldTag(cfg.Game.World)); e == nil {
			if scr, e2 := buildGameScreen(ctx, strings.TrimSpace(wname)); e2 != nil {
				err = e2
			} else {
				scr.DebugColonyStart()
				err = scr.Run()
			}
		}

	case "death":
		err = runGameScreen(ctx, cfg.Game.World, gameScreenDeath, noMovie)

	case "win":
		err = runGameScreen(ctx, rand.Intn(menus.WorldCount), gameScreenWin, noMovie)

	case "lose":
		err = runGameScreen(ctx, rand.Intn(menus.WorldCount), gameScreenLose, noMovie)

	case "popup":
		err = runGameScreen(ctx, cfg.Game.World, gameScreenPopup, noMovie)

	default:
		return false, nil
	}

	return true, err
}

// jumpToUnitBench runs the -emulate unit benches, the scenes staged on world 0
// (SNOW_1) to exercise one mechanic, and reports whether the name was one of them.
func jumpToUnitBench(ctx *menus.Context, name string, noMovie bool) (bool, error) {
	var err error

	switch name {
	case "assets":
		// World 0 is SNOW_1: white snow terrain (units read clearly) with water
		// around it for the naval roster.
		err = runGameScreen(ctx, 0, gameScreenAssets, noMovie)

	case "soldier":
		// World 0 (SNOW_1): infantry + commando in all 8 facings, walk cycle animating.
		err = runGameScreen(ctx, 0, gameScreenSoldier, noMovie)

	case "apcunit":
		// World 0 (SNOW_1): a sound test bench: select a unit (idle hum), click to
		// move (drive + stop sounds), right click the enemy tank (fire/hit sounds).
		err = runGameScreen(ctx, 0, gameScreenAPCUnit, noMovie)

	case "headrotate":
		// World 0 (SNOW_1): turreted tanks/buildings vs frozen targets at different
		// angles - fire to verify the head rotates while the hull stays put.
		err = runGameScreen(ctx, 0, gameScreenHeadRotate, noMovie)

	case "transport":
		// World 0 (SNOW_1): a land APC + an air transport, each loaded with two units.
		// Popup Activate opens the Repair Shop screen to pick WHICH carried unit to
		// unload. It ejects through the Depot style build ready exit.
		err = runGameScreen(ctx, 0, gameScreenTransport, noMovie)

	case "commando":
		// World 0 puts a commando among enemy units. Use Disable or Steal, then
		// click an adjacent tank or radar. Infantry remains immune.
		err = runGameScreen(ctx, 0, gameScreenCommando, noMovie)

	case "mines":
		// World 0 puts a mine layer near enemy tanks. Place Mine blocks enemies from
		// crossing its cell, and Remove Mine retrieves it.
		err = runGameScreen(ctx, 0, gameScreenMines, noMovie)

	case "bulldozer":
		// World 0 parks two bulldozers on fresh rubble piles. Popup Remove clears
		// the pile below each one in one or four turns, depending on its size.
		err = runGameScreen(ctx, 0, gameScreenBulldozer, noMovie)

	case "levelup":
		// World 0 (SNOW_1): a player team with Cost+Hits research preset to level 5
		// (enemy at 0) plus full+damaged tank pairs - verify the build menu shows
		// reduced turns and a HITS rise keeps a full unit full / a damaged one's gap.
		err = runGameScreen(ctx, 0, gameScreenLevelUp, noMovie)

	case "submarine":
		// World 0 (SNOW_1): a friendly AWAC + Corvette over open water beside an enemy
		// submarine - the Corvette's sonar reveals the sub, raising the distinct
		// "Submarine detected!" cue (V_F201), NOT the generic "Enemy detected!".
		err = runGameScreen(ctx, 0, gameScreenSubmarine, noMovie)

	case "navalfire":
		// World 0 (SNOW_1): three player ships vs static enemy Corvettes (AI off). Land between blocks the
		// shot; open water and a friendly ship between do not (klei 23/08/26 naval line of fire).
		err = runGameScreen(ctx, 0, gameScreenNavalFire, noMovie)

	case "parkedbuild":
		// World 0 (SNOW_1): a Constructor and an Engineer parked on the buildings they just finished. Load the
		// Constructor with the Air Transport (its Depot unfolds), force fire the Engineer (its turret is lost).
		err = runGameScreen(ctx, 0, gameScreenParkedBuild, noMovie)

	case "air":
		// World 0 gives the player a bomber and tanks. Land makes the bomber
		// immobile, blocking, and vulnerable. Take Off returns it to the air.
		err = runGameScreen(ctx, 0, gameScreenAir, noMovie)

	case "missile":
		// World 0 (SNOW_1): all missile firing units (land rocket/missile launchers,
		// sea sub/corvette/boat, air bomber) vs static enemies (AI off), fire to
		// watch the rocket/torpedo fly + trail + explode on arrival.
		err = runGameScreen(ctx, 0, gameScreenMissile, noMovie)

	case "spotted":
		// World 0 (SNOW_1): a player scout + a three tank AI hunter squad. Once the squad
		// has spotted the scout, drive it away into the fog and End Turn. The squad
		// advances to where it LAST saw you (the SpottedUnit memory), not your true cell.
		err = runGameScreen(ctx, 0, gameScreenSpotted, noMovie)

	case "aliens":
		// World 0 places neutral alien derelicts beside a player tank. They never
		// move, fire, or take turns, but the tank can attack and salvage them.
		err = runGameScreen(ctx, 0, gameScreenAliens, noMovie)

	case "factoryrun":
		// World 0 shows a powered base with a queued Tank and Scanner. The advanced
		// turn state leaves a produced Tank beside the plant for checking FACBUILD
		// deployment.
		err = runGameScreen(ctx, 0, gameScreenFactoryRun, noMovie)

	default:
		return false, nil
	}

	return true, err
}

// runInGameDialogDebug opens Preferences / Load / Research / Allocate / Transfer over a
// live game frame, via the real in game ops (to inspect the in game dialog).
func runInGameDialogDebug(ctx *menus.Context, cfg *settings.Settings, name string) error {
	var err error

	if wname, e := ctx.Lib.ReadString(menus.WorldTag(cfg.Game.World)); e == nil {
		scr, e2 := buildGameScreen(ctx, strings.TrimSpace(wname))
		if e2 != nil {
			err = e2
		} else {
			scr.DebugSpawn()

			switch name {
			case "loadgame":
				scr.DebugOpenLoad()

			case "research":
				scr.DebugOpenResearch()

			case "alloc":
				scr.DebugOpenAlloc()

			case "upgrade":
				scr.DebugOpenUpgrade()

			case "transfer":
				scr.DebugOpenTransfer()

			case "selfdestruct":
				scr.DebugOpenSelfDestruct()

			case "repairshop":
				scr.DebugOpenRepairShop()

			case "reports":
				scr.DebugOpenReports()

			case "help":
				scr.DebugOpenHelp()

			default:
				scr.DebugOpenPrefs()
			}
		}
	}

	return err
}

// runOptionsDebug opens the Options dialog. It loads FRAMEPIC's palette before centering
// the dialog so its UI colors are correct without a palette swap.
func runOptionsDebug(ctx *menus.Context) error {
	if data, e := ctx.Lib.Read("FRAMEPIC"); e == nil {
		if img, e := sprites.DecodeBig(data); e == nil {
			ctx.Plat.SetPalette(&img.Palette)
		}
	}

	return menus.RunOptions(ctx, gamescreen.MainMap)
}

// runCargoDebug opens the start of game purchase/loadout screen with a sample budget and
// a few distinct units already in the Purchased list (so the list + scroll
// are exercisable).
func runCargoDebug(ctx *menus.Context) error {
	var err error

	lo := &menus.CargoLoadout{Team: 0, StartGold: 250, TeamGold: 250,
		Units: []assets.UnitType{
			assets.Constrct, assets.Engineer, assets.Surveyor, assets.Scout,
			assets.Scanner, assets.Tank, assets.Artillry,
		},
	}

	lo.Cargos = make([]int, len(lo.Units))
	lo.Cargos[0] = 30 // sample cargo so the Cargo worm bar shows not empty

	if err = menus.RunCargo(ctx, lo); err == nil {
		dbg.Printf("cargo loadout: %d units, %d gold left", len(lo.Units), lo.TeamGold)
	}

	return err
}

// runConstructionDebug opens the constructor/engineer Construction Menu (CONBUILD). A sample
// material budget so the Build X1/X2/X4 rates compute realistically.
func runConstructionDebug(ctx *menus.Context, name string) error {
	var err error

	builder := assets.Constrct

	if name == "construct-eng" {
		builder = assets.Engineer
	}

	var choice screendata.BuildSelection
	var ok bool

	if choice, ok, err = menus.RunConstruction(ctx, builder, 0, 60); err == nil {
		dbg.Printf("construction: chose=%v rate=%d path=%v ok=%v", choice.Type, choice.Rate, choice.Path, ok)
	}

	return err
}

// runFactoryBuildDebug opens the factory Production Menu (FACBUILD). A Heavy Vehicle Plant with a
// sample material budget and a one unit starting queue, so the queue edit +
// Build X1/X2/X4 rates compute realistically.
func runFactoryBuildDebug(ctx *menus.Context) error {
	var err error

	var choice screendata.FactorySelection
	var ok bool

	if choice, ok, err = menus.RunFactoryBuild(ctx, assets.LandPlt, 0, 60, []assets.UnitType{assets.Tank}, false); err == nil {
		dbg.Printf("factory build: queue=%v rate=%d repeat=%v ok=%v", choice.Queue, choice.Rate, choice.Repeat, ok)
	}

	return err
}

// runUnitStatusDebug opens the right click Unit Status Screen (STATS) for a sample constructor.
func runUnitStatusDebug(ctx *menus.Context) error {
	u := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	if int(assets.Constrct) < len(ctx.Assets.Attribs) {
		u.Values = game.NewUnitValues(ctx.Assets.Attribs[assets.Constrct])
	}

	u.InitState()

	return menus.RunUnitStatus(ctx, u, "Mk 1 Constructor 1")
}

// spriteViewDebug renders a multi frame unit sprite (resource tag, e.g.
// sprite:BOMBER) cycling through its frames, drawn once per team color so
// the team color remap is visible.
func spriteViewDebug(ctx *menus.Context, cfg *settings.Settings, tag string) error {
	data, err := ctx.Lib.Read(resfileTag(tag))
	if err != nil {
		return err
	}

	m, err := sprites.DecodeMultiSprite(data)
	if err != nil {
		return err
	}

	dbg.Printf("sprite: %s frames=%d f0=%dx%d hot=%d,%d", tag, len(m.Frames), m.Frames[0].W, m.Frames[0].H, m.Frames[0].HotX, m.Frames[0].HotY)

	var pal gfx.Palette
	if d, err := os.ReadFile(filepath.Join(ctx.DataDir, "COLOR.PAL")); err == nil {
		if pf, err := gfx.ParsePalFile(d); err == nil {
			pal = pf.Palette
		}
	}

	ctx.Plat.SetPalette(&pal)

	type teamSwatch struct {
		name  string
		table [256]uint8
	}
	teams := []teamSwatch{
		{"Red", sprites.TeamColorTable(sprites.TeamRed)},
		{"Green", sprites.TeamColorTable(sprites.TeamGreen)},
		{"Blue", sprites.TeamColorTable(sprites.TeamBlue)},
		{"Gray", sprites.TeamColorTable(sprites.TeamGray)},
		{"Derelict", sprites.TeamColorTable(sprites.TeamDerelict)},
	}

	buf := ctx.Sys.Buf(1)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	ctx.Sys.MouseShow()

	frame := 0
	var stamp uint64

	for {
		now := platform.Ticks()
		if now-stamp >= 150 { // advance one frame ~7 fps
			stamp = now
			frame = (frame + 1) % len(m.Frames)
		}

		ctx.Sys.Lock()
		buf.Fill(0)
		for i := range teams {
			cx := buf.W * (2*i + 1) / (2 * len(teams))
			cy := buf.H / 2
			m.Frames[frame].Draw(buf, cx, cy, &teams[i].table)
		}
		ctx.Sys.Unlock()

		// Print acquires the non reentrant system lock, so call it after releasing
		// the lock above. The text then overlays this frame's sprites.
		for i := range teams {
			cx := buf.W * (2*i + 1) / (2 * len(teams))
			ctx.Sys.Print(1, teams[i].name, 110, cx-50, buf.H/2+80, 0xFF)
		}
		ctx.Sys.Print(1, fmt.Sprintf("%s  frame %d/%d", tag, frame, len(m.Frames)), 320, 8, 8, 0xFF)

		switch ctx.Sys.GetInput() {
		case platform.KeyEscape, platform.KeyReturn:
			return nil
		}
	}
}

// resfileTag uppercases and trims a resource tag the way the resource
// library indexes them, so `-emulate sprite:bomber` finds BOMBER.
func resfileTag(tag string) string {
	return resfile.NormTag(strings.ToUpper(strings.TrimSpace(tag)))
}
