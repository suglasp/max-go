# SmartFile V70 save/scenario header

Retail M.A.X. save,
scenario,
campaign,
training,
multiplayer,
and demo state files use the Interplay **SmartFile V70** format used by the reference implementation.
This page documents the fixed header (used for menu listing,
`missionfile.ReadInfo`) and the **map layer** of the body (`missionfile.ReadMissionMap`: the terrain surface map and the resource/cargo map).
The remaining body - per team info,
the unit lists,
hash tables,
and heat maps - needs the SmartFile shared object graph and is not yet read (see "Body - units" below).

All scalar integers in the header are **little endian**.
The C++ reader uses `SmartFileReader::Read`,
which is a direct `fread` for these V70 fields on the little endian x86 target.

## Header

| Offset | Size | Type          | Field         | Meaning                                      |
|--------|------|---------------|---------------|----------------------------------------------|
| 0      | 2    | uint16        | version       | SmartFile version, must be `70`              |
| 2      | 1    | uint8         | save_type     | Raw retail game/save type                    |
| 3      | 30   | char[30]      | save_name     | NUL padded display name                      |
| 33     | 1    | uint8         | world_index   | 1-based world/map id                         |
| 34     | 2    | uint16        | mission_index | 1-based mission id in the category; 0 = none  |
| 36     | 120  | char[4][30]   | team_names    | Red, green, blue, gray names; NUL padded     |
| 156    | 5    | uint8[5]      | team_type     | Four player slots plus alien slot            |
| 161    | 5    | uint8[5]      | team_clan     | Four player slots plus alien slot            |
| 166    | 4    | uint32        | rng_seed      | Saved random seed                            |

Total fixed header size: 170 bytes.

**`mission_index` is the mission's identity,
and it carries real weight.** Vanilla holds each mission's victory and loss conditions in the executable and keys them on this field;
its own format notes describe it as determining the hardcoded victory and loss conditions,
with `0` meaning the generic custom game rules apply.
The retail files bear that out: `SAVE1..9.CAM` read 1 to 9,
`SAVE1..15.TRA` read 1 to 15,
`SAVE1..24.SCE` read 1 to 24,
and `SAVE1.DMO` reads 0.
This project keys its own objective table on `(category,
mission_index)` for the same reason - see `internal/objectives`.

`SaveLoad_GetSaveFileInfoV70` can optionally continue into a `load_options` block for in game loading;
`missionfile.ReadInfo` stops after `rng_seed`,
while `missionfile.ReadMissionMap` continues into the body described below.

## Body - options prefix and maps

`SaveLoad_LoadFormatV70` (saveload.cpp) continues straight after the 170-byte header with a small options prefix,
then the two whole map arrays.
All little endian.
Map dimensions are **not stored** - `map_cell_count = W*H` comes from the mission's world `.WRL` (all retail worlds are 112x112,
so `W*H = 12544`).

| Offset | Size | Type   | Field           | Meaning                                  |
|--------|------|--------|-----------------|------------------------------------------|
| 170    | 1    | int8   | opponent        | difficulty                               |
| 171    | 2    | uint16 | turn_timer_time | per turn time limit                      |
| 173    | 2    | uint16 | endturn_time    | end turn grace time                      |
| 175    | 1    | int8   | play_mode       | turn based / simultaneous                |
| 176    | 48   | int32[12] | load_options | `SaveLoad_LoadOptions`: world, timer, endturn, start_gold, play_mode, victory_type, victory_limit, opponent, raw_resource, fuel_resource, gold_resource, alien_derelicts |
| 224    | W*H  | uint8[]  | surface_map  | `World::GetSurfaceMap` - one byte per cell |
| 224+W*H | 2*W*H | uint16[] | cargo_map   | `ResourceManager_CargoMap` - one word per cell |

(The 6 options prefix bytes appear in a different field order in `SaveLoad_GetSaveFileInfoV70` vs `SaveLoad_LoadFormatV70`,
but always total 6,
so the offset to the maps is unaffected.)

`internal/maxsave.LoadFormatV70` captures `timer`,
`endturn`,
`start_gold`,
`play_mode`,
`victory_type`,
`victory_limit` and `opponent` from this block as `GameState.Options`;
the launcher feeds the win condition,
per turn timer and starting gold into the game so a mission ends on its own terms (the rest are consumed for alignment).
From the per team CTInfo block it also captures the full research state (level,
turns to complete and allocation per topic) plus the gold spent on upgrades stat,
and from each TeamUnits table the per type base and current UnitValues - the launcher separates the gold upgrade offsets out of current-vs-base (the research part re-derives from the levels) so a save that ships mid research or pre upgraded types restores them.

### surface_map cell

`SURFACE_TYPE_*` bits,
one per cell: `0x1` land,
`0x2` water,
`0x4` coast,
`0x8` air (`0x0` none).
A few cells in some maps carry stray high bits (e.g. `0x30` seen in 14 cells of `SAVE5.SCE`);
the surface type is taken from the low bits.
A correctly located map reads ~100% of cells as `{1,2,4,8}` - used as the alignment check in `TestReadMissionMap`.

### cargo_map cell

The per cell resource word,
matching the `CARGO_*` enum and `internal/economy`:

