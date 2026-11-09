// t5_combat_effects_test.go : testing unit for combat.go infiltrate and repairAt, and effects.go tickDamage
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"slices"
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// bannerIs reports whether the current banner shows exactly text.
func bannerIs(s *Screen, text string) bool {
	got, ok, _ := s.bannerText()

	return ok && got == text
}

// anyVoice reports whether one of tags was played.
func anyVoice(played []string, tags []string) bool {
	for _, v := range played {
		if slices.Contains(tags, v) {
			return true
		}
	}

	return false
}

// TestInfiltrateOutcomes pins the disable and steal results: message, voice, shot cost and the state change on the target.
func TestInfiltrateOutcomes(t *testing.T) {
	t.Run("disable success", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		s.selected = raider
		forceRoll(t, g.StealthChance(raider, victim, false), true)

		s.infiltrate(2, 1)

		if !bannerIs(s, "Unit disabled.") {
			t.Errorf("banner is not the disabled notice")
		}

		if !anyVoice(*played, voiceLines[vsUnitDisabled].tags) {
			t.Errorf("disable did not speak the disabled line, voices %v", *played)
		}

		if victim.Order != game.OrderDisable || g.DisabledTurns(victim) <= 0 {
			t.Errorf("victim order %v, disabled turns %d, want a disabled unit", victim.Order, g.DisabledTurns(victim))
		}

		if raider.Shots != 0 {
			t.Errorf("shots left = %d, want the one shot spent", raider.Shots)
		}
	})

	t.Run("steal success", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		s.selected = raider
		s.orderInput.infiltrateSteal = true
		forceRoll(t, g.StealthChance(raider, victim, true), true)

		s.infiltrate(2, 1)

		if !bannerIs(s, "Unit captured!") {
			t.Errorf("banner is not the captured notice")
		}

		if !anyVoice(*played, voiceLines[vsUnitCaptured].tags) {
			t.Errorf("steal did not speak the captured line, voices %v", *played)
		}

		if victim.Team != 0 {
			t.Errorf("victim team = %d, want it captured onto team 0", victim.Team)
		}
	})

	t.Run("missed roll", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		s.selected = raider
		forceRoll(t, g.StealthChance(raider, victim, false), false)

		s.infiltrate(2, 1)

		if !bannerIs(s, "Infiltration unsuccessful!") {
			t.Errorf("banner is not the failure notice")
		}

		if !anyVoice(*played, voiceLines[vsInfiltrateFailed].tags) {
			t.Errorf("a miss did not speak the failure line, voices %v", *played)
		}

		if victim.Order == game.OrderDisable || victim.Team != 1 {
			t.Error("a missed roll must leave the victim untouched")
		}

		if raider.Shots != 0 {
			t.Errorf("shots left = %d, a missed attempt still costs the shot", raider.Shots)
		}
	})
}

