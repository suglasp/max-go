// enums.go : defines sample formats and other miniaudio.h enumerations.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

// Format is a sample format, mirroring ma_format (miniaudio.h ~line 4311).
type Format int32

const (
	FormatUnknown Format = 0
	FormatU8      Format = 1
	FormatS16     Format = 2 // most widely supported
	FormatS24     Format = 3 // tightly packed, 3 bytes/sample
	FormatS32     Format = 4
	FormatF32     Format = 5
)

// BytesPerSample returns the number of bytes one sample of the format occupies.
func (f Format) BytesPerSample() int {
	switch f {
	case FormatU8:
		return 1
	case FormatS16:
		return 2
	case FormatS24:
		return 3
	case FormatS32, FormatF32:
		return 4
	default:
		return 0
	}
}

// Sound flags mirror ma_sound_flags (miniaudio.h ~line 11122).
// Combine them with bitwise OR and pass them to NewSoundFromFile or NewSoundFromMemory.
const (
	SoundFlagStream           uint32 = 0x00000001 // stream from disk instead of fully loading
	SoundFlagDecode           uint32 = 0x00000002 // decode up front rather than at mix time
	SoundFlagAsync            uint32 = 0x00000004 // load asynchronously
	SoundFlagWaitInit         uint32 = 0x00000008
	SoundFlagUnknownLength    uint32 = 0x00000010
	SoundFlagLooping          uint32 = 0x00000020
	SoundFlagNoDefaultAttach  uint32 = 0x00001000 // do not attach to the endpoint by default
	SoundFlagNoPitch          uint32 = 0x00002000 // disable pitch shifting (optimization)
	SoundFlagNoSpatialization uint32 = 0x00004000 // disable 3D spatialization
)

// AttenuationModel mirrors ma_attenuation_model (miniaudio.h ~line 5153).
type AttenuationModel int32

const (
	AttenuationNone        AttenuationModel = 0
	AttenuationInverse     AttenuationModel = 1 // AL_INVERSE_DISTANCE_CLAMPED
	AttenuationLinear      AttenuationModel = 2 // AL_LINEAR_DISTANCE_CLAMPED
	AttenuationExponential AttenuationModel = 3 // AL_EXPONENT_DISTANCE_CLAMPED
)
