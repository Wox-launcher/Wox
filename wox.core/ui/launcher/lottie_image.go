package launcher

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	woxui "wox/ui/runtime"
	"wox/util/lottie"
)

const (
	lottieFrameInterval = time.Second / 30
	lottieActiveWindow  = 200 * time.Millisecond
	lottieCacheLifetime = 30 * time.Second
	lottieCacheLimit    = 32
)

type lottieImageEntry struct {
	data      string
	size      int
	animation *lottie.Animation
	image     *woxui.Image
	start     time.Time
	lastUsed  time.Time
	failed    bool
}

type lottieImageCache struct {
	ctx        context.Context
	invalidate func()
	mu         sync.Mutex
	entries    map[string]*lottieImageEntry
	wake       chan struct{}
	startOnce  sync.Once
}

// newLottieImageCache binds all animation work to the owning launcher lifecycle.
func newLottieImageCache(ctx context.Context, invalidate func()) *lottieImageCache {
	return &lottieImageCache{ctx: ctx, invalidate: invalidate, entries: map[string]*lottieImageEntry{}, wake: make(chan struct{}, 1)}
}

// frame returns the latest rasterized frame and marks the animation as visible.
func (c *lottieImageCache) frame(key, data string, size int) *woxui.Image {
	now := time.Now()
	c.mu.Lock()
	entry := c.entries[key]
	if entry == nil {
		if len(c.entries) >= lottieCacheLimit {
			c.mu.Unlock()
			return nil
		}
		entry = &lottieImageEntry{data: data, size: size, start: now}
		c.entries[key] = entry
	}
	entry.lastUsed = now
	image := entry.image
	c.mu.Unlock()

	c.startOnce.Do(func() { go c.run() })
	select {
	case c.wake <- struct{}{}:
	default:
	}
	return image
}

// run advances every recently rendered Lottie from one shared ticker.
func (c *lottieImageCache) run() {
	ticker := time.NewTicker(lottieFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			c.close()
			return
		case <-c.wake:
			c.renderVisible()
		case <-ticker.C:
			c.renderVisible()
		}
	}
}

// renderVisible rasterizes active entries and releases entries that have stayed cold.
func (c *lottieImageCache) renderVisible() {
	now := time.Now()
	var active []*lottieImageEntry
	var expired []*lottie.Animation
	c.mu.Lock()
	for key, entry := range c.entries {
		age := now.Sub(entry.lastUsed)
		if age > lottieCacheLifetime {
			delete(c.entries, key)
			if entry.animation != nil {
				expired = append(expired, entry.animation)
			}
			continue
		}
		if age <= lottieActiveWindow && !entry.failed {
			active = append(active, entry)
		}
	}
	c.mu.Unlock()
	for _, animation := range expired {
		animation.Close()
	}

	changed := false
	for _, entry := range active {
		if entry.animation == nil {
			animation, err := lottie.New(entry.data, entry.size, entry.size)
			if err != nil {
				log.Printf("decode lottie result image: %v", err)
				entry.failed = true
				continue
			}
			entry.animation = animation
		}
		duration := entry.animation.Duration()
		progress := 0.0
		if duration > 0 {
			progress = math.Mod(now.Sub(entry.start).Seconds(), duration) / duration
		}
		rgba, err := entry.animation.Render(progress)
		if err != nil {
			log.Printf("render lottie result image: %v", err)
			entry.failed = true
			continue
		}
		image, err := woxui.NewImage(rgba)
		if err != nil {
			log.Printf("store lottie result image: %v", err)
			entry.failed = true
			continue
		}
		c.mu.Lock()
		entry.image = image
		c.mu.Unlock()
		changed = true
	}
	if changed && c.invalidate != nil {
		c.invalidate()
	}
}

// close releases all native ThorVG documents when the launcher is destroyed.
func (c *lottieImageCache) close() {
	c.mu.Lock()
	entries := c.entries
	c.entries = map[string]*lottieImageEntry{}
	c.mu.Unlock()
	for _, entry := range entries {
		if entry.animation != nil {
			entry.animation.Close()
		}
	}
}

func lottieImageCacheKey(source woxImage, size int) string {
	return fmt.Sprintf("%s-lottie-%d", imageKey(source), size)
}