// TestInfiltrateGuards pins who may not infiltrate what: nothing happens or the attempt fails without a shot.
func TestInfiltrateGuards(t *testing.T) {
	t.Run("no selection does nothing", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		mkTank(g.Units, 1, 7, 2, 1, raidTank)

		s.infiltrate(2, 1)

		if len(*played) != 0 {
			t.Errorf("voices %v, want none", *played)
		}

		if _, ok, _ := s.bannerText(); ok {
			t.Error("a banner appeared without a selection")
		}
	})

	t.Run("a unit of another team is ignored", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		raider := raidCommando(g, 1, 1, 1) // active team is 0
		mkTank(g.Units, 0, 7, 2, 1, raidTank)
		s.selected = raider

		s.infiltrate(2, 1)

		if len(*played) != 0 || raider.Shots != 1 {
			t.Errorf("voices %v, shots %d, want silence and the shot kept", *played, raider.Shots)
		}
	})

	t.Run("empty cell fails without a shot", func(t *testing.T) {
		s, g, played := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		s.selected = raider

		s.infiltrate(2, 1)

		if !bannerIs(s, "Infiltration unsuccessful!") || !anyVoice(*played, voiceLines[vsInfiltrateFailed].tags) {
			t.Errorf("empty cell: want the failure notice and voice, voices %v", *played)
		}

		if raider.Shots != 1 {
			t.Errorf("shots = %d, an empty cell must not cost the shot", raider.Shots)
		}
	})

	t.Run("organic target fails without a shot", func(t *testing.T) {
		s, g, _ := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		victim.Flags &^= game.FlagElectronicUnit
		s.selected = raider

		s.infiltrate(2, 1)

		if raider.Shots != 1 || victim.Order == game.OrderDisable {
			t.Errorf("shots %d, victim order %v, want an untouched organic unit", raider.Shots, victim.Order)
		}
	})

	t.Run("a building cannot be stolen", func(t *testing.T) {
		s, g, _ := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		victim.Flags |= game.FlagStationary
		s.selected = raider
		s.orderInput.infiltrateSteal = true

		s.infiltrate(2, 1)

		if victim.Team != 1 || raider.Shots != 1 {
			t.Errorf("victim team %d, shots %d, want a refused steal", victim.Team, raider.Shots)
		}
	})

	t.Run("a spent commando cannot act", func(t *testing.T) {
		s, g, _ := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		raider.Shots = 0
		victim := mkTank(g.Units, 1, 7, 2, 1, raidTank)
		s.selected = raider

		s.infiltrate(2, 1)

		if victim.Order == game.OrderDisable {
			t.Error("a commando without a shot disabled a unit")
		}
	})

	t.Run("a distant target is refused", func(t *testing.T) {
		s, g, _ := newRaidScreen(t)
		raider := raidCommando(g, 0, 1, 1)
		victim := mkTank(g.Units, 1, 7, 6, 6, raidTank)
		s.selected = raider

		s.infiltrate(6, 6)

		if victim.Order == game.OrderDisable || raider.Shots != 1 {
			t.Errorf("shots %d, victim order %v, want a refused attempt", raider.Shots, victim.Order)
		}
	})
}

// repairFixture deploys a repair unit with storage next to a damaged tank of the same team.
func repairFixture(t *testing.T, storage, hits int) (*Screen, *game.UnitInfo, *game.UnitInfo, *[]string) {
	t.Helper()

	s, g, played := newRaidScreen(t)

	repairer := game.NewUnitInfo(assets.Repair, 0, 50, 0)
	repairer.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	repairer.Values = &game.UnitValues{}
	repairer.Storage = storage
	g.Units.Deploy(repairer, 3, 3)

	tank := game.NewUnitInfo(assets.Tank, 0, 51, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribHits, 80)
	tank.Hits = hits
	g.Units.Deploy(tank, 4, 3)

	s.selected = repairer

	return s, repairer, tank, played
}

// TestRepairAtMendsAndSpendsMaterial pins the repair rate (4 hits per material, rounded up) and the cap by the material carried.
func TestRepairAtMendsAndSpendsMaterial(t *testing.T) {
	cases := []struct {
		name        string
		storage     int
		hits        int
		wantHits    int
		wantStorage int
	}{
		{"enough material mends fully", 10, 50, 80, 2},
		{"rounded up cost", 10, 77, 80, 9},
		{"material caps the gain", 3, 50, 62, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, repairer, tank, played := repairFixture(t, tc.storage, tc.hits)

			s.repairAt(4, 3)

			if tank.Hits != tc.wantHits || repairer.Storage != tc.wantStorage {
				t.Errorf("hits %d storage %d, want %d / %d", tank.Hits, repairer.Storage, tc.wantHits, tc.wantStorage)
			}

			if !bannerIs(s, "Unit repaired.") {
				t.Error("banner is not the repaired notice")
			}

			if !anyVoice(*played, voiceLines[vsRepaired].tags) {
				t.Errorf("repair did not speak the repaired line, voices %v", *played)
			}
		})
	}
}

