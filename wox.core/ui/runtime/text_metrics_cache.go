package woxui

import "sync"

// textMetricsCacheCapacity bounds retained (text, style, family) → metrics entries.
// Launcher result rows reuse the same titles every frame; wrap binary-search prefixes
// also repeat across stable layouts, so a few thousand entries absorb the hot set.
const textMetricsCacheCapacity = 4096

// textMetricsCacheMaxTextBytes skips caching for oversized strings so wrap binary-search
// prefixes of long preview/chat lines cannot pin megabytes in the LRU.
const textMetricsCacheMaxTextBytes = 256

// textMetricsCacheMaxBytes caps total retained key string bytes (text + family).
const textMetricsCacheMaxBytes = 1 << 20

type textMetricsCacheKey struct {
	text   string
	size   float32
	weight FontWeight
	family string
	kind   FontFamily
	italic bool
}

type textMetricsCacheEntry struct {
	key   textMetricsCacheKey
	value TextMetrics
	bytes int
	prev  *textMetricsCacheEntry
	next  *textMetricsCacheEntry
}

// textMetricsCache is a process-wide LRU for MeasureText results.
// Metrics depend only on text, style, and font family (logical pixels), so entries
// are shared across windows. Entry count and key-byte budget both bound memory.
type textMetricsCache struct {
	mu       sync.Mutex
	capacity int
	maxBytes int
	bytes    int
	entries  map[textMetricsCacheKey]*textMetricsCacheEntry
	head     *textMetricsCacheEntry
	tail     *textMetricsCacheEntry
}

var globalTextMetricsCache = newTextMetricsCache(textMetricsCacheCapacity, textMetricsCacheMaxBytes)

func newTextMetricsCache(capacity, maxBytes int) *textMetricsCache {
	if capacity < 1 {
		capacity = 1
	}
	if maxBytes < 1 {
		maxBytes = 1
	}
	return &textMetricsCache{
		capacity: capacity,
		maxBytes: maxBytes,
		entries:  make(map[textMetricsCacheKey]*textMetricsCacheEntry, capacity),
	}
}

// cacheableTextReports whether text is short enough to retain in the metrics LRU.
func cacheableText(text string) bool {
	return len(text) <= textMetricsCacheMaxTextBytes
}

func textMetricsKeyBytes(key textMetricsCacheKey) int {
	return len(key.text) + len(key.family)
}

func (c *textMetricsCache) get(key textMetricsCacheKey) (TextMetrics, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return TextMetrics{}, false
	}
	c.moveToFrontLocked(entry)
	return entry.value, true
}

func (c *textMetricsCache) put(key textMetricsCacheKey, value TextMetrics) {
	if !cacheableText(key.text) {
		return
	}
	keyBytes := textMetricsKeyBytes(key)
	if keyBytes > c.maxBytes {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[key]; ok {
		entry.value = value
		c.moveToFrontLocked(entry)
		return
	}
	entry := &textMetricsCacheEntry{key: key, value: value, bytes: keyBytes}
	c.entries[key] = entry
	c.bytes += keyBytes
	c.pushFrontLocked(entry)
	for len(c.entries) > c.capacity || c.bytes > c.maxBytes {
		c.evictTailLocked()
	}
}

// ReleaseIdleTextMetricsCache drops all cached MeasureText results while every
// window is hidden. The hot set is re-measured lazily on the first frame after
// show, which costs far less than keeping the strings resident while idle.
func ReleaseIdleTextMetricsCache() {
	globalTextMetricsCache.clear()
}

func (c *textMetricsCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[textMetricsCacheKey]*textMetricsCacheEntry)
	c.head = nil
	c.tail = nil
	c.bytes = 0
}

func (c *textMetricsCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *textMetricsCache) byteSize() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

func (c *textMetricsCache) moveToFrontLocked(entry *textMetricsCacheEntry) {
	if c.head == entry {
		return
	}
	c.detachLocked(entry)
	c.pushFrontLocked(entry)
}

func (c *textMetricsCache) pushFrontLocked(entry *textMetricsCacheEntry) {
	entry.prev = nil
	entry.next = c.head
	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry
	if c.tail == nil {
		c.tail = entry
	}
}

func (c *textMetricsCache) detachLocked(entry *textMetricsCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		c.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		c.tail = entry.prev
	}
	entry.prev = nil
	entry.next = nil
}

func (c *textMetricsCache) evictTailLocked() {
	if c.tail == nil {
		return
	}
	entry := c.tail
	c.detachLocked(entry)
	c.bytes -= entry.bytes
	delete(c.entries, entry.key)
}
