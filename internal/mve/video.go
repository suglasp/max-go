// video.go : the M.A.X. MVE video chunk decoder - decodes delta compressed video blocks into 8-bit palette frames.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package mve

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type videoState struct {
	wBlocks, hBlocks int
	w, h             int
	/* // note: unused code in Go port
	// stride was only read by the commented out mode 10 / unpack4 decoders.
	stride           int
	*/
	cur, prev []byte
	conf      [256]int
	lut2      [256]uint16
	lut3      [256]uint16
	/* // note: unused code in Go port
	// lut4 / lut5 / lut6 were only read by the commented out mode 10 / unpack4 decoders.
	lut4             [16]uint32
	lut5             [256]uint32
	lut6             [256]uint32
	*/
}

// config allocates frame buffers for block-based MVE video geometry.
func (v *videoState) config(widthBlocks, heightBlocks, stride int) error {
	if widthBlocks <= 0 || heightBlocks <= 0 || stride <= 0 {
		return fmt.Errorf("mve: invalid video geometry %dx%d stride %d", widthBlocks, heightBlocks, stride)
	}
	v.wBlocks, v.hBlocks = widthBlocks, heightBlocks /* , v.stride = stride: note: unused code in Go port, see videoState */
	v.w = widthBlocks * 8
	v.h = heightBlocks * 8 * stride
	v.cur = make([]byte, v.w*v.h)
	v.prev = make([]byte, v.w*v.h)
	for i := 0; i < 128; i++ {
		v.conf[i] = i * v.w
	}
	for i := 128; i < 256; i++ {
		v.conf[i] = (i - 256) * v.w
	}
	v.initLUTs()
	return nil
}

// advance swaps the current and previous buffers for inter-frame modes.
func (v *videoState) advance() {
	v.cur, v.prev = v.prev, v.cur
}

// C++: nfPkDecomp (mvelib32.c)
func (v *videoState) decomp(decodingMap, data []byte, posX, posY, width, height int) error {
	if v.w == 0 || v.h == 0 {
		return errors.New("mve: video buffers not allocated")
	}
	if posX < 0 || posY < 0 || width < 0 || height < 0 || posX+width > v.wBlocks || posY+height > v.hBlocks {
		return fmt.Errorf("mve: video rectangle outside frame x=%d y=%d w=%d h=%d", posX, posY, width, height)
	}
	mapPos, dataPos := 0, 0
	for by := 0; by < height; by++ {
		for bx := 0; bx < width; bx += 2 {
			if mapPos >= len(decodingMap) {
				return errors.New("mve: decoding map truncated")
			}
			codeByte := decodingMap[mapPos]
			mapPos++
			for j := 0; j < 2 && bx+j < width; j++ {
				code := int(codeByte & 0x0f)
				if j == 1 {
					code = int(codeByte >> 4)
				}
				blockX := posX + bx + j
				blockY := posY + by
				used, err := v.decodeBlock(code, data[dataPos:], blockX, blockY)
				if err != nil {
					return fmt.Errorf("mve: block (%d,%d) mode %d data offset %d: %w", blockX, blockY, code, dataPos, err)
				}
				dataPos += used
			}
		}
	}
	return nil
}

