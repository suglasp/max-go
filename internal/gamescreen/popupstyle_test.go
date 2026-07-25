// popupstyle_test.go : covers the order popup button style switch (video.popup_vanilla_look): the uniform face and the bevel art.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

// popupStyleScreen is a bare Screen carrying just the popup art the style switch
// picks between: a uniform face for every order, and one bevel face for Sentry.
func popupStyleScreen() *Screen {
	s := &Screen{}
	s.hudArt.untbtnUp = gfx.New(42, 21)
	s.hudArt.untbtnDown = gfx.New(42, 21)
	s.hudArt.popupArt = map[string]popupButtonArt{
		"SNTRY": {up: gfx.New(38, 21), down: gfx.New(38, 21)},
	}

	return s
}

// TestPopupDefaultsToRetailFace verifies that the zero-value Screen uses the uniform popup face rather than bevel art.
func TestPopupDefaultsToRetailFace(t *testing.T) {
	s := popupStyleScreen()

	face, captioned := s.popupFaceFor("SNTRY", false)

	if face != s.hudArt.untbtnUp {
		t.Error("an unconfigured Screen must draw the retail UNTBTN face, not the bevel art")
	}

	if !captioned {
		t.Error("the retail face carries no icon, so the order name must still be written over it")
	}
}

// TestPopupVanillaLookPicksTheFace verifies that the uniform style draws captions and that an order uses available bevel art when the setting is off.
func TestPopupVanillaLookPicksTheFace(t *testing.T) {
	s := popupStyleScreen()

	s.SetPopupVanillaLook(true)

	if face, captioned := s.popupFaceFor("SNTRY", false); face != s.hudArt.untbtnUp || !captioned {
		t.Error("popup_vanilla_look=true must draw the uniform face and caption it")
	}

	s.SetPopupVanillaLook(false)

	face, captioned := s.popupFaceFor("SNTRY", false)
	if face != s.hudArt.popupArt["SNTRY"].up {
		t.Error("popup_vanilla_look=false must draw the order's own bevel face")
	}

	if captioned {
		t.Error("a bevel face already carries the order's icon and must not be captioned")
	}
}

// TestPopupPressedFaceFollowsStyle checks the pressed/active row swaps to the down
// face in both styles.
func TestPopupPressedFaceFollowsStyle(t *testing.T) {
	s := popupStyleScreen()

	s.SetPopupVanillaLook(true)

	if face, _ := s.popupFaceFor("SNTRY", true); face != s.hudArt.untbtnDown {
		t.Error("an active row must use the pressed retail face")
	}

	s.SetPopupVanillaLook(false)

	if face, _ := s.popupFaceFor("SNTRY", true); face != s.hudArt.popupArt["SNTRY"].down {
		t.Error("an active row must use the pressed bevel face")
	}
}

// TestPopupBevelFallsBackToTheUniformFace verifies that orders without bevel art use the wider uniform face and retain captions. Narrow bevel faces stay centred in their rows.
func TestPopupBevelFallsBackToTheUniformFace(t *testing.T) {
	s := popupStyleScreen()
	s.SetPopupVanillaLook(false)

	for _, order := range []string{"TAKEOF", "LAND", "AUTO", "MANUAL", "LAYMNE", "RMVMNE"} {
		face, captioned := s.popupFaceFor(order, false)

		if face != s.hudArt.untbtnUp {
			t.Errorf("%s has no bevel art in MAX.RES and must fall back to the uniform face", order)
		}

		if !captioned {
			t.Errorf("%s falls back to a blank face, so it must be captioned", order)
		}
	}
}
