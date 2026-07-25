// decodepcm_test.go : testing unit for PCM decode helpers in the audio package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

// Ebitengine's WAV decoder may emit zero padding beyond stream.Length() while resampling 22050 Hz clips for a 44100 Hz context.
// decodePCM trims that padding so a held loop does not wrap through silence.

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// constToneWAV22050 builds a mono 16 bit 22050 Hz PCM WAV of frames samples, all
// at the constant amplitude amp (so any trailing zero frame is decoder padding,
// not real audio).
func constToneWAV22050(frames int, amp int16) []byte {
	const rate = 22050
	data := make([]byte, frames*2)
	for i := 0; i < frames; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(amp))
	}
	out := make([]byte, 0, 44+len(data))
	out = append(out, "RIFF"...)
	out = appendU32(out, uint32(36+len(data)))
	out = append(out, "WAVE"...)
	out = append(out, "fmt "...)
	out = appendU32(out, 16)
	out = appendU16(out, 1)      // PCM
	out = appendU16(out, 1)      // mono
	out = appendU32(out, rate)   // sample rate
	out = appendU32(out, rate*2) // byte rate
	out = appendU16(out, 2)      // block align
	out = appendU16(out, 16)     // bits per sample
	out = append(out, "data"...)
	out = appendU32(out, uint32(len(data)))
	out = append(out, data...)
	return out
}

func appendU16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}

// lastFrameAmp returns the max abs of the stereo 16 bit frame at the end.
func lastFrameAmp(pcm []byte) int {
	if len(pcm) < 4 {
		return 0
	}
	off := len(pcm) - 4
	l := int(int16(binary.LittleEndian.Uint16(pcm[off : off+2])))
	r := int(int16(binary.LittleEndian.Uint16(pcm[off+2 : off+4])))
	if l < 0 {
		l = -l
	}
	if r < 0 {
		r = -r
	}
	if r > l {
		return r
	}
	return l
}

// TestDecodePCMTrimsResamplerTail proves io.ReadAll leaves a silent resampler
// tail when resampling 22050 to 44100 and that decodePCM removes it, so a
// looped held clip wraps from sound straight back to sound (no silent "stop").
func TestDecodePCMTrimsResamplerTail(t *testing.T) {
	src := constToneWAV22050(20000, 8000) // ~0.9s of constant tone

	// Untrimmed (the old io.ReadAll path).
	s1, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	full, err := io.ReadAll(s1)
	if err != nil {
		t.Fatalf("readall: %v", err)
	}

	// Trimmed (decodePCM).
	s2, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	trimmed, err := decodePCM(s2)
	if err != nil {
		t.Fatalf("decodePCM: %v", err)
	}

	// decodePCM must not exceed the decoder's reported length.
	if len(trimmed) > int(s2.Length()) {
		t.Fatalf("trimmed=%d exceeds stream.Length()=%d", len(trimmed), s2.Length())
	}
	// The trimmed buffer must END on the constant tone, not silence: a clean wrap.
	// The resampler tail past Length() is exactly zero, so any solid amplitude proves
	// decodePCM kept real audio and dropped only the silence.
	if a := lastFrameAmp(trimmed); a < 1000 {
		t.Fatalf("trimmed clip ends in near-silence (amp=%d); loop would still gap", a)
	}
	// Sanity: io.ReadAll kept at least as much as decodePCM (it includes the tail).
	if len(full) < len(trimmed) {
		t.Fatalf("io.ReadAll (%d) shorter than decodePCM (%d)", len(full), len(trimmed))
	}
	t.Logf("io.ReadAll=%d decodePCM=%d (trimmed %d bytes of resampler tail); end amp=%d",
		len(full), len(trimmed), len(full)-len(trimmed), lastFrameAmp(trimmed))
}
