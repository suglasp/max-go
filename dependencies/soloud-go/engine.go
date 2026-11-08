// engine.go : defines Soloud engine bindings for the SoLoud Go binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import "unsafe"

// Bindings for the Soloud_* functions of soloud_c.h.
var (
	soloudDestroy                    func(uintptr)
	soloudCreate                     func() uintptr
	soloudInit                       func(uintptr) int32
	soloudInitEx                     func(uintptr, uint32, uint32, uint32, uint32, uint32) int32
	soloudDeinit                     func(uintptr)
	soloudGetVersion                 func(uintptr) uint32
	soloudGetErrorString             func(uintptr, int32) string
	soloudGetBackendId               func(uintptr) uint32
	soloudGetBackendString           func(uintptr) string
	soloudGetBackendChannels         func(uintptr) uint32
	soloudGetBackendSamplerate       func(uintptr) uint32
	soloudGetBackendBufferSize       func(uintptr) uint32
	soloudSetSpeakerPosition         func(uintptr, uint32, float32, float32, float32) int32
	soloudGetSpeakerPosition         func(uintptr, uint32, *float32, *float32, *float32) int32
	soloudPlay                       func(uintptr, uintptr) uint32
	soloudPlayEx                     func(uintptr, uintptr, float32, float32, bool, uint32) uint32
	soloudPlayClocked                func(uintptr, float64, uintptr) uint32
	soloudPlayClockedEx              func(uintptr, float64, uintptr, float32, float32, uint32) uint32
	soloudPlay3d                     func(uintptr, uintptr, float32, float32, float32) uint32
	soloudPlay3dEx                   func(uintptr, uintptr, float32, float32, float32, float32, float32, float32, float32, bool, uint32) uint32
	soloudPlay3dClocked              func(uintptr, float64, uintptr, float32, float32, float32) uint32
	soloudPlay3dClockedEx            func(uintptr, float64, uintptr, float32, float32, float32, float32, float32, float32, float32, uint32) uint32
	soloudPlayBackground             func(uintptr, uintptr) uint32
	soloudPlayBackgroundEx           func(uintptr, uintptr, float32, bool, uint32) uint32
	soloudSeek                       func(uintptr, uint32, float64) int32
	soloudStop                       func(uintptr, uint32)
	soloudStopAll                    func(uintptr)
	soloudStopAudioSource            func(uintptr, uintptr)
	soloudCountAudioSource           func(uintptr, uintptr) int32
	soloudSetFilterParameter         func(uintptr, uint32, uint32, uint32, float32)
	soloudGetFilterParameter         func(uintptr, uint32, uint32, uint32) float32
	soloudFadeFilterParameter        func(uintptr, uint32, uint32, uint32, float32, float64)
	soloudOscillateFilterParameter   func(uintptr, uint32, uint32, uint32, float32, float32, float64)
	soloudGetStreamTime              func(uintptr, uint32) float64
	soloudGetStreamPosition          func(uintptr, uint32) float64
	soloudGetPause                   func(uintptr, uint32) bool
	soloudGetVolume                  func(uintptr, uint32) float32
	soloudGetOverallVolume           func(uintptr, uint32) float32
	soloudGetPan                     func(uintptr, uint32) float32
	soloudGetSamplerate              func(uintptr, uint32) float32
	soloudGetProtectVoice            func(uintptr, uint32) bool
	soloudGetActiveVoiceCount        func(uintptr) uint32
	soloudGetVoiceCount              func(uintptr) uint32
	soloudIsValidVoiceHandle         func(uintptr, uint32) bool
	soloudGetRelativePlaySpeed       func(uintptr, uint32) float32
	soloudGetPostClipScaler          func(uintptr) float32
	soloudGetGlobalVolume            func(uintptr) float32
	soloudGetMaxActiveVoiceCount     func(uintptr) uint32
	soloudGetLooping                 func(uintptr, uint32) bool
	soloudGetLoopPoint               func(uintptr, uint32) float64
	soloudSetLoopPoint               func(uintptr, uint32, float64)
	soloudSetLooping                 func(uintptr, uint32, bool)
	soloudSetMaxActiveVoiceCount     func(uintptr, uint32) int32
	soloudSetInaudibleBehavior       func(uintptr, uint32, bool, bool)
	soloudSetGlobalVolume            func(uintptr, float32)
	soloudSetPostClipScaler          func(uintptr, float32)
	soloudSetPause                   func(uintptr, uint32, bool)
	soloudSetPauseAll                func(uintptr, bool)
	soloudSetRelativePlaySpeed       func(uintptr, uint32, float32) int32
	soloudSetProtectVoice            func(uintptr, uint32, bool)
	soloudSetSamplerate              func(uintptr, uint32, float32)
	soloudSetPan                     func(uintptr, uint32, float32)
	soloudSetPanAbsolute             func(uintptr, uint32, float32, float32)
	soloudSetPanAbsoluteEx           func(uintptr, uint32, float32, float32, float32, float32, float32, float32)
	soloudSetVolume                  func(uintptr, uint32, float32)
	soloudSetDelaySamples            func(uintptr, uint32, uint32)
	soloudFadeVolume                 func(uintptr, uint32, float32, float64)
	soloudFadePan                    func(uintptr, uint32, float32, float64)
	soloudFadeRelativePlaySpeed      func(uintptr, uint32, float32, float64)
	soloudFadeGlobalVolume           func(uintptr, float32, float64)
	soloudSchedulePause              func(uintptr, uint32, float64)
	soloudScheduleStop               func(uintptr, uint32, float64)
	soloudOscillateVolume            func(uintptr, uint32, float32, float32, float64)
	soloudOscillatePan               func(uintptr, uint32, float32, float32, float64)
	soloudOscillateRelativePlaySpeed func(uintptr, uint32, float32, float32, float64)
	soloudOscillateGlobalVolume      func(uintptr, float32, float32, float64)
	soloudSetGlobalFilter            func(uintptr, uint32, uintptr)
	soloudSetVisualizationEnable     func(uintptr, bool)
	soloudCalcFFT                    func(uintptr) *float32
	soloudGetWave                    func(uintptr) *float32
	soloudGetApproximateVolume       func(uintptr, uint32) float32
	soloudGetLoopCount               func(uintptr, uint32) uint32
	soloudGetInfo                    func(uintptr, uint32, uint32) float32
	soloudCreateVoiceGroup           func(uintptr) uint32
	soloudDestroyVoiceGroup          func(uintptr, uint32) int32
	soloudAddVoiceToGroup            func(uintptr, uint32, uint32) int32
	soloudIsVoiceGroup               func(uintptr, uint32) bool
	soloudIsVoiceGroupEmpty          func(uintptr, uint32) bool
	soloudUpdate3dAudio              func(uintptr)
	soloudSet3dSoundSpeed            func(uintptr, float32) int32
	soloudGet3dSoundSpeed            func(uintptr) float32
	soloudSet3dListenerParameters    func(uintptr, float32, float32, float32, float32, float32, float32, float32, float32, float32)
	soloudSet3dListenerParametersEx  func(uintptr, float32, float32, float32, float32, float32, float32, float32, float32, float32, float32, float32, float32)
	soloudSet3dListenerPosition      func(uintptr, float32, float32, float32)
	soloudSet3dListenerAt            func(uintptr, float32, float32, float32)
	soloudSet3dListenerUp            func(uintptr, float32, float32, float32)
	soloudSet3dListenerVelocity      func(uintptr, float32, float32, float32)
	soloudSet3dSourceParameters      func(uintptr, uint32, float32, float32, float32)
	soloudSet3dSourceParametersEx    func(uintptr, uint32, float32, float32, float32, float32, float32, float32)
	soloudSet3dSourcePosition        func(uintptr, uint32, float32, float32, float32)
	soloudSet3dSourceVelocity        func(uintptr, uint32, float32, float32, float32)
	soloudSet3dSourceMinMaxDistance  func(uintptr, uint32, float32, float32)
	soloudSet3dSourceAttenuation     func(uintptr, uint32, uint32, float32)
	soloudSet3dSourceDopplerFactor   func(uintptr, uint32, float32)
	soloudMix                        func(uintptr, *float32, uint32)
	soloudMixSigned16                func(uintptr, *int16, uint32)
)

