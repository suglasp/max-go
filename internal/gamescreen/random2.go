// random2.go : provides the -random2 dev bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/world"
)

// random2.go builds the -emulate random2 test state: TWO fully developed bases
// (player team 0 + computer team 1) on turn 50 - multiple mining stations and
// heavy power stations, factories, defensive launchers, connectors / road /
// bridge, plus air and naval units. A mid game snapshot for exercising lots of
// unit and building types at once; you drop straight into the running game with
// no landing sequence.

// big2x2Buildings is the packed block of 2x2 buildings each base lays down
// (placed adjacent so they auto join into complexes): two mining stations + two
// heavy power stations + the light/heavy/air plants + support structures, plus the
// Eco Sphere + habitat (the colonisation pair). The Shipyard is NOT here: it is laid
// only for a COASTAL base (placeCoastalShipyard), so a landlocked base never wastes a
// dock it cannot use.
var big2x2Buildings = []assets.UnitType{
	assets.MiningSt, assets.PowerStn, assets.LandPlt,
	assets.LightPlt, assets.AirPlt, assets.MiningSt,
	assets.PowerStn, assets.Depot, assets.Research,
	assets.Habitat, assets.Greenhse,
}

// big1x1Buildings fill the base out: connectors tie the complexes together, plus
// a gold vault, dumps, an extra generator and the radar (a 1x1 building).
var big1x1Buildings = []assets.UnitType{
	assets.Cnct4W, assets.Cnct4W, assets.GoldSm, assets.ADump, assets.FDump, assets.PowGen, assets.Radar,
}

// the mobile force each base fields.
var (
	bigLandRoster = []assets.UnitType{
		assets.Tank, assets.Tank, assets.MisslLch, assets.RocktLch, assets.SpFlak,
		assets.Scout, assets.Scanner, assets.Surveyor, assets.Constrct, assets.Engineer, assets.SplyTrck,
	}

	bigAirRoster = []assets.UnitType{assets.Fighter, assets.Bomber, assets.Awac}
	bigSeaRoster = []assets.UnitType{assets.Corvette, assets.Battlshp, assets.SeaTrans}
)

// playerTestLand / playerTestSea are extra units added to the PLAYER base only,
// for exercising the on water animations: foot soldiers, two
// amphibious personnel carriers (drive them onto water to watch the land->boat
// sprite + water engine sound swap), and a submarine spawned in open water.
// The Mine Layer (land) + Sea Mine Layer (sea) also reveal resources at radius 1
// as they drive, like a Surveyor: toggle SURVEY on and
// drive them to watch the 3x3 reveal trail. A Commando is already present.
var (
	playerTestLand = []assets.UnitType{
		assets.Infantry, assets.Infantry, assets.Commando, // a few foot "human" units
		assets.ClnTrans, assets.ClnTrans, // amphibious personnel carriers
		assets.MineLayr, // lays land mines + surveys resources at radius 1
	}

	playerTestSea = []assets.UnitType{assets.Submarne, assets.SeaMnLyr}
)

// RandomStart2 deploys the mid game test state (turn 50) for the human plus npc
// computer opponents (1..3) and drops the camera into the player's base with control
// already in hand (no landing). Each computer team gets its own full base on its own
// landmass. C++: a stand in dev scenario; -emulate random2.
func (s *Screen) RandomStart2(seed int64, npc int) {
	rng := rand.New(rand.NewSource(seed))
	teams := npc + 1 // team 0 = human, 1..npc = computer opponents
	spot := s.chooseStartSpots(teams, rng)

	s.game.State.Teams[0].Type = game.TeamPlayer

	for team := 1; team < teams; team++ {
		s.game.State.Teams[team].Type = game.TeamComputer
	}

	if s.game.MiningDifficulty == 0 {
		s.game.MiningDifficulty = 4
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

	// -startcredit hands the human a landing gold reserve (the bench has no
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
		if u := s.buildBigBase(team, spot[team], rng); team == 0 {
			first = u
		}
	}

	// Reconcile every team's power so any building that ended up off the complex
	// reads idle from frame one (the bench drops the player straight into the turn).
	for team := 0; team < teams; team++ {
		s.game.ReconcilePower(team)
	}

	// Mid game stores so the bases read as established at turn 50.
	for _, c := range s.game.Complexes {
		c.Material += 300
		c.Fuel += 200
		c.Gold += 80
	}

	s.game.State.TurnCounter = 50
	s.selected = first
	s.game.State.ActiveTeam = s.game.State.FirstActiveTeam()
	s.game.State.PlayerTeam = s.game.State.ActiveTeam
	s.fogShow = true
	s.game.RefreshVisibility()
	s.centerOnTeam(0)
	s.turnFlow.turnStart = platform.Ticks() // control in hand at once: the turn clock runs
}

