// transfer_test.go : tests resource-transfer bounds and direction.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import "testing"

// transferLimits mirrors the give/take/start math at the top of RunTransfer so
// the slider bounds can be checked without a window/render context.
// C++: the TransferMenu constructor (transfermenu.cpp).
func transferLimits(srcMat, srcCap, tgtMat, tgtCap int) (giveMax, takeMax, start int) {
	giveMax = min(tgtCap-tgtMat, srcMat)
	srcFree := min(srcCap-srcMat, tgtMat)
	takeMax = -srcFree
	start = giveMax
	return
}

func TestTransferLimits(t *testing.T) {
	tests := []struct {
		name                           string
		srcMat, srcCap, tgtMat, tgtCap int
		give, take                     int
	}{
		// Loaded truck (30/40) giving to a nearly empty truck (5/40): can give the
		// 30 it holds (target has room for 35); can take the target's 5.
		{"loaded to empty", 30, 40, 5, 40, 30, -5},
		// Target is the constraint: a full source can give the 10 the target has
		// room for, but being full itself it can take nothing back.
		{"target nearly full", 40, 40, 30, 40, 10, 0},
		// Source is the constraint on a take: it can only hold 10 more.
		{"source nearly full", 30, 40, 40, 40, 0, -10},
		// Both empty: nothing to move either way.
		{"both empty", 0, 40, 0, 40, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			give, take, start := transferLimits(tc.srcMat, tc.srcCap, tc.tgtMat, tc.tgtCap)
			if give != tc.give {
				t.Errorf("giveMax = %d, want %d", give, tc.give)
			}
			if take != tc.take {
				t.Errorf("takeMax = %d, want %d", take, tc.take)
			}
			if start != tc.give {
				t.Errorf("start = %d, want giveMax %d", start, tc.give)
			}
		})
	}
}

// TestTransferSliderFill checks the slider fill maps the target's resulting
// level to a fraction of the slider width (LoadHorizontalBar's proportional
// fill), so an empty target reads 0 and a full one reads the whole bar.
func TestTransferSliderFill(t *testing.T) {
	const sliderW = 223
	fill := func(value, tgtMat, tgtCap int) int {
		zeroOffset := -tgtMat
		capacity := tgtCap // freeCapacity - zeroOffset
		f := (value - zeroOffset) * sliderW / capacity
		return max(0, min(f, sliderW))
	}
	// Target empty (0/40), source takes its whole self -> 0 transfer keeps it 0.
	if got := fill(0, 0, 40); got != 0 {
		t.Errorf("empty target fill = %d, want 0", got)
	}
	// Target 0/40, give it 40 -> the bar is full.
	if got := fill(40, 0, 40); got != sliderW {
		t.Errorf("full target fill = %d, want %d", got, sliderW)
	}
	// Target 20/40 with no transfer -> half full.
	if got := fill(0, 20, 40); got != sliderW/2 {
		t.Errorf("half target fill = %d, want %d", got, sliderW/2)
	}
}