func registerEngine() {
	reg(&soloudDestroy, "Soloud_destroy")
	reg(&soloudCreate, "Soloud_create")
	reg(&soloudInit, "Soloud_init")
	reg(&soloudInitEx, "Soloud_initEx")
	reg(&soloudDeinit, "Soloud_deinit")
	reg(&soloudGetVersion, "Soloud_getVersion")
	reg(&soloudGetErrorString, "Soloud_getErrorString")
	reg(&soloudGetBackendId, "Soloud_getBackendId")
	reg(&soloudGetBackendString, "Soloud_getBackendString")
	reg(&soloudGetBackendChannels, "Soloud_getBackendChannels")
	reg(&soloudGetBackendSamplerate, "Soloud_getBackendSamplerate")
	reg(&soloudGetBackendBufferSize, "Soloud_getBackendBufferSize")
	reg(&soloudSetSpeakerPosition, "Soloud_setSpeakerPosition")
	reg(&soloudGetSpeakerPosition, "Soloud_getSpeakerPosition")
	reg(&soloudPlay, "Soloud_play")
	reg(&soloudPlayEx, "Soloud_playEx")
	reg(&soloudPlayClocked, "Soloud_playClocked")
	reg(&soloudPlayClockedEx, "Soloud_playClockedEx")
	reg(&soloudPlay3d, "Soloud_play3d")
	reg(&soloudPlay3dEx, "Soloud_play3dEx")
	reg(&soloudPlay3dClocked, "Soloud_play3dClocked")
	reg(&soloudPlay3dClockedEx, "Soloud_play3dClockedEx")
	reg(&soloudPlayBackground, "Soloud_playBackground")
	reg(&soloudPlayBackgroundEx, "Soloud_playBackgroundEx")
	reg(&soloudSeek, "Soloud_seek")
	reg(&soloudStop, "Soloud_stop")
	reg(&soloudStopAll, "Soloud_stopAll")
	reg(&soloudStopAudioSource, "Soloud_stopAudioSource")
	reg(&soloudCountAudioSource, "Soloud_countAudioSource")
	reg(&soloudSetFilterParameter, "Soloud_setFilterParameter")
	reg(&soloudGetFilterParameter, "Soloud_getFilterParameter")
	reg(&soloudFadeFilterParameter, "Soloud_fadeFilterParameter")
	reg(&soloudOscillateFilterParameter, "Soloud_oscillateFilterParameter")
	reg(&soloudGetStreamTime, "Soloud_getStreamTime")
	reg(&soloudGetStreamPosition, "Soloud_getStreamPosition")
	reg(&soloudGetPause, "Soloud_getPause")
	reg(&soloudGetVolume, "Soloud_getVolume")
	reg(&soloudGetOverallVolume, "Soloud_getOverallVolume")
	reg(&soloudGetPan, "Soloud_getPan")
	reg(&soloudGetSamplerate, "Soloud_getSamplerate")
	reg(&soloudGetProtectVoice, "Soloud_getProtectVoice")
	reg(&soloudGetActiveVoiceCount, "Soloud_getActiveVoiceCount")
	reg(&soloudGetVoiceCount, "Soloud_getVoiceCount")
	reg(&soloudIsValidVoiceHandle, "Soloud_isValidVoiceHandle")
	reg(&soloudGetRelativePlaySpeed, "Soloud_getRelativePlaySpeed")
	reg(&soloudGetPostClipScaler, "Soloud_getPostClipScaler")
	reg(&soloudGetGlobalVolume, "Soloud_getGlobalVolume")
	reg(&soloudGetMaxActiveVoiceCount, "Soloud_getMaxActiveVoiceCount")
	reg(&soloudGetLooping, "Soloud_getLooping")
	reg(&soloudGetLoopPoint, "Soloud_getLoopPoint")
	reg(&soloudSetLoopPoint, "Soloud_setLoopPoint")
	reg(&soloudSetLooping, "Soloud_setLooping")
	reg(&soloudSetMaxActiveVoiceCount, "Soloud_setMaxActiveVoiceCount")
	reg(&soloudSetInaudibleBehavior, "Soloud_setInaudibleBehavior")
	reg(&soloudSetGlobalVolume, "Soloud_setGlobalVolume")
	reg(&soloudSetPostClipScaler, "Soloud_setPostClipScaler")
	reg(&soloudSetPause, "Soloud_setPause")
	reg(&soloudSetPauseAll, "Soloud_setPauseAll")
	reg(&soloudSetRelativePlaySpeed, "Soloud_setRelativePlaySpeed")
	reg(&soloudSetProtectVoice, "Soloud_setProtectVoice")
	reg(&soloudSetSamplerate, "Soloud_setSamplerate")
	reg(&soloudSetPan, "Soloud_setPan")
	reg(&soloudSetPanAbsolute, "Soloud_setPanAbsolute")
	reg(&soloudSetPanAbsoluteEx, "Soloud_setPanAbsoluteEx")
	reg(&soloudSetVolume, "Soloud_setVolume")
	reg(&soloudSetDelaySamples, "Soloud_setDelaySamples")
	reg(&soloudFadeVolume, "Soloud_fadeVolume")
	reg(&soloudFadePan, "Soloud_fadePan")
	reg(&soloudFadeRelativePlaySpeed, "Soloud_fadeRelativePlaySpeed")
	reg(&soloudFadeGlobalVolume, "Soloud_fadeGlobalVolume")
	reg(&soloudSchedulePause, "Soloud_schedulePause")
	reg(&soloudScheduleStop, "Soloud_scheduleStop")
	reg(&soloudOscillateVolume, "Soloud_oscillateVolume")
	reg(&soloudOscillatePan, "Soloud_oscillatePan")
	reg(&soloudOscillateRelativePlaySpeed, "Soloud_oscillateRelativePlaySpeed")
	reg(&soloudOscillateGlobalVolume, "Soloud_oscillateGlobalVolume")
	reg(&soloudSetGlobalFilter, "Soloud_setGlobalFilter")
	reg(&soloudSetVisualizationEnable, "Soloud_setVisualizationEnable")
	reg(&soloudCalcFFT, "Soloud_calcFFT")
	reg(&soloudGetWave, "Soloud_getWave")
	reg(&soloudGetApproximateVolume, "Soloud_getApproximateVolume")
	reg(&soloudGetLoopCount, "Soloud_getLoopCount")
	reg(&soloudGetInfo, "Soloud_getInfo")
	reg(&soloudCreateVoiceGroup, "Soloud_createVoiceGroup")
	reg(&soloudDestroyVoiceGroup, "Soloud_destroyVoiceGroup")
	reg(&soloudAddVoiceToGroup, "Soloud_addVoiceToGroup")
	reg(&soloudIsVoiceGroup, "Soloud_isVoiceGroup")
	reg(&soloudIsVoiceGroupEmpty, "Soloud_isVoiceGroupEmpty")
	reg(&soloudUpdate3dAudio, "Soloud_update3dAudio")
	reg(&soloudSet3dSoundSpeed, "Soloud_set3dSoundSpeed")
	reg(&soloudGet3dSoundSpeed, "Soloud_get3dSoundSpeed")
	reg(&soloudSet3dListenerParameters, "Soloud_set3dListenerParameters")
	reg(&soloudSet3dListenerParametersEx, "Soloud_set3dListenerParametersEx")
	reg(&soloudSet3dListenerPosition, "Soloud_set3dListenerPosition")
	reg(&soloudSet3dListenerAt, "Soloud_set3dListenerAt")
	reg(&soloudSet3dListenerUp, "Soloud_set3dListenerUp")
	reg(&soloudSet3dListenerVelocity, "Soloud_set3dListenerVelocity")
	reg(&soloudSet3dSourceParameters, "Soloud_set3dSourceParameters")
	reg(&soloudSet3dSourceParametersEx, "Soloud_set3dSourceParametersEx")
	reg(&soloudSet3dSourcePosition, "Soloud_set3dSourcePosition")
	reg(&soloudSet3dSourceVelocity, "Soloud_set3dSourceVelocity")
	reg(&soloudSet3dSourceMinMaxDistance, "Soloud_set3dSourceMinMaxDistance")
	reg(&soloudSet3dSourceAttenuation, "Soloud_set3dSourceAttenuation")
	reg(&soloudSet3dSourceDopplerFactor, "Soloud_set3dSourceDopplerFactor")
	reg(&soloudMix, "Soloud_mix")
	reg(&soloudMixSigned16, "Soloud_mixSigned16")
}

