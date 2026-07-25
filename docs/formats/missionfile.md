# Mission file catalogue

Retail M.A.X. missions are stored as numbered SmartFile V70 game state files paired with optional plain text briefing files.
This project reads these through `internal/missionfile` for the New Game menus and for the shallow mission map loader.

This page documents the catalogue and pairing conventions.
The byte layout of the shared SmartFile V70 header and the currently parsed map layer is documented in `docs/formats/savefile.md`.

## File pairs

Each selectable mission uses this pair:

| Pattern | Meaning |
| --- | --- |
| `SAVE<N>.<ext>` | SmartFile V70 mission/game state file. `N` is the menu ordinal. |
| `DESCR<N>.<ext>` | Optional ASCII briefing text for the same ordinal. |

`internal/missionfile` scans `SAVE<N>.<ext>` ignoring case,
decodes the V70 header with `ReadInfo`,
pairs `DESCR<N>.<ext>` when present,
and sorts by the numeric `N`.
A bad `SAVE` file,
or one that is not SmartFile,
is skipped.
A missing or unreadable `DESCR` file leaves the description empty.

Description files are plain text and are trimmed of trailing NULs,
spaces,
tabs,
CR and LF.
They are not SmartFiles and must fail `ReadInfo`.

## Categories and extensions

The reference implementation maps the one byte `save_game_type` in the V70 header through `SaveLoad_TranslateSaveFileCategory` in `saveload.cpp`.
This project mirrors this in `missionfile.translateSaveFileCategory`.

| Extension | `save_game_type` | Reference category | Go list helper | Notes |
| --- | ---: | --- | --- | --- |
| `.DTA` | `0x00` | `MISSION_CATEGORY_CUSTOM` | none | Custom game files in the save layout. |
| `.TRA` | `0x01` | `MISSION_CATEGORY_TRAINING` | `ListTraining` | Retail training missions. |
| `.CAM` | `0x02` | `MISSION_CATEGORY_CAMPAIGN` | `ListCampaign` | Campaign missions; `INTRO*.CAM` and `WIN*.CAM` are cinematics/text companions, not `SAVE` entries. |
| `.HOT` | `0x03` | `MISSION_CATEGORY_HOT_SEAT` | none | Hot seat save category. |
| `.MLT` | `0x04` | `MISSION_CATEGORY_MULTI` | none | Multiplayer save category. |
| `.DMO` | `0x05` | `MISSION_CATEGORY_DEMO` | `ListDemos` | Attract mode demo recordings. |
| `.SCE` | `0x08` | `MISSION_CATEGORY_SCENARIO` | `ListScenarios` | Standalone single player scenarios. |
| `.MPS` | `0x09` | `MISSION_CATEGORY_MULTI_PLAYER_SCENARIO` | `ListMultiScenarios` | Retail multiplayer scenarios, surfaced by this project as custom scenarios. |

The reference implementation also defines internal/debug text game types `0x06` and `0x07`,
but the V70 info reader rejects them because they do not translate to mission categories used by normal save loading.

## Parsed header

Every selectable mission file starts with the same fixed 170-byte V70 header:

| Offset | Size | Field | Go field |
| ---: | ---: | --- | --- |
| `0` | `2` | SmartFile version, must be `70` | `Info.Version` |
| `2` | `1` | raw save/game type | `Info.SaveType`, translated to `Info.Category` |
| `3` | `30` | NUL padded save/mission name | `Info.Name` |
| `33` | `1` | world index | `Info.WorldIndex` |
| `34` | `2` | mission index | `Info.MissionIndex` |
| `36` | `120` | four NUL padded team names | `Info.TeamNames` |
| `156` | `5` | four player team types plus alien slot | `Info.TeamType` |
| `161` | `5` | four player clans plus alien slot | `Info.TeamClan` |
| `166` | `4` | RNG seed | `Info.RNGSeed` |

All scalar fields are little endian.
Fixed strings are read as 30 bytes and trimmed at the first NUL byte;
no codepage conversion is applied.

## Parsed map layer

`ReadMissionMap` validates the header,
skips the first body settings,
then reads the surface and resource maps.
The mission file does not store map dimensions in this shallow parser;
callers pass the width and height from the selected `.WRL`.

After the 170-byte header:

| Offset | Size | Field |
| ---: | ---: | --- |
| `170` | `1` | opponent/difficulty byte |
| `171` | `2` | turn timer |
| `173` | `2` | end turn timer |
| `175` | `1` | play mode |
| `176` | `48` | twelve `int32` load options from `SaveLoad_LoadOptions` |
| `224` | `W*H` | surface map, one byte per cell |
| `224+W*H` | `2*W*H` | cargo/resource map, one little endian `uint16` per cell |

The cargo word uses the low byte for deposit data: low five bits are amount (capped to 16 by the Go helper),
`0x20` is fuel,
`0x40` is gold,
and `0x80` is raw material.
The upper byte holds surveyed flags.

The remaining body is the full reference SmartFile object graph: team state,
unit lists,
unit hashes,
map hashes and heat maps.
That is not this project's `MAXGOSV` save format;
see `docs/formats/save.md` for native saves.

## Current Go behaviour

- `ListScenarios`, `ListCampaign`, `ListTraining`, `ListMultiScenarios` and `ListDemos` are
menu catalogue helpers only;
they do not load the whole game state body.
- `ReadInfo` rejects files whose version is not `70` or whose category is not in
the retail mission category set.
- `ListCampaign` naturally excludes `INTRO*.CAM` and `WIN*.CAM` because it only
matches `SAVE<N>.CAM`.
- Empty mission names are allowed. Callers should fall back to the `SAVE<N>`
stem or other metadata for display.
- `internal/saveload` is separate: it reads and writes this project's `MAXGOSV`
`.sav` snapshots,
not retail SmartFile V70 mission files.
