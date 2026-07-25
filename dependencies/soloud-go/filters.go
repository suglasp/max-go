// filters.go : defines filter bindings for the SoLoud Go binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

// Every C API filter type exports the same parameter introspection functions with a different name prefix, such as EchoFilter_getParamCount.
// filterFuncs holds those common bindings for one prefix. Only create and setParams differ per filter.
type filterFuncs struct {
	destroy       func(uintptr)
	getParamCount func(uintptr) int32
	getParamName  func(uintptr, uint32) string
	getParamType  func(uintptr, uint32) uint32
	getParamMax   func(uintptr, uint32) float32
	getParamMin   func(uintptr, uint32) float32
}

func registerFilterFuncs(prefix string) *filterFuncs {
	f := &filterFuncs{}
	reg(&f.destroy, prefix+"_destroy")
	reg(&f.getParamCount, prefix+"_getParamCount")
	reg(&f.getParamName, prefix+"_getParamName")
	reg(&f.getParamType, prefix+"_getParamType")
	reg(&f.getParamMax, prefix+"_getParamMax")
	reg(&f.getParamMin, prefix+"_getParamMin")
	return f
}

// filterBase is the common base of all filter wrapper types.
type filterBase struct {
	ptr uintptr
	fn  *filterFuncs
}

func (f *filterBase) filterHandle() uintptr {
	return f.ptr
}

// Destroy frees the native filter object. It must not be used afterwards.
func (f *filterBase) Destroy() {
	f.fn.destroy(f.ptr)
	f.ptr = 0
}

func (f *filterBase) GetParamCount() int32 {
	return f.fn.getParamCount(f.ptr)
}
func (f *filterBase) GetParamName(aParamIndex uint32) string {
	return f.fn.getParamName(f.ptr, aParamIndex)
}
func (f *filterBase) GetParamType(aParamIndex uint32) uint32 {
	return f.fn.getParamType(f.ptr, aParamIndex)
}
func (f *filterBase) GetParamMax(aParamIndex uint32) float32 {
	return f.fn.getParamMax(f.ptr, aParamIndex)
}
func (f *filterBase) GetParamMin(aParamIndex uint32) float32 {
	return f.fn.getParamMin(f.ptr, aParamIndex)
}

var (
	bassboostFilterFuncs      *filterFuncs
	biquadResonantFilterFuncs *filterFuncs
	dcRemovalFilterFuncs      *filterFuncs
	echoFilterFuncs           *filterFuncs
	fftFilterFuncs            *filterFuncs
	flangerFilterFuncs        *filterFuncs
	freeverbFilterFuncs       *filterFuncs
	lofiFilterFuncs           *filterFuncs
	robotizeFilterFuncs       *filterFuncs
	waveShaperFilterFuncs     *filterFuncs

	bassboostFilterCreate         func() uintptr
	bassboostFilterSetParams      func(uintptr, float32) int32
	biquadResonantFilterCreate    func() uintptr
	biquadResonantFilterSetParams func(uintptr, int32, float32, float32) int32
	dcRemovalFilterCreate         func() uintptr
	dcRemovalFilterSetParams      func(uintptr) int32
	dcRemovalFilterSetParamsEx    func(uintptr, float32) int32
	echoFilterCreate              func() uintptr
	echoFilterSetParams           func(uintptr, float32) int32
	echoFilterSetParamsEx         func(uintptr, float32, float32, float32) int32
	fftFilterCreate               func() uintptr
	flangerFilterCreate           func() uintptr
	flangerFilterSetParams        func(uintptr, float32, float32) int32
	freeverbFilterCreate          func() uintptr
	freeverbFilterSetParams       func(uintptr, float32, float32, float32, float32) int32
	lofiFilterCreate              func() uintptr
	lofiFilterSetParams           func(uintptr, float32, float32) int32
	robotizeFilterCreate          func() uintptr
	robotizeFilterSetParams       func(uintptr, float32, int32)
	waveShaperFilterCreate        func() uintptr
	waveShaperFilterSetParams     func(uintptr, float32) int32
)

func registerFilters() {
	bassboostFilterFuncs = registerFilterFuncs("BassboostFilter")
	biquadResonantFilterFuncs = registerFilterFuncs("BiquadResonantFilter")
	dcRemovalFilterFuncs = registerFilterFuncs("DCRemovalFilter")
	echoFilterFuncs = registerFilterFuncs("EchoFilter")
	fftFilterFuncs = registerFilterFuncs("FFTFilter")
	flangerFilterFuncs = registerFilterFuncs("FlangerFilter")
	freeverbFilterFuncs = registerFilterFuncs("FreeverbFilter")
	lofiFilterFuncs = registerFilterFuncs("LofiFilter")
	robotizeFilterFuncs = registerFilterFuncs("RobotizeFilter")
	waveShaperFilterFuncs = registerFilterFuncs("WaveShaperFilter")

	reg(&bassboostFilterCreate, "BassboostFilter_create")
	reg(&bassboostFilterSetParams, "BassboostFilter_setParams")
	reg(&biquadResonantFilterCreate, "BiquadResonantFilter_create")
	reg(&biquadResonantFilterSetParams, "BiquadResonantFilter_setParams")
	reg(&dcRemovalFilterCreate, "DCRemovalFilter_create")
	reg(&dcRemovalFilterSetParams, "DCRemovalFilter_setParams")
	reg(&dcRemovalFilterSetParamsEx, "DCRemovalFilter_setParamsEx")
	reg(&echoFilterCreate, "EchoFilter_create")
	reg(&echoFilterSetParams, "EchoFilter_setParams")
	reg(&echoFilterSetParamsEx, "EchoFilter_setParamsEx")
	reg(&fftFilterCreate, "FFTFilter_create")
	reg(&flangerFilterCreate, "FlangerFilter_create")
	reg(&flangerFilterSetParams, "FlangerFilter_setParams")
	reg(&freeverbFilterCreate, "FreeverbFilter_create")
	reg(&freeverbFilterSetParams, "FreeverbFilter_setParams")
	reg(&lofiFilterCreate, "LofiFilter_create")
	reg(&lofiFilterSetParams, "LofiFilter_setParams")
	reg(&robotizeFilterCreate, "RobotizeFilter_create")
	reg(&robotizeFilterSetParams, "RobotizeFilter_setParams")
	reg(&waveShaperFilterCreate, "WaveShaperFilter_create")
	reg(&waveShaperFilterSetParams, "WaveShaperFilter_setParams")
}

