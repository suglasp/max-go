// saveload.go : reads and writes MAXGOSV save games, a versioned snapshot of essential game state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// magic identifies a MAXGOSV file; Version is the snapshot schema version.
var magic = [8]byte{'M', 'A', 'X', 'G', 'O', 'S', 'V', 0}

// Version is the current snapshot schema version. v2 adds the unit relationship
// links (ParentID/AttackTargetID); v3 the in flight build orders (Builds); v4 the
// resumable over budget moves (PendingMoves); v5 the engineer line builds
// (BuildPaths); v6 the AI spotted enemy memory (Spotted); v7 the build unit ID
// counter, debris piles, bulldozer clear jobs and parked build ready builders;
// v8 the factory production queue + repeat flag (BuildQueue/RepeatBuild); v9 the
// resource deposit map (Resources) so mining sites + the survey overlay survive a
// reload (older saves decode with a nil Resources); v10 the per team end game
// score statistics (TeamStats) so the game over table is correct after a reload;
// v11 each complex's pooled stores (Complexes - material/fuel/gold) so a reloaded
// base keeps its stockpile and its buildings can still be powered on; v12 the
// player Surveyor's Auto Survey flag (AutoSurvey) so a saved auto surveyor keeps
// auto driving after a reload; v13 the previously dropped state batch: per team
// Research (levels/progress/allocation), Upgrades (bought stat offsets) and Clan;
// the match VictoryType/VictoryLimit and the ScoreHistory graph; the commando
// Disable countdowns; and per unit Experience, DisabledReactionFire, LastBuildRate
// and BridgeRaise. v14 the AI opponent difficulty tier (OpponentTier) so a loaded
// game keeps the computer's strength instead of reverting to a default (a pre v14
// save has no field and restores as "unset", the AI's launch default, rather than
// the CLUELESS that gob's zero fill would otherwise leave). v15 the player's
// F5..F8 camera bookmarks (Bookmarks) so the pinned view spots survive a reload
// (empty for older saves; only assigned slots are stored). v16 the numbered unit
// control groups (Groups - 1..9) so a saved selection roster survives a reload
// (empty for older saves; only assigned slots are stored). v17 each computer team's
// committed AI win goal (AIWinGoal, stored as goal+1) so a reload keeps the AI's kill
// vs colonise plan instead of re-rolling it (0 for older saves = the AI rolls afresh).
// v18 each computer team's committed strategy (AIStrategy, stored as strategy+1) so a
// reload keeps the AI's posture instead of re-rolling it from the map (0 for older
// saves = the AI rolls afresh). v19 the mission category (Mission) so a reloaded
// training mission is still a training mission - the trimmed build roster, the locked
// AI tier and the build prompt voice all key off it (0 for older saves = MissionNone,
// a free game, which is what every pre v19 save effectively was). v20 the nine display
// toggles (Displays - Survey, Status, Colors, Hits, Ammo, Range, Scan, Grid, Names) so a
// reload gives you back the overlays you were playing with instead of switching them all
// off (nil for older saves = the defaults, exactly what a pre v20 load did). v21 which
// teams have uncovered a stealth unit (UnitSnapshot.SpottedBy, a bit per team) so an enemy
// mine your surveyor found stays on the map after a reload instead of going invisible again
// (0 for older saves = nobody has found it, and a detector has to walk past it once more).
// v22 the fog EXPLORED memory (Explored - one packed bitmap per team, a bit per cell): the
// scan counters are re derived from the units on load, but where a team has BEEN cannot be,
// so every load used to black out the whole map outside the units' current scan range. A pre
// v22 save has no bitmap and still loses its exploration, which is all it ever held.
// v24 the routes a HALT ON ENEMY stop is holding (HeldMoves): a halted unit keeps the route it
// was driving, shown but never auto resumed, so without this a save and load would re introduce
// the very forgetting the hold exists to fix. Empty on an older save.
// v23 WHICH mission is running (MissionIndex, the retail header's mission_index): the
// category alone does not say whether this is campaign 1 or campaign 8, and the mission's
// win/loss OBJECTIVE is keyed on it - a reloaded mission without it would quietly revert
// to the generic rules (0 for older saves = no scripted objective, which is exactly what
// every pre v23 save ran on).
// v25 the screen's unit counter (SpawnSeq), the number the last unit made by the unit factory
// got: a load that starts it again at 0 hands out IDs the saved units already carry, and the
// registry then drops the older unit. A pre v25 save has 0 here and the screen derives the
// counter from the highest unit ID it restores.
// v26 the game's own Turn Time and End Turn Time (TurnTimer, EndTurn): a load used to take both
// from the current settings. A pre v26 save has no fields and keeps the settings, as before.
const Version = 26

