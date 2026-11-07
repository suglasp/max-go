// scenario_test.go : testing unit for scenario.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package missionfile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestListScenarios(t *testing.T) {
	dir := dataDir(t)
	missions, err := ListScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 24 {
		t.Fatalf("ListScenarios returned %d scenarios, want 24", len(missions))
	}

	// Entries follow their numeric suffix and carry a decoded header and title.
	for i, m := range missions {
		if m.Index != i+1 {
			t.Errorf("missions[%d].Index = %d, want %d", i, m.Index, i+1)
		}
		if m.Info == nil {
			t.Errorf("missions[%d] (%s) has nil Info", i, filepath.Base(m.SavePath))
		}
		if m.Title == "" {
			t.Errorf("missions[%d] (%s) has empty Title", i, filepath.Base(m.SavePath))
		}
	}

	// SAVE1.SCE pairs with DESCR1.SCE, whose briefing starts with "Objective:".
	first := missions[0]
	if !strings.HasPrefix(first.Description, "Objective:") {
		t.Fatalf("scenario 1 briefing = %q, want it to start with %q", first.Description, "Objective:")
	}
}

func TestListCampaign(t *testing.T) {
	dir := dataDir(t)
	missions, err := ListCampaign(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Only SAVE1.CAM through SAVE9.CAM are missions. INTRO and WIN files are cinematics.
	if len(missions) != 9 {
		t.Fatalf("ListCampaign returned %d missions, want 9", len(missions))
	}
	for i, m := range missions {
		if m.Index != i+1 {
			t.Errorf("missions[%d].Index = %d, want %d", i, m.Index, i+1)
		}
		if m.Info == nil || m.Title == "" {
			t.Errorf("missions[%d] (%s) missing header/title", i, filepath.Base(m.SavePath))
		}
		if filepath.Base(m.SavePath) == "INTRO1.CAM" || filepath.Base(m.SavePath) == "WIN9.CAM" {
			t.Errorf("cinematic file leaked into the campaign list: %s", m.SavePath)
		}
	}
	if !strings.HasPrefix(missions[0].Description, "Objective:") {
		t.Fatalf("campaign 1 briefing = %q, want it to start with %q", missions[0].Description, "Objective:")
	}
}

func TestListTraining(t *testing.T) {
	dir := dataDir(t)
	missions, err := ListTraining(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 15 {
		t.Fatalf("ListTraining returned %d missions, want 15", len(missions))
	}
	for i, m := range missions {
		if m.Index != i+1 || m.Info == nil || m.Title == "" {
			t.Errorf("missions[%d] (%s) bad: index=%d info=%v title=%q", i, filepath.Base(m.SavePath), m.Index, m.Info != nil, m.Title)
		}
	}
}

func TestListMultiScenarios(t *testing.T) {
	dir := dataDir(t)
	missions, err := ListMultiScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 12 {
		t.Fatalf("ListMultiScenarios returned %d missions, want 12", len(missions))
	}
	for i, m := range missions {
		if m.Index != i+1 {
			t.Errorf("missions[%d].Index = %d, want %d", i, m.Index, i+1)
		}
		if m.Info == nil {
			t.Errorf("missions[%d] (%s) has nil Info", i, filepath.Base(m.SavePath))
		}
		if m.Info.Category != CategoryMultiPlayerScenario {
			t.Errorf("missions[%d] category = %d, want %d", i, m.Info.Category, CategoryMultiPlayerScenario)
		}
	}
}

func TestListDemos(t *testing.T) {
	dir := dataDir(t)
	missions, err := ListDemos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 {
		t.Fatalf("ListDemos returned %d demos, want 1", len(missions))
	}
	demo := missions[0]
	if demo.Index != 1 {
		t.Fatalf("demo Index = %d, want 1", demo.Index)
	}
	if filepath.Base(demo.SavePath) != "SAVE1.DMO" {
		t.Fatalf("demo SavePath = %q, want SAVE1.DMO", filepath.Base(demo.SavePath))
	}
	if demo.Info == nil {
		t.Fatal("demo has nil Info")
	}
	if demo.Info.Category != CategoryDemo {
		t.Fatalf("demo category = %d, want %d", demo.Info.Category, CategoryDemo)
	}
}

func TestReadDescription(t *testing.T) {
	dir := dataDir(t)
	text, err := ReadDescription(filepath.Join(dir, "DESCR1.SCE"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "Objective:") {
		t.Fatalf("DESCR1.SCE = %q, want it to start with %q", text, "Objective:")
	}
	// ReadDescription removes trailing NUL bytes and whitespace.
	if text != strings.TrimRight(text, "\x00 \t\r\n") {
		t.Error("ReadDescription left trailing padding")
	}

	// A missing DESCR path produces an empty description.
	if got, err := ReadDescription(""); err != nil || got != "" {
		t.Fatalf("ReadDescription(\"\") = %q, %v; want \"\", nil", got, err)
	}
}
