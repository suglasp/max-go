// sound.go : defines file and memory-backed sound playback for the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import "unsafe"

// Sound is a playable sound owned by an Engine and backed by a file or in-memory encoded buffer.
// It mirrors ma_sound.
type Sound struct {
	h uintptr
}

// Bindings for the maw_sound_* wrapper functions.
var (
	mawSoundCreateFromFile   func(uintptr, string, uint32) uintptr
	mawSoundCreateFromMemory func(uintptr, unsafe.Pointer, uint64, uint32) uintptr
	mawSoundDestroy          func(uintptr)
	mawSoundStart            func(uintptr) int32
	mawSoundStop             func(uintptr) int32
	mawSoundSetVolume        func(uintptr, float32)
	mawSoundGetVolume        func(uintptr) float32
	mawSoundSetPan           func(uintptr, float32)
	mawSoundSetPitch         func(uintptr, float32)
	mawSoundSetLooping       func(uintptr, int32)
	mawSoundIsLooping        func(uintptr) int32
	mawSoundIsPlaying        func(uintptr) int32
	mawSoundAtEnd            func(uintptr) int32
	mawSoundSeekToPCMFrame   func(uintptr, uint64) int32
	mawSoundGetLength        func(uintptr) uint64
	mawSoundSetPosition      func(uintptr, float32, float32, float32)
	mawSoundSetDirection     func(uintptr, float32, float32, float32)
	mawSoundSetAttenuation   func(uintptr, int32)
	mawSoundSetRolloff       func(uintptr, float32)
	mawSoundSetMinDistance   func(uintptr, float32)
	mawSoundSetMaxDistance   func(uintptr, float32)
	mawSoundSetFade          func(uintptr, float32, float32, uint64)
)

func registerSound() {
	reg(&mawSoundCreateFromFile, "maw_sound_create_from_file")
	reg(&mawSoundCreateFromMemory, "maw_sound_create_from_memory")
	reg(&mawSoundDestroy, "maw_sound_destroy")
	reg(&mawSoundStart, "maw_sound_start")
	reg(&mawSoundStop, "maw_sound_stop")
	reg(&mawSoundSetVolume, "maw_sound_set_volume")
	reg(&mawSoundGetVolume, "maw_sound_get_volume")
	reg(&mawSoundSetPan, "maw_sound_set_pan")
	reg(&mawSoundSetPitch, "maw_sound_set_pitch")
	reg(&mawSoundSetLooping, "maw_sound_set_looping")
	reg(&mawSoundIsLooping, "maw_sound_is_looping")
	reg(&mawSoundIsPlaying, "maw_sound_is_playing")
	reg(&mawSoundAtEnd, "maw_sound_at_end")
	reg(&mawSoundSeekToPCMFrame, "maw_sound_seek_to_pcm_frame")
	reg(&mawSoundGetLength, "maw_sound_get_length_in_pcm_frames")
	reg(&mawSoundSetPosition, "maw_sound_set_position")
	reg(&mawSoundSetDirection, "maw_sound_set_direction")
	reg(&mawSoundSetAttenuation, "maw_sound_set_attenuation_model")
	reg(&mawSoundSetRolloff, "maw_sound_set_rolloff")
	reg(&mawSoundSetMinDistance, "maw_sound_set_min_distance")
	reg(&mawSoundSetMaxDistance, "maw_sound_set_max_distance")
	reg(&mawSoundSetFade, "maw_sound_set_fade")
}

// NewSoundFromFile loads a sound from a file.
// flags is a combination of SoundFlag* values. Use 0 for defaults.
func (e *Engine) NewSoundFromFile(filePath string, flags uint32) (*Sound, error) {
	h := mawSoundCreateFromFile(e.h, filePath, flags)
	if h == 0 {
		return nil, errCreate("sound")
	}
	return &Sound{h: h}, nil
}

