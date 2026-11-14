// customstart.go : implements the custom game startup.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/landmass"
)

// CustomStart lands a custom single player game: the human team (0) deploys with
// the units bought on the Cargo/purchase screen, an opponent team (1) with the
// default quick start roster, each on its own landmass. units is the player's
// purchased loadout (it falls back to the base roster when nothing was bought)
// and cargos is the raw/fuel load the player put on each of them, index aligned
// with units; gold is the credits LEFT OVER after the purchase screen, which carry
// into the game as the team's spendable gold balance (drawn down by unit upgrades,
// grown by a Gold Refinery). C++: the unspent start_gold becomes the team's gold reserve.
//
// CustomStart reuses RandomStart's landmass selection and finished base layout. It chooses the landing site automatically and does not animate a MASTER lander unfolding.
// C++: GameManager_PlayerMissionSetup -> GameManager_SelectSite.
func (s *Screen) CustomStart(units []assets.UnitType, cargos []int, gold int, seed int64) {
	rng := rand.New(rand.NewSource(seed))

	humanRoster := baseRoster
	humanCargos := []int(nil) // no purchase screen ran: the founders take their default cargo

	if specs := loadoutRoster(units); len(specs) > 0 {
		humanRoster = specs
		humanCargos = cargos
	}

	// One human team plus the computer opponents the player chose on the setup
	// screen (1..3); an unset count keeps the original single opponent game.
	opponents := s.customOpponents
	if opponents < 1 {
		opponents = 1
	}

	teams := opponents + 1

	// Each opponent (no real AI depth yet) lands with the full quick start force.
	opponentRoster := append(append([]unitSpec{}, baseRoster...), randomExtraRoster...)
	rosters := make([][]unitSpec, teams)
	rosters[0] = humanRoster

	// Only the human bought its loadout on the purchase screen; every opponent
	// lands with the founder default cargo (a nil entry).
	teamCargos := make([][]int, teams)
	teamCargos[0] = humanCargos

	for team := 1; team < teams; team++ {
		rosters[team] = opponentRoster
	}

	// One landing spot per team (distinct islands where the map allows): spot[0] is the
	// human's first candidate cell, the others are each computer team's preferred spot,
	// kept only when it lands clear of the human (placeComputerSpots, once the human has
	// landed). The deposits are seeded around the final spots.
	spot := s.chooseStartSpots(teams, rng)

	// The human is team 0; every opponent is a computer team.
	s.game.State.Teams[0].Type = game.TeamPlayer

	for team := 1; team < teams; team++ {
		s.game.State.Teams[team].Type = game.TeamComputer
	}

	// The credits left over from the purchase screen become the player's spendable
	// gold balance (the in game Upgrade screen draws on g.Gold; a Gold Refinery adds
	// to it). The opponents have no upgrade path yet, so they land with no reserve.
	s.game.Gold[0] = gold

	// deployBases lands every team once the human confirms a landing site (the
	// opponents' spots are chosen automatically). C++: GameManager_ProcessTeamMissionSupplyUnits.
	deployBases := func(humanX, humanY int) {
		spot[0] = game.Point{X: humanX, Y: humanY} // the player's chosen site

		// The computer teams land after the human, clear of every team already down (N1).
		s.placeComputerSpots(spot, rng)

		if s.game.MiningDifficulty == 0 {
			s.game.MiningDifficulty = 4 // human normal mining scale
		}

		s.seedCustomWorld(spot, seed)

		var first *game.UnitInfo

		for team := 0; team < teams; team++ {
			// A custom game lands the player's PURCHASED loadout, so it does not get an
			// auto coastal shipyard (coastal=false); only the random benches do.
			if u := s.deployStartBase(team, spot[team], rosters[team], teamCargos[team], false, rng); team == 0 {
				first = u
			}
		}

		s.debugCheckDeployOverlap() // catch a real same cell mobile stack on the live map (debug only)

		s.landStartingStock()

		s.selected = first
		s.game.State.ActiveTeam = s.game.State.FirstActiveTeam()
		s.game.State.PlayerTeam = s.game.State.ActiveTeam
		s.fogShow = true

		// The landing turn IS a turn: run the player's economy and construction pass now,
		// the same one every later turn opens with (and the one MissionStart already ran
		// for a scripted mission). Without it the mine's first output and the first step of
		// any build only arrived at the START of turn 2, quietly costing the player a turn.
		// C++ ported : game_manager.cpp - GameManager_ManageEconomy on turn one
		s.game.BeginTeamTurn(s.game.State.PlayerTeam)

		// Descend onto the planet (camera zoom in) and unfold the base to full size.
		s.StartLanding(s.r.Zoom() * 2)
	}

	// Let the player pick the human team's landing site before deploying. No computer
	// team has landed yet, so there is no enemy zone to keep clear of.
	// C++ ported : game_manager.cpp - the human teams run GameManager_PlayerMissionSetup
	// before any computer team sets up.
	/* s.BeginSiteSelect(0, spot[0].X, spot[0].Y, spot[1:], deployBases): note: unused code in Go port, the computer spots were fixed before the human picked and only a 3 cell zone around them was refused, so the human could land beside the computer (N1) */
	s.BeginSiteSelect(0, spot[0].X, spot[0].Y, deployBases)
}

