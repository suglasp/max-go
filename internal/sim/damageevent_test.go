// damageevent_test.go : tests the non fatal hit event queue fed by attackAt.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import "testing"

// TestSurvivedHitQueuesDamageEvent checks that every non-fatal shot queues an event for impact, sound, and the "under attack!" banner.
func TestSurvivedHitQueuesDamageEvent(t *testing.T) {
	g := newGame(t, 8, 8)
	attacker := addGunTank(g, 1, 0, 2, 2)
	victim := addGunTank(g, 2, 1, 3, 2)

	g.BeginTeamTurn(0)

	res, ok := g.Attack(attacker, 3, 2)
	if !ok || res.Destroyed || res.Damage <= 0 {
		t.Fatalf("attack = %+v ok=%v, want a hit the target survives", res, ok)
	}

	events := g.DrainDamage()
	if len(events) != 1 {
		t.Fatalf("drained %d damage events, want 1", len(events))
	}

	if events[0].Victim != victim || events[0].Attacker != attacker {
		t.Errorf("event = %+v, want victim/attacker to be the two tanks", events[0])
	}

	if events[0].Amount != res.Damage {
		t.Errorf("event amount = %d, want the shot's damage %d", events[0].Amount, res.Damage)
	}

	if kills := g.DrainCasualties(); len(kills) != 0 {
		t.Errorf("a survived hit queued %d casualties, want 0", len(kills))
	}

	// The queue is emptied by the drain: the screen must not see the hit twice.
	if again := g.DrainDamage(); len(again) != 0 {
		t.Errorf("second drain returned %d events, want 0", len(again))
	}
}

// TestKillQueuesCasualtyNotDamage keeps the two queues apart: a fatal shot is a death,
// not a wound, so it must not also bloom as a survivable hit.
func TestKillQueuesCasualtyNotDamage(t *testing.T) {
	g := newGame(t, 8, 8)
	attacker := addGunTank(g, 1, 0, 2, 2)
	victim := addGunTank(g, 2, 1, 3, 2)
	victim.Hits = 1

	g.BeginTeamTurn(0)

	if res, ok := g.Attack(attacker, 3, 2); !ok || !res.Destroyed {
		t.Fatalf("attack = %+v ok=%v, want a kill", res, ok)
	}

	if kills := g.DrainCasualties(); len(kills) != 1 || kills[0].Victim != victim {
		t.Errorf("drained %d casualties, want the killed tank", len(kills))
	}

	if events := g.DrainDamage(); len(events) != 0 {
		t.Errorf("a kill queued %d damage events, want 0", len(events))
	}
}