// Snapshot is the serializable game state. Restore rebuilds derived grids, visibility, and complexes from it.
type Snapshot struct {
	Version      int
	MapName      string // the .WRL file to reload
	Name         string // player entered save name (shown in the save/load screen)
	Turn         int
	ActiveTeam   int
	PlayerTeam   int
	Phase        int
	PlayMode     int
	Teams        [game.TeamCount]TeamSnapshot
	Gold         [game.TeamCount]int
	TeamPoints   [game.TeamCount]int
	RNG          [4]uint32
	Units        []UnitSnapshot
	Difficulty   int
	Builds       []BuildOrderSnapshot          // active construction orders (in flight builds)
	PendingMoves []PendingMoveSnapshot         // over budget moves resuming next turn
	HeldMoves    []PendingMoveSnapshot         // routes HELD by a halt on enemy stop (v24): shown, never auto resumed
	BuildPaths   []BuildPathSnapshot           // engineer line (Path) builds in progress
	Spotted      []SpottedSnapshot             // per team memory of spotted enemies
	NextBuilt    int                           // the built unit ID counter (avoids post load ID collisions)
	Rubble       []RubbleSnapshot              // debris piles on the map
	Clears       []ClearJobSnapshot            // bulldozer clear jobs in progress
	BuildReady   []BuildReadySnapshot          // builders parked after finishing a structure (OFF the registry)
	Resources    *ResourceMapSnapshot          // the deposit + survey map (v9); nil for older saves / un landed games
	TeamStats    [game.TeamCount]sim.TeamStats // per team end game score tallies (v10); zero for older saves
	Complexes    []ComplexSnapshot             // per complex pooled stores (v11); empty for older saves
	VictoryType  int                           // match victory rule (winloss.VictoryType), v13; 0 = default elimination for older saves
	VictoryLimit int                           // the score/turn limit that pairs with VictoryType
	ScoreHistory [game.TeamCount][50]int16     // the Reports score graph ring buffer; zero for older saves
	Disabled     []DisabledSnapshot            // commando disable recovery countdowns; empty for older saves
	OpponentTier int                           // the AI difficulty tier (0..5), v14; older saves keep the AVERAGE default (see Restore)
	Bookmarks    []CameraBookmarkSnapshot      // the player's assigned F5..F8 camera pins (v15); empty for older saves
	Groups       []UnitGroupSnapshot           // the player's assigned 1..9 control groups (v16); empty for older saves
	AIWinGoal    [game.TeamCount]int           // each computer team's committed AI win goal + 1 (v17); 0 = unset (older saves / not yet rolled)
	AIStrategy   [game.TeamCount]int           // each computer team's committed AI strategy + 1 (v18); 0 = unset (older saves / not yet rolled)
	Mission      int                           // the mission category (game.MissionCategory), v19; 0 = MissionNone for older saves
	MissionIndex int                           // WHICH mission of that category (the retail mission_index), v23; 0 = no scripted objective
	Displays     *DisplaySnapshot              // the nine in game display toggles (v20); nil for older saves = the defaults
	Explored     *ExploredSnapshot             // the per team fog memory (v22); nil for older saves = the exploration is lost, as before
	Losses       []LossSiteSnapshot            // the AI's recent loss memory (v22); empty for older saves
	TurnSeconds  int                           // seconds left on the running turn's clock (v22); 0 = untimed / older save, and the turn restarts full
	MissionTitle string                        // a scripted mission's title (v22); empty for a skirmish / older save
	MissionBrief string                        // a scripted mission's briefing text (v22), so a reload keeps its objective instead of the "Random game:" block
	SpawnSeq     int                           // the screen's unit counter (v25), so a load never reuses a saved unit's ID; 0 on an older save
	TurnTimer    int                           // the game's full Turn Time in seconds (v26); 0 = untimed, or an older save (see Version)
	EndTurn      int                           // the game's End Turn Time in seconds (v26); 0 = none, or an older save (see Version)
}

