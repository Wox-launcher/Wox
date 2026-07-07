package speech

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/util"
)

// RecognizerPool caches loaded OfflineRecognizer instances keyed by model
// path so the model stays in memory across dictation sessions, eliminating
// the model-loading delay on every startRecording call.
//
// The pool evicts entries idle for longer than idleTTL to reclaim memory.
type RecognizerPool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	idleTTL time.Duration
	cancel  context.CancelFunc
}

type poolEntry struct {
	recognizer Recognizer
	config     RecognizerConfig
	lastUsed   time.Time
	inUse      bool
}

// NewRecognizerPool creates a pool with the given idle eviction timeout.
func NewRecognizerPool(idleTTL time.Duration) *RecognizerPool {
	return &RecognizerPool{
		entries: make(map[string]*poolEntry),
		idleTTL: idleTTL,
	}
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

// Acquire returns a recognizer for the given config. If the model is already
// cached and not in use, it reuses it (fast path). Otherwise it loads the
// model from disk (slow path) and caches it.
func (p *RecognizerPool) Acquire(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	key := poolKey(config)

	p.mu.Lock()
	if entry, ok := p.entries[key]; ok && !entry.inUse {
		entry.inUse = true
		entry.lastUsed = time.Now()
		rec := entry.recognizer
		p.mu.Unlock()
		util.GetLogger().Debug(ctx, "dictation timing: pool.Acquire reused cached model")
		return rec, nil
	}
	p.mu.Unlock()

	// Slow path: load model from disk.
	t0 := time.Now()
	rec, err := newRecognizer(ctx, config)
	if err != nil {
		return nil, err
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("dictation timing: pool.Acquire loaded new model cost=%dms", time.Since(t0).Milliseconds()))

	p.mu.Lock()
	if old, ok := p.entries[key]; ok {
		old.recognizer.Close()
	}
	p.entries[key] = &poolEntry{
		recognizer: rec,
		config:     config,
		lastUsed:   time.Now(),
		inUse:      true,
	}
	p.mu.Unlock()

	return rec, nil
}

// Release returns a recognizer to the pool, keeping the model in memory.
// Streaming recognizers also keep per-stream decoder state, so clear that
// state before reuse while still caching the loaded model.
func (p *RecognizerPool) Release(ctx context.Context, rec Recognizer) {
	if rec == nil {
		return
	}

	if rec.IsStreaming() {
		rec.Reset()
	}
	p.mu.Lock()
	for _, entry := range p.entries {
		if entry.recognizer == rec {
			entry.inUse = false
			entry.lastUsed = time.Now()
			break
		}
	}
	p.mu.Unlock()
}

func (p *RecognizerPool) evictIdle(now time.Time) {
	p.mu.Lock()
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

// EvictExcept closes and removes all cached recognizers whose key does not
// match keepKey. Recognizers currently in use are left alone so they can be
// returned to the pool normally. Use this when the user switches models so
// the old model is immediately evicted from memory instead of waiting for the
// idle timeout.
func (p *RecognizerPool) EvictExcept(keepKey string) {
	p.mu.Lock()
	for key, entry := range p.entries {
		if key == keepKey || entry.inUse {
			continue
		}
		entry.recognizer.Close()
		delete(p.entries, key)
		util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: recognizer pool evicted model %s (model switch)", key))
	}
	p.mu.Unlock()
}

func poolKey(config RecognizerConfig) string {
	return config.ModelPath
}
