# max-go changes

Check the version file next to the game executables.


## 1.0.1.2 (in development)

### Fixes
- On Windows, fixed race condition that could crash the game at startup when the debug console is switched on.
- A path build stops before a cell the Engineer cannot afford: it waits on the last finished building with its exit
  markers (or stays on the last road) and the insufficient material warning shows, instead of starting the cell and
  taking it down again.


## 1.0.1.1 (2026-11-03)

### Changes
- Built on Ebitengine 2.10.5 and Go 1.26.7.
- Letter keys run the popup buttons of the selected unit (B = Build, S = Sentry, ...), as in the original game.
  The port's own keys moved to Ctrl: Ctrl+S status, Ctrl+R survey, Ctrl+O options, Ctrl+M resources.
- F1 jumps to the last event, whatever its kind.
- A unit that stopped for an enemy drives on along its planned route at the next turn start.
- Unit sounds pan and fade by their distance from the view centre and by the zoom; units off screen sound quieter.
- The build, cargo, upgrade and Unit Status screens show the real unit descriptions, with one Description toggle shared by all of them.
- The texts ship in the new `LANG.RES` archive; a loose `lang-<code>.ini` next to the game still overrides it.
- 47 English texts in `LANG.RES` have new wording: unit descriptions, the "is ready, click a marked square" exit
  hints, the radar contact lines, the clan hint, the end of game notice and the farewell line.
- The build exit markers blink at the original pace.
- A Corvette fires on land units standing on the shore. Corvettes and Submarines fire only along open water, never across land.
- Every unit shows as a 2x2 blip on the minimap, as in the original game.
- The "Game will end in 10 turns" notice comes 10 turns before the end (it came after 11).
- Power Off is offered on generators and the Gold Refinery whenever they are not already off.
- The reaction fire voice plays only when the shooting unit is not selected and not on screen.
- A builder at work has no Sentry button, and a transport cannot pick it up.
- Expert and harder AI players sometimes pick the Espionage strategy, and the AI strategy weights on crowded maps follow the original game more closely.
- Missiles and rockets show the in between headings on shallow shots.
- The misspelled `-derilicts` flag is gone; use `-derelicts`.

### QOL
- Tested rendering and zoom quality on 2K and 4K screen resolutions.
- Surveyors on Auto Survey (and the AI's Surveyors) map the ground around the base first, then the rest of the map.
- `-random` never lands a team on an islet too small to build a base on.
- The "storage is full" voices wait until after the first End Turn.
- Small trackpad scroll steps add up into whole zoom steps.
- New `audio.channels_reversed` setting in `max-go.json` swaps the left and right speakers.
- The key list under the map shows the current keys.
- `build-windows.cmd` builds the Linux game as well.
- The release includes this change list (`CHANGES.md`) next to `README.md`.
- When the game crashes, `max-go-crash.log` next to the game holds the reason.

### Fixes
- Linux: the Custom Game map previews were empty and starting a game quit with "no such file", because data file names were matched by case.
- A Constructor stayed drawn on top of its finished building after a Sentry press.
- The "time is running out" voice repeated below 20 seconds, and a new voice line could cut off the one still playing.
- A shot at a builder parked on its finished building hit the hidden building; it now hits the builder, and the
  building is lost with it.
- The missile hit message showed before the missile arrived.
- Old path arrows stayed on the map after Auto Survey or a new order replaced a halted route.
- A unit that stopped for an enemy halfway into a cell stayed drawn between two cells and blocked clicks next to it.
- Random games: factories started powered on with nothing to build, and a plant could be boxed in by other
  buildings so its units had no way out.
- Random games ignored `-difficulty` when choosing the AI strategy.
- A unit that could not afford its next step lost its leftover movement instead of keeping it.
- A Commando could disable or steal a unit that was busy firing, exploding or being destroyed.
- A Constructor destroyed while building a 2x2 building left a small rubble pile on its own cell; it now leaves the site's large pile.
- Alien derelict buildings stood without their slabs.
- A unit that drove on after an enemy halt stopped again after one cell, every turn; it now halts only when it reveals an enemy that can attack it.


## 1.0.0.2 (2026-10-20 - skipped release)

### Changes
- Gold Refinery upgrade prices follow the original game.

### QOL
- The build scripts build for Windows and Linux where the toolchain allows, each into its own `build` folder, and pack the release archives into `release`.
- The Linux release archive keeps the execute permissions, also when packed on Windows.


## 1.0.0.1 (2026-07-26)

### Fixes
- Choose Players (Custom Game): after swapping a player, a row could show Human and Computer lit at the same time, and the clan button seemed not to unlock.


## 1.0.0.0 (2026-07-26)

First release: the single player M.A.X. Go version (based on M.A.X. patch v1.0.4) for Windows and Linux (x86-64).

### QOL over the original game
- Windows and Linux support, widescreen (16:9) next to the original 4:3 view.
- Mouse wheel zoom toward the cursor.
- Unit groups 1 to 9 (Alt+1..9 to set, 1..9 to recall).
- F10 brightness, F11 fullscreen or window, F12 screenshots.
- The turn timer pauses while a dialog is open.
- Random games (`-random`) with difficulty, alien derelicts, resource richness and the number of AI players.
- Settings in `max-go.json`, edited with `max-settings`; saves in `saves`.
- Three audio backends: Oto, SoLoud and miniaudio.
- The first start finds an installed GOG or Steam copy and copies its game data.
- Tools: `max-restool` for RES archives, `max-convert` for converting game data to BMP, PNG, WAV and TXT.
- The original easter eggs are kept.
