// resources_test.go : tests resource loading helpers shared by menu screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestPreGameArtDecodes verifies every image resource the pre game
// menus draw exists in the retail archives and decodes.
func TestPreGameArtDecodes(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	bigs := []string{"MAINPIC", "CHOSPLYR", "CLANSEL", "OPTNFRM", "HELPFRAM"}
	for _, tag := range bigs {
		data, err := lib.Read(tag)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if _, err := sprites.DecodeBig(data); err != nil {
			t.Errorf("%s: %v", tag, err)
		}
	}

	simples := []string{
		"CH_HUM_U", "CH_HUM_D", "CH_CMP_U", "CH_CMP_D", "CH_NON_U", "CH_NON_D",
		"MNUBTN3U", "MNUBTN3D", "MNUBTN4U", "MNUBTN4D",
		"MNUBTN5U", "MNUBTN5D", "MNUBTN6U", "MNUBTN6D",
		"HELPOK_U", "HELPOK_D", "XFRCAN_U", "XFRCAN_D",
		"BLDUP__U", "BLDUP__D", "BLDDWN_U", "BLDDWN_D",
		"PREFEDIT", "HANDPTR",
	}
	for i := 1; i <= 4; i++ {
		simples = append(simples,
			fmt.Sprintf("CH_TM%d_U", i), fmt.Sprintf("CH_TM%d_D", i))
	}
	for i := 1; i <= 8; i++ {
		simples = append(simples,
			fmt.Sprintf("CH_CN%d_U", i), fmt.Sprintf("CH_CN%d_D", i))
	}
	for i := 0; i <= 8; i++ {
		simples = append(simples, fmt.Sprintf("CLN%dLOGO", i))
	}
	for _, tag := range simples {
		data, err := lib.Read(tag)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if _, err := sprites.DecodeSimple(data); err != nil {
			t.Errorf("%s: %v", tag, err)
		}
	}
}

// TestPreGameStringsExist checks the language keys used by the pre-game screens
// on are present in the retail lang_english.ini.
func TestPreGameStringsExist(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	a, err := assets.Load(lib, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{
		// submenus
		"04b5", "182b", "02c9", "3d29", "884f", "878a", "7eec",
		"f2b3", "c7a9", "7064", "f731", "bf00", "847e", "e13e",
		// choose player
		"5512", "55c8", "5985", "9ffa", "76ed", "dc93", "bda5",
		"4042", "9605", "ef0a", "bc94", "06ec",
		"f394", "a8a6", "a3ee", "319d",
		// clan select
		"897f", "7e91", "f5ee", "eb1b", "aff0",
		// game config
		"8448", "de1b", "e96a", "4599", "1d15", "e1e4", "3b7b",
		"fde2", "e644", "2d88", "8b31", "a2bb", "9c54",
		"779f", "99b8", "366c", "03d2", "8f89", "218f", "f7e4", "0ba9",
		"dbc7", "f793", "e512", "a724", "c4eb", "3ade",
		"d558", "10c4", "4ef9", "48bd",
		"94df", "8e58", "dca9", "2484", "291d", "3099",
		"298d", "9689", "f5d6", "9427", "d1d8", "76b7", "b459", "4065",
		"a07a", "7015", "a80a", "8610", "da62",
		"1dc3", "eb1c", "0e00", "2de0", "e22c", "7cb8",
	}
	for _, k := range keys {
		if a.Strings.Get(k, "") == "" {
			t.Errorf("lang key %s missing", k)
		}
	}
}