// decodeBlock applies one MVE 8x8 block mode and returns the number of source bytes consumed.
func (v *videoState) decodeBlock(code int, data []byte, bx, by int) (int, error) {
	dst := by*8*v.w + bx*8
	switch code {
	case 0:
		return 0, v.copyBlock(dst, v.prev, dst)
	case 1:
		return 0, nil
	case 2, 3:
		if len(data) < 1 {
			return 0, errBlockShort(code)
		}
		vec := int(v.lut3[data[0]])
		if code == 3 {
			vec = ((-vec) & 0xff) | (((-(vec >> 8)) & 0xff) << 8)
		}
		off := int(int8(byte(vec))) + v.conf[(vec>>8)&0xff]
		return 1, v.copyBlock(dst, v.cur, dst+off)
	case 4:
		if len(data) < 1 {
			return 0, errBlockShort(code)
		}
		vec := int(v.lut2[data[0]])
		off := int(int8(byte(vec))) + v.conf[(vec>>8)&0xff]
		return 1, v.copyBlock(dst, v.prev, dst+off)
	case 5:
		if len(data) < 2 {
			return 0, errBlockShort(code)
		}
		vec := int(binary.LittleEndian.Uint16(data))
		off := int(int8(byte(vec))) + v.conf[(vec>>8)&0xff]
		return 2, v.copyBlock(dst, v.prev, dst+off)
	case 6:
		return 0, errors.New("mve: block mode 6 is not used by M.A.X. movies")
	case 7:
		return v.decodeMode7(dst, data)
	case 8:
		return v.decodeMode8(dst, data)
	case 9:
		return v.decodeMode9(dst, data)
	case 10:
		return v.decodeMode10(dst, data)
	case 11:
		if len(data) < 64 {
			return 0, errBlockShort(code)
		}
		for y := 0; y < 8; y++ {
			copy(v.cur[dst+y*v.w:dst+y*v.w+8], data[y*8:y*8+8])
		}
		return 64, nil
	case 12:
		if len(data) < 16 {
			return 0, errBlockShort(code)
		}
		for i := 0; i < 4; i++ {
			a := repeat2(data[i*4]) | repeat2(data[i*4+1])<<16
			b := repeat2(data[i*4+2]) | repeat2(data[i*4+3])<<16
			v.put4(dst+i*2*v.w, a)
			v.put4(dst+i*2*v.w+4, b)
			v.put4(dst+(i*2+1)*v.w, a)
			v.put4(dst+(i*2+1)*v.w+4, b)
		}
		return 16, nil
	case 13:
		if len(data) < 4 {
			return 0, errBlockShort(code)
		}
		for q := 0; q < 2; q++ {
			a := repeat4(data[q*2])
			b := repeat4(data[q*2+1])
			for y := 0; y < 2; y++ {
				row := dst + (q*4+y)*v.w
				v.put4(row, a)
				v.put4(row+4, b)
				row += 2 * v.w
				v.put4(row, a)
				v.put4(row+4, b)
			}
		}
		return 4, nil
	case 14:
		if len(data) < 1 {
			return 0, errBlockShort(code)
		}
		val := repeat4(data[0])
		for y := 0; y < 8; y++ {
			v.put4(dst+y*v.w, val)
			v.put4(dst+y*v.w+4, val)
		}
		return 1, nil
	case 15:
		if len(data) < 2 {
			return 0, errBlockShort(code)
		}
		lo, hi := data[0], data[1]
		v1 := uint32(lo) | uint32(hi)<<8 | uint32(lo)<<16 | uint32(hi)<<24
		v2 := uint32(hi) | uint32(lo)<<8 | uint32(hi)<<16 | uint32(lo)<<24
		for y := 0; y < 4; y++ {
			v.put4(dst+y*2*v.w, v1)
			v.put4(dst+y*2*v.w+4, v1)
			v.put4(dst+(y*2+1)*v.w, v2)
			v.put4(dst+(y*2+1)*v.w+4, v2)
		}
		return 2, nil
	default:
		return 0, fmt.Errorf("mve: invalid block mode %d", code)
	}
}

// decodeMode7 draws a block with two colors using either pixel or 2x2 flags.
func (v *videoState) decodeMode7(dst int, data []byte) (int, error) {
	if len(data) < 4 {
		return 0, errBlockShort(7)
	}
	if data[0] <= data[1] {
		if len(data) < 10 {
			return 0, errBlockShort(7)
		}
		for y := 0; y < 8; y++ {
			flags := data[2+y]
			for x := 0; x < 8; x++ {
				v.cur[dst+y*v.w+x] = data[flags&1]
				flags >>= 1
			}
		}
		return 10, nil
	}
	flags := binary.LittleEndian.Uint16(data[2:4])
	for y := 0; y < 8; y += 2 {
		for x := 0; x < 8; x += 2 {
			v.fill2x2(dst+x+y*v.w, data[flags&1])
			flags >>= 1
		}
	}
	return 4, nil
}

