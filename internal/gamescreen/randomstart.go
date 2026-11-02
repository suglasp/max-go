// randomstart.go : provides the -random dev bench startup.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"
	"sort"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/landmass"
	"github.com/suglasp/max-go/internal/world"
)

// unitSpec is one entry in a team's starting line up: a unit type and the
// movement/role flags it deploys with.
type unitSpec struct {
	t     assets.UnitType
	flags game.Flags
}

const (
	flagsLand       = game.FlagMobileLandUnit | game.FlagSelectable
	flagsHeadRotate = flagsLand | game.FlagTurretSprite | game.FlagCanHeadRotate
)

// baseRoster is the minimal mobile line up every team starts with, in any game
// mode: a constructor (large 2x2 buildings) and an engineer (small 1x1
// buildings) to found a base, plus a tank and scout to defend and explore.
var baseRoster = []unitSpec{
	{assets.Constrct, flagsLand | game.FlagConstructorUnit},
	{assets.Engineer, flagsLand | game.FlagConstructorUnit},
	{assets.Tank, flagsHeadRotate},
	{assets.Scout, flagsLand},
}

// randomExtraRoster is added on top of baseRoster for a -random quick start, so
// each team lands with a combat ready force: recon (scanner + surveyor), the
// rocket and missile launchers, a mobile anti aircraft unit and two more tanks.
var randomExtraRoster = []unitSpec{
	{assets.Scanner, flagsLand},
	{assets.Surveyor, flagsLand},
	{assets.RocktLch, flagsLand},
	{assets.SpFlak, flagsLand}, // self propelled anti aircraft
	{assets.MisslLch, flagsLand},
	{assets.Tank, flagsHeadRotate},
	{assets.Tank, flagsHeadRotate},
}

// startBuildings is the minimal base every team begins with: a mining station
// (the resource source) and a power generator to run it. C++: the standard
// mission setup's starting structures.
var startBuildings = []unitSpec{
	{assets.MiningSt, game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable | game.FlagBuilding}, // 2x2
	{assets.PowGen, game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable},                       // 1x1
}

// startSupplyRaw / startSupplyFuel are the materials a freshly landed base begins
// with. The raw is a small pool buffer that also seeds the mining station's turn 0
// cargo bar; the fuel keeps the generator running until the mine's own fuel income
// takes over (it would otherwise brown out on turn one). startSupplyRaw is the
// AVERAGE richness figure, which is what the -random / -random2 benches land on
// (they carry no resource level); a custom game reads its own level through
// startSupplyRawFor.
const (
	startSupplyRaw  = startSupplyRawAverage
	startSupplyFuel = 30
)

// The raw a freshly landed base pools, per the RAW resource richness the player chose
// on the setup screen. These are the turn 0 readings observed in vanilla on
// the mining station's cargo bar (capacity 25): a poor map lands 10, an average one 13,
// and a rich one lands NOTHING - the deposits are so good that the drop ships bring no
// buffer at all.
const (
	startSupplyRawPoor    = 10
	startSupplyRawAverage = 13
	startSupplyRawRich    = 0
)

// startSupplyRawFor returns the landing raw buffer for a raw resource level (0 poor,
// 1 average, 2 rich - economy.ResourceParamsForLevels' scale). An unknown level lands
// the average buffer, which is what a game with no level set (the benches) gets.
func startSupplyRawFor(rawLevel int) int {
	switch rawLevel {
	case 0:
		return startSupplyRawPoor

	case 2:
		return startSupplyRawRich

	default:
		return startSupplyRawAverage
	}
}

// startConstructorRaw / startEngineerRaw are the DEFAULT raw cargo the first
// Constructor (max 60) and first Engineer (max 40) deploy with, a partial load,
// never full, fixed regardless of the resource richness setting. Any extra
// builder starts empty (the player tops it up from the custom game starting
// credit bar). The purchase screen seeds the same amounts, so a custom game
// lands its builders loaded exactly like a random one.
const (
	startConstructorRaw = economy.StartConstructorCargo
	startEngineerRaw    = economy.StartEngineerCargo
)

// startBuildRoomPercent is the build room a team needs on its island, as a share of the map's land tiles,
// so a map with more land asks for a bigger base site.
const startBuildRoomPercent = 5

