// wavloop.go : the M.A.X. WAV loop region parser: reads the SMPL chunk loop boundaries for seamless music looping.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"encoding/binary"
	"time"
)

// wavloop.go: minimal RIFF/WAV parsing for the loop region carried in a sound's
// "smpl" chunk, plus slicing a WAV down to that running region in memory. The
// game's power up and running effects (e.g. HVP17, POWGN17) hold an intro
// followed by a loopable running region; looping only that region gives the
// steady "running" sound with no repeated power up. Backends that cannot loop
// part of a WAV (miniaudio) instead loop a sliced WAV produced here.
//
// C++ ported : sound_manager.cpp - SoundManager::LoadLoopPoints reads the same
// smpl chunk; the building sound is restricted to the range [loop_start, loop_end]
//
// wavInfo describes the parts of a RIFF/WAV needed to slice it: the byte range
// of the fmt chunk (id+size+body), the byte range of the data chunk body, the
// frame size (channels times bytes per sample), and the loop range in sample
// frames from the smpl chunk (hasLoop=false when there is no usable loop).
type wavInfo struct {
	fmtStart, fmtEnd   int // the whole "fmt " chunk incl. its 8-byte header
	dataStart, dataEnd int // the data chunk body (samples)
	blockAlign         int // bytes per sample frame (channels * bits/8)
	sampleRate         int // frames per second
	loopStartFrame     int // smpl loop start, in sample frames
	loopEndFrame       int // smpl loop end, used as the exclusive playback boundary
	hasLoop            bool
}

// parseWAV reads the fmt/data/smpl layout of a RIFF/WAV. It returns ok=false for
// anything that is not a PCM WAV it can slice.
func parseWAV(b []byte) (wavInfo, bool) {
	var w wavInfo
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return w, false
	}
	off := 12
	for off+8 <= len(b) {
		id := string(b[off : off+4])
		size := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		body := off + 8
		if body+size > len(b) {
			break // truncated chunk
		}
		switch id {
		case "fmt ":
			if size >= 16 {
				channels := int(binary.LittleEndian.Uint16(b[body+2 : body+4]))
				w.sampleRate = int(binary.LittleEndian.Uint32(b[body+4 : body+8]))
				bits := int(binary.LittleEndian.Uint16(b[body+14 : body+16]))
				w.blockAlign = channels * bits / 8
			}
			w.fmtStart, w.fmtEnd = off, body+size
		case "data":
			w.dataStart, w.dataEnd = body, body+size
		case "smpl":
			// SamplerChunk: 9 uint32 (36 bytes); num_sample_loops at +28. Each
			// SampleLoop is 6 uint32 (24 bytes); start/end are at +8/+12. Use the first.
			if size >= 36+24 {
				num := binary.LittleEndian.Uint32(b[body+28 : body+32])
				if num > 0 {
					start := int(binary.LittleEndian.Uint32(b[body+36+8 : body+36+12]))
					end := int(binary.LittleEndian.Uint32(b[body+36+12 : body+36+16]))
					w.loopStartFrame = start
					w.loopEndFrame = end
					w.hasLoop = true
				}
			}
		}
		off = body + size + (size & 1) // chunks are word aligned
	}
	ok := w.fmtEnd > w.fmtStart && w.dataEnd > w.dataStart && w.blockAlign > 0
	return w, ok
}

// sliceFromLoop returns a standalone WAV holding only the running region
// samples from the smpl loop start up to the smpl loop end boundary; so looping
// it plays just the intended running sound. It returns nil when the WAV has no
// usable loop or cannot be parsed (the caller then loops the whole clip).
func sliceFromLoop(b []byte) []byte {
	w, ok := parseWAV(b)
	if !ok || !w.hasLoop {
		return nil
	}
	cut := w.dataStart + w.loopStartFrame*w.blockAlign
	if cut < w.dataStart || cut >= w.dataEnd {
		return nil // loop point outside the data
	}
	end := w.dataEnd
	if w.loopEndFrame > w.loopStartFrame {
		end = w.dataStart + w.loopEndFrame*w.blockAlign // the original passes this as an exclusive end.
		if end > w.dataEnd {
			end = w.dataEnd
		}
	}
	if end <= cut {
		return nil
	}
	fmtChunk := b[w.fmtStart:w.fmtEnd]
	data := b[cut:end]

	out := make([]byte, 0, 12+len(fmtChunk)+8+len(data))
	out = append(out, "RIFF"...)
	out = appendU32(out, uint32(4+len(fmtChunk)+8+len(data))) // size after this field
	out = append(out, "WAVE"...)
	out = append(out, fmtChunk...)
	out = append(out, "data"...)
	out = appendU32(out, uint32(len(data)))
	out = append(out, data...)
	return out
}

// introLoopWAV returns a standalone WAV holding samples [0, loopEnd), the intro
// plus the running region, but NOT the tail after the smpl loop end, together
// with loopStart, the time offset where the loop region begins. It feeds the
// single voice intro plus loop path: a backend plays the whole returned WAV
// once, then loops back to loopStart (its native loop point). Returns ok=false
// when the WAV has no usable loop region.
func introLoopWAV(b []byte) (wav []byte, loopStart time.Duration, ok bool) {
	w, parsed := parseWAV(b)
	if !parsed || !w.hasLoop || w.sampleRate <= 0 || w.blockAlign <= 0 {
		return nil, 0, false
	}
	if w.loopStartFrame < 0 || w.loopEndFrame <= w.loopStartFrame {
		return nil, 0, false
	}
	end := w.dataStart + w.loopEndFrame*w.blockAlign // exclusive: drop the post loop tail
	if end > w.dataEnd {
		end = w.dataEnd
	}
	if end <= w.dataStart {
		return nil, 0, false
	}
	fmtChunk := b[w.fmtStart:w.fmtEnd]
	data := b[w.dataStart:end]

	out := make([]byte, 0, 12+len(fmtChunk)+8+len(data))
	out = append(out, "RIFF"...)
	out = appendU32(out, uint32(4+len(fmtChunk)+8+len(data)))
	out = append(out, "WAVE"...)
	out = append(out, fmtChunk...)
	out = append(out, "data"...)
	out = appendU32(out, uint32(len(data)))
	out = append(out, data...)

	loopStart = time.Duration(w.loopStartFrame) * time.Second / time.Duration(w.sampleRate)
	return out, loopStart, true
}

func wavLoopEndDuration(b []byte) (time.Duration, bool) {
	w, ok := parseWAV(b)
	if !ok || !w.hasLoop || w.sampleRate <= 0 || w.blockAlign <= 0 || w.loopEndFrame <= 0 {
		return 0, false
	}
	totalFrames := (w.dataEnd - w.dataStart) / w.blockAlign
	frames := w.loopEndFrame
	if frames > totalFrames {
		frames = totalFrames
	}
	if frames <= 0 {
		return 0, false
	}
	return time.Duration(frames) * time.Second / time.Duration(w.sampleRate), true
}

/* // note: unused code in Go port
func wavDuration(b []byte) (time.Duration, bool) {
	w, ok := parseWAV(b)
	if !ok || w.sampleRate <= 0 || w.blockAlign <= 0 {
		return 0, false
	}
	frames := (w.dataEnd - w.dataStart) / w.blockAlign
	if frames <= 0 {
		return 0, false
	}
	return time.Duration(frames) * time.Second / time.Duration(w.sampleRate), true
}
*/

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
