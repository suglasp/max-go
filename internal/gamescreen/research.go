// research.go : implements research progress and the per team unit values.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// baseValues returns the base UnitValues from ATTRIBS, or a minimal value set
// with a sensible hit count if unavailable.
func (s *Screen) baseValues(t assets.UnitType) *game.UnitValues {
	if int(t) >= 0 && int(t) < len(s.buildUI.attribs) {
		return game.NewUnitValues(s.buildUI.attribs[t])
	}

	v := &game.UnitValues{}
	v.Set(game.AttribHits, 40)

	return v
}

// researchValues returns a unit's research-only values. Combat attributes gain level*base/10 and Cost reduces build turns; the result is the Upgrade menu baseline.
func (s *Screen) researchValues(team int, t assets.UnitType) *game.UnitValues {
	v := s.baseValues(t)

	if team < 0 || team >= game.TeamCount {
		return v
	}

	s.applyClanMods(v, team, t) // clan modifies the base before research stacks on it

	for topic := 0; topic < game.ResearchTopicCount; topic++ {
		level := s.game.State.Teams[team].Research[topic].Level

		if level == 0 {
			continue
		}

		if topic == game.ResearchCost {
			// Cost reduces build turns: val(n) = max(((n+10)/2 + base*10)/(n+10), 1); deltas
			// telescope to val(level), so we set turns = val(level) directly.
			base := v.Get(game.AttribTurns) // clan adjusted base build turns
			turns := ((level+10)/2 + base*10) / (level + 10)

			if turns < 1 {
				turns = 1
			}

			v.Set(game.AttribTurns, turns)

			continue
		}

		a := game.Attrib(topic) // combat topics 0..6 == attribs 0..6
		base := v.Get(a)
		v.Set(a, base+level*base/10)
	}

	return v
}

// teamValues returns a team's research baseline plus purchased per-type upgrades.
func (s *Screen) teamValues(team int, t assets.UnitType) *game.UnitValues {
	v := s.researchValues(team, t)
	if team < 0 || team >= game.TeamCount {
		return v
	}

	if up := s.game.State.Teams[team].Upgrades[t]; up != nil {
		for a := game.Attrib(0); a < game.AttribCount; a++ {
			if up[a] != 0 {
				v.Add(a, up[a])
			}
		}
	}

	return v
}

// advanceResearch runs one team research turn and refreshes unit values when a topic gains a level.
func (s *Screen) advanceResearch(team int) {
	leveled := false
	var leveledTopics []int // which topics gained a level this turn (for the message)

	for i := range s.game.State.Teams[team].Research {
		if s.game.State.Teams[team].Research[i].NewTurn() {
			leveled = true
			leveledTopics = append(leveledTopics, i)
		}
	}

	if leveled {
		for _, u := range s.game.Units.Units() {
			if u.Team == team {
				s.upgradeUnitValues(team, u)
			}
		}

		if s.isHumanTeam(team) {
			s.playVoice(vsResearch) // "Research complete"; the message below names the topic
			for _, i := range leveledTopics {
				name := s.lang.Get(researchTopicLabels[i].key, researchTopicLabels[i].fallback)
				s.showMessage(fmt.Sprintf("Research complete: %s now level %d", name, s.game.State.Teams[team].Research[i].Level))
			}
		}
	}
}

// researchLabel is a topic name: the lang_english.ini key and the ASCII fallback.
type researchLabel struct {
	key      string // string table key
	fallback string // text used when the key is missing
}

// researchTopicLabels are the lang_english.ini keys + ASCII fallbacks for the eight
// research topics, indexed by ResearchTopic (mirrors menus.researchTopicLabelKeys).
var researchTopicLabels = [game.ResearchTopicCount]researchLabel{
	{"4cd5", "Attack"}, {"7382", "Shots"}, {"86bc", "Range"}, {"a592", "Armor"},
	{"07f9", "Hits"}, {"5857", "Speed"}, {"920c", "Scan"}, {"dbae", "Cost"},
}