// buildBigBase lays one team's full base near center and returns its first
// mobile unit. The 2x2 buildings are packed in a grid so they share edges and
// form interconnected complexes; the mobile force clusters around them; air
// units sit overhead and boats on the nearest water (skipped if landlocked).
func (s *Screen) buildBigBase(team int, center game.Point, rng *rand.Rand) *game.UnitInfo {
	// A coastal base goes sea oriented (a wired shipyard + boats); a landlocked base
	// stays land only and leans on its Eco Sphere + habitat for the colonisation game.
	coastal := s.coastalBase(center)
	taken := s.placeBigBaseBuildings(team, center, coastal, rng)

	return s.deployBigBaseForce(team, center, coastal, taken, rng)
}

// placeBigBaseBuildings lays this team's buildings around center and returns the
// footprint map the deploy pass keeps clear.
func (s *Screen) placeBigBaseBuildings(team int, center game.Point, coastal bool, rng *rand.Rand) map[game.Point]bool {
	taken := map[game.Point]bool{}
	buildingFlags := game.FlagStationary | game.FlagBuilding | game.FlagRequiresSlab | game.FlagSelectable

	// Track the placed LAND footprints so every later building (and the
	// connectors) lands EDGE ADJACENT to the growing mass. M.A.X. buildings only
	// pool material/fuel/gold through a shared edge (or a connector chain), not
	// across a gap, so an adjacency connected base forms ONE complex: the mining
	// stations then feed every building and a supply truck parked next to any of
	// them can X-Fer reload. The old fixed grid placement fragmented the base on
	// irregular terrain, stranding complexes with no raw store - the truck next to
	// one of those showed the NO cursor. Mirrors deployStartBase.
	// C++: AttachToPrimaryComplex by adjacency.
	var placed []placedBuilding
	// placeConnected lays one building either hugging the existing mass (so it
	// joins the complex) or, for the first / a naval building, by an area search.
	placeConnected := func(t assets.UnitType, flags game.Flags, n int, water bool) {
		if game.CanHeadRotateType(t) {
			flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
		}

		d := access.UnitDesc{Type: t, Flags: flags, LandType: access.LandTypeForUnit(t, flags)}
		valid := func(x, y int) bool {
			return build.SiteValid(s.game.Grid, d, team, x, y) && !footprintTaken(taken, x, y, n)
		}

		var x, y int
		var ok bool

		switch {
		case water:
			// A naval building (the Shipyard) searches wide for the nearest coast; it
			// stands on its own complex (its water edges do not abut the land mass).
			x, y, ok = s.nearestCell(center, baseSearchRadius+28, valid)
		case len(placed) == 0:
			// The first land building anchors the base near the centre.
			x, y, ok = s.nearestCell(center, baseSearchRadius, valid)
		default:
			// Every later land building hugs the placed mass so the complex connects.
			if x, y, ok = adjacentSite(placed, n, valid); !ok {
				x, y, ok = s.nearestCell(center, baseSearchRadius, valid) // fall back to any valid site
			}
		}

		if !ok {
			return
		}

		s.placeStartBuilding(team, t, flags, x, y)
		markFootprint(taken, x, y, n)

		// Only land footprints seed the adjacency search (a water shipyard cannot
		// edge share with land buildings).
		if !water {
			placed = append(placed, placedBuilding{x: x, y: y, n: n})
		}
	}

	// The 2x2 buildings (mining stations, power stations, factories, support).
	for _, t := range big2x2Buildings {
		placeConnected(t, buildingFlags, 2, access.IsWaterBuilding(t))
	}

	// The 1x1 buildings (connectors / dumps / generator / radar) tie in beside the
	// mass - the "collectors" visible in game now genuinely bridge into the complex.
	// A terrain special (Cnct4W) lays NO slab, exactly as the player's build flow
	// (buildDesc) and the AI (buildorders.go buildFlagsFor) strip FlagRequiresSlab
	// for it - only real 1x1 buildings (dumps / generator / radar) keep the slab.
	smallFlags := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

	for _, t := range big1x1Buildings {
		flags := smallFlags

		if access.IsTerrainSpecial(t) {
			flags &^= game.FlagRequiresSlab
		}

		placeConnected(t, flags, 1, false)
	}

	// A coastal base adds the shipyard at the shore, wired into the complex with
	// connectors and reached by a span of bridge pieces.
	if coastal {
		s.placeCoastalShipyard(team, center, placed, taken, rng)
	}

	s.layBigRoads(team, center, taken)

	return taken
}

