// strategy.go : the M.A.X. AI unit strategies.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// This file ports the terrain aware half of the original's AiPlayer::SelectStrategy
// (aiplayer.cpp): it reads the map's landmasses and scores the nine high level
// strategies by how the continents lie and how clever the opponent difficulty is,
// then weighted rolls one. The build/production roster already keys off the chosen
// Strategy, so this is what makes a computer team go naval on an archipelago, air on
// a cramped map, or armour rush on one shared continent.
//
// Reductions vs the original (all terrain only, one way ai->sim read of g.Grid.Surf):
//   - no landing site selection / mission supply rolls (team placement is the game
//     layer's job; teams are already on the map by AI init);
//   - no enemy start proximity filtering of continents (needs the setup's per team
//     starting positions, which the AI is not fed) - the viability test is the
//     terrain only IsViableContinent(false) path;
//   - the flood fill is an iterative stack rather than the original's scanline
//     MAXFloodFill (same components, idiomatic Go).
//
// C++ ported : continent.cpp (Continent) + aiplayer.cpp - AiPlayer::SelectStrategy.

// proximityRange is the "is this landmass small / cramped" yardstick: a continent
// whose bounding box fits inside this many cells in both axes counts as close
// proximity (everyone is packed together on it). It is the vanilla settings default
// for "proximity_range" (settings.cpp). C++: ResourceManager_GetSettings()->
// GetNumericValue("proximity_range").
const proximityRange = 14

// firstFiller is the first continent label id. The original numbers continents from 3 so
// the values 0 (blocked) / 1 (water) / 2 (unvisited land) stay distinct in the
// access map; the isolation BFS keys off "label >= 3" to mean "a landmass".
const firstFiller = 3

// surfBlocked/surfWater/surfLand are the access map cell classes the original builds
// before flooding (aiplayer.cpp): air/none is impassable, water and coast are sea,
// land is floodable. Land cells are later overwritten with their continent's filler.
const (
	surfBlocked = 0
	surfWater   = 1
	surfLand    = 2
)

// continent is one viable landmass: its size, bounding box (ulx/uly inclusive,
// lrx/lry exclusive, like the original's Rect), and the two shape verdicts the strategy
// table reads. filler is the label id its cells carry in the classified grid.
// C++: class Continent (continent.hpp).
type continent struct {
	filler              int
	size                int
	ulx, uly, lrx, lry  int
	closeProx, isolated bool
}

// strategyCount is the number of strategy slots in the score table (Random..Espionage).
const strategyCount = int(StrategyEspionage) + 1

// dir8X/dir8Y are the 8 neighbour offsets in N,NE,E,SE,S,SW,W,NW order, so an even
// index is an orthogonal step and an odd index a diagonal one - matching the original's
// DIRECTION_OFFSETS and its `direction & 1` orthogonal/diagonal cost split.
var (
	dir8X = [8]int{0, 1, 1, 1, 0, -1, -1, -1}
	dir8Y = [8]int{-1, -1, 0, 1, 1, 1, 0, -1}
)

// findContinents classifies the map's terrain and flood fills its landmasses into
// viable continents, each tagged with its close proximity and isolation verdicts.
// Only landmasses big enough to host a base (a 4x4 all land block) are returned, so
// slivers and single cells do not inflate the strategy scores. Reads g.Grid.Surf
// directly - no new sim seam. C++: the access map build + per land Continent loop in
// AiPlayer::SelectStrategy (aiplayer.cpp) over continent.cpp's ctor.
func findContinents(g *sim.Game) []continent {
	if g == nil || g.Grid == nil || g.Grid.Surf == nil || g.W <= 0 || g.H <= 0 {
		return nil
	}

	w, h := g.W, g.H

	// Classify every cell into blocked / water / floodable land.
	cells := make([]int, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := g.Grid.Surf.SurfaceAt(x, y)
			switch {
			case s&world.SurfaceLand != 0:
				cells[y*w+x] = surfLand
			case s&(world.SurfaceWater|world.SurfaceCoast) != 0:
				cells[y*w+x] = surfWater
			default:
				cells[y*w+x] = surfBlocked
			}
		}
	}

	// Flood fill each unvisited land cell into a labelled component. cells is reused
	// as the label grid: a flooded cell holds its continent's filler id (>= 3).
	var all []continent
	filler := firstFiller
	stack := make([][2]int, 0, 64)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if cells[y*w+x] != surfLand {
				continue
			}

			c := continent{filler: filler, ulx: x, uly: y, lrx: x + 1, lry: y + 1}
			stack = stack[:0]
			stack = append(stack, [2]int{x, y})
			cells[y*w+x] = filler

			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				px, py := p[0], p[1]
				c.size++
				c.ulx, c.uly = min(c.ulx, px), min(c.uly, py)
				c.lrx, c.lry = max(c.lrx, px+1), max(c.lry, py+1)

				for d := 0; d < 8; d++ {
					nx, ny := px+dir8X[d], py+dir8Y[d]
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}

					if cells[ny*w+nx] == surfLand {
						cells[ny*w+nx] = filler
						stack = append(stack, [2]int{nx, ny})
					}
				}
			}

			all = append(all, c)
			filler++
		}
	}

	// Keep only landmasses that can host a base, then read their shape verdicts.
	viable := all[:0]
	for _, c := range all {
		if !continentViable(cells, w, h, c) {
			continue
		}

		c.closeProx = continentCloseProximity(c)
		c.isolated = continentIsolated(cells, w, h, c)
		viable = append(viable, c)
	}

	return viable
}

