// sources.go : defines type-specific audio-source bindings for the SoLoud Go binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package soloud

import "runtime"

// This file binds functions unique to each audio-source class.
// source.go binds the common functions, such as setVolume, setLooping, set3d*, setFilter, and stop, by prefix.
//
// File* parameters in the C API are opaque SoLoud::File pointers exposed as uintptr.
var (
	monotoneFuncs  *sourceFuncs
	noiseFuncs     *sourceFuncs
	openmptFuncs   *sourceFuncs
	queueFuncs     *sourceFuncs
	sfxrFuncs      *sourceFuncs
	speechFuncs    *sourceFuncs
	tedSidFuncs    *sourceFuncs
	vicFuncs       *sourceFuncs
	vizsnFuncs     *sourceFuncs
	wavFuncs       *sourceFuncs
	wavStreamFuncs *sourceFuncs

	monotoneCreate      func() uintptr
	monotoneSetParams   func(uintptr, int32) int32
	monotoneSetParamsEx func(uintptr, int32, int32) int32
	monotoneLoad        func(uintptr, string) int32
	monotoneLoadMemEx   func(uintptr, *byte, uint32, bool, bool) int32
	monotoneLoadFile    func(uintptr, uintptr) int32

	noiseCreate         func() uintptr
	noiseSetOctaveScale func(uintptr, float32, float32, float32, float32, float32, float32, float32, float32, float32, float32)
	noiseSetType        func(uintptr, int32)

	openmptCreate    func() uintptr
	openmptLoad      func(uintptr, string) int32
	openmptLoadMemEx func(uintptr, *byte, uint32, bool, bool) int32
	openmptLoadFile  func(uintptr, uintptr) int32

	queueCreate                   func() uintptr
	queuePlay                     func(uintptr, uintptr) int32
	queueGetQueueCount            func(uintptr) uint32
	queueIsCurrentlyPlaying       func(uintptr, uintptr) bool
	queueSetParamsFromAudioSource func(uintptr, uintptr) int32
	queueSetParams                func(uintptr, float32) int32
	queueSetParamsEx              func(uintptr, float32, uint32) int32

	sfxrCreate          func() uintptr
	sfxrResetParams     func(uintptr)
	sfxrLoadParams      func(uintptr, string) int32
	sfxrLoadParamsMemEx func(uintptr, *byte, uint32, bool, bool) int32
	sfxrLoadParamsFile  func(uintptr, uintptr) int32
	sfxrLoadPreset      func(uintptr, int32, int32) int32

	speechCreate      func() uintptr
	speechSetText     func(uintptr, string) int32
	speechSetParams   func(uintptr) int32
	speechSetParamsEx func(uintptr, uint32, float32, float32, int32) int32

	tedSidCreate        func() uintptr
	tedSidLoad          func(uintptr, string) int32
	tedSidLoadToMem     func(uintptr, string) int32
	tedSidLoadMemEx     func(uintptr, *byte, uint32, bool, bool) int32
	tedSidLoadFileToMem func(uintptr, uintptr) int32
	tedSidLoadFile      func(uintptr, uintptr) int32

	vicCreate      func() uintptr
	vicSetModel    func(uintptr, int32)
	vicGetModel    func(uintptr) int32
	vicSetRegister func(uintptr, int32, uint8)
	vicGetRegister func(uintptr, int32) uint8

	vizsnCreate  func() uintptr
	vizsnSetText func(uintptr, string)

	wavCreate          func() uintptr
	wavLoad            func(uintptr, string) int32
	wavLoadMemEx       func(uintptr, *byte, uint32, bool, bool) int32
	wavLoadFile        func(uintptr, uintptr) int32
	wavLoadRawWave8    func(uintptr, *byte, uint32) int32
	wavLoadRawWave8Ex  func(uintptr, *byte, uint32, float32, uint32) int32
	wavLoadRawWave16   func(uintptr, *int16, uint32) int32
	wavLoadRawWave16Ex func(uintptr, *int16, uint32, float32, uint32) int32
	wavLoadRawWaveEx   func(uintptr, *float32, uint32, float32, uint32, bool, bool) int32
	wavGetLength       func(uintptr) float64

	wavStreamCreate        func() uintptr
	wavStreamLoad          func(uintptr, string) int32
	wavStreamLoadMemEx     func(uintptr, *byte, uint32, bool, bool) int32
	wavStreamLoadToMem     func(uintptr, string) int32
	wavStreamLoadFile      func(uintptr, uintptr) int32
	wavStreamLoadFileToMem func(uintptr, uintptr) int32
	wavStreamGetLength     func(uintptr) float64

	/* // note: unused code in Go port
	// bound but never called: every wrapper calls the Ex variant or loads from a path.
	monotoneLoadMem func(uintptr, *byte, uint32) int32
	openmptLoadMem func(uintptr, *byte, uint32) int32
	sfxrLoadParamsMem func(uintptr, *byte, uint32) int32
	tedSidLoadMem func(uintptr, *byte, uint32) int32
	wavLoadMem func(uintptr, *byte, uint32) int32
	wavLoadRawWave func(uintptr, *float32, uint32) int32
	wavStreamLoadMem func(uintptr, *byte, uint32) int32
	*/
)

