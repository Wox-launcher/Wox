package speech

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/util"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// RecognizerPool caches loaded sherpa OnlineRecognizer instances keyed by
// model path so the model stays in memory across dictation sessions. This
// eliminates the 600ms+ model-loading delay on every startRecording call.
//
// The pool evicts entries that have not been used for idleTTL (default 10
// minutes) so memory is reclaimed when the user stops dictating for a while.
// Only one model is cached at a time in typical usage; switching models in
// settings replaces the cached entry.
type RecognizerPool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
	idleTTL time.Duration
	cancel  context.CancelFunc
}

type poolEntry struct {
	recognizer *sherpa.OnlineRecognizer
	config     RecognizerConfig
	lastUsed   time.Time
	// inUse marks the entry as actively being used by a session so the reaper
	// does not evict it mid-recording.
	inUse bool
}

// NewRecognizerPool creates a pool with the given idle eviction timeout.
// Start the reaper with StartReaper and stop it with Close.
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

// Acquire returns a recognizer wrapper for the given config. If the model is
// already cached and not in use, it reuses the loaded model and only creates
// a fresh stream (fast path). Otherwise it loads the model from disk (slow
// path) and caches it.
//
// The returned recognizer must be returned via Release when the session ends.
func (p *RecognizerPool) Acquire(ctx context.Context, config RecognizerConfig) (*sherpaRecognizer, error) {
	key := poolKey(config)

	p.mu.Lock()
	if entry, ok := p.entries[key]; ok && !entry.inUse {
		entry.inUse = true
		entry.lastUsed = time.Now()
		recognizer := entry.recognizer
		p.mu.Unlock()

		wrapper, err := wrapSherpaRecognizer(ctx, config, recognizer)
		if err != nil {
			// Stream creation failed; release the in-use mark so the entry
			// can be reused or evicted.
			p.mu.Lock()
			entry.inUse = false
			p.mu.Unlock()
			return nil, err
		}
		util.GetLogger().Debug(ctx, "dictation timing: pool.Acquire reused cached model")
		return wrapper, nil
	}
	p.mu.Unlock()

	// Slow path: load model from disk.
	t0 := time.Now()
	recognizer, err := newSherpaModel(ctx, config)
	if err != nil {
		return nil, err
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("dictation timing: pool.Acquire loaded new model cost=%dms", time.Since(t0).Milliseconds()))

	// Replace any existing entry for this key (e.g. model files changed).
	p.mu.Lock()
	if old, ok := p.entries[key]; ok {
		sherpa.DeleteOnlineRecognizer(old.recognizer)
	}
	p.entries[key] = &poolEntry{
		recognizer: recognizer,
		config:     config,
		lastUsed:   time.Now(),
		inUse:      true,
	}
	p.mu.Unlock()

	wrapper, err := wrapSherpaRecognizer(ctx, config, recognizer)
	if err != nil {
		// Stream creation failed; clean up the entry.
		p.mu.Lock()
		delete(p.entries, key)
		p.mu.Unlock()
		sherpa.DeleteOnlineRecognizer(recognizer)
		return nil, err
	}
	return wrapper, nil
}

// Release returns a recognizer to the pool. It closes the per-session stream
// but keeps the loaded model in memory for future Acquire calls.
func (p *RecognizerPool) Release(ctx context.Context, r *sherpaRecognizer) {
	if r == nil {
		return
	}
	r.CloseStream()

	key := poolKey(r.config)
	p.mu.Lock()
	if entry, ok := p.entries[key]; ok {
		entry.inUse = false
		entry.lastUsed = time.Now()
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
			sherpa.DeleteOnlineRecognizer(entry.recognizer)
			delete(p.entries, key)
			util.GetLogger().Info(context.Background(), fmt.Sprintf("dictation: recognizer pool evicted idle model %s (idle for %s)", key, now.Sub(entry.lastUsed).Round(time.Second)))
		}
	}
	p.mu.Unlock()
}

func (p *RecognizerPool) evictAll() {
	p.mu.Lock()
	for key, entry := range p.entries {
		sherpa.DeleteOnlineRecognizer(entry.recognizer)
		delete(p.entries, key)
	}
	p.mu.Unlock()
}

// poolKey generates a cache key from the model config. ModelPath uniquely
// identifies a loaded model on disk.
func poolKey(config RecognizerConfig) string {
	return config.ModelPath
}
