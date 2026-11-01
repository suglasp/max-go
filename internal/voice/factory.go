// factory.go : the M.A.X. factory voice constants - announcer voice resource tags for factory events shared across screens.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Package voice names announcer voice resource tags that are shared across
// gameplay screens. The audio manager resolves V_F### tags to the shipped WAVs.
package voice

// Factory and builder voice tags. These are the female announcer tags used by
// the Go port; the original SoundManager call usually receives a V_M###..V_F### range.
//
// C++ ported : game_manager.cpp - GameManager_PlayUnitStatusVoice,
// GameManager_ReportNewUnitsMessage; buildmenu.cpp - FactoryBuildMenu::Build.
const (
	// The placement prompt is a two sample pool in the original (the PlayVoice span
	// runs V_M049..V_F050), so an armed tape draws one of the two at random.
	BuildPlacementPrompt  = "V_F050" // "Position tape and click inside it begin building."
	BuildPlacementPromptB = "V_F049" // "Position tape and click inside it begin building."

	FactoryBuilding = "V_F284" // "Building!"

	ConstructionCompleteA = "V_F162" // "Construction complete"
	ConstructionCompleteB = "V_F163" // "Ready for operation!"
	ConstructionCompleteC = "V_F164" // "Complete and ready!"
	ConstructionCompleteD = "V_F165" // "Building finished"

	FactoryUnitCompleteA  = "V_F166" // "Unit completed"
	FactoryUnitCompleteB  = "V_F169" // "Unit finished"
	FactorySingleNewUnitA = "V_F215" // "New unit available this turn!"
	FactorySingleNewUnitB = "V_F216" // "New unit!"
	FactorySingleNewUnitC = "V_F217" // "Unit ready!"

	FactoryMultipleNewUnitsA = "V_F206" // "New units available this turn!"
	FactoryMultipleNewUnitsB = "V_F207" // "New units!"
)

// ConstructionComplete returns the interchangeable lines for construction complete.
func ConstructionComplete() []string {
	return []string{
		ConstructionCompleteA,
		ConstructionCompleteB,
		ConstructionCompleteC,
		ConstructionCompleteD,
	}
}

// FactoryUnitComplete returns the interchangeable completion lines for a single unit.
func FactoryUnitComplete() []string {
	return []string{
		FactoryUnitCompleteA,
		FactoryUnitCompleteB,
		FactorySingleNewUnitA,
		FactorySingleNewUnitB,
		FactorySingleNewUnitC,
	}
}

// FactoryMultipleNewUnits returns the interchangeable completion lines for multiple units.
func FactoryMultipleNewUnits() []string {
	return []string{
		FactoryMultipleNewUnitsA,
		FactoryMultipleNewUnitsB,
	}
}
