// convert.go : turns unpacked RES resources into inspectable files.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/restool"
	"github.com/suglasp/max-go/internal/sprites"
)

// rgb555Count is the size of COLOR.PAL's RGB555 nearest index table that
// follows the 768-byte palette; used here only to recognise a resource
// shaped like COLOR.PAL.
const rgb555Count = 32768

// Format selects the image encoder.
type Format int

const (
	// FormatBMP writes an uncompressed 24-bit BMP. It has no alpha channel,
	// so transparent pixels come out fuchsia.
	FormatBMP Format = iota
	// FormatPNG writes a PNG and keeps real transparency.
	FormatPNG
)

// ParseFormat maps a command line format name onto a Format.
func ParseFormat(name string) (Format, bool) {
	switch strings.ToLower(name) {
	case "bmp":
		return FormatBMP, true
	case "png":
		return FormatPNG, true
	}
	return FormatBMP, false
}

// Ext is the file extension images of this format are written with.
func (f Format) Ext() string {
	if f == FormatPNG {
		return ".png"
	}
	return ".bmp"
}

/* // note: unused code in Go port
// String names the format for messages and flag errors.
func (f Format) String() string {
	if f == FormatPNG {
		return "png"
	}
	return "bmp"
}
*/

// alreadyConverted lists the extensions written by this tool so a later walk
// can skip its own output.
var alreadyConverted = map[string]bool{
	".bmp": true, ".png": true, ".txt": true, ".wav": true,
	".ttf": true, ".otf": true, ".ttc": true,
}

// FileStatus is what became of one file in a ConvertDir walk.
type FileStatus int

const (
	// StatusConverted means the file produced at least one output.
	StatusConverted FileStatus = iota
	// StatusAlreadyConverted means the file is output from an earlier run.
	StatusAlreadyConverted
	// StatusFailed means nothing could be made of the file.
	StatusFailed
)

// FileResult reports one file of a ConvertDir walk to the caller, which is free
// to print it as it goes rather than wait for the whole directory.
type FileResult struct {
	Source  string   // file name, relative to the walked directory
	Outputs []string // what was written, in order; empty unless converted
	Status  FileStatus
	Err     error // why it failed; nil otherwise
}

// Ext names the output type of a converted file, without its leading dot.
func (r FileResult) Ext() string {
	if len(r.Outputs) == 0 {
		return ""
	}
	return strings.TrimPrefix(filepath.Ext(r.Outputs[0]), ".")
}

// Options controls a conversion pass over a directory of assets.
type Options struct {
	// Palette colours simple images and sprites with multiple frames, which
	// carry no palette of their own (big images and palette resources bring
	// their own colours). When nil a grayscale ramp is used.
	Palette *gfx.Palette
	// DumpPalette also writes a 256x1 colour strip for Palette into the
	// output directory.
	DumpPalette bool
	// Format picks the image encoder.
	Format Format
	// OnFile, when set, is called for every file the walk touches, as it is
	// touched. A ConvertDir run is otherwise silent.
	OnFile func(FileResult)
}

// Report counts a ConvertDir walk.
type Report struct {
	Converted   int // files that produced output
	AlreadyDone int // files this tool had already written on an earlier run
	Failed      int // files nothing could be made of
}