// decodeMode8 draws four two-color 4x4 blocks or a pair of split rectangles.
func (v *videoState) decodeMode8(dst int, data []byte) (int, error) {
	if len(data) < 12 {
		return 0, errBlockShort(8)
	}
	if data[0] <= data[1] {
		if len(data) < 16 {
			return 0, errBlockShort(8)
		}
		v.draw1Bit4x4(dst, data[0], data[1], binary.LittleEndian.Uint16(data[2:4]))
		v.draw1Bit4x4(dst+4*v.w, data[4], data[5], binary.LittleEndian.Uint16(data[6:8]))
		v.draw1Bit4x4(dst+4, data[8], data[9], binary.LittleEndian.Uint16(data[10:12]))
		v.draw1Bit4x4(dst+4+4*v.w, data[12], data[13], binary.LittleEndian.Uint16(data[14:16]))
		return 16, nil
	}
	flags := uint32(binary.LittleEndian.Uint32(data[2:6]))
	if data[6] <= data[7] {
		v.draw1BitRect(dst, 4, 8, data[0], data[1], flags)
		flags = binary.LittleEndian.Uint32(data[8:12])
		v.draw1BitRect(dst+4, 4, 8, data[6], data[7], flags)
		return 12, nil
	}
	v.draw1BitRect(dst, 8, 4, data[0], data[1], flags)
	flags = binary.LittleEndian.Uint32(data[8:12])
	v.draw1BitRect(dst+4*v.w, 8, 4, data[6], data[7], flags)
	return 12, nil
}

// decodeMode9 draws a four-color block using one of the packed flag layouts selected by its color ordering.
func (v *videoState) decodeMode9(dst int, data []byte) (int, error) {
	if len(data) < 8 {
		return 0, errBlockShort(9)
	}
	switch {
	case data[0] <= data[1] && data[2] <= data[3]:
		if len(data) < 20 {
			return 0, errBlockShort(9)
		}
		for y := 0; y < 8; y++ {
			flags := binary.LittleEndian.Uint16(data[4+y*2 : 6+y*2])
			for x := 0; x < 8; x++ {
				v.cur[dst+y*v.w+x] = data[flags&3]
				flags >>= 2
			}
		}
		return 20, nil
	case data[0] <= data[1]:
		flags := binary.LittleEndian.Uint32(data[4:8])
		for y := 0; y < 8; y += 2 {
			for x := 0; x < 8; x += 2 {
				v.fill2x2(dst+x+y*v.w, data[flags&3])
				flags >>= 2
			}
		}
		return 8, nil
	case data[2] <= data[3]:
		if len(data) < 12 {
			return 0, errBlockShort(9)
		}
		flags := binary.LittleEndian.Uint64(data[4:12])
		for i := 0; i < 8; i++ {
			for x := 0; x < 8; x += 2 {
				c := data[flags&3]
				v.cur[dst+i*v.w+x] = c
				v.cur[dst+i*v.w+x+1] = c
				flags >>= 2
			}
		}
		return 12, nil
	default:
		if len(data) < 12 {
			return 0, errBlockShort(9)
		}
		flags := binary.LittleEndian.Uint64(data[4:12])
		for y := 0; y < 8; y += 2 {
			for x := 0; x < 8; x++ {
				c := data[flags&3]
				v.cur[dst+y*v.w+x] = c
				v.cur[dst+(y+1)*v.w+x] = c
				flags >>= 2
			}
		}
		return 12, nil
	}
}

