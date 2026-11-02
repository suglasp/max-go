# max-go

Main game executable and debug launcher.

## Usage

```text
max-go
max-go -demo
max-go -screenshot <file>
max-go -demo [-screenshot <file>]
max-go -mve <file>
max-go -mve <file> [-screenshot <file>]
max-go -flic <file>
max-go -flic <file> [-screenshot <file>]
max-go -emulate <screen>
max-go -emulate sprite:<tag>
max-go -emulate <screen> [-screenshot <file>]
max-go -emulate sprite:<tag> [-screenshot <file>]
max-go -mission <file> [-screenshot <file>]
max-go -save <slot> [-screenshot <file>]
max-go -random [-npc <1-3>] [-difficulty <0-5>]
max-go -emulate random2 [-npc <1-3>] [-difficulty <0-5>]
max-go [-skipintro] [-nomovie] [-nosplash] [-nosound] [-widescreen] [-language <lang>] [-set-a-star]
max-go [-datadir <path>] [-playername <name>] [-audio <backend>] [-scale <1-8>] [-windowed | -fullscreen] [-debug] [-reset]
```

`-nosound`,
`-skipintro`,
`-nomovie`,
`-nosplash`,
`-widescreen`,
`-language`,
and `-set-a-star` are modifiers that combine with the launch modes above. `-mission` and `-save` are direct launch modes that skip the menus. `-npc` and `-difficulty` apply to the `-random` / `-random2` quick starts.
The settings override flags (`-datadir`,
`-playername`,
`-audio`,
`-scale`,
`-windowed`,
`-fullscreen`,
`-debug`) replace their `max-go.json` values for that run only and are never written back;
`-reset` rewrites `max-go.json` to its defaults.
The settings file is `max-go.json` next to the binary;
a `settings.json` from older max-go versions is still honored,
and `max-go.json` wins when both exist.

## Parameters

```text
-demo
```

Starts the GNW/test pattern demo instead of the normal menu flow.
It still uses the configured palette and supports `-screenshot`.

```text
-screenshot <file>
```

Saves the first rendered frame to the named PNG file and exits.
The path is used exactly as supplied.
This can be combined with normal launch,
`-demo`,
`-mve`,
`-flic`,
or `-emulate`.

For interactive screenshots,
press F12 while the game is running.
Those PNG files go into the `screenshots` folder next to the executable.

```text
-mve <file>
```

Debug movie player for Interplay MVE files.
The value may be an absolute path,
a path relative to the current directory,
or a base name in the configured game data directory.
If no extension is supplied,
`.MVE` is appended.

```text
-flic <file>
```

Debug animation player for FLI/FLC files.
The value may be an absolute path,
a path relative to the current directory,
or a base name in the configured game data directory.
If no extension is supplied,
`.FLC` is appended.

```text
-emulate <screen>
```

Debug launcher for one menu or game screen entry after normal asset startup.
Use `sprite:<tag>` to inspect a unit sprite resource,
for example `sprite:APC1`.

`-emulate` is gated by the `DEBUG_GAME` build constant in `cmd/max-go/main.go`: in a `DEBUG_GAME` build it is registered and listed in `-help`;
with `DEBUG_GAME = false` `-emulate` is not a known flag.
The custom game launch flags (`-turnlimit`/`-turns`,
`-endturn`,
`-score`,
`-duration`,
`-turnbased`/`-simultane`,
`-raw`/`-fuel`/`-gold`,
`-derelicts`,
`-startcredit`/`-credit`) are NOT gated - `registerCustomFlags` runs unconditionally and their `-help` block prints in every build,
`DEBUG_GAME` or not.

The in game developer UI includes the `F2` debug flag menu with unit/building placement. The old `U` spawn and `B`
build menu keys no longer apply. The developer keys `A` access overlay and `V` fog, plus the on-screen key hint,
are active in a `DEBUG_GAME` build. Resource reveal uses `Ctrl`+`M` and is always active; see `docs/controls.md`.
and otherwise opt in at runtime via the `debug.menu` flag in `max-go.json` (edit it with `max-settings`,
key `debug.menu`;
default off).
So a release build (`DEBUG_GAME = false`) ships as a plain vanilla launch,
but a player can still open the `F2` menu by setting `debug.menu` to `true`. `debug.menu` is independent of the `debug.console` / `debug.log_file` logging switches.

