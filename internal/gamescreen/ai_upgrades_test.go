// ai_upgrades_test.go : tests the computer team's landing gold spend on unit upgrades.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// newUpgradeScreen builds a headless screen with one human and one computer team, a
// full ATTRIBS table (so every unit type has a real base value to price an upgrade
// against) and a live AI, which is what settles the computer team's strategy.
func newUpgradeScreen(t *testing.T) (*Screen, *sim.Game) {
	t.Helper()

	m := &world.Map{Surface: make([]uint8, 24*24)}
	m.W, m.H = 24, 24

	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer

	g := sim.New(p, m)
	g.State.ActiveTeam = 0
	g.State.PlayerTeam = 0

	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}
	s.ai = ai.New(func(team int, ut assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(ut, team, 0, 0)
		u.Flags = flags
		u.Values = &game.UnitValues{}
		u.InitState()

		return u
	})

	// One generic stat row for every type: enough for a real price on any attribute the
	// AI's wish list can name, whichever strategy it rolls.
	attribs := make([]assets.UnitAttributes, assets.UnitCount)
	for i := range attribs {
		attribs[i] = assets.UnitAttributes{
			TurnsToBuild: 4, HitPoints: 30, Armor: 5, Attack: 10,
			Speed: 10, Range: 4, Rounds: 1, Scan: 4,
		}
	}

	s.SetAttribs(attribs)

	return s, g
}

// TestAIBuysUpgradesAtLanding checks that a computer team spends landing credit on strategy upgrades.
func TestAIBuysUpgradesAtLanding(t *testing.T) {
	s, g := newUpgradeScreen(t)
	s.SetStartGold(200)

	orders := s.ai.InitialUpgradeOrders(g, 1)
	if len(orders) == 0 {
		t.Fatal("the computer team wants no upgrades at all")
	}

	before := map[ai.UpgradeOrder]int{}
	for _, o := range orders {
		before[o] = s.teamValues(1, o.Type).Get(o.Attrib)
	}

	s.applyAIStartUpgrades(1, []unitSpec{{t: assets.Constrct}, {t: assets.Engineer}})

	raised := 0

	for _, o := range orders {
		if s.teamValues(1, o.Type).Get(o.Attrib) > before[o] {
			raised++
		}
	}

	if raised == 0 {
		t.Fatal("the computer team spent nothing: no attribute on its wish list went up")
	}

	spent := g.TeamStats[1].GoldOnUpgrades
	if spent <= 0 {
		t.Fatalf("gold spent on upgrades = %d, want a positive tally for the score screen", spent)
	}

	if got := 200 - g.Gold[1]; got != spent {
		t.Fatalf("the reserve fell by %d but the tally says %d", got, spent)
	}
}

// TestAIUpgradesLeaveTheHumanAlone checks the spend never runs for a player team: the
// human's credits are spent on their own purchase screen.
func TestAIUpgradesLeaveTheHumanAlone(t *testing.T) {
	s, g := newUpgradeScreen(t)
	s.SetStartGold(200)

	s.applyAIStartUpgrades(0, []unitSpec{{t: assets.Tank}})

	if g.Gold[0] != 0 {
		t.Fatalf("the human team was handed %d gold behind the purchase screen's back", g.Gold[0])
	}

	if got := g.TeamStats[0].GoldOnUpgrades; got != 0 {
		t.Fatalf("the human team bought %d gold of upgrades by itself", got)
	}
}

// TestNoStartCreditBuysNothing checks a game that never set a landing credit (a quick
// start bench without -startcredit) leaves the computer team exactly as it was.
func TestNoStartCreditBuysNothing(t *testing.T) {
	s, g := newUpgradeScreen(t)

	s.applyAIStartUpgrades(1, []unitSpec{{t: assets.Tank}})

	if g.Gold[1] != 0 || g.TeamStats[1].GoldOnUpgrades != 0 {
		t.Fatalf("bought upgrades with no start credit: gold %d, spent %d", g.Gold[1], g.TeamStats[1].GoldOnUpgrades)
	}
}

