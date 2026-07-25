// dismissclick_test.go : tests right click dismiss/deselect behavior on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/platform"
)

// TestSwallowDismissClick verifies the modal dismiss click guard: it keeps
// swallowing while the button is held, consumes the trailing release once, then
// deactivates so a fresh later click is not dropped.
func TestSwallowDismissClick(t *testing.T) {
	// Dialog returned while the button is still held: swallow, stay active.
	if swallow, active := swallowDismissClick(platform.MousePressLeft); !swallow || !active {
		t.Errorf("held press: got swallow=%v active=%v, want true,true", swallow, active)
	}

	// Button up with the trailing release this frame: swallow it, deactivate.
	if swallow, active := swallowDismissClick(platform.MouseReleaseLeft); !swallow || active {
		t.Errorf("trailing release: got swallow=%v active=%v, want true,false", swallow, active)
	}

	// Button already up, no release pending (release passed inside the modal):
	// do not swallow, deactivate - a future genuine click is unaffected.
	if swallow, active := swallowDismissClick(0); swallow || active {
		t.Errorf("settled up: got swallow=%v active=%v, want false,false", swallow, active)
	}
}
