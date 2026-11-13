// stream.go : defines push-PCM playback streams for the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

import "unsafe"

// Stream is a push-PCM playback channel with a dedicated audio device fed by a lock-free ring buffer.
// Decode audio yourself, such as movie or music PCM, then push it with Write.
// The audio thread drains the buffer in C and emits silence on underrun. Stream is independent of Engine and has no host-language callback.
type Stream struct {
	h        uintptr
	channels uint32
}

var (
	mawStreamCreate         func(int32, uint32, uint32, uint32) uintptr
	mawStreamDestroy        func(uintptr)
	mawStreamStart          func(uintptr) int32
	mawStreamStop           func(uintptr) int32
	mawStreamWrite          func(uintptr, unsafe.Pointer, uint32) uint32
	mawStreamAvailableWrite func(uintptr) uint32
	mawStreamSetVolume      func(uintptr, float32) int32
)

func registerStream() {
	reg(&mawStreamCreate, "maw_stream_create")
	reg(&mawStreamDestroy, "maw_stream_destroy")
	reg(&mawStreamStart, "maw_stream_start")
	reg(&mawStreamStop, "maw_stream_stop")
	reg(&mawStreamWrite, "maw_stream_write")
	reg(&mawStreamAvailableWrite, "maw_stream_available_write")
	reg(&mawStreamSetVolume, "maw_stream_set_volume")
}

// NewStream creates a push-PCM stream.
// capacityFrames is the ring-buffer size in frames. Larger values tolerate more jitter at the cost of latency. The stream starts stopped. Call Start to begin pulling audio.
func NewStream(format Format, channels, sampleRate, capacityFrames uint32) (*Stream, error) {
	h := mawStreamCreate(int32(format), channels, sampleRate, capacityFrames)
	if h == 0 {
		return nil, errCreate("stream")
	}
	return &Stream{h: h, channels: channels}, nil
}

// Destroy stops and frees the stream and its device.
func (s *Stream) Destroy() {
	if s == nil || s.h == 0 {
		return
	}
	mawStreamDestroy(s.h)
	s.h = 0
}

// Start begins playback (the device starts pulling from the ring buffer).
func (s *Stream) Start() error {
	return toErr(mawStreamStart(s.h))
}

// Stop pauses playback.
func (s *Stream) Stop() error {
	return toErr(mawStreamStop(s.h))
}

// WriteFrames queues interleaved PCM frames from a raw byte buffer in the stream's Format.
// It returns the number of frames queued. Fewer than requested means the ring buffer is full, so retry later.
func (s *Stream) WriteFrames(data []byte, frameCount uint32) uint32 {
	if len(data) == 0 || frameCount == 0 {
		return 0
	}
	return mawStreamWrite(s.h, unsafe.Pointer(&data[0]), frameCount)
}

// WriteFloat32 queues interleaved float32 samples for FormatF32.
// It returns the number of frames queued. len(samples) must be a multiple of the channel count.
func (s *Stream) WriteFloat32(samples []float32) uint32 {
	if len(samples) == 0 || s.channels == 0 {
		return 0
	}
	return mawStreamWrite(s.h, unsafe.Pointer(&samples[0]), uint32(len(samples))/s.channels)
}

// WriteInt16 queues interleaved int16 samples for FormatS16.
// It returns the number of frames queued. len(samples) must be a multiple of the channel count.
func (s *Stream) WriteInt16(samples []int16) uint32 {
	if len(samples) == 0 || s.channels == 0 {
		return 0
	}
	return mawStreamWrite(s.h, unsafe.Pointer(&samples[0]), uint32(len(samples))/s.channels)
}

// AvailableWrite returns how many frames can currently be written without blocking.
func (s *Stream) AvailableWrite() uint32 {
	return mawStreamAvailableWrite(s.h)
}

// SetVolume sets the stream's master volume (1.0 == unity).
func (s *Stream) SetVolume(volume float32) error {
	return toErr(mawStreamSetVolume(s.h, volume))
}