// landingSafeRange is how far, in grid cells on either axis, a computer team lands from
// every team already down: the proximity range plus 4, as the reference keeps it.
// C++ ported : continent.cpp - Continent::IsViableSite (IsDangerousProximity with
// proximity_range + 4).
const landingSafeRange = landingProximityRange + 4

// placeComputerSpots picks the computer teams' landing spots once the human has landed
// on spot[0]. Each team in turn keeps its preferred spot when it lies clear of every team
// already down; otherwise it takes the best inland cell that does, trying the roomy
// islands no team holds yet first, then any roomy land, then any land. On a map too small
// to keep that distance it takes the land cell farthest from the teams already down, so
// the pick never hangs (the reference retries without end).
//
// C++ ported : continent.cpp - Continent::SelectLandingSite / IsViableSite (the computer
// teams set up after the humans, game_manager.cpp).
func (s *Screen) placeComputerSpots(spot []game.Point, rng *rand.Rand) {
	islands := s.islandsWithRoom(landmass.Find(s.wmap))

	for team := 1; team < len(spot); team++ {
		placed := spot[:team]
		spot[team] = s.safeComputerSpot(spot[team], placed, islands, rng)
		dbg.Printf("landing: team %d at (%d,%d), %d cells from the nearest team", team, spot[team].X, spot[team].Y,
			nearestTeamRange(spot[team], placed))
	}
}

// safeComputerSpot returns the landing cell of one computer team: preferred when it is
// clear of placed, else the best clear cell of the free roomy islands, of all roomy land,
// or of all land, in that order; the farthest land cell when none is clear.
func (s *Screen) safeComputerSpot(preferred game.Point, placed []game.Point, islands []landmass.Component, rng *rand.Rand) game.Point {
	if landingClear(preferred, placed) {
		return preferred
	}

	var free, roomy []game.Point

	for _, c := range islands {
		clear := clearCells(c.Cells, placed)
		roomy = append(roomy, clear...)

		if !holdsAny(c.Cells, placed) {
			free = append(free, clear...)
		}
	}

	land := s.landCells()

	for _, cells := range [][]game.Point{free, roomy, clearCells(land, placed)} {
		if len(cells) > 0 {
			return s.landingSpot(cells, rng)
		}
	}

	return farthestCell(land, placed, preferred)
}

// landingClear reports whether p lies more than landingSafeRange cells from every placed
// spot on at least one axis. C++ ported : continent.cpp - Continent::IsDangerousProximity.
func landingClear(p game.Point, placed []game.Point) bool {
	return nearestTeamRange(p, placed) > landingSafeRange
}

// nearestTeamRange is the per axis (Chebyshev) distance from p to the nearest placed spot.
func nearestTeamRange(p game.Point, placed []game.Point) int {
	nearest := -1

	for _, q := range placed {
		d := max(iabsInt(p.X-q.X), iabsInt(p.Y-q.Y))
		if nearest < 0 || d < nearest {
			nearest = d
		}
	}

	return nearest
}

// clearCells returns the cells that lie clear of every placed spot (landingClear).
func clearCells(cells, placed []game.Point) []game.Point {
	var out []game.Point

	for _, c := range cells {
		if landingClear(c, placed) {
			out = append(out, c)
		}
	}

	return out
}

// holdsAny reports whether any placed spot is one of the cells.
func holdsAny(cells, placed []game.Point) bool {
	for _, c := range cells {
		for _, q := range placed {
			if c == q {
				return true
			}
		}
	}

	return false
}