// Soloud is the core engine object, mirroring the SoLoud::Soloud class.
type Soloud struct {
	ptr uintptr
}

// NewSoloud creates a new engine instance (Soloud_create).
func NewSoloud() *Soloud {
	return &Soloud{ptr: soloudCreate()}
}

// Destroy frees the native engine object. The Soloud must not be used afterwards.
func (s *Soloud) Destroy() {
	soloudDestroy(s.ptr)
	s.ptr = 0
}

// Init initializes the engine with default parameters.
// It uses CLIP_ROUNDOFF, the automatic backend, default sample rate and buffer size, and two channels.
func (s *Soloud) Init() error {
	return toErr(soloudInit(s.ptr))
}

// InitEx initializes the engine with explicit parameters.
// Use SOLOUD_* constants for aFlags and aBackend, and SOLOUD_AUTO for defaults.
func (s *Soloud) InitEx(aFlags, aBackend, aSamplerate, aBufferSize, aChannels uint32) error {
	return toErr(soloudInitEx(s.ptr, aFlags, aBackend, aSamplerate, aBufferSize, aChannels))
}

func (s *Soloud) Deinit() {
	soloudDeinit(s.ptr)
}
func (s *Soloud) GetVersion() uint32 {
	return soloudGetVersion(s.ptr)
}
func (s *Soloud) GetErrorString(aErrorCode int32) string {
	return soloudGetErrorString(s.ptr, aErrorCode)
}
func (s *Soloud) GetBackendId() uint32 {
	return soloudGetBackendId(s.ptr)
}
func (s *Soloud) GetBackendString() string {
	return soloudGetBackendString(s.ptr)
}
func (s *Soloud) GetBackendChannels() uint32 {
	return soloudGetBackendChannels(s.ptr)
}
func (s *Soloud) GetBackendSamplerate() uint32 {
	return soloudGetBackendSamplerate(s.ptr)
}
func (s *Soloud) GetBackendBufferSize() uint32 {
	return soloudGetBackendBufferSize(s.ptr)
}

