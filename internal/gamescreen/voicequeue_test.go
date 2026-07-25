// voicequeue_test.go : tests the serialised announcer voice queue in voice.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestVoiceQueueSerialisesEndTurnLines verifies the fix for the fast AI overlap: the
// per team end turn voices queue and play one at a time as the channel frees, so
// three NPC turns resolving inside ~1s do not cut each other off.
func TestVoiceQueueSerialisesEndTurnLines(t *testing.T) {
	var played []string
	var now uint64
	playing := false

	s := &Screen{}
	s.announcer.voiceFn = func(tag string) { played = append(played, tag); playing = true }
	s.announcer.voicePlayingFn = func() bool { return playing }
	s.announcer.clock = func() uint64 { return now }

	// Three NPC turns end within the same instant (the fast AI bunch).
	s.teamEndedTurnVoice(1)
	s.teamEndedTurnVoice(2)
	s.teamEndedTurnVoice(3)
	if len(s.announcer.voiceQueue) != 3 {
		t.Fatalf("queued %d lines, want 3", len(s.announcer.voiceQueue))
	}

	// First drain plays exactly one line and marks the channel busy.
	now += voiceQueueMinGapMS
	s.tickVoiceQueue()
	if len(played) != 1 || played[0] != "V_F280" {
		t.Fatalf("first drain played %v, want [V_F280]", played)
	}

	// While that line is audible, no further line plays even after the gap.
	now += 10000
	s.tickVoiceQueue()
	if len(played) != 1 {
		t.Fatalf("a line played while the channel was busy: %v", played)
	}

	// Once the channel frees, the next line plays; repeat to drain the queue.
	playing = false
	now += voiceQueueMinGapMS
	s.tickVoiceQueue()
	playing = false
	now += voiceQueueMinGapMS
	s.tickVoiceQueue()

	want := []string{"V_F280", "V_F281", "V_F282"}
	if len(played) != len(want) {
		t.Fatalf("played %v, want %v", played, want)
	}
	for i := range want {
		if played[i] != want[i] {
			t.Fatalf("played %v, want %v", played, want)
		}
	}
	if len(s.announcer.voiceQueue) != 0 {
		t.Errorf("queue not fully drained: %d left", len(s.announcer.voiceQueue))
	}
}

// TestPlayerOwnEndTurnVoiceSkipped checks the single player rule: the local player's
// own "<colour> team ended turn" line is suppressed (the pressed End Turn button
// already confirms the click), while an NPC opponent's line is queued. In a
// multiplayer game every human team's line would play.
func TestPlayerOwnEndTurnVoiceSkipped(t *testing.T) {
	s, _ := simTestScreen() // team 0 = player, team 1 = computer
	s.announcer.voiceFn = func(string) {}

	s.teamEndedTurnVoice(0) // the player's own team -> suppressed (the button press confirms it)
	if len(s.announcer.voiceQueue) != 0 {
		t.Errorf("player's own end-turn voice should be suppressed, queued %d", len(s.announcer.voiceQueue))
	}
	s.teamEndedTurnVoice(1) // an NPC -> queued
	if len(s.announcer.voiceQueue) != 1 {
		t.Errorf("an NPC's end-turn voice should queue, got %d", len(s.announcer.voiceQueue))
	}
}

// TestVoiceQueueFallbackSpacing checks that, with no channel playing query wired
// (headless / a backend without it), lines are spaced by the fixed fallback delay.
func TestVoiceQueueFallbackSpacing(t *testing.T) {
	var played []string
	var now uint64

	s := &Screen{}
	s.announcer.voiceFn = func(tag string) { played = append(played, tag) }
	s.announcer.clock = func() uint64 { return now }

	s.teamEndedTurnVoice(0)
	s.teamEndedTurnVoice(1)

	// First line plays after the minimum gap.
	now += voiceQueueMinGapMS
	s.tickVoiceQueue()
	if len(played) != 1 {
		t.Fatalf("first drain played %d, want 1", len(played))
	}

	// The second is held until the fallback spacing elapses, then plays.
	now += voiceQueueMinGapMS
	s.tickVoiceQueue()
	if len(played) != 1 {
		t.Fatalf("second line played too early: %v", played)
	}
	now += voiceQueueFallbackMS
	s.tickVoiceQueue()
	if len(played) != 2 {
		t.Fatalf("second line did not play after the fallback spacing: %v", played)
	}
}