// landingTiles counts the tiles the landing force covers: the start buildings' footprints plus one tile per unit
// of the largest random roster.
func landingTiles() int {
	tiles := len(baseRoster) + len(randomExtraRoster)
	for _, b := range startBuildings {
		tiles += build.Footprint(b.flags) * build.Footprint(b.flags)
	}
	return tiles
}

// startRoomNeeded returns the land tiles a team's island must hold: the landing force plus startBuildRoomPercent
// of the map's land (mapLand).
func startRoomNeeded(mapLand int) int {
	return landingTiles() + mapLand*startBuildRoomPercent/100
}

// islandLand counts an island's land tiles; coast tiles do not count as room to build.
func (s *Screen) islandLand(c landmass.Component) int {
	land := 0
	for _, p := range c.Cells {
		if s.wmap.SurfaceAt(p.X, p.Y)&world.SurfaceLand != 0 {
			land++
		}
	}
	return land
}

// islandsWithRoom keeps the islands holding the land a team needs on this map, largest first.
func (s *Screen) islandsWithRoom(comps []landmass.Component) []landmass.Component {
	lands := make([]int, len(comps))
	mapLand := 0
	for i, c := range comps {
		lands[i] = s.islandLand(c)
		mapLand += lands[i]
	}
	need := startRoomNeeded(mapLand)
	var out []landmass.Component
	for i, c := range comps {
		if lands[i] >= need {
			out = append(out, c)
		}
	}
	return out
}

// baseSearchRadius bounds how far deployStartBase looks outward from the chosen
// landing spot for a valid building site or unit cell.
const baseSearchRadius = 24