// LossSiteSnapshot is one remembered casualty: the cell where a team lost a mobile unit
// and the turn it happened. The AI avoids ground that recently cost it a unit, so a load
// without these walks the next unit back into the same ambush.
type LossSiteSnapshot struct {
	Team int
	X, Y int
	Turn int
}

// ExploredSnapshot persists the fog of war memory: for each team a bitmap with one bit
// per cell, row major, set for every cell that team has ever had within scan range. The
// live scan coverage is rebuilt from the units on load and is NOT stored; this is the
// part that cannot be re derived, because it records where the team has been.
type ExploredSnapshot struct {
	W, H  int
	Teams [][]byte
}

// DisplaySnapshot persists the in game display toggles - the seven panel squares plus the
// grid and the unit name overlays. They live on the game screen, not the sim, so the screen
// fills this in after Capture and applies it after Restore, the way the camera bookmarks and
// the control groups already do. The original saves its display flags with the game
// (saveload.cpp writes the settings block), so a reload keeps the view you set up.
type DisplaySnapshot struct {
	Survey bool
	Status bool
	Colors bool
	Hits   bool
	Ammo   bool
	Range  bool
	Scan   bool
	Grid   bool
	Names  bool
}

// UnitGroupSnapshot persists one assigned control group: the slot index (0..8 =
// keys 1..9) and the ids of its member units. Unassigned slots are not stored.
// The groups live on the game screen, not the sim, so the screen fills this in
// after Capture and applies it after Restore. M.A.X. itself has no unit control
// groups; this is a port quality of life addition (like the camera bookmarks).
type UnitGroupSnapshot struct {
	Slot    int
	UnitIDs []uint16
}

// CameraBookmarkSnapshot persists one assigned camera bookmark: the slot index
// (0..3 = F5..F8) and the pinned map cell. Unassigned slots are not stored. The
// bookmarks live on the game screen, not the sim, so the screen fills this in
// after Capture and applies it after Restore.
// C++ ported : ctinfo.hpp - CTInfo screen_locations[4], written in saveload.cpp.
type CameraBookmarkSnapshot struct {
	Slot  int
	CellX int
	CellY int
}

// DisabledSnapshot persists one commando disabled unit's recovery countdown (v13):
// the unit's OrderDisable is saved with the unit, but the turns remaining timer
// lives in a side map, so without this a disabled unit never recovers correctly.
type DisabledSnapshot struct {
	UnitID uint16
	Turns  int
}

// ComplexSnapshot stores a v11 complex's pooled resources under its lowest member ID. Restore uses that anchor to find the adjacency-rebuilt complex.
type ComplexSnapshot struct {
	AnchorID uint16
	Material int
	Fuel     int
	Gold     int
}

// ResourceMapSnapshot persists the resource deposit map: the per cell packed
// deposit bytes and the per team surveyed bits (each row major, length W*H). The
// surface source is rebuilt from the map on load, not stored.
type ResourceMapSnapshot struct {
	W, H     int
	Deposits []uint8
	Surveyed []uint8
}

// RubbleSnapshot is one debris pile: its cell, whether it is a large (building)
// pile, and the raw a bulldozer reclaims on clear. Material is additive (gob
// tolerates the added field, so older saves decode it as 0).
type RubbleSnapshot struct {
	X        int
	Y        int
	Large    bool
	Material int
}

// ClearJobSnapshot persists one bulldozer clear job: the dozer, the site, the turns
// of work left, and the hazard markers (on map units re linked by ID).
type ClearJobSnapshot struct {
	BulldozerID uint16
	CellX       int
	CellY       int
	Footprint   int
	TurnsLeft   int
	MarkerIDs   []uint16
}

// BuildReadySnapshot stores a builder parked after completing a structure and links the finished structure by ID.
type BuildReadySnapshot struct {
	Builder    UnitSnapshot
	Product    int
	BuildingID uint16
	SiteX      int
	SiteY      int
	Footprint  int
}

// PendingMoveSnapshot persists one over budget (multi turn) move: the unit and the
// destination of the route it could not finish this turn. On load it is re queued
// so the unit resumes toward the destination next turn (the path is recomputed).
type PendingMoveSnapshot struct {
	UnitID uint16
	DestX  int
	DestY  int
}