// decodeMode10 draws four-color 4x4 blocks or a pair of split rectangles.
func (v *videoState) decodeMode10(dst int, data []byte) (int, error) {
	if len(data) < 16 {
		return 0, errBlockShort(10)
	}
	if data[0] <= data[1] {
		if len(data) < 32 {
			return 0, errBlockShort(10)
		}
		v.draw2Bit4x4(dst, data[0:4], binary.LittleEndian.Uint32(data[4:8]))
		v.draw2Bit4x4(dst+4*v.w, data[8:12], binary.LittleEndian.Uint32(data[12:16]))
		v.draw2Bit4x4(dst+4, data[16:20], binary.LittleEndian.Uint32(data[20:24]))
		v.draw2Bit4x4(dst+4+4*v.w, data[24:28], binary.LittleEndian.Uint32(data[28:32]))
		return 32, nil
	}
	if len(data) < 24 {
		return 0, errBlockShort(10)
	}
	flags := binary.LittleEndian.Uint64(data[4:12])
	if data[12] <= data[13] {
		v.draw2BitRect(dst, 4, 8, data[0:4], flags)
		flags = binary.LittleEndian.Uint64(data[16:24])
		v.draw2BitRect(dst+4, 4, 8, data[12:16], flags)
		return 24, nil
	}
	v.draw2BitRect(dst, 8, 4, data[0:4], flags)
	flags = binary.LittleEndian.Uint64(data[16:24])
	v.draw2BitRect(dst+4*v.w, 8, 4, data[12:16], flags)
	return 24, nil
}

// draw1Bit4x4 expands one bit flags into a 4x4 region with two colors.
func (v *videoState) draw1Bit4x4(dst int, c0, c1 byte, flags uint16) {
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := c0
			if flags&1 != 0 {
				c = c1
			}
			v.cur[dst+y*v.w+x] = c
			flags >>= 1
		}
	}
}

// draw1BitRect expands one bit flags into a rectangle with two colors.
func (v *videoState) draw1BitRect(dst, width, height int, c0, c1 byte, flags uint32) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := c0
			if flags&1 != 0 {
				c = c1
			}
			v.cur[dst+y*v.w+x] = c
			flags >>= 1
		}
	}
}

// draw2Bit4x4 expands two bit flags into a 4x4 region with four colors.
func (v *videoState) draw2Bit4x4(dst int, colors []byte, flags uint32) {
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			v.cur[dst+y*v.w+x] = colors[flags&3]
			flags >>= 2
		}
	}
}

// draw2BitRect expands two bit flags into a rectangle with four colors.
func (v *videoState) draw2BitRect(dst, width, height int, colors []byte, flags uint64) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v.cur[dst+y*v.w+x] = colors[flags&3]
			flags >>= 2
		}
	}
}

// fill2x2 fills a 2x2 pixel region with one palette index.
func (v *videoState) fill2x2(dst int, c byte) {
	v.cur[dst] = c
	v.cur[dst+1] = c
	v.cur[dst+v.w] = c
	v.cur[dst+v.w+1] = c
}

/* // note: ported but unused code in Go port
// mode10Map expands compact mode-10 lookup indices into a byte map.
func (v *videoState) mode10Map(src []byte, dst []byte, count int) {
	for i := 0; i < count; i++ {
		value := v.lut6[src[i]]
		dst = append(dst, byte(value), byte(value>>8), byte(value>>16), byte(value>>24))
	}
}

// mode10Colors prepares color lookup entries for mode-10 pattern expansion.
func (v *videoState) mode10Colors(map2 *[256]uint32, src []byte) {
	map2[0xc1], map2[0xe1] = uint32(src[2]), uint32(src[2])
	map2[0xc3], map2[0xe3] = uint32(src[0]), uint32(src[0])
	map2[0xc5], map2[0xe5] = uint32(src[3]), uint32(src[3])
	map2[0xc7], map2[0xe7] = uint32(src[1]), uint32(src[1])
}
*/

