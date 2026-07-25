// bootstrap_boot_test.go : tests bootstrap input handling and error messages.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/platform"
)

// feed pushes a string into the input line byte by byte.
func feed(s *bootScreen, text string) {
	for _, c := range []byte(text) {
		s.handleInputKey(int(c))
	}
}

func TestHandleInputKeyTypesAndSubmits(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, `D:\Games\MAX`)

	action, path := s.handleInputKey(platform.KeyReturn)
	if action != inputSubmit || path != `D:\Games\MAX` {
		t.Fatalf("submit = %d %q, want inputSubmit %q", action, path, `D:\Games\MAX`)
	}

	// The line resets after a submit.
	if len(s.input) != 0 {
		t.Fatalf("input after submit = %q, want empty", s.input)
	}
}

func TestHandleInputKeyBackspaceTrims(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, "abc")
	s.handleInputKey(platform.KeyBackspace)

	if got := string(s.input); got != "ab" {
		t.Fatalf("input = %q, want ab", got)
	}

	// Backspace on an empty line stays empty.
	s.input = nil
	s.handleInputKey(platform.KeyBackspace)
	if len(s.input) != 0 {
		t.Fatalf("input = %q, want empty", s.input)
	}
}

func TestHandleInputKeySubmitCleansPath(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, `  "C:\Program Files\MAX"  `)

	action, path := s.handleInputKey(platform.KeyReturn)
	if action != inputSubmit || path != `C:\Program Files\MAX` {
		t.Fatalf("submit = %d %q, want the dequoted trimmed path", action, path)
	}
}

func TestHandleInputKeyEmptySubmitIgnored(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, "   ")

	if action, _ := s.handleInputKey(platform.KeyReturn); action != inputNone {
		t.Fatalf("blank submit action = %d, want inputNone", action)
	}
}

func TestHandleInputKeyEscapeCancels(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	if action, _ := s.handleInputKey(platform.KeyEscape); action != inputCancel {
		t.Fatalf("escape action = %d, want inputCancel", action)
	}
}

func TestHandleInputKeyIgnoresNonPrintable(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, "ok")

	// Alt combos, function keys and other high codes never reach the line.
	for _, code := range []int{platform.AltMod | 's', platform.KeyF5, 7, 300} {
		if action, _ := s.handleInputKey(code); action != inputNone {
			t.Fatalf("code %d action = %d, want inputNone", code, action)
		}
	}

	if got := string(s.input); got != "ok" {
		t.Fatalf("input = %q, want ok", got)
	}
}

func TestHandleInputKeyCapsLength(t *testing.T) {
	s := &bootScreen{phase: bootPathInput}
	feed(s, strings.Repeat("x", inputCap+50))

	if len(s.input) != inputCap {
		t.Fatalf("input length = %d, want the %d cap", len(s.input), inputCap)
	}
}

func TestExplicitFailLinesNameAllSources(t *testing.T) {
	text := strings.Join(explicitFailLines(`E:\nowhere`), "\n")

	for _, want := range []string{`E:\nowhere`, "data_dir", "MAX_DATA_DIR", "-datadir", "Escape"} {
		if !strings.Contains(text, want) {
			t.Errorf("explicit fail screen misses %q:\n%s", want, text)
		}
	}
}

func TestHuntFailLinesKeepSpecWording(t *testing.T) {
	text := strings.Join(huntFailLines("some detail"), "\n")

	// The first line is fixed by the project spec, verbatim.
	if !strings.Contains(text, "Bootstrap failed! Install M.A.X. Gog or Steam edition") {
		t.Errorf("hunt fail screen misses the spec wording:\n%s", text)
	}
	if !strings.Contains(text, "some detail") {
		t.Errorf("hunt fail screen misses the detail line:\n%s", text)
	}
}
