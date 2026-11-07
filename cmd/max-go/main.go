// main.go : is the entry point for the open source port of M.A.X.: Mechanized Assault & Exploration.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/suglasp/max-go/internal/bootstrap"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/settings"
)

// DEBUG_GAME enables developer command-line shortcuts, including -emulate.
const DEBUG_GAME = true // Flip to false for a release build

// menuScreensHelp lists the developer screens accepted by -emulate.
const menuScreensHelp = "planetselect, newgame, multiplayer, gameconfig, chooseplayer, clanselect, " +
	"mapview, drawmap, sprite:<tag>, shadow, game, random, random2, build, blewup, colony, popup, bevelart, assets, soldier, apcunit, headrotate, transport, commando, mines, bulldozer, air, missile, navalfire, parkedbuild, reactionfire, spotted, aliens, factoryrun, death, win, lose, ending, " +
	"options, prefs-game, loadgame, research, alloc, upgrade, transfer, selfdestruct, repairshop, reports, help, cargo, construct, construct-eng, factorybuild, scrollbar, listbox, saveload, save, load, credits, intro, splash, " +
	"setup, gamesetup, training, campaign, mission, customscenario, tips, gameconfig-mission, custom, levelup, submarine, status"

type launchFlags struct {
	demo       *bool
	screenshot *string
	movie      *string
	flic       *string
	emulate    *string
	mission    *string
	save       *string
	skipintro  *bool
	nomovie    *bool
	nosplash   *bool
	random     *bool
	mapeditor  *bool
	nosound    *bool
	widescreen *bool
	language   *string
	setAStar   *bool
	difficulty *int
	npc        *int
	custom     *CustomGameFlags
	override   *OverrideFlags
}

// main parses launch flags, loads settings, builds the platform, then starts a debug player, the GNW demo, or
// the main menu.
func main() {
	// parse CLI flags
	flags := defineLaunchFlags(flag.CommandLine)

	flag.Usage = func() {
		printUsage(flag.CommandLine.Output(), filepath.Base(os.Args[0]), flag.CommandLine)
	}

	flag.Parse()

	if err := ValidateCustomFlags(flags.custom); err != nil {
		fatalf("max-go: %v", err)
	}

	if err := ValidateOverrideFlags(flags.override); err != nil {
		fatalf("max-go: %v", err)
	}

	if *flags.difficulty > 5 {
		fatalf("max-go: flag -difficulty must be an integer from 0 to 5 (Clueless..God)")
	}

	noSound = *flags.nosound
	showAStarPath = *flags.setAStar
	difficultyOverride = *flags.difficulty
	npcOverride = *flags.npc
	derelictsLevel = resourceDensity(*flags.custom.Derelicts) // -derelicts for the quick start benches (empty = Rare)

	cfg, baseDir, cfgPath := loadLaunchSettings(flags)

	initDebugLog(cfg, baseDir)
	defer dbg.Close()

	// A crash with the console hidden leaves its trace in max-go-crash.log.
	openCrashLog(baseDir)
	defer closeCrashLog()

	dbg.Printf("base dir: %s", baseDir)

	p := newPlatform(&cfg, baseDir, cfgPath, *flags.screenshot)

	if runMediaPlayers(p, cfg, flags) {
		return
	}

	// A missing MAX.RES starts the first run bootstrap for the default data directory.
	// Explicit paths show an error instead of searching elsewhere.
	// The -demo bench keeps the plain fallback below.
	if !*flags.demo {
		dataDir, dirErr := cfg.ResolveDataDir()
		if dirErr != nil {
			fatalf("max-go: %v", dirErr)
		}
		if !bootstrap.HasMaxRes(dataDir) {
			if err := runDataBootstrap(p, &cfg, cfgPath, flags); err != nil {
				fatalf("max-go: %v", err)
			}
			return
		}
	}

	// The GNW demo needs the game fonts.
	// Use the plain frame hook when the data directory has no FONT*.FON files.
	var fontSet *fonts.Set
	if dir, err := cfg.ResolveDataDir(); err == nil {
		if fs, err := fonts.LoadSet(dir); err == nil {
			fontSet = fs
		} else {
			dbg.Printf("fonts unavailable: %v", err)
		}
	}

	if err := runApp(p, &cfg, cfgPath, fontSet, flags); err != nil {
		fatalf("max-go: %v", err)
	}
}

