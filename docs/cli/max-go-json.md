# max-go.json: settings file reference

`max-go.json` is the port's settings file - the replacement for the original's `settings.ini` (see `internal/settings/settings.go`,
C++ ported from `settings.cpp`).
It lives next to the binary (`bin\max-go.json`) and holds audio/video/player/game/preference/debug settings plus the remappable keybinds.
Save games are separate (`saves\*.sav`,
MAXGOSV format - see `docs/formats/save.md`);
this file is configuration only.

For an interactive editor see `docs/cli/max-settings.md` (the `max-settings` terminal tool).
For the CLI flags that override some of these fields for a single launch without persisting,
see `docs/cli/custom-game-flags.md` and `-help`'s launch override flags (`-playername`,
`-audio`,
`-scale`,
`-windowed`/`-fullscreen`,
`-debug`,
`-widescreen`,
`-difficulty`,
...).

## File discovery and load order

`settings.ResolvePath(dir)`: prefers `max-go.json` in `dir`;
if that doesn't exist but a legacy `settings.json` does,
uses that instead;
otherwise defaults to `max-go.json` (so a fresh install creates the new name). `dir` is the binary's own directory unless overridden.

`LoadOrCreate`: reads the resolved file,
or writes `Default()` if it doesn't exist yet.
Either way the loaded/created settings are immediately written back,
so the file on disk always shows the FULL current schema - a field added by a newer build appears in an old `max-go.json` the first time it loads,
seeded with its default rather than silently missing.

`Load` decodes the JSON **over a copy of `Default()`**, not over a zero value.
A field the file predates keeps its default instead of reading back as `0`/`false`/`""`.
An older `max-go.json` therefore keeps a new boolean setting enabled when the file omits it.

`Validate()` runs after every load and clamps/repairs invalid values back to a supported default (see the per-field notes below for exactly what it checks) - a hand-edited or corrupted field degrades gracefully instead of crashing the game.

`Save` first backs out any one-run launch-override flag (`-fullscreen`,
`-audio`,
`-scale`,
`-playername`,
...) from the copy being written,
so launching with an override flag never permanently changes the saved file - only an actual in-game menu edit persists.

## Top-level fields

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `data_dir` | string | `""` | Original game-data directory. Empty means "resolve automatically" - see below. |
| `language` | string | `"en-us"` | Bundled `lang-<code>.ini` choice. Only `en-us` ships today; any other value falls back to `en-us` on load (`Validate`). |
| `audio` | object | see below | |
| `video` | object | see below | |
| `player` | object | see below | |
| `game` | object | see below | |
| `preferences` | object | see below | |
| `debug` | object | see below | |
| `keybinds` | object (string→string) | see below | |

`data_dir` resolution order (`Settings.ResolveDataDir`): the `-datadir` launch flag,
then the `MAX_DATA_DIR` environment variable,
then this `data_dir` field,
then the binary's own directory (the only case where a first-run bootstrap may go hunting for an installed copy of M.A.X.;
an explicit path of any of the first three never triggers that hunt - see `DataDirExplicit`).

## `audio`

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `backend` | string | `"default"` | `"default"` (alias for `ebiten`), `"ebiten"`, `"soloud"`, or `"miniaudio"`. An unrecognized value resets to `"default"` (`Validate`). The game falls back to Ebitengine if the chosen native backend is unavailable at runtime. |
| `soloud_backend` | string | `"auto"` | SoLoud's device backend selector (platform specific presets, e.g. `wasapi`/`alsa`/`coreaudio`); an empty value resets to `"auto"`. Only consulted when `backend` is `"soloud"`. |
| `music_volume` | int | `100` | `0..100`, clamped by `Validate`. |
| `sfx_volume` | int | `100` | `0..100`, clamped by `Validate`. |
| `voice_volume` | int | `50` | `0..100`, clamped by `Validate`. |
| `music_enabled` | bool | `true` | |
| `sfx_enabled` | bool | `true` | |
| `voice_enabled` | bool | `true` | |
| `channels_reversed` | bool | `false` | Swaps left and right for positional unit sounds (the original `settings.ini` `[AUDIO_SETTINGS] channels_reversed`). |

## `video`

