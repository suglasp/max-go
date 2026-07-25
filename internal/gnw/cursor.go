// cursor.go : draws the software cursor over the composited screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : mouse.c - GNW mouse/cursor rendering
package gnw

import "github.com/suglasp/max-go/internal/gfx"

// Software mouse cursor. The OS cursor stays hidden; GNW draws its
// own as the last composition step, exactly like mouse.c blitting the
// cursor shape over the screen.

// MouseShow makes the software cursor visible.
//
// C++: mouse_show (mouse.c)
func (s *System) MouseShow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorVisible = true
}

// MouseHide hides the software cursor.
//
// C++: mouse_hide (mouse.c)
func (s *System) MouseHide() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorVisible = false
}

// SetCursor installs a cursor shape with its hotspot; pixels of the
// transparent color are skipped (CURSOR resources use 0). The game
// replaces the default arrow once its cursor resources are loaded.
//
// C++: mouse_set_shape (mouse.c)
func (s *System) SetCursor(img *gfx.Buffer, hotX, hotY int, transparent uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorImage = img
	s.cursorHotX = hotX
	s.cursorHotY = hotY
	s.cursorTrans = transparent
}

// drawCursor blits the cursor over the composited screen; callers
// hold s.mu.
func (s *System) drawCursor(mx, my int) {
	img := s.cursorImage
	x := mx - s.cursorHotX
	y := my - s.cursorHotY

	pos := gfx.NewRect(x, y, x+img.W-1, y+img.H-1)
	clipped, ok := pos.Intersect(gfx.NewRect(0, 0, s.screen.W-1, s.screen.H-1))
	if !ok {
		return
	}
	src := img.Sub(clipped.ULX-pos.ULX, clipped.ULY-pos.ULY, clipped.Width()+1, clipped.Height()+1)
	dst := s.screen.SubRect(clipped)
	for sy := 0; sy < src.H; sy++ {
		srow := src.Row(sy)
		drow := dst.Pix[sy*dst.Stride : sy*dst.Stride+src.W]
		for sx, c := range srow {
			if c != s.cursorTrans {
				drow[sx] = c
			}
		}
	}
}

// cursorTransparentDefault marks unused pixels of the built in arrow;
// it must differ from black (0) so the outline can be black.
const cursorTransparentDefault = 7

// defaultCursor builds a classic arrow (white fill, black outline)
// used until the game installs a CURSOR resource. Color 255 is white
// and 0 black in the game palette.
func defaultCursor() (img *gfx.Buffer, hotX, hotY int, transparent uint8) {
	shape := []string{
		"1.......",
		"11......",
		"121.....",
		"1221....",
		"12221...",
		"122221..",
		"1222221.",
		"12222221",
		"1221221.",
		"11.1221.",
		"1...121.",
		".....1..",
	}
	b := gfx.New(8, 12)
	b.Fill(cursorTransparentDefault)
	for y, row := range shape {
		for x := 0; x < len(row); x++ {
			switch row[x] {
			case '2':
				b.Set(x, y, 255) // fill
			case '1':
				b.Set(x, y, 0) // outline
			}
		}
	}
	return b, 0, 0, cursorTransparentDefault
}
