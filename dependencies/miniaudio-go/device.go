// device.go : defines context and device enumeration for the miniaudio binding.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package miniaudio

// Context enumerates the available audio devices. It mirrors ma_context.
// The device lists are captured when the context is created.
type Context struct {
	h uintptr
}

var (
	mawContextCreate              func() uintptr
	mawContextDestroy             func(uintptr)
	mawContextPlaybackDeviceCount func(uintptr) uint32
	mawContextPlaybackDeviceName  func(uintptr, uint32) string
	mawContextCaptureDeviceCount  func(uintptr) uint32
	mawContextCaptureDeviceName   func(uintptr, uint32) string
)

func registerContext() {
	reg(&mawContextCreate, "maw_context_create")
	reg(&mawContextDestroy, "maw_context_destroy")
	reg(&mawContextPlaybackDeviceCount, "maw_context_playback_device_count")
	reg(&mawContextPlaybackDeviceName, "maw_context_playback_device_name")
	reg(&mawContextCaptureDeviceCount, "maw_context_capture_device_count")
	reg(&mawContextCaptureDeviceName, "maw_context_capture_device_name")
}

// NewContext creates a context and enumerates the system audio devices.
func NewContext() (*Context, error) {
	h := mawContextCreate()
	if h == 0 {
		return nil, errCreate("context")
	}
	return &Context{h: h}, nil
}

// Destroy uninitializes and frees the context.
func (c *Context) Destroy() {
	if c == nil || c.h == 0 {
		return
	}
	mawContextDestroy(c.h)
	c.h = 0
}

// PlaybackDevices returns the names of the available playback devices.
func (c *Context) PlaybackDevices() []string {
	return collectNames(int(mawContextPlaybackDeviceCount(c.h)), func(i uint32) string {
		return mawContextPlaybackDeviceName(c.h, i)
	})
}

// CaptureDevices returns the names of the available capture devices.
func (c *Context) CaptureDevices() []string {
	return collectNames(int(mawContextCaptureDeviceCount(c.h)), func(i uint32) string {
		return mawContextCaptureDeviceName(c.h, i)
	})
}

func collectNames(n int, name func(uint32) string) []string {
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		names = append(names, name(uint32(i)))
	}
	return names
}