// NewSoundFromMemory decodes a sound from an in-memory encoded file such as WAV, FLAC, or MP3.
// The bytes are copied, so data need not stay alive after this call.
func (e *Engine) NewSoundFromMemory(data []byte, flags uint32) (*Sound, error) {
	if len(data) == 0 {
		return nil, errCreate("sound")
	}
	h := mawSoundCreateFromMemory(e.h, unsafe.Pointer(&data[0]), uint64(len(data)), flags)
	if h == 0 {
		return nil, errCreate("sound")
	}
	return &Sound{h: h}, nil
}

// Destroy uninitializes and frees the sound.
func (s *Sound) Destroy() {
	if s == nil || s.h == 0 {
		return
	}
	mawSoundDestroy(s.h)
	s.h = 0
}

// Start begins (or resumes) playback.
func (s *Sound) Start() error {
	return toErr(mawSoundStart(s.h))
}

// Stop pauses playback. Playback resumes from the same position on Start.
func (s *Sound) Stop() error {
	return toErr(mawSoundStop(s.h))
}

// SetVolume sets the linear volume (1.0 == unity).
func (s *Sound) SetVolume(volume float32) {
	mawSoundSetVolume(s.h, volume)
}

// Volume returns the current linear volume.
func (s *Sound) Volume() float32 {
	return mawSoundGetVolume(s.h)
}

// SetPan sets the stereo pan in [-1, 1] (left to right).
func (s *Sound) SetPan(pan float32) {
	mawSoundSetPan(s.h, pan)
}

// SetPitch sets the pitch multiplier (1.0 == original).
func (s *Sound) SetPitch(pitch float32) {
	mawSoundSetPitch(s.h, pitch)
}

// SetLooping enables or disables looping.
func (s *Sound) SetLooping(looping bool) {
	mawSoundSetLooping(s.h, b2i(looping))
}

// IsLooping reports whether the sound loops.
func (s *Sound) IsLooping() bool {
	return mawSoundIsLooping(s.h) != 0
}

// IsPlaying reports whether the sound is currently playing.
func (s *Sound) IsPlaying() bool {
	return mawSoundIsPlaying(s.h) != 0
}

// AtEnd reports whether playback has reached the end.
func (s *Sound) AtEnd() bool {
	return mawSoundAtEnd(s.h) != 0
}

// SeekToFrame seeks to the given PCM frame.
func (s *Sound) SeekToFrame(frameIndex uint64) error {
	return toErr(mawSoundSeekToPCMFrame(s.h, frameIndex))
}

// LengthInFrames returns the total length in PCM frames (0 if unknown).
func (s *Sound) LengthInFrames() uint64 {
	return mawSoundGetLength(s.h)
}

// SetPosition sets the 3D world position for spatialization.
func (s *Sound) SetPosition(x, y, z float32) {
	mawSoundSetPosition(s.h, x, y, z)
}

// SetDirection sets the 3D facing direction for spatialization.
func (s *Sound) SetDirection(x, y, z float32) {
	mawSoundSetDirection(s.h, x, y, z)
}

// SetAttenuationModel selects how volume falls off with distance.
func (s *Sound) SetAttenuationModel(model AttenuationModel) {
	mawSoundSetAttenuation(s.h, int32(model))
}

// SetRolloff sets the attenuation rolloff factor.
func (s *Sound) SetRolloff(rolloff float32) {
	mawSoundSetRolloff(s.h, rolloff)
}

// SetMinDistance sets the distance below which there is no attenuation.
func (s *Sound) SetMinDistance(minDistance float32) {
	mawSoundSetMinDistance(s.h, minDistance)
}

// SetMaxDistance sets the distance beyond which attenuation no longer increases.
func (s *Sound) SetMaxDistance(maxDistance float32) {
	mawSoundSetMaxDistance(s.h, maxDistance)
}

// Fade changes the volume from volumeBeg to volumeEnd over the given milliseconds.
// Pass a negative volumeBeg to start at the current volume.
func (s *Sound) Fade(volumeBeg, volumeEnd float32, fadeLengthMs uint64) {
	mawSoundSetFade(s.h, volumeBeg, volumeEnd, fadeLengthMs)
}