`F2` opens the **debug flag menu**: a developer overlay for poking the live game that pauses the turn clock while open (like the Files / Preferences screens).
Arrow keys navigate (`Up/Down`) and adjust (`Left/Right` or `Enter`);
`F2`/`Esc` close.
It toggles the AI on/off,
fog of war,
an "endless mode" (suppresses the win/loss check),
and **Show AI paths** (off by default - vanilla never shows enemy paths;
on,
the computer teams' planned routes draw in gray/red vs the player's green/blue),
rolls the per turn timer and turn cap,
and has **Add unit** / **Add building** rows - selecting one arms a click to place drop (the next left click on the map drops that unit/building for the active team;
`Esc` cancels).
Rows whose backing system does not exist yet (simultaneous mode,
the AI combat/reasoning split) are shown `(n/a)`.

```text
-mission <file>
```

Loads a mission file directly,
skipping the menus: a training `.TRA`,
scenario `.SCE`,
campaign `.CAM`,
or custom scenario `.MPS`.
The value may be an absolute path,
a path relative to the current directory,
or a base name (for example `SAVE1.TRA`) in the game data directory.
The paired `DESCR` briefing file lights the GOAL button.
The splash logo still shows unless `-skipintro` / `-nosplash` suppress it.

```text
-save <slot>
```

Loads a saved game directly by slot number (the N of the save in the `saves` folder next to the executable),
skipping the Load Game picker.

```text
-skipintro
```

Skips the boot intro movie and splash logos,
going straight to the main menu.

```text
-nomovie
```

Skips the intro movie but still shows (and fades in) the splash logos.

```text
-nosplash
```

Skips the splash logos.
Pairs well with `-random` for a fast quick start.
During the movie,
a key or left click skips ahead to the splash (it does not also skip the splash);
during a splash,
a key/left click (or the hold timer) continues to the main menu.
The splash dissolves in via a black -> image palette fade (~1.5s).

```text
-random   (alias: -rng)
```

Quick starts a game on a random map instead of going through the menus: the human plus `-npc` computer opponents (default one),
each landed on its own landmass with a base that adapts to its terrain (a coastal base fields a wired shipyard + boats;
a landlocked base skips the dock and leans on its Eco Sphere + habitat).

```text
-npc <1-3>
```

Number of computer opponents (1 to 3,
default 1).
With four team slots,
that is one human plus up to three NPCs,
each on its own landmass.
It applies to a menu **custom game** as well as to `-random` / `-random2`: the setup screen has no opponent count panel (its "Computer Opponent" panel is the difficulty tier,
and the original picks team types on a separate player setup screen that is not ported yet),
so `-npc` is the only way to face more than one computer team.

```text
-difficulty <0-5>
```

AI difficulty tier for this run,
overriding the saved setting: `0` Clueless,
`1` Apprentice,
`2` Average,
`3` Expert,
`4` Master,
`5` God.
A single global tier drives every computer team (as in vanilla M.A.X.).
Default (flag omitted) is Average for `-random` / `-random2` and the `max-go.json` value for a menu game. `-random` / `-random2` show an "AI Difficulty : N [name]" banner on entry;
the F2 debug menu also cycles it live.

```text
-nosound
```

Disables audio initialization (skips the audio backend entirely).
Useful for quick visual testing without the device init delay/pop.

```text
-widescreen
```

Widens the live game view to 16:9 (logical 854x480) for this run;
all menus stay at the classic 4:3 (640x480,
pillarboxed),
and in game dialogs open over the wide view.
Overrides `video.widescreen` in `max-go.json` without saving it.
The setting itself takes `true`,
`false`,
or `"auto"` (the default),
where `auto` picks the aspect closest to the monitor at startup.

```text
-language <lang>
```

Overrides the UI string file for this run.
The bundled default language is `en-us` (entry `LANG_EN` in `LANG.RES`, see `docs/formats/lang-res.md`);
an added `lang-<code>.ini` in the configured data directory or next to the executable overrides `LANG.RES`,
and missing keys fall back to English.
Without the flag the `language` value in `max-go.json` is used.
Note the FON fonts are byte based,
so a translation is limited to the ASCII/8-bit glyph set (Cyrillic needs a matching FON codepage and is not supported).

```text
-set-a-star
```

Debug overlay: draws the selected unit's full A\* path as white cell markers while it moves.
Off by default: without the flag a unit simply moves and never reveals the path it takes,
in any game mode.
This is independent of the queued route arrows: a move that runs out of movement points this turn always shows its unreached remainder as green markers (with a blue marker at each turn boundary),
and the unit continues on its own along it on its next turn.

