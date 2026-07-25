// research_test.go : tests allocation, progress, and research levels.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

// TestResearchTurnsPositive checks the turns to complete grows with level and
// shrinks with a bigger factor (faster research).
func TestResearchTurnsPositive(t *testing.T) {
	t0 := ResearchTurns(0, 16)
	t1 := ResearchTurns(1, 16)
	if t0 <= 0 || t1 <= 0 {
		t.Fatalf("turns must be positive: t0=%d t1=%d", t0, t1)
	}
	if t1 <= t0 {
		t.Errorf("higher level should need more turns: t0=%d t1=%d", t0, t1)
	}
	if ResearchTurns(0, 32) >= t0 {
		t.Errorf("bigger factor should research faster")
	}
}

// TestResearchAllocationAndLevel checks the allocation/turns bookkeeping and a
// level up via NewTurn.
func TestResearchAllocationAndLevel(t *testing.T) {
	var topic ResearchTopic
	factor := ResearchFactors[ResearchAttack]

	// No allocation: NewTurn does nothing.
	if topic.NewTurn() {
		t.Fatal("idle topic should not level up")
	}

	// Allocate 1: turns to complete is set.
	topic.SetAllocation(factor, 1)
	if topic.Allocation != 1 || topic.TurnsToComplete <= 0 {
		t.Fatalf("after alloc 1: alloc=%d ttc=%d", topic.Allocation, topic.TurnsToComplete)
	}

	// Allocate enough labs to finish in one turn, then advance.
	topic.SetAllocation(factor, topic.TurnsToComplete) // labs >= turns left
	if !topic.NewTurn() {
		t.Fatal("topic with full allocation should level up in one turn")
	}
	if topic.Level != 1 || topic.TurnsToComplete != 0 {
		t.Errorf("after level-up: level=%d ttc=%d", topic.Level, topic.TurnsToComplete)
	}

	// Setting allocation to zero clears the pending turns.
	topic.SetAllocation(factor, 0)
	if topic.Allocation != 0 {
		t.Errorf("alloc should be 0, got %d", topic.Allocation)
	}
}