func (s *Soloud) SetSpeakerPosition(aChannel uint32, aX, aY, aZ float32) error {
	return toErr(soloudSetSpeakerPosition(s.ptr, aChannel, aX, aY, aZ))
}

func (s *Soloud) GetSpeakerPosition(aChannel uint32) (x, y, z float32, err error) {
	err = toErr(soloudGetSpeakerPosition(s.ptr, aChannel, &x, &y, &z))
	return
}

// Play starts playing a sound and returns its voice handle.
func (s *Soloud) Play(aSound AudioSource) uint32 {
	return soloudPlay(s.ptr, sourcePtr(aSound))
}

func (s *Soloud) PlayEx(aSound AudioSource, aVolume, aPan float32, aPaused bool, aBus uint32) uint32 {
	return soloudPlayEx(s.ptr, sourcePtr(aSound), aVolume, aPan, aPaused, aBus)
}

func (s *Soloud) PlayClocked(aSoundTime float64, aSound AudioSource) uint32 {
	return soloudPlayClocked(s.ptr, aSoundTime, sourcePtr(aSound))
}

func (s *Soloud) PlayClockedEx(aSoundTime float64, aSound AudioSource, aVolume, aPan float32, aBus uint32) uint32 {
	return soloudPlayClockedEx(s.ptr, aSoundTime, sourcePtr(aSound), aVolume, aPan, aBus)
}