```text
-datadir <path>   (alias: -data)
```

Points the game data lookup at `<path>` for this run.
Wins over the `MAX_DATA_DIR` environment variable and the `data_dir` setting.
The directory must hold the original game data (MAX.RES,
PATCHES.RES,
the `*.WRL` worlds,
...).

First run note: when no data path is set at all (no flag,
no `MAX_DATA_DIR`,
no `data_dir` setting) and `MAX.RES` is missing from the folder next to the executable,
the game runs its data bootstrap - it looks for an installed GOG or Steam copy of M.A.X. (falling back to a typed path prompt) and copies the game data next to the executable.
An explicitly set path never triggers the bootstrap: missing `MAX.RES` there shows an error screen instead.

```text
-playername <name>
```

Player name for this run,
overriding `player.name` in `max-go.json`.

```text
-audio <native|ebiten|soloud|miniaudio>
```

Audio backend for this run,
overriding `audio.backend`.
ative` is the built in Ebitengine backend (alias `ebiten`);
`soloud` and `miniaudio` select the external mixer backends.

```text
-scale <1-8>
```

Desktop window scale for this run,
overriding `video.window_scale`: the window is the logical resolution multiplied by this factor.

```text
-windowed / -fullscreen
```

Start in a desktop window or fullscreen for this run,
overriding `video.fullscreen`.
The two are mutually exclusive;
F11 still toggles at runtime.

```text
-debug
```

Enables the debug console and `max-go.log` for this run,
overriding `debug.console` and `debug.log_file`.

```text
-reset
```

Rewrites `max-go.json` to its defaults before the game boots,
then boots on them.
The `saves` folder and every other file stay untouched.
Other flags on the same command line still apply on top of the fresh defaults.

Known screen values:

```text
Pre game menus:   planetselect, newgame, multiplayer, gameconfig, chooseplayer, clanselect
Render/inspect:   mapview, drawmap, sprite:<tag>, shadow, assets, soldier, apcunit, transport, commando, mines
In game screens:  game, random, random2, custom, build, popup, death, win, lose, blewup, colony
Main menu screens: setup
In game dialogs:  options, prefs-game, loadgame, research, alloc, upgrade, transfer, selfdestruct, repairshop, reports, help
Loadout:          cargo, construct, construct-eng
Unit info:        status
Save/load:        saveload, save, load
Movies/screens:   credits, intro, splash
```

- `popup` opens a unit's order popup menu over the demo game for inspection.
- `random` starts the `-random` quick start (the human plus `-npc` opponents on separate landmasses; each base adapts to coastal vs landlocked terrain).
- `random2` is a developer test bed: the human plus `-npc` fully developed bases (default one opponent) on turn 100 - multiple mining stations, heavy power stations, the light/heavy/air plants, launchers, connectors, a T shaped road, and (for a coastal base) a shipyard wired into the complex with connectors and a span of bridge pieces at the shore, plus air and naval units - so lots of unit/building types can be exercised at once. A landlocked base skips the shipyard/boats and leans on its Eco Sphere + habitat. You drop straight into the running game (no landing).
 - `custom` lands a sample custom game: the human team deploys with a fixed purchased loadout (the first slice of the landing phase) and an opponent team with the quick start roster, each on its own landmass. The full flow (Main Menu -> New Game -> Custom) runs the Cargo purchase screen first, then lands with the selected loadout.
- `setup` opens the standalone main menu Setup screen (SETUPPIC): the setup section only (audio volumes/disables, auto save, player name), distinct from the in game Preferences (`options`, PREFSPIC) which shows the full preferences/options list.
- `prefs-game` / `loadgame` open the in game Preferences / Load dialogs over a live game frame; `research` / `alloc` / `upgrade` open those in game menus.
- `transfer` opens the cargo exchange dialog between two adjacent supply trucks (the X-Fer popup order).
- `selfdestruct` opens the SELFDSTR arm/detonate dialog (the building popup's Remove action).
- `repairshop` opens the Repair Shop / Depot (DEPOTFRM): a depot with a damaged tank and scout (plus a raw dump) so reload/repair/upgrade are exercisable.
- `reports` opens the Reports screen (REP_FRME): the Units / Casualties / Score / Messages tabs; the Messages tab is seeded with 50 entries so the up/down scroll and the icon/coords/text columns can be tested.
- `help` shows the game screen help blurb (the HELPFRAM modal the in game `?` button opens).
- `cargo` opens the start of game purchase/loadout screen (CARGOPIC) with a sample gold budget; buy/delete units against the roster.
- `construct` / `construct-eng` open the in game Construction Menu (CONBUILD) for a constructor / engineer: the buildable list (large vs small structures), portrait + description toggle, icon gauge stats, and the Build X1/X2/X4 rate boxes (the engineer adds the Path button). In game this is reached by selecting a constructor/engineer and pressing B (or the popup Build order).
- `status` opens the Unit Status Screen (STATS): a unit's portrait, name + description, and attribute gauges (cargo / armor / hits / scan / speed / cost, plus the combat rows for armed units). In game this is reached by right clicking a unit (an attackable enemy is fired on instead).
- `death` jumps to the end of game (victory/defeat) screen.
- `blewup` places a 2x2 building then destroys it, to inspect the explosion + rubble (the foundation slab is cleared, only rubble remains).
- `colony` spawns a powered Eco Sphere (Greenhouse) for the player and one enemy team and fires the colony started announce: the "&lt;colour&gt; team colony started!" voice (red/green/blue/gray = F098/F103/F108/F113) plus the banner + message log entry.
- `assets` lays out every building and unit on a snow/water world, each animating and labelled with its name (a sprite gallery).
- `soldier` lays out the two marching soldier types (infantry, commando) in all eight facings on snow, cycling their walk animation in place (inspect the soldier walk cycle).
- `apcunit` is a unit sound test bench: two APCs, a tank and a bulldozer (player) plus one enemy tank on snow. Select a unit to hear its looped idle hum, click to move it (drive sound, then the stop sound on arrival), and right click the enemy tank to fire (fire/hit sounds).
- `transport` is the transport unload bench: a land APC and an air transport (player), each starting loaded with two land units, on snow. Select a transport, long press for its popup, and pick **Activate** to open the Repair Shop screen listing its carried units; pick one to unload it with the Depot style build ready exit (it parks build ready on the transport with BLDMRK exit markers - click a marker to drive it out). Two free land units (Free Tank, Free Scout) sit nearby to test the Enter/Load boarding flow first (move one adjacent to the APC, Enter, click the APC), then Activate to unload it again.
- `commando` is the disable/steal infiltration bench: a player commando in open ground with three enemy units a few cells away - an electronic tank (north), an organic infantry (west) and a 2x2 powered radar (south east). Move the commando next to a target, long press for its popup, pick Disable or Steal, then click the enemy: the tank can be disabled or stolen, the radar disabled only (a building cannot be stolen), and the infantry is immune (organic). End the turn to watch a disabled unit count down and recover.
- `mines` is the mine laying bench: a player mine layer (loaded with its full mine store) between two enemy tanks. Select the layer, long press for its popup, and pick Place Mine to drop a LandMine on its own cell (no target pick - it mines where it stands); move it a cell and lay again to build a minefield. An enemy unit cannot enter a mined cell; Remove Mine picks the layer's own mine back up (store refills, capped at capacity).
- `spotted` is the AI last seen memory bench: a player Scout and a three tank **computer** hunter squad on open snow, with the scout starting inside the squad's scan (already spotted). Drive the scout far away into the open so it drops into the fog, then End Turn - the squad heads toward the cell where it LAST saw you (the SpottedUnit memory), not your scout's true position, and it does not instantly forget you. When it reaches that spot and you are gone, it SEARCHES the area: each lost contact randomly picks either a focused ZoneWalker sweep of the disc around the last sighting (the original's behaviour) or the wider fog explore wander (ours). The debug log prints `ai: ... pursues last seen ...` while it chases and `ai: ... searching (zone|explore) ...` when it starts hunting.
- `splash` previews just the MLOGO splash palette fade in (no movie).
- `intro` plays the full debug sequence including the Interplay logo (ILOGO, from PATCHES.RES). The normal boot flow does not show ILOGO: it is movie -> MLOGO splash -> menu, matching the retail game.

## Notes

Mode precedence follows the launcher code:

- `-mve` exits after movie playback and wins over `-flic`, `-demo`, and `-emulate`.
- `-flic` exits after animation playback and wins over `-demo` and `-emulate`.
- `-demo` starts the demo and ignores `-emulate`.
