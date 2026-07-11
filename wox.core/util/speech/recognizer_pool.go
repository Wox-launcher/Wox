package speech

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/util"
)

// RecognizerPool caches loaded OfflineRecognizer instances keyed by model path
// and thread count so the model stays in memory across dictation sessions, eliminating
// the model-loading delay on every startRecording call.
//
// The pool evicts entries idle for longer than idleTTL to reclaim memory. Set
// idleTTL to 0 or below to keep cached recognizers until Close or explicit eviction.
type RecognizerPool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	loading map[string]chan struct{}
	idleTTL time.Duration
	cancel  context.CancelFunc
}

type poolEntry struct {
	recognizer     Recognizer
	config         RecognizerConfig
	lastUsed       time.Time
	inUse          bool
	evictOnRelease bool
}

// NewRecognizerPool creates a pool with the given idle eviction timeout.
func NewRecognizerPool(idleTTL time.Duration) *RecognizerPool {
	return &RecognizerPool{
		entries: make(map[string]*poolEntry),
		loading: make(map[string]chan struct{}),
		idleTTL: idleTTL,
	}
}

// SetIdleTTL updates idle eviction policy for future reaper ticks.
func (p *RecognizerPool) SetIdleTTL(idleTTL time.Duration) {
	p.mu.Lock()
	p.idleTTL = idleTTL
	p.mu.Unlock()
}

// StartReaper launches a background goroutine that evicts idle entries every
// minute. Call Close to stop it.
func (p *RecognizerPool) StartReaper(ctx context.Context) {
	reaperCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	util.Go(reaperCtx, "recognizer pool reaper", func() {
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

// Close stops the reaper and releases all cached recognizers.
func (p *RecognizerPool) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.evictAll()
}

// IsCached reports whether the recognizer model for config is already loaded
// so callers can avoid transient model-loading feedback.
func (p *RecognizerPool) IsCached(config RecognizerConfig) bool {
	if p == nil {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[poolKey(config)]
	return ok && !entry.evictOnRelease
}

// Acquire returns a recognizer for the given config. If the model is already
// cached and not in use, it reuses it (fast path). Otherwise it loads the
// model from disk once and caches it.
func (p *RecognizerPool) Acquire(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	key := poolKey(config)

	for {
		p.mu.Lock()
		if entry, ok := p.entries[key]; ok {
			if entry.inUse {
				p.mu.Unlock()
				return nil, fmt.Errorf("recognizer for the selected model is already in use")
			}
			entry.inUse = true
			entry.lastUsed = time.Now()
			rec := entry.recognizer
			p.mu.Unlock()
			util.GetLogger().Debug(ctx, "dictation timing: pool.Acquire reused cached model")
			return rec, nil
		}
		if wait, ok := p.loading[key]; ok {
			p.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		wait := make(chan struct{})
		p.loading[key] = wait
		p.mu.Unlock()

		// Only one goroutine may load a model for this key. Other callers wait
		// for this load to finish and then either reuse the cache or see busy.
		t0 := time.Now()
		rec, err := newRecognizer(ctx, config)

		p.mu.Lock()
		delete(p.loading, key)
		close(wait)
		if err == nil {
			p.entries[key] = &poolEntry{
				recognizer: rec,
				config:     config,
				lastUsed:   time.Now(),
				inUse:      true,
			}
		}
		p.mu.Unlock()

		if err != nil {
			return nil, err
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("dictation timing: pool.Acquire loaded new model cost=%dms", time.Since(t0).Milliseconds()))
		return rec, nil
	}
}

// Release returns a recognizer to the pool, or closes it if a model switch
// marked it for eviction. Streaming recognizers also keep per-stream decoder
// state, so clear that state before reuse while still caching the loaded model.
func (p *RecognizerPool) Release(ctx context.Context, rec Recognizer) {
	if rec == nil {
		return
	}

	p.mu.Lock()
	for key, entry := range p.entries {
		if entry.recognizer == rec {
			if entry.evictOnRelease {
				entry.recognizer.Close()
				delete(p.entries, key)
				util.GetLogger().Info(ctx, fmt.Sprintf("dictation: recognizer pool evicted model %s after release", key))
				break
			}
			if rec.IsStreaming() {
				rec.Reset()
			}
			entry.inUse = false
			entry.lastUsed = time.Now()
			break
		}
	}
	p.mu.Unlock()
}

func (p *RecognizerPool) evictIdle(now time.Time) {
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
			entry.recognizer.Close()
			delete(p.entries, key)
			util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: recognizer pool evicted idle model %s (idle for %s)", key, now.Sub(entry.lastUsed).Round(time.Second)))
		}
	}
	p.mu.Unlock()
}

func (p *RecognizerPool) evictAll() {
	p.mu.Lock()
	for key, entry := range p.entries {
		entry.recognizer.Close()
		delete(p.entries, key)
	}
	p.mu.Unlock()
}

// EvictExcept closes and removes all cached recognizers whose model path does not
// match keepModelPath. Recognizers currently in use are closed when released. Use
// this when the user switches models so the old model is evicted from memory
// instead of waiting for the idle timeout.
func (p *RecognizerPool) EvictExcept(keepModelPath string) {
	p.mu.Lock()
	for key, entry := range p.entries {
		if entry.config.ModelPath == keepModelPath {
			continue
		}
		if entry.inUse {
			entry.evictOnRelease = true
			continue
		}
		entry.recognizer.Close()
		delete(p.entries, key)
		util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: recognizer pool evicted model %s (model switch)", key))
	}
	p.mu.Unlock()
}

func poolKey(config RecognizerConfig) string {
	return fmt.Sprintf("%s|threads=%d", config.ModelPath, config.NumThreads)
}
