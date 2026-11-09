// combat_feedback_test.go : tests the victim side of a raid and the reaction fire report gate.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// newRaidScreen builds a headless Screen on a flat map with the player as team 0 and
// two computer opponents, and records every announcer line it speaks.
func newRaidScreen(t *testing.T) (*Screen, *sim.Game, *[]string) {
	t.Helper()

	s, g := newTestScreen(t, 16, 1, 2)
	s.r = nil // no map view: every reactor is off screen, so its announcer cue plays
	s.turnFlow = turnFlowState{handover: -1}

	var played []string
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.announcer.clock = func() uint64 { return 10000 }

	return s, g, &played
}

// raidCommando deploys a commando with one shot ready.
func raidCommando(g *sim.Game, team, cellX, cellY int) *game.UnitInfo {
	c := game.NewUnitInfo(assets.Commando, team, 100+team, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	c.Values.Set(game.AttribRounds, 1)
	c.Shots = 1
	g.Units.Deploy(c, cellX, cellY)

	return c
}

// raidTank is an electronic (raidable) tank.
var raidTank = tankStats{angle: 2, turns: 8, hits: 30, flags: game.FlagElectronicUnit}

/* // note: unused code in Go port
// replaced by the shared mkTank(g.Units, team, id, cellX, cellY, raidTank) in fixture_test.go
// raidTarget deploys an electronic (raidable) tank.
func raidTarget(g *sim.Game, team, id, cellX, cellY int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable | game.FlagElectronicUnit
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribTurns, 8)
	u.Values.Set(game.AttribHits, 30)
	u.Hits = 30
	g.Units.Deploy(u, cellX, cellY)

	return u
}
*/

// forceRoll installs a randomizer seed whose next roll succeeds (want true) or misses
// (want false) against chance, so a raid's outcome is decided rather than rolled.
func forceRoll(t *testing.T, chance int, want bool) {
	t.Helper()

	for seed := uint32(1); seed < 100000; seed++ {
		game.RandomizerSetSeed(seed)

		if (game.RandomizerGenerate(101) <= chance) == want {
			game.RandomizerSetSeed(seed)

			return
		}
	}

	t.Fatalf("no seed found for chance=%d want=%v", chance, want)
}

// TestVictimHearsCommandoRaid verifies that a player receives a report when an enemy commando captures, disables, or is caught beside one of their units.
func TestVictimHearsCommandoRaid(t *testing.T) {
	// A capture: the player loses the tank and is told so.
	s, g, played := newRaidScreen(t)
	raider := raidCommando(g, 1, 1, 1)
	victim := mkTank(g.Units, 0, 7, 2, 1, raidTank)

	forceRoll(t, g.StealthChance(raider, victim, true), true)

	if ok, _ := g.Steal(raider, 2, 1); !ok {
		t.Fatal("the forced steal did not land")
	}

	s.tickCommandoEvents()

	if text, ok, tint := s.bannerText(); !ok || tint != bannerTintWorld || text != "Tank has been captured!" {
		t.Errorf("capture banner = (%q, ok=%v, tint=%v), want the captured notice on a world tint", text, ok, tint)
	}

	if !spoke(*played, "V_F243") {
		t.Errorf("a seized unit must speak V_F243 \"Units have been seized by the enemy!\", got %v", *played)
	}

	// A disable: same story, different line.
	s, g, played = newRaidScreen(t)
	raider = raidCommando(g, 1, 1, 1)
	victim = mkTank(g.Units, 0, 8, 2, 1, raidTank)

	forceRoll(t, g.StealthChance(raider, victim, false), true)

	if ok, _ := g.Disable(raider, 2, 1); !ok {
		t.Fatal("the forced disable did not land")
	}

	s.tickCommandoEvents()

	if text, ok, _ := s.bannerText(); !ok || text != "Tank has been disabled!" {
		t.Errorf("disable banner = (%q, ok=%v), want the disabled notice", text, ok)
	}

	if !spoke(*played, "V_F249") {
		t.Errorf("a disabled unit must speak V_F249 \"Unit not functioning!\", got %v", *played)
	}

	// A caught infiltrator: the attempt is reported to the team it was tried on, which is
	// the player's warning that an enemy commando is standing next to that unit.
	s, g, played = newRaidScreen(t)
	raider = raidCommando(g, 1, 1, 1)
	victim = mkTank(g.Units, 0, 9, 2, 1, raidTank)

	forceRoll(t, g.StealthChance(raider, victim, true), false)

	if ok, exposed := g.Steal(raider, 2, 1); ok || !exposed {
		t.Fatalf("steal ok=%v exposed=%v, want a miss that exposes the raider", ok, exposed)
	}

	s.tickCommandoEvents()

	if text, ok, _ := s.bannerText(); !ok || text != "Attempt to capture Tank!" {
		t.Errorf("exposure banner = (%q, ok=%v), want the attempted capture notice", text, ok)
	}

	if !spoke(*played, "V_F012") {
		t.Errorf("a caught infiltrator must speak V_F012, got %v", *played)
	}
}

// TestRaidBetweenComputerTeamsStaysQuiet verifies that a commando raid reports only to its victim, not to an uninvolved player.
func TestRaidBetweenComputerTeamsStaysQuiet(t *testing.T) {
	s, g, played := newRaidScreen(t)
	raider := raidCommando(g, 1, 1, 1)
	victim := mkTank(g.Units, 2, 7, 2, 1, raidTank)

	forceRoll(t, g.StealthChance(raider, victim, true), true)

	if ok, _ := g.Steal(raider, 2, 1); !ok {
		t.Fatal("the forced steal did not land")
	}

	s.tickCommandoEvents()

	if text, ok, _ := s.bannerText(); ok {
		t.Errorf("a raid between two computer teams posted the banner %q, want silence", text)
	}

	if len(*played) != 0 {
		t.Errorf("a raid between two computer teams spoke %v, want silence", *played)
	}
}

// TestLeaderChangeAnnounce: the "<colour> team has the most Ecospheres!" call goes out at
// a round close when the points LEAD changes hands, and only then - the opening round
// just records who is ahead, and a lead that stays put says nothing.
func TestLeaderChangeAnnounce(t *testing.T) {
	s, g, played := newRaidScreen(t)
	s.announcer.pointsLeader = announcerNoLeader

	// The first round close seeds the leader silently: nobody is announced into an
	// empty scoreboard.
	s.announceLeaderChange()

	if len(*played) != 0 {
		t.Errorf("the opening round announced %v, want a silent seed", *played)
	}

	// Green takes the lead: it is called out.
	g.TeamPoints[1] = 2
	s.announceLeaderChange()

	if !spoke(*played, "V_F122") {
		t.Errorf("green taking the lead must speak V_F122, got %v", *played)
	}

	// It holds the lead through the next round: nothing more to say.
	before := len(*played)
	g.TeamPoints[1] = 4
	s.announceLeaderChange()

	if len(*played) != before {
		t.Errorf("holding the lead spoke %v, want silence", (*played)[before:])
	}

	// Blue overtakes: the lead changed hands again.
	g.TeamPoints[2] = 9
	s.announceLeaderChange()

	if !spoke(*played, "V_F126") {
		t.Errorf("blue overtaking must speak V_F126, got %v", *played)
	}
}

// TestLeaderVoiceTagPerTeam pins the four team lines: they sit four resource ids apart
// because the original's table pairs a male and a female sample per team.
func TestLeaderVoiceTagPerTeam(t *testing.T) {
	want := []string{"V_F118", "V_F122", "V_F126", "V_F130"}

	for team, tag := range want {
		if got := leaderVoiceTag(team); got != tag {
			t.Errorf("leaderVoiceTag(%d) = %q, want %q", team, got, tag)
		}
	}

	if got := leaderVoiceTag(game.TeamCount); got != "" {
		t.Errorf("leaderVoiceTag out of range = %q, want empty", got)
	}
}

// reactionTurret deploys an armed, stationary turret that can see and shoot.
func reactionTurret(g *sim.Game, team, id, cellX, cellY int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.GunTurrt, team, id, 0)
	u.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribAttack, 12)
	u.Values.Set(game.AttribRange, 5)
	u.Values.Set(game.AttribRounds, 1)
	u.Values.Set(game.AttribAmmo, 6)
	u.Values.Set(game.AttribScan, 8)
	u.Values.Set(game.AttribHits, 40)
	u.Hits = 40
	u.InitState()
	g.Units.Deploy(u, cellX, cellY)

	return u
}