// continentViable reports whether the landmass has at least one 4x4 block entirely
// its own cells - room to set down a base. C++: Continent::IsViableContinent /
// IsViableSite (continent.cpp), with the enemy proximity test dropped.
func continentViable(label []int, w, h int, c continent) bool {
	for x := c.ulx; x+4 <= c.lrx && x+4 <= w; x++ {
		for y := c.uly; y+4 <= c.lry && y+4 <= h; y++ {
			ok := true

			for i := x; i < x+4 && ok; i++ {
				for j := y; j < y+4; j++ {
					if label[j*w+i] != c.filler {
						ok = false
						break
					}
				}
			}

			if ok {
				return true
			}
		}
	}

	return false
}

// continentCloseProximity reports whether the landmass is small/cramped enough that
// teams on it sit on top of each other (favours air and combined arms). C++:
// Continent::IsCloseProximity (continent.cpp).
func continentCloseProximity(c continent) bool {
	return (c.lrx-c.ulx) <= proximityRange && (c.lry-c.uly) <= proximityRange
}

// continentIsolated reports whether a close proximity landmass is cut off by water
// with no short land bridge to a bigger mass (favours sea and scout horde). It runs
// the original's bounded hand rolled Dijkstra out of the continent over the gap:
// orthogonal steps cost 4, diagonal 6, capped at 24; reaching another landmass within
// the cap means it is NOT isolated. Only close proximity continents can be isolated.
// C++: Continent::TestIsolated (continent.cpp).
func continentIsolated(label []int, w, h int, c continent) bool {
	if !c.closeProx {
		return false
	}

	// Work over the continent's bounds grown by 6 cells (the search zone), clamped.
	ulx, uly := max(0, c.ulx-6), max(0, c.uly-6)
	lrx, lry := min(w, c.lrx+6), min(h, c.lry+6)

	// acc is the cost/visited map: continent cells start settled at 0, every other
	// zone cell starts at 0x7F (unreached, bit0 set = still eligible). The original
	// reuses the odd bit as the "in the frontier" flag.
	acc := make([]int, w*h)
	frontier := make([][2]int, 0, 64)

	for x := ulx; x < lrx; x++ {
		for y := uly; y < lry; y++ {
			if label[y*w+x] == c.filler {
				acc[y*w+x] = 0x00
				frontier = append(frontier, [2]int{x, y})
			} else {
				acc[y*w+x] = 0x7F
			}
		}
	}

	for len(frontier) > 0 {
		p := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		px, py := p[0], p[1]
		value := acc[py*w+px] | 0x01
		acc[py*w+px] = value

		for d := 0; d < 8; d++ {
			nx, ny := px+dir8X[d], py+dir8Y[d]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}

			value2 := value + 4
			if d&1 == 1 {
				value2 = value + 6
			}

			if value2 <= 24 && value2 < acc[ny*w+nx] {
				lab := label[ny*w+nx]
				if lab >= firstFiller && lab != c.filler {
					return false // reached another landmass within the cap: connected
				}

				value2 &= 0xFE
				if acc[ny*w+nx]&0x01 != 0 && value2 <= 20 {
					frontier = append(frontier, [2]int{nx, ny})
				}

				acc[ny*w+nx] = value2
			}
		}
	}

	return true
}

