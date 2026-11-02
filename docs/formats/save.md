# MAXGOSV: save game format (v24)

This project uses its own save format,
rather than the reference SmartFile V70/V71 format.
A save is a snapshot of the essential game state;
the derived structures (passability grid,
visibility,
complex networks) are rebuilt from the units on load,
so they are never serialised.

Files live in the `saves\` folder next to the binary,
extension `.sav`.

## File layout

| Offset | Size | Field | Notes |
|--------|------|-------|-------|
| 0 | 8 | magic | ASCII `MAXGOSV\0` |
| 8 | 4 | version | little endian `uint32`, currently `24` |
| 12 | ... | payload | `encoding/gob` stream of the `Snapshot` struct |

Loading validates the magic,
then the version: a bad magic is an error,
and a version NEWER than the binary supports is rejected.
The schema is purely additive (each version only adds optional fields,
which gob fills with zero values when absent),
so any save at or below the current version loads.

## Snapshot (gob payload)

- `Version int`
- `MapName string`: the `.WRL` file the game started from. The loader reloads
this map,
then rebuilds the sim around the saved units.
- `Name string`: the player entered save name, shown in the save/load screen.
- `Turn, ActiveTeam, PlayerTeam, Phase, PlayMode int`
- `Teams [4]TeamSnapshot`: `{Type, Eliminated, FinishedTurn}` per team, plus (v13)
`Clan int`,
`Research [8]ResearchTopic` (each `{Level,
TurnsToComplete,
Allocation}`) and `Upgrades map[int][14]int` (bought per attribute stat offsets,
keyed by unit type id).
Without these a reload lost all research progress/allocation and every bought upgrade for future units.
Zero/nil for any pre-v13 save
- `Gold [4]int`, `TeamPoints [4]int`
- `RNG [4]uint32`: the xoshiro128** PRNG state, so a loaded game continues the
exact random stream (deterministic replay)
- `Difficulty int`: mining difficulty factor
- `Units []UnitSnapshot`
- `Builds []BuildOrderSnapshot` (v3): the active build orders (in flight construction)
- `PendingMoves []PendingMoveSnapshot` (v4): over budget (multi turn) moves resuming next turn
- `BuildPaths []BuildPathSnapshot` (v5): engineer line (Path) builds in progress
- `Spotted []SpottedSnapshot` (v6): each team's memory of spotted enemies
- `NextBuilt int` (v7): the built unit ID counter (so post load IDs do not collide)
- `Rubble []RubbleSnapshot` (v7): debris piles on the map
- `Clears []ClearJobSnapshot` (v7): bulldozer clear jobs in progress
- `BuildReady []BuildReadySnapshot` (v7): builders parked after finishing a structure
- `Resources *ResourceMapSnapshot` (v9): the resource deposit + survey map (nil for an
game not yet landed,
or any save older than v9)
- `TeamStats [4]TeamStats` (v10): per team end game score tallies (factories / mines /
buildings / units built,
buildings / units lost,
gold spent on upgrades) shown on the game over screen;
zero for any pre-v10 save
- `Complexes []ComplexSnapshot` (v11): each complex's pooled stores (`AnchorID`,
`Material`,
`Fuel`,
`Gold`) - the source of truth for a base's resources.
Keyed by the complex's lowest member building ID,
since complexes are rebuilt from the units on load.
Empty for any pre-v11 save
- `VictoryType int`, `VictoryLimit int` (v13): the match win condition + its score/turn
limit.
Without these a reload reverted to the default elimination rule. `0` for older saves. (`WinLossConditions`,
the hook a scripted mission overrides the outcome through,
is NOT serialised,
because it is a function: the game layer rebuilds it after the restore from `Mission` + `MissionIndex` below.
A game with no scripted objective runs the default rule off these two fields.)
- `ScoreHistory [4][50]int16` (v13): the Reports score graph ring buffer. Zero for older saves
- `Disabled []DisabledSnapshot` (v13): `{UnitID, Turns}` per commando disabled unit - the
recovery countdown (the unit's `OrderDisable` rides with the unit;
only the timer lives here).
Empty for older saves
- `OpponentTier int` (v14): the AI difficulty tier (0..5) so a reload keeps the computer's
strength.
A pre-v14 save restores as "unset" (the AI's launch default),
not the Clueless(0) gob's zero fill would leave
- `Bookmarks []CameraBookmarkSnapshot` (v15): `{Slot, CellX, CellY}` per assigned F5..F8
camera pin (the original's per-team `screen_locations[4]`;
this project stores the player's).
A load replaces all four slots,
so a pre-v15 save (empty list) clears them
- `Groups []UnitGroupSnapshot` (v16): `{Slot, UnitIDs}` per assigned 1..9 unit control
group (a quality-of-life addition).
Only assigned slots are stored;
a load replaces all nine,
so a pre-v16 save (empty list) clears them.
A recalled id no longer in the registry (a destroyed unit) is skipped
- `AIWinGoal [TeamCount]int` (v17): each computer team's committed AI win goal stored as
goal+1 (0 = unset,
1 = kill,
2 = colonise).
Routed through `GameState.AIWinGoal` so a reload keeps the AI's kill vs colonise plan instead of re-rolling it.
A pre-v17 save has all zeros (unset),
so the AI rolls its goal fresh on the first turn
- `AIStrategy [TeamCount]int` (v18): each computer team's committed AI strategy stored as
strategy+1 (0 = unset;
the internal/ai Strategy is resolved to a concrete one before it is stored).
Routed through `GameState.AIStrategy` so a reload keeps the AI's posture instead of re-rolling it from the map.
A pre-v18 save has all zeros (unset),
so each team's SelectStrategy roll stands on the first turn
- `Mission int` (v19): the mission category (`game.MissionCategory`: 0 none/free game,
1 training,
2 scenario,
3 campaign,
4 multi-scenario,
5 demo),
so a reloaded training mission is still a training mission.
The locked AI tier and the build-prompt voice key off it.
A pre-v19 save reads 0,
a free game,
which is what it effectively was
- `Displays *DisplaySnapshot` (v20): the nine in-game display toggles
(`Survey,
Status,
Colors,
Hits,
Ammo,
Range,
Scan,
Grid,
Names`),
so a reload gives back the overlays you were playing with.
They live on the game screen,
not the sim,
so the screen fills them in after `Capture` and applies them after `Restore` (like the camera bookmarks and the control groups).
il` on a pre-v20 save,
and then the running screen simply keeps its current toggles
- `Explored *ExploredSnapshot` (v22): `{W, H, Teams [][]byte}` - the fog of war MEMORY, one
packed bitmap per team,
one bit per cell,
row major.
The live scan coverage is rebuilt from the units on load;
where a team has BEEN is the half that cannot be re-derived,
so without this every load blacked the map out again beyond the units' current scan range.
il` on a pre-v22 save,
which then loses its exploration exactly as it always did
- `Losses []LossSiteSnapshot` (v22): `{Team, X, Y, Turn}` per remembered casualty. The AI
steers clear of ground that recently cost it a unit;
without these a reloaded computer walked its next unit back into the same ambush.
Empty for older saves
- `TurnSeconds int` (v22): seconds left on the running turn's clock, so a load resumes the
countdown instead of handing back a full fresh turn.
A screen-layer value like the display toggles. `0` = an untimed turn or an older save,
and the clock is left alone
- `MissionTitle string` / `MissionBrief string` (v22): a scripted mission's title and DESCR
briefing,
so a reloaded campaign or scenario shows its own objective on the GOAL button instead of the "Random game:" info block a skirmish gets.
Empty for a skirmish or an older save
- `MissionIndex int` (v23): WHICH mission of its category is running - the `mission_index` the
retail save header carries (`SAVE3.CAM` is campaign mission 3).
This is what a mission's scripted win and loss conditions are keyed on,
and it is vanilla's own key: its save format documents the field as determining the hardcoded victory and loss conditions,
with `0` meaning the generic rules apply.
It rides the save because the objective itself cannot: the hook is a function,
so `attachGameServices` rebuilds it from `Mission` + `MissionIndex` on every load.
Without it a reloaded campaign quietly reverted to the generic rules and could be won by simply killing everything. `0` on a pre-v23 save,
which is exactly what those saves ran on
- `HeldMoves []PendingMoveSnapshot` (v24): the routes of units that HALTED on spotting an
enemy (the "halt movement when enemy is detected" preference).
A halted unit stops but keeps the rest of its drive,
so the player is not left to remember the destination and re-issue the whole move by hand.
These are stored SEPARATELY from `PendingMoves`: a held unit offers Continue in its popup and reads as "Halted" in Reports.
Both kinds drive on at the team's next turn start; Continue presses on at once in the halt turn.
Restored as the destination alone,
since continuing re paths from wherever the unit now stands.
Empty on a pre-v24 save

### UnitSnapshot

Persists each unit's gameplay state: `ID,
Type,
Team,
Flags,
GridX,
GridY,
Order,
State,
Angle,
TurretAngle,
TurretTarget,
Hits,
Speed,
Shots,
Ammo,
Storage`,
the mining fields (`RawMining`/`FuelMining`/`GoldMining` + their max + `TotalMining`),
and the unit's attribute set as `Values [14]int` + `ValVersion`.

Relationship links (v2): `ParentID int32` (-1 = on the map;
else the carrier this unit is aboard) and `AttackTargetID int32` (-1 = none;
else the enemy a move to attack order is pursuing).
A transport's CARRIED passengers live off the unit registry (`LoadUnit` removes a boarded unit from `g.Units`),
so `Capture` walks each unit's `Carried` list to snapshot them with `ParentID` set - otherwise they would be lost on save.

Factory production (v8): `BuildQueue []int` (a factory's queued unit type ids,
element 0 = the unit currently in production),
`RepeatBuild bool` (loop the queue),
and `BuildRate int` (its 1/2/4 speed).
The active head's build ORDER is persisted separately as a `BuildOrderSnapshot`;
these fields keep the rest of the queue so a mid production factory resumes its full list (and rate) on load.

Auto Survey (v12): `AutoSurvey bool` - a player Surveyor in Auto Survey mode (drives itself to the nearest unsurveyed cell each turn).
Persisted so the mode survives a reload;
`false` for any pre-v12 save (auto off).

v13 fields: `Experience int` (commando veterancy points),
`DisabledReactionFire bool` (the per unit reaction fire '4' toggle),
`LastBuildRate int` (a stopped factory's remembered rate,
so START resumes at it) and `BridgeRaise int` (a bridge's raise level,
0 flat .. up).
Zero/false for any pre-v13 save.

Stealth detection (v21): `SpottedBy uint8` - which teams have uncovered this stealth unit,
one bit per team (bit N = team N).
A laid mine is invisible until a mine detector (surveyor,
mine layer,
or commando) comes within one cell of it,
and that mark then sticks,
so the mine stays drawn once found.
Persisting it keeps a cleared lane cleared across a reload;
`0` for any pre-v21 save,
and then a detector has to walk past the mine once more.

Prior order (v22): `PriorOrder uint8` / `PriorState uint8` - the order a unit reverts to.
A unit saved mid initialisation restored with a default prior order,
which the economy's `NetProduction` reads while a building is still coming up. `0` for any pre-v22 save,
which is what restore left there before.

Note: the F2 developer "cheat" flags (fog of war reveal,
endless mode,
show AI paths) are deliberately NOT part of a save - they are volatile UI state,
reset to the vanilla start (fog ON,
win/loss active) whenever a game is loaded.

Derived fields are not saved and are recomputed on load: pixel position (`X,
Y` from the grid cell),
and the sprite frame bases (`ImageBase` etc.,
set by the renderer from the unit's type each frame).

### BuildOrderSnapshot (v3)

Persists one active build order (a constructor raising a building,
or a factory producing a mobile unit): `BuilderID` (the on map builder it belongs to),
the unplaced product's identity (`ProductType`,
`ProductTeam`,
`ProductFlags`,
`ProductValues` + `ProductValVersion`),
the site `GridX/GridY`,
`BuildRate`,
`Progress`,
and `Started`. `TotalTurns` is recomputed from the product's `ATTRIB_TURNS`;
the product's ID + live state are (re)assigned by `finishBuild` when the build completes.

### PendingMoveSnapshot (v4)

One over budget move that ran out of movement points mid path: `UnitID` + the route's destination (`DestX/DestY`).
Only the destination is stored - on load the move is queued again and the path is recomputed at the next `BeginTeamTurn`.

### BuildPathSnapshot (v5)

One engineer line (Path) build: `BuilderID`,
the template structure being laid (`ProductType`,
`ProductFlags`,
`ProductValues` + `ProductValVersion`),
the build `Rate`,
the run of `Cells [][2]int`,
and the next cell `Idx`.
The current cell's order is in `Builds`;
this carries the remaining run,
resumed by `tickBuildPaths`.

### SpottedSnapshot (v6)

One team's memory of one enemy: `Team`,
`EnemyID`,
the last seen cell (`LastX/LastY`,
frozen while the enemy is fogged) and `Visible`.
Without it,
`Restore`'s fog recompute recomputes only the currently visible enemies,
so the AI would forget every fogged contact it was pursuing.

### ResourceMapSnapshot (v9)

The resource deposit map: `W,
H int` and two row major `[]uint8` cell arrays of length `W*H`. `Deposits` holds the packed deposit byte per cell (low 5 bits = amount,
one type bit for raw/fuel/gold);
`Surveyed` holds the per team reveal bits (bit `t` set = team `t` has surveyed that cell).
The surface source is not stored - `Restore` wires it to the reloaded map.
Without this,
a reloaded game had an empty map: existing mining stations still earned (their per resource rates are on the unit) but a NEW post load mining station yielded nothing and the AI saw no resource sites to expand toward.

### RubbleSnapshot / ClearJobSnapshot / BuildReadySnapshot (v7)

- `RubbleSnapshot`: one debris pile (`X/Y`, `Large`). Rubble is otherwise lost on
load (Restore starts on a clean field).
- `ClearJobSnapshot`: one bulldozer clear (`BulldozerID`, the site, `TurnsLeft`,
`MarkerIDs`).
The dozer + its hazard markers stay on the map (normal units);
this relinks them by ID.
- `BuildReadySnapshot`: a builder parked after finishing a structure, awaiting its
exit.
The builder is OFF the unit registry (`finishBuild` calls `g.Units.Remove`),
so it is stored here as a full `UnitSnapshot` (not in `Units`) and the finished structure is relinked by `BuildingID` - otherwise the parked constructor vanishes on save.
The structure carries `HiddenUntilExpand` so it stays hidden behind the parked builder after load.

## Load / rebuild

`Snapshot.Restore(map)` recreates a `sim.Game`:
1. `sim.New` with team types from the snapshot (fresh grid + visibility).
2. Rebuild each unit from its `UnitSnapshot` (including its `UnitValues`).
3. `sim.Game.Restore(onMap)` deploys the ON MAP units as they are (slabs are not laid again),
rejoins every building (`FlagStationary && !FlagGroundCover`) into its team complex network,
and refreshes visibility.
ABOARD passengers (`ParentID >= 0`) are NOT deployed.
4. Relink by ID: each aboard unit's `Parent` / the carrier's `Carried`, and each
`AttackTargetID` -> the pursued unit (nil if it was not saved).
5. Recreate the in flight build orders (v3): rebuild each product and reissue it
through `g.StartBuild` (which reestablishes a constructor's build pose),
then restore the order's `Progress`/`Started`.
6. Queue the resumable moves again (v4) via `g.SetPendingDestination`; restore the
engineer line builds (v5) via `g.RestoreBuildPath`;
restore the spotted memory (v6) via `g.AddSpotted` (after the fog recompute,
so frozen contacts are not lost).
7. Restore the ID counter (`g.SetNextBuilt`), the debris piles (`g.RestoreRubble`),
the bulldozer clear jobs (`g.RestoreClearJob`,
markers relinked by ID) and the parked build ready builders (`g.RestoreBuildReady`,
rebuilt off the grid) (v7).
8. Restore turn state, scores, mining difficulty, the resource deposit map (v9,
with its `Surf` wired to the reloaded map),
the per team end game statistics (v10,
assigned after unit reconstruction so a rebuild cannot inflate them) and the RNG state.
9. Reapply each complex's pooled stores (v11) to the rebuilt complex that contains
its anchor member (the rebuilt complexes start empty,
so without this the base loses its material/fuel/gold pool and an Eco Sphere/consumer can't be powered on),
then `ReconcilePower` every team so a detached,
unpowered building idles from the first frame.

## Compatibility

The version field gates the schema,
which is purely additive: each version only adds optional fields. `Load` accepts any save at or below the current `Version` (gob defaults the fields a newer field set adds) and rejects only a save written by a NEWER build.
So a v11 binary still loads v2-v10 saves;
their absent sections simply default (e.g. a pre-v9 save restores with a nil resource map,
a pre-v10 save with zeroed end game statistics,
and a pre-v11 save with empty complex stores - rebuilt to a zero pool,
as before).

## Not yet persisted (carry over)

Persisted now (a save mid action resumes faithfully): transport passengers + pursuit (v2),
active build orders (v3),
resumable moves (v4),
engineer line builds (v5),
the AI spotted memory (v6),
(v7) the build unit ID counter,
debris piles,
bulldozer clear jobs and the parked build ready builders (the last closes the off registry data loss where a finished constructor vanished on save),
(v8) the factory production queue + repeat flag + build rate,
(v9) the resource deposit + survey map (so post load mining + AI expansion are not blind),
(v10) the per team end game score statistics (so the game over table is correct after a reload),
and (v11) each complex's pooled material/fuel/gold (so a reloaded base keeps its stockpile and its buildings can still be powered on).

Still not persisted: the per team AI TASK latches (committed / search mode) - the AI recomputes these on load,
so this is graceful,
not data loss;
the per COMPUTER team build exit queue (reissued next turn);
and clan selection / per team upgrade versions beyond each unit's pinned `ValVersion`.