// BuildPathSnapshot persists one engineer line (Path) build: the builder, the
// template structure being laid, the run of cells and the next index. The current
// cell's build order lives in Builds; this carries the remaining run.
type BuildPathSnapshot struct {
	BuilderID         uint16
	ProductType       int
	ProductFlags      uint32
	ProductValues     [game.AttribCount]int
	ProductValVersion int
	Rate              int
	Cells             [][2]int
	Idx               int
}

// SpottedSnapshot persists one team's memory of one enemy: the last seen cell
// (frozen while the enemy is fogged) and whether it is currently visible. Without
// it, Restore's fog recompute re derives only visible enemies, so the AI forgets
// every fogged contact it was tracking.
type SpottedSnapshot struct {
	Team    int
	EnemyID uint16
	LastX   int
	LastY   int
	Visible bool
}

// BuildOrderSnapshot persists one active build order: a builder (factory or
// constructor, referenced by ID) turning material into a product over turns. The
// product is the unplaced unit being built - its identity (type/flags/values) is
// stored so the order is re created with g.StartBuild on load; the product's ID
// and live state are (re)assigned by finishBuild when the build completes.
type BuildOrderSnapshot struct {
	BuilderID         uint16
	ProductType       int
	ProductTeam       int
	ProductFlags      uint32
	ProductValues     [game.AttribCount]int
	ProductValVersion int
	GridX, GridY      int
	BuildRate         int
	Progress          int
	Started           bool
}

// TeamSnapshot is one team's persisted turn state.
type TeamSnapshot struct {
	Type         int
	Eliminated   bool
	FinishedTurn bool
	// v13: per team state previously dropped on save.
	Clan     int                                         // clan id (AI strategy / colour)
	Research [game.ResearchTopicCount]game.ResearchTopic // research level/progress/allocation per topic
	Upgrades map[int][game.AttribCount]int               // bought stat offsets, keyed by unit type int; nil for older saves
}

// UnitSnapshot is one unit's persisted gameplay state. Derived fields (pixel
// position, sprite frame bases) are recomputed on load.
type UnitSnapshot struct {
	ID                   uint16
	Type                 int
	Team                 int
	Flags                uint32
	GridX, GridY         int
	Order                uint8
	State                uint8
	PriorOrder           uint8 // v22: the order a unit reverts to; NetProduction reads it while a building is still initialising
	PriorState           uint8
	Angle                uint8
	TurretAngle          uint8
	TurretTarget         uint8
	Hits                 int
	Speed                int
	Shots                int
	Ammo                 int
	Storage              int
	RawMining            int
	FuelMining           int
	GoldMining           int
	RawMiningMax         int
	FuelMiningMax        int
	GoldMiningMax        int
	TotalMining          int
	Values               [game.AttribCount]int
	ValVersion           int
	ParentID             int32 // carrier (transport) this unit is aboard, or -1 (captured via Carried, off the unit registry)
	AttackTargetID       int32 // enemy this unit is pursuing (ORDER_MOVE_TO_ATTACK), or -1
	HiddenUntilExpand    bool  // a just finished structure parked behind its build ready builder, not yet drawn
	BuildQueue           []int // factory's production queue ([0] = unit currently building); active head's ORDER is in Snapshot.Builds
	RepeatBuild          bool  // loops BuildQueue
	BuildRate            int   // factory build speed (1/2/4)
	AutoSurvey           bool  // player Surveyor in Auto Survey mode; persisted v12, older saves decode false
	Experience           int   // commando veterancy points (v13)
	DisabledReactionFire bool  // the per unit reaction fire toggle ('4')
	LastBuildRate        int   // a STOPPED factory's remembered rate (START resumes here)
	BridgeRaise          int   // a bridge's raise level (0 flat .. BridgeRaiseMax up)
	SpottedBy            uint8 // teams that uncovered this stealth unit (a found mine stays found); persisted v21, older saves decode 0
}

