// scenario.go : the M.A.X. scenario descriptor - parses the SAVE/DESCR file pairs that enumerate available missions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Mission pairs a selectable SAVE<N> file and its SmartFile header with the matching DESCR<N> briefing when one exists.
type Mission struct {
	Index       int    // Numeric suffix and display order.
	SavePath    string // Absolute SAVE path.
	DescPath    string // Absolute DESCR path, or empty.
	Title       string // SmartFile save name.
	Description string // DESCR briefing text, or empty.
	Info        *Info  // Decoded SmartFile header.
}

// saveFileRe matches SAVE<N>.<ext> without case sensitivity and captures N. Cinematic names such as INTRO1.CAM and WIN9.CAM do not match.
func saveFileRe(ext string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)^SAVE(\d+)\.` + regexp.QuoteMeta(ext) + `$`)
}

// ReadDescription reads a plain-ASCII DESCR briefing and trims trailing NUL bytes and whitespace. An empty path returns an empty description.
func ReadDescription(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\x00 \t\r\n"), nil
}

// ListScenarios returns the single player scenarios (SAVE<N>.SCE pairs) in dir.
//
// C++: MissionRegistry scanning the data root for MISSION_CATEGORY_SCENARIO.
func ListScenarios(dir string) ([]Mission, error) {
	return listMissions(dir, "SCE")
}

// ListCampaign returns the campaign missions (SAVE<N>.CAM pairs) in dir; the
// INTRO/WIN cinematic .CAM files are excluded.
//
// C++: MissionRegistry scanning the data root for MISSION_CATEGORY_CAMPAIGN.
func ListCampaign(dir string) ([]Mission, error) {
	return listMissions(dir, "CAM")
}

// ListTraining returns the training missions (SAVE<N>.TRA pairs) in dir.
//
// C++: MissionRegistry scanning the data root for MISSION_CATEGORY_TRAINING.
func ListTraining(dir string) ([]Mission, error) {
	return listMissions(dir, "TRA")
}

// ListMultiScenarios returns the "Custom Scenarios" missions (SAVE<N>.MPS pairs)
// in dir. These are the retail multiplayer scenarios; the New Game menu launches
// them as single player (the local player vs computer opponents).
//
// C++: MissionRegistry scanning for MISSION_CATEGORY_MULTI_PLAYER_SCENARIO.
func ListMultiScenarios(dir string) ([]Mission, error) {
	return listMissions(dir, "MPS")
}

// ListDemos returns the attract mode demo recordings (SAVE<N>.DMO pairs) in dir.
//
// C++: MissionRegistry scanning for MISSION_CATEGORY_DEMO.
func ListDemos(dir string) ([]Mission, error) {
	return listMissions(dir, "DMO")
}

// listMissions orders SAVE<N>.<ext> files by N, decodes each V70 header, and loads the matching DESCR file. Files with invalid headers are skipped.
func listMissions(dir, ext string) ([]Mission, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	re := saveFileRe(ext)
	var missions []Mission
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		savePath := filepath.Join(dir, entry.Name())
		info, err := readInfoFile(savePath)
		if err != nil {
			continue // Skip files that do not contain a decodable V70 save.
		}

		descPath := filepath.Join(dir, "DESCR"+m[1]+"."+ext)
		if _, err := os.Stat(descPath); err != nil {
			descPath = ""
		}
		desc, _ := ReadDescription(descPath) // A briefing read error leaves the description empty.

		title := info.Name
		if title == "" {
			title = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		}
		missions = append(missions, Mission{
			Index:       idx,
			SavePath:    savePath,
			DescPath:    descPath,
			Title:       title,
			Description: desc,
			Info:        info,
		})
	}
	sort.Slice(missions, func(i, j int) bool { return missions[i].Index < missions[j].Index })
	return missions, nil
}

// LoadMission decodes one explicit SAVE<N>.<ext> file and pairs its DESCR briefing when present. The -mission flag uses it to bypass the menus.
func LoadMission(path string) (Mission, error) {
	info, err := readInfoFile(path)
	if err != nil {
		return Mission{}, err
	}
	dir, name := filepath.Dir(path), filepath.Base(path)
	// Pair the briefing when the file follows the SAVE<N>.<ext> convention.
	idx := 0
	descPath := ""
	if m := regexp.MustCompile(`(?i)^SAVE(\d+)\.([A-Za-z]+)$`).FindStringSubmatch(name); m != nil {
		idx, _ = strconv.Atoi(m[1])
		descPath = filepath.Join(dir, "DESCR"+m[1]+"."+m[2])
		if _, err := os.Stat(descPath); err != nil {
			descPath = ""
		}
	}
	desc, _ := ReadDescription(descPath)
	// Use the file's base name when the header has no title.
	title := info.Name
	if title == "" {
		title = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return Mission{
		Index:       idx,
		SavePath:    path,
		DescPath:    descPath,
		Title:       title,
		Description: desc,
		Info:        info,
	}, nil
}

// readInfoFile opens path and decodes its SmartFile V70 header.
func readInfoFile(path string) (*Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadInfo(f)
}
