// mission.go : defines the mission category a running game belongs to.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : mission.hpp - MissionCategory
package game

// MissionCategory identifies the running game type. Training missions limit the computer to a passive tier, trim the buildable roster, and use
// the "position the tape" build prompt. Custom skirmishes and development benches use MissionNone.
//
// C++ ported : mission.hpp - MISSION_CATEGORY_*
type MissionCategory uint8

const (
	MissionNone          MissionCategory = iota // no mission: a custom game or a dev bench
	MissionTraining                             // the .TRA teaching missions
	MissionScenario                             // a standalone .SCE scenario
	MissionCampaign                             // a .CAM story mission
	MissionMultiScenario                        // a .MPS multiplayer scenario, played solo here
	MissionDemo                                 // a .DMO demo playing itself at the main menu
)

// String names the category for debug logs and the save file.
func (c MissionCategory) String() string {
	switch c {
	case MissionTraining:
		return "training"

	case MissionScenario:
		return "scenario"

	case MissionCampaign:
		return "campaign"

	case MissionMultiScenario:
		return "multi scenario"

	case MissionDemo:
		return "demo"

	default:
		return "none"
	}
}

// IsMission reports whether a scripted mission is running rather than a free game.
func (c MissionCategory) IsMission() bool {
	return c != MissionNone
}
