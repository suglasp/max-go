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
| `F1` | Centre on the latest event: an enemy you spot, or your unit is hit, lost, or targeted by a commando | `F1` |
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

(Save/Load use `F9`/`F5` in the port;
they also match the original's `Alt`+`S` / `Alt`+`L`.)

## Unit popup hotkeys

With one of your units selected, a plain letter runs the first order-popup button whose caption starts with that
letter, whether the popup is open or not. For example, `S` toggles Sentry, `B` opens Build, `D` selects Done,
`L` selects Load, and `A` selects Attack. This matches the original (`GameManager_ProcessPopupMenuInput`) and
runs after Enter, Esc, `F`, `G`, `?`, `=`, and `-`, which keep their own meanings.

No hotkey runs during the computer's turn, a hand-over banner, or landing; while a build is placed; for a disabled
unit; for a Constructor or Engineer parked on its finished building; or for a unit between order steps, such as
one that is driving. Digits remain the unit group keys below; the original's popup digits are unused.

## Port keys (project additions)

These overlays and screens have no key in the original (panel buttons only). They sit on `Ctrl` so every plain
letter stays free for the popup hotkeys, and they are remappable in `max-go.json`.

| Key | Action |
|-----|--------|
| `Ctrl`+`S` | Toggle the status overlay |
| `Ctrl`+`R` | Toggle the survey (resource) overlay |
| `Ctrl`+`O` | Open the Options menu |

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
Membership is exclusive: assigning a unit to a group removes it from every other group it occupies.

## Developer keys

`F2`,
`A`,
and `V` are gated on `s.debug`,
which is true in a `DEBUG_GAME` build OR at runtime when `debug.menu` is set to `true` in `max-go.json` (edit it with `max-settings`) - so a release build can still open these by opting in,
without a rebuild. `Ctrl`+`M` has no gate at all and always works,
`DEBUG_GAME` or not. A plain `A` or `V` goes to the popup hotkeys first, so these two act only when the selected
unit has no button with that letter.

| Key | Action |
|-----|--------|
| `F2` | Open the debug flag menu (AI on/off, clock pause, **Add unit / Add building** to place a unit/building, etc.) - gated on `s.debug` |
| `A` | Toggle the access (passability) overlay - gated on `s.debug` |
| `V` | Toggle fog of war - gated on `s.debug` |
| `Ctrl`+`M` | Reveal every cell's resources - **always active, no gate** |

(The old `U` spawn unit and `B` build menu keys no longer apply;
the `F2` debug menu's **Add unit** / **Add building** rows provide both actions.)