The game renders at a fixed logical 640x480 (854x480 in widescreen);
`window_scale` multiplies that for the actual desktop window size.

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `fullscreen` | bool | `false` | |
| `window_scale` | int | `2` | `1..8`; an out-of-range value resets to `2` (`Validate`). |
| `brightness` | int | `3` | `1..5`; the startup brightness mode, also cyclable in game with `F10`. Out of range resets to `3` (`Validate`). |
| `widescreen` | bool or `"auto"` | `"auto"` | `true` forces the wide 854x480 game view (menus stay 4:3 regardless), `false` forces classic 640x480, `"auto"` picks whichever aspect is closer to the monitor at startup. `Validate` also accepts common hand-typed spellings (`"on"`/`"yes"`/`"wide"`/`"1"` → `true`; `"off"`/`"no"`/`"0"` → `false`); anything else becomes `"auto"`. Written back to JSON as a real boolean for the two forced modes, or the string `"auto"`. |
| `popup_vanilla_look` | bool | `true` | The in-game order popup's button style (`Screen.SetPopupVanillaLook`, `internal/gamescreen/popup.go`): the retail uniform face (`true`) or the per-order bevel art baked into `MAX.RES` but unused by the retail UI (`false`) - a kept alternative look. |

## `player`

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
|
ame` | string | `"Player 1"` | **Multiplayer only** - single player never displays it. |
| `clan` | int | `7` | Clan index into the 8 clans (`A`..`H`) defined in `CLANATRB` - see `docs/formats/tables.md`; resolved per team through the Choose Players screen. |

## `game`

Mirrors the original's `[SETUP]`/`[OPTIONS]` fields: startup behavior plus the defaults offered on the Custom Game setup screen.
Most of these are also settable per-launch via the CLI flags in `docs/cli/custom-game-flags.md` (passing a flag pre-selects the setup screen AND carries into the launched game;
an unpassed flag leaves this saved value untouched).

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `intro_movie` | bool | `true` | Play the MAXINT intro movie on boot. |
| `auto_save` | bool | `true` | Silent auto-save to save slot 10 (gated on this flag; failure only logs, never blocks play). |
| `world` | int | `0` | Which bundled `.WRL` map a quick-start bench/menu defaults to. |
| `turn_timer` | int | `180` | Per-turn time budget in seconds (`0` = untimed). Custom Game screen's Turn Time box; CLI `-turnlimit`/`-turns` (`60..250`). |
| `end_turn` | int | `30` | The simultaneous-mode End Turn grace period in seconds. CLI `-endturn` (`15..90`). |
| `start_gold` | int | `200` | Starting cargo credit for the landing purchase screen. CLI `-startcredit`/`-credit` (`Low`/`Limited`/`Normal`/`High`/`TooMuch`, mapping to `0/50/100/150/200/250`). |
| `play_mode` | int | `0` | `0` = turn based, `1` = simultaneous. CLI `-turnbased`/`-simultane`. Note this port's default (turn based) is the opposite of the original's shipped default (simultaneous) - a deliberate opt-in choice. |
| `victory_type` | int | `0` | `0` = Duration (win at a turn count), `1` = Score (win at a point total). CLI `-duration`/`-score` (mutually exclusive). |
| `victory_limit` | int | `100` | The turn count (Duration) or point total (Score) `victory_type` counts toward. |
| `opponent` | int | `2` | AI difficulty tier, `0..5` = Clueless/Apprentice/Average/Expert/Master/God (matches `ai.OpponentClueless..OpponentGod`). CLI `-difficulty N` overrides for one run without changing this. |
| `raw_resource` | int | `1` | Map raw-material richness, `0/1/2` = Poor/Medium/Rich. CLI `-raw`. |
| `fuel_resource` | int | `1` | Map fuel richness, `0/1/2` = Poor/Medium/Rich. CLI `-fuel`. |
| `gold_resource` | int | `1` | Map gold richness, `0/1/2` = Poor/Medium/Rich. CLI `-gold`. |
| `alien_derelicts` | int | `0` | Neutral alien derelict density on the map. CLI `-derelicts` (`Rare`/`Common`). |
| `last_campaign` | int | `1` | One-based number of unlocked campaign missions (campaign progression). |

## `preferences`

Mirrors the original's `[PREFERENCES]` section - all apply live from the in game Options menu,
no relaunch needed.

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `effects` | bool | `true` | "Animate Effects" - sfx/animation flourishes. |
| `click_scroll` | bool | `false` | "Click-to-Scroll" edge-scroll behavior. |
| `quick_scroll` | int | `16` | Edge-scroll speed. |
| `fast_movement` | bool | `true` | "Double Unit Steps" - faster unit move animation. |
| `follow_unit` | bool | `false` | "Follow Unit" - camera tracks the selected unit. |
| `auto_select` | bool | `false` | "Auto-Select" - automatically select the next idle unit. |
| `enemy_halt` | bool | `true` | "Halt on enemy detected" - a moving unit stops when its scan reveals an enemy that can attack it (one its team could not see before), holding the rest of its route (arrows stay up) rather than losing it; it drives on at the next turn start, or at once on Continue in its popup. |

## `debug`

| JSON key | Type | Default | Notes |
| --- | --- | --- | --- |
| `menu` | bool | `false` | Opts into the in-game developer UI at runtime - the `F2` debug flag menu and the developer keys `A`/`V` (see `docs/controls.md`) - in a release build, without a `DEBUG_GAME` rebuild. Independent of the logging switches below. A `DEBUG_GAME` dev build forces this on regardless of the field's value. |
| `console` | bool | `false` | Console output; Windows keeps it visible. QuickEdit is disabled to prevent click-drag freezes. |
| `log_file` | bool | `false` | Debug output to `max-go.log` next to the binary (overwritten every launch). |
| `log_buffer_lines` | int | `100` | How many log lines batch in memory before a disk write. `<=0` resets to `100`; capped at `100000` (`Validate`). |

## `keybinds`

A `map[string]string` of remappable action → key name.
Vanilla has no key rebinding;
this is port infrastructure layered on top of the fixed original keys,
and the shipped defaults reproduce those original keys exactly.
Only these 10 actions are recognized (`internal/gamescreen/keybinds.go`) - every other shortcut (`Alt+S` save,
`Alt+L` load,
`F1` recenter,
`F5`-`F8` camera bookmarks,
unit group hotkeys,
etc.,
see `docs/controls.md`) is fixed and not remappable through this map.

| JSON key | Default | Action |
| --- | --- | --- |
| `end_turn` | `"Enter"` | End the current turn. |
| `menu` | `"Escape"` | Back out of a popup/menu, or the exit confirm. |
| `grid` | `"G"` | Toggle the grid overlay. |
| `survey` | `"Ctrl+R"` | Toggle the survey (resource) overlay. No vanilla key (panel button only) - a port pick. |
| `status` | `"Ctrl+S"` | Toggle the status overlay. No vanilla key (panel button only) - a port pick. |
| `find` | `"F"` | Centre on the selected unit. |
| `help` | `"?"` | Context help mode. |
| `options` | `"Ctrl+O"` | Open the in-game Options menu. No vanilla key - a port pick. |
| `zoom_in` | `"="` | Zoom in. |
| `zoom_out` | `"-"` | Zoom out. |

A key name is a single character, a named key (`Enter`, `Escape`, `Space`, `Tab`, `F1`-`F12`, `PageUp`, ...),
`Ctrl+<letter>`, or `Alt+<letter or digit>`; case does not matter. A plain letter is better left alone: with a
unit selected, the unit popup hotkeys run first (`docs/controls.md`), so the three port picks (`survey`,
`status`, `options`) only see a plain letter when the selected unit has no button with it. The vanilla actions
above them always win.

`Validate` fills a missing/`nil` keybinds map from the defaults;
for an existing map it only ADDS actions absent from an older schema (preserving every value the player actually set),
and moves the one known legacy collision (`survey` bound to `"F"` while `find` is also `"F"`) to `"R"`. An intentional
`survey = "F"` binding with `find` on another key stays unchanged. It also moves the old plain defaults of the port
picks (`survey` `"R"`, `status` `"S"`, `options` `"O"`, in any case) to their `Ctrl` forms once; other values stay.
`SetKeybinds` itself: an unresolvable key name keeps that action's default (and logs a warning);
an empty string value explicitly unbinds the action;
two actions sharing one key both keep it,
and the one listed first in dispatch order wins at input time (also warned).