func registerSources() {
	monotoneFuncs = registerSourceFuncs("Monotone")
	noiseFuncs = registerSourceFuncs("Noise")
	openmptFuncs = registerSourceFuncs("Openmpt")
	queueFuncs = registerSourceFuncs("Queue")
	sfxrFuncs = registerSourceFuncs("Sfxr")
	speechFuncs = registerSourceFuncs("Speech")
	tedSidFuncs = registerSourceFuncs("TedSid")
	vicFuncs = registerSourceFuncs("Vic")
	vizsnFuncs = registerSourceFuncs("Vizsn")
	wavFuncs = registerSourceFuncs("Wav")
	wavStreamFuncs = registerSourceFuncs("WavStream")

	reg(&monotoneCreate, "Monotone_create")
	reg(&monotoneSetParams, "Monotone_setParams")
	reg(&monotoneSetParamsEx, "Monotone_setParamsEx")
	reg(&monotoneLoad, "Monotone_load")
	reg(&monotoneLoadMemEx, "Monotone_loadMemEx")
	reg(&monotoneLoadFile, "Monotone_loadFile")

	reg(&noiseCreate, "Noise_create")
	reg(&noiseSetOctaveScale, "Noise_setOctaveScale")
	reg(&noiseSetType, "Noise_setType")

	reg(&openmptCreate, "Openmpt_create")
	reg(&openmptLoad, "Openmpt_load")
	reg(&openmptLoadMemEx, "Openmpt_loadMemEx")
	reg(&openmptLoadFile, "Openmpt_loadFile")

	reg(&queueCreate, "Queue_create")
	reg(&queuePlay, "Queue_play")
	reg(&queueGetQueueCount, "Queue_getQueueCount")
	reg(&queueIsCurrentlyPlaying, "Queue_isCurrentlyPlaying")
	reg(&queueSetParamsFromAudioSource, "Queue_setParamsFromAudioSource")
	reg(&queueSetParams, "Queue_setParams")
	reg(&queueSetParamsEx, "Queue_setParamsEx")

	reg(&sfxrCreate, "Sfxr_create")
	reg(&sfxrResetParams, "Sfxr_resetParams")
	reg(&sfxrLoadParams, "Sfxr_loadParams")
	reg(&sfxrLoadParamsMemEx, "Sfxr_loadParamsMemEx")
	reg(&sfxrLoadParamsFile, "Sfxr_loadParamsFile")
	reg(&sfxrLoadPreset, "Sfxr_loadPreset")

	reg(&speechCreate, "Speech_create")
	reg(&speechSetText, "Speech_setText")
	reg(&speechSetParams, "Speech_setParams")
	reg(&speechSetParamsEx, "Speech_setParamsEx")

	reg(&tedSidCreate, "TedSid_create")
	reg(&tedSidLoad, "TedSid_load")
	reg(&tedSidLoadToMem, "TedSid_loadToMem")
	reg(&tedSidLoadMemEx, "TedSid_loadMemEx")
	reg(&tedSidLoadFileToMem, "TedSid_loadFileToMem")
	reg(&tedSidLoadFile, "TedSid_loadFile")

	reg(&vicCreate, "Vic_create")
	reg(&vicSetModel, "Vic_setModel")
	reg(&vicGetModel, "Vic_getModel")
	reg(&vicSetRegister, "Vic_setRegister")
	reg(&vicGetRegister, "Vic_getRegister")

	reg(&vizsnCreate, "Vizsn_create")
	reg(&vizsnSetText, "Vizsn_setText")

	reg(&wavCreate, "Wav_create")
	reg(&wavLoad, "Wav_load")
	reg(&wavLoadMemEx, "Wav_loadMemEx")
	reg(&wavLoadFile, "Wav_loadFile")
	reg(&wavLoadRawWave8, "Wav_loadRawWave8")
	reg(&wavLoadRawWave8Ex, "Wav_loadRawWave8Ex")
	reg(&wavLoadRawWave16, "Wav_loadRawWave16")
	reg(&wavLoadRawWave16Ex, "Wav_loadRawWave16Ex")
	reg(&wavLoadRawWaveEx, "Wav_loadRawWaveEx")
	reg(&wavGetLength, "Wav_getLength")

	reg(&wavStreamCreate, "WavStream_create")
	reg(&wavStreamLoad, "WavStream_load")
	reg(&wavStreamLoadMemEx, "WavStream_loadMemEx")
	reg(&wavStreamLoadToMem, "WavStream_loadToMem")
	reg(&wavStreamLoadFile, "WavStream_loadFile")
	reg(&wavStreamLoadFileToMem, "WavStream_loadFileToMem")
	reg(&wavStreamGetLength, "WavStream_getLength")

	/* // note: unused code in Go port, the binds of the never called func vars above
	reg(&monotoneLoadMem, "Monotone_loadMem")
	reg(&openmptLoadMem, "Openmpt_loadMem")
	reg(&sfxrLoadParamsMem, "Sfxr_loadParamsMem")
	reg(&tedSidLoadMem, "TedSid_loadMem")
	reg(&wavLoadMem, "Wav_loadMem")
	reg(&wavLoadRawWave, "Wav_loadRawWave")
	reg(&wavStreamLoadMem, "WavStream_loadMem")
	*/
}

