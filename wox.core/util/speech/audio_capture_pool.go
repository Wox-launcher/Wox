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
	capture  *AudioCapture
	deviceID string
	lastUsed time.Time
	inUse    bool
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

// Acquire returns an AudioCapture for the given device. If a cached capture
// exists and is not in use, it reuses it (fast path, only swaps the callback).
// Otherwise it creates a new one (slow path) and caches it.
func (p *AudioCapturePool) Acquire(ctx context.Context, deviceID string, onSamples func(samples []float32)) (*AudioCapture, error) {
	key := audioPoolKey(deviceID)

	p.mu.Lock()
	if entry, ok := p.entries[key]; ok && !entry.inUse {
		entry.inUse = true
		entry.lastUsed = time.Now()
		capture := entry.capture
		p.mu.Unlock()

		capture.SetOnSamples(onSamples)
		util.GetLogger().Debug(ctx, "dictation timing: audio pool reused cached capture")
		return capture, nil
	}
	p.mu.Unlock()

	// Slow path: create new capture.
	capture, err := NewAudioCapture(ctx, deviceID, onSamples)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	// Replace any existing entry for this key.
	if old, ok := p.entries[key]; ok {
		old.capture.Close()
	}
	p.entries[key] = &audioPoolEntry{
		capture:  capture,
		deviceID: deviceID,
		lastUsed: time.Now(),
		inUse:    true,
	}
	p.mu.Unlock()

	return capture, nil
}

// Release returns a capture to the pool. It stops the device (so the
// microphone is not actively recording) but keeps the malgo context and
// device initialized for fast reuse.
func (p *AudioCapturePool) Release(ctx context.Context, capture *AudioCapture) {
	if capture == nil {
		return
	}
	_ = capture.Stop()

	// Find the entry by pointer to mark it not in use.
	p.mu.Lock()
	for _, entry := range p.entries {
		if entry.capture == capture {
			entry.inUse = false
			entry.lastUsed = time.Now()
			break
		}
	}
	p.mu.Unlock()
}

func (p *AudioCapturePool) evictIdle(now time.Time) {
	p.mu.Lock()
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
