// icon_test.go : icon package test
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package icon

import "testing"

func TestImagesDecode(t *testing.T) {
	imgs := Images()

	if len(imgs) != 2 {
		t.Fatalf("Images() returned %d images, want 2 (embed corrupted or dropped by a failed decode)", len(imgs))
	}

	wantSizes := [2]int{32, 16}
	for i, im := range imgs {
		b := im.Bounds()
		w, h := b.Dx(), b.Dy()
		if w != wantSizes[i] || h != wantSizes[i] {
			t.Errorf("Images()[%d] = %dx%d, want %dx%d", i, w, h, wantSizes[i], wantSizes[i])
		}
	}
}