// copyBlock copies one 8x8 block from a current or previous frame buffer.
func (v *videoState) copyBlock(dst int, src []byte, srcAt int) error {
	if srcAt < 0 || srcAt+7*v.w+8 > len(src) || dst < 0 || dst+7*v.w+8 > len(v.cur) {
		return fmt.Errorf("mve: block copy out of bounds dst=%d src=%d", dst, srcAt)
	}
	for y := 0; y < 8; y++ {
		copy(v.cur[dst+y*v.w:dst+y*v.w+8], src[srcAt+y*v.w:srcAt+y*v.w+8])
	}
	return nil
}

/* // note: ported but unused code in Go port
// fillPattern expands packed pattern data used by MVE pattern block modes.
func (v *videoState) fillPattern(dst int, src []byte, stride, offset, mode int) {
	var map1 [128]byte
	var map2 [256]uint32
	for i := 0; i < stride; i++ {
		value := v.lut5[src[2+i]]
		map1[offset+i*4+0] = byte(value)
		map1[offset+i*4+1] = byte(value >> 8)
		map1[offset+i*4+2] = byte(value >> 16)
		map1[offset+i*4+3] = byte(value >> 24)
	}
	map2[0xc1] = uint32(src[1])<<8 | uint32(src[0])
	map2[0xc2] = uint32(src[0])<<8 | uint32(src[1])
	map2[0xc3] = uint32(src[0])<<8 | uint32(src[0])
	map2[0xc5] = uint32(src[1])<<8 | uint32(src[1])
	for i := 0; i < stride; i++ {
		row := dst + i*v.w
		switch mode {
		case 2:
			v.put4(row, (map2[map1[offset+i*4+0]]<<16)|map2[map1[offset+i*4+1]])
			v.put4(row+v.w, (map2[map1[offset+i*4+2]]<<16)|map2[map1[offset+i*4+3]])
		case 1:
			v.put4(row, (map2[map1[offset+i*4+2]]<<16)|map2[map1[offset+i*4+1]])
			v.put4(row+v.w, (map2[map1[offset+i*4+2]]<<16)|map2[map1[offset+i*4+3]])
		case 0:
			v.put4(row, (map2[map1[offset+i*4+0]]<<16)|map2[map1[offset+i*4+1]])
			v.put4(row+4, (map2[map1[offset+i*4+2]]<<16)|map2[map1[offset+i*4+3]])
		}
	}
}

// fillPattern11 builds lookup tables for block mode 11 pattern expansion.
func (v *videoState) fillPattern11(src []byte, map1 *[128]byte, map2 *[256]uint32, stride int, swap bool) {
	for i := 0; i < stride; i++ {
		value := v.lut6[src[4+i]]
		if swap {
			value = value<<24 | (value&0xff00)<<8 | (value&0xff0000)>>8 | value>>24
		}
		map1[i*4+0] = byte(value)
		map1[i*4+1] = byte(value >> 8)
		map1[i*4+2] = byte(value >> 16)
		map1[i*4+3] = byte(value >> 24)
	}
	map2[0xc1], map2[0xe1] = uint32(src[2]), uint32(src[2])
	map2[0xc3], map2[0xe3] = uint32(src[0]), uint32(src[0])
	map2[0xc5], map2[0xe5] = uint32(src[3]), uint32(src[3])
	map2[0xc7], map2[0xe7] = uint32(src[1]), uint32(src[1])
}

// fillPattern12 writes a packed pattern row used by block mode 12.
func (v *videoState) fillPattern12(dst int, map1 *[128]byte, map2 *[256]uint32, index, mode int) {
	if mode == 0 {
		v.put4(dst, map2[map1[index+0]]<<16|map2[map1[index+1]]<<24|map2[map1[index+2]]|map2[map1[index+3]]<<8)
		v.put4(dst+4, map2[map1[index+4]]<<16|map2[map1[index+5]]<<24|map2[map1[index+6]]|map2[map1[index+7]]<<8)
		return
	}
	v.put4(dst, map2[map1[index+0]]<<24|map2[map1[index+0]]<<16|map2[map1[index+1]]<<8|map2[map1[index+1]])
	v.put4(dst+4, map2[map1[index+2]]<<24|map2[map1[index+2]]<<16|map2[map1[index+3]]<<8|map2[map1[index+3]])
}

// fillPattern12Half writes the left half of a block mode 12 pattern row.
func (v *videoState) fillPattern12Half(dst int, map1 *[128]byte, map2 *[256]uint32, index int) {
	v.put4(dst, map2[map1[index+0]]<<16|map2[map1[index+1]]<<24|map2[map1[index+2]]|map2[map1[index+3]]<<8)
}
*/