// loadLaunchSettings resolves where the game was started from, loads the settings file
// (creating it when missing) and folds the launch flags that override a setting for this
// run into the loaded copy. It returns the settings, the base directory and the settings path.
func loadLaunchSettings(flags launchFlags) (settings.Settings, string, string) {
	baseDir, err := settings.BaseDir()
	if err != nil {
		fatalf("max-go: %v", err)
	}

	// max-go.json preferred, a legacy settings.json still honored.
	cfgPath := settings.ResolvePath(baseDir)

	// -reset restores the shipped settings before loading them. It leaves saves
	// and all other files untouched.
	if boolValue(flags.override.Reset) {
		if err := settings.Default().Save(cfgPath); err != nil {
			fatalf("max-go: reset settings: %v", err)
		}
		log.Printf("max-go: %s reset to defaults", filepath.Base(cfgPath))
	}

	cfg, err := settings.LoadOrCreate(cfgPath)
	if err != nil {
		fatalf("max-go: %v", err)
	}

	if *flags.language != "" {
		cfg.Language = *flags.language
	}

	// Apply explicit Custom Game flags before setup or quick start reads the settings.
	// Default flag values do not replace saved resource levels.
	// C++: command line overrides of MAX.INI.
	customProvided := map[string]bool{}
	flag.CommandLine.Visit(func(f *flag.Flag) {
		customProvided[f.Name] = true
	})
	applyCustomGameFlags(&cfg.Game, flags.custom, customProvided)

	// Quick-start benches use explicitly supplied Custom Game flags. Read the flag values rather than cfg.Game
	// so saved setup values cannot leak into a bench.
	if customProvided["raw"] || customProvided["fuel"] || customProvided["gold"] {
		benchResourceLevels = &[3]int{
			resourceDensity(stringValue(flags.custom.Raw)),
			resourceDensity(stringValue(flags.custom.Fuel)),
			resourceDensity(stringValue(flags.custom.Gold)),
		}
	}
	benchStartGold = startCreditValue(stringValue(flags.custom.StartCredit)) // "" parses to -1 = unset
	benchVictoryProvided = customProvided["score"] || customProvided["duration"]

	// -difficulty overrides the computer opponent tier for this run.
	// Copy it to settings.Game.Opponent so the setup screen shows the chosen tier too.
	if difficultyOverride >= 0 {
		cfg.Game.Opponent = difficultyOverride
	}

	// -widescreen changes this run's game view without persisting video.widescreen.
	if *flags.widescreen {
		overrideForRun(&cfg, func(s *settings.Settings) *settings.WidescreenMode { return &s.Video.Widescreen }, settings.WidescreenOn)
	}

	// Apply settings override flags for this run without writing them back to the settings file.
	applyLaunchOverrides(&cfg, flags.override)

	return cfg, baseDir, cfgPath
}

// initDebugLog opens the configured debug console and log file. The caller closes it when the run ends.
func initDebugLog(cfg settings.Settings, baseDir string) {
	logPath := ""
	if cfg.Debug.LogFile {
		logPath = filepath.Join(baseDir, "max-go.log")
	}

	if err := dbg.Init(cfg.Debug.Console, logPath, cfg.Debug.LogBufferLines); err != nil {
		log.Printf("max-go: debug log unavailable: %v", err)
	}
}

// newPlatform opens the game window with the configured video settings and persists F10 brightness changes.
func newPlatform(cfg *settings.Settings, baseDir, cfgPath, screenshotPath string) *platform.Platform {
	p := platform.New(platform.Config{
		Title:          "M.A.X. - Golang edition",
		WindowScale:    cfg.Video.WindowScale,
		Fullscreen:     cfg.Video.Fullscreen,
		Brightness:     cfg.Video.Brightness,
		Widescreen:     widescreenMode(cfg.Video.Widescreen),
		BaseDir:        baseDir,
		ScreenshotPath: screenshotPath,
	})

	// Reload before saving so only brightness reaches disk; launch overrides in cfg remain temporary.
	p.OnBrightnessChange = func(mode int) {
		cfg.Video.Brightness = mode
		saved, err := settings.Load(cfgPath)
		if err != nil {
			saved = *cfg
		}
		saved.Video.Brightness = mode
		if err := saved.Save(cfgPath); err != nil {
			dbg.Printf("settings save failed: %v", err)
		}
	}

	return p
}

