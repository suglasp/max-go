// keyname_test.go : testing unit for the keybind name parser (KeyFromName in keys.go).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package platform

import "testing"

// TestKeyFromName checks case-insensitive names, printable characters, lowercase letter folding, and invalid values.
func TestKeyFromName(t *testing.T) {
	good := []struct {
		name string
		want int
	}{
		{"Enter", KeyReturn},
		{"return", KeyReturn},
		{"ESCAPE", KeyEscape},
		{"esc", KeyEscape},
		{"Space", KeySpace},
		{"f11", KeyF11},
		{"PageUp", KeyPageUp},
		{"delete", KeyDelete},
		{"G", 'g'},
		{"g", 'g'},
		{" t ", 't'},
		{"?", '?'},
		{"=", '='},
		{"-", '-'},
		{"5", '5'},
	}

	for _, c := range good {
		got, ok := KeyFromName(c.name)

		if !ok || got != c.want {
			t.Errorf("KeyFromName(%q) = %d/%v, want %d", c.name, got, ok, c.want)
		}
	}

	for _, name := range []string{"", "  ", "no such key", "f13", "ctrl+g"} {
		if got, ok := KeyFromName(name); ok {
			t.Errorf("KeyFromName(%q) = %d, want a miss", name, got)
		}
	}
}