// ConvertDir walks every file in dir and converts it in place, writing the
// result beside the original. Anything unpacked from a RES archive becomes an
// image, a font, or a .txt; a .MSC or .WAV becomes a .wav; a .FLC or .MVE
// becomes numbered frames, a movie also yielding its sound track.
//
// Files this tool wrote on an earlier run are recognised by extension and left
// alone, and one file that cannot be converted does not stop the walk. The
// originals are never touched, so a directory unpacked by max-restool still
// packs back into an archive identical bit for bit.
func ConvertDir(dir string, opt Options) (Report, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Report{}, err
	}

	report := func(result FileResult) {
		if opt.OnFile != nil {
			opt.OnFile(result)
		}
	}

	var rep Report
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == restool.ManifestName {
			continue
		}
		if alreadyConverted[strings.ToLower(filepath.Ext(name))] {
			rep.AlreadyDone++
			report(FileResult{Source: name, Status: StatusAlreadyConverted})
			continue
		}

		outputs, err := ConvertFile(filepath.Join(dir, name), dir, opt)
		if err != nil {
			rep.Failed++
			report(FileResult{Source: name, Status: StatusFailed, Err: err})
			continue
		}
		rep.Converted++
		report(FileResult{Source: name, Outputs: outputs, Status: StatusConverted})
	}

	if opt.DumpPalette && opt.Palette != nil {
		out := encode(paletteStripFromPalette(opt.Palette), opt.Format)
		name := "_palette" + opt.Format.Ext()
		if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// ConvertResource classifies one resource by content and selected tag hints. It
// returns the output suffix, encoded bytes, and whether conversion succeeded.
// Strict checks leave unrecognized resources unchanged with ok set to false.
func ConvertResource(tag string, data []byte, pal *gfx.Palette, format Format) (suffix string, out []byte, ok bool) {
	up := strings.ToUpper(tag)
	imgExt := format.Ext()

	// Fonts: gated by tag (FONT0..6, FONT_1, ...). GNW bitmap .FON renders to
	// a glyph image; the modern port instead ships TrueType/OpenType fonts in
	// PATCHES.RES, which are already usable as they are, so emit them verbatim
	// with the right extension.
	if strings.HasPrefix(up, "FONT") {
		if f, err := fonts.Decode(data); err == nil {
			return imgExt, encode(fontStrip(f), format), true
		}
		if ext, ok := sfntExt(data); ok {
			return ext, data, true
		}
	}

	// Palette resources: a bare 256x3 table, or a file shaped like COLOR.PAL
	// (palette + RGB555 table, optionally + the NEWC intensity table).
	// Sizes are matched exactly so large images are not mistaken for one.
	switch len(data) {
	case 768, 768 + rgb555Count, 768 + rgb555Count + 4 + 65536:
		if img, ok := paletteStripFromBytes(data); ok {
			return imgExt, encode(img, format), true
		}
	}

	// Simple image: exact 8 + w*h bytes (a strong signature resistant to
	// collisions). Big images have ulx/uly (often 0) at offset 0, so they
	// fail this and fall through.
	if s, ok := trySimple(data); ok {
		img := indexedImage(s.W, s.H, s.Pix, nil, int(s.Pix[0]), pal)
		return imgExt, encode(img, format), true
	}

	// Big image: own palette + RLE that fills exactly w*h.
	if b, err := sprites.DecodeBig(data); err == nil && b.W <= 640 && b.H <= 480 {
		img := indexedImage(b.W, b.H, b.Pix, nil, -1, &b.Palette)
		return imgExt, encode(img, format), true
	}

	// Sprite or shadow with multiple frames (S_ prefix). Rendered as one strip.
	if strings.HasPrefix(up, "S_") {
		if m, err := sprites.DecodeMultiShadow(data); err == nil && multiNonEmpty(m) {
			return imgExt, encode(multiStrip(m, pal, true), format), true
		}
	}
	if m, err := sprites.DecodeMultiSprite(data); err == nil && multiNonEmpty(m) {
		return imgExt, encode(multiStrip(m, pal, false), format), true
	}

	// Text and sound filename references (e.g. "main_msc.msc").
	if txt, ok := asText(data); ok {
		return ".txt", []byte(txt), true
	}

	return "", nil, false
}

// trySimple validates a resource as a simple image by its exact size
// without emitting the decoder's error on the common path where it does not match.
func trySimple(data []byte) (*sprites.Simple, bool) {
	if len(data) < 8 {
		return nil, false
	}
	w := int(int16(binary.LittleEndian.Uint16(data[0:2])))
	h := int(int16(binary.LittleEndian.Uint16(data[2:4])))
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 || len(data) != 8+w*h {
		return nil, false
	}
	s, err := sprites.DecodeSimple(data)
	if err != nil {
		return nil, false
	}
	return s, true
}

// sfntExt recognises a font wrapped in SFNT by its 4-byte version tag and
// returns the matching file extension. The bytes are written verbatim.
func sfntExt(data []byte) (string, bool) {
	if len(data) < 12 {
		return "", false
	}
	switch string(data[0:4]) {
	case "\x00\x01\x00\x00", "true", "typ1":
		return ".ttf", true
	case "OTTO":
		return ".otf", true
	case "ttcf":
		return ".ttc", true
	}
	return "", false
}

func multiNonEmpty(m *sprites.Multi) bool {
	for i := range m.Frames {
		if m.Frames[i].W > 0 && m.Frames[i].H > 0 {
			return true
		}
	}
	return false
}

// indexedImage paints w*h palette indices into an image. A pixel is
// transparent when opaque is not nil and false for it, or when its index
// equals transIdx (use -1 for a fully opaque image).
func indexedImage(w, h int, pix []uint8, opaque []bool, transIdx int, pal *gfx.Palette) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		idx := pix[i]
		if (opaque != nil && !opaque[i]) || (transIdx >= 0 && int(idx) == transIdx) {
			continue // leave fully transparent
		}
		r, g, b := pal.RGBA8(int(idx))
		img.Pix[i*4+0] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 255
	}
	return img
}

// paletteImage paints w*h indices through a raw 768-byte RGB table. Movie and
// animation frames carry their own palette this way, already at full 8-bit.
func paletteImage(w, h int, pix []byte, pal []byte) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h && i < len(pix); i++ {
		idx := int(pix[i]) * 3
		if idx+2 >= len(pal) {
			continue
		}
		img.Pix[i*4+0] = pal[idx+0]
		img.Pix[i*4+1] = pal[idx+1]
		img.Pix[i*4+2] = pal[idx+2]
		img.Pix[i*4+3] = 255
	}
	return img
}