// runMediaPlayers handles -mve and -flic, which play one file and quit. It reports whether either player ran.
func runMediaPlayers(p *platform.Platform, cfg settings.Settings, flags launchFlags) bool {
	if *flags.movie != "" {
		if err := runMovieDebug(p, cfg, *flags.movie); err != nil {
			fatalf("max-go: %v", err)
		}
		return true
	}

	if *flags.flic != "" {
		if err := runFlicDebug(p, cfg, *flags.flic); err != nil {
			fatalf("max-go: %v", err)
		}
		return true
	}

	return false
}

// runApp starts the GNW demo, main menu, or plain missing-data window. It returns when the platform loop ends.
func runApp(p *platform.Platform, cfg *settings.Settings, cfgPath string, fontSet *fonts.Set, flags launchFlags) error {
	var err error

	if *flags.demo {
		setupDemo(p, *cfg)
	} else {
		loadGamePalette(p, *cfg)
	}

	// game state
	switch {
	case *flags.demo && fontSet != nil:
		sys := gnw.New(p.Screen(), fontSet)
		err = sys.Run(p, func() error { return demoLogic(p, sys, *cfg) })

	case !*flags.demo && fontSet != nil:
		err = runMainMenu(p, cfg, cfgPath, fontSet, menuJumpName(flags), *flags.skipintro, *flags.nomovie, *flags.nosplash)

	default:
		// Keep a plain window open so the log can report the missing data and its expected location.
		log.Printf("max-go: game data not found, set data_dir in max-go.json (or MAX_DATA_DIR) to your M.A.X. installation folder")
		p.SetFrameHook(func(p *platform.Platform) error {
			showConsoleOnce(*cfg)
			if *flags.demo {
				cycleDemoPalette(p)
			}
			for {
				switch key := p.GetInput(); key {
				case -1:
					return nil
				case platform.KeyEscape:
					return ebiten.Termination
				default:
					dbg.Printf("input: %d", key)
				}
			}
		})
		err = p.Run()
	}

	return err
}

// menuJumpName maps quick-start flags to the name runMainMenu accepts. Bootstrap reuses it so first-run
// launches keep their target.
func menuJumpName(flags launchFlags) string {
	name := *flags.emulate
	if *flags.random && name == "" {
		name = "random" // -random / -rng jumps straight into a quick start game
	}
	if *flags.mapeditor && name == "" {
		name = "mapeditor" // -mapeditor opens the ME7 editor shell (undocumented while work in progress)
	}
	// -mission / -save jump straight into a specific file, bypassing the menus
	// (the path/slot rides in the jump name and is resolved once the data dir is known).
	if *flags.mission != "" {
		name = "loadmission:" + *flags.mission
	} else if *flags.save != "" {
		name = "loadsave:" + *flags.save
	}
	return name
}

// widescreenMode maps the video.widescreen setting onto the platform enum.
func widescreenMode(v settings.WidescreenMode) platform.WidescreenMode {
	switch v {
	case settings.WidescreenOn:
		return platform.WidescreenOn
	case settings.WidescreenOff:
		return platform.WidescreenOff
	default:
		return platform.WidescreenAuto
	}
}

