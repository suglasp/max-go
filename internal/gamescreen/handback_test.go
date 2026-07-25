// handback_test.go : tests the end of turn voice hand back hold in gamescreen.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestEndTurnVoicesPending checks the gate the hand back waits on: a line still
// queued OR a voice still audible on the channel counts as pending; an empty queue
// with a silent (or unwired) channel is clear.
func TestEndTurnVoicesPending(t *testing.T) {
	s := &Screen{}
	if s.endTurnVoicesPending() {
		t.Error("an empty queue with no channel query should not be pending")
	}

	s.announcer.voiceQueue = []queuedVoice{{tag: "V_F280"}}
	if !s.endTurnVoicesPending() {
		t.Error("a queued line should count as pending")
	}

	s.announcer.voiceQueue = nil
	playing := true
	s.announcer.voicePlayingFn = func() bool { return playing }
	if !s.endTurnVoicesPending() {
		t.Error("an audible voice should count as pending")
	}
	playing = false
	if s.endTurnVoicesPending() {
		t.Error("a silent channel with an empty queue should be clear")
	}
}

// TestHandBackHoldsForVoicesThenGoesLive verifies the turn based fix: when the
// opponents finish but the announcer is still speaking their end of turn lines, the
// hand back to the human is HELD (aiTurn stays true, the board stays locked) until
// the voice queue drains, then the human turn goes live ("Begin!" follows).
func TestHandBackHoldsForVoicesThenGoesLive(t *testing.T) {
	s, g := simTestScreen() // team 0 = human, team 1 = computer; PlayMode is irrelevant to this path
	s.SetPlayMode(int(game.PlayTurnBased))

	// Arrange the moment just after the last opponent finished: a hand back to the
	// human is owed (handBackTeam) with aiTurn still true and one NPC line queued.
	var now uint64
	s.announcer.clock = func() uint64 { return now }
	s.turnFlow.aiTurn = true
	s.turnFlow.aiTurnTeam = 1
	s.turnFlow.handBackTeam = 0
	s.turnFlow.handBackStartMS = now
	s.announcer.voiceQueue = []queuedVoice{{tag: "V_F281"}}

	// While the line is still queued, the settle holds: it does not go live.
	s.tickComputerTurn()
	if s.turnFlow.handBackTeam != 0 || !s.turnFlow.aiTurn {
		t.Fatalf("hand-back should hold while a voice is pending (handBackTeam=%d aiTurn=%v)", s.turnFlow.handBackTeam, s.turnFlow.aiTurn)
	}

	// The queue drains: the next tick releases the hold and the human turn goes live.
	s.announcer.voiceQueue = nil
	s.tickComputerTurn()
	if s.turnFlow.handBackTeam != -1 {
		t.Errorf("hand-back should clear once voices finish, got %d", s.turnFlow.handBackTeam)
	}
	if s.turnFlow.aiTurn {
		t.Error("the human turn should be live (aiTurn cleared) after the hold releases")
	}
	if g.State.PlayerTeam != 0 || g.State.ActiveTeam != 0 {
		t.Errorf("player/active = %d/%d, want 0/0 after go-live", g.State.PlayerTeam, g.State.ActiveTeam)
	}
	if s.turnFlow.handover != -1 {
		t.Errorf("a single-human game should resume without a pass banner, handover=%d", s.turnFlow.handover)
	}
}

// TestHandBackTimeoutGoesLive checks the safety net: even if the voice channel never
// reports free, the hand back releases once handBackMaxMS elapses so a stuck channel
// can never freeze the turn.
func TestHandBackTimeoutGoesLive(t *testing.T) {
	s, _ := simTestScreen()
	s.SetPlayMode(int(game.PlayTurnBased))

	var now uint64
	s.announcer.clock = func() uint64 { return now }
	s.announcer.voicePlayingFn = func() bool { return true } // channel wedged: always "playing"
	s.turnFlow.aiTurn = true
	s.turnFlow.aiTurnTeam = 1
	s.turnFlow.handBackTeam = 0
	s.turnFlow.handBackStartMS = now

	// Still inside the window: the wedged channel holds the turn.
	now = handBackMaxMS - 1
	s.tickComputerTurn()
	if s.turnFlow.handBackTeam != 0 {
		t.Fatal("hand-back should still hold inside the timeout window")
	}

	// Past the window: the safety net releases the turn despite the stuck channel.
	now = handBackMaxMS
	s.tickComputerTurn()
	if s.turnFlow.handBackTeam != -1 || s.turnFlow.aiTurn {
		t.Errorf("the timeout should force go-live (handBackTeam=%d aiTurn=%v)", s.turnFlow.handBackTeam, s.turnFlow.aiTurn)
	}
}
