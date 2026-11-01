// unitdescription_test.go : testing unit for the shared unit description toggle in buildmenudraw.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestUnitDescriptionToggle checks the shared button_description_rest_state: the checkbox starts on, one
// state serves every menu, the portrait overlay follows it, and the Unit Status text ignores it.
func TestUnitDescriptionToggle(t *testing.T) {
	if !unitDescriptionShown {
		t.Fatal("the Description checkbox must start on (klei button_description_rest_state = true)")
	}
	t.Cleanup(func() { unitDescriptionShown = true })

	a := &assets.Assets{Strings: assets.Strings{"d28e": "Large construction vehicle"}}
	a.UnitDescKeys[assets.Constrct] = "d28e"
	ctx := &Context{Assets: a}

	if got := ctx.unitDescription(assets.Constrct); got != "Large construction vehicle" {
		t.Errorf("Constructor description = %q, want the lang text", got)
	}
	if got := ctx.unitDescription(assets.Tank); got != "" {
		t.Errorf("a unit without a key got %q, want empty", got)
	}

	unitDescriptionShown = false // what a key 1200 click in any of the four menus does
	if got := ctx.unitDescription(assets.Constrct); got != "" {
		t.Errorf("with the checkbox off the overlay still got %q", got)
	}
	if got := ctx.unitDescriptionText(assets.Constrct); got == "" {
		t.Error("the Unit Status text must not depend on the checkbox")
	}

	if got := (&Context{}).unitDescriptionText(assets.Constrct); got != "" {
		t.Errorf("no assets: got %q, want empty", got)
	}
}