func (s *Soloud) Play3d(aSound AudioSource, aPosX, aPosY, aPosZ float32) uint32 {
	return soloudPlay3d(s.ptr, sourcePtr(aSound), aPosX, aPosY, aPosZ)
}

func (s *Soloud) Play3dEx(aSound AudioSource, aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume float32, aPaused bool, aBus uint32) uint32 {
	return soloudPlay3dEx(s.ptr, sourcePtr(aSound), aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume, aPaused, aBus)
}

func (s *Soloud) Play3dClocked(aSoundTime float64, aSound AudioSource, aPosX, aPosY, aPosZ float32) uint32 {
	return soloudPlay3dClocked(s.ptr, aSoundTime, sourcePtr(aSound), aPosX, aPosY, aPosZ)
}

func (s *Soloud) Play3dClockedEx(aSoundTime float64, aSound AudioSource, aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume float32, aBus uint32) uint32 {
	return soloudPlay3dClockedEx(s.ptr, aSoundTime, sourcePtr(aSound), aPosX, aPosY, aPosZ, aVelX, aVelY, aVelZ, aVolume, aBus)
}

func (s *Soloud) PlayBackground(aSound AudioSource) uint32 {
	return soloudPlayBackground(s.ptr, sourcePtr(aSound))
}

func (s *Soloud) PlayBackgroundEx(aSound AudioSource, aVolume float32, aPaused bool, aBus uint32) uint32 {
	return soloudPlayBackgroundEx(s.ptr, sourcePtr(aSound), aVolume, aPaused, aBus)
}

func (s *Soloud) Seek(aVoiceHandle uint32, aSeconds float64) error {
	return toErr(soloudSeek(s.ptr, aVoiceHandle, aSeconds))
}

func (s *Soloud) Stop(aVoiceHandle uint32) {
	soloudStop(s.ptr, aVoiceHandle)
}
func (s *Soloud) StopAll() {
	soloudStopAll(s.ptr)
}
func (s *Soloud) StopAudioSource(aSound AudioSource) {
	soloudStopAudioSource(s.ptr, sourcePtr(aSound))
}
func (s *Soloud) CountAudioSource(aSound AudioSource) int32 {
	return soloudCountAudioSource(s.ptr, sourcePtr(aSound))
}

func (s *Soloud) SetFilterParameter(aVoiceHandle, aFilterId, aAttributeId uint32, aValue float32) {
	soloudSetFilterParameter(s.ptr, aVoiceHandle, aFilterId, aAttributeId, aValue)
}

func (s *Soloud) GetFilterParameter(aVoiceHandle, aFilterId, aAttributeId uint32) float32 {
	return soloudGetFilterParameter(s.ptr, aVoiceHandle, aFilterId, aAttributeId)
}

func (s *Soloud) FadeFilterParameter(aVoiceHandle, aFilterId, aAttributeId uint32, aTo float32, aTime float64) {
	soloudFadeFilterParameter(s.ptr, aVoiceHandle, aFilterId, aAttributeId, aTo, aTime)
}

func (s *Soloud) OscillateFilterParameter(aVoiceHandle, aFilterId, aAttributeId uint32, aFrom, aTo float32, aTime float64) {
	soloudOscillateFilterParameter(s.ptr, aVoiceHandle, aFilterId, aAttributeId, aFrom, aTo, aTime)
}

