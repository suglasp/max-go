// popupstyle_test.go : covers video.popup_vanilla_look: its default, and that a
// max-go.json written before the key existed still loads with the retail look on.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPopupVanillaLookDefaultsOn checks that a fresh install draws the retail
// popup: the bevel art in MAX.RES is opt in, because the shipped game never used it.
func TestPopupVanillaLookDefaultsOn(t *testing.T) {
	if !Default().Video.PopupVanillaLook {
		t.Fatal("video.popup_vanilla_look must default to true (the retail popup face)")
	}
}

// TestLoadKeepsDefaultsForAbsentKeys checks that an older max-go.json retains defaults for omitted keys, including popup_vanilla_look.
func TestLoadKeepsDefaultsForAbsentKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")

	// A settings file from before the key existed: video is present, the key is not.
	old := `{
	  "language": "en-us",
	  "video": {"fullscreen": false, "window_scale": 2, "brightness": 3, "widescreen": "auto"}
	}`

	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if !s.Video.PopupVanillaLook {
		t.Error("an absent popup_vanilla_look key must keep the retail look, got the bevel art")
	}

	// Values the file DOES carry still win over the defaults.
	if s.Video.WindowScale != 2 || s.Video.Widescreen != WidescreenAuto {
		t.Errorf("file values were lost: scale=%d widescreen=%q", s.Video.WindowScale, s.Video.Widescreen)
	}
}

// TestPopupVanillaLookRoundTrips checks an explicit false survives a save and load,
// so opting into the bevel art actually sticks.
func TestPopupVanillaLookRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "max-go.json")

	s := Default()
	s.Video.PopupVanillaLook = false

	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Video.PopupVanillaLook {
		t.Error("an explicit popup_vanilla_look=false was not preserved across save/load")
	}
}
