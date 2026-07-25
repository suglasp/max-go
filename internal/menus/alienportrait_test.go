// alienportrait_test.go : tests alien portrait selection.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestAlienUnitPortraits locks the right click info image (P_*) mapping for the alien
// derelict faction. SUPRTPLT is the "Life Support" artifact whose info image is P_LIFESP
// (an enum misnomer; the sprite family is SUPRTP). unitPortrait must select P_LIFESP instead of the team-colored icon.
func TestAlienUnitPortraits(t *testing.T) {
	want := map[assets.UnitType]string{
		assets.SuprtPlt: "P_LIFESP", // Life Support (NOT a "Support Vehicle Plant")
		assets.ShieldGn: "P_SHIELD",
		assets.RecCentr: "P_RECCTR",
		assets.AlnTank:  "P_ALNTAN",
		assets.AlnAsGun: "P_ALNASG",
		assets.AlnPlane: "P_ALNPLA",
		assets.Juggrnt:  "P_JUGGER",
	}
	for typ, tag := range want {
		if got := unitPortrait[typ]; got != tag {
			t.Errorf("unitPortrait[%s] = %q, want %q", typ.Tag(), got, tag)
		}
	}
}