// RandomStart drops the human plus npc computer opponents (1..3) onto the loaded
// map and centres the view on the human: the -random / -rng quick start. It picks a
// distinct landing spot per team and lets deployStartBase lay out a full base whose
// roster adapts to the terrain (a coastal team fields a small navy + a shipyard).
//
// C++: a stand in for the scenario setup that seeds opposing teams.
func (s *Screen) RandomStart(seed int64, npc int) {
	rng := rand.New(rand.NewSource(seed))
	teams := npc + 1 // team 0 = human, 1..npc = computer opponents
	spot := s.chooseStartSpots(teams, rng)

	// A single player skirmish: team 0 is the human; the rest are computer teams (so
	// ending the turn does not stop for a pass the keyboard banner). C++: the mission
	// setup assigning TEAM_TYPE_COMPUTER to the opposing slots.
	s.game.State.Teams[0].Type = game.TeamPlayer

	for team := 1; team < teams; team++ {
		s.game.State.Teams[team].Type = game.TeamComputer
	}

	// -random uses a predefined landing spot (no interactive site selection, that
	// is only for a custom game - customstart.go runs this same sequence as the
	// site select callback), so scatter resources around every final landing
	// spot, land each team's base, supply them, and start the descent straight
	// away. C++: a mission that is not custom skips GameManager_SelectSite.
	if s.game.MiningDifficulty == 0 {
		s.game.MiningDifficulty = 4 // human normal mining scale
	}

	s.game.Resources = economy.NewResourceMap(s.wmap.W, s.wmap.H)
	if s.resourceLevelsSet {
		// An explicit -raw/-fuel/-gold seeds the bench map at the flag richness
		// (the step 1284 ruling); without the flags the default allocator params
		// apply as always. Same branch as CustomStart's deployBases.
		params := economy.ResourceParamsForLevels(s.resourceLevels[0], s.resourceLevels[1], s.resourceLevels[2])
		economy.AllocateResourcesWith(s.game.Resources, s.wmap.W, s.wmap.H, spot, game.NewRandomizer(uint32(seed)), params)
	} else {
		economy.AllocateResources(s.game.Resources, s.wmap.W, s.wmap.H, spot, game.NewRandomizer(uint32(seed)))
	}

	// -startcredit hands the human a landing gold reserve (the benches have no
	// purchase screen; the in game Upgrade menu draws on it). Unset = none.
	if s.startCreditSet {
		s.game.Gold[0] = s.startCredit
	}

	// Scatter the neutral alien derelict faction, density per the -derelicts flag
	// (None = inert). Sites stay clear of every team's landing spot.
	sep, val := alienDensity(s.alienDerelicts)
	s.populateAlienDerelicts(sep, val, spot, game.NewRandomizer(uint32(seed)))

	// Rare hidden gag prop (Waldo / Harvster) on a clear land tile, independent of
	// the derelict density. A distinct seed keeps it reproducible but uncorrelated.
	s.spawnEasterEgg(spot, game.NewRandomizer(uint32(seed)^easterEggSeedSalt))

	// Resolve each team's clan (computer teams strategy flavored) before deploy
	// so every team fields a varied clan with its stat bonuses.
	s.resolveTeamClans(game.NewRandomizer(uint32(seed)))

	var first *game.UnitInfo

	for team := 0; team < teams; team++ {
		// -random lands a MINIMAL, randomized LAND force per team (no shipyard or
		// navy): the three founders + a Scanner always, then an RNG combat/support
		// tail (see randomRoster). coastal=false so deployStartBase skips the shore
		// shipyard. C++: a stand in for the scenario setup's per team units.
		roster := randomRoster(rng)

		if u := s.deployStartBase(team, spot[team], roster, nil, false, rng); team == 0 {
			first = u
		}
	}

	s.debugCheckDeployOverlap() // catch a real same cell mobile stack on the live map (debug only)

	// Land each base with a small raw buffer + fuel, then seed the mining
	// station's turn 0 cargo bar from that buffer (the economy's redistribute
	// runs only from turn one, so mirror it once here). The mine reads the
	// MEDIUM default 13/25 and ramps toward its 25 cap as it produces; the
	// fuel keeps the generator burning until the mine's own fuel income covers
	// it (it would otherwise brown out on turn one).
	for _, c := range s.game.Complexes {
		c.Material += startSupplyRaw
		c.Fuel += startSupplyFuel
		seedRawStoreDisplay(c)
	}

	s.selected = first
	s.game.State.ActiveTeam = s.game.State.FirstActiveTeam()
	s.game.State.PlayerTeam = s.game.State.ActiveTeam
	// A -random quick start begins a fresh game, so reset the turn counter to 0
	// (NewState defaults it to 1; the AI test benches seed a mid game count).
	s.game.State.TurnCounter = 0
	s.fogShow = true
	// The landing turn is a turn: advance the player's economy and construction once now,
	// like every later turn start (and like MissionStart). C++: GameManager_ManageEconomy.
	s.game.BeginTeamTurn(s.game.State.PlayerTeam)
	// Land at one step closer than the default (16 -> 32) via the descent
	// animation: the camera zooms in from orbit while the base unfolds.
	s.StartLanding(s.r.Zoom() * 2)
}

// randomRoster builds a -random team's MINIMAL, randomized LAND force. Every team
// always lands with the three founders (Constructor, Engineer, Surveyor) plus a
// Scanner; on top of that it rolls a variable tail so a start ranges from bare bones
// to a small combat force: up to two Scouts, then 1-2 Tanks (or none) and each of a
// Supply Truck (cargo) / an Assault Gun / an SP Flak (anti aircraft) / a Mine
// Layer on an independent coin flip. No shipyard or navy - a -random base is land only.
// C++: a stand in for the scenario setup's per team starting units.
func randomRoster(rng *rand.Rand) []unitSpec {
	roster := []unitSpec{
		{assets.Constrct, flagsLand | game.FlagConstructorUnit},
		{assets.Engineer, flagsLand | game.FlagConstructorUnit},
		{assets.Surveyor, flagsLand},
		{assets.Scanner, flagsLand},
	}

	for n := rng.Intn(3); n > 0; n-- { // maybe up to two scouts
		roster = append(roster, unitSpec{assets.Scout, flagsLand})
	}

	if rng.Intn(2) == 0 { // 1 or 2 tanks, sometimes none

		for n := 1 + rng.Intn(2); n > 0; n-- {
			roster = append(roster, unitSpec{assets.Tank, flagsHeadRotate})
		}
	}

	// Each support/combat extra on an independent coin flip, so the tail varies from
	// nothing to the full set (deployUnit adds turret flags for the head rotaters).
	for _, u := range []unitSpec{
		{assets.SplyTrck, flagsLand}, // cargo truck
		{assets.Artillry, flagsLand}, // Assault Gun
		{assets.SpFlak, flagsLand},   // mobile anti aircraft
		{assets.MineLayr, flagsLand}, // mine layer
	} {
		if rng.Intn(2) == 0 {
			roster = append(roster, u)
		}
	}

	return roster
}