/*
 * Monotone
 */

// Monotone plays MONOTONE tracker tunes.
type Monotone struct{ audioSource }

func NewMonotone() *Monotone {
	return &Monotone{audioSource{ptr: monotoneCreate(), fn: monotoneFuncs}}
}

func (m *Monotone) SetParams(aHardwareChannels int32) error {
	return toErr(monotoneSetParams(m.ptr, aHardwareChannels))
}
func (m *Monotone) SetParamsEx(aHardwareChannels, aWaveform int32) error {
	return toErr(monotoneSetParamsEx(m.ptr, aHardwareChannels, aWaveform))
}
func (m *Monotone) Load(aFilename string) error {
	return toErr(monotoneLoad(m.ptr, aFilename))
}

// LoadMem loads from memory. The data is copied, so the slice may be modified or collected afterwards.
func (m *Monotone) LoadMem(aMem []byte) error {
	err := toErr(monotoneLoadMemEx(m.ptr, slicePtr(aMem), uint32(len(aMem)), true, false))
	runtime.KeepAlive(aMem)
	return err
}
func (m *Monotone) LoadFile(aFile uintptr) error {
	return toErr(monotoneLoadFile(m.ptr, aFile))
}

/*
 * Noise
 */

// Noise is a noise generator audio source.
type Noise struct{ audioSource }