func (s *Soloud) GetStreamTime(aVoiceHandle uint32) float64 {
	return soloudGetStreamTime(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetStreamPosition(aVoiceHandle uint32) float64 {
	return soloudGetStreamPosition(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetPause(aVoiceHandle uint32) bool {
	return soloudGetPause(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetVolume(aVoiceHandle uint32) float32 {
	return soloudGetVolume(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetOverallVolume(aVoiceHandle uint32) float32 {
	return soloudGetOverallVolume(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetPan(aVoiceHandle uint32) float32 {
	return soloudGetPan(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetSamplerate(aVoiceHandle uint32) float32 {
	return soloudGetSamplerate(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetProtectVoice(aVoiceHandle uint32) bool {
	return soloudGetProtectVoice(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetActiveVoiceCount() uint32 {
	return soloudGetActiveVoiceCount(s.ptr)
}
func (s *Soloud) GetVoiceCount() uint32 {
	return soloudGetVoiceCount(s.ptr)
}
func (s *Soloud) IsValidVoiceHandle(aVoiceHandle uint32) bool {
	return soloudIsValidVoiceHandle(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetRelativePlaySpeed(aVoiceHandle uint32) float32 {
	return soloudGetRelativePlaySpeed(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetPostClipScaler() float32 {
	return soloudGetPostClipScaler(s.ptr)
}
func (s *Soloud) GetGlobalVolume() float32 {
	return soloudGetGlobalVolume(s.ptr)
}
func (s *Soloud) GetMaxActiveVoiceCount() uint32 {
	return soloudGetMaxActiveVoiceCount(s.ptr)
}
func (s *Soloud) GetLooping(aVoiceHandle uint32) bool {
	return soloudGetLooping(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetLoopPoint(aVoiceHandle uint32) float64 {
	return soloudGetLoopPoint(s.ptr, aVoiceHandle)
}
func (s *Soloud) SetLoopPoint(aVoiceHandle uint32, aLoopPoint float64) {
	soloudSetLoopPoint(s.ptr, aVoiceHandle, aLoopPoint)
}
func (s *Soloud) SetLooping(aVoiceHandle uint32, aLooping bool) {
	soloudSetLooping(s.ptr, aVoiceHandle, aLooping)
}
func (s *Soloud) SetMaxActiveVoiceCount(aVoiceCount uint32) error {
	return toErr(soloudSetMaxActiveVoiceCount(s.ptr, aVoiceCount))
}
func (s *Soloud) SetInaudibleBehavior(aVoiceHandle uint32, aMustTick, aKill bool) {
	soloudSetInaudibleBehavior(s.ptr, aVoiceHandle, aMustTick, aKill)
}
func (s *Soloud) SetGlobalVolume(aVolume float32) {
	soloudSetGlobalVolume(s.ptr, aVolume)
}
func (s *Soloud) SetPostClipScaler(aScaler float32) {
	soloudSetPostClipScaler(s.ptr, aScaler)
}
func (s *Soloud) SetPause(aVoiceHandle uint32, aPause bool) {
	soloudSetPause(s.ptr, aVoiceHandle, aPause)
}
func (s *Soloud) SetPauseAll(aPause bool) {
	soloudSetPauseAll(s.ptr, aPause)
}
func (s *Soloud) SetRelativePlaySpeed(aVoiceHandle uint32, aSpeed float32) error {
	return toErr(soloudSetRelativePlaySpeed(s.ptr, aVoiceHandle, aSpeed))
}
func (s *Soloud) SetProtectVoice(aVoiceHandle uint32, aProtect bool) {
	soloudSetProtectVoice(s.ptr, aVoiceHandle, aProtect)
}
func (s *Soloud) SetSamplerate(aVoiceHandle uint32, aSamplerate float32) {
	soloudSetSamplerate(s.ptr, aVoiceHandle, aSamplerate)
}
func (s *Soloud) SetPan(aVoiceHandle uint32, aPan float32) {
	soloudSetPan(s.ptr, aVoiceHandle, aPan)
}
func (s *Soloud) SetPanAbsolute(aVoiceHandle uint32, aLVolume, aRVolume float32) {
	soloudSetPanAbsolute(s.ptr, aVoiceHandle, aLVolume, aRVolume)
}
func (s *Soloud) SetPanAbsoluteEx(aVoiceHandle uint32, aLVolume, aRVolume, aLBVolume, aRBVolume, aCVolume, aSVolume float32) {
	soloudSetPanAbsoluteEx(s.ptr, aVoiceHandle, aLVolume, aRVolume, aLBVolume, aRBVolume, aCVolume, aSVolume)
}
func (s *Soloud) SetVolume(aVoiceHandle uint32, aVolume float32) {
	soloudSetVolume(s.ptr, aVoiceHandle, aVolume)
}
func (s *Soloud) SetDelaySamples(aVoiceHandle, aSamples uint32) {
	soloudSetDelaySamples(s.ptr, aVoiceHandle, aSamples)
}
func (s *Soloud) FadeVolume(aVoiceHandle uint32, aTo float32, aTime float64) {
	soloudFadeVolume(s.ptr, aVoiceHandle, aTo, aTime)
}
func (s *Soloud) FadePan(aVoiceHandle uint32, aTo float32, aTime float64) {
	soloudFadePan(s.ptr, aVoiceHandle, aTo, aTime)
}
func (s *Soloud) FadeRelativePlaySpeed(aVoiceHandle uint32, aTo float32, aTime float64) {
	soloudFadeRelativePlaySpeed(s.ptr, aVoiceHandle, aTo, aTime)
}
func (s *Soloud) FadeGlobalVolume(aTo float32, aTime float64) {
	soloudFadeGlobalVolume(s.ptr, aTo, aTime)
}
func (s *Soloud) SchedulePause(aVoiceHandle uint32, aTime float64) {
	soloudSchedulePause(s.ptr, aVoiceHandle, aTime)
}
func (s *Soloud) ScheduleStop(aVoiceHandle uint32, aTime float64) {
	soloudScheduleStop(s.ptr, aVoiceHandle, aTime)
}
func (s *Soloud) OscillateVolume(aVoiceHandle uint32, aFrom, aTo float32, aTime float64) {
	soloudOscillateVolume(s.ptr, aVoiceHandle, aFrom, aTo, aTime)
}
func (s *Soloud) OscillatePan(aVoiceHandle uint32, aFrom, aTo float32, aTime float64) {
	soloudOscillatePan(s.ptr, aVoiceHandle, aFrom, aTo, aTime)
}
func (s *Soloud) OscillateRelativePlaySpeed(aVoiceHandle uint32, aFrom, aTo float32, aTime float64) {
	soloudOscillateRelativePlaySpeed(s.ptr, aVoiceHandle, aFrom, aTo, aTime)
}
func (s *Soloud) OscillateGlobalVolume(aFrom, aTo float32, aTime float64) {
	soloudOscillateGlobalVolume(s.ptr, aFrom, aTo, aTime)
}
func (s *Soloud) SetGlobalFilter(aFilterId uint32, aFilter Filter) {
	soloudSetGlobalFilter(s.ptr, aFilterId, filterPtr(aFilter))
}
func (s *Soloud) SetVisualizationEnable(aEnable bool) {
	soloudSetVisualizationEnable(s.ptr, aEnable)
}

// CalcFFT returns a copy of the 256-float FFT data in the visualization buffer.
// Enable visualization before calling it.
func (s *Soloud) CalcFFT() []float32 {
	return copyFloats(soloudCalcFFT(s.ptr))
}

// GetWave returns a copy of the 256-float waveform data in the visualization buffer.
// Enable visualization before calling it.
func (s *Soloud) GetWave() []float32 {
	return copyFloats(soloudGetWave(s.ptr))
}

func (s *Soloud) GetApproximateVolume(aChannel uint32) float32 {
	return soloudGetApproximateVolume(s.ptr, aChannel)
}
func (s *Soloud) GetLoopCount(aVoiceHandle uint32) uint32 {
	return soloudGetLoopCount(s.ptr, aVoiceHandle)
}
func (s *Soloud) GetInfo(aVoiceHandle, aInfoKey uint32) float32 {
	return soloudGetInfo(s.ptr, aVoiceHandle, aInfoKey)
}
func (s *Soloud) CreateVoiceGroup() uint32 {
	return soloudCreateVoiceGroup(s.ptr)
}
func (s *Soloud) DestroyVoiceGroup(aVoiceGroupHandle uint32) error {
	return toErr(soloudDestroyVoiceGroup(s.ptr, aVoiceGroupHandle))
}
func (s *Soloud) AddVoiceToGroup(aVoiceGroupHandle, aVoiceHandle uint32) error {
	return toErr(soloudAddVoiceToGroup(s.ptr, aVoiceGroupHandle, aVoiceHandle))
}
func (s *Soloud) IsVoiceGroup(aVoiceGroupHandle uint32) bool {
	return soloudIsVoiceGroup(s.ptr, aVoiceGroupHandle)
}
func (s *Soloud) IsVoiceGroupEmpty(aVoiceGroupHandle uint32) bool {
	return soloudIsVoiceGroupEmpty(s.ptr, aVoiceGroupHandle)
}
func (s *Soloud) Update3dAudio() {
	soloudUpdate3dAudio(s.ptr)
}
func (s *Soloud) Set3dSoundSpeed(aSpeed float32) error {
	return toErr(soloudSet3dSoundSpeed(s.ptr, aSpeed))
}
func (s *Soloud) Get3dSoundSpeed() float32 {
	return soloudGet3dSoundSpeed(s.ptr)
}
func (s *Soloud) Set3dListenerParameters(aPosX, aPosY, aPosZ, aAtX, aAtY, aAtZ, aUpX, aUpY, aUpZ float32) {
	soloudSet3dListenerParameters(s.ptr, aPosX, aPosY, aPosZ, aAtX, aAtY, aAtZ, aUpX, aUpY, aUpZ)
}
func (s *Soloud) Set3dListenerParametersEx(aPosX, aPosY, aPosZ, aAtX, aAtY, aAtZ, aUpX, aUpY, aUpZ, aVelocityX, aVelocityY, aVelocityZ float32) {
	soloudSet3dListenerParametersEx(s.ptr, aPosX, aPosY, aPosZ, aAtX, aAtY, aAtZ, aUpX, aUpY, aUpZ, aVelocityX, aVelocityY, aVelocityZ)
}
func (s *Soloud) Set3dListenerPosition(aPosX, aPosY, aPosZ float32) {
	soloudSet3dListenerPosition(s.ptr, aPosX, aPosY, aPosZ)
}
func (s *Soloud) Set3dListenerAt(aAtX, aAtY, aAtZ float32) {
	soloudSet3dListenerAt(s.ptr, aAtX, aAtY, aAtZ)
}
func (s *Soloud) Set3dListenerUp(aUpX, aUpY, aUpZ float32) {
	soloudSet3dListenerUp(s.ptr, aUpX, aUpY, aUpZ)
}
func (s *Soloud) Set3dListenerVelocity(aVelocityX, aVelocityY, aVelocityZ float32) {
	soloudSet3dListenerVelocity(s.ptr, aVelocityX, aVelocityY, aVelocityZ)
}
func (s *Soloud) Set3dSourceParameters(aVoiceHandle uint32, aPosX, aPosY, aPosZ float32) {
	soloudSet3dSourceParameters(s.ptr, aVoiceHandle, aPosX, aPosY, aPosZ)
}
func (s *Soloud) Set3dSourceParametersEx(aVoiceHandle uint32, aPosX, aPosY, aPosZ, aVelocityX, aVelocityY, aVelocityZ float32) {
	soloudSet3dSourceParametersEx(s.ptr, aVoiceHandle, aPosX, aPosY, aPosZ, aVelocityX, aVelocityY, aVelocityZ)
}
func (s *Soloud) Set3dSourcePosition(aVoiceHandle uint32, aPosX, aPosY, aPosZ float32) {
	soloudSet3dSourcePosition(s.ptr, aVoiceHandle, aPosX, aPosY, aPosZ)
}
func (s *Soloud) Set3dSourceVelocity(aVoiceHandle uint32, aVelocityX, aVelocityY, aVelocityZ float32) {
	soloudSet3dSourceVelocity(s.ptr, aVoiceHandle, aVelocityX, aVelocityY, aVelocityZ)
}
func (s *Soloud) Set3dSourceMinMaxDistance(aVoiceHandle uint32, aMinDistance, aMaxDistance float32) {
	soloudSet3dSourceMinMaxDistance(s.ptr, aVoiceHandle, aMinDistance, aMaxDistance)
}
func (s *Soloud) Set3dSourceAttenuation(aVoiceHandle, aAttenuationModel uint32, aAttenuationRolloffFactor float32) {
	soloudSet3dSourceAttenuation(s.ptr, aVoiceHandle, aAttenuationModel, aAttenuationRolloffFactor)
}
func (s *Soloud) Set3dSourceDopplerFactor(aVoiceHandle uint32, aDopplerFactor float32) {
	soloudSet3dSourceDopplerFactor(s.ptr, aVoiceHandle, aDopplerFactor)
}

// Mix fills aBuffer with aSamples interleaved stereo float samples.
// It only works with the NULLDRIVER backend.
func (s *Soloud) Mix(aBuffer []float32, aSamples uint32) {
	p := slicePtr(aBuffer)
	soloudMix(s.ptr, p, aSamples)
}

// MixSigned16 fills aBuffer with aSamples interleaved stereo 16-bit samples.
// It only works with the NULLDRIVER backend.
func (s *Soloud) MixSigned16(aBuffer []int16, aSamples uint32) {
	p := slicePtr(aBuffer)
	soloudMixSigned16(s.ptr, p, aSamples)
}

// waveSize is the length of the FFT and wave buffers SoLoud hands back (always 256 floats).
const waveSize = 256

// copyFloats copies waveSize floats from a C pointer into a fresh Go slice.
func copyFloats(p *float32) []float32 {
	if p == nil {
		return nil
	}
	out := make([]float32, waveSize)
	copy(out, unsafe.Slice(p, waveSize))
	return out
}
