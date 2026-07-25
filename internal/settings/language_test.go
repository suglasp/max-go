// language_test.go : testing unit for language/localisation helpers in the settings package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package settings

import "testing"

func TestLanguageValidation(t *testing.T) {
	if Default().Language != LangEnglish {
		t.Fatalf("default language = %q, want en-us", Default().Language)
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
	// Unsupported and empty values fall back to en-us.
	for _, l := range []string{"english", "fr", "de", "russian", "klingon", ""} {
		s := Default()
		s.Language = l
		s.Validate()
		if s.Language != LangEnglish {
			t.Errorf("Validate(%q) = %q, want en-us", l, s.Language)
		}
	}
}