// scoreStrategies builds the weight of each strategy from the continent layout,
// scaled by the opponent difficulty tier. A faithful port of aiplayer.cpp:
// one viable big mass biases a tank horde (more so at higher tiers); many masses
// bias missiles/defensive; cramped masses bias air/combined; isolated masses bias
// sea/scout. Espionage is never scored (the original leaves it for an explicit menu choice).
func scoreStrategies(continents []continent, tier int) [strategyCount]int {
	var scores [strategyCount]int

	maxSize := 0
	for _, c := range continents {
		maxSize = max(maxSize, c.size)
	}

	// One viable big landmass (everyone shares it) -> roll armour straight at the
	// enemy, weighted up sharply with difficulty.
	if tier >= OpponentAverage {
		continentScore := 0

		for _, c := range continents {
			if maxSize <= 80 || c.size >= 80 {
				continentScore++
			}
		}

		if continentScore == 1 {
			switch tier {
			case OpponentAverage:
				scores[StrategyTankHorde] = 1
			case OpponentExpert:
				scores[StrategyTankHorde] = 6
			case OpponentMaster:
				scores[StrategyTankHorde] = 12
			case OpponentGod:
				scores[StrategyTankHorde] = 24
			}
		}
	}

	// When a big mass dominates, drop the small fry from the count the rest of the
	// table reads (erase continents under 80 if the max is over 80).
	count, isolatedCount, closeCount := 0, 0, 0
	for _, c := range continents {
		if maxSize > 80 && c.size < 80 {
			continue
		}

		count++

		if c.isolated {
			isolatedCount++
		}

		if c.closeProx {
			closeCount++
		}
	}

	// The main score table.
	scores[StrategyMissiles] = count

	if tier >= OpponentApprentice {
		scores[StrategyDefensive] = count
	}

	if tier >= OpponentAverage && scores[StrategyTankHorde] == 0 {
		scores[StrategyDefensive] = count

		if isolatedCount > 0 {
			scores[StrategyScoutHorde] = 1
		} else {
			scores[StrategyScoutHorde] = count
		}

		if tier >= OpponentMaster {
			scores[StrategyScoutHorde] *= 2
		}

		if tier >= OpponentGod {
			scores[StrategyScoutHorde] *= 2
		}
	}

	scores[StrategyAir] = closeCount * 2

	if tier >= OpponentExpert {
		scores[StrategyTankHorde] = count
	}

	scores[StrategySea] = isolatedCount * 2
	scores[StrategyFastAttack] = count - closeCount
	scores[StrategyCombinedArms] = (count + closeCount) * 2

	return scores
}

// strategyRoll returns a value in [0,n) for the weighted strategy pick. It is a
// package var so a test can force the choice (mirrors winGoalRoll / pickZoneSearch).
var strategyRoll = func(n int) int {
	if n <= 0 {
		return 0
	}

	return rand.Intn(n)
}

// pickStrategy weighted rolls one strategy from the score table. To diversify the
// computer teams it first zeroes any strategy a sibling team already committed to
// (decided); if that would leave nothing, it rolls on the undiversified table
// instead (the original's potential_strategies > 0 guard). An empty table yields
// StrategyRandom, which the caller resolves to the clan default. C++: the
// weighted random selection in AiPlayer::SelectStrategy (aiplayer.cpp).
func pickStrategy(scores [strategyCount]int, decided []Strategy) Strategy {
	s := scores
	for _, d := range decided {
		if int(d) >= 0 && int(d) < strategyCount {
			s[d] = 0
		}
	}

	total := 0
	for _, v := range s {
		total += v
	}

	if total <= 0 {
		// Every positive option is already taken: fall back to the full table.
		s = scores
		total = 0

		for _, v := range s {
			total += v
		}

		if total <= 0 {
			return StrategyRandom
		}
	}

	roll := strategyRoll(total) + 1
	idx := -1
	for roll > 0 {
		idx++
		roll -= s[idx]
	}

	return Strategy(idx)
}

// resolveStrategy returns the team's committed strategy: the one a loaded save carries
// (g.State.AIStrategy stores strategy+1, 0 = unset) if set, else the freshly chosen
// strategy written back onto the game state so a save captures it. Unlike the win goal,
// the strategy is rolled earlier in SelectStrategy, so this only persists/restores it -
// the same persist through the game state pattern the win goal and difficulty tier use,
// so a reload keeps the AI's posture instead of re-rolling it from the map.
func resolveStrategy(g *sim.Game, team int, chosen Strategy) Strategy {
	var st *game.GameState
	if g != nil {
		st = g.State
	}

	if st != nil && team >= 0 && team < len(st.AIStrategy) && st.AIStrategy[team] > 0 {
		return Strategy(st.AIStrategy[team] - 1)
	}

	if st != nil && team >= 0 && team < len(st.AIStrategy) {
		st.AIStrategy[team] = int(chosen) + 1
	}

	return chosen
}

// strategyName labels a strategy for the debug trace (the original's strategy_strings,
// aiplayer.cpp).
func strategyName(s Strategy) string {
	names := [strategyCount]string{
		StrategyRandom:       "random",
		StrategyDefensive:    "defensive",
		StrategyMissiles:     "missiles",
		StrategyAir:          "air",
		StrategySea:          "sea",
		StrategyScoutHorde:   "scout horde",
		StrategyTankHorde:    "tank horde",
		StrategyFastAttack:   "fast attack",
		StrategyCombinedArms: "combined arms",
		StrategyEspionage:    "espionage",
	}

	if int(s) < 0 || int(s) >= strategyCount {
		return "random"
	}

	return names[s]
}