// TestRepairAtRefusals pins the cases that mend nothing and the guards that skip the whole action.
func TestRepairAtRefusals(t *testing.T) {
	t.Run("no material", func(t *testing.T) {
		s, _, tank, played := repairFixture(t, 0, 50)

		s.repairAt(4, 3)

		if tank.Hits != 50 || !bannerIs(s, "Cannot repair that unit.") || anyVoice(*played, voiceLines[vsRepaired].tags) {
			t.Errorf("hits %d, voices %v, want a refusal", tank.Hits, *played)
		}
	})

	t.Run("target at full health", func(t *testing.T) {
		s, repairer, tank, _ := repairFixture(t, 10, 80)

		s.repairAt(4, 3)

		if tank.Hits != 80 || repairer.Storage != 10 || !bannerIs(s, "Cannot repair that unit.") {
			t.Errorf("hits %d storage %d, want nothing spent", tank.Hits, repairer.Storage)
		}
	})

	t.Run("target out of reach", func(t *testing.T) {
		s, repairer, tank, _ := repairFixture(t, 10, 50)
		s.game.Units.Move(tank, 8, 8)

		s.repairAt(8, 8)

		if tank.Hits != 50 || repairer.Storage != 10 {
			t.Errorf("hits %d storage %d, want an unreached target untouched", tank.Hits, repairer.Storage)
		}
	})

	t.Run("enemy target", func(t *testing.T) {
		s, repairer, tank, _ := repairFixture(t, 10, 50)
		s.game.Units.Remove(tank)
		tank.Team = 1
		s.game.Units.Deploy(tank, 4, 3)

		s.repairAt(4, 3)

		if tank.Hits != 50 || repairer.Storage != 10 {
			t.Errorf("hits %d storage %d, want an enemy left alone", tank.Hits, repairer.Storage)
		}
	})

	t.Run("a non repair unit mends nothing", func(t *testing.T) {
		s, repairer, tank, _ := repairFixture(t, 10, 50)
		repairer.Type = assets.Tank

		s.repairAt(4, 3)

		if tank.Hits != 50 {
			t.Errorf("hits %d, a tank must not repair", tank.Hits)
		}
	})

	t.Run("no selection does nothing", func(t *testing.T) {
		s, _, tank, played := repairFixture(t, 10, 50)
		s.selected = nil

		s.repairAt(4, 3)

		if tank.Hits != 50 || len(*played) != 0 {
			t.Errorf("hits %d voices %v, want silence", tank.Hits, *played)
		}

		if _, ok, _ := s.bannerText(); ok {
			t.Error("a banner appeared without a selection")
		}
	})

	t.Run("a unit of another team is ignored", func(t *testing.T) {
		s, repairer, tank, played := repairFixture(t, 10, 50)
		repairer.Team = 1 // active team is 0

		s.repairAt(4, 3)

		if tank.Hits != 50 || len(*played) != 0 {
			t.Errorf("hits %d voices %v, want the other team ignored", tank.Hits, *played)
		}
	})
}

// damageFixture is two tanks in range of each other plus a screen that records sound effects.
type damageFixture struct {
	s       *Screen
	g       *sim.Game
	player  *game.UnitInfo
	enemy   *game.UnitInfo
	voices  []string
	sfxTags []string
}

// newDamageFixture builds the fixture: the player tank at (5,5), the enemy tank at (7,5).
func newDamageFixture(t *testing.T) *damageFixture {
	t.Helper()

	const w, h = 24, 24

	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	f := &damageFixture{g: g}
	f.player = mkTank(mgr, 0, 1, 5, 5, clickTank)
	f.enemy = mkTank(mgr, 1, 2, 7, 5, clickTank)

	f.s = &Screen{game: g, turnFlow: turnFlowState{handover: -1}}
	f.s.announcer.voiceFn = func(tag string) { f.voices = append(f.voices, tag) }
	f.s.announcer.clock = func() uint64 { return 10000 }
	f.s.sound.sfxFn = func(string, string, float64) {}
	f.s.sound.sfxPosFn = func(tag, _ string, _, _ float64) { f.sfxTags = append(f.sfxTags, tag) }

	return f
}

