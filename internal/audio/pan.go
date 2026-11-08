// pan.go : the M.A.X. audio panning (play a sample most left or most right in stereo mode)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

import (
	"encoding/binary"
	"math"
)

// panGains returns the left/right channel gains for a stereo pan in [-1, 1]:
// pan < 0 attenuates the right channel, pan > 0 attenuates the left, and 0
// leaves both channels at unity. Shared by the fixed pan copy (panStereoPCM)
// and the streaming reader that can be panned again live (panReader) so the
// math never drifts.
func panGains(pan float64) (leftGain, rightGain float64) {
	pan = clampPan(pan)
	leftGain, rightGain = 1.0, 1.0

	// A pan toward one side drops the opposite channel's gain.
	if pan < 0 {
		rightGain = 1 + pan
	} else if pan > 0 {
		leftGain = 1 - pan
	}

	return leftGain, rightGain
}

// panStereoInPlace applies pan to the leading whole frames of signed 16 bit
// little endian stereo PCM, mutating data directly. Any partial frame at the
// tail (len not a multiple of frameSize) is left untouched so the channels
// never drift out of phase across a streamed read boundary.
func panStereoInPlace(data []byte, pan float64) {
	leftGain, rightGain := panGains(pan)

	// Centre pan (both gains unity) does nothing; skip the work per sample.
	if leftGain == 1 && rightGain == 1 {
		return
	}

	end := len(data) / frameSize * frameSize
	for i := 0; i < end; i += frameSize {
		left := int16(binary.LittleEndian.Uint16(data[i : i+2]))
		right := int16(binary.LittleEndian.Uint16(data[i+2 : i+4]))

		binary.LittleEndian.PutUint16(data[i:i+2], uint16(scaleSample16(left, leftGain)))
		binary.LittleEndian.PutUint16(data[i+2:i+4], uint16(scaleSample16(right, rightGain)))
	}
}

// panStereoPCM applies a fixed pan to signed 16 bit little endian stereo PCM,
// returning a new slice. Pan is clamped to [-1, 1]: -1 keeps only the left
// channel, +1 keeps only the right channel, and 0 returns the original byte
// slice unchanged (no copy).
func panStereoPCM(data []byte, pan float64) []byte {
	if clampPan(pan) == 0 || len(data) < frameSize {
		return data
	}

	out := make([]byte, len(data))
	copy(out, data)
	panStereoInPlace(out, pan)
	return out
}

// Rescale the sample to PCM16.
func scaleSample16(sample int16, gain float64) int16 {
	v := math.Round(float64(sample) * gain)

	if v > math.MaxInt16 {
		return math.MaxInt16
	}

	if v < math.MinInt16 {
		return math.MinInt16
	}

	return int16(v)
}

// Clamp pan to the stereo field.
func clampPan(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}

	if v < -1 {
		return -1
	}

	if v > 1 {
		return 1
	}

	return v
}

// ClampVolume32 normalizes a 0..1 volume to the float32 scale the SoLoud and miniaudio backends take; NaN and
// negatives become 0.
func ClampVolume32(v float64) float32 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}

	return float32(min(v, 1))
}

// ClampPan32 normalizes a stereo pan to [-1, 1] as float32 for the SoLoud and miniaudio backends; NaN becomes 0.
func ClampPan32(v float64) float32 {
	return float32(clampPan(v))
}
