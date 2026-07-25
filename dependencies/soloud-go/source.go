// source.go : defines common audio-source bindings shared by all SoLoud source types.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

// Every C API audio-source type exports the same common functions with a different name prefix, such as Wav_setVolume or Speech_setVolume.
// sourceFuncs holds those bindings for one prefix, and audioSource provides the Go methods over them.
type sourceFuncs struct {
	destroy               func(uintptr)
	setVolume             func(uintptr, float32)
	setLooping            func(uintptr, bool)
	set3dMinMaxDistance   func(uintptr, float32, float32)
	set3dAttenuation      func(uintptr, uint32, float32)
	set3dDopplerFactor    func(uintptr, float32)
	set3dListenerRelative func(uintptr, bool)
	set3dDistanceDelay    func(uintptr, bool)
	set3dCollider         func(uintptr, uintptr)
	set3dColliderEx       func(uintptr, uintptr, int32)
	set3dAttenuator       func(uintptr, uintptr)
	setInaudibleBehavior  func(uintptr, bool, bool)
	setLoopPoint          func(uintptr, float64)
	getLoopPoint          func(uintptr) float64
	setFilter             func(uintptr, uint32, uintptr)
	stop                  func(uintptr)
}

// registerSourceFuncs binds the common audio-source functions for one C API name prefix.
// For example, "Wav" binds Wav_destroy and Wav_setVolume.
func registerSourceFuncs(prefix string) *sourceFuncs {
	f := &sourceFuncs{}
	reg(&f.destroy, prefix+"_destroy")
	reg(&f.setVolume, prefix+"_setVolume")
	reg(&f.setLooping, prefix+"_setLooping")
	reg(&f.set3dMinMaxDistance, prefix+"_set3dMinMaxDistance")
	reg(&f.set3dAttenuation, prefix+"_set3dAttenuation")
	reg(&f.set3dDopplerFactor, prefix+"_set3dDopplerFactor")
	reg(&f.set3dListenerRelative, prefix+"_set3dListenerRelative")
	reg(&f.set3dDistanceDelay, prefix+"_set3dDistanceDelay")
	reg(&f.set3dCollider, prefix+"_set3dCollider")
	reg(&f.set3dColliderEx, prefix+"_set3dColliderEx")
	reg(&f.set3dAttenuator, prefix+"_set3dAttenuator")
	reg(&f.setInaudibleBehavior, prefix+"_setInaudibleBehavior")
	reg(&f.setLoopPoint, prefix+"_setLoopPoint")
	reg(&f.getLoopPoint, prefix+"_getLoopPoint")
	reg(&f.setFilter, prefix+"_setFilter")
	reg(&f.stop, prefix+"_stop")
	return f
}

// audioSource is the common base of all audio source wrapper types.
type audioSource struct {
	ptr uintptr
	fn  *sourceFuncs
}

func (a *audioSource) handle() uintptr {
	return a.ptr
}

// Destroy frees the native object. It must not be used afterwards.
func (a *audioSource) Destroy() {
	a.fn.destroy(a.ptr)
	a.ptr = 0
}

func (a *audioSource) SetVolume(aVolume float32) {
	a.fn.setVolume(a.ptr, aVolume)
}
func (a *audioSource) SetLooping(aLoop bool) {
	a.fn.setLooping(a.ptr, aLoop)
}
func (a *audioSource) Set3dMinMaxDistance(aMinDistance, aMaxDistance float32) {
	a.fn.set3dMinMaxDistance(a.ptr, aMinDistance, aMaxDistance)
}
func (a *audioSource) Set3dAttenuation(aAttenuationModel uint32, aAttenuationRolloffFactor float32) {
	a.fn.set3dAttenuation(a.ptr, aAttenuationModel, aAttenuationRolloffFactor)
}
func (a *audioSource) Set3dDopplerFactor(aDopplerFactor float32) {
	a.fn.set3dDopplerFactor(a.ptr, aDopplerFactor)
}
func (a *audioSource) Set3dListenerRelative(aListenerRelative bool) {
	a.fn.set3dListenerRelative(a.ptr, aListenerRelative)
}
func (a *audioSource) Set3dDistanceDelay(aDistanceDelay bool) {
	a.fn.set3dDistanceDelay(a.ptr, aDistanceDelay)
}

// Set3dCollider takes a native AudioCollider pointer.
// The C API cannot create one, so use a pointer obtained elsewhere. Pass 0 to clear it.
func (a *audioSource) Set3dCollider(aCollider uintptr) {
	a.fn.set3dCollider(a.ptr, aCollider)
}
func (a *audioSource) Set3dColliderEx(aCollider uintptr, aUserData int32) {
	a.fn.set3dColliderEx(a.ptr, aCollider, aUserData)
}

// Set3dAttenuator takes a native AudioAttenuator pointer. Pass 0 to clear.
func (a *audioSource) Set3dAttenuator(aAttenuator uintptr) {
	a.fn.set3dAttenuator(a.ptr, aAttenuator)
}

func (a *audioSource) SetInaudibleBehavior(aMustTick, aKill bool) {
	a.fn.setInaudibleBehavior(a.ptr, aMustTick, aKill)
}
func (a *audioSource) SetLoopPoint(aLoopPoint float64) {
	a.fn.setLoopPoint(a.ptr, aLoopPoint)
}
func (a *audioSource) GetLoopPoint() float64 {
	return a.fn.getLoopPoint(a.ptr)
}
func (a *audioSource) SetFilter(aFilterId uint32, aFilter Filter) {
	a.fn.setFilter(a.ptr, aFilterId, filterPtr(aFilter))
}
func (a *audioSource) Stop() {
	a.fn.stop(a.ptr)
}
