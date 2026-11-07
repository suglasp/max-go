// mve.go : the M.A.X. Interplay MVE movie decoder - demuxes audio/video chunks and drives the playback loop.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : mvelib32.c - Interplay MVE decoder
//
// Package mve decodes the Interplay MVE movies used by M.A.X.
package mve

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	opEndOfStream       = 0x00
	opEndOfRecord       = 0x01
	opCreateTimer       = 0x02
	opAllocAudioBuffers = 0x03
	opSynchAudio        = 0x04
	opAllocVideoBuffers = 0x05
	opShowVideoFrame    = 0x07
	opAudioFrame        = 0x08
	opSilenceFrame      = 0x09
	opInitGfxMode       = 0x0a
	opLoadPalette       = 0x0c
	opSetDecodingMap2   = 0x0f
	opDecompVideo2      = 0x11
	opIgnoredFrameMeta  = 0x13
	opIgnoredMovieMeta  = 0x15
	headerSize          = 30
	controlSize         = 4
	paletteBytes        = 768
	mveTag              = "Interplay MVE File\x1a\x00"
	defaultTrackMask    = 1
)

// AudioFormat describes the stream declared by the MVE audio-allocation opcode.
type AudioFormat struct {
	Rate       int
	Channels   int
	Bits       int
	Compressed bool
}

// Frame is one decoded presentation frame.
type Frame struct {
	W, H       int
	Pix        []byte
	Palette    [paletteBytes]byte
	PaletteSet bool
	Audio      []int16
	DelayMS    int
}

// Movie is an Interplay MVE stream decoder.
type Movie struct {
	r       io.ReadSeeker
	next    control
	record  []byte
	pos     int
	ended   bool
	delayMS int

	audio AudioFormat
	video videoState

	palette         [paletteBytes]byte
	paletteDirty    bool
	pendingAudio    []int16
	pendingPalette  bool
	pendingMap2     []byte
	currentFrameNum int
}

type control struct {
	size    uint16
	opcode  uint8
	version uint8
}

// Open validates an Interplay MVE stream and prepares it for decoding.
//
// C++: ioReset + MVE_rmPrepMovie (mvelib32.c)
func Open(r io.ReadSeeker) (*Movie, error) {
	if r == nil {
		return nil, errors.New("mve: nil reader")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("mve: seek start: %w", err)
	}
	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, fmt.Errorf("mve: read header: %w", err)
	}
	if string(hdr[:20]) != mveTag {
		return nil, errors.New("mve: invalid header tag")
	}
	sep := binary.LittleEndian.Uint16(hdr[20:22])
	version := binary.LittleEndian.Uint16(hdr[22:24])
	checksum := binary.LittleEndian.Uint16(hdr[24:26])
	if sep != 0x1a || version != 0x0100 || checksum != (^version+0x1234) {
		return nil, fmt.Errorf("mve: invalid header sep=%#x version=%#x checksum=%#x", sep, version, checksum)
	}
	first := readControl(hdr[26:30])
	meta := &Movie{}
	if err := meta.scanMetadata(r, first); err != nil {
		return nil, err
	}
	if _, err := r.Seek(headerSize, io.SeekStart); err != nil {
		return nil, fmt.Errorf("mve: seek first record: %w", err)
	}
	m := &Movie{
		r:       r,
		next:    first,
		delayMS: meta.delayMS,
		audio:   meta.audio,
		video:   meta.video,
	}
	if err := m.loadNextRecord(); err != nil {
		return nil, err
	}
	return m, nil
}

// W returns the width declared by the video-allocation opcode.
func (m *Movie) W() int {
	return m.video.w
}

// H returns the height declared by the video-allocation opcode.
func (m *Movie) H() int {
	return m.video.h
}

// AudioFormat returns the decoded audio parameters.
func (m *Movie) AudioFormat() AudioFormat {
	return m.audio
}

