package launcher

import (
	"context"
	"sync"
	"sync/atomic"

	"wox/common"
	woxui "wox/ui/runtime"
)

// imageResolver is the core capability needed by the per-instance decoded image cache.
type imageResolver interface {
	LoadLazyResultImage(context.Context, string, string) (common.WoxImage, error)
	ResolveImage(context.Context, string, common.WoxImage, int) (common.WoxImage, error)
}

// imageStore owns decoding, request deduplication, and the decoded-image budget.
// Window visibility and row-selection policies remain with their respective owners.
type imageStore struct {
	isDestroyed          func() bool
	services             imageResolver
	sessionID            string
	lifecycleCtx         context.Context
	runOnUI              func(string, func()) error
	invalidateAllWindows func()
	imageMu              sync.RWMutex
	images               map[string]*woxui.Image
	imagesRevision       atomic.Uint64
	imageRequested       map[string]string
	imageVariants        map[string]string
	imageVariantKeys     map[string]string
	imageLastUsed        map[string]uint64
	imageUseSequence     uint64
	// Viewport pins are excluded from the ordinary icon-cache budget.
	imageViewport  map[string]struct{}
	imageCacheSize int
	imageErrors    map[string]string
}

// bindImageStore supplies capabilities without exposing launcher or window state.
func (a *App) bindImageStore() {
	a.imageStore.isDestroyed = a.isDestroyed
	a.imageStore.services = a.services
	a.imageStore.sessionID = a.sessionID
	a.imageStore.lifecycleCtx = a.lifecycleCtx
	a.imageStore.runOnUI = a.runOnUI
	a.imageStore.invalidateAllWindows = a.invalidateAllWindows
}

// decodedBytes reports the cache budget without exposing its synchronization to callers.
func (s *imageStore) decodedBytes() uint64 {
	s.imageMu.RLock()
	defer s.imageMu.RUnlock()
	return uint64(max(0, s.imageCacheSize))
}

// dispose releases request bookkeeping; cached pixels may still be painted while windows close.
func (s *imageStore) dispose() {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	clear(s.imageRequested)
}

// retryErrors allows failed network assets to be requested after connectivity changes.
func (s *imageStore) retryErrors() {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	for key := range s.imageErrors {
		delete(s.imageRequested, key)
	}
	clear(s.imageErrors)
}

// trimIdle applies the hidden budget after the window coordinator checks its consumers.
func (s *imageStore) trimIdle() {
	s.imageMu.Lock()
	defer s.imageMu.Unlock()
	s.evictImagesToBudget("", hiddenImageCacheKeepCount, hiddenImageCacheMaxBytes)
	s.imagesRevision.Add(1)
}