// deployBigBaseForce deploys this team's mobile force around the placed base and
// returns its first land unit.
func (s *Screen) deployBigBaseForce(team int, center game.Point, coastal bool, taken map[game.Point]bool, rng *rand.Rand) *game.UnitInfo {
	var first *game.UnitInfo
	constructorsSeen, engineersSeen := 0, 0

	for _, t := range bigLandRoster {
		flags := game.FlagMobileLandUnit | game.FlagSelectable

		if isTurretedLand(t) {
			flags |= game.FlagTurretSprite | game.FlagCanHeadRotate
		}

		if t == assets.Constrct || t == assets.Engineer {
			flags |= game.FlagConstructorUnit // so the popup offers Build (and the build flow works)
		}

		u := s.deployBigUnit(team, t, flags, center, baseSearchRadius, taken, rng)

		if u == nil {
			continue
		}

		// The founder builders deploy at the same partial default as -random.
		setStarterBuilderCargo(u, &constructorsSeen, &engineersSeen)

		if first == nil {
			first = u
		}
	}

	airFlags := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	for _, t := range bigAirRoster {
		s.deployBigUnit(team, t, airFlags, center, baseSearchRadius, taken, rng)
	}

	// Boats go in OPEN water (a full water tile clear of the shore), never on a
	// coast/shore tile a pure water unit cannot normally enter. A landlocked base
	// fields none (it has no water to float them on).
	if coastal {
		for _, t := range bigSeaRoster {
			s.deployBigSea(team, t, center, taken, rng)
		}
	}

	// Player only extras for testing the on water animations: foot soldiers, two
	// amphibious personnel carriers (deployed on land near the base - drive them
	// to water), and a submarine in open water (coastal bases only).
	if team == 0 {
		for _, t := range playerTestLand {
			flags := game.FlagMobileLandUnit | game.FlagSelectable
			s.deployBigUnit(team, t, flags, center, baseSearchRadius, taken, rng)
		}

		if coastal {
			for _, t := range playerTestSea {
				s.deployBigSea(team, t, center, taken, rng)
			}
		}
	}

	return first
}

// deployBigSea places one pure water unit in open water (a full water tile clear
// of the shore) within reach of center, marking the cell taken. Does nothing if the
// base is landlocked / no open water is in range.
func (s *Screen) deployBigSea(team int, t assets.UnitType, center game.Point, taken map[game.Point]bool, rng *rand.Rand) {
	seaFlags := game.FlagMobileSeaUnit | game.FlagSelectable
	d := access.UnitDesc{Type: t, Flags: seaFlags, LandType: access.LandTypeForUnit(t, seaFlags)}
	x, y, found := s.nearestCell(center, baseSearchRadius+28, func(x, y int) bool {
		return !taken[game.Point{X: x, Y: y}] && s.isOpenWater(x, y) &&
			s.game.Grid.IsAccessible(d, team, x, y, access.SameClassBlocks).Passable()
	})

	if !found {
		return
	}

	taken[game.Point{X: x, Y: y}] = true
	s.deployUnit(t, seaFlags, team, uint8(rng.Intn(game.AngleCount)), x, y)
}

// isOpenWater reports whether (x,y) is a full water tile clear of the shore: the
// cell and all eight neighbours are water (not land or coast), so a pure water
// unit sits in open sea rather than jammed one tile from land.
func (s *Screen) isOpenWater(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if s.wmap.SurfaceAt(x+dx, y+dy) != world.SurfaceWater {
				return false
			}
		}
	}

	return true
}

// deployBigUnit places one mobile unit on the nearest free, accessible cell
// within radius of center, marking the cell taken. Returns nil if none fits
// (e.g. a boat when the base is landlocked).
func (s *Screen) deployBigUnit(team int, t assets.UnitType, flags game.Flags, center game.Point, radius int, taken map[game.Point]bool, rng *rand.Rand) *game.UnitInfo {
	d := access.UnitDesc{Type: t, Flags: flags, LandType: access.LandTypeForUnit(t, flags)}
	x, y, found := s.nearestCell(center, radius, func(x, y int) bool {
		return !taken[game.Point{X: x, Y: y}] && s.game.Grid.IsAccessible(d, team, x, y, access.SameClassBlocks).Passable()
	})

	if !found {
		return nil
	}

	taken[game.Point{X: x, Y: y}] = true
	return s.deployUnit(t, flags, team, uint8(rng.Intn(game.AngleCount)), x, y)
}