// NextFrame decodes and returns the next presentation frame.
//
// C++: MVE_rmStepMovie (mvelib32.c)
func (m *Movie) NextFrame() (*Frame, error) {
	if m.ended {
		return nil, io.EOF
	}
	var map2 []byte
	for {
		if m.pos >= len(m.record) {
			if err := m.loadNextRecord(); err != nil {
				return nil, err
			}
			map2 = nil
		}
		if len(m.record)-m.pos < controlSize {
			return nil, fmt.Errorf("mve: truncated opcode header at record offset %d", m.pos)
		}
		cb := readControl(m.record[m.pos : m.pos+controlSize])
		m.pos += controlSize
		if len(m.record)-m.pos < int(cb.size) {
			return nil, fmt.Errorf("mve: opcode %#x truncated: size %d at offset %d", cb.opcode, cb.size, m.pos)
		}
		payload := m.record[m.pos : m.pos+int(cb.size)]
		m.pos += int(cb.size)

		switch cb.opcode {
		case opEndOfStream:
			m.ended = true
			return nil, io.EOF
		case opEndOfRecord:
			if err := m.loadNextRecord(); err != nil {
				return nil, err
			}
			map2 = nil
		case opCreateTimer:
			if len(payload) < 6 {
				return nil, errShortOpcode(cb.opcode)
			}
			m.delayMS = timerDelayMS(payload)
		case opAllocAudioBuffers:
			if err := m.handleAudioAlloc(cb.version, payload); err != nil {
				return nil, err
			}
		case opSynchAudio:
			// Timing is caller owned in this package.
		case opAllocVideoBuffers:
			if err := m.handleVideoAlloc(cb.version, payload); err != nil {
				return nil, err
			}
		case opShowVideoFrame:
			if len(payload) < 6 {
				return nil, errShortOpcode(cb.opcode)
			}
			m.currentFrameNum++
			f := &Frame{
				W:          m.video.w,
				H:          m.video.h,
				Pix:        append([]byte(nil), m.video.cur...),
				Palette:    m.palette,
				PaletteSet: m.pendingPalette || m.paletteDirty,
				Audio:      append([]int16(nil), m.pendingAudio...),
				DelayMS:    m.delayMS,
			}
			m.pendingAudio = m.pendingAudio[:0]
			m.pendingPalette = false
			m.paletteDirty = false
			return f, nil
		case opAudioFrame, opSilenceFrame:
			audio, err := m.decodeAudio(cb.opcode, payload)
			if err != nil {
				return nil, err
			}
			m.pendingAudio = append(m.pendingAudio, audio...)
		case opInitGfxMode:
			if len(payload) < 6 {
				return nil, errShortOpcode(cb.opcode)
			}
		case opLoadPalette:
			if len(payload) < 4 {
				return nil, errShortOpcode(cb.opcode)
			}
			start := int(binary.LittleEndian.Uint16(payload[0:2]))
			count := int(binary.LittleEndian.Uint16(payload[2:4]))
			if start < 0 || count < 0 || start+count > 256 || len(payload[4:]) < count*3 {
				return nil, fmt.Errorf("mve: invalid palette range start=%d count=%d payload=%d", start, count, len(payload))
			}
			copy(m.palette[start*3:(start+count)*3], payload[4:4+count*3])
			m.pendingPalette = true
			m.paletteDirty = true
		case opSetDecodingMap2:
			map2 = payload
			m.pendingMap2 = payload
		case opDecompVideo2:
			if map2 == nil {
				map2 = m.pendingMap2
			}
			if len(map2) == 0 {
				return nil, errors.New("mve: video opcode without decoding map")
			}
			if err := m.handleVideo(cb.version, payload, map2); err != nil {
				return nil, err
			}
		case opIgnoredFrameMeta, opIgnoredMovieMeta:
			// Retail M.A.X. movies contain these records, but mvelib32.c does not process them.
		default:
			return nil, fmt.Errorf("mve: unsupported opcode %#x version %d size %d", cb.opcode, cb.version, cb.size)
		}
	}
}

// loadNextRecord reads the next record body and primes the following control word.
func (m *Movie) loadNextRecord() error {
	size := int(m.next.size)
	buf := make([]byte, size+controlSize)
	if _, err := io.ReadFull(m.r, buf); err != nil {
		return fmt.Errorf("mve: read record: %w", err)
	}
	m.record = buf[:size]
	m.pos = 0
	m.next = readControl(buf[size:])
	return nil
}

// scanMetadata reads early records until it knows the frame delay, audio format, and video geometry.
func (m *Movie) scanMetadata(r io.Reader, next control) error {
	for {
		size := int(next.size)
		buf := make([]byte, size+controlSize)
		if _, err := io.ReadFull(r, buf); err != nil {
			return fmt.Errorf("mve: read metadata record: %w", err)
		}
		record := buf[:size]
		recordDone := false
		for pos := 0; pos < len(record); {
			if len(record)-pos < controlSize {
				return fmt.Errorf("mve: truncated opcode header at record offset %d", pos)
			}
			cb := readControl(record[pos : pos+controlSize])
			pos += controlSize
			if len(record)-pos < int(cb.size) {
				return fmt.Errorf("mve: opcode %#x truncated: size %d at offset %d", cb.opcode, cb.size, pos)
			}
			payload := record[pos : pos+int(cb.size)]
			pos += int(cb.size)

			switch cb.opcode {
			case opEndOfStream:
				return nil
			case opEndOfRecord:
				recordDone = true
			case opShowVideoFrame:
			case opCreateTimer:
				if len(payload) < 6 {
					return errShortOpcode(cb.opcode)
				}
				m.delayMS = timerDelayMS(payload)
			case opAllocAudioBuffers:
				if err := m.handleAudioAlloc(cb.version, payload); err != nil {
					return err
				}
			case opAllocVideoBuffers:
				if err := m.handleVideoAlloc(cb.version, payload); err != nil {
					return err
				}
			}
			if m.video.w > 0 && m.audio.Rate > 0 {
				return nil
			}
			if recordDone {
				break
			}
		}
		next = readControl(buf[size:])
	}
}

