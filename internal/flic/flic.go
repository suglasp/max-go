// flic.go : decodes Autodesk Animator FLI and FLC streams.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package flic provides streaming frame and palette decoding for M.A.X. animations.
package flic

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	typeFLI      = 0xaf11
	typeFLC      = 0xaf12
	typeFrame    = 0xf1fa
	typeFrameFLI = 0xf100

	chunkColor256 = 4
	chunkDeltaFLC = 7
	chunkColor64  = 11
	chunkDeltaFLI = 12
	chunkBlack    = 13
	chunkByteRun  = 15
	chunkLiteral  = 16
	chunkPStamp   = 18

	headerSize   = 128
	paletteBytes = 256 * 3
)

// Header describes the fixed 128-byte FLI/FLC file header.
type Header struct {
	Size    uint32
	Type    uint16
	Frames  int
	W, H    int
	Depth   int
	SpeedMS int
}

// Frame is one decoded palettized animation frame.
type Frame struct {
	W, H       int
	Pix        []byte
	Palette    [paletteBytes]byte
	PaletteSet bool
	DelayMS    int
}

// Animation is a streaming FLI/FLC decoder.
type Animation struct {
	r      io.ReadSeeker
	header Header
	pos    int
	pix    []byte
	pal    [paletteBytes]byte
}

// Open validates a FLI/FLC stream and prepares it for frame decoding.
//
// C++: flicsmgr_read (flicsmgr.cpp)
func Open(r io.ReadSeeker) (*Animation, error) {
	if r == nil {
		return nil, errors.New("flic: nil reader")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("flic: seek start: %w", err)
	}
	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, fmt.Errorf("flic: read header: %w", err)
	}
	typ := binary.LittleEndian.Uint16(hdr[4:6])
	if typ != typeFLI && typ != typeFLC {
		return nil, fmt.Errorf("flic: invalid type %#x", typ)
	}

	// width and height, plus Depth
	w := int(binary.LittleEndian.Uint16(hdr[8:10]))
	h := int(binary.LittleEndian.Uint16(hdr[10:12]))
	depth := int(binary.LittleEndian.Uint16(hdr[12:14]))

	if (w <= 0) || (h <= 0) {
		return nil, fmt.Errorf("flic: invalid geometry %dx%d", w, h)
	}
	if depth != 8 {
		return nil, fmt.Errorf("flic: unsupported depth %d", depth)
	}

	// animation speed
	speed := int(binary.LittleEndian.Uint32(hdr[16:20]))
	if typ == typeFLI {
		speed = speed * 1000 / 70
	}
	if speed <= 0 {
		speed = 67
	}

	// offset tracking
	offset := int64(binary.LittleEndian.Uint32(hdr[80:84]))
	if offset == 0 {
		offset = headerSize
	}
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("flic: seek first frame: %w", err)
	}

	a := &Animation{
		r: r,
		header: Header{
			Size:    binary.LittleEndian.Uint32(hdr[0:4]),
			Type:    typ,
			Frames:  int(binary.LittleEndian.Uint16(hdr[6:8])),
			W:       w,
			H:       h,
			Depth:   depth,
			SpeedMS: speed,
		},
		pix: make([]byte, w*h),
	}

	for i := 0; i < 256; i++ {
		v := uint8(i / 4)
		a.pal[i*3], a.pal[i*3+1], a.pal[i*3+2] = v, v, v
	}
	return a, nil
}

// Header returns the parsed animation header.
func (a *Animation) Header() Header {
	return a.header
}

// W returns the frame width.
func (a *Animation) W() int {
	return a.header.W
}

// H returns the frame height.
func (a *Animation) H() int {
	return a.header.H
}

