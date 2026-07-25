// spottedbench.go : provides the spotted unit emulation bench.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// spottedBenchZoom frames the chase clearly.
const spottedBenchZoom = 32

// DeploySpottedBench lays out the -emulate spotted test bench: ONE player Scout vs a
// THREE tank COMPUTER hunter squad on open snow. The squad starts with the scout inside
// its scan, so the AI has SPOTTED you. Move the scout far away into the open so it drops
// out of the squad's scan (into the fog), then End Turn: the squad advances toward the
// cell where it LAST saw you - the SpottedUnit last seen memory - not your
// scout's true position, and it does not instantly forget you. Watch the debug log for
// "ai: ... pursues last seen ...". (Three armed hunters clears the muster gate,
// minStrikeForce=3, so the squad commits once it has spotted you; their scan is set
// wider than their weapon range so they spot then advance rather than firing at once.)
// C++: n/a (a debug layout helper for the SpottedUnit / AiPlayer::UnitSpotted memory).
func (s *Screen) DeploySpottedBench() {
	originX, originY, ok := s.snowBlock(16, 14)

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	// Computer hunter squad: three armed tanks in a column on the left.
	hx := originX + 3

	for i := 0; i < 3; i++ {
		hunter := s.deployUnit(assets.Tank, mobile|game.FlagTurretSprite, enemy, 2, hx, originY+5+i)
		hunter.Values.Set(game.AttribScan, 7) // see out 7 (> weapon range 5): spot then advance
	}

	// Player Scout, 6 cells east of the squad: inside scan 7 (spotted), outside range 5.
	scout := s.deployUnit(assets.Scout, mobile, player, 6, originX+9, originY+6)

	s.game.State.Teams[enemy].Type = game.TeamComputer // a REAL AI hunter squad
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player
	s.game.RefreshVisibility() // seed the squad's spotted memory with the scout's start cell

	s.selected = scout
	s.showNames = true
	s.fogShow = false // reveal everything so you can watch the AI hunters move
	s.r.SetZoom(spottedBenchZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(originX+8, originY+7, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	s.showMessage("Spotted bench: the AI tank squad has SEEN your scout. Drive the scout far away into the open and End Turn - the squad heads to where it LAST saw you, then SEARCHES the area (a focused zone sweep or a wider wander, picked at random). Watch the debug log: 'pursues last-seen' / 'searching'.")
	dbg.Printf("spotted bench: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}
