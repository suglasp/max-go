# Custom Game CLI Flags

These flags mirror the Custom Game setup screen.
At boot `applyCustomGameFlags` (`cmd/max-go/custom_flags.go`) folds a passed flag into `settings.Game` before any setup screen or launch reads it,
so a flag both shows on the Custom Game setup screen and carries into the launched game.
Only flags the player actually passes take effect,
so the `-raw` / `-fuel` / `-gold` Medium defaults never clobber a saved level.
The `-random` / `-emulate random2` quick starts adopt EXPLICITLY passed flags too: `-raw`/`-fuel`/`-gold` seed the bench map at the flag richness,
`-startcredit` hands the landing gold,
and `-score`/`-duration` set a real victory (otherwise the benches stay endless with their own defaults). `-emulate custom` keeps its hardcoded dev loadout.

The value parser is case insensitive for named options and normalizes accepted values to the spelling shown below.

| Flag | Type | Default | Valid values | Example | C++ setting / panel |
| --- | --- | --- | --- | --- | --- |
| `-turnlimit` | string | `""` | `""`, integer `60`..`250` | `-turnlimit 120` | Turn timer seconds (the setup screen Turn Time), `settings["timer"]`. Despite the name it is NOT a turn cap; the Duration victory turn count is `-duration`. |
| `-turns` | string | `""` | `""`, integer `60`..`250` | `-turns 120` | Alias of `-turnlimit`; normalized into the same field. |
| `-endturn` | string | `""` | `""`, integer `15`..`90` | `-endturn 45` | End of turn grace seconds, `settings["endturn"]`. |
| `-score` | string | `""` | `""`, `Low`, `Medium`, `High` | `-score High` | Score victory preset (200/400/800 points); maps to `victory_type` 1 / `victory_limit`. Mutually exclusive with `-duration`. |
| `-duration` | string | `""` | `""`, integer `1`..`9999` | `-duration 200` | Duration victory turn count (the setup screen Turns box; presets there are 100/200/400); maps to `victory_type` 0 / `victory_limit`. Mutually exclusive with `-score`. |
| `-turnbased` | bool | effective default | `true` / `false` | `-turnbased` | Play mode, `settings["play_mode"] == 0`. |
| `-simultane` | bool | `false` | `true` / `false` | `-simultane` | Play mode, `settings["play_mode"] == 1`. Mutually exclusive with `-turnbased`. |
| `-raw` | string | `Medium` | `Poor`, `Medium`, `Rich` | `-raw Rich` | Raw resource density, `settings["raw_resource"]`. |
| `-fuel` | string | `Medium` | `Poor`, `Medium`, `Rich` | `-fuel Poor` | Fuel resource density, `settings["fuel_resource"]`. |
| `-gold` | string | `Medium` | `Poor`, `Medium`, `Rich` | `-gold Medium` | Gold resource density, `settings["gold_resource"]`. |
| `-derelicts` | string | `""` | `""`, `Rare`, `Common` | `-derelicts Common` | Alien derelict density, `settings["alien_derelicts"]`. |
| `-startcredit` | string | `""` | `""`, `Low`, `Limited`, `Normal`, `High`, `TooMuch` | `-startcredit Normal` | Starting cargo credits, `settings["start_gold"]`. |
| `-credit` | string | `""` | `""`, `Low`, `Limited`, `Normal`, `High`, `TooMuch` | `-credit Normal` | Alias of `-startcredit`; normalized into the same field. |

When neither `-turnbased` nor `-simultane` is supplied,
the intended effective mode is turn based.
Supplying both is invalid.

## Examples

```powershell
.\max-go.exe -random -turns 120 -endturn 45 -raw Rich -fuel Medium -gold Poor -credit Normal
.\max-go.exe -emulate random2 -simultane -score High -derelicts Common
.\max-go.exe -duration 200 -startcredit High
```