// Capture reads a live game into a serialisable snapshot. mapName is the .WRL
// file the game was started from (needed to rebuild the map on load).
func Capture(g *sim.Game, mapName string) *Snapshot {
	s := &Snapshot{
		Version:    Version,
		MapName:    mapName,
		Turn:       g.State.TurnCounter,
		ActiveTeam: g.State.ActiveTeam,
		PlayerTeam: g.State.PlayerTeam,
		Phase:      int(g.State.Phase),
		PlayMode:   int(g.State.PlayMode),
		Gold:       g.Gold,
		TeamPoints: g.TeamPoints,
		TeamStats:  g.TeamStats,
		RNG:        game.RandomizerState(),
		Difficulty: g.MiningDifficulty,
		// v19: which kind of mission this is (a reloaded training mission must stay one).
		Mission: int(g.Mission),
		// v23: WHICH mission, so the reload can put its objective back.
		MissionIndex: g.MissionIndex,
		// v13: match victory rule + the score graph history.
		VictoryType:  int(g.VictoryType),
		VictoryLimit: g.VictoryLimit,
		ScoreHistory: g.ScoreHistory,
		// v14: the computer opponent tier so a reload keeps the AI's strength.
		OpponentTier: g.State.Difficulty,
		// v17: each computer team's committed win goal so a reload keeps its plan.
		AIWinGoal: g.State.AIWinGoal,
		// v18: each computer team's committed strategy so a reload keeps its posture.
		AIStrategy: g.State.AIStrategy,
	}
	captureTeams(s, g)
	captureUnits(s, g)
	captureEconomy(s, g)
	captureOrders(s, g)
	captureFieldState(s, g)
	captureMapState(s, g)
	// v22: the AI's recent loss memory, so a reloaded computer still avoids the ground
	// that just cost it a unit.
	for team := 0; team < game.TeamCount; team++ {
		for _, site := range g.RecentLosses(team) {
			s.Losses = append(s.Losses, LossSiteSnapshot{Team: team, X: site.X, Y: site.Y, Turn: site.Turn})
		}
	}
	return s
}

// captureTeams copies the per team roster state: type, elimination, turn flag, clan,
// research and the bought upgrade offsets.
func captureTeams(s *Snapshot, g *sim.Game) {
	// Only the playable teams are serialised; the fixed alien slot (index TeamCount)
	// carries no per team save state, so the snapshot array stays sized to TeamCount.
	for i := 0; i < game.TeamCount; i++ {
		t := g.State.Teams[i]
		ts := TeamSnapshot{
			Type: int(t.Type), Eliminated: t.Eliminated, FinishedTurn: t.FinishedTurn,
			Clan: t.Clan, Research: t.Research, // v13
		}
		// v13: the bought upgrade offset map, copied by value (key = unit type int).
		if len(t.Upgrades) > 0 {
			ts.Upgrades = make(map[int][game.AttribCount]int, len(t.Upgrades))
			for typ, offs := range t.Upgrades {
				if offs != nil {
					ts.Upgrades[int(typ)] = *offs
				}
			}
		}
		s.Teams[i] = ts
	}
}

// captureUnits copies every unit, passengers included, plus the commando disable
// countdowns that live beside them.
func captureUnits(s *Snapshot, g *sim.Game) {
	// v13: the commando disable recovery countdowns (the units carry OrderDisable;
	// only the per unit timer lives in the side map).
	for _, ds := range g.DisabledStates() {
		s.Disabled = append(s.Disabled, DisabledSnapshot{UnitID: ds.Unit.ID, Turns: ds.Turns})
	}
	for _, u := range g.Units.Units() {
		captureWithCarried(s, u, -1)
	}
}

// captureEconomy copies each complex's pooled stores, the source of truth for what a
// base actually holds.
func captureEconomy(s *Snapshot, g *sim.Game) {
	// Per complex pooled stores (v11): the source of truth for a base's resources.
	// Keyed by the complex's lowest member building ID so it re attaches to the
	// rebuilt complex on load (membership is identical, derived from the units).
	for _, c := range g.Complexes {
		if c == nil || len(c.Members) == 0 {
			continue
		}
		anchor := c.Members[0].ID
		for _, m := range c.Members {
			if m.ID < anchor {
				anchor = m.ID
			}
		}
		s.Complexes = append(s.Complexes, ComplexSnapshot{
			AnchorID: anchor, Material: c.Material, Fuel: c.Fuel, Gold: c.Gold,
		})
	}
}