// TestLandingBudgetChargesTheForce checks the team pays for the landing force it is
// handed, at the purchase screen's price, with the three founders free.
func TestLandingBudgetChargesTheForce(t *testing.T) {
	s, _ := newUpgradeScreen(t)
	s.SetStartGold(200)

	founders := []unitSpec{{t: assets.Constrct}, {t: assets.Engineer}, {t: assets.Surveyor}}
	if got := s.aiLandingBudget(founders); got != 200 {
		t.Fatalf("budget with only the free founders = %d, want the full 200", got)
	}

	tank := s.unitCredit(assets.Tank)
	if tank <= 0 {
		t.Fatalf("a Tank costs %d credits", tank)
	}

	// A second constructor is NOT free: only the one founder copy is.
	force := append(append([]unitSpec{}, founders...), unitSpec{t: assets.Tank}, unitSpec{t: assets.Constrct})
	want := 200 - tank - s.unitCredit(assets.Constrct)

	if got := s.aiLandingBudget(force); got != want {
		t.Fatalf("budget = %d, want %d (the tank and the second constructor are paid for)", got, want)
	}

	// A force that eats the whole credit leaves nothing to upgrade with, never a debt.
	big := make([]unitSpec, 0, 40)
	for i := 0; i < 40; i++ {
		big = append(big, unitSpec{t: assets.Tank})
	}

	if got := s.aiLandingBudget(big); got != 0 {
		t.Fatalf("budget for an unaffordable force = %d, want 0", got)
	}
}

// TestBuyUpgradesTakesTheCheapestStep checks the buy loop's rule: always the cheapest
// affordable step on the wish list, and it stops once nothing fits.
func TestBuyUpgradesTakesTheCheapestStep(t *testing.T) {
	s, g := newUpgradeScreen(t)

	orders := []ai.UpgradeOrder{
		{Type: assets.Tank, Attrib: game.AttribAttack},
		{Type: assets.Tank, Attrib: game.AttribArmor},
	}

	attack := s.upgradePrice(1, assets.Tank, game.AttribAttack)
	armor := s.upgradePrice(1, assets.Tank, game.AttribArmor)

	if attack == armor {
		t.Skip("the two steps cost the same: nothing to tell apart")
	}

	cheapest, dear := game.AttribAttack, game.AttribArmor
	price := attack

	if armor < attack {
		cheapest, dear = game.AttribArmor, game.AttribAttack
		price = armor
	}

	g.Gold[1] = price // exactly one step's worth

	if spent := s.buyUpgrades(1, orders); spent != price {
		t.Fatalf("spent %d, want the one cheapest step at %d", spent, price)
	}

	if got := g.State.Teams[1].UpgradeOffset(assets.Tank, cheapest); got <= 0 {
		t.Fatalf("the cheapest attribute was not raised (offset %d)", got)
	}

	if got := g.State.Teams[1].UpgradeOffset(assets.Tank, dear); got != 0 {
		t.Fatalf("bought the dearer attribute too (offset %d) with money for one step", got)
	}

	if g.Gold[1] != 0 {
		t.Fatalf("gold left = %d, want the reserve spent down", g.Gold[1])
	}
}

// TestAISpendsItsBankedRefineryGold checks that a computer team spends refinery credit on upgrades at turn start.
func TestAISpendsItsBankedRefineryGold(t *testing.T) {
	s, g := newUpgradeScreen(t)

	orders := s.ai.TurnUpgradeOrders(g, 1)
	if len(orders) == 0 {
		t.Fatal("the computer team wants no upgrades during a game")
	}

	// A unit of the team's in the field, and a pile of gold its refineries earned.
	tank := game.NewUnitInfo(assets.Tank, 1, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = s.teamValues(1, assets.Tank)
	tank.InitState()
	g.Units.Deploy(tank, 8, 8)

	g.Gold[1] = 300

	s.buyAITurnUpgrades(1)

	if g.Gold[1] == 300 {
		t.Fatal("the computer team banked its gold and spent nothing")
	}

	if spent := g.TeamStats[1].GoldOnUpgrades; spent != 300-g.Gold[1] {
		t.Errorf("the score screen tally says %d but the reserve fell by %d", spent, 300-g.Gold[1])
	}

	raised := 0

	for _, o := range orders {
		if g.State.Teams[1].UpgradeOffset(o.Type, o.Attrib) > 0 {
			raised++
		}
	}

	if raised == 0 {
		t.Error("gold left the reserve but no attribute on the wish list went up")
	}
}

// TestBankedGoldSpendLeavesTheHumanAlone checks the turn start spend never touches a player
// team: the human's credits are theirs to spend on the Upgrade screen.
func TestBankedGoldSpendLeavesTheHumanAlone(t *testing.T) {
	s, g := newUpgradeScreen(t)
	g.Gold[0] = 300

	s.buyAITurnUpgrades(0)

	if g.Gold[0] != 300 {
		t.Fatalf("the human's reserve was spent behind their back: %d left of 300", g.Gold[0])
	}
}
