// cursors.go : loads the M.A.X. cursor catalog from the resource archive.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : cursor.cpp / cursor.hpp - Cursor_Init, Cursor_ResourceLut, Cursor_SetCursor
//
// Package cursors exposes cursor graphics by ID. The host selects an ID for the
// current pointer context and blits its frame during redraw.
//
// Design (matches the engine, see cursor.cpp):
//   - Catalog holds one Cursor per ID, loaded once from the archive.
//   - Manager keeps the currently selected ID plus the animation frame;
//     the host calls Set(id) when the context under the pointer changes
//     and Frame()/Hotspot()/Transparent() when it draws.
//   - Cursor pixels are 8 bit palette indices and pixel 0 (data[0]) is the
//     transparent colour, so the existing gnw software cursor path
//     (System.SetCursor(buf, hotX, hotY, transparent)) draws them through
//     the active palette with no extra colour handling here.
//   - A cursor sprite taller than it is wide is a vertical strip of square
//     animation frames (frame_count = height/width); e.g. PTR_PATH is ten
//     23x23 frames. Manager.Tick advances them at ~200ms/frame.
//
// The package depends only on resource reading, image decoding, and gfx buffers.
// The host owns hit testing and decides which cursor matches the pointer context.
package cursors

import (
	"errors"
	"fmt"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sprites"
)

// ID identifies a cursor by its game context.
//
// C++: the anonymous enum in cursor.hpp (CURSOR_HIDDEN .. CURSOR_HELP).
type ID uint8

const (
	Hidden ID = iota
	Hand
	ArrowN
	ArrowNE
	ArrowE
	ArrowSE
	ArrowS
	ArrowSW
	ArrowW
	ArrowNW
	Map
	Mini
	Repair
	Transfer
	Fuel
	Reload
	Load
	Friend
	Enemy
	FarTarget
	UnitGo
	UnitNoGo
	Way
	Group
	Activate
	Map2
	Steal
	Disable
	Path
	Help
	// Count is the number of cursor types.
	Count = iota
)

// resourceTags maps each cursor ID to its archive tag, in ID order.
//
// C++: Cursor_ResourceLut (cursor.cpp).
var resourceTags = [Count]string{
	Hidden:    "HIDDNPTR",
	Hand:      "HANDPTR",
	ArrowN:    "ARROW_N",
	ArrowNE:   "ARROW_NE",
	ArrowE:    "ARROW_E",
	ArrowSE:   "ARROW_SE",
	ArrowS:    "ARROW_S",
	ArrowSW:   "ARROW_SW",
	ArrowW:    "ARROW_W",
	ArrowNW:   "ARROW_NW",
	Map:       "MAP_PTR",
	Mini:      "MINI_PTR",
	Repair:    "FRND_FIX",
	Transfer:  "FRND_XFR",
	Fuel:      "FRND_FUE",
	Reload:    "PTR_RLD",
	Load:      "FRND_LOD",
	Friend:    "FRND_PTR",
	Enemy:     "ENMY_PTR",
	FarTarget: "PTR_FTRG",
	UnitGo:    "UNIT_GO",
	UnitNoGo:  "UNIT_NGO",
	Way:       "WAY_PTR",
	Group:     "GROUPPTR",
	Activate:  "ACTVTPTR",
	Map2:      "MAP_PTR",
	Steal:     "STEALPTR",
	Disable:   "DISBLPTR",
	Path:      "PTR_PATH",
	Help:      "PTR_HELP",
}

// Tag returns the archive tag a cursor ID loads from.
func (id ID) Tag() string {
	if int(id) < Count {
		return resourceTags[id]
	}
	return ""
}

// Cursor is one decoded cursor: its pixels for each frame (palette indices),
// hotspot, transparent colour, and animation frame count. A static cursor
// has FrameCount == 1.
type Cursor struct {
	W, H        int   // per frame dimensions
	HotX, HotY  int   // hotspot (the click point), the sprite's ulx/uly
	Transparent uint8 // the skipped colour: pixel 0 of the resource
	FrameCount  int
	pix         []uint8 // all frames stacked vertically, W x (H*FrameCount)
}

