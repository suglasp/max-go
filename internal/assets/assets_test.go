// assets_test.go : tests the M.A.X. data table parsers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

func loadReal(t *testing.T) *Assets {
	t.Helper()
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { lib.Close() })
	a, err := Load(lib, dir, "")

	if err != nil {
		t.Fatal(err)
	}

	return a
}

func TestAttribs(t *testing.T) {
	a := loadReal(t)

	if len(a.Attribs) < UnitCount {
		t.Fatalf("got %d attribute rows", len(a.Attribs))
	}

	// COMMTWR row golden, visible in the resource header comment:
	// 12 56 08 00 00 00 00 00 00 03 00 00 00
	ct := a.Attribs[CommTwr]
	if ct.TurnsToBuild != 12 || ct.HitPoints != 56 || ct.Armor != 8 || ct.Scan != 3 {
		t.Errorf("COMMTWR = %+v", ct)
	}

	if ct.Attack != 0 || ct.Speed != 0 {
		t.Errorf("COMMTWR should not attack/move: %+v", ct)
	}

	// TANK golden row from the retail data:
	// 4 turns, 24 hits, 10 armor, 16 attack, speed 6, range 4,
	// 2 rounds, scan 4, ammo 14.
	tank := a.Attribs[Tank]
	want := UnitAttributes{TurnsToBuild: 4, HitPoints: 24, Armor: 10, Attack: 16, Speed: 6, Fuel: 80, Range: 4, Rounds: 2, Scan: 4, Ammo: 14}

	if tank != want {
		t.Errorf("TANK = %+v, want %+v", tank, want)
	}

	scout := a.Attribs[Scout]

	if scout.Speed <= tank.Speed {
		t.Errorf("SCOUT speed %d should exceed TANK %d", scout.Speed, tank.Speed)
	}

	if a.Attribs[Fighter].Fuel <= 0 {
		t.Errorf("FIGHTER needs fuel: %+v", a.Attribs[Fighter])
	}
}

func TestClans(t *testing.T) {
	a := loadReal(t)

	if len(a.Clans) != 8 {
		t.Fatalf("got %d clans", len(a.Clans))
	}

	wantNames := []string{"The Chosen", "Crimson Path", "Von Griffin"} // clans A, B, C

	for i, c := range a.Clans {
		if c.Name == "" {
			t.Errorf("clan %d has no name", i)
		}

		if i < len(wantNames) && c.Name != wantNames[i] {
			t.Errorf("clan %d name = %q, want %q", i, c.Name, wantNames[i])
		}
	}

	// Clan A boosts air units.
	clanA := a.Clans[0]
	found := false

	for _, u := range clanA.Upgrades {
		if u.Unit == "AWAC" && strings.Contains(u.Mods, "scan") {
			found = true
		}
	}

	if !found {
		t.Errorf("Clan A AWAC upgrade missing: %+v", clanA.Upgrades)
	}
}

func TestParseClanMods(t *testing.T) {
	cases := []struct {
		in   string
		want []ClanMod
	}{
		{"+2 scan", []ClanMod{{"scan", 2, false}}},
		{"+1 range, 7 turns", []ClanMod{{"range", 1, false}, {"turns", 7, true}}},
		{"15 turns", []ClanMod{{"turns", 15, true}}},
		// The "disable" word has no UnitValues field and is skipped.
		{"+1 scan, +1 speed, +2 disable, 8 turns", []ClanMod{{"scan", 1, false}, {"speed", 1, false}, {"turns", 8, true}}},
	}
	for _, c := range cases {
		got := ParseClanMods(c.in)
		if len(got) != len(c.want) {
			t.Errorf("ParseClanMods(%q) = %+v, want %+v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseClanMods(%q)[%d] = %+v, want %+v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestClanModsResolved(t *testing.T) {
	a := loadReal(t)

	// Clan A (The Chosen): AWAC gets +2 scan.
	scan := 0
	for _, m := range a.Clans[0].Mods[Awac] {
		if m.Attr == "scan" {
			scan += m.Delta
		}
	}
	if scan != 2 {
		t.Errorf("Clan A AWAC scan delta = %d, want 2", scan)
	}

	// "Assault Gun" is the mobile ARTILLRY (Clan F = Sacred Eights, index 5).
	if _, ok := a.Clans[5].Mods[Artillry]; !ok {
		t.Errorf("Clan F Assault Gun (Artillry) not resolved: %+v", a.Clans[5].Mods)
	}
	// Defensive "Artillery" is ARTYTRRT (Clan G = 7 Knights, index 6).
	if _, ok := a.Clans[6].Mods[ArtyTrrt]; !ok {
		t.Errorf("Clan G Artillery (ArtyTrrt) not resolved: %+v", a.Clans[6].Mods)
	}
	// "Ground Attack Plane" resolves to the Bomber (Clan A again).
	if _, ok := a.Clans[0].Mods[Bomber]; !ok {
		t.Errorf("Clan A Ground Attack Plane (Bomber) not resolved: %+v", a.Clans[0].Mods)
	}
}

func TestLanguageStrings(t *testing.T) {
	a := loadReal(t)

	tests := map[string]string{
		"35f9": "Main Menu",
		"28f9": "New Game",
		"dc41": "Load Game",
		"d17d": "Exit",
	}

	for key, want := range tests {
		if got := a.Strings.Get(key, "?"); got != want {
			t.Errorf("string %s = %q, want %q", key, got, want)
		}
	}

	if got := a.Strings.Get("zzzz", "fallback"); got != "fallback" {
		t.Errorf("missing key fallback broken: %q", got)
	}

	// '\n' = unescaping
	if v := a.Strings.Get("b9d9", ""); !strings.HasSuffix(v, "\n") || strings.Contains(v, `\n`) {
		t.Errorf("escape handling broken: %q", v)
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := ParseAttribs([]byte("1 2 3\n")); err == nil {
		t.Error("short row should fail")
	}

	if _, err := ParseAttribs([]byte(";only comments\n")); err == nil {
		t.Error("no rows should fail")
	}

	if _, err := ParseClans([]byte("Name=orphan\n")); err == nil {
		t.Error("entry before section should fail")
	}

	if _, err := ParseLanguage([]byte("[other]\nx=y\n")); err == nil {
		t.Error("missing [language] section should fail")
	}
}

func TestUnitTags(t *testing.T) {
	if Tank.Tag() != "TANK" || CommTwr.Tag() != "COMMTWR" || Waldo.Tag() != "WALDO" {
		t.Error("tag mapping broken")
	}

	if UnitType(-1).Tag() != "INVALID" || UnitType(99).Tag() != "INVALID" {
		t.Error("out of range tags")
	}

	seen := map[string]bool{}
	for _, tag := range UnitTags {
		if tag == "" || seen[tag] {
			t.Fatalf("duplicate/empty tag %q", tag)
		}

		seen[tag] = true
	}
}
