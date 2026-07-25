// pcm.go : PCM helpers: converts signed 16 bit PCM to little endian stereo byte streams for the audio backend.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"io"
	"sync"
)

/* // note: unused code in Go port
// PCMBytes converts signed 16 bit PCM to little endian stereo bytes.
func PCMBytes(samples []int16, format PCMFormat, outRate int) []byte {
	out, _ := PCMBytesPhase(samples, format, outRate, 0)
	return out
}
*/

// PCMBytesPhase converts PCM while preserving resampler phase across chunks.
func PCMBytesPhase(samples []int16, format PCMFormat, outRate, phase int) ([]byte, int) {
	out16, phase := PCMResamplePhase(samples, format, outRate, phase)
	if len(out16) == 0 {
		return nil, phase
	}
	out := make([]byte, 0, len(out16)*2)
	for _, s := range out16 {
		out = appendSample16(out, s)
	}
	return out, phase
}

// PCMResamplePhase resamples signed 16 bit PCM to outRate and upmixes to stereo,
// preserving resampler phase across chunks. The returned samples are interleaved
// stereo (L,R) at outRate.
//
// This is the shared core of the movie PCM path. Carrying the phase across chunks avoids discontinuities when the movie and device rates differ.
func PCMResamplePhase(samples []int16, format PCMFormat, outRate, phase int) ([]int16, int) {
	inRate := format.Rate
	if inRate <= 0 {
		inRate = 22050
	}
	inChannels := format.Channels
	if inChannels != 1 && inChannels != 2 {
		inChannels = 2
	}
	if outRate <= 0 {
		outRate = inRate
	}
	inFrames := len(samples) / inChannels
	if inFrames == 0 {
		return nil, phase
	}
	if phase < 0 || phase >= outRate {
		phase = 0
	}
	outFrames := ((inFrames*outRate - phase) + inRate - 1) / inRate
	if outFrames < 0 {
		outFrames = 0
	}
	out := make([]int16, 0, outFrames*2)
	for i := 0; i < outFrames; i++ {
		srcFrame := phase / outRate
		if srcFrame >= inFrames {
			break
		}
		base := srcFrame * inChannels
		left := samples[base]
		right := left
		if inChannels == 2 {
			right = samples[base+1]
		}
		out = append(out, left, right)
		phase += inRate
	}
	phase -= inFrames * outRate
	return out, phase
}

// appendSample16 appends one signed 16 bit sample in little endian order.
func appendSample16(out []byte, sample int16) []byte {
	u := uint16(sample)
	return append(out, byte(u), byte(u>>8))
}

// pcmStream is a non blocking byte queue for little endian signed 16 bit
// stereo PCM. Underruns return silence so the audio device cannot stall.
type pcmStream struct {
	mu     sync.Mutex
	buf    []byte
	off    int
	closed bool
}

// newPCMStream builds the byte queue consumed by the Oto player.
func newPCMStream() *pcmStream {
	return &pcmStream{}
}

// Write appends decoded PCM bytes for future reads by the audio device.
func (s *pcmStream) Write(p []byte) {
	if len(p) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.off > 0 && (s.off == len(s.buf) || s.off > 64*1024) {
		copy(s.buf, s.buf[s.off:])
		s.buf = s.buf[:len(s.buf)-s.off]
		s.off = 0
	}
	s.buf = append(s.buf, p...)
}

// Read fills p from queued PCM bytes, padding underruns with silence.
func (s *pcmStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.off >= len(s.buf) {
		if s.closed {
			return 0, io.EOF
		}
		for i := range p {
			p[i] = 0
		}
		return len(p), nil
	}
	n := copy(p, s.buf[s.off:])
	s.off += n
	if s.off == len(s.buf) {
		s.buf = s.buf[:0]
		s.off = 0
	}
	if n < len(p) {
		for i := n; i < len(p); i++ {
			p[i] = 0
		}
		return len(p), nil
	}
	return n, nil
}

// Close marks the stream complete so drained reads can report EOF.
func (s *pcmStream) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}