// defineLaunchFlags registers the command line options and returns their handles.
// Custom Game flags register in every build; DEBUG_GAME builds additionally register -emulate.
func defineLaunchFlags(fs *flag.FlagSet) launchFlags {
	// Always available flags (Release or Debug mode)
	lf := launchFlags{
		demo:       fs.Bool("demo", false, "show a test pattern with palette cycling"),
		screenshot: fs.String("screenshot", "", "save the first frame to this PNG file and quit"),
		movie:      fs.String("mve", "", "debug: play an Interplay MVE movie from the data dir or an explicit path"),
		flic:       fs.String("flic", "", "debug: play a FLI/FLC animation from the data dir or an explicit path"),
		skipintro:  fs.Bool("skipintro", false, "skip the boot intro movie and splash logos"),
		nomovie:    fs.Bool("nomovie", false, "skip the intro movie but still show the splash logos"),
		nosplash:   fs.Bool("nosplash", false, "skip the splash logos (e.g. with -random for a fast start)"),
		random:     fs.Bool("random", false, "quick-start a 2-team game on a random map (alias -rng)"),
		mapeditor:  fs.Bool("mapeditor", false, "open the map / scenario editor (milestone ME7, work in progress)"),
		nosound:    fs.Bool("nosound", false, "disable audio initialization"),
		widescreen: fs.Bool("widescreen", false, "widen the live game view to 16:9 (854x480) for this run; menus stay 4:3 (overrides max-go.json)"),
		language:   fs.String("language", "", "override the UI language for this run; default en-us/max-go.json; drop lang-<code>.ini next to the data dir or binary to add one"),
		setAStar:   fs.Bool("set-a-star", false, "debug: draw the unit's full A* path as white cells (off = move without showing the path)"),
		difficulty: fs.Int("difficulty", -1, "AI difficulty for this run: 0..5 (Clueless..God); overrides the saved setting. Default -1 = Average for -random/-random2, settings value for a menu game"),
		npc:        fs.Int("npc", 1, "number of computer opponents, 1..3 (custom game, -random and -random2 alike; the setup screen has no opponent count panel)"),
		mission:    fs.String("mission", "", "load a mission file directly (.TRA/.SCE/.CAM/.MPS), bypassing the menus; value is a path or a base name in the data dir"),
		save:       fs.String("save", "", "load a saved game directly by slot number (the N in the saves folder), bypassing the menus"),
	}

	lf.custom = registerCustomFlags(fs)
	lf.override = registerOverrideFlags(fs)

	// -emulate is a developer shortcut: registered (and documented) only in a DEBUG_GAME build.
	// In a release build it is not a known flag, so the launch is like the vanilla M.A.X.
	// emulate stays a valid (empty) pointer either way.
	if DEBUG_GAME {
		lf.emulate = fs.String("emulate", "", "debug: jump directly to (emulate) one screen ("+menuScreensHelp+")")
	} else {
		lf.emulate = new(string)
	}

	fs.BoolVar(lf.random, "rng", false, "alias of -random")

	return lf
}