// captureOrders copies the work in flight: the build orders, the moves that ran out of
// budget, the routes an enemy halt is holding and the engineer line builds.
func captureOrders(s *Snapshot, g *sim.Game) {
	for _, o := range g.BuildOrders() {
		if o.Builder == nil || o.Product == nil {
			continue
		}
		bos := BuildOrderSnapshot{
			BuilderID:    o.Builder.ID,
			ProductType:  int(o.Product.Type),
			ProductTeam:  o.Product.Team,
			ProductFlags: uint32(o.Product.Flags),
			GridX:        o.GridX,
			GridY:        o.GridY,
			BuildRate:    o.BuildRate,
			Progress:     o.Progress,
			Started:      o.Started,
		}
		if o.Product.Values != nil {
			for a := game.Attrib(0); a < game.AttribCount; a++ {
				bos.ProductValues[a] = o.Product.Values.Get(a)
			}
			bos.ProductValVersion = o.Product.Values.Version()
		}
		s.Builds = append(s.Builds, bos)
	}
	for _, pr := range g.PendingRoutes() {
		if pr.Unit == nil || len(pr.Steps) == 0 {
			continue
		}
		dest := pr.Steps[len(pr.Steps)-1].To
		s.PendingMoves = append(s.PendingMoves, PendingMoveSnapshot{
			UnitID: pr.Unit.ID, DestX: dest.X, DestY: dest.Y,
		})
	}
	// v24: the routes a halt on enemy stop is HOLDING. Same shape as a pending move (only
	// the destination is worth keeping - continuing re paths anyway), but a DIFFERENT list,
	// because a held route must never be resumed automatically on load.
	for _, hr := range g.HeldRoutes() {
		if hr.Unit == nil || len(hr.Steps) == 0 {
			continue
		}
		dest := hr.Steps[len(hr.Steps)-1].To
		s.HeldMoves = append(s.HeldMoves, PendingMoveSnapshot{
			UnitID: hr.Unit.ID, DestX: dest.X, DestY: dest.Y,
		})
	}
	for _, bp := range g.BuildPathStates() {
		if bp.Builder == nil || len(bp.Cells) == 0 {
			continue
		}
		bps := BuildPathSnapshot{
			BuilderID:    bp.Builder.ID,
			ProductType:  int(bp.Product),
			ProductFlags: uint32(bp.Flags),
			Rate:         bp.Rate,
			Cells:        bp.Cells,
			Idx:          bp.Idx,
		}
		if bp.Values != nil {
			for a := game.Attrib(0); a < game.AttribCount; a++ {
				bps.ProductValues[a] = bp.Values.Get(a)
			}
			bps.ProductValVersion = bp.Values.Version()
		}
		s.BuildPaths = append(s.BuildPaths, bps)
	}
}

// captureFieldState copies what sits on the field rather than on a unit: the AI's
// spotted enemy memory, the built unit ID counter, the debris piles, the bulldozer
// clear jobs and the builders parked on a finished structure.
func captureFieldState(s *Snapshot, g *sim.Game) {
	for _, sp := range g.SpottedStates() {
		if sp.Enemy == nil {
			continue
		}
		s.Spotted = append(s.Spotted, SpottedSnapshot{
			Team: sp.Team, EnemyID: sp.Enemy.ID,
			LastX: sp.LastX, LastY: sp.LastY, Visible: sp.Visible,
		})
	}
	s.NextBuilt = g.NextBuilt()
	for _, rc := range g.RubbleCells() {
		s.Rubble = append(s.Rubble, RubbleSnapshot{X: rc.X, Y: rc.Y, Large: rc.Large, Material: rc.Material})
	}
	for _, cj := range g.ClearJobStates() {
		if cj.Bulldozer == nil {
			continue
		}
		markerIDs := make([]uint16, 0, len(cj.Markers))
		for _, m := range cj.Markers {
			if m != nil {
				markerIDs = append(markerIDs, m.ID)
			}
		}
		s.Clears = append(s.Clears, ClearJobSnapshot{
			BulldozerID: cj.Bulldozer.ID, CellX: cj.CellX, CellY: cj.CellY,
			Footprint: cj.Footprint, TurnsLeft: cj.TurnsLeft, MarkerIDs: markerIDs,
		})
	}
	for _, br := range g.BuildReadyStates() {
		if br.Builder == nil {
			continue
		}
		brs := BuildReadySnapshot{
			Builder: captureUnit(br.Builder, -1),
			Product: int(br.Product),
			SiteX:   br.SiteX, SiteY: br.SiteY, Footprint: br.Footprint,
		}
		if br.Building != nil {
			brs.BuildingID = br.Building.ID
		}
		s.BuildReady = append(s.BuildReady, brs)
	}
}

