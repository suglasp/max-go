// v13_test.go : save format v13 upgrade tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// TestSaveLoadV13Fields covers the team, victory, commando-disable, and per-unit fields added by schema v13.
func TestSaveLoadV13Fields(t *testing.T) {
	g := newTestGame(t) // tank ID 1, gen ID 2 on an all land 16x16

	// Per unit v13 fields on the tank (ID 1).
	tank := g.Units.ByID.Get(1)
	if tank == nil {
		t.Fatal("setup: tank (ID 1) missing")
	}
	tank.Experience = 4
	tank.DisabledReactionFire = true
	tank.LastBuildRate = 2

	// A raised bridge (ID 3) to exercise BridgeRaise.
	bridge := game.NewUnitInfo(assets.Bridge, 0, 3, 0)
	bridge.Flags = game.FlagStationary
	bridge.Values = &game.UnitValues{}
	bridge.InitState()
	bridge.BridgeRaise = 2
	g.Units.Deploy(bridge, 5, 5)

	// A commando disabled enemy unit (ID 4): OrderDisable + a 3-turn countdown.
	cmd := game.NewUnitInfo(assets.Commando, 1, 4, 0)
	cmd.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	cmd.Values = &game.UnitValues{}
	cmd.InitState()
	cmd.Order = game.OrderDisable
	g.Units.Deploy(cmd, 10, 10)
	g.RestoreDisabled(cmd, 3)

	// Per team v13 state.
	g.State.Teams[0].Clan = 5
	g.State.Teams[0].Research[game.ResearchAttack] = game.ResearchTopic{Level: 2, TurnsToComplete: 6, Allocation: 3}
	armor := [game.AttribCount]int{}
	armor[game.AttribArmor] = 4
	g.State.Teams[0].Upgrades = map[assets.UnitType]*[game.AttribCount]int{assets.Tank: &armor}

	// Match victory rule + score graph.
	g.VictoryType = winloss.VictoryScore
	g.VictoryLimit = 50
	g.ScoreHistory[0][49] = 7
	g.ScoreHistory[1][0] = 3

	// Round trip.
	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if snap.Version != Version {
		t.Errorf("version = %d, want %d", snap.Version, Version)
	}
	g2 := snap.Restore(landMap(16, 16))

	// Per unit.
	t2 := g2.Units.ByID.Get(1)
	if t2 == nil || t2.Experience != 4 || !t2.DisabledReactionFire || t2.LastBuildRate != 2 {
		t.Errorf("tank v13 fields lost: %+v", t2)
	}
	b2 := g2.Units.ByID.Get(3)
	if b2 == nil || b2.BridgeRaise != 2 {
		t.Errorf("bridge raise lost: %+v", b2)
	}

	// Commando disable countdown.
	c2 := g2.Units.ByID.Get(4)
	if c2 == nil || g2.DisabledTurns(c2) != 3 {
		t.Errorf("disable countdown lost: unit=%v turns=%d want 3", c2, g2.DisabledTurns(c2))
	}

	// Per team.
	if g2.State.Teams[0].Clan != 5 {
		t.Errorf("clan lost: %d", g2.State.Teams[0].Clan)
	}
	if r := g2.State.Teams[0].Research[game.ResearchAttack]; r.Level != 2 || r.TurnsToComplete != 6 || r.Allocation != 3 {
		t.Errorf("research lost: %+v", r)
	}
	ups := g2.State.Teams[0].Upgrades
	if ups == nil || ups[assets.Tank] == nil || ups[assets.Tank][game.AttribArmor] != 4 {
		t.Errorf("upgrades lost: %+v", ups)
	}

	// Victory + score graph.
	if g2.VictoryType != winloss.VictoryScore || g2.VictoryLimit != 50 {
		t.Errorf("victory lost: type=%v limit=%d", g2.VictoryType, g2.VictoryLimit)
	}
	if g2.ScoreHistory[0][49] != 7 || g2.ScoreHistory[1][0] != 3 {
		t.Errorf("score history lost: %v", g2.ScoreHistory)
	}
}

// TestLoadPreV13DefaultsZero confirms a snapshot WITHOUT the v13 fields (older save)
// still restores, with the new fields at their zero values.
func TestLoadPreV13DefaultsZero(t *testing.T) {
	g := newTestGame(t)
	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Simulate an older save: clear the v13 fields, as gob would leave them.
	snap.VictoryType, snap.VictoryLimit = 0, 0
	snap.ScoreHistory = [game.TeamCount][50]int16{}
	snap.Disabled = nil
	for i := range snap.Teams {
		snap.Teams[i].Clan = 0
		snap.Teams[i].Research = [game.ResearchTopicCount]game.ResearchTopic{}
		snap.Teams[i].Upgrades = nil
	}
	g2 := snap.Restore(landMap(16, 16))
	if g2.State.Teams[0].Upgrades != nil {
		t.Errorf("pre-v13 upgrades should default nil, got %+v", g2.State.Teams[0].Upgrades)
	}
	if g2.VictoryType != winloss.VictoryDuration { // VictoryDuration == iota 0
		t.Errorf("pre-v13 victory should default to 0/duration, got %v", g2.VictoryType)
	}
}
