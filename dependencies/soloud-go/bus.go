// bus.go : defines Bus audio-source bindings for the SoLoud Go binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

// Bindings for the Bus_* functions unique to Bus.
// registerSourceFuncs("Bus") binds the common audio-source functions.
var (
	busFuncs *sourceFuncs

	busCreate                 func() uintptr
	busPlay                   func(uintptr, uintptr) uint32
	busPlayEx                 func(uintptr, uintptr, float32, float32, bool) uint32
	busPlayClocked            func(uintptr, float64, uintptr) uint32
	busPlayClockedEx          func(uintptr, float64, uintptr, float32, float32) uint32
	busPlay3d                 func(uintptr, uintptr, float32, float32, float32) uint32
	busPlay3dEx               func(uintptr, uintptr, float32, float32, float32, float32, float32, float32, float32, bool) uint32
	busPlay3dClocked          func(uintptr, float64, uintptr, float32, float32, float32) uint32
	busPlay3dClockedEx        func(uintptr, float64, uintptr, float32, float32, float32, float32, float32, float32, float32) uint32
	busSetChannels            func(uintptr, uint32) int32
	busSetVisualizationEnable func(uintptr, bool)
	busAnnexSound             func(uintptr, uint32)
	busCalcFFT                func(uintptr) *float32
	busGetWave                func(uintptr) *float32
	busGetApproximateVolume   func(uintptr, uint32) float32
	busGetActiveVoiceCount    func(uintptr) uint32
)

func registerBus() {
	busFuncs = registerSourceFuncs("Bus")
	reg(&busCreate, "Bus_create")
	reg(&busPlay, "Bus_play")
	reg(&busPlayEx, "Bus_playEx")
	reg(&busPlayClocked, "Bus_playClocked")
	reg(&busPlayClockedEx, "Bus_playClockedEx")
	reg(&busPlay3d, "Bus_play3d")
	reg(&busPlay3dEx, "Bus_play3dEx")
	reg(&busPlay3dClocked, "Bus_play3dClocked")
	reg(&busPlay3dClockedEx, "Bus_play3dClockedEx")
	reg(&busSetChannels, "Bus_setChannels")
	reg(&busSetVisualizationEnable, "Bus_setVisualizationEnable")
	reg(&busAnnexSound, "Bus_annexSound")
	reg(&busCalcFFT, "Bus_calcFFT")
	reg(&busGetWave, "Bus_getWave")
	reg(&busGetApproximateVolume, "Bus_getApproximateVolume")
	reg(&busGetActiveVoiceCount, "Bus_getActiveVoiceCount")
}

// Bus is a mixing bus and an AudioSource that can play on the engine or another bus.
// Other sounds can play through it.
type Bus struct{ audioSource }

func NewBus() *Bus {
	return &Bus{audioSource{ptr: busCreate(), fn: busFuncs}}
}

func (b *Bus) Play(aSound AudioSource) uint32 {
	return busPlay(b.ptr, sourcePtr(aSound))
}
func (b *Bus) PlayEx(aSound AudioSource, aVolume, aPan float32, aPaused bool) uint32 {
	return busPlayEx(b.ptr, sourcePtr(aSound), aVolume, aPan, aPaused)
}
func (b *Bus) PlayClocked(aSoundTime float64, aSound AudioSource) uint32 {
	return busPlayClocked(b.ptr, aSoundTime, sourcePtr(aSound))
}
func (b *Bus) PlayClockedEx(aSoundTime float64, aSound AudioSource, aVolume, aPan float32) uint32 {
	return busPlayClockedEx(b.ptr, aSoundTime, sourcePtr(aSound), aVolume, aPan)
}
func (b *Bus) Play3d(aSound AudioSource, aPosX, aPosY, aPosZ float32) uint32 {
	return busPlay3d(b.ptr, sourcePtr(aSound), aPosX, aPosY, aPosZ)
}
func (b *Bus) Play3dEx(aSound AudioSource, aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume float32, aPaused bool) uint32 {
	return busPlay3dEx(b.ptr, sourcePtr(aSound), aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume, aPaused)
}
func (b *Bus) Play3dClocked(aSoundTime float64, aSound AudioSource, aPosX, aPosY, aPosZ float32) uint32 {
	return busPlay3dClocked(b.ptr, aSoundTime, sourcePtr(aSound), aPosX, aPosY, aPosZ)
}
func (b *Bus) Play3dClockedEx(aSoundTime float64, aSound AudioSource, aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume float32) uint32 {
	return busPlay3dClockedEx(b.ptr, aSoundTime, sourcePtr(aSound), aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume)
}
func (b *Bus) SetChannels(aChannels uint32) error {
	return toErr(busSetChannels(b.ptr, aChannels))
}
func (b *Bus) SetVisualizationEnable(aEnable bool) {
	busSetVisualizationEnable(b.ptr, aEnable)
}

// AnnexSound moves a playing voice from another bus (or the engine) to this bus.
func (b *Bus) AnnexSound(aVoiceHandle uint32) {
	busAnnexSound(b.ptr, aVoiceHandle)
}

// CalcFFT returns a copy of the 256-float FFT data in the bus visualization buffer.
// Enable visualization before calling it.
func (b *Bus) CalcFFT() []float32 {
	return copyFloats(busCalcFFT(b.ptr))
}

// GetWave returns a copy of the 256-float waveform data in the bus visualization buffer.
// Enable visualization before calling it.
func (b *Bus) GetWave() []float32 {
	return copyFloats(busGetWave(b.ptr))
}

func (b *Bus) GetApproximateVolume(aChannel uint32) float32 {
	return busGetApproximateVolume(b.ptr, aChannel)
}
func (b *Bus) GetActiveVoiceCount() uint32 {
	return busGetActiveVoiceCount(b.ptr)
}
