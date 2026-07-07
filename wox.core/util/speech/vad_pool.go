package speech

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/util"
)

// VadPool caches VoiceActivityDetector instances keyed by VAD model path.
// The VAD model (silero_vad.onnx) is small (~629KB) so keeping it in memory
// is cheap, but the initialization still takes a few ms that we can avoid.
type VadPool struct {
	mu      sync.Mutex
	entries map[string]*vadPoolEntry
	idleTTL time.Duration
	cancel  context.CancelFunc
}

type vadPoolEntry struct {
	vad      *VoiceActivityDetector
	config   VadConfig
	lastUsed time.Time
	inUse    bool
}

// NewVadPool creates a VAD pool with the given idle eviction timeout.
func NewVadPool(idleTTL time.Duration) *VadPool {
	return &VadPool{
		entries: make(map[string]*vadPoolEntry),
		idleTTL: idleTTL,
	}
}

// StartReaper launches a background goroutine that evicts idle entries.
func (p *VadPool) StartReaper(ctx context.Context) {
	reaperCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	util.Go(reaperCtx, "vad pool reaper", func() {
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

// Close stops the reaper and releases all cached VADs.
func (p *VadPool) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.evictAll()
}

// Acquire returns a VAD for the given config. Reuses cached when available.
func (p *VadPool) Acquire(ctx context.Context, config VadConfig) (*VoiceActivityDetector, error) {
	key := config.ModelPath

	p.mu.Lock()
	if entry, ok := p.entries[key]; ok && !entry.inUse {
		entry.inUse = true
		entry.lastUsed = time.Now()
		vad := entry.vad
		p.mu.Unlock()
		// Reset VAD state for a fresh session.
		vad.Clear()
		util.GetLogger().Debug(ctx, "dictation timing: vad pool reused cached VAD")
		return vad, nil
	}
	p.mu.Unlock()

	// Slow path: create new VAD.
	vad, err := NewVoiceActivityDetector(ctx, config)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	if old, ok := p.entries[key]; ok {
		old.vad.Close()
	}
	p.entries[key] = &vadPoolEntry{
		vad:      vad,
		config:   config,
		lastUsed: time.Now(),
		inUse:    true,
	}
	p.mu.Unlock()

	return vad, nil
}

// Release returns a VAD to the pool.
func (p *VadPool) Release(ctx context.Context, vad *VoiceActivityDetector) {
	if vad == nil {
		return
	}

	p.mu.Lock()
	for _, entry := range p.entries {
		if entry.vad == vad {
			entry.inUse = false
			entry.lastUsed = time.Now()
			break
		}
	}
	p.mu.Unlock()
}

func (p *VadPool) evictIdle(now time.Time) {
	p.mu.Lock()
	for key, entry := range p.entries {
		if entry.inUse {
			continue
		}
		if now.Sub(entry.lastUsed) > p.idleTTL {
			entry.vad.Close()
			delete(p.entries, key)
			util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: vad pool evicted idle VAD %s (idle for %s)", key, now.Sub(entry.lastUsed).Round(time.Second)))
		}
	}
	p.mu.Unlock()
}

func (p *VadPool) evictAll() {
	p.mu.Lock()
	for key, entry := range p.entries {
		entry.vad.Close()
		delete(p.entries, key)
	}
	p.mu.Unlock()
}