// reactionTank is a tank that drives into the turret's range.
var reactionTank = tankStats{angle: 2, speed: 6, scan: 6, armor: 4, hits: 40}

/* // note: unused code in Go port
// replaced by the shared mkTank(g.Units, team, id, cellX, cellY, reactionTank) in fixture_test.go
// reactionMover deploys a tank that drives into the turret's range.
func reactionMover(g *sim.Game, team, id, cellX, cellY int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, 6)
	u.Values.Set(game.AttribScan, 6)
	u.Values.Set(game.AttribArmor, 4)
	u.Values.Set(game.AttribHits, 40)
	u.Hits = 40
	u.InitState()
	g.Units.Deploy(u, cellX, cellY)

	return u
}
*/

// driveIntoReaction runs a mover into the turret's range, and checks the turret really
// did take its opportunity shot - so a test asserting on the REPORT cannot pass by the
// reaction never happening at all.
func driveIntoReaction(t *testing.T, g *sim.Game, mover *game.UnitInfo, cellX, cellY int) {
	t.Helper()

	before := mover.Hits

	g.RefreshVisibility()
	g.BeginTeamTurn(mover.Team)

	if !g.OrderMove(mover, cellX, cellY) {
		t.Fatal("the mover could not be ordered into the turret's range")
	}

	g.ResolveMovement()

	if mover.Hits >= before {
		t.Fatalf("no reaction shot landed: the mover still has %d hits", mover.Hits)
	}
}

