// doubleclick.go : distinguishes menu double-clicks by target and time window.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

const menuDoubleClickMS uint64 = 500

// menuDoubleClick detects a repeated click on the same logical list row.
//
// C++: GNW list selectors treat a second click within the double click window
// as the list's confirm action.
type menuDoubleClick struct {
	key   int
	index int
	stamp uint64
	valid bool
}

func (d *menuDoubleClick) Click(key, index int, now uint64) bool {
	double := d.valid &&
		d.key == key &&
		d.index == index &&
		now >= d.stamp &&
		now-d.stamp <= menuDoubleClickMS
	if double {
		d.Reset()
		return true
	}
	d.key = key
	d.index = index
	d.stamp = now
	d.valid = true
	return false
}

func (d *menuDoubleClick) Reset() {
	d.valid = false
}
