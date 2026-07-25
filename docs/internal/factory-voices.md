# Factory voice events

This note maps the factory/build voice tags in the original C++ source to the Go wiring.
The Go port plays the female announcer tags (`V_F###`);
the original usually calls `SoundManager::PlayVoice(V_M###,
V_F###)` and lets the sound manager pick inside that resource range.

## C++ reference points

| Event | C++ source | C++ call | Go tag set |
|-------|-------------|-----------|------------|
| Build placement prompt | `buildmenu.cpp`, `FactoryBuildMenu::Build`; `game_manager.cpp`, placement helpers | `PlayVoice(V_M049, V_F050)` | `V_F050` |
| Builder/factory is still building | `game_manager.cpp`, `GameManager_PlayUnitStatusVoice`, `ORDER_BUILD` default branch | `PlayVoice(V_M284, V_F284)` | `V_F284` |
| Construction ready | `game_manager.cpp`, `GameManager_PlayUnitStatusVoice`, `ORDER_STATE_BUILDING_READY` | `PlayVoice(V_M162, V_F165)` | `V_F162`, `V_F163`, `V_F164`, `V_F165` |
| Factory unit ready on selected factory | `game_manager.cpp`, `GameManager_PlayUnitStatusVoice`, `ORDER_STATE_UNIT_READY` | `PlayVoice(V_M166, V_F169)` | `V_F166`, `V_F169` |
| One new factory unit this turn | `game_manager.cpp`, `GameManager_ReportNewUnitsMessage` | `PlayVoice(V_M215, V_F217)` | `V_F215`, `V_F216`, `V_F217` |
| Multiple new factory units this turn | `game_manager.cpp`, `GameManager_ReportNewUnitsMessage` | `PlayVoice(V_M206, V_F207)` | `V_F206`, `V_F207` |

`production_manager.cpp` does not play announcer voices directly for factory start/stop/stall.
It resumes halted factories when resources become available (`ProductionManager_UpdateIndustryOrders`) and composes resource messages;
the voice events live in `game_manager.cpp` / `buildmenu.cpp`.

## Go wiring

`internal/gamescreen/voicelines.go` holds these completion groups (`voice.go` itself is just the announcer engine - the queue,
cooldown,
and priority gate):

- `vsBuildComplete`: `V_F162`, `V_F163`, `V_F164`, `V_F165`
- `vsUnitComplete`: `V_F166`, `V_F169`, `V_F215`, `V_F216`, `V_F217`
- `vsNewUnits`: `V_F206`, `V_F207`
- `vsBuilding`: `V_F284`

`internal/voice/factory.go` exposes the same factory/build tags as named constants and grouping helpers.
Trigger wiring:

- The completion events (`vsBuildComplete`, `vsUnitComplete`, `vsNewUnits`) fire
from `announceBuildsCompleted` at turn end;
`vsBuilding` (F284) fires on selecting a producing factory,
never on build start (faithful: `GameManager_PlayUnitStatusVoice` speaks it for a SELECTED mid build unit).
- The **build placement prompt** (`V_F050`, `voice.BuildPlacementPrompt`) is
**TRAINING ONLY** in the original (`buildmenu.cpp:931`,
`mission_category == MISSION_CATEGORY_TRAINING`),
so a normal game stays SILENT on the build box.
Shipped: `Screen.playBuildPlacementPrompt()` (`voiceunits.go`) gates on `s.game.Mission == game.MissionTraining` and is called from both placement sites in `buildmenu.go`.
- Research `V_F093` plays in this project on the research-complete turn event (the
original plays it on selecting a powered centre with a ready topic - a kept enhancement).
All five non training events are wired.

## Transcript cross check

`compare/_voice_lines.txt` confirms the important English WAV lines:

- `F049.WAV`: position tape / begin building prompt
- `F162.WAV` and `F165.WAV`: construction/building finished
- `F166.WAV` and `F169.WAV`: unit completed/finished
- `F206.WAV` and `F207.WAV`: new units available / new units
- `F215.WAV`, `F216.WAV`, `F217.WAV`: new unit available / new unit / unit ready
- `F284.WAV`: building