// chooseStartSpots picks k distinct team landing spots: one per island with room for a base
// when the map has at least k of them, otherwise it splits those islands' land (or all land
// when none has the room) into k bands so the teams still start apart. C++: the scenario
// setup's per team start position pick.
func (s *Screen) chooseStartSpots(k int, rng *rand.Rand) []game.Point {
	spots := make([]game.Point, k)
	islands := s.islandsWithRoom(landmass.Find(s.wmap))

	if len(islands) >= k {
		perm := rng.Perm(len(islands)) // k distinct islands, chosen at random

		for i := 0; i < k; i++ {
			spots[i] = s.landingSpot(islands[perm[i]].Cells, rng)
		}

		return spots
	}

	// Not enough distinct islands: split the land of the roomy islands into k bands so teams
	// still start apart and never on an islet (covers a single continent); all land when none has the room.
	cells := roomyCells(islands)
	if len(cells) == 0 {
		cells = s.landCells()
	}

	if len(cells) == 0 {
		return spots // degenerate: no land at all
	}

	for i := 0; i < k; i++ {
		lo, hi := i*len(cells)/k, (i+1)*len(cells)/k

		if lo >= hi {
			hi = lo + 1
		}

		if hi > len(cells) {
			hi = len(cells)
		}

		spots[i] = s.landingSpot(cells[lo:hi], rng)
	}

	return spots
}

// landingMargin is the clearance radius landingSpot prefers around the landing:
// a cell whose landMargin x landMargin block is all land lands the base well
// inland rather than at a sea edge.
const landingMargin = 3

// landingSpot picks a landing cell from a landmass that has the most land
// clearance around it, so the base does not start jammed against the coast or on
// the lip of a small island. It scores each cell by the land in its
// landingMargin neighbourhood and picks at random among the best scoring ones,
// so the spot stays varied across a large interior.
func (s *Screen) landingSpot(cells []game.Point, rng *rand.Rand) game.Point {
	if len(cells) == 0 {
		return game.Point{}
	}

	best := make([]game.Point, 0, len(cells))
	bestScore := -1

	for _, c := range cells {
		score := s.landClearance(c.X, c.Y, landingMargin)

		switch {
		case score > bestScore:
			bestScore, best = score, append(best[:0], c)
		case score == bestScore:
			best = append(best, c)
		}
	}

	return best[rng.Intn(len(best))]
}

// landClearance counts the land cells in the (2r+1) square around (cellX,cellY);
// off map and water cells count as zero, so coastal and map edge spots score low.
func (s *Screen) landClearance(cellX, cellY, r int) int {
	land := 0

	for y := cellY - r; y <= cellY+r; y++ {
		for x := cellX - r; x <= cellX+r; x++ {
			if s.wmap.SurfaceAt(x, y)&world.SurfaceLand != 0 {
				land++
			}
		}
	}

	return land
}

