// options_test.go : tests option bindings and control layout.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/settings"
)

// TestSliderValue checks the mouse x to value mapping (and clamping) used by
// the Preferences sliders.
func TestSliderValue(t *testing.T) {
	// track at x=100, width 200, range 0..100
	cases := []struct{ mx, want int }{
		{100, 0}, {150, 25}, {200, 50}, {300, 100},
		{90, 0},    // left of track clamps low
		{500, 100}, // right of track clamps high
	}
	for _, c := range cases {
		if got := sliderValue(c.mx, 100, 200, 100); got != c.want {
			t.Errorf("sliderValue(mx=%d) = %d, want %d", c.mx, got, c.want)
		}
	}
}

// TestOptionsControlsBindings checks the control table binds to the working
// copy: disable_* checkboxes invert the channel enabled flag, and setters
// write through.
func TestOptionsControlsBindings(t *testing.T) {
	ctx := &Context{} // Assets nil -> ctx.text returns fallbacks
	wc := settings.Default()
	wc.Audio.MusicEnabled = true
	wc.Audio.SfxVolume = 70
	controls := optionsControls(ctx, &wc)

	byKey := func(key string) *optControl {
		for i := range controls {
			if controls[i].key == key && controls[i].kind != optSection {
				return &controls[i]
			}
		}
		return nil
	}

	// disable_music is checked (1) only when music is OFF; music is on here.
	dm := byKey("disable_music")
	if dm == nil {
		t.Fatal("no disable_music control")
	}
	if dm.getV() != 0 {
		t.Fatalf("disable_music with music enabled = %d, want 0", dm.getV())
	}
	dm.setV(1) // check it -> music disabled
	if wc.Audio.MusicEnabled {
		t.Fatal("checking disable_music did not disable music")
	}

	// fx volume slider reads/writes the setting directly.
	fx := byKey("fx_sound_level")
	if fx == nil || fx.getV() != 70 {
		t.Fatalf("fx slider value = %v, want 70", fx)
	}
	fx.setV(40)
	if wc.Audio.SfxVolume != 40 {
		t.Fatalf("fx slider setV did not update setting: %d", wc.Audio.SfxVolume)
	}

	// The in game menu stops before the network team player section, so it
	// must NOT contain a victory edit beyond the read only label.
	if byKey("red_team_player") != nil {
		t.Fatal("in game Preferences must omit the network team section")
	}
}

// TestPick guards the label index helper.
func TestPick(t *testing.T) {
	s := []string{"a", "b"}
	if pick(s, 1) != "b" || pick(s, -1) != "" || pick(s, 9) != "" {
		t.Fatal("pick out-of-range handling wrong")
	}
}