func NewNoise() *Noise {
	return &Noise{audioSource{ptr: noiseCreate(), fn: noiseFuncs}}
}

func (n *Noise) SetOctaveScale(aOct0, aOct1, aOct2, aOct3, aOct4, aOct5, aOct6, aOct7, aOct8, aOct9 float32) {
	noiseSetOctaveScale(n.ptr, aOct0, aOct1, aOct2, aOct3, aOct4, aOct5, aOct6, aOct7, aOct8, aOct9)
}

// SetType sets the noise type (NOISE_WHITE, NOISE_PINK, NOISE_BROWNISH, NOISE_BLUEISH).
func (n *Noise) SetType(aType int32) {
	noiseSetType(n.ptr, aType)
}

/*
 * Openmpt
 */

// Openmpt plays module files through libopenmpt, which must be available at runtime for loading to succeed.
type Openmpt struct{ audioSource }

func NewOpenmpt() *Openmpt {
	return &Openmpt{audioSource{ptr: openmptCreate(), fn: openmptFuncs}}
}

func (o *Openmpt) Load(aFilename string) error {
	return toErr(openmptLoad(o.ptr, aFilename))
}

// LoadMem loads from memory. The data is copied.
func (o *Openmpt) LoadMem(aMem []byte) error {
	err := toErr(openmptLoadMemEx(o.ptr, slicePtr(aMem), uint32(len(aMem)), true, false))
	runtime.KeepAlive(aMem)
	return err
}
func (o *Openmpt) LoadFile(aFile uintptr) error {
	return toErr(openmptLoadFile(o.ptr, aFile))
}

/*
 * Queue
 */

// Queue plays a list of audio sources one after another.
type Queue struct{ audioSource }

func NewQueue() *Queue {
	return &Queue{audioSource{ptr: queueCreate(), fn: queueFuncs}}
}

func (q *Queue) Play(aSound AudioSource) error {
	return toErr(queuePlay(q.ptr, sourcePtr(aSound)))
}
func (q *Queue) GetQueueCount() uint32 {
	return queueGetQueueCount(q.ptr)
}
func (q *Queue) IsCurrentlyPlaying(aSound AudioSource) bool {
	return queueIsCurrentlyPlaying(q.ptr, sourcePtr(aSound))
}
func (q *Queue) SetParamsFromAudioSource(aSound AudioSource) error {
	return toErr(queueSetParamsFromAudioSource(q.ptr, sourcePtr(aSound)))
}
func (q *Queue) SetParams(aSamplerate float32) error {
	return toErr(queueSetParams(q.ptr, aSamplerate))
}
func (q *Queue) SetParamsEx(aSamplerate float32, aChannels uint32) error {
	return toErr(queueSetParamsEx(q.ptr, aSamplerate, aChannels))
}

/*
 * Sfxr
 */

// Sfxr is the sfxr sound effect synthesizer.
type Sfxr struct{ audioSource }

func NewSfxr() *Sfxr {
	return &Sfxr{audioSource{ptr: sfxrCreate(), fn: sfxrFuncs}}
}

func (s *Sfxr) ResetParams() {
	sfxrResetParams(s.ptr)
}
func (s *Sfxr) LoadParams(aFilename string) error {
	return toErr(sfxrLoadParams(s.ptr, aFilename))
}

// LoadParamsMem loads sfxr parameters from memory. The data is copied.
func (s *Sfxr) LoadParamsMem(aMem []byte) error {
	err := toErr(sfxrLoadParamsMemEx(s.ptr, slicePtr(aMem), uint32(len(aMem)), true, false))
	runtime.KeepAlive(aMem)
	return err
}
func (s *Sfxr) LoadParamsFile(aFile uintptr) error {
	return toErr(sfxrLoadParamsFile(s.ptr, aFile))
}

