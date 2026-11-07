// language_test.go : testing unit for language/localisation helpers in the settings package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageValidation(t *testing.T) {
	if Default().Language != "" {
		t.Fatalf("default language = %q, want empty (no key, texts from LANG.RES)", Default().Language)
	}
	// Validation preserves every bundled language.
	for _, l := range SupportedLanguages {
		s := Default()
		s.Language = l
		s.Validate()
		if s.Language != l {
			t.Errorf("supported %q clamped to %q", l, s.Language)
		}
	}
	// Unsupported values count as absent, and empty stays empty.
	for _, l := range []string{"english", "fr", "de", "russian", "klingon", ""} {
		s := Default()
		s.Language = l
		s.Validate()
		if s.Language != "" {
			t.Errorf("Validate(%q) = %q, want empty", l, s.Language)
		}
	}
}

// TestLanguageKeyOptional checks that a default config writes no language key, and that a key added by hand
// survives the load and save round trip the game does at every start.
func TestLanguageKeyOptional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")
	if _, err := LoadOrCreate(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"language"`) {
		t.Fatalf("a default max-go.json carries a language key:\n%s", data)
	}

	enabled := strings.Replace(string(data), `"data_dir": "",`, "\"data_dir\": \"\",\n  \"language\": \"en-us\",", 1)
	if err := os.WriteFile(path, []byte(enabled), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Language != LangEnglish {
		t.Errorf("an enabled key loads as %q, want en-us", s.Language)
	}
	if data, err = os.ReadFile(path); err != nil || !strings.Contains(string(data), `"language": "en-us"`) {
		t.Errorf("the rewrite dropped the enabled language key (err %v):\n%s", err, data)
	}
}
