// doubleclick_test.go : tests menu double-click timing and target matching.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"
)

func TestMenuDoubleClick(t *testing.T) {
	var d menuDoubleClick
	if d.Click(1300, 2, 1000) {
		t.Fatal("first click reported double-click")
	}
	if !d.Click(1300, 2, 1000+menuDoubleClickMS) {
		t.Fatal("same row inside window did not report double-click")
	}
	if d.Click(1300, 2, 1000+menuDoubleClickMS+1) {
		t.Fatal("third click after a completed double-click reported another double-click")
	}

	d.Reset()
	_ = d.Click(1300, 2, 1000)
	if d.Click(1300, 3, 1100) {
		t.Fatal("different row reported double-click")
	}

	d.Reset()
	_ = d.Click(1300, 2, 1000)
	if d.Click(1301, 2, 1100) {
		t.Fatal("different list key reported double-click")
	}

	d.Reset()
	_ = d.Click(1300, 2, 1000)
	if d.Click(1300, 2, 1000+menuDoubleClickMS+1) {
		t.Fatal("expired click reported double-click")
	}
}

/* // note: unused code in Go port
func TestRunOkCancelCallbacks(t *testing.T) {
	ok, cancel := 0, 0
	if !runOkCancelCallbacks(dialog.ResultAccept, func() { ok++ }, func() { cancel++ }) {
		t.Fatal("accept returned false")
	}
	if ok != 1 || cancel != 0 {
		t.Fatalf("accept callbacks ok=%d cancel=%d, want 1/0", ok, cancel)
	}

	if runOkCancelCallbacks(dialog.ResultCancel, func() { ok++ }, func() { cancel++ }) {
		t.Fatal("cancel returned true")
	}
	if ok != 1 || cancel != 1 {
		t.Fatalf("cancel callbacks ok=%d cancel=%d, want 1/1", ok, cancel)
	}
}
*/
