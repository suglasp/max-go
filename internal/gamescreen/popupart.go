// popupart.go : draws the unit popup and loads its per order button art.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sprites"
)

// drawPopup paints the order popup next to the unit in the style selected by SetPopupVanillaLook. The default retail style uses UNTBTN faces with order names overlaid.
func (s *Screen) drawPopup(buf *gfx.Buffer) {
	f := s.sys.Fonts().Get(fonts.Font2)
	hover := s.popupRowAt(s.plat.Mouse().X, s.plat.Mouse().Y)

	for i, it := range s.orderInput.popupItems {
		x := s.orderInput.popupX + 3
		y := s.orderInput.popupY + popupPad + i*popupPitch
		on := it.active || i == hover

		face, captioned := s.popupFaceFor(it.tex, on)

		if face != nil {
			// Centre 38-pixel bevel faces within the 42-pixel UNTBTN row.
			faceX := x + (popupBtnW-face.W)/2
			w := min(face.W, buf.W-faceX)
			h := min(face.H, buf.H-y)

			if w > 0 && h > 0 && faceX >= 0 && y >= 0 {
				buf.Sub(faceX, y, w, h).TransCopy(face.Sub(0, 0, w, h))
			}
		}

		if !captioned {
			continue // the bevel face already carries the order's own icon
		}

		color := colorPopupGold

		if it.active {
			color = 0x02 // green caption for a toggled on order (e.g. Sentry)
		}

		s.blitStatText(buf, x, y, popupBtnW, popupBtnH, f, it.label, color, true)
	}
}

// popupFaceFor returns a popup-row face and whether it still needs a caption. In bevel style, orders without MAX.RES art use the plain face and receive a caption.
func (s *Screen) popupFaceFor(tex string, on bool) (*gfx.Buffer, bool) {
	if s.hudArt.popupBevelArt {
		art, named := s.popupButtonFace(tex, on)

		return art, !named
	}

	if on {
		return s.hudArt.untbtnDown, true
	}

	return s.hudArt.untbtnUp, true
}

// popupButtonFace returns the up or down art for an order and whether the art belongs to that order. Missing art falls back to the plain UNTBTN plate.
func (s *Screen) popupButtonFace(tex string, on bool) (*gfx.Buffer, bool) {
	if base := popupArtBase(tex); base != "" {
		if art, ok := s.hudArt.popupArt[base]; ok {
			if on && art.down != nil {
				return art.down, true
			}

			if art.up != nil {
				return art.up, true
			}
		}
	}

	if on && s.hudArt.untbtnDown != nil {
		return s.hudArt.untbtnDown, false
	}

	return s.hudArt.untbtnUp, false
}

// popupArtBase maps an order name to its RES art tag. Orders with no dedicated art return an empty string for the plain plate fallback.
func popupArtBase(tex string) string {
	switch tex {
	case "ATTACK":
		return "ATTK"
	case "RELOAD":
		return "RLOAD"
	case "TAKEOF", "LAND", "AUTO", "MANUAL", "LAYMNE", "RMVMNE":
		return ""
	default:
		return tex
	}
}

// loadPopupButtonArt loads each popup order's up and down faces once, keyed by art tag. Tags with no usable face remain absent so the caller uses the plain plate.
func loadPopupButtonArt(lib interface{ Read(string) ([]byte, error) }) map[string]popupButtonArt {
	tags := []string{
		"BUILD", "ALLOC", "ATTK", "SNTRY", "XFER", "LOAD", "ACTVT", "CLEAR",
		"RECHG", "START", "STOP", "REPAR", "UPGRD", "BYUPG", "RLOAD",
		"PLACE", "REMVE", "ENTER", "STEAL", "DISBL", "DONE", "XFORM",
		"WAIT", "FLLOW", "RSRCH",
	}

	art := make(map[string]popupButtonArt, len(tags))

	for _, tag := range tags {
		up := LoadPopupFace(lib, tag+"_OF")
		down := LoadPopupFace(lib, tag+"_ON")

		if up != nil || down != nil {
			art[tag] = popupButtonArt{up: up, down: down}
		}
	}

	return art
}

// LoadPopupFace reads one popup button face and converts its transparent colour to zero. Missing or unreadable tags
// return nil. The live popup and the -emulate bevelart bench both draw their faces through it.
func LoadPopupFace(lib interface{ Read(string) ([]byte, error) }, tag string) *gfx.Buffer {
	data, err := lib.Read(tag)

	if err != nil {
		return nil
	}

	img, err := sprites.DecodeSimple(data)

	if err != nil {
		return nil
	}

	buf := img.Buffer().Clone()

	if buf.W > 0 && buf.H > 0 {
		if key := buf.At(0, 0); maxTransparentKey(key) {
			buf.KeyToZero(key)
		}
	}

	return buf
}

// maxTransparentKey reports whether popup art treats a palette index as transparent, including the ranges reassigned per team.
func maxTransparentKey(idx uint8) bool {
	return idx == 0 || idx == 2 || idx == 3 || (idx >= 9 && idx <= 31) || (idx >= 64 && idx <= 159)
}
