// factory_test.go : testing unit for factory.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package voice

import "testing"

func TestFactoryVoiceTags(t *testing.T) {
	check := func(name string, tags []string, want []string) {
		t.Helper()
		if len(tags) != len(want) {
			t.Fatalf("%s has %d tags, want %d", name, len(tags), len(want))
		}
		for i := range want {
			if tags[i] != want[i] {
				t.Fatalf("%s[%d] = %q, want %q", name, i, tags[i], want[i])
			}
		}
	}

	if BuildPlacementPrompt != "V_F050" {
		t.Fatalf("BuildPlacementPrompt = %q, want V_F050", BuildPlacementPrompt)
	}
	if FactoryBuilding != "V_F284" {
		t.Fatalf("FactoryBuilding = %q, want V_F284", FactoryBuilding)
	}
	check("ConstructionComplete", ConstructionComplete(), []string{"V_F162", "V_F163", "V_F164", "V_F165"})
	check("FactoryUnitComplete", FactoryUnitComplete(), []string{"V_F166", "V_F169", "V_F215", "V_F216", "V_F217"})
	check("FactoryMultipleNewUnits", FactoryMultipleNewUnits(), []string{"V_F206", "V_F207"})
}