| Bits   | Mask   | Meaning                              |
|--------|--------|--------------------------------------|
| 0-4    | `0x1F` | deposit amount (read capped at 16)   |
| 5      | `0x20` | `CARGO_FUEL`                         |
| 6      | `0x40` | `CARGO_GOLD`                         |
| 7      | `0x80` | `CARGO_RAW`                          |
| 8-15   | high   | per team "surveyed" flags (ignored on load) |

`missionfile.MissionMap.DepositAt(x,
y)` decodes a cell to `(CargoKind,
amount)`.
Example: `SAVE1.SCE` decodes to 607 deposit cells (76 gold,
260 fuel,
271 raw).

## Body - per team info + units

After the maps,
`SaveLoad_LoadFormatV70` reads per team `CTInfo`,
then the unit lists via `SmartList_UnitInfo_FileLoad` -> `UnitInfo::FileLoad` (unitinfo.cpp:2295),
then the `UnitHash`/`MapHash`,
then per team heat maps.

`internal/maxsave.LoadFormatV70` reads this whole stretch: this project launches missions at their scripted unit positions,
so the object graph IS ported.

Per team it keeps:
- **Research** - each `CTInfo.research_topics[8]` is `ResearchTopic{research_level i32,
turns_to_complete i32,
allocation i32}` (12 bytes),
all three kept per topic (RESEARCH_TOPIC_* order: attack,
shots,
range,
armor,
hits,
speed,
scan,
cost),
so a mission resumes mid research instead of restarting the topic.
- **Gold** - `TeamUnits.gold` is a `uint16` in V70 (the first word of the team's TeamUnits
table): the team's spendable reserve.
- **Unit values** - the per type `base_unit_values[93]` and `current_unit_values[93]` tables,
which is how a mission's pre bought upgrades reach its units.
- **Gold spent on upgrades** - the score screen's tally.

`UnitInfo::FileLoad`'s fixed fields are sequential,
but its tail uses `SmartFileReader::ReadObject` for `path`,
`base_values`,
`complex`,
`parent`,
and `enemy_unit` - the SmartFile **shared object graph**,
where each object is written once and referenced by index afterwards. `internal/maxsave` ports that (`Reader.ReadObjectTyped` / `ReadObjectAs` + a class registry) along with `FileLoad` for `UnitValues`,
`Complex`,
the `UnitPath` subclasses and the `SmartObjectArray` build list.

### The mining block (a byte order trap)

Inside `UnitInfo` sits an eight byte run that a mining station's whole economy hangs on:

| bytes | field           |
|-------|-----------------|
| 1     | `build_time`    |
| 1     | `total_mining`  |
| 1     | `raw_mining`    |
| 1     | `fuel_mining`   |
| 1     | `gold_mining`   |
| 1     | `raw_mining_max`  |
| 1     | `gold_mining_max` |
| 1     | `fuel_mining_max` |

**The three rates run raw,
fuel,
gold - but the three caps run raw,
GOLD,
fuel.** Swapping the last two silently produces plausible looking nonsense,
so it is worth a sanity check when touching this: in every retail mission file a station's three rates sum to exactly 16 (the per station throughput cap) and each rate sits at or under its own cap.
Training 1's station,
for instance,
reads `raw=12 fuel=4 gold=0`.

These bytes matter because a mission AUTHORS its stations' extraction.
The live build path derives it from the map instead (`PlaceBuilding` -> `SetInitialMining`),
but `sim.Restore` deploys a mission's units exactly as they stand,
so nothing fills them in afterwards: skipping this block lands every mission's mining station extracting nothing,
with its Allocate sliders pinned at zero.

## Category mapping

`save_type` is translated by `SaveLoad_TranslateSaveFileCategory`:

| save_type | Retail name                  | Mission category                         |
|-----------|------------------------------|------------------------------------------|
| 0         | `GAME_TYPE_CUSTOM`           | `MISSION_CATEGORY_CUSTOM`                |
| 1         | `GAME_TYPE_TRAINING`         | `MISSION_CATEGORY_TRAINING`              |
| 2         | `GAME_TYPE_CAMPAIGN`         | `MISSION_CATEGORY_CAMPAIGN`              |
| 3         | `GAME_TYPE_HOT_SEAT`         | `MISSION_CATEGORY_HOT_SEAT`              |
| 4         | `GAME_TYPE_MULTI`            | `MISSION_CATEGORY_MULTI`                 |
| 5         | `GAME_TYPE_DEMO`             | `MISSION_CATEGORY_DEMO`                  |
| 8         | `GAME_TYPE_SCENARIO`         | `MISSION_CATEGORY_SCENARIO`              |
| 9         | `GAME_TYPE_MULTI_PLAYER_SCENARIO` | `MISSION_CATEGORY_MULTI_PLAYER_SCENARIO` |

Other values are rejected as invalid,
matching the C++ info reader.

## Retail data notes

In the reference 0.7.2 data folder,
the actual SmartFile V70 files are the large `SAVE*.CAM`,
`SAVE*.SCE`,
`SAVE*.TRA`,
`SAVE*.MPS`,
and `SAVE*.DMO` files.
Some companion files with the same extensions are plain mission description text instead of SmartFiles,
for example `DESCR*.SCE`,
`INTRO1.CAM`,
and `WIN9.CAM`;
these start with prose such as `Objective:` and are correctly rejected by the V70 header reader.

At least one valid retail V70 file has an empty `save_name`,
so callers should be prepared to fall back to mission metadata or the file name when building a menu label.