// LoadPreset generates sound effect parameters from a preset, such as SFXR_COIN or SFXR_LASER, and a random seed.
func (s *Sfxr) LoadPreset(aPresetNo, aRandSeed int32) error {
	return toErr(sfxrLoadPreset(s.ptr, aPresetNo, aRandSeed))
}

/*
 * Speech
 */

// Speech is a text to speech audio source built on rsynth.
type Speech struct{ audioSource }

func NewSpeech() *Speech {
	return &Speech{audioSource{ptr: speechCreate(), fn: speechFuncs}}
}

func (s *Speech) SetText(aText string) error {
	return toErr(speechSetText(s.ptr, aText))
}
func (s *Speech) SetParams() error {
	return toErr(speechSetParams(s.ptr))
}
func (s *Speech) SetParamsEx(aBaseFrequency uint32, aBaseSpeed, aBaseDeclination float32, aBaseWaveform int32) error {
	return toErr(speechSetParamsEx(s.ptr, aBaseFrequency, aBaseSpeed, aBaseDeclination, aBaseWaveform))
}

/*
 * TedSid
 */

// TedSid plays TED and SID register dump files.
type TedSid struct{ audioSource }

func NewTedSid() *TedSid {
	return &TedSid{audioSource{ptr: tedSidCreate(), fn: tedSidFuncs}}
}

func (t *TedSid) Load(aFilename string) error {
	return toErr(tedSidLoad(t.ptr, aFilename))
}
func (t *TedSid) LoadToMem(aFilename string) error {
	return toErr(tedSidLoadToMem(t.ptr, aFilename))
}

// LoadMem loads from memory. The data is copied.
func (t *TedSid) LoadMem(aMem []byte) error {
	err := toErr(tedSidLoadMemEx(t.ptr, slicePtr(aMem), uint32(len(aMem)), true, false))
	runtime.KeepAlive(aMem)
	return err
}
func (t *TedSid) LoadFileToMem(aFile uintptr) error {
	return toErr(tedSidLoadFileToMem(t.ptr, aFile))
}
func (t *TedSid) LoadFile(aFile uintptr) error {
	return toErr(tedSidLoadFile(t.ptr, aFile))
}

/*
 * Vic
 */

// Vic is a VIC-20 sound chip emulator audio source.
type Vic struct{ audioSource }

func NewVic() *Vic {
	return &Vic{audioSource{ptr: vicCreate(), fn: vicFuncs}}
}

// SetModel selects the chip model (VIC_PAL or VIC_NTSC).
func (v *Vic) SetModel(model int32) {
	vicSetModel(v.ptr, model)
}
func (v *Vic) GetModel() int32 {
	return vicGetModel(v.ptr)
}
func (v *Vic) SetRegister(reg int32, value uint8) {
	vicSetRegister(v.ptr, reg, value)
}
func (v *Vic) GetRegister(reg int32) uint8 {
	return vicGetRegister(v.ptr, reg)
}

/*
 * Vizsn
 */

// Vizsn is another text to speech audio source.
type Vizsn struct{ audioSource }

func NewVizsn() *Vizsn {
	return &Vizsn{audioSource{ptr: vizsnCreate(), fn: vizsnFuncs}}
}

func (v *Vizsn) SetText(aText string) {
	vizsnSetText(v.ptr, aText)
}

/*
 * Wav
 */

// Wav loads a complete sound (wav, ogg, mp3, flac) into memory.
type Wav struct{ audioSource }

func NewWav() *Wav {
	return &Wav{audioSource{ptr: wavCreate(), fn: wavFuncs}}
}

func (w *Wav) Load(aFilename string) error {
	return toErr(wavLoad(w.ptr, aFilename))
}

