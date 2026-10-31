// langres_test.go : testing unit for langres.go and unitdesc.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/restool"
)

func TestLanguageTag(t *testing.T) {
	for code, want := range map[string]string{"": "LANG_EN", "en-us": "LANG_EN", "fr": "LANG_FR", "de-de": "LANG_DE", "pt-br": "LANG_PT"} {
		if got := LanguageTag(code); got != want {
			t.Errorf("LanguageTag(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestParseUnitDescriptions(t *testing.T) {
	keys, err := ParseUnitDescriptions([]byte("# comment\r\n\r\nCONSTRCT=D28E\r\nengineer = 52b5\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if keys[Constrct] != "d28e" || keys[Engineer] != "52b5" {
		t.Errorf("Constrct=%q Engineer=%q, want d28e and 52b5 (keys lower cased, tags any case)", keys[Constrct], keys[Engineer])
	}
	if keys[Tank] != "" {
		t.Errorf("an unlisted unit got key %q, want empty", keys[Tank])
	}
	if _, err := ParseUnitDescriptions([]byte("NOSUCHUN=1234\n")); err == nil {
		t.Error("an unknown unit tag was accepted")
	}
	if _, err := ParseUnitDescriptions([]byte("CONSTRCT d28e\n")); err == nil {
		t.Error("a line without '=' was accepted")
	}
}

// langResDirs points the language search at dir only, for the duration of a test.
func langResDirs(t *testing.T, dir string) {
	t.Helper()
	orig := languageSearchDirs
	languageSearchDirs = func(string) []string { return []string{dir} }
	t.Cleanup(func() { languageSearchDirs = orig })
}

// TestLangResSources packs the repo's langres sources into a LANG.RES and loads it the way the game does:
// the strings come from LANG_EN, every unit description key resolves to a text, and a loose ini wins.
func TestLangResSources(t *testing.T) {
	dir := t.TempDir()
	if err := restool.Pack(filepath.Join("..", "..", "langres"), filepath.Join(dir, LangResFile)); err != nil {
		t.Fatalf("packing langres: %v", err)
	}
	langResDirs(t, dir)

	langRes := openLangRes(dir)
	if langRes == nil {
		t.Fatal("LANG.RES not found in the search directory")
	}
	defer langRes.Close()

	s, err := loadStrings(dir, "en-us", langRes)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := loadUnitDescriptions(langRes)
	if err != nil {
		t.Fatal(err)
	}

	if keys[Constrct] != "d28e" {
		t.Errorf("Constrct description key = %q, want d28e", keys[Constrct])
	}
	n := 0
	for ut, key := range keys {
		if key == "" {
			continue
		}
		n++
		if s.Get(key, "") == "" {
			t.Errorf("%s description key %s has no text in LANG_EN", UnitTags[ut], key)
		}
	}
	if n != 69 {
		t.Errorf("%d units have a description key, want 69", n)
	}

	// A loose lang-en-us.ini next to LANG.RES is the override.
	if err := os.WriteFile(filepath.Join(dir, "lang-en-us.ini"), []byte("[language]\nd28e=Override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = loadStrings(dir, "en-us", langRes)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get("d28e", ""); got != "Override" {
		t.Errorf("with a loose ini d28e = %q, want Override", got)
	}
}