// handleAudioAlloc records the audio format advertised by an allocation opcode.
func (m *Movie) handleAudioAlloc(version uint8, payload []byte) error {
	if version < 1 {
		if len(payload) < 8 {
			return errShortOpcode(opAllocAudioBuffers)
		}
		flags := binary.LittleEndian.Uint16(payload[2:4])
		m.audio = AudioFormat{
			Rate:     int(binary.LittleEndian.Uint16(payload[4:6])),
			Channels: 1 + int(flags&1),
			Bits:     8,
		}
		if flags&2 != 0 {
			m.audio.Bits = 16
		}
		return nil
	}
	if len(payload) < 10 {
		return errShortOpcode(opAllocAudioBuffers)
	}
	flags := binary.LittleEndian.Uint16(payload[2:4])
	m.audio = AudioFormat{
		Rate:       int(binary.LittleEndian.Uint16(payload[4:6])),
		Channels:   1,
		Bits:       8,
		Compressed: flags&4 != 0,
	}
	if flags&1 != 0 {
		m.audio.Channels = 2
	}
	if flags&2 != 0 {
		m.audio.Bits = 16
	}
	return nil
}

// handleVideoAlloc configures video frame buffers from an allocation opcode.
func (m *Movie) handleVideoAlloc(version uint8, payload []byte) error {
	if len(payload) < 4 {
		return errShortOpcode(opAllocVideoBuffers)
	}
	widthBlocks := int(binary.LittleEndian.Uint16(payload[0:2]))
	heightBlocks := int(binary.LittleEndian.Uint16(payload[2:4]))
	stride := 1
	hicolor := 0
	if version >= 1 {
		if len(payload) < 6 {
			return errShortOpcode(opAllocVideoBuffers)
		}
		stride = int(binary.LittleEndian.Uint16(payload[4:6]))
	}
	if version >= 2 {
		if len(payload) < 8 {
			return errShortOpcode(opAllocVideoBuffers)
		}
		hicolor = int(binary.LittleEndian.Uint16(payload[6:8]))
	}
	if hicolor != 0 {
		return errors.New("mve: hicolor video is not supported")
	}
	return m.video.config(widthBlocks, heightBlocks, stride)
}

// handleVideo applies one compressed video frame opcode to the current buffer.
func (m *Movie) handleVideo(version uint8, payload, map2 []byte) error {
	if version < 3 {
		return fmt.Errorf("mve: unsupported video opcode version %d", version)
	}
	if len(payload) < 14 {
		return errShortOpcode(opDecompVideo2)
	}
	flags := binary.LittleEndian.Uint16(payload[12:14])
	x := int(binary.LittleEndian.Uint16(payload[4:6]))
	y := int(binary.LittleEndian.Uint16(payload[6:8]))
	w := int(binary.LittleEndian.Uint16(payload[8:10]))
	h := int(binary.LittleEndian.Uint16(payload[10:12]))
	if flags&1 != 0 {
		m.video.advance()
	}
	return m.video.decomp(map2, payload[14:], x, y, w, h)
}

// decodeAudio converts one audio opcode payload into signed 16-bit samples.
func (m *Movie) decodeAudio(op uint8, payload []byte) ([]int16, error) {
	if len(payload) < 6 {
		return nil, errShortOpcode(op)
	}
	mask := binary.LittleEndian.Uint16(payload[2:4])
	length := int(binary.LittleEndian.Uint16(payload[4:6]))
	if mask&defaultTrackMask == 0 {
		return nil, nil
	}
	if m.audio.Bits != 16 {
		return nil, errors.New("mve: only 16-bit movie audio is supported")
	}
	if length%2 != 0 {
		return nil, fmt.Errorf("mve: odd audio byte length %d", length)
	}
	samples := length / 2
	out := make([]int16, samples)
	if op == opSilenceFrame {
		return out, nil
	}
	data := payload[6:]
	if !m.audio.Compressed {
		if len(data) < length {
			return nil, fmt.Errorf("mve: uncompressed audio truncated: need %d have %d", length, len(data))
		}
		for i := range out {
			out[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
		}
		return out, nil
	}
	if m.audio.Channels == 2 {
		return decompS16(data, samples)
	}
	return decompM16(data, samples)
}

// readControl decodes the four-byte MVE record or opcode header.
func readControl(b []byte) control {
	return control{size: binary.LittleEndian.Uint16(b[0:2]), opcode: b[2], version: b[3]}
}

// errShortOpcode reports a truncated payload for an MVE opcode.
func errShortOpcode(op uint8) error {
	return fmt.Errorf("mve: opcode %#x payload truncated", op)
}

// timerDelayMS is the frame delay of a CREATE_TIMER payload (rate in microseconds times the divider) in
// milliseconds; the caller checks the payload holds 6 bytes.
func timerDelayMS(payload []byte) int {
	rate := binary.LittleEndian.Uint32(payload[0:4])
	divider := binary.LittleEndian.Uint16(payload[4:6])
	return int((rate * uint32(divider)) / 1000)
}