// TestReactionBetweenComputerTeamsIsNotNarrated verifies that opportunity fire reports only when it involves the player's team.
func TestReactionBetweenComputerTeamsIsNotNarrated(t *testing.T) {
	// Computer team 1's turret reacts to computer team 2's tank: no banner.
	s, g, _ := newRaidScreen(t)
	reactionTurret(g, 1, 20, 5, 5)
	mover := mkTank(g.Units, 2, 21, 11, 5, reactionTank)
	driveIntoReaction(t, g, mover, 8, 5)

	s.tickReactions()

	if text, ok, _ := s.bannerText(); ok {
		t.Errorf("a computer vs computer reaction posted the banner %q, want silence", text)
	}

	// The player's own turret reacting is reported, as it always was.
	s, g, played := newRaidScreen(t)
	reactionTurret(g, 0, 22, 5, 5)
	mover = mkTank(g.Units, 1, 23, 11, 5, reactionTank)
	driveIntoReaction(t, g, mover, 8, 5)

	s.tickReactions()

	if _, ok, _ := s.bannerText(); !ok {
		t.Error("the player's own turret reacting must still report")
	}

	if len(*played) == 0 {
		t.Error("the player's own reaction fire must still speak the announcer cue")
	}
}

// TestReactionVoiceOnlyForUnseenUnselectedShooter checks the gate: the reaction fire voice
// plays only when the player's shooter is neither selected nor on screen. The banner always shows.
func TestReactionVoiceOnlyForUnseenUnselectedShooter(t *testing.T) {
	cases := []struct {
		name      string
		offScreen bool
		selected  bool
		wantVoice bool
	}{
		{"on screen", false, false, false},
		{"selected, off screen", true, true, false},
		{"unseen and not selected", true, false, true},
	}
	for _, tc := range cases {
		s, g, played := newRaidScreen(t)
		s.r = drawmap.New(s.wmap)
		turret := reactionTurret(g, 0, 22, 5, 5)
		mover := mkTank(g.Units, 1, 23, 11, 5, reactionTank)
		if tc.offScreen {
			s.camera.scrollX = 20 * s.r.Zoom() // well past the 16 cell map
		}
		if tc.selected {
			s.selected = turret
		}
		driveIntoReaction(t, g, mover, 8, 5)

		s.tickReactions()

		if _, ok, _ := s.bannerText(); !ok {
			t.Errorf("%s: the player's reaction must always post the banner", tc.name)
		}
		if gotVoice := len(*played) > 0; gotVoice != tc.wantVoice {
			t.Errorf("%s: voice played = %v, want %v (lines %v)", tc.name, gotVoice, tc.wantVoice, *played)
		}
	}
}

