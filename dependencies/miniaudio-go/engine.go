// engine.go : defines the high level engine for the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

// Engine is a high level miniaudio engine with an audio device, node graph, and resource manager.
// It owns sounds, groups, and listeners. It mirrors ma_engine.
type Engine struct {
	h uintptr
}

// Bindings for the maw_engine_* wrapper functions.
var (
	mawEngineCreate               func() uintptr
	mawEngineCreateEx             func(uint32, uint32, uint32) uintptr
	mawEngineCreateNull           func() uintptr
	mawEngineDestroy              func(uintptr)
	mawEngineStart                func(uintptr) int32
	mawEngineStop                 func(uintptr) int32
	mawEngineSetVolume            func(uintptr, float32) int32
	mawEngineGetSampleRate        func(uintptr) uint32
	mawEngineGetChannels          func(uintptr) uint32
	mawEngineGetTime              func(uintptr) uint64
	mawEnginePlaySound            func(uintptr, string) int32
	mawEngineListenerSetPosition  func(uintptr, uint32, float32, float32, float32)
	mawEngineListenerSetDirection func(uintptr, uint32, float32, float32, float32)
	mawEngineListenerSetWorldUp   func(uintptr, uint32, float32, float32, float32)
	mawEngineListenerSetCone      func(uintptr, uint32, float32, float32, float32)
)

func registerEngine() {
	reg(&mawEngineCreate, "maw_engine_create")
	reg(&mawEngineCreateEx, "maw_engine_create_ex")
	reg(&mawEngineCreateNull, "maw_engine_create_null")
	reg(&mawEngineDestroy, "maw_engine_destroy")
	reg(&mawEngineStart, "maw_engine_start")
	reg(&mawEngineStop, "maw_engine_stop")
	reg(&mawEngineSetVolume, "maw_engine_set_volume")
	reg(&mawEngineGetSampleRate, "maw_engine_get_sample_rate")
	reg(&mawEngineGetChannels, "maw_engine_get_channels")
	reg(&mawEngineGetTime, "maw_engine_get_time")
	reg(&mawEnginePlaySound, "maw_engine_play_sound")
	reg(&mawEngineListenerSetPosition, "maw_engine_listener_set_position")
	reg(&mawEngineListenerSetDirection, "maw_engine_listener_set_direction")
	reg(&mawEngineListenerSetWorldUp, "maw_engine_listener_set_world_up")
	reg(&mawEngineListenerSetCone, "maw_engine_listener_set_cone")
}

// NewEngine creates an engine with the default playback device, sample rate, and channel count.
func NewEngine() (*Engine, error) {
	h := mawEngineCreate()
	if h == 0 {
		return nil, errCreate("engine")
	}
	return &Engine{h: h}, nil
}

// NewEngineEx creates an engine with an explicit format.
// Pass 0 for any argument to keep the device default.
func NewEngineEx(sampleRate, channels, periodSizeInFrames uint32) (*Engine, error) {
	h := mawEngineCreateEx(sampleRate, channels, periodSizeInFrames)
	if h == 0 {
		return nil, errCreate("engine")
	}
	return &Engine{h: h}, nil
}

// NewNullEngine creates a silent engine with no device.
// It needs no audio hardware and is intended for headless tests, like SoLoud's NoSound backend.
func NewNullEngine() (*Engine, error) {
	h := mawEngineCreateNull()
	if h == 0 {
		return nil, errCreate("null engine")
	}
	return &Engine{h: h}, nil
}

// Destroy uninitializes the engine and frees it. Destroy sounds and groups first.
func (e *Engine) Destroy() {
	if e == nil || e.h == 0 {
		return
	}
	mawEngineDestroy(e.h)
	e.h = 0
}

// Start (re)starts the audio device. Engines start automatically on creation.
func (e *Engine) Start() error {
	return toErr(mawEngineStart(e.h))
}

// Stop pauses the audio device.
func (e *Engine) Stop() error {
	return toErr(mawEngineStop(e.h))
}

// SetVolume sets the master volume (1.0 == unity).
func (e *Engine) SetVolume(volume float32) error {
	return toErr(mawEngineSetVolume(e.h, volume))
}

// SampleRate returns the engine mixing sample rate in Hz.
func (e *Engine) SampleRate() uint32 {
	return mawEngineGetSampleRate(e.h)
}

// Channels returns the engine mixing channel count.
func (e *Engine) Channels() uint32 {
	return mawEngineGetChannels(e.h)
}

// TimeInFrames returns the engine's running time in PCM frames.
func (e *Engine) TimeInFrames() uint64 {
	return mawEngineGetTime(e.h)
}

// PlaySound plays a file fire and forget through the engine.
func (e *Engine) PlaySound(filePath string) error {
	return toErr(mawEnginePlaySound(e.h, filePath))
}

// SetListenerPosition sets the position of the given listener (usually 0).
func (e *Engine) SetListenerPosition(listenerIndex uint32, x, y, z float32) {
	mawEngineListenerSetPosition(e.h, listenerIndex, x, y, z)
}

// SetListenerDirection sets the forward direction of the given listener.
func (e *Engine) SetListenerDirection(listenerIndex uint32, x, y, z float32) {
	mawEngineListenerSetDirection(e.h, listenerIndex, x, y, z)
}

// SetListenerWorldUp sets the world up vector of the given listener.
func (e *Engine) SetListenerWorldUp(listenerIndex uint32, x, y, z float32) {
	mawEngineListenerSetWorldUp(e.h, listenerIndex, x, y, z)
}

// SetListenerCone sets the directional sound cone of the given listener.
func (e *Engine) SetListenerCone(listenerIndex uint32, innerAngleRadians, outerAngleRadians, outerGain float32) {
	mawEngineListenerSetCone(e.h, listenerIndex, innerAngleRadians, outerAngleRadians, outerGain)
}