// captureMapState copies the two map wide layers a load cannot re derive from the
// units: the resource deposit map and the fog exploration memory.
func captureMapState(s *Snapshot, g *sim.Game) {
	if g.Resources != nil {
		dep, surv := g.Resources.Export()
		s.Resources = &ResourceMapSnapshot{W: g.Resources.W, H: g.Resources.H, Deposits: dep, Surveyed: surv}
	}
	// v22: the fog memory. Everything else about visibility is derived from the units,
	// so this is the only piece of the map's exploration a load cannot rebuild.
	if g.Vis != nil {
		s.Explored = &ExploredSnapshot{W: g.Vis.W, H: g.Vis.H, Teams: g.Vis.ExportExplored()}
	}
}

// captureWithCarried appends u's snapshot (linked to parentID, -1 if on the map)
// then recurses into u.Carried - the passengers a transport holds OFF the unit
// registry (LoadUnit removes them from g.Units), which the g.Units.Units() loop
// in Capture would otherwise miss, losing them on save.
func captureWithCarried(s *Snapshot, u *game.UnitInfo, parentID int32) {
	s.Units = append(s.Units, captureUnit(u, parentID))
	for _, p := range u.Carried {
		captureWithCarried(s, p, int32(u.ID))
	}
}

// captureUnit copies one unit's persisted fields. parentID is the carrier this
// unit is aboard (-1 if on the map).
func captureUnit(u *game.UnitInfo, parentID int32) UnitSnapshot {
	us := UnitSnapshot{
		ID: u.ID, Type: int(u.Type), Team: u.Team, Flags: uint32(u.Flags),
		GridX: u.GridX, GridY: u.GridY,
		Order: uint8(u.Order), State: uint8(u.State),
		PriorOrder: uint8(u.PriorOrder), PriorState: uint8(u.PriorState),
		Angle: u.Angle, TurretAngle: u.TurretAngle, TurretTarget: u.TurretTarget,
		Hits: u.Hits, Speed: u.Speed, Shots: u.Shots, Ammo: u.Ammo, Storage: u.Storage,
		RawMining: u.RawMining, FuelMining: u.FuelMining, GoldMining: u.GoldMining,
		RawMiningMax: u.RawMiningMax, FuelMiningMax: u.FuelMiningMax,
		GoldMiningMax: u.GoldMiningMax, TotalMining: u.TotalMining,
		ParentID: parentID, AttackTargetID: -1,
		HiddenUntilExpand: u.HiddenUntilExpand,
		AutoSurvey:        u.AutoSurvey,
		// v13:
		Experience:           u.Experience,
		DisabledReactionFire: u.DisabledReactionFire,
		LastBuildRate:        u.LastBuildRate,
		BridgeRaise:          u.BridgeRaise,
		SpottedBy:            u.SpottedBy, // v21: a mine the enemy already found stays found
	}
	if u.AttackTarget != nil {
		us.AttackTargetID = int32(u.AttackTarget.ID)
	}
	us.RepeatBuild = u.RepeatBuild
	us.BuildRate = u.BuildRate
	for _, t := range u.BuildQueue {
		us.BuildQueue = append(us.BuildQueue, int(t))
	}
	if u.Values != nil {
		for a := game.Attrib(0); a < game.AttribCount; a++ {
			us.Values[a] = u.Values.Get(a)
		}
		us.ValVersion = u.Values.Version()
	}
	return us
}

// Save writes a snapshot as a MAXGOSV file: the magic header, a little endian
// uint32 version, then the gob encoded snapshot.
func Save(w io.Writer, s *Snapshot) error {
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(Version)); err != nil {
		return err
	}
	if err := gob.NewEncoder(w).Encode(s); err != nil {
		return fmt.Errorf("saveload: encode: %w", err)
	}
	return nil
}

// Load reads a MAXGOSV snapshot and validates its header and schema version.
func Load(r io.Reader) (*Snapshot, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("saveload: read header: %w", err)
	}
	if hdr != magic {
		return nil, fmt.Errorf("saveload: not a MAXGOSV file (bad magic %q)", bytes.TrimRight(hdr[:], "\x00"))
	}
	var ver uint32
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return nil, fmt.Errorf("saveload: read version: %w", err)
	}
	// Gob zero-fills fields absent from older additive schemas. Reject only versions newer than this build understands.
	if ver > Version {
		return nil, fmt.Errorf("saveload: save version %d is newer than supported %d", ver, Version)
	}
	var s Snapshot
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return nil, fmt.Errorf("saveload: decode: %w", err)
	}
	return &s, nil
}
