// teamcolor.go : the M.A.X. team colour palette remapper - recolours unit sprites to the owning team's palette range.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sprites

// TeamColor selects the palette ramp used for a unit's colorable pixels. Unit art reserves fixed palette bands so one sprite can represent every team.
//
// C++: HASH_TEAM_* / ResourceManager_TeamUnits* (resource_manager.cpp)
type TeamColor int

const (
	TeamRed TeamColor = iota
	TeamGreen
	TeamBlue
	TeamGray
	TeamDerelict
)

// teamRamps lists the eight palette indices used by each team.
//
// C++: ResourceManager_Team*ColorIndices (resource_manager.cpp)
var teamRamps = [...][8]uint8{
	TeamRed:      {56, 57, 58, 59, 60, 61, 62, 63},
	TeamGreen:    {32, 33, 34, 35, 36, 37, 38, 39},
	TeamBlue:     {48, 49, 50, 51, 52, 53, 54, 55},
	TeamGray:     {255, 161, 172, 169, 216, 213, 212, 207},
	TeamDerelict: {216, 215, 214, 213, 212, 211, 210, 209},
}

// TeamColorTable builds a 256-entry lookup table. It maps 32..39 and 48..63 onto the team's eight-color ramp, repeats the ramp as needed, and leaves 40..47 unchanged.
//
// C++: ResourceManager_BuildColorTables (resource_manager.cpp)
func TeamColorTable(team TeamColor) [256]uint8 {
	var t [256]uint8
	for i := range t {
		t[i] = uint8(i)
	}
	ramp := teamRamps[team]
	k := 32
	for j := 0; j < 24; j++ {
		if j == 8 {
			k += 8 // skip the 40..47 gap
		}
		t[k] = ramp[j&7]
		k++
	}
	return t
}