// TestTickDamagePresentsSurvivingHits pins the hit presentation: impact bloom, hit sound and the player warning only for the player unit.
func TestTickDamagePresentsSurvivingHits(t *testing.T) {
	t.Run("a hit on the player unit warns", func(t *testing.T) {
		f := newDamageFixture(t)
		hitTag := units.SfxTagFor(assets.Tank, game.SfxHit)

		if _, ok := f.g.Attack(f.enemy, 5, 5); !ok {
			t.Fatal("the enemy shot did not fire")
		}

		f.s.tickDamage()

		if len(f.s.effects.explosions) != 1 || f.s.effects.explosions[0].sprite != hitImpactType {
			t.Errorf("explosions = %+v, want one hit impact", f.s.effects.explosions)
		}

		if hitTag != "" && !slices.Contains(f.sfxTags, hitTag) {
			t.Errorf("sfx %v, want the hit sound %q", f.sfxTags, hitTag)
		}

		if !bannerIs(f.s, "Tank under attack!") {
			got, _, _ := f.s.bannerText()
			t.Errorf("banner = %q, want the under attack notice", got)
		}

		if !anyVoice(f.voices, voiceLines[vsUnderAttack].tags) {
			t.Errorf("voices %v, want the under attack line", f.voices)
		}

		// The queue is drained: a second pass adds nothing.
		f.s.tickDamage()

		if len(f.s.effects.explosions) != 1 {
			t.Errorf("a second pass left %d impacts, want 1", len(f.s.effects.explosions))
		}
	})

	t.Run("a hit on an enemy unit has no warning", func(t *testing.T) {
		f := newDamageFixture(t)

		if _, ok := f.g.Attack(f.player, 7, 5); !ok {
			t.Fatal("the player shot did not fire")
		}

		f.s.tickDamage()

		if len(f.s.effects.explosions) != 1 {
			t.Errorf("impacts = %d, want 1 on the struck enemy", len(f.s.effects.explosions))
		}

		if len(f.voices) != 0 {
			t.Errorf("voices %v, an enemy hit must stay silent", f.voices)
		}

		if _, ok, _ := f.s.bannerText(); ok {
			got, _, _ := f.s.bannerText()
			t.Errorf("banner %q appeared for an enemy hit", got)
		}
	})

	t.Run("a unit finished in the same frame is left to the kill path", func(t *testing.T) {
		f := newDamageFixture(t)

		if _, ok := f.g.Attack(f.enemy, 5, 5); !ok {
			t.Fatal("the enemy shot did not fire")
		}

		f.player.Hits = 0
		f.s.tickDamage()

		if len(f.s.effects.explosions) != 0 || len(f.voices) != 0 || len(f.sfxTags) != 0 {
			t.Errorf("a dead victim got impacts %d, voices %v, sfx %v", len(f.s.effects.explosions), f.voices, f.sfxTags)
		}
	})

	t.Run("no damage queued does nothing", func(t *testing.T) {
		f := newDamageFixture(t)

		f.s.tickDamage()

		if len(f.s.effects.explosions) != 0 || len(f.voices) != 0 {
			t.Errorf("an empty queue produced impacts %d voices %v", len(f.s.effects.explosions), f.voices)
		}
	})
}

// TestTickDamageStatusBands pins that the player warning voice follows the remaining health.
func TestTickDamageStatusBands(t *testing.T) {
	cases := []struct {
		name string
		hits int
		want voiceState
	}{
		{"healthy", 80, vsUnderAttack},
		{"half or less", 40, vsStatusYellow},
		{"quarter or less", 20, vsStatusRed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDamageFixture(t)
			f.player.Hits = tc.hits + 8 // the 8 point shot lands it on the band value

			if _, ok := f.g.Attack(f.enemy, 5, 5); !ok {
				t.Fatal("the enemy shot did not fire")
			}

			f.s.tickDamage()

			if f.player.Hits != tc.hits {
				t.Fatalf("test setup: hits = %d, want %d", f.player.Hits, tc.hits)
			}

			if !anyVoice(f.voices, voiceLines[tc.want].tags) {
				t.Errorf("voices %v, want the %q line", f.voices, voiceLines[tc.want].name)
			}
		})
	}
}
