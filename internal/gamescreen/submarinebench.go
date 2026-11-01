// submarinebench.go : provides the submarine detection bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// submarineBenchZoom frames the Corvette and the (initially hidden) sub together.
const submarineBenchZoom = 32

// DeploySubmarineBench lays out the -emulate submarine bench so the "Submarine detected!"
// edge can be watched LIVE: a friendly (player) Corvette + AWAC sit in open water with an
// ENEMY submarine seated OUT of the Corvette's sonar, so the sub starts SUBMERGED (hidden
// by fog). Drive the Corvette toward it and the moment its sonar reaches the sub it
// surfaces / is revealed and the DISTINCT V_F201 "Submarine detected!" cue fires (PING +
// red banner), not the generic "Enemy detected!". Drive away again and it resubmerges,
// rearming the cue. The AWAC is air recon flavor - in the port only a Corvette's sonar
// detects a SUBMERGED sub (an AWAC sees one only once it surfaces). C++: n/a (debug helper).
func (s *Screen) DeploySubmarineBench() {
	const player, enemy = 0, 1
	seaFlags := game.FlagMobileSeaUnit | game.FlagSelectable
	airFlags := game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable

	// Anchor near a stable on map block, then find the nearest open water for the cluster.
	originX, originY, ok := s.snowBlock(8, 8)

	if !ok {
		originX, originY = 6, 6
	}

	center := game.Point{X: originX + 4, Y: originY + 4}

	seaX, seaY, haveWater := s.nearestCell(center, 60, func(x, y int) bool { return s.isOpenWater(x, y) })

	if !haveWater {
		seaX, seaY = center.X, center.Y // landlocked fallback: the cue works, just no submerge visual
	}

	// Gather the open water cells around that anchor so we can seat the Corvette and the
	// sub far apart (a long water lane to drive along).
	var water []game.Point

	for y := seaY - 16; y <= seaY+16; y++ {
		for x := seaX - 16; x <= seaX+16; x++ {
			if s.isOpenWater(x, y) {
				water = append(water, game.Point{X: x, Y: y})
			}
		}
	}

	// Corvette at the water cell nearest the anchor; the sub at the FARTHEST water cell
	// from it (max separation), so it starts out of sonar.
	corvCell := game.Point{X: seaX, Y: seaY}
	subCell := corvCell
	bestD2 := -1

	for _, p := range water {
		d2 := sq(p.X-corvCell.X) + sq(p.Y-corvCell.Y)

		if d2 > bestD2 {
			bestD2, subCell = d2, p
		}
	}

	if bestD2 <= 0 { // no separate water cell (landlocked / tiny pond): seat the sub a few cells over
		subCell = game.Point{X: corvCell.X + 8, Y: corvCell.Y}
		bestD2 = sq(8)
	}

	// A small, controlled sonar so the sub sits a few cells OUTSIDE it (a clear drive to
	// close the gap). gap-4, clamped, keeps the sub hidden until the Corvette approaches.
	gap := int(math.Sqrt(float64(bestD2)))
	sonar := gap - 4

	if sonar < 2 {
		sonar = 2
	}

	if sonar > 6 {
		sonar = 6
	}

	corvette := s.deployUnit(assets.Corvette, seaFlags, player, 0, corvCell.X, corvCell.Y)
	corvette.Values.Set(game.AttribScan, sonar) // controlled sonar radius for the bench
	sub := s.deployUnit(assets.Submarne, seaFlags, enemy, 0, subCell.X, subCell.Y)
	sub.Order = game.OrderAwait // not Idle, so it projects and can be detected
	awac := s.deployUnit(assets.Awac, airFlags, player, 0, corvCell.X, corvCell.Y-2)

	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.selected = corvette
	s.showNames = true
	s.fogShow = true // honest fog: the sub stays hidden until the Corvette's sonar reaches it
	s.fogTeam = player
	s.game.RefreshVisibility()

	s.r.SetZoom(submarineBenchZoom)
	midX, midY := (corvCell.X+subCell.X)/2, (corvCell.Y+subCell.Y)/2
	s.startSingleBench("Submarine bench: the enemy sub starts SUBMERGED (out of sonar). Drive your Corvette toward it - when your sonar reaches it you hear 'Submarine detected!' (F201). Back off and it re-submerges + re-arms. The AWAC alone cannot detect a submerged sub.", game.Point{X: midX, Y: midY}, corvette)

	detected := s.game.Vis != nil && s.game.Vis.VisibleToTeam(sub, player)
	dbg.Printf("submarine bench: corvette (%d,%d) sonar=%d, awac (%d,%d), enemy sub (%d,%d) gap=%d cells; detected at start=%v (want false)",
		corvette.GridX, corvette.GridY, sonar, awac.GridX, awac.GridY, sub.GridX, sub.GridY, gap, detected)
}

// sq returns n squared (a small integer helper for cell distance math).
func sq(n int) int {
	return n * n
}
