// bmp_test.go : tests uncompressed 24-bit BMP encoding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package convert

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// A 3x2 image exercises row padding: 3 pixels are 9 bytes, padded up to 12.
func TestEncodeBMPHeader(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	out := encodeBMP(img)

	const stride = 12
	wantSize := bmpFileHeaderSize + bmpInfoHeaderSize + stride*2
	if len(out) != wantSize {
		t.Fatalf("len = %d, want %d", len(out), wantSize)
	}
	if out[0] != 'B' || out[1] != 'M' {
		t.Fatalf("missing BM signature")
	}
	if got := binary.LittleEndian.Uint32(out[2:6]); int(got) != wantSize {
		t.Errorf("file size field = %d, want %d", got, wantSize)
	}
	if got := binary.LittleEndian.Uint32(out[10:14]); got != bmpFileHeaderSize+bmpInfoHeaderSize {
		t.Errorf("pixel offset = %d, want %d", got, bmpFileHeaderSize+bmpInfoHeaderSize)
	}

	info := out[bmpFileHeaderSize:]
	if got := binary.LittleEndian.Uint32(info[0:4]); got != bmpInfoHeaderSize {
		t.Errorf("info header size = %d, want %d", got, bmpInfoHeaderSize)
	}
	if got := binary.LittleEndian.Uint32(info[4:8]); got != 3 {
		t.Errorf("width = %d, want 3", got)
	}
	if got := binary.LittleEndian.Uint32(info[8:12]); got != 2 {
		t.Errorf("height = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint16(info[14:16]); got != bmpBitsPerPixel {
		t.Errorf("bits per pixel = %d, want %d", got, bmpBitsPerPixel)
	}
}

// Rows run bottom to top and pixels are stored blue, green, red.
func TestEncodeBMPPixelOrder(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255}) // top row
	img.Set(0, 1, color.NRGBA{R: 40, G: 50, B: 60, A: 255}) // bottom row
	out := encodeBMP(img)

	data := out[bmpFileHeaderSize+bmpInfoHeaderSize:]
	const stride = 4 // 3 bytes rounded up to a 4-byte boundary
	bottom := data[0:3]
	top := data[stride : stride+3]

	if bottom[0] != 60 || bottom[1] != 50 || bottom[2] != 40 {
		t.Errorf("first stored row = %v, want the image's bottom row as BGR", bottom)
	}
	if top[0] != 30 || top[1] != 20 || top[2] != 10 {
		t.Errorf("second stored row = %v, want the image's top row as BGR", top)
	}
}

// BMP has no alpha channel, so a transparent pixel comes out fuchsia.
func TestEncodeBMPTransparentKeysToFuchsia(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 0})
	out := encodeBMP(img)

	pixel := out[bmpFileHeaderSize+bmpInfoHeaderSize:]
	if pixel[0] != keyB || pixel[1] != keyG || pixel[2] != keyR {
		t.Fatalf("transparent pixel = %v, want fuchsia as BGR", pixel[:3])
	}
}