// deployStartBase lays a team's opening position near center: the minimal base
// buildings (mining station + power generator), then the given mobile roster
// clustered on the nearest passable cells. It returns the team's first mobile
// unit so the caller can make it the selection.
//
// cargos is the per roster unit cargo of a PURCHASED loadout (index aligned with
// roster), so a custom game lands the load the player set on the purchase screen.
// A nil cargos means no purchase screen ran: the founder builders take their
// default cargo instead.
//
// Every cell is vetted through the same Access_IsAccessible test the in game
// move uses, so a unit never lands on water or on an impassable prop (a cell
// it could move off but not onto).
func (s *Screen) deployStartBase(team int, center game.Point, roster []unitSpec, cargos []int, coastal bool, rng *rand.Rand) *game.UnitInfo {
	taken := map[game.Point]bool{}

	// A computer team buys its upgrades first: every unit below is minted at the team's
	// values, so an upgrade bought after the landing would miss the landing force.
	// C++ ported : aiplayer.cpp - ChooseInitialUpgrades, run while the supplies are rolled.
	s.applyAIStartUpgrades(team, roster)

	// Buildings first, so the units cluster around the finished base. The mining
	// station anchors the base near the centre; each later building (the power
	// generator) is placed orthogonally adjacent to an already placed footprint
	// so the complex auto connects, M.A.X. buildings only wire up for
	// fuel/material/gold through a shared edge (or a connector), not across a
	// gap or a mere diagonal touch. C++: AttachToPrimaryComplex by adjacency.
	var placed []placedBuilding

	for i, b := range startBuildings {
		d := access.UnitDesc{Type: b.t, Flags: b.flags, LandType: access.LandTypeForUnit(b.t, b.flags)}
		n := build.Footprint(b.flags)
		valid := func(x, y int) bool {
			return build.SiteValid(s.game.Grid, d, team, x, y) && !footprintTaken(taken, x, y, n)
		}

		var x, y int
		var ok bool

		if i == 0 {
			x, y, ok = s.nearestCell(center, baseSearchRadius, valid)
		} else if x, y, ok = adjacentSite(placed, n, valid); !ok {
			x, y, ok = s.nearestCell(center, baseSearchRadius, valid) // fall back to any valid site
		}

		if !ok {
			continue
		}

		s.placeStartBuilding(team, b.t, b.flags, x, y)
		markFootprint(taken, x, y, n)
		placed = append(placed, placedBuilding{x: x, y: y, n: n})
	}

	// A coastal team gets a shipyard at the shore, wired into the base with
	// connectors and reached by a span of bridge pieces (so it powers up and the
	// shore reads as developed). A landlocked base skips it.
	if coastal {
		s.placeCoastalShipyard(team, center, placed, taken, rng)
	}

	// Mobile units on the nearest passable, unoccupied cells.
	var first, firstBuilder *game.UnitInfo
	constructorsSeen, engineersSeen := 0, 0

	for i, def := range roster {
		d := access.UnitDesc{Type: def.t, Flags: def.flags, LandType: access.LandTypeForUnit(def.t, def.flags)}
		x, y, ok := s.nearestCell(center, baseSearchRadius, func(x, y int) bool {
			p := game.Point{X: x, Y: y}
			return !taken[p] && s.game.Grid.IsAccessible(d, team, x, y, access.SameClassBlocks).Passable()
		})

		if !ok {
			continue
		}

		taken[game.Point{X: x, Y: y}] = true
		u := s.deployUnit(def.t, def.flags, team, uint8(rng.Intn(game.AngleCount)), x, y)

		if cargos != nil {
			// A purchased loadout lands with the cargo the player loaded on the
			// purchase screen, whatever it is: what that screen showed is what lands.
			setPurchasedCargo(u, cargoAt(cargos, i))
		} else {
			// Give a STARTING builder its founder cargo (the first constructor / engineer
			// a partial default, extras empty), overriding deployUnit's full fill.
			setStarterBuilderCargo(u, &constructorsSeen, &engineersSeen)
		}

		if i == 0 {
			first = u
		}

		// Remember the first builder (constructor / engineer) so map entry preselects it.
		if firstBuilder == nil && (u.Type == assets.Constrct || u.Type == assets.Engineer) {
			firstBuilder = u
		}
	}

	// Preselect the first builder on map entry (the unit the player most likely acts with
	// first); fall back to the first deployed unit if the roster carries no builder.
	if firstBuilder != nil {
		return firstBuilder
	}

	return first
}

// setStarterBuilderCargo gives a freshly deployed STARTING builder its default
// raw cargo: the first Constructor startConstructorRaw (40/60), the first
// Engineer startEngineerRaw (20/40), any later builder 0 (the player tops extra
// builders up from the custom game starting credit). Overrides deployUnit's
// "constructor starts full" for the opening base only, so the founders deploy at
// the vanilla partial fill regardless of resource richness.
func setStarterBuilderCargo(u *game.UnitInfo, constructorsSeen, engineersSeen *int) {
	// Only the constructor / engineer carry founder cargo; other units are skipped.
	switch u.Type {
	case assets.Constrct:
		// The first constructor gets the default; any extra one starts empty.
		*constructorsSeen++

		if *constructorsSeen == 1 {
			u.Storage = startConstructorRaw
		} else {
			u.Storage = 0
		}
	case assets.Engineer:
		// The first engineer gets the default; any extra one starts empty.
		*engineersSeen++

		if *engineersSeen == 1 {
			u.Storage = startEngineerRaw
		} else {
			u.Storage = 0
		}
	}
}

