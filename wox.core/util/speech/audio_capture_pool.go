package speech

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/util"
)

// AudioCapturePool caches AudioCapture instances keyed by deviceID so the
// malgo context and device stay alive across sessions. This eliminates the
// ~47ms InitDevice cost on subsequent dictations.
//
// The pool evicts entries idle for longer than idleTTL. Only one capture per
// device is cached (typical usage is the system default device).
type AudioCapturePool struct {
	mu      sync.Mutex
	entries map[string]*audioPoolEntry
	idleTTL time.Duration
	cancel  context.CancelFunc
}

type audioPoolEntry struct {
	capture        *AudioCapture
	deviceID       string
	lastUsed       time.Time
	inUse          bool
	evictOnRelease bool
}

// NewAudioCapturePool creates a pool with the given idle eviction timeout.
func NewAudioCapturePool(idleTTL time.Duration) *AudioCapturePool {
	return &AudioCapturePool{
		entries: make(map[string]*audioPoolEntry),
		idleTTL: idleTTL,
	}
}

// StartReaper launches a background goroutine that evicts idle entries every
// minute. Call Close to stop it.
func (p *AudioCapturePool) StartReaper(ctx context.Context) {
	reaperCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	util.Go(reaperCtx, "audio capture pool reaper", func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.evictIdle(time.Now())
			case <-reaperCtx.Done():
				return
			}
		}
	})
}

// Close stops the reaper and releases all cached captures.
func (p *AudioCapturePool) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.evictAll()
}

// Acquire returns an AudioCapture for the given device. If a stopped cached
// capture exists, it reuses it and only swaps the callback. Otherwise it
// creates and caches a new capture device.
func (p *AudioCapturePool) Acquire(ctx context.Context, deviceID string, onSamples func(samples []float32)) (*AudioCapture, error) {
	key := audioPoolKey(deviceID)

	p.mu.Lock()
	if entry, ok := p.entries[key]; ok {
		if entry.inUse {
			p.mu.Unlock()
			return nil, fmt.Errorf("audio capture for the selected device is already in use")
		}
		entry.inUse = true
		entry.lastUsed = time.Now()
		capture := entry.capture
		p.mu.Unlock()

		capture.SetOnSamples(onSamples)
		util.GetLogger().Debug(ctx, "dictation timing: audio pool reused cached capture")
		return capture, nil
	}
	p.mu.Unlock()

	// Slow path: create a capture for a device that is not in the pool yet.
	capture, err := NewAudioCapture(ctx, deviceID, onSamples)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.entries[key] = &audioPoolEntry{
		capture:  capture,
		deviceID: deviceID,
		lastUsed: time.Now(),
		inUse:    true,
	}
	p.mu.Unlock()

	return capture, nil
}

// Release stops a capture before returning it to the pool. Stopping waits for
// CoreAudio callbacks to finish, so the next session can safely replace the
// callback without keeping the microphone active between recordings.
func (p *AudioCapturePool) Release(ctx context.Context, capture *AudioCapture) {
	if capture == nil {
		return
	}
	if err := capture.Stop(); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("dictation: failed to stop cached audio capture: %s", err.Error()))
		p.Discard(ctx, capture)
		return
	}
	// The capture remains cached, so detach the completed Session before it can
	// retain old state or receive a callback after the next device restart.
	capture.SetOnSamples(discardAudioSamples)

	p.mu.Lock()
	for key, entry := range p.entries {
		if entry.capture == capture {
			if entry.evictOnRelease {
				capture.Close()
				delete(p.entries, key)
				util.GetLogger().Info(ctx, fmt.Sprintf("dictation: audio pool evicted capture %s after release", key))
				break
			}
			entry.inUse = false
			entry.lastUsed = time.Now()
			break
		}
	}
	p.mu.Unlock()
}

// Discard closes and removes a capture that failed to start or can no longer
// be reused safely.
func (p *AudioCapturePool) Discard(ctx context.Context, capture *AudioCapture) {
	if capture == nil {
		return
	}

	p.mu.Lock()
	found := false
	for key, entry := range p.entries {
		if entry.capture == capture {
			capture.Close()
			delete(p.entries, key)
			util.GetLogger().Info(ctx, fmt.Sprintf("dictation: audio pool discarded capture %s", key))
			found = true
			break
		}
	}
	p.mu.Unlock()
	if !found {
		capture.Close()
	}
}

func (p *AudioCapturePool) evictIdle(now time.Time) {
	p.mu.Lock()
	if p.idleTTL <= 0 {
		p.mu.Unlock()
		return
	}
	for key, entry := range p.entries {
		if entry.inUse {
			continue
		}
		if now.Sub(entry.lastUsed) > p.idleTTL {
			entry.capture.Close()
			delete(p.entries, key)
			util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: audio pool evicted idle capture %s (idle for %s)", key, now.Sub(entry.lastUsed).Round(time.Second)))
		}
	}
	p.mu.Unlock()
}

// EvictExcept releases cached devices other than keepDeviceID immediately.
// A device still recording is kept until its Session releases it.
func (p *AudioCapturePool) EvictExcept(keepDeviceID string) {
	keepKey := audioPoolKey(keepDeviceID)

	p.mu.Lock()
	for key, entry := range p.entries {
		if key == keepKey {
			entry.evictOnRelease = false
			continue
		}
		if entry.inUse {
			entry.evictOnRelease = true
			continue
		}
		entry.capture.Close()
		delete(p.entries, key)
		util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: audio pool evicted capture %s (input device changed)", key))
	}
	p.mu.Unlock()
}

func (p *AudioCapturePool) evictAll() {
	p.mu.Lock()
	for key, entry := range p.entries {
		entry.capture.Close()
		delete(p.entries, key)
	}
	p.mu.Unlock()
}

func audioPoolKey(deviceID string) string {
	if deviceID == "" {
		return "system"
	}
	return deviceID
}

func discardAudioSamples([]float32) {}