// multiStrip lays every frame that is not empty left to right into one image,
// frame heights aligned at the top. Sprite frames use the palette; shadow
// frames render their silhouette as solid black on transparent.
func multiStrip(m *sprites.Multi, pal *gfx.Palette, shadow bool) image.Image {
	totalW, maxH := 0, 0
	for i := range m.Frames {
		f := &m.Frames[i]
		if f.W <= 0 || f.H <= 0 {
			continue
		}
		totalW += f.W
		if f.H > maxH {
			maxH = f.H
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, totalW, maxH))
	startX := 0
	for i := range m.Frames {
		f := &m.Frames[i]
		if f.W <= 0 || f.H <= 0 {
			continue
		}
		for y := 0; y < f.H; y++ {
			for x := 0; x < f.W; x++ {
				if !f.Opaque[y*f.W+x] {
					continue
				}
				var r, g, b uint8
				if shadow {
					r, g, b = 0, 0, 0
				} else {
					r, g, b = pal.RGBA8(int(f.Pix[y*f.W+x]))
				}
				o := img.PixOffset(startX+x, y)
				img.Pix[o+0] = r
				img.Pix[o+1] = g
				img.Pix[o+2] = b
				img.Pix[o+3] = 255
			}
		}
		startX += f.W
	}
	return img
}

// fontStrip renders the printable ASCII glyphs (32..126) in a row, white
// on transparent, separated by one pixel.
func fontStrip(f *fonts.Font) image.Image {
	const lo, hi = 32, 126
	glyphOf := func(c byte) *fonts.Glyph {
		if int(c) < len(f.Glyphs) {
			return &f.Glyphs[c]
		}
		return nil
	}
	total := 0
	for c := lo; c <= hi; c++ {
		w := 0
		if g := glyphOf(byte(c)); g != nil {
			w = g.Width
		}
		if w == 0 {
			w = f.Spacing
		}
		total += w + 1
	}
	if total <= 0 {
		total = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, total, f.Height))
	startX := 0
	for c := lo; c <= hi; c++ {
		g := glyphOf(byte(c))
		w := 0
		if g != nil {
			w = g.Width
			for y := 0; y < f.Height; y++ {
				for x := 0; x < w; x++ {
					if g.Pixel(x, y) {
						o := img.PixOffset(startX+x, y)
						img.Pix[o+0] = 255
						img.Pix[o+1] = 255
						img.Pix[o+2] = 255
						img.Pix[o+3] = 255
					}
				}
			}
		}
		if w == 0 {
			w = f.Spacing
		}
		startX += w + 1
	}
	return img
}

// paletteStripFromBytes renders the first 256 RGB entries of a palette
// resource as a 256x1 strip (one pixel per colour). Components are taken
// as 6-bit (scaled x4) unless any value exceeds 63, in which case they
// are treated as full 8-bit.
func paletteStripFromBytes(data []byte) (image.Image, bool) {
	if len(data) < 768 {
		return nil, false
	}
	comps := data[:768]
	eightBit := false
	for _, c := range comps {
		if c > 63 {
			eightBit = true
			break
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for i := 0; i < 256; i++ {
		r, g, b := comps[i*3], comps[i*3+1], comps[i*3+2]
		if !eightBit {
			r, g, b = r*4, g*4, b*4
		}
		o := img.PixOffset(i, 0)
		img.Pix[o+0] = r
		img.Pix[o+1] = g
		img.Pix[o+2] = b
		img.Pix[o+3] = 255
	}
	return img, true
}

// paletteStripFromPalette renders a loaded (6-bit) palette as a 256x1
// strip through its display scaling.
func paletteStripFromPalette(p *gfx.Palette) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for i := 0; i < 256; i++ {
		r, g, b := p.RGBA8(i)
		o := img.PixOffset(i, 0)
		img.Pix[o+0] = r
		img.Pix[o+1] = g
		img.Pix[o+2] = b
		img.Pix[o+3] = 255
	}
	return img
}

// asText returns the resource as a string when, after trailing NULs are
// dropped, every byte is printable ASCII or common whitespace (covers
// sound filename references and plain text resources). Resources with
// internal NULs or control bytes are treated as binary.
func asText(data []byte) (string, bool) {
	s := strings.TrimRight(string(data), "\x00")
	if s == "" {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		if c < 32 || c > 126 {
			return "", false
		}
	}
	return s, true
}

// grayscalePalette is the fallback colouring when no COLOR.PAL is
// available: index i maps to gray ~i.
func grayscalePalette() gfx.Palette {
	var p gfx.Palette
	for i := 0; i < gfx.PaletteSize; i++ {
		v := uint8(i >> 2) // 6-bit; RGBA8 scales back up x4
		p.SetRGB(i, v, v, v)
	}
	return p
}

// encode renders an image through the chosen encoder.
func encode(img image.Image, format Format) []byte {
	if format == FormatPNG {
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		return buf.Bytes()
	}
	return encodeBMP(img)
}
