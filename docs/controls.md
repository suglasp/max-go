# In game keyboard controls

The in-game shortcuts this project wires,
with the original M.A.X. key in the right column.
Sourced from the reference C++ `GameManager_ProcessKey` (`game_manager.cpp`).

## View and map

| Key | Action | Original (M.A.X.) |
|-----|--------|-------------------|
| Arrow keys | Scroll the map | same |
| `=` / `+` / PageUp | Zoom in | `=` / KP `+` |
| `-` / PageDown | Zoom out | `-` / KP `-` |
| `G` | Toggle the grid overlay | `G` |
| `F` | Find: centre on the selected unit | `F` (Find button) |
| `?` | Context help for the game screen | `?` (Shift + `/`) |
| `F1` | Centre on the last spotted enemy | `F1` |
| `F5`-`F8` | Recall a camera bookmark | `F5`-`F8` |
| `Alt`+`F5`-`F8` | Set a camera bookmark at the current view | `Alt`+`F5`-`F8` |

## Turn and game

| Key | Action | Original (M.A.X.) |
|-----|--------|-------------------|
| Return / Enter | End turn (confirm landing site during site select) | KP Enter |
| Esc | Back out of a popup / build menu / armed action, else confirm exit | Esc |
| `Alt`+`S` | Save game | `Alt`+`S` |
| `Alt`+`L` | Load game | `Alt`+`L` |
| `Alt`+`F` | File menu (save/load) | `Alt`+`F` |
| `Alt`+`P` | Pause ("Game Paused. Click OK to continue.") | `Alt`+`P` |
| `Alt`+`X` | Exit to the main menu (confirm dialog) | `Alt`+`X` |

(Save/Load were on `F9`/`F5` before;
they now match the original's `Alt`+`S` / `Alt`+`L`.)

## Selecting units

| Input | Action | Original (M.A.X.) |
|-------|--------|-------------------|
| Left click a unit | Select it (click again on a stacked cell cycles the occupants) | same |
| Left drag a box on the map | Select every own mobile unit inside the box (up to 10) | drag box (`Access_MultiSelect`) |
| `Shift` + left click a friendly unit | Add it to, or drop it from, the current selection | `Shift` add/remove (`GameManager_UpdateSelection`) |
| Left click ground with a group selected | Move the whole group in formation (each keeps its offset to the leader) | group move |
| Left click a far enemy with a group | The group moves to attack (attackers pursue, others advance) | group attack |
| Right click empty ground | Deselect | (project addition) |

A drag box selects mobile units only (buildings and enemies are skipped) and is capped at ten,
matching the original. `Ctrl` + left click stays the force attack modifier (it can fire on a friendly),
so `Shift` is free for the additive select.

## Unit control groups (C&C style, project addition)

The original has no numbered control groups;
this is a quality of life addition built on the drag box selection above.
Assigned groups ride the save game.

| Key | Action |
|-----|--------|
| `Alt`+`1`..`9` | Assign the current selection to group 1-9 |
| `1`..`9` | Recall a group: reselect its whole roster and centre on the leader |

A group slot stores the unit ids of the selection,
so a recall reselects every surviving member (destroyed units are skipped).
Membership is exclusive: assigning a unit to a group removes it from any other group it was in.

## Developer keys

`F2`,
`A`,
and `V` are gated on `s.debug`,
which is true in a `DEBUG_GAME` build OR at runtime when `debug.menu` is set to `true` in `max-go.json` (edit it with `max-settings`) - so a release build can still open these by opting in,
without a rebuild. `M` has no gate at all and always works,
`DEBUG_GAME` or not.

| Key | Action |
|-----|--------|
| `F2` | Open the debug flag menu (AI on/off, clock pause, **Add unit / Add building** to place a unit/building, etc.) - gated on `s.debug` |
| `A` | Toggle the access (passability) overlay - gated on `s.debug` |
| `V` | Toggle fog of war - gated on `s.debug` |
| `M` | Reveal every cell's resources - **always active, no gate** |

(The old `U` spawn unit and `B` build menu keys were removed;
both are now reached through the `F2` debug menu's **Add unit** / **Add building** rows.)