// NextFrame decodes and returns the next frame. It returns io.EOF after the header frame count.
//
// C++: flicsmgr_advance_animation + flicsmgr_decode_frame (flicsmgr.cpp)
func (a *Animation) NextFrame() (*Frame, error) {
	if a.pos >= a.header.Frames {
		return nil, io.EOF
	}
	header := make([]byte, 16)
	if _, err := io.ReadFull(a.r, header); err != nil {
		return nil, fmt.Errorf("flic: read frame header: %w", err)
	}
	frameSize := int(binary.LittleEndian.Uint32(header[0:4]))
	frameType := binary.LittleEndian.Uint16(header[4:6])
	chunks := int(binary.LittleEndian.Uint16(header[6:8]))
	if frameType != typeFrame && frameType != typeFrameFLI {
		return nil, fmt.Errorf("flic: invalid frame type %#x", frameType)
	}
	if frameSize < 16 {
		return nil, fmt.Errorf("flic: invalid frame size %d", frameSize)
	}
	payload := make([]byte, frameSize-16)
	if _, err := io.ReadFull(a.r, payload); err != nil {
		return nil, fmt.Errorf("flic: read frame payload: %w", err)
	}
	paletteSet := false
	pos := 0
	for i := 0; i < chunks; i++ {
		if len(payload)-pos < 6 {
			return nil, fmt.Errorf("flic: truncated chunk header at frame offset %d", pos)
		}
		chunkSize := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
		chunkType := binary.LittleEndian.Uint16(payload[pos+4 : pos+6])
		if chunkSize < 6 || pos+chunkSize > len(payload) {
			return nil, fmt.Errorf("flic: invalid chunk size %d type %#x at frame offset %d", chunkSize, chunkType, pos)
		}
		data := payload[pos+6 : pos+chunkSize]
		switch chunkType {
		case chunkColor256:
			if err := a.decodeColor(data, 2); err != nil {
				return nil, err
			}
			paletteSet = true
		case chunkColor64:
			if err := a.decodeColor(data, 0); err != nil {
				return nil, err
			}
			paletteSet = true
		case chunkDeltaFLC:
			if err := a.decodeDeltaFLC(data); err != nil {
				return nil, err
			}
		case chunkDeltaFLI:
			if err := a.decodeDeltaFLI(data); err != nil {
				return nil, err
			}
		case chunkBlack:
			clear(a.pix)
		case chunkByteRun:
			if err := a.decodeByteRun(data); err != nil {
				return nil, err
			}
		case chunkLiteral:
			if len(data) < len(a.pix) {
				return nil, fmt.Errorf("flic: literal chunk truncated: need %d have %d", len(a.pix), len(data))
			}
			copy(a.pix, data[:len(a.pix)])
		case chunkPStamp:
			// PSTAMP thumbnails are present in the retail files but are not part of playback.
		default:
			return nil, fmt.Errorf("flic: unsupported chunk type %#x size %d", chunkType, chunkSize)
		}
		pos += chunkSize
	}
	a.pos++
	return &Frame{
		W:          a.header.W,
		H:          a.header.H,
		Pix:        append([]byte(nil), a.pix...),
		Palette:    a.pal,
		PaletteSet: paletteSet,
		DelayMS:    a.header.SpeedMS,
	}, nil
}

// C++: flicsmgr_decode_color (flicsmgr.cpp)
func (a *Animation) decodeColor(data []byte, shift uint) error {
	if len(data) < 2 {
		return errors.New("flic: color chunk truncated")
	}
	packets := int(binary.LittleEndian.Uint16(data[0:2]))
	pos := 2
	entry := 0
	for p := 0; p < packets; p++ {
		if len(data)-pos < 2 {
			return errors.New("flic: color packet truncated")
		}
		entry += int(data[pos])
		count := int(data[pos+1])
		pos += 2
		if count == 0 {
			count = 256
		}
		if entry+count > 256 || len(data)-pos < count*3 {
			return fmt.Errorf("flic: invalid color packet entry=%d count=%d", entry, count)
		}
		for i := 0; i < count; i++ {
			a.pal[entry*3] = data[pos] >> shift
			a.pal[entry*3+1] = data[pos+1] >> shift
			a.pal[entry*3+2] = data[pos+2] >> shift
			pos += 3
			entry++
		}
	}
	return nil
}

// C++: flicsmgr_decode_byte_run (flicsmgr.cpp)
func (a *Animation) decodeByteRun(data []byte) error {
	pos := 0
	for y := 0; y < a.header.H; y++ {
		if pos >= len(data) {
			return errors.New("flic: byte-run row truncated")
		}
		pos++ // packet count, ignored by the original decoder.
		x := 0
		for x < a.header.W {
			if pos >= len(data) {
				return errors.New("flic: byte-run packet truncated")
			}
			count := int(int8(data[pos]))
			pos++
			if count >= 0 {
				if pos >= len(data) || x+count > a.header.W {
					return errors.New("flic: byte-run replicate packet invalid")
				}
				for i := 0; i < count; i++ {
					a.pix[y*a.header.W+x+i] = data[pos]
				}
				pos++
				x += count
				continue
			}
			count = -count
			if len(data)-pos < count || x+count > a.header.W {
				return errors.New("flic: byte-run literal packet invalid")
			}
			copy(a.pix[y*a.header.W+x:y*a.header.W+x+count], data[pos:pos+count])
			pos += count
			x += count
		}
	}
	return nil
}