// isTurretedLand reports whether a land unit has an independently rotating head.
func isTurretedLand(t assets.UnitType) bool {
	return game.CanHeadRotateType(t)
}

// coastalRadius is how near (in cells) open water must be for a base to count as
// coastal: within reach of the base centre (baseSearchRadius is 24), so a "coastal"
// base genuinely has shore to put a shipyard / boats on.
const coastalRadius = 14

// coastalBase reports whether the base centred at center has water nearby, so a
// shipyard and boats make sense. A landlocked centre (no water within coastalRadius)
// stays a pure land base.
func (s *Screen) coastalBase(center game.Point) bool {
	for y := center.Y - coastalRadius; y <= center.Y+coastalRadius; y++ {
		for x := center.X - coastalRadius; x <= center.X+coastalRadius; x++ {
			if s.wmap.SurfaceAt(x, y)&world.SurfaceWater != 0 {
				return true
			}
		}
	}

	return false
}

// layBigRoads lays a T shaped road feature beside the base (a horizontal bar with a
// vertical stem) to exercise road rendering and give the base a wired look. Roads are
// ground cover and bypass the complex/slab path. Bridges are NOT laid here any more -
// they belong to the coastal shipyard (placeCoastalShipyard), so a landlocked base
// gets no stray bridge.
func (s *Screen) layBigRoads(team int, center game.Point, taken map[game.Point]bool) {
	ry := center.Y + 8
	road := func(x, y int) {
		if x < 0 || y < 0 || x >= s.wmap.W || y >= s.wmap.H {
			return
		}

		if s.wmap.SurfaceAt(x, y)&world.SurfaceLand == 0 || taken[game.Point{X: x, Y: y}] {
			return
		}

		s.deployProp(team, assets.Road, game.FlagGroundCover, x, y)
	}

	for dx := -2; dx <= 2; dx++ { // the horizontal bar of the T
		road(center.X+dx, ry)
	}

	for dy := 1; dy <= 3; dy++ { // the vertical stem rising from the bar's centre
		road(center.X, ry-dy)
	}
}

// placeCoastalShipyard sites a Shipyard at the shore near the base, lays a span of
// bridge pieces over the adjacent water, and (when the dock could not be sited
// edge adjacent to the base mass) runs a short connector chain back to it so it joins
// the complex and powers up. Does nothing if no coastal shipyard site fits. C++: a coastal
// base's naval yard wired into the primary complex.
func (s *Screen) placeCoastalShipyard(team int, center game.Point, placed []placedBuilding, taken map[game.Point]bool, rng *rand.Rand) {
	shipFlags := game.FlagStationary | game.FlagBuilding | game.FlagRequiresSlab | game.FlagSelectable
	d := access.UnitDesc{Type: assets.Shipyard, Flags: shipFlags, LandType: access.LandTypeForUnit(assets.Shipyard, shipFlags)}
	valid := func(x, y int) bool {
		return build.SiteValid(s.game.Grid, d, team, x, y) && !footprintTaken(taken, x, y, 2)
	}

	// Prefer a coastal site that edge touches the base mass, so the shipyard joins the
	// complex directly (and powers). Otherwise take the nearest coastal site and chain
	// connectors back to the base.
	sx, sy, ok := adjacentSite(placed, 2, valid)
	chain := false

	if !ok {
		sx, sy, ok = s.nearestCell(center, baseSearchRadius+16, valid)
		chain = true
	}

	if !ok {
		return // no coastal site fits: leave the base land only
	}

	s.placeStartBuilding(team, assets.Shipyard, shipFlags, sx, sy)
	markFootprint(taken, sx, sy, 2)
	s.layShipyardBridges(team, sx, sy, taken)

	if chain {
		s.layConnectorChain(team, sx, sy, placed, taken)
	}
}