// BassboostFilter boosts the low frequencies of a sound.
type BassboostFilter struct{ filterBase }

func NewBassboostFilter() *BassboostFilter {
	return &BassboostFilter{filterBase{ptr: bassboostFilterCreate(), fn: bassboostFilterFuncs}}
}

func (f *BassboostFilter) SetParams(aBoost float32) error {
	return toErr(bassboostFilterSetParams(f.ptr, aBoost))
}

// BiquadResonantFilter is a low/high/bandpass filter.
type BiquadResonantFilter struct{ filterBase }

func NewBiquadResonantFilter() *BiquadResonantFilter {
	return &BiquadResonantFilter{filterBase{ptr: biquadResonantFilterCreate(), fn: biquadResonantFilterFuncs}}
}

// SetParams configures the filter.
// AType is BIQUADRESONANTFILTER_LOWPASS, _HIGHPASS, or _BANDPASS.
func (f *BiquadResonantFilter) SetParams(aType int32, aFrequency, aResonance float32) error {
	return toErr(biquadResonantFilterSetParams(f.ptr, aType, aFrequency, aResonance))
}

// DCRemovalFilter removes DC offset from a sound.
type DCRemovalFilter struct{ filterBase }

func NewDCRemovalFilter() *DCRemovalFilter {
	return &DCRemovalFilter{filterBase{ptr: dcRemovalFilterCreate(), fn: dcRemovalFilterFuncs}}
}

func (f *DCRemovalFilter) SetParams() error {
	return toErr(dcRemovalFilterSetParams(f.ptr))
}
func (f *DCRemovalFilter) SetParamsEx(aLength float32) error {
	return toErr(dcRemovalFilterSetParamsEx(f.ptr, aLength))
}

// EchoFilter adds an echo to a sound.
type EchoFilter struct{ filterBase }

func NewEchoFilter() *EchoFilter {
	return &EchoFilter{filterBase{ptr: echoFilterCreate(), fn: echoFilterFuncs}}
}

func (f *EchoFilter) SetParams(aDelay float32) error {
	return toErr(echoFilterSetParams(f.ptr, aDelay))
}
func (f *EchoFilter) SetParamsEx(aDelay, aDecay, aFilter float32) error {
	return toErr(echoFilterSetParamsEx(f.ptr, aDelay, aDecay, aFilter))
}

// FFTFilter is a base FFT domain filter.
type FFTFilter struct{ filterBase }

func NewFFTFilter() *FFTFilter {
	return &FFTFilter{filterBase{ptr: fftFilterCreate(), fn: fftFilterFuncs}}
}

// FlangerFilter adds a flanger effect to a sound.
type FlangerFilter struct{ filterBase }

func NewFlangerFilter() *FlangerFilter {
	return &FlangerFilter{filterBase{ptr: flangerFilterCreate(), fn: flangerFilterFuncs}}
}

func (f *FlangerFilter) SetParams(aDelay, aFreq float32) error {
	return toErr(flangerFilterSetParams(f.ptr, aDelay, aFreq))
}

// FreeverbFilter is a reverb filter based on Freeverb.
type FreeverbFilter struct{ filterBase }

func NewFreeverbFilter() *FreeverbFilter {
	return &FreeverbFilter{filterBase{ptr: freeverbFilterCreate(), fn: freeverbFilterFuncs}}
}

func (f *FreeverbFilter) SetParams(aMode, aRoomSize, aDamp, aWidth float32) error {
	return toErr(freeverbFilterSetParams(f.ptr, aMode, aRoomSize, aDamp, aWidth))
}

// LofiFilter decreases the fidelity of a sound.
type LofiFilter struct{ filterBase }

func NewLofiFilter() *LofiFilter {
	return &LofiFilter{filterBase{ptr: lofiFilterCreate(), fn: lofiFilterFuncs}}
}

func (f *LofiFilter) SetParams(aSampleRate, aBitdepth float32) error {
	return toErr(lofiFilterSetParams(f.ptr, aSampleRate, aBitdepth))
}

// RobotizeFilter makes a sound robotic.
type RobotizeFilter struct{ filterBase }

func NewRobotizeFilter() *RobotizeFilter {
	return &RobotizeFilter{filterBase{ptr: robotizeFilterCreate(), fn: robotizeFilterFuncs}}
}

func (f *RobotizeFilter) SetParams(aFreq float32, aWaveform int32) {
	robotizeFilterSetParams(f.ptr, aFreq, aWaveform)
}

// WaveShaperFilter distorts a sound.
type WaveShaperFilter struct{ filterBase }

func NewWaveShaperFilter() *WaveShaperFilter {
	return &WaveShaperFilter{filterBase{ptr: waveShaperFilterCreate(), fn: waveShaperFilterFuncs}}
}

func (f *WaveShaperFilter) SetParams(aAmount float32) error {
	return toErr(waveShaperFilterSetParams(f.ptr, aAmount))
}