// TestUnitInsideMapView checks the viewport test: any part of the unit's cell counts, and a building
// covers 2x2 cells.
func TestUnitInsideMapView(t *testing.T) {
	m := &world.Map{Surface: make([]uint8, 64*64)}
	m.W, m.H = 64, 64
	s := &Screen{wmap: m, r: drawmap.New(m)}
	zoom := s.r.Zoom()
	vw, _ := viewSize(MainMap)

	unit := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	building := game.NewUnitInfo(assets.LightPlt, 0, 2, 0)
	building.Flags = game.FlagBuilding | game.FlagStationary

	cases := []struct {
		name    string
		u       *game.UnitInfo
		cellX   int
		scrollX int
		want    bool
	}{
		{"unit at the view origin", unit, 0, 0, true},
		{"unit right of the view", unit, vw/zoom + 2, 0, false},
		{"unit just scrolled off the left edge", unit, 5, 6 * zoom, false},
		{"building half past the left edge", building, 5, 6 * zoom, true},
		{"building fully past the left edge", building, 5, 7 * zoom, false},
	}
	for _, tc := range cases {
		tc.u.GridX, tc.u.GridY = tc.cellX, 1
		s.camera.scrollX, s.camera.scrollY = tc.scrollX, 0
		if got := s.unitInsideMapView(tc.u); got != tc.want {
			t.Errorf("%s: unitInsideMapView = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestMineKillIsAnnouncedLikeAnyOtherDeath verifies that a mine kill uses the normal unit-destruction voice and message.
func TestMineKillIsAnnouncedLikeAnyOtherDeath(t *testing.T) {
	s, g, played := newRaidScreen(t)

	// An enemy mine, and the player's own weak scout parked on top of it.
	mine := game.NewUnitInfo(assets.LandMine, 1, 9001, 0)
	mine.Flags = game.FlagStationary
	mine.Values = &game.UnitValues{}
	mine.Values.Set(game.AttribAttack, 24)
	mine.Values.Set(game.AttribHits, 1)
	mine.Hits = 1
	g.Units.Deploy(mine, 8, 4)

	victim := game.NewUnitInfo(assets.Scout, 0, 9002, 0)
	victim.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	victim.Values = &game.UnitValues{}
	victim.Values.Set(game.AttribHits, 8) // a one hit kill
	victim.Hits = 8
	g.Units.Deploy(victim, 8, 4)

	g.Tick() // detonateMines kills the scout and queues the blast

	if g.Units.ByID.Get(victim.ID) != nil {
		t.Fatal("the mine should have destroyed the scout")
	}

	s.tickMineBlasts()

	if !spoke(*played, "V_F234") && !spoke(*played, "V_F236") {
		t.Errorf("a mine kill must speak the unit destroyed line, got %v", *played)
	}

	if text, ok, _ := s.bannerText(); !ok || text != "Scout Car destroyed!" {
		t.Errorf("mine kill banner = (%q, ok=%v), want the destroyed notice", text, ok)
	}
}
