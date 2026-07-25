// language_test.go : tests localized string loading and fallback behavior.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageFile(t *testing.T) {
	cases := map[string]string{
		"en-us": "lang-en-us.ini",
		"fr":    "lang-fr.ini",
		"de":    "lang-de.ini",
		"":      "lang-en-us.ini", // empty -> en-us
	}
	for in, want := range cases {
		if got := LanguageFile(in); got != want {
			t.Errorf("LanguageFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadLanguageStringsFallback(t *testing.T) {
	dir := t.TempDir()
	binDir := t.TempDir()

	origExecutableDir := executableDir
	executableDir = func() (string, error) { return binDir, nil }
	origExtraLanguageDirs := extraLanguageDirs

	extraLanguageDirs = func() []string { return nil }

	defer func() {
		executableDir = origExecutableDir
		extraLanguageDirs = origExtraLanguageDirs
	}()

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeBin := func(name, body string) {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("lang-en-us.ini", "[language]\n0001=Hello\n")
	write("lang-fr.ini", "[language]\n0001=Bonjour\n")
	writeBin("lang-de.ini", "[language]\n0001=Guten Tag\n")

	// Requested language present
	s, err := loadLanguageStrings(dir, "fr")

	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Bonjour" {
		t.Errorf("french = %q, want Bonjour", got)
	}

	// Requested language can be supplied next to the binary.
	s, err = loadLanguageStrings(dir, "de")
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Guten Tag" {
		t.Errorf("de = %q, want Guten Tag", got)
	}

	// Missing language falls back to en-us.
	s, err = loadLanguageStrings(dir, "it")
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Hello" {
		t.Errorf("it fallback = %q, want Hello", got)
	}

	// Empty language is en-us.
	s, err = loadLanguageStrings(dir, "")

	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Hello" {
		t.Errorf("empty = %q, want Hello", got)
	}

	// The bundled en-us file next to the binary is used if dataDir lacks it.
	os.Remove(filepath.Join(dir, "lang-en-us.ini"))
	writeBin("lang-en-us.ini", "[language]\n0001=Bundled\n")

	s, err = loadLanguageStrings(dir, "it")
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Bundled" {
		t.Errorf("bundled fallback = %q, want Bundled", got)
	}

	// Neither requested nor en-us present -> error.
	emptyBin := t.TempDir()
	executableDir = func() (string, error) { return emptyBin, nil }

	if _, err := loadLanguageStrings(t.TempDir(), "fr"); err == nil {
		t.Error("expected error when no language file exists")
	}

}

// TestLanguageStringsOrFallbackMissingFile checks that missing language files
// return an empty table instead of failing startup. Callers provide their own
// English fallback to Get.
func TestLanguageStringsOrFallbackMissingFile(t *testing.T) {
	origExecutableDir := executableDir
	executableDir = func() (string, error) { return t.TempDir(), nil }
	origExtraLanguageDirs := extraLanguageDirs
	extraLanguageDirs = func() []string { return nil }

	defer func() {
		executableDir = origExecutableDir
		extraLanguageDirs = origExtraLanguageDirs
	}()

	s, err := languageStringsOrFallback(t.TempDir(), "")
	if err != nil {
		t.Fatalf("missing lang file: %v, want the empty fallback table", err)
	}

	if got := s.Get("0001", "Fallback"); got != "Fallback" {
		t.Errorf("Get on the empty table = %q, want the call site fallback", got)
	}

	// A present file still loads normally through the same path.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lang-en-us.ini"), []byte("[language]\n0001=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err = languageStringsOrFallback(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := s.Get("0001", "x"); got != "Hello" {
		t.Errorf("present file = %q, want Hello", got)
	}
}