// printUsage writes command-line help.
func printUsage(w io.Writer, exe string, fs *flag.FlagSet) {
	fmt.Fprintf(w, `Usage:
  %[1]s
  %[1]s -screenshot <file>
  %[1]s -mve <file> [-screenshot <file>]
  %[1]s -flic <file> [-screenshot <file>]
  %[1]s -mission <file> [-screenshot <file>]  // load a .TRA/.SCE/.CAM/.MPS mission directly
  %[1]s -save <slot> [-screenshot <file>]     // load a saved game slot directly
  %[1]s -random -npc 3 -difficulty 3 -derelicts common  // random game, 3 AI opponents, average difficulty, common derelicts
`, exe)
	// The -mapeditor usage entry stays commented out until the ME7 editor ships.
	// The flag itself already works:
	//   %[1]s -mapeditor                          // open the map / scenario editor
	if DEBUG_GAME {
		fmt.Fprintf(w, `  %[1]s -emulate <screen> [-screenshot <file>]
  %[1]s -emulate sprite:<tag> [-screenshot <file>]

Menu screens:
  %[2]s
`, exe, menuScreensHelp)
	}
	fmt.Fprint(w, "\nOptions:\n")
	printFlagDefaults(w, fs)
	fmt.Fprintln(w)
	fmt.Fprint(w, `Parameter details: 	  
  -screenshot <file>
      Saves the first rendered frame to the named PNG file and exits.
      The path is used exactly as supplied. For interactive screenshots,
      press F12; those PNGs go into the screenshots folder next to the
      executable.

  -mve <file>
      Play movie Interplay MVE file. The value may be an
      absolute path, a path relative to the current directory, or a base
      name in the configured game data directory.
	  If no extension is supplied, .MVE is appended.

  -flic <file>
      Debug animation player for FLI/FLC files. The value may be an
      absolute path, a path relative to the current directory, or a base
      name in the configured game data directory. If no extension is
      supplied, .FLC is appended.

  -mission <file>
      Loads a mission file directly, skipping the menus: a training
      .TRA, scenario .SCE, campaign .CAM, or custom-scenario .MPS. The
      value may be an absolute path, a path relative to the current
      directory, or a base name (e.g. SAVE1.TRA) in the game data
      directory. The paired DESCR briefing lights the GOAL button.

  -save <slot>
      Loads a saved game directly by slot number (the N of the save in
      the saves folder next to the executable), skipping the Load Game
      picker.

  -language <code>
      Overrides the UI string language for this run. The bundled/default UI
      language is en-us (LANG_EN in LANG.RES); a user-added
      lang-<code>.ini in the configured data directory or next to
      the executable overrides LANG.RES.

  -nomovie / -nosplash
      Boot-sequence controls. -nomovie skips the intro movie but still
      shows the splash logo; -nosplash skips the splash
      logo. -nosplash pairs well with -random for a fast quick-start.
      -skipintro is a combo of -nomovie and -nosplash.

  -widescreen
      Widens the live game view to 16:9 (logical 854x480) for this
      run; menus and full-screen screens stay at the classic 4:3
      (640x480, pillarboxed), while in-game dialogs open over the
      wide view. Overrides video.widescreen in max-go.json without
      saving it. The setting takes true, false, or "auto" (default):
      auto picks the aspect closest to the monitor at startup.

  -datadir <path>
      Points the game data lookup at <path> for this run (alias -data).
      Wins over the MAX_DATA_DIR environment variable and the data_dir
      setting. The directory must hold the original game data (MAX.RES,
      PATCHES.RES, ...).

  -playername / -audio / -scale / -windowed / -fullscreen / -debug
      Settings overrides for this run: they replace player.name,
      audio.backend, video.window_scale, video.fullscreen and
      debug.console + debug.log_file from the settings file without
      ever writing it. The settings file is max-go.json next to the
      binary; a settings.json from older versions is still honored
      (max-go.json wins when both exist). For -audio, native is the
      built in Ebitengine backend (alias ebiten).

  -reset
      Rewrites the settings file to its defaults and boots on them.
      The saves folder and every other file stay untouched.

  -random - Custom game flags
      -turnlimit/-turns, -endturn, -score, -duration, -turnbased,
      -simultane, -raw, -fuel, -gold, -derelicts,
	  and -startcredit/-credit mirror the Custom Game setup screen.

`)
	if DEBUG_GAME {
		fmt.Fprint(w, `  -emulate <screen>
      Debug launcher for one menu/game-screen entry after normal asset
      startup. Use sprite:<tag> to inspect a unit sprite resource, for
      example sprite:APC1. Use -emulate splash to preview just the splash
      palette fade-in.

`)
	}
	if DEBUG_GAME {
		fmt.Fprint(w, `Precedence:
  -mve exits after movie playback and wins over -flic and -emulate.
  -flic exits after animation playback and wins over -emulate.
`)
	} else {
		fmt.Fprint(w, `Precedence:
  -mve exits after movie playback and wins over -flic.
  -flic exits after animation playback.
`)
	}
}

func printFlagDefaults(w io.Writer, fs *flag.FlagSet) {
	fs.VisitAll(func(f *flag.Flag) {
		// -demo is an undocumented dev flag: keep it working but out of -help.
		// -mapeditor stays hidden too until the ME7 editor ships.
		if f.Name == "demo" || f.Name == "mapeditor" {
			return
		}

		fmt.Fprintf(w, "  -%s%s\n", f.Name, flagValueHint(f.Name))
		fmt.Fprintf(w, "      %s\n", f.Usage)
	})
}

func flagValueHint(name string) string {
	switch name {
	case "screenshot":
		return " <file>"

	case "mve":
		return " <file>"

	case "flic":
		return " <file>"

	case "emulate":
		return " <screen>"

	case "language":
		return " <code>"

	case "difficulty":
		return " <0..5>"

	case "npc":
		return " <1..3>"

	case "turnlimit", "turns":
		return " <60..250>"

	case "endturn":
		return " <15..90>"

	case "score":
		return " <Low|Medium|High>"

	case "duration":
		return " <1..9999>"

	case "raw", "fuel", "gold":
		return " <Poor|Medium|Rich>"

	case "derelicts":
		return " <None|Rare|Common>"

	case "startcredit", "credit":
		return " <Low|Limited|Normal|High|TooMuch>"

	case "datadir", "data":
		return " <path>"

	case "playername":
		return " <name>"

	case "audio":
		return " <native|ebiten|soloud|miniaudio>"

	case "scale":
		return " <1..8>"

	default:
		return ""
	}
}
