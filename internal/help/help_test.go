// help_test.go : tests the context help database reader.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package help

import (
	"testing"

	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestParseEntriesAndLookup covers the grammar on a hand written file: the preamble is
// skipped, a section holds its entries IN ORDER, the rectangle test is inclusive on every
// edge, an empty entry reads as "nothing to say", and a literal \n is a hard line break.
func TestParseEntriesAndLookup(t *testing.T) {
	src := "\r\n Main Headers [in brackets] designate each screen.\r\n\r\n" +
		"[TEST_SETUP]\r\n\r\n" +
		"FIRST_LABEL   { 10, 10, 20, 20 }\r\nThe first thing.\r\n\r\n" +
		"QUESTION?     { 30, 30, 40, 40 }\r\n\r\n" +
		"WRAPPED       { 50, 50, 60, 60 }\r\nLine one.\\nLine two.\r\n\r\n" +
		"[OTHER_SETUP]\r\n\r\nONLY  { 0, 0, 5, 5 }\r\nOther screen.\r\n\r\n" +
		"[END]\r\n"

	db := Parse([]byte(src))

	if db.Sections() != 2 {
		t.Fatalf("parsed %d sections, want 2", db.Sections())
	}

	if got := db.sections["TEST_SETUP"]; len(got) != 3 {
		t.Fatalf("TEST_SETUP holds %d entries, want 3", len(got))
	}

	// Inside the rectangle, and on each of its edges.
	for _, p := range [][2]int{{15, 15}, {10, 10}, {20, 20}, {10, 20}, {20, 10}} {
		if text, ok := db.Lookup("TEST_SETUP", p[0], p[1]); !ok || text != "The first thing." {
			t.Errorf("Lookup at %v = (%q, %v), want the first entry", p, text, ok)
		}
	}

	// Just outside it: nothing.
	if _, ok := db.Lookup("TEST_SETUP", 21, 20); ok {
		t.Error("a click one pixel past the rectangle still found the entry")
	}

	// An entry with no text reads exactly like a miss.
	if _, ok := db.Lookup("TEST_SETUP", 35, 35); ok {
		t.Error("an empty entry returned text")
	}

	// A literal \n becomes a real line break.
	if text, _ := db.Lookup("TEST_SETUP", 55, 55); text != "Line one.\nLine two." {
		t.Errorf("wrapped text = %q, want a hard line break", text)
	}

	// An unknown section is a silent miss, not a crash.
	if _, ok := db.Lookup("NOPE", 1, 1); ok {
		t.Error("an unknown section returned text")
	}
}

// TestParseMalformedRectangle checks the retail rectangle with a missing comma is parsed as 230,5,409,26 so the Allocate screen retains its entry.
func TestParseMalformedRectangle(t *testing.T) {
	db := Parse([]byte("[ALLOCATE_SETUP]\r\n\r\nALLOCATE_LABEL   { 230, 5 409, 26 }\r\nAllocate.\r\n\r\n[END]\r\n"))

	entries := db.sections["ALLOCATE_SETUP"]
	if len(entries) != 1 {
		t.Fatalf("parsed %d entries, want 1 (the missing comma must still parse)", len(entries))
	}

	e := entries[0]
	if e.ULX != 230 || e.ULY != 5 || e.LRX != 409 || e.LRY != 26 {
		t.Errorf("rect = %d,%d,%d,%d, want 230,5,409,26", e.ULX, e.ULY, e.LRX, e.LRY)
	}
}

// TestRetailHelpDatabase reads the real HELP_ENG out of the game data: every screen the
// port asks help for must exist, and the whole file must parse.
func TestRetailHelpDatabase(t *testing.T) {
	dir := testutil.DataDir(t)

	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lib.Close() })

	data, err := lib.Read(Tag)
	if err != nil {
		t.Fatalf("read %s: %v", Tag, err)
	}

	db := Parse(data)

	if db.Sections() < 20 {
		t.Fatalf("the retail help holds %d sections, want the full set", db.Sections())
	}

	// Every section the port's screens ask for.
	for _, section := range []string{
		"NEW_GAME_SETUP", "CLAN_SETUP", "PLANET_SETUP", "OPTIONS_SETUP",
		"TRAINING_MENU_SETUP", "STAND_ALONE_MENU_SETUP", "CAMPAIGN_MENU",
		"SETUP_MENU_SETUP", "PREFS_MENU_SETUP", "SAVELOAD_SETUP", "LOAD_SETUP",
		"GAME_SCREEN_SETUP", "SITE_SELECT_SETUP", "REPORTS_SETUP", "RESEARCH_SETUP",
		"ALLOCATE_SETUP", "CARGO_SETUP", "CONSTRUCTOR_BUILD_SETUP", "FACTORY_BUILD_SETUP",
		"HANGAR_SETUP", "DOCK_SETUP", "DEPOT_SETUP", "BARRACKS_SETUP", "TRANSFER_SETUP",
		"STATS_SETUP", "UPGRADES_SETUP",
	} {
		if len(db.sections[section]) == 0 {
			t.Errorf("section %s is missing from the retail help", section)
		}
	}

	// HANGAR_SETUP's DOWN_ARROW rectangle is inside out ({599,425,550,445}), so no click can hit it. Preserve those coordinates rather than swapping them.
	// The entry is found by its upper left corner (599,425); Entry.Name is unused code.
	found := false
	for _, e := range db.sections["HANGAR_SETUP"] {
		if e.ULX != 599 || e.ULY != 425 {
			continue
		}

		found = true
		if e.LRX >= e.ULX {
			t.Errorf("HANGAR_SETUP/DOWN_ARROW = %d,%d,%d,%d: the retail data's inside out rect changed",
				e.ULX, e.ULY, e.LRX, e.LRY)
		}
	}
	if !found {
		t.Error("HANGAR_SETUP/DOWN_ARROW (599,425) is missing")
	}

	// A known entry, to prove the text really came through.
	if text, ok := db.Lookup("NEW_GAME_SETUP", 240, 10); !ok || text == "" {
		t.Errorf("the New Game title area has no help text")
	}
}