// farthestCell returns the cell with the largest distance to its nearest placed spot (the
// first one on a tie), or fallback when there are no cells.
func farthestCell(cells, placed []game.Point, fallback game.Point) game.Point {
	best, bestRange := fallback, -1

	for _, c := range cells {
		if d := nearestTeamRange(c, placed); d > bestRange {
			best, bestRange = c, d
		}
	}

	return best
}

// seedCustomWorld fills the map around the chosen landing spots before any base deploys:
// the resource deposits, the neutral alien derelicts, the hidden gag prop, and each team's
// clan. Everything here must run before deployStartBase, whose units read the clan bonuses.
func (s *Screen) seedCustomWorld(spot []game.Point, seed int64) {
	s.game.Resources = economy.NewResourceMap(s.wmap.W, s.wmap.H)
	if s.resourceLevelsSet {
		// The player picked Poor / Average / Rich per material on the setup screen;
		// seed the deposits at that richness. C++: menu_update_resource_levels.
		params := economy.ResourceParamsForLevels(s.resourceLevels[0], s.resourceLevels[1], s.resourceLevels[2])
		economy.AllocateResourcesWith(s.game.Resources, s.wmap.W, s.wmap.H, spot, game.NewRandomizer(uint32(seed)), params)
	} else {
		economy.AllocateResources(s.game.Resources, s.wmap.W, s.wmap.H, spot, game.NewRandomizer(uint32(seed)))
	}

	// Scatter the neutral alien derelict faction across the map, density per the
	// custom game "alien derelicts" setting. Sites stay clear of every team's landing
	// spot. None (level 0) spawns nothing; missions carry their own placement and
	// never call this. C++: GameManager_PopulateMapWithAlienUnits (game_manager.cpp).
	sep, val := alienDensity(s.alienDerelicts)
	s.populateAlienDerelicts(sep, val, spot, game.NewRandomizer(uint32(seed)))

	// Rare hidden gag prop (Waldo / Harvster) on a clear land tile, independent of
	// the derelict density. A distinct seed keeps it reproducible but uncorrelated.
	s.spawnEasterEgg(spot, game.NewRandomizer(uint32(seed)^easterEggSeedSalt))

	// Resolve each team's clan (computer teams get a strategy flavored one) before the
	// bases deploy, so deployStartBase's units carry their clan stat bonuses. A clan
	// the player set via SetTeamClans is already explicit and is left as chosen.
	s.resolveTeamClans(game.NewRandomizer(uint32(seed)))
}

// landStartingStock gives every landed base its opening pool, exactly as the -random bench
// does: a raw buffer sized by the map's RAW richness (a poor map ships more of it, a rich
// one none at all - the deposits are right there), and the fuel that keeps the generator lit
// until the mine's own fuel income covers it. A custom game used to land with an EMPTY pool:
// the mine read 0/25 on turn 0 and the generator had nothing to burn. seedRawStoreDisplay
// mirrors the pool into the mine's cargo bar, which the economy's redistribute would
// otherwise only do from turn one.
func (s *Screen) landStartingStock() {
	startRaw := startSupplyRawFor(s.rawResourceLevel())

	for _, c := range s.game.Complexes {
		c.Material += startRaw
		c.Fuel += startSupplyFuel
		seedRawStoreDisplay(c)
	}
}

// loadoutRoster turns a purchased unit list into deployable specs, resolving each
// type's movement/role flags. C++: the CargoMenu loadout deployed by
// GameManager_DeployUnit.
func loadoutRoster(units []assets.UnitType) []unitSpec {
	specs := make([]unitSpec, 0, len(units))

	for _, t := range units {
		specs = append(specs, unitSpec{t, customLandFlags(t)})
	}

	return specs
}

// customLandFlags returns the deploy flags for a buyable unit type. Every Cargo
// roster unit is a mobile land unit; the constructor/engineer also carry the
// builder flag. Only the Tank has an independently rotating mobile head.
func customLandFlags(t assets.UnitType) game.Flags {
	f := flagsLand

	switch t {
	case assets.Constrct, assets.Engineer:
		f |= game.FlagConstructorUnit
	}

	if t == assets.Tank {
		f |= game.FlagTurretSprite | game.FlagCanHeadRotate
	}

	return f
}