// LoadMem loads an encoded sound file (wav, ogg, mp3, flac) from memory.
// The data is copied, so the slice may be modified or collected afterwards.
func (w *Wav) LoadMem(aMem []byte) error {
	err := toErr(wavLoadMemEx(w.ptr, slicePtr(aMem), uint32(len(aMem)), true, false))
	runtime.KeepAlive(aMem)
	return err
}
func (w *Wav) LoadFile(aFile uintptr) error {
	return toErr(wavLoadFile(w.ptr, aFile))
}

// LoadRawWave8 loads raw unsigned 8-bit sample data. It defaults to 44.1 kHz mono; use LoadRawWave8Ex for other formats.
// SoLoud copies the data.
func (w *Wav) LoadRawWave8(aMem []byte) error {
	err := toErr(wavLoadRawWave8(w.ptr, slicePtr(aMem), uint32(len(aMem))))
	runtime.KeepAlive(aMem)
	return err
}
func (w *Wav) LoadRawWave8Ex(aMem []byte, aSamplerate float32, aChannels uint32) error {
	err := toErr(wavLoadRawWave8Ex(w.ptr, slicePtr(aMem), uint32(len(aMem)), aSamplerate, aChannels))
	runtime.KeepAlive(aMem)
	return err
}

// LoadRawWave16 loads raw signed 16-bit sample data. The data is copied by SoLoud.
func (w *Wav) LoadRawWave16(aMem []int16) error {
	p := slicePtr(aMem)
	err := toErr(wavLoadRawWave16(w.ptr, p, uint32(len(aMem))))
	runtime.KeepAlive(aMem)
	return err
}
func (w *Wav) LoadRawWave16Ex(aMem []int16, aSamplerate float32, aChannels uint32) error {
	p := slicePtr(aMem)
	err := toErr(wavLoadRawWave16Ex(w.ptr, p, uint32(len(aMem)), aSamplerate, aChannels))
	runtime.KeepAlive(aMem)
	return err
}

// LoadRawWave loads raw float sample data (44.1kHz mono). The data is copied.
func (w *Wav) LoadRawWave(aMem []float32) error {
	return w.LoadRawWaveEx(aMem, 44100.0, 1)
}

// LoadRawWaveEx loads raw float sample data with an explicit sample rate and channel count.
// The data is copied.
func (w *Wav) LoadRawWaveEx(aMem []float32, aSamplerate float32, aChannels uint32) error {
	p := slicePtr(aMem)
	err := toErr(wavLoadRawWaveEx(w.ptr, p, uint32(len(aMem)), aSamplerate, aChannels, true, false))
	runtime.KeepAlive(aMem)
	return err
}

// GetLength returns the length of the sound in seconds.
func (w *Wav) GetLength() float64 {
	return wavGetLength(w.ptr)
}

/*
 * WavStream
 */

// WavStream streams a sound (wav, ogg, mp3, flac) from disk or memory.
type WavStream struct{ audioSource }

func NewWavStream() *WavStream {
	return &WavStream{audioSource{ptr: wavStreamCreate(), fn: wavStreamFuncs}}
}

func (w *WavStream) Load(aFilename string) error {
	return toErr(wavStreamLoad(w.ptr, aFilename))
}

// LoadMem streams an encoded sound file from memory. The data is copied.
func (w *WavStream) LoadMem(aData []byte) error {
	err := toErr(wavStreamLoadMemEx(w.ptr, slicePtr(aData), uint32(len(aData)), true, false))
	runtime.KeepAlive(aData)
	return err
}
func (w *WavStream) LoadToMem(aFilename string) error {
	return toErr(wavStreamLoadToMem(w.ptr, aFilename))
}
func (w *WavStream) LoadFile(aFile uintptr) error {
	return toErr(wavStreamLoadFile(w.ptr, aFile))
}
func (w *WavStream) LoadFileToMem(aFile uintptr) error {
	return toErr(wavStreamLoadFileToMem(w.ptr, aFile))
}

// GetLength returns the length of the sound in seconds.
func (w *WavStream) GetLength() float64 {
	return wavStreamGetLength(w.ptr)
}
