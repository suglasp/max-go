// apcunit.go : implements the APC unit handling.
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

// apcUnitZoom opens the sound test bench large enough that the units and their
// drive read clearly while the whole roster fits on screen.
const apcUnitZoom = 48

// DeployAPCs lays out a small unit sound test bench for -emulate apcunit, on
// solid snow. Mobile row: two APCs + a tank + a bulldozer (player) and one enemy
// tank: select one for its idle hum, click to move it (drive then stop sound),
// right click the enemy tank to fire (fire/hit). Building row: a powered mining
// station and power plant (running hum on select) plus a powered OFF plant
// (silent on select), so the operational versus down building sound can be heard.
// Player units are team 0 and the enemy tank team 1 (hostile to the attack path).
// C++: n/a (a debug layout helper).
func (s *Screen) DeployAPCs() {
	// Rows are three cells apart so each unit's name label (drawn two cells below
	// its sprite) clears the next row down; the building row is 2x2 footprints.
	originX, originY, ok := s.snowBlock(8, 9)

	if !ok {
		originX, originY = 4, 4
	}

	const (
		player = 0
		enemy  = 1
	)
	mobile := game.FlagMobileLandUnit | game.FlagSelectable

	type benchUnit struct {
		t      assets.UnitType
		team   int
		flags  game.Flags
		dx, dy int
		name   string
	}
	roster := []benchUnit{
		{assets.ClnTrans, player, mobile, 0, 0, "APC"},
		{assets.ClnTrans, player, mobile, 1, 0, "APC"},
		{assets.Tank, player, mobile | game.FlagTurretSprite, 0, 3, "Tank"},
		{assets.Bulldozr, player, mobile, 2, 3, "Bulldozer"},
		{assets.Tank, enemy, mobile | game.FlagTurretSprite, 5, 0, "Enemy Tank"},
	}

	for _, r := range roster {
		gx, gy := originX+r.dx, originY+r.dy
		s.deployUnit(r.t, r.flags, r.team, 0, gx, gy)
		s.galleryLabels = append(s.galleryLabels, galleryLabel{name: r.name, gx: gx, gy: gy})
	}

	// Building row (2x2 footprints): two powered structures that hum when
	// selected and one powered off plant that stays silent.
	type benchBuilding struct {
		t       assets.UnitType
		dx      int
		powered bool
		name    string
	}
	buildings := []benchBuilding{
		{assets.MiningSt, 0, true, "Mining (on)"},
		{assets.PowerStn, 3, true, "Power (on)"},
		{assets.PowerStn, 6, false, "Power (off)"},
	}

	for _, b := range buildings {
		gx, gy := originX+b.dx, originY+6
		s.placeTestBuilding(b.t, player, gx, gy, b.powered)
		s.galleryLabels = append(s.galleryLabels, galleryLabel{name: b.name, gx: gx, gy: gy})
	}

	// Team 1 is a live opponent so the enemy tank is a valid attack target and
	// the turn loop has two sides to hand over between.
	s.game.State.Teams[enemy].Type = game.TeamPlayer
	s.game.State.ActiveTeam = player
	s.game.State.PlayerTeam = player

	s.fogShow = false
	s.r.SetZoom(apcUnitZoom)
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(originX+4, originY+4, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
	dbg.Printf("apcunit: snow block (%d,%d) deployed=%d", originX, originY, len(s.game.Units.Units()))
}

// placeTestBuilding deploys a 2x2 building for the sound bench, powered on
// (operational, hums when selected) or off (silent). C++: n/a (debug helper).
func (s *Screen) placeTestBuilding(t assets.UnitType, team, gx, gy int, powered bool) {
	u := game.NewUnitInfo(t, team, s.spawnSeq+1, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	u.Values = s.teamValues(team, t)
	u.InitState()

	if powered {
		u.Order = game.OrderPowerOn
	} else {
		u.Order = game.OrderPowerOff
	}

	u.State = game.OrderStateExecutingOrder
	s.spawnSeq++
	s.game.PlaceBuilding(u, gx, gy)
}