// put4 writes four packed palette indices into the current frame.
func (v *videoState) put4(at int, value uint32) {
	v.cur[at+0] = byte(value)
	v.cur[at+1] = byte(value >> 8)
	v.cur[at+2] = byte(value >> 16)
	v.cur[at+3] = byte(value >> 24)
}

/* // note: ported but unused code in Go port
// unpack4 writes four packed palette indices into a temporary 8x8 block.
func (v *videoState) unpack4(dst *[64]byte, offset int, value uint32) {
	dst[offset+0] = byte(value)
	dst[offset+1] = byte(value >> 8)
	dst[offset+2] = byte(value >> 16)
	dst[offset+3] = byte(value >> 24)
}
*/

// initLUTs builds the motion vector and pattern lookup tables used by MVE modes.
func (v *videoState) initLUTs() {
	for i := 0; i < 256; i++ {
		x := (i & 0x0f) - 8
		y := (i >> 4) - 8
		v.lut2[i] = uint16(byte(x)) | uint16(byte(y))<<8
	}
	n := 0
	for y := 0; y <= 7; y++ {
		for x := 8; x <= 14; x++ {
			v.lut3[n] = uint16(byte(x)) | uint16(byte(y))<<8
			n++
		}
	}
	for y := 8; y <= 13; y++ {
		for x := -14; x <= 14; x++ {
			v.lut3[n] = uint16(byte(x)) | uint16(byte(y))<<8
			n++
		}
	}
	for x := -14; x <= 11; x++ {
		v.lut3[n] = uint16(byte(x)) | uint16(byte(14))<<8
		n++
	}
	/* // note: unused code in Go port
	// the lut4 / lut5 / lut6 tables (see videoState).
	two := [2]byte{0xc3, 0xc1}
	for i := 0; i < 16; i++ {
		var b [4]byte
		b[0] = two[(i>>2)&1]
		b[1] = two[i&1]
		b[2] = two[(i>>3)&1]
		b[3] = two[(i>>1)&1]
		v.lut4[i] = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	}
	four := [4]byte{0xc3, 0xc2, 0xc1, 0xc5}
	for i := 0; i < 256; i++ {
		v.lut5[i] = uint32(four[(i/4)%4]) | uint32(four[i%4])<<8 | uint32(four[(i/16)%4])<<16 | uint32(four[(i/64)%4])<<24
	}
	cVals := [4]byte{0xc3, 0xc7, 0xc1, 0xc5}
	eVals := [4]byte{0xe3, 0xe7, 0xe1, 0xe5}
	for i := 0; i < 256; i++ {
		v.lut6[i] = uint32(cVals[(i/64)%4]) |
			uint32(eVals[(i/16)%4])<<8 |
			uint32(cVals[i%4])<<16 |
			uint32(eVals[(i/4)%4])<<24
	}
	*/
}

// repeat2 packs one byte twice into a 16-bit lane inside a uint32.
func repeat2(b byte) uint32 {
	return uint32(b) | uint32(b)<<8
}

// repeat4 packs one byte into all four bytes of a uint32.
func repeat4(b byte) uint32 {
	return uint32(b) | uint32(b)<<8 | uint32(b)<<16 | uint32(b)<<24
}

// errBlockShort reports truncated data for an MVE video block mode.
func errBlockShort(code int) error {
	return fmt.Errorf("mve: block mode %d data truncated", code)
}
