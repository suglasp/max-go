// silent.go : the silent audio backend, a Backend implementation that does nothing, for headless/test runs without an audio device.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package audio

// SilentBackend satisfies Backend when no audio device is available.
type SilentBackend struct{}

// NewSilentBackend creates a backend that does nothing, used when audio is disabled.
func NewSilentBackend() *SilentBackend {
	return &SilentBackend{}
}

// PlayMusic ignores music playback requests.
func (b *SilentBackend) PlayMusic(string, bool, float64) error {
	return nil
}

// StopMusic ignores music stop requests.
func (b *SilentBackend) StopMusic() {}

// SetMusicVolume ignores music volume changes.
func (b *SilentBackend) SetMusicVolume(float64) {}

// PlaySfx ignores sound effect playback requests.
func (b *SilentBackend) PlaySfx(string, float64, float64) error {
	return nil
}

// PlayVoice ignores voice playback requests.
func (b *SilentBackend) PlayVoice(string, float64) error {
	return nil
}

// VoicePlaying always reports no active voice.
func (b *SilentBackend) VoicePlaying() bool {
	return false
}

// StopAll ignores global stop requests.
func (b *SilentBackend) StopAll() {}

// Close releases no resources and always succeeds.
func (b *SilentBackend) Close() error {
	return nil
}
