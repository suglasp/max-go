// gameover_credits_test.go : guards the upgrades row unit word of the game over screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestGoldSpentSuffix checks Credit for exactly 1 and Credits otherwise.
func TestGoldSpentSuffix(t *testing.T) {
	ctx := &Context{Assets: &assets.Assets{Strings: assets.Strings{"bd00": "Credit", "7423": "Credits"}}}
	cases := []struct {
		amount int
		want   string
	}{
		{0, " Credits"},
		{1, " Credit"},
		{2, " Credits"},
		{60, " Credits"},
	}
	for _, c := range cases {
		if got := goldSpentSuffix(ctx, c.amount); got != c.want {
			t.Errorf("goldSpentSuffix(%d) = %q, want %q", c.amount, got, c.want)
		}
	}
}

// TestLangEnUsHasCreditStrings checks the shipped ini carries the Credit / Credits keys.
func TestLangEnUsHasCreditStrings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "langres", "lang-en-us.ini"))
	if err != nil {
		t.Fatal(err)
	}
	strs, err := assets.ParseLanguage(data)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"bd00": "Credit", "7423": "Credits"} {
		if got := strs.Get(key, ""); got != want {
			t.Errorf("lang-en-us.ini %s = %q, want %q", key, got, want)
		}
	}
}