// cargoAt returns the purchased cargo at index i, or 0 when the loadout carries
// no entry for it (a roster longer than the cargo list lands its tail empty).
func cargoAt(cargos []int, i int) int {
	if i < 0 || i >= len(cargos) {
		return 0
	}

	return cargos[i]
}

// setPurchasedCargo loads a unit deployed from a PURCHASED loadout with the cargo
// the player put on it on the purchase screen, clamped to the unit's own storage
// capacity. It replaces both deployUnit's "constructor starts full" fill and the
// founder default, so a custom game deploys exactly what the purchase screen showed.
func setPurchasedCargo(u *game.UnitInfo, cargo int) {
	capacity := u.Values.Get(game.AttribStorage)

	if cargo > capacity {
		cargo = capacity
	}

	if cargo < 0 {
		cargo = 0
	}

	u.Storage = cargo
}

// seedRawStoreDisplay mirrors a complex's pooled raw into its raw cargo members'
// display bars (mining station / material dump) so the cargo gauge reads a
// partial start stock on turn 0, before the first BalanceTurn redistribute runs.
// Fills each raw store in member order up to its capacity. C++: the scenario
// setup's Cargo_UpdateResourceLevels mirror of the pool into the stores.
func seedRawStoreDisplay(c *economy.Complex) {
	raw := c.Material

	// Fill each raw cargo store (mine / dump) up to its capacity, in member order.
	for _, u := range c.Members {
		if economy.CargoTypeOf(u.Type) != economy.CargoRaw {
			continue // only raw stores display raw material
		}

		capacity := 0

		if u.Values != nil {
			capacity = u.Values.Get(game.AttribStorage)
		}

		switch {
		case raw <= 0 || capacity <= 0:
			u.Storage = 0 // nothing left to show (or no capacity)
		case raw >= capacity:
			u.Storage = capacity // fill this store and carry the remainder on
			raw -= capacity
		default:
			u.Storage = raw // the last partial store takes what is left
			raw = 0
		}
	}
}

// nearestCell expands outward from center in growing square rings and returns
// the first cell (in bounds) satisfying ok, or false if none within maxR.
func (s *Screen) nearestCell(center game.Point, maxR int, ok func(x, y int) bool) (int, int, bool) {
	for r := 0; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if r > 0 && dx > -r && dx < r && dy > -r && dy < r {
					continue // the smaller-radius scan covers the interior; only the ring edge is new
				}

				x, y := center.X+dx, center.Y+dy

				if x < 0 || y < 0 || x >= s.wmap.W || y >= s.wmap.H {
					continue
				}

				if ok(x, y) {
					return x, y, true
				}
			}
		}
	}

	return 0, 0, false
}

// debugCheckDeployOverlap reports (debug builds only) when two ground or sea mobile
// units share a cell after a start deploy. Every deploy path guards this with a per
// team taken map and the grid same class block, so a hit here is a genuine bug: it
// prints the exact types, teams and cell so the offending path can be traced. Air
// units are skipped: a hovering aircraft legitimately sits over a ground unit.
func (s *Screen) debugCheckDeployOverlap() {
	if !dbg.Enabled() {
		return
	}

	seen := map[game.Point]*game.UnitInfo{}

	for _, u := range s.game.Units.Units() {
		// Only ground and sea movers claim a cell exclusively; a hovering air unit does not.
		if !u.Flags.Has(game.FlagMobileLandUnit) && !u.Flags.Has(game.FlagMobileSeaUnit) {
			continue
		}

		if u.Flags.Has(game.FlagMobileAirUnit) && u.Flags.Has(game.FlagHovering) {
			continue
		}

		p := game.Point{X: u.GridX, Y: u.GridY}

		// A cell already claimed by another mobile unit is the reported overlap.
		if other, ok := seen[p]; ok {
			dbg.Printf("DEPLOY-OVERLAP: %s (team %d) and %s (team %d) share cell (%d,%d)",
				displayName(other.Type), other.Team, displayName(u.Type), u.Team, p.X, p.Y)
			continue
		}

		seen[p] = u
	}
}