// Frame returns frame i as a W x H buffer sharing the cursor's pixels
// (not copied). Indices out of range clamp to frame 0.
func (c *Cursor) Frame(i int) *gfx.Buffer {
	if i < 0 || i >= c.FrameCount {
		i = 0
	}
	n := c.W * c.H
	off := n * i
	return gfx.FromPix(c.pix[off:off+n], c.W, c.H)
}

// Animated reports whether the cursor has more than one frame.
func (c *Cursor) Animated() bool {
	return c.FrameCount > 1
}

// decodeCursor parses a cursor resource. The bytes are an ImageSimpleHeader
// (width, height, ulx, uly) followed by the pixels; pixel 0 is the
// transparent colour. A sprite taller than it is wide is a vertical strip
// of frame_count = height/width square frames.
//
// C++: the loop over every cursor in Cursor_Init (cursor.cpp).
func decodeCursor(data []byte) (*Cursor, error) {
	s, err := sprites.DecodeSimple(data)
	if err != nil {
		return nil, err
	}
	c := &Cursor{
		W:           s.W,
		H:           s.H,
		HotX:        s.ULX,
		HotY:        s.ULY,
		Transparent: s.Pix[0],
		FrameCount:  1,
		pix:         s.Pix,
	}
	if c.W < c.H {
		c.FrameCount = c.H / c.W
		c.H = c.H / c.FrameCount // per frame height
	}
	return c, nil
}

// Reader reads a resource by tag (satisfied by resfile.Library / File).
type Reader interface {
	Read(tag string) ([]byte, error)
}

// Catalog holds every cursor by ID. A slot is nil when its resource was
// missing or failed to decode.
type Catalog struct {
	cursors [Count]*Cursor
}

// Get returns the cursor for id, or nil when absent.
func (c *Catalog) Get(id ID) *Cursor {
	if int(id) < Count {
		return c.cursors[id]
	}
	return nil
}

// Missing reports the cursor IDs that did not load.
func (c *Catalog) Missing() []ID {
	var out []ID
	for id := ID(0); int(id) < Count; id++ {
		if c.cursors[id] == nil {
			out = append(out, id)
		}
	}
	return out
}

// LoadCatalog decodes every cursor from r and retains successful entries when
// others fail. The returned error joins all decode failures. Its name avoids a
// collision with the Load cursor ID.
func LoadCatalog(r Reader) (*Catalog, error) {
	cat := &Catalog{}
	var errs []error
	for id := ID(0); int(id) < Count; id++ {
		data, err := r.Read(resourceTags[id])
		if err != nil {
			errs = append(errs, fmt.Errorf("cursors: read %s: %w", resourceTags[id], err))
			continue
		}
		cur, err := decodeCursor(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("cursors: decode %s: %w", resourceTags[id], err))
			continue
		}
		cat.cursors[id] = cur
	}
	return cat, errors.Join(errs...)
}

/* // note: ported but unused code in Go port
// defaultWindowCursorLut gives the resting cursor for each interface
// window, indexed by the engine's WindowInfo order: the tactical map area
// (index 38) shows the map cursor and its eight edge strips (39..50) show
// the matching scroll arrow; everything else rests on the hand.
//
// C++: Cursor_DefaultWindowCursorLut (cursor.cpp).
var defaultWindowCursorLut = [...]ID{
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand, Hand, Hand,
	Hand, Hand, Map, ArrowN, ArrowNE, ArrowNE,
	ArrowE, ArrowSE, ArrowSE, ArrowS, ArrowSW, ArrowSW,
	ArrowW, ArrowNW, ArrowNW, Hand, Hand, Hand,
	Hand, Hand, Hand, Hand,
}

// DefaultWindowCursor returns the resting cursor for an interface window
// index, defaulting to Hand for unknown indices.
//
// C++: Cursor_GetDefaultWindowCursor (cursor.cpp).
func DefaultWindowCursor(windowIndex int) ID {
	if windowIndex < 0 || windowIndex >= len(defaultWindowCursorLut) {
		return Hand
	}
	return defaultWindowCursorLut[windowIndex]
}
*/
