package speech

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"
	"wox/util/mainthread"

	"github.com/gen2brain/malgo"
)

// AudioDevice describes a capture device available on the system.
type AudioDevice struct {
	// ID is the malgo device ID string used to select this device.
	ID string
	// Name is the human-readable device name.
	Name string
}

// ListCaptureDevices enumerates all available audio capture devices using
// malgo (miniaudio). The caller does not need to hold a reference to the
// context; this function creates and destroys a temporary context.
//
// On macOS this runs on the main thread to satisfy CoreAudio requirements.
func ListCaptureDevices(ctx context.Context) ([]AudioDevice, error) {
	var devices []AudioDevice
	var err error

	mainthread.Call(func() {
		devices, err = listCaptureDevicesOnMainThread(ctx)
	})

	return devices, err
}

func listCaptureDevicesOnMainThread(ctx context.Context) ([]AudioDevice, error) {
	allocator, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo context: %w", err)
	}
	defer func() {
		_ = allocator.Uninit()
		allocator.Free()
	}()

	infos, err := allocator.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate capture devices: %w", err)
	}

	devices := make([]AudioDevice, 0, len(infos))
	for _, info := range infos {
		devices = append(devices, AudioDevice{
			ID:   info.ID.String(),
			Name: info.Name(),
		})
	}
	return devices, nil
}

// AudioCapture manages a single malgo capture device session. It reads
// 16kHz mono S16 audio and converts samples to float32 for the recognizer.
//
// The onSamples callback is stored in an atomic pointer so it can be swapped
// between sessions without recreating the device. This allows an
// AudioCapturePool to keep the malgo context + device alive across sessions
// and only Start/Stop it, saving the ~47ms InitDevice cost.
type AudioCapture struct {
	ctx        context.Context
	allocator  *malgo.AllocatedContext
	device     *malgo.Device
	sampleChan chan []float32
	mu         sync.Mutex
	started    bool

	// onSamples is read from the malgo callback goroutine. Using atomic lets
	// the pool swap it before Start without a lock in the hot path.
	onSamples atomic.Pointer[func(samples []float32)]
}

// NewAudioCapture creates a new audio capture session. The deviceID can be
// "" or "system" to use the system default capture device; otherwise it
// should be a device ID obtained from ListCaptureDevices.
//
// On macOS, malgo (miniaudio) uses CoreAudio which requires all device
// initialization to happen on the Cocoa main thread. This function uses
// mainthread.Call to ensure correct thread affinity.
func NewAudioCapture(ctx context.Context, deviceID string, onSamples func(samples []float32)) (*AudioCapture, error) {
	var result *AudioCapture
	var resultErr error

	mainthread.Call(func() {
		result, resultErr = newAudioCaptureOnMainThread(ctx, deviceID, onSamples)
	})

	return result, resultErr
}

// newAudioCaptureOnMainThread performs the actual malgo initialization.
// It must be called on the main thread (macOS) or any thread (other platforms).
func newAudioCaptureOnMainThread(ctx context.Context, deviceID string, onSamples func(samples []float32)) (*AudioCapture, error) {
	t0 := time.Now()
	logger := util.GetLogger()

	allocator, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo context: %w", err)
	}
	logger.Debug(ctx, fmt.Sprintf("dictation timing: audio.InitContext cost=%dms", time.Since(t0).Milliseconds()))

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = 16000
	deviceConfig.PeriodSizeInMilliseconds = 100

	// Resolve the device ID. Leave DeviceID as nil (zeroed) for system default.
	var captureDeviceID *malgo.DeviceID
	if deviceID != "" && deviceID != "system" {
		infos, listErr := allocator.Devices(malgo.Capture)
		if listErr != nil {
			_ = allocator.Uninit()
			allocator.Free()
			return nil, fmt.Errorf("failed to enumerate capture devices: %w", listErr)
		}
		logger.Debug(ctx, fmt.Sprintf("dictation timing: audio.enumerateDevices cost=%dms", time.Since(t0).Milliseconds()))
		found := false
		for _, info := range infos {
			if info.ID.String() == deviceID {
				id := info.ID
				captureDeviceID = &id
				found = true
				break
			}
		}
		if !found {
			_ = allocator.Uninit()
			allocator.Free()
			return nil, fmt.Errorf("capture device not found: %s", deviceID)
		}
	}
	// DeviceID.Pointer() dereferences the struct, so we must not call it on
	// a nil pointer. For the system default, leave DeviceID as zero value.
	if captureDeviceID != nil {
		deviceConfig.Capture.DeviceID = captureDeviceID.Pointer()
	}

	capture := &AudioCapture{
		ctx:       ctx,
		allocator: allocator,
	}
	capture.onSamples.Store(&onSamples)

	// The malgo callback reads onSamples via the atomic pointer so the pool
	// can swap it between sessions without recreating the device.
	onRecvFrames := func(_, pSample []byte, framecount uint32) {
		samples := samplesInt16ToFloat(pSample)
		if len(samples) > 0 {
			if cb := capture.onSamples.Load(); cb != nil {
				(*cb)(samples)
			}
		}
	}

	callbacks := malgo.DeviceCallbacks{Data: onRecvFrames}
	device, err := malgo.InitDevice(allocator.Context, deviceConfig, callbacks)
	if err != nil {
		_ = allocator.Uninit()
		allocator.Free()
		return nil, fmt.Errorf("failed to init capture device: %w", err)
	}
	capture.device = device
	logger.Debug(ctx, fmt.Sprintf("dictation timing: audio.InitDevice cost=%dms", time.Since(t0).Milliseconds()))
	logger.Debug(ctx, fmt.Sprintf("dictation timing: audio.total cost=%dms", time.Since(t0).Milliseconds()))

	return capture, nil
}

// SetOnSamples swaps the sample callback. Safe to call when the device is
// stopped (before Start). Used by the audio capture pool to rebind the
// callback to a new session without recreating the device.
func (c *AudioCapture) SetOnSamples(onSamples func(samples []float32)) {
	c.onSamples.Store(&onSamples)
}

// Start begins audio capture. Samples are delivered to the callback
// provided at construction time. On macOS this runs on the main thread.
func (c *AudioCapture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	var err error
	mainthread.Call(func() {
		err = c.device.Start()
	})
	if err != nil {
		return fmt.Errorf("failed to start capture device: %w", err)
	}
	c.started = true
	return nil
}

// Stop stops audio capture but does not release resources.
func (c *AudioCapture) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	var err error
	mainthread.Call(func() {
		err = c.device.Stop()
	})
	if err != nil {
		return fmt.Errorf("failed to stop capture device: %w", err)
	}
	c.started = false
	return nil
}

// Close releases all audio resources. Must be called exactly once.
// On macOS this runs on the main thread.
func (c *AudioCapture) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.device != nil {
		mainthread.Call(func() {
			c.device.Uninit()
		})
		c.device = nil
	}
	if c.allocator != nil {
		mainthread.Call(func() {
			_ = c.allocator.Uninit()
			c.allocator.Free()
		})
		c.allocator = nil
	}
}

// samplesInt16ToFloat converts 16-bit PCM bytes to float32 samples in [-1, 1].
func samplesInt16ToFloat(inSamples []byte) []float32 {
	numSamples := len(inSamples) / 2
	outSamples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		s16 := int16(inSamples[2*i]) | int16(inSamples[2*i+1])<<8
		outSamples[i] = float32(s16) / 32768.0
	}
	return outSamples
}