// placeStartBuilding deploys a finished building for an explicit team: a game
// start lands its structures complete, unlike the in game constructor build
// loop. C++: the scenario setup placing prebuilt team structures.
func (s *Screen) placeStartBuilding(team int, t assets.UnitType, flags game.Flags, cellX, cellY int) {
	u := game.NewUnitInfo(t, team, s.spawnSeq+1, 0)
	u.Flags = flags
	u.Values = s.teamValues(team, t) // base ATTRIBS + team research
	u.InitState()                    // Hits etc. from the values so the stat window has something to show
	// Start operational so the economy runs from turn one: a powered generator
	// supplies the mining station, which then produces. A unit factory is the
	// exception: it starts powered off like one built in play, because it only
	// runs while it has something queued.
	u.Order = game.OrderPowerOn
	if isFactory(t) {
		u.Order = game.OrderPowerOff
	}
	u.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(u, cellX, cellY)
}

// placedBuilding records an already laid building's footprint (top left + size)
// so the next building can be placed sharing an edge with it.
type placedBuilding struct{ x, y, n int }

// adjacentSite finds a top left position for an nxn building that shares an edge
// with one of the already placed footprints (so the complex auto connects) and
// passes valid. Returns false when no edge adjacent valid site exists.
func adjacentSite(placed []placedBuilding, n int, valid func(x, y int) bool) (int, int, bool) {
	for _, p := range placed {
		for y := p.y - n; y <= p.y+p.n; y++ {
			for x := p.x - n; x <= p.x+p.n; x++ {
				if footprintsEdgeAdjacent(x, y, n, p.x, p.y, p.n) && valid(x, y) {
					return x, y, true
				}
			}
		}
	}

	return 0, 0, false
}

// footprintsEdgeAdjacent reports whether an a sized block at (ax,ay) and a
// b sized block at (bx,by) touch along a full edge (4-connected) without
// overlapping: the condition under which two buildings join one complex.
func footprintsEdgeAdjacent(ax, ay, an, bx, by, bn int) bool {
	xOverlap := ax < bx+bn && bx < ax+an
	yOverlap := ay < by+bn && by < ay+an
	horizTouch := (ax+an == bx || bx+bn == ax) && yOverlap
	vertTouch := (ay+an == by || by+bn == ay) && xOverlap
	return horizTouch || vertTouch
}

// footprintTaken reports whether any of the nxn cells at (x,y) are already used.
func footprintTaken(taken map[game.Point]bool, x, y, n int) bool {
	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			if taken[game.Point{X: x + dx, Y: y + dy}] {
				return true
			}
		}
	}

	return false
}

// markFootprint records the nxn cells at (x,y) as used.
func markFootprint(taken map[game.Point]bool, x, y, n int) {
	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			taken[game.Point{X: x + dx, Y: y + dy}] = true
		}
	}
}

// landCells returns every land cell of the map as grid points.
func (s *Screen) landCells() []game.Point {
	var cells []game.Point

	for y := 0; y < s.wmap.H; y++ {
		for x := 0; x < s.wmap.W; x++ {
			if s.wmap.SurfaceAt(x, y)&(world.SurfaceLand|world.SurfaceCoast) != 0 {
				cells = append(cells, game.Point{X: x, Y: y})
			}
		}
	}

	return cells
}

// roomyCells returns every cell of the given islands in row order (top to bottom, then left to right),
// so splitting the list into bands splits the land spatially, as landCells does.
func roomyCells(islands []landmass.Component) []game.Point {
	var cells []game.Point
	for _, c := range islands {
		cells = append(cells, c.Cells...)
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Y != cells[j].Y {
			return cells[i].Y < cells[j].Y
		}
		return cells[i].X < cells[j].X
	})
	return cells
}

// centerOnTeam scrolls the view to the first unit of a team, if it has one.
func (s *Screen) centerOnTeam(team int) {
	for _, u := range s.game.Units.Units() {
		if u.Team == team {
			vw, vh := viewSize(MainMap)
			s.camera.scrollX, s.camera.scrollY = centerScroll(u.GridX, u.GridY, s.r.Zoom(), vw, vh)
			return
		}
	}
}
