// group.go : defines SoundGroup, a submix bus for the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

// SoundGroup is a submix bus that sounds can use for grouped volume control.
// It mirrors ma_sound_group.
type SoundGroup struct {
	h uintptr
}

var (
	mawSoundGroupCreate    func(uintptr, uint32) uintptr
	mawSoundGroupDestroy   func(uintptr)
	mawSoundGroupStart     func(uintptr) int32
	mawSoundGroupStop      func(uintptr) int32
	mawSoundGroupSetVolume func(uintptr, float32)
)

func registerGroup() {
	reg(&mawSoundGroupCreate, "maw_sound_group_create")
	reg(&mawSoundGroupDestroy, "maw_sound_group_destroy")
	reg(&mawSoundGroupStart, "maw_sound_group_start")
	reg(&mawSoundGroupStop, "maw_sound_group_stop")
	reg(&mawSoundGroupSetVolume, "maw_sound_group_set_volume")
}

// NewSoundGroup creates a sound group attached to the engine endpoint.
// flags is a combination of SoundFlag* values. Use 0 for defaults.
func (e *Engine) NewSoundGroup(flags uint32) (*SoundGroup, error) {
	h := mawSoundGroupCreate(e.h, flags)
	if h == 0 {
		return nil, errCreate("sound group")
	}
	return &SoundGroup{h: h}, nil
}

// Destroy uninitializes and frees the group.
func (g *SoundGroup) Destroy() {
	if g == nil || g.h == 0 {
		return
	}
	mawSoundGroupDestroy(g.h)
	g.h = 0
}

// Start starts the group.
func (g *SoundGroup) Start() error {
	return toErr(mawSoundGroupStart(g.h))
}

// Stop stops the group.
func (g *SoundGroup) Stop() error {
	return toErr(mawSoundGroupStop(g.h))
}

// SetVolume sets the group's linear volume (1.0 == unity).
func (g *SoundGroup) SetVolume(volume float32) {
	mawSoundGroupSetVolume(g.h, volume)
}
