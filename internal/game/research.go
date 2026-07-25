// research.go : tracks per-team research allocation and progress.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : research.cpp / research.hpp - ResearchTopics and upgrade logic
package game

import "math"

// Research tracks each team's progress for the eight upgradeable combat attributes. Assigned research centers reduce the remaining turns;
// completing a topic raises its level and upgrades every unit of the corresponding type.
//
// C++ ported : ctinfo.hpp - ResearchTopic + researchmenu.cpp - ResearchMenu

// ResearchTopic is one team/topic's state: achieved level, turns left at the current allocation,
// and how many research centers are assigned.
//
// C++ ported : ctinfo.hpp - ResearchTopic
type ResearchTopic struct {
	Level           int
	TurnsToComplete int
	Allocation      int
}

// Research topic indices match the first eight Attrib values, so a topic and its upgraded attribute share an index.
//
// C++ ported : enums.hpp - ResearchTopicType
const (
	ResearchAttack = iota // ATTRIB_ATTACK
	ResearchShots         // ATTRIB_ROUNDS
	ResearchRange         // ATTRIB_RANGE
	ResearchArmor         // ATTRIB_ARMOR
	ResearchHits          // ATTRIB_HITS
	ResearchSpeed         // ATTRIB_SPEED
	ResearchScan          // ATTRIB_SCAN
	ResearchCost          // ATTRIB_TURNS (build cost)
)

// ResearchFactors is the per topic difficulty divisor in the turns to complete formula (higher =
// faster). These are the original settings.cpp defaults.
//
// C++ ported : settings.cpp - *_factor settings
var ResearchFactors = [ResearchTopicCount]int{16, 16, 8, 32, 32, 16, 8, 8}

// ResearchTurns returns the turns one research center needs to advance a topic at the given factor.
//
// C++ ported : researchmenu.cpp - ResearchMenu_UpdateResearchProgress (turns_to_complete)
func ResearchTurns(level, factor int) int {
	if factor <= 0 {
		factor = 1
	}
	base := float64(level)/10.0 + 1.0
	f := float64(factor)
	return int(math.Pow(base+0.1, 7.5)*256.0/f - math.Pow(base, 7.5)*256.0/f)
}

// updateProgress adjusts the allocation and resets the remaining turns when needed.
//
// C++ ported : researchmenu.cpp - ResearchMenu_UpdateResearchProgress
func (t *ResearchTopic) updateProgress(factor, allocation int) {
	turns := ResearchTurns(t.Level, factor)
	t.Allocation += allocation
	if t.Allocation < 0 {
		t.Allocation = 0
	}
	if t.Allocation > 0 && t.TurnsToComplete == 0 {
		t.TurnsToComplete = turns
	}
	if t.Allocation == 0 && t.TurnsToComplete == turns {
		t.TurnsToComplete = 0
	}
}

// NewTurn advances the topic by one turn's research and reports whether it just gained a level.
//
// C++ ported : researchmenu.cpp - ResearchMenu_NewTurn (per topic)
func (t *ResearchTopic) NewTurn() (leveledUp bool) {
	if t.Allocation <= 0 {
		return false
	}
	t.TurnsToComplete -= t.Allocation
	if t.TurnsToComplete <= 0 {
		t.TurnsToComplete = 0
		t.Level++
		return true
	}
	return false
}

// SetAllocation changes the topic to exactly n research centers, keeping turns to complete consistent.
func (t *ResearchTopic) SetAllocation(factor, n int) {
	t.updateProgress(factor, n-t.Allocation)
}