// C++: flicsmgr_decode_delta_flc (flicsmgr.cpp)
func (a *Animation) decodeDeltaFLC(data []byte) error {
	if len(data) < 2 {
		return errors.New("flic: delta-flc chunk truncated")
	}
	lines := int(binary.LittleEndian.Uint16(data[0:2]))
	pos := 2
	y := 0
	for line := 0; line < lines; line++ {
		if len(data)-pos < 2 {
			return errors.New("flic: delta-flc line header truncated")
		}
		opt := int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
		pos += 2
		for uint16(opt)&0x8000 != 0 {
			if uint16(opt)&0x4000 != 0 {
				y += int(-opt)
				if y > a.header.H {
					return errors.New("flic: delta-flc line skip out of bounds")
				}
			} else if y < a.header.H {
				a.pix[y*a.header.W+a.header.W-1] = byte(opt)
			}
			if len(data)-pos < 2 {
				return errors.New("flic: delta-flc optional word truncated")
			}
			opt = int16(binary.LittleEndian.Uint16(data[pos : pos+2]))
			pos += 2
		}
		if y >= a.header.H {
			return errors.New("flic: delta-flc row out of bounds")
		}
		x := 0
		for packet := 0; packet < int(opt); packet++ {
			if len(data)-pos < 2 {
				return errors.New("flic: delta-flc packet header truncated")
			}
			x += int(data[pos])
			count := int(int8(data[pos+1]))
			pos += 2
			if x > a.header.W {
				return errors.New("flic: delta-flc column skip out of bounds")
			}
			if count >= 0 {
				n := count * 2
				if len(data)-pos < n || x+n > a.header.W {
					return errors.New("flic: delta-flc literal packet invalid")
				}
				copy(a.pix[y*a.header.W+x:y*a.header.W+x+n], data[pos:pos+n])
				pos += n
				x += n
				continue
			}
			count = -count
			n := count * 2
			if len(data)-pos < 2 || x+n > a.header.W {
				return errors.New("flic: delta-flc replicate packet invalid")
			}
			b0, b1 := data[pos], data[pos+1]
			pos += 2
			for i := 0; i < count; i++ {
				a.pix[y*a.header.W+x] = b0
				a.pix[y*a.header.W+x+1] = b1
				x += 2
			}
		}
		y++
	}
	return nil
}

// decodeDeltaFLI applies an FLI delta frame chunk (line by line deltas) to the current pixels.
func (a *Animation) decodeDeltaFLI(data []byte) error {
	if len(data) < 4 {
		return errors.New("flic: delta-fli chunk truncated")
	}
	y := int(binary.LittleEndian.Uint16(data[0:2]))
	lines := int(binary.LittleEndian.Uint16(data[2:4]))
	pos := 4
	if y+lines > a.header.H {
		return errors.New("flic: delta-fli lines out of bounds")
	}
	for line := 0; line < lines; line++ {
		if pos >= len(data) {
			return errors.New("flic: delta-fli packet count truncated")
		}
		packets := int(data[pos])
		pos++
		x := 0
		for packet := 0; packet < packets; packet++ {
			if len(data)-pos < 2 {
				return errors.New("flic: delta-fli packet header truncated")
			}
			x += int(data[pos])
			count := int(int8(data[pos+1]))
			pos += 2
			if x > a.header.W {
				return errors.New("flic: delta-fli column skip out of bounds")
			}
			if count >= 0 {
				if len(data)-pos < count || x+count > a.header.W {
					return errors.New("flic: delta-fli literal packet invalid")
				}
				copy(a.pix[(y+line)*a.header.W+x:(y+line)*a.header.W+x+count], data[pos:pos+count])
				pos += count
				x += count
				continue
			}
			count = -count
			if pos >= len(data) || x+count > a.header.W {
				return errors.New("flic: delta-fli replicate packet invalid")
			}
			for i := 0; i < count; i++ {
				a.pix[(y+line)*a.header.W+x+i] = data[pos]
			}
			pos++
			x += count
		}
	}
	return nil
}
