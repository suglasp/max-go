// voice_test.go : tests voice.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// TestSentryUnitOrderVoiceIsBlendedPool checks a healthy mobile unit on guard maps to
// the blended ready/sentry select pool (not the deterministic on sentry confirmation).
func TestSentryUnitOrderVoiceIsBlendedPool(t *testing.T) {
	u := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	u.Order = game.OrderSentry
	if got := unitOrderVoice(u); got != vsSelectSentry {
		t.Errorf("a sentry unit's select voice = %v, want vsSelectSentry", got)
	}
}

// TestEnemyDetectedPlaysPing verifies the QOL cue: when a new hostile becomes visible,
// checkEnemyDetected plays the PING.WAV alert blip on the SFX channel (alongside the
// voice + banner). A player scanner reveals an enemy tank in its scan range.
func TestEnemyDetectedPlaysPing(t *testing.T) {
	s, g := newClickScreen(t)
	var sfx []string
	s.sound.sfxFn = func(primary, _ string, _ float64) { sfx = append(sfx, primary) }
	s.announcer.voiceFn = func(string) {} // swallow the voice line
	s.announcer.clock = func() uint64 { return 0 }

	scout := mkUnit(g, assets.Scout, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	scout.Values.Set(game.AttribScan, 6) // reveals the nearby enemy
	enemy := game.NewUnitInfo(assets.Tank, 1, 2, 0)
	enemy.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	enemy.Values = &game.UnitValues{}
	enemy.Order = game.OrderAwait // not Idle, so VisibleToTeam does not early out
	g.Units.Deploy(enemy, 6, 5)
	g.Vis.Rebuild(g.Units.Units())

	s.checkEnemyDetected()
	found := false
	for _, tag := range sfx {
		if tag == "PING" {
			found = true
		}
	}
	if !found {
		t.Errorf("a newly spotted enemy should play PING, sfx=%v", sfx)
	}

	// A nil sfxFn must not panic (headless / muted).
	s.sound.sfxFn = nil
	s.playEnemySpottedPing()
}

// newVoiceTestScreen returns a Screen wired with a capturing voice hook and a
// controllable clock, plus the slice the hook appends played tags to.
func newVoiceTestScreen() (*Screen, *[]string, *uint64) {
	var played []string
	var clk uint64
	s := &Screen{}
	s.announcer.clock = func() uint64 { return clk }
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	return s, &played, &clk
}

// TestVoiceTagsInSet checks every state with multiple variants only ever speaks a
// tag from its own set, across repeated plays (the random pick stays in range).
func TestVoiceTagsInSet(t *testing.T) {
	s, played, clk := newVoiceTestScreen()
	want := map[string]bool{}
	for _, tag := range voiceLines[vsSelectUnit].tags {
		want[tag] = true
	}
	for i := 0; i < 50; i++ {
		*clk += 1000 // step well past the cooldown each time so the line plays
		s.playVoice(vsSelectUnit)
	}
	if len(*played) != 50 {
		t.Fatalf("played %d lines, want 50", len(*played))
	}
	for _, tag := range *played {
		if !want[tag] {
			t.Errorf("played %q, not in select-unit set", tag)
		}
	}
}

// TestRemoveMinePlaysVoice verifies the voice coverage gap: picking a mine back up via
// the Remove Mine popup action speaks the "removing mines" line (V_F186/187), which
// the dispatcher defined but never triggered (laying is acknowledged on select).
func TestRemoveMinePlaysVoice(t *testing.T) {
	s, g := newClickScreen(t)
	var played []string
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.announcer.clock = func() uint64 { return 0 }

	// A mine layer loaded with room in store, sitting on its own freshly laid mine.
	layer := mkUnit(g, assets.MineLayr, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	layer.Values.Set(game.AttribStorage, 3)
	layer.Storage = 3
	if !g.LayMine(layer) {
		t.Fatal("LayMine setup failed")
	}

	// Find and invoke the Remove Mine popup action.
	var act func()
	for _, it := range s.popupItemsFor(layer) {
		if it.label == "Remove Mine" {
			act = it.action
		}
	}
	if act == nil {
		t.Fatal("no Remove Mine popup item for a layer on its own mine")
	}
	act()

	want := map[string]bool{}
	for _, tag := range voiceLines[vsRemovingMines].tags {
		want[tag] = true
	}
	found := false
	for _, tag := range played {
		if want[tag] {
			found = true
		}
	}
	if !found {
		t.Errorf("removing a mine should play a vsRemovingMines tag (V_F186/187), played=%v", played)
	}
}

// playedHasState reports whether any tag in played belongs to state's line set.
func playedHasState(played []string, state voiceState) bool {
	want := map[string]bool{}
	for _, tag := range voiceLines[state].tags {
		want[tag] = true
	}
	for _, tag := range played {
		if want[tag] {
			return true
		}
	}
	return false
}

// TestInWorldReloadPlaysVoice checks an in world Supply Truck reload speaks the
// "reloaded" line (V_F085/089) - a state the dispatcher defined but never fired.
func TestInWorldReloadPlaysVoice(t *testing.T) {
	s, g := newClickScreen(t)
	var played []string
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.announcer.clock = func() uint64 { return 0 }
	s.sound.sfxFn = func(_, _ string, _ float64) {} // swallow the reload SFX

	truck := mkUnit(g, assets.SplyTrck, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	truck.Storage = 5
	truck.Hits = 1
	tank := mkUnit(g, assets.Tank, 2, 5, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Values.Set(game.AttribAmmo, 4)
	tank.Ammo = 0
	tank.Hits = 1

	s.selected = truck
	s.reloadAt(5, 4)
	if !playedHasState(played, vsReloaded) {
		t.Errorf("an in-world reload should play vsReloaded (V_F085/089), played=%v", played)
	}
}

// timeWarnScreen builds a screen in round 1 of a two team game with the given configured turn timer.
func timeWarnScreen(fullTurnSeconds int) *Screen {
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	st := game.NewGameState(p)
	st.TurnCounter = 1
	st.ActiveTeam = 0
	return &Screen{game: &sim.Game{State: st}, turnFlow: turnFlowState{turnSeconds: fullTurnSeconds, fullTurnSeconds: fullTurnSeconds, turnStart: 1000}}
}

// TestShouldWarnTimeLow checks the turn low warning latch: it fires once per turn when the clock shows 20s or
// less on a configured timer over 20s, a modal pause or the End Turn grace (both move turnStart) never fire it
// again, a new turn re arms it, and a configured timer of 20s or less never warns.
func TestShouldWarnTimeLow(t *testing.T) {
	s := timeWarnScreen(180)
	if s.shouldWarnTimeLow(21) {
		t.Error("warned above the 20s threshold")
	}
	if !s.shouldWarnTimeLow(20) {
		t.Error("did not warn at 20s left")
	}
	if s.shouldWarnTimeLow(15) {
		t.Error("warned twice in the same turn (latch failed)")
	}
	s.resumeTurnClock(5000, 9000) // a blocking modal was open for 4s: turnStart moves
	if s.shouldWarnTimeLow(14) {
		t.Error("a closed modal fired the warning again in the same turn")
	}
	s.turnFlow.turnSeconds, s.turnFlow.turnStart = 15, 20000 // the End Turn grace cut the running clock
	if s.shouldWarnTimeLow(15) {
		t.Error("the End Turn grace fired the warning again in the same turn")
	}
	s.game.State.TurnCounter = 2 // a new turn re arms the warning
	if !s.shouldWarnTimeLow(20) {
		t.Error("did not warn again on a new turn")
	}
	if short := timeWarnScreen(20); short.shouldWarnTimeLow(5) {
		t.Error("a configured timer of 20s or less must never warn")
	}
}

// TestTimeWarnVoice checks the line choice: "20 seconds to end turn" when the clock ticks through 20s,
// "about to run out of time" when it jumped below 20s, and silence for a team that already ended its turn.
func TestTimeWarnVoice(t *testing.T) {
	s := timeWarnScreen(180)
	if v, ok := s.timeWarnVoice(20); !ok || v != vsTimeWarn {
		t.Errorf("20s left: got %v %v, want vsTimeWarn", v, ok)
	}
	if v, ok := s.timeWarnVoice(15); !ok || v != vsGraceWarn {
		t.Errorf("clock cut to 15s: got %v %v, want vsGraceWarn", v, ok)
	}
	s.game.State.Teams[0].FinishedTurn = true
	if _, ok := s.timeWarnVoice(15); ok {
		t.Error("a team that already ended its turn must not hear the run out of time line")
	}
}

// TestGoldFirstRefinedVoice checks the one time first gold refined cue: it speaks
// when the human team's credits rise across a turn, only once, and never for a
// non human team.
func TestGoldFirstRefinedVoice(t *testing.T) {
	s, g := newClickScreen(t)
	var played []string
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.announcer.clock = func() uint64 { return 0 }

	// Credits rose this turn (200 -> 205): the first refine speaks once.
	g.Gold[0] = 205
	s.checkGoldFirstRefined(0, 200)
	if !playedHasState(played, vsGoldRefined) {
		t.Errorf("a first gold-refine should play vsGoldRefined (V_F276), played=%v", played)
	}
	// Latched: a second rise the next turn does not reannounce.
	played = nil
	g.Gold[0] = 210
	s.checkGoldFirstRefined(0, 205)
	if playedHasState(played, vsGoldRefined) {
		t.Error("the first-gold voice must not repeat (latch failed)")
	}
	// No rise = no voice (fresh team).
	s2, g2 := newClickScreen(t)
	s2.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s2.announcer.clock = func() uint64 { return 0 }
	played = nil
	g2.Gold[0] = 200
	s2.checkGoldFirstRefined(0, 200) // unchanged
	if playedHasState(played, vsGoldRefined) {
		t.Error("no credit rise should not play the first-gold voice")
	}
}

// TestVoiceCooldown checks a state is suppressed while inside its cooldown window
// and plays again once the window has passed.
func TestVoiceCooldown(t *testing.T) {
	s, played, clk := newVoiceTestScreen()
	cd := voiceLines[vsSelectUnit].cooldown

	*clk = 10000
	s.playVoice(vsSelectUnit)
	if len(*played) != 1 {
		t.Fatalf("first play: got %d, want 1", len(*played))
	}
	*clk += cd - 1 // still inside the cooldown
	s.playVoice(vsSelectUnit)
	if len(*played) != 1 {
		t.Fatalf("within cooldown: got %d, want 1 (suppressed)", len(*played))
	}
	*clk += 2 // now past the cooldown
	s.playVoice(vsSelectUnit)
	if len(*played) != 2 {
		t.Fatalf("after cooldown: got %d, want 2", len(*played))
	}
}

// TestVoicePriorityGate checks a lower priority line is dropped while a higher one is
// still "speaking", and an equal or higher one still plays.
func TestVoicePriorityGate(t *testing.T) {
	s, played, clk := newVoiceTestScreen()

	*clk = 50000
	s.playVoice(vsStatusRed) // high priority (55)
	if len(*played) != 1 {
		t.Fatalf("status red: got %d, want 1", len(*played))
	}
	*clk += 100               // still within the speak window
	s.playVoice(vsSelectUnit) // low priority (10) -> dropped
	if len(*played) != 1 {
		t.Fatalf("low priority within window: got %d, want 1 (dropped)", len(*played))
	}
	*clk += 100                  // still within the window
	s.playVoice(vsUnitDestroyed) // higher priority (60) -> plays
	if len(*played) != 2 {
		t.Fatalf("higher priority within window: got %d, want 2", len(*played))
	}
	*clk += voiceSpeakWindowMS + 1 // window elapsed
	s.playVoice(vsSelectUnit)      // low priority now allowed
	if len(*played) != 3 {
		t.Fatalf("after window: got %d, want 3", len(*played))
	}
}

// TestStorageFullVoiceForcesPastEndedTurn verifies that all storage-full lines play when
// checkStorageFull runs at the same
// turn start as the queued "team ended turn" line (priority 20), which lands inside the
// speak window right before a storage-full check (priority 15) - the same "no voice on
// completion" class of bug already fixed for announceBuildsCompleted. storageFullVoice
// must use playVoiceForced, or a resource crossing full right after "ended turn" is
// silently dropped by the cross voice priority gate.
func TestStorageFullVoiceForcesPastEndedTurn(t *testing.T) {
	s, played, clk := newVoiceTestScreen()

	*clk = 50000
	s.playVoiceTag("V_F280", 20, "team ended turn") // a just-drained queue entry
	if len(*played) != 1 {
		t.Fatalf("ended turn: got %d, want 1", len(*played))
	}

	*clk += 100 // well inside voiceSpeakWindowMS

	var fuelFull bool
	s.storageFullVoice(&fuelFull, 100, 100, vsStorageFuelFull) // fuel just crossed full
	if len(*played) != 2 {
		t.Fatalf("storage full right after ended turn: got %d, want 2 (must not be dropped)", len(*played))
	}
	if !fuelFull {
		t.Error("fuelFull latch should be true once stored reaches capacity")
	}
}

// TestSelectSentryPoolBlendsF158 checks the on sentry select pool contains both the
// generic "Ready" lines and the "On sentry" line F158, and that across many picks it
// only ever speaks a tag from that blended set.
func TestSelectSentryPoolBlendsF158(t *testing.T) {
	want := map[string]bool{}
	for _, tag := range voiceLines[vsSelectSentry].tags {
		want[tag] = true
	}
	// The pool must include F158 (the "On sentry" line) plus at least one Ready line.
	if !want["V_F158"] {
		t.Error("the on-sentry select pool should include V_F158")
	}
	if !want["V_F004"] {
		t.Error("the on-sentry select pool should still include the Ready lines")
	}

	s, played, clk := newVoiceTestScreen()
	for i := 0; i < 60; i++ {
		*clk += 1000 // past the cooldown each time so the line plays
		s.playVoice(vsSelectSentry)
	}
	if len(*played) != 60 {
		t.Fatalf("played %d lines, want 60", len(*played))
	}
	for _, tag := range *played {
		if !want[tag] {
			t.Errorf("played %q, not in the on-sentry select set", tag)
		}
	}
}

// TestVoiceNilHook checks the dispatcher does nothing when no voice hook is wired.
func TestVoiceNilHook(t *testing.T) {
	s := &Screen{announcer: announcerState{clock: func() uint64 { return 0 }}}
	s.playVoice(vsSelectUnit) // must not panic with voiceFn nil
	s.playVoiceTag("V_F001", 10, "test")
}

// TestStorageFullWaitsForOpeningTurn verifies that full storage stays quiet during the opening turn and announces
// at the next turn start when it remains full.
func TestStorageFullWaitsForOpeningTurn(t *testing.T) {
	s, _ := newClickScreen(t)
	var played []string
	var clk uint64 = 100000
	s.announcer.clock = func() uint64 { return clk }
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }

	mine := s.benchPlaceBuilding(assets.MiningSt, 0, 4, 4, true)
	mine.Values.Set(game.AttribStorage, 25)
	mine.Storage = 25 // full raw storage

	s.checkStorageFull(0) // the opening turn start
	if len(played) != 0 || s.announcer.rawFull {
		t.Fatalf("storage full played during the opening turn: %v", played)
	}

	s.announcer.openingTurnDone = true // simulate End Turn
	s.checkStorageFull(0)
	if len(played) != 1 || !s.announcer.rawFull {
		t.Fatalf("after the opening turn: played %v (raw full latch %v), want the raw storage full line", played, s.announcer.rawFull)
	}
}