// layShipyardBridges lays up to bridgeSpan bridge pieces on the water cells around the
// shipyard's seaward edge (multiple pieces, not one) so the shore by the dock reads as
// spanned. Bridges are 1x1 props that bypass the complex/slab path.
func (s *Screen) layShipyardBridges(team, sx, sy int, taken map[game.Point]bool) {
	const bridgeSpan = 5
	laid := 0

	for r := 1; r <= 3 && laid < bridgeSpan; r++ {
		for dy := -r; dy <= r+1 && laid < bridgeSpan; dy++ {
			for dx := -r; dx <= r+1 && laid < bridgeSpan; dx++ {
				x, y := sx+dx, sy+dy

				if x < 0 || y < 0 || x >= s.wmap.W || y >= s.wmap.H {
					continue
				}

				p := game.Point{X: x, Y: y}

				if taken[p] || s.wmap.SurfaceAt(x, y)&world.SurfaceWater == 0 {
					continue
				}

				s.deployProp(team, assets.Bridge, game.FlagStationary, x, y)
				taken[p] = true
				laid++
			}
		}
	}
}

// layConnectorChain wires a detached shipyard at (sx,sy) into the base complex by
// laying a CONTINUOUS chain of Cnct4W connectors from the dock's landward edge to a
// cell touching the base mass. It BFS searches over FREE LAND cells (the only cells a
// connector can stand on) so the path never crosses water or an occupied cell: every
// connector edge shares with the next, the first edge shares with the shipyard and the
// last with a base building, so the whole run conducts and the dock powers up. A
// a straight L-shaped walk can clip a water inlet on irregular coastlines and leave a gap
// that silently broke the link; the BFS closes it whenever any land route exists.
func (s *Screen) layConnectorChain(team, sx, sy int, placed []placedBuilding, taken map[game.Point]bool) {
	if len(placed) == 0 {
		return
	}

	// The base footprint cells, so a path cell "reaches the base" when it edge touches one.
	baseCells := map[game.Point]bool{}

	for _, p := range placed {
		for dy := 0; dy < p.n; dy++ {
			for dx := 0; dx < p.n; dx++ {
				baseCells[game.Point{X: p.x + dx, Y: p.y + dy}] = true
			}
		}
	}

	orth := [4]game.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	touchesBase := func(x, y int) bool {
		for _, o := range orth {
			if baseCells[game.Point{X: x + o.X, Y: y + o.Y}] {
				return true
			}
		}

		return false
	}

	// A connector may stand on an in bounds, free, land cell.
	layable := func(x, y int) bool {
		if x < 0 || y < 0 || x >= s.wmap.W || y >= s.wmap.H {
			return false
		}

		return !taken[game.Point{X: x, Y: y}] && s.wmap.SurfaceAt(x, y)&world.SurfaceLand != 0
	}

	// Seed the BFS with the free land cells edge adjacent to the 2x2 shipyard footprint.
	prev := map[game.Point]game.Point{}
	seen := map[game.Point]bool{}
	var queue []game.Point

	for fy := sy; fy < sy+2; fy++ {
		for fx := sx; fx < sx+2; fx++ {
			for _, o := range orth {
				n := game.Point{X: fx + o.X, Y: fy + o.Y}

				if !seen[n] && layable(n.X, n.Y) {
					seen[n] = true
					prev[n] = game.Point{X: -1, Y: -1} // a root cell (adjacent to the dock)
					queue = append(queue, n)
				}
			}
		}
	}

	// BFS outward over free land until a cell edge touches the base mass.
	goal := game.Point{X: -1, Y: -1}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if touchesBase(cur.X, cur.Y) {
			goal = cur
			break
		}

		for _, o := range orth {
			n := game.Point{X: cur.X + o.X, Y: cur.Y + o.Y}

			if !seen[n] && layable(n.X, n.Y) {
				seen[n] = true
				prev[n] = cur
				queue = append(queue, n)
			}
		}
	}

	if goal.X < 0 {
		return // no free land route to the base: leave the dock land only (best effort)
	}

	// Lay a connector on every cell of the found path (dock adjacent root .. base touching goal).
	connFlags := game.FlagStationary | game.FlagSelectable // a connector lays no slab

	for c := goal; c.X >= 0; c = prev[c] {
		s.placeStartBuilding(team, assets.Cnct4W, connFlags, c.X, c.Y)
		taken[c] = true
	}
}

// deployProp deploys a prop that is not part of a complex (road tile, bridge) straight into the
// unit registry so it renders, without the building/slab/complex path.
func (s *Screen) deployProp(team int, t assets.UnitType, flags game.Flags, x, y int) {
	u := game.NewUnitInfo(t, team, 20000+s.spawnSeq+1, 0)
	u.Flags = flags
	s.spawnSeq++
	s.game.Units.Deploy(u, x, y)
}