// soonestResearch returns the team topic that will level first and its remaining turns. Allocation is pooled across research centres.
func (s *Screen) soonestResearch(team int) (topic, turns int, ok bool) {
	best := -1

	for i := range s.game.State.Teams[team].Research {
		r := s.game.State.Teams[team].Research[i]
		if r.Allocation <= 0 || r.TurnsToComplete <= 0 {
			continue
		}

		t := (r.TurnsToComplete + r.Allocation - 1) / r.Allocation
		if best == -1 || t < turns {
			best, turns = i, t
		}
	}

	if best == -1 {
		return 0, 0, false
	}

	return best, turns, true
}

// upgradeUnitValues updates an in-service unit to team values and preserves its missing-hit-point gap. Fresh units start at full hits.
// C++ ported : unitinfo.cpp - UnitInfo::UpgradeInt.
func (s *Screen) upgradeUnitValues(team int, u *game.UnitInfo) {
	oldMax := 0
	if u.Values != nil {
		oldMax = u.Values.Get(game.AttribHits)
	}

	next := s.teamValues(team, u.Type)

	// The "Mk" version, the GetBestValues gate (unitinfo.cpp:3893-3902):
	// identical values keep the mark, a REAL change bumps it. The bookkeeping
	// fields align first so Equal (the ported operator==) compares only the
	// attributes; the assigned in service set is then marked in use like the original's
	// MarkAsInUse, which is what arms UpdateVersion.
	if u.Values != nil {
		next.SetVersion(u.Values.Version())

		if u.Values.InUse() {
			next.MarkInUse()
		}

		changed := !next.Equal(u.Values)
		next.MarkInUse()

		if changed {
			next.UpdateVersion() // the unit reads one Mk higher from now on
		}
	} else {
		next.MarkInUse()
	}

	u.Values = next
	if d := u.Values.Get(game.AttribHits) - oldMax; d > 0 {
		u.Hits += d
	}

	if max := u.Values.Get(game.AttribHits); u.Hits > max {
		u.Hits = max
	}
}

// activeResearchCenters counts the team's Research buildings not force powered off
// by the player (an idled one still counts). C++: active_research_centers (researchmenu.cpp).
func (s *Screen) activeResearchCenters(team int) int {
	n := 0

	for _, u := range s.game.Units.Units() {
		if u.Team == team && u.Type == assets.Research && u.Order != game.OrderPowerOff {
			n++
		}
	}

	return n
}

// syncResearchOrders runs the required number of research centres for the team's pooled allocation. Remaining centres idle without using power or workers.
func (s *Screen) syncResearchOrders(team int) {
	allocated := 0 // sum the pooled allocation across the eight topics

	for i := range s.game.State.Teams[team].Research {
		allocated += s.game.State.Teams[team].Research[i].Allocation
	}

	var avail []*game.UnitInfo // sorted by ID below: Units() order is random each call
	for _, u := range s.game.Units.Units() {
		if u.Team == team && u.Type == assets.Research && u.Order != game.OrderPowerOff {
			avail = append(avail, u)
		}
	}

	sort.Slice(avail, func(i, j int) bool { return avail[i].ID < avail[j].ID })

	// The stored pool must never exceed the surviving powered centres: a centre
	// lost to combat, power off or capture would otherwise keep researching as a
	// GHOST (NewTurn subtracts the STORED allocation, not this local clamp). Write
	// the clamp BACK, shrinking the highest index topics first like the research
	// menu's freeing order, via SetAllocation so TurnsToComplete stays consistent.
	// The original cannot drift here: its allocation is per building and PowerDown
	// decrements it (unitinfo.cpp:4959), so only the pooled model has to reapply the cap.
	if allocated > len(avail) {
		excess := allocated - len(avail)
		research := &s.game.State.Teams[team].Research

		for i := len(research) - 1; i >= 0 && excess > 0; i-- {
			cut := research[i].Allocation
			if cut > excess {
				cut = excess
			}

			if cut > 0 {
				research[i].SetAllocation(game.ResearchFactors[i], research[i].Allocation-cut)
				excess -= cut
			}
		}

		allocated = len(avail)
	}

	for idx, u := range avail { // first allocated centres run; the remainder idle
		if idx < allocated {
			u.Order = game.OrderPowerOn
		} else {
			u.Order = game.OrderIdle
		}

		u.State = game.OrderStateExecutingOrder
	}
}
