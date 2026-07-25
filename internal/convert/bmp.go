// bmp.go : encodes images as uncompressed 24-bit Windows BMP files.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"encoding/binary"
	"image"
)

const (
	bmpFileHeaderSize = 14
	bmpInfoHeaderSize = 40
	bmpBitsPerPixel   = 24
)

// keyR, keyG, keyB are the fuchsia a transparent pixel takes on in a BMP.
// 24-bit BMP carries no alpha channel, and fuchsia is what M.A.X. itself
// parks in the transparent palette slot, so the choice matches the art.
const (
	keyR = 255
	keyG = 0
	keyB = 255
)

// encodeBMP writes img as an uncompressed bottom-up 24-bit BMP. Pixels that
// are not fully opaque become the fuchsia key colour: the format has no alpha,
// and silently compositing onto black would hide a sprite's dark outlines.
func encodeBMP(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Each row is padded out to a 4-byte boundary.
	stride := (w*3 + 3) &^ 3
	pixels := stride * h
	out := make([]byte, bmpFileHeaderSize+bmpInfoHeaderSize+pixels)

	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[10:14], bmpFileHeaderSize+bmpInfoHeaderSize)

	info := out[bmpFileHeaderSize:]
	binary.LittleEndian.PutUint32(info[0:4], bmpInfoHeaderSize)
	binary.LittleEndian.PutUint32(info[4:8], uint32(w))
	binary.LittleEndian.PutUint32(info[8:12], uint32(h))
	binary.LittleEndian.PutUint16(info[12:14], 1) // colour planes
	binary.LittleEndian.PutUint16(info[14:16], bmpBitsPerPixel)
	binary.LittleEndian.PutUint32(info[20:24], uint32(pixels))

	data := out[bmpFileHeaderSize+bmpInfoHeaderSize:]
	for y := 0; y < h; y++ {
		// BMP rows run bottom to top.
		row := data[(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			cr, cg, cb := uint8(r>>8), uint8(g>>8), uint8(bl>>8)
			if a>>8 < 255 {
				cr, cg, cb = keyR, keyG, keyB
			}
			// BMP stores each pixel as blue, green, red.
			row[x*3+0] = cb
			row[x*3+1] = cg
			row[x*3+2] = cr
		}
	}
	return out
}
