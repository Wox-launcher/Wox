package launcher

import (
	"crypto/sha256"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	chatMarkdownCacheEntries = 1024
	chatMarkdownCacheBytes   = 512 << 10
)

// Heights are logical units; display scale, font and image changes invalidate them.
type chatMarkdownLayoutKey struct {
	width, scale float32
	font         string
	images       uint64
	window       *woxui.Window
}

type chatMarkdownEntry struct {
	hash     [32]byte
	bytes    int
	document woxcomponent.MarkdownDocument
	layout   chatMarkdownLayoutKey
	size     woxui.Size
}

// Keep one version per message, not one per streamed token. Both source bytes and
// entry count are bounded; documents contain no native images or widget trees.
type chatMarkdownCache struct {
	chatID  string
	entries map[string]chatMarkdownEntry
	bytes   int
}

// measure reuses historical parsing and height while rebuilding callbacks from current props.
func (c *chatMarkdownCache) measure(id, value string, layout chatMarkdownLayoutKey, props woxcomponent.MarkdownProps) (woxcomponent.MarkdownProps, woxui.Size) {
	hash := sha256.Sum256([]byte(value))
	entry, found := c.entries[id]
	if found && entry.hash == hash {
		props.Document = entry.document
		if entry.layout == layout {
			return props, entry.size
		}
	} else {
		props.Document = woxcomponent.ParseMarkdown(value)
	}
	size := woxwidget.MeasureStateless(props.Window, woxcomponent.WoxMarkdown(props), props.Width)
	if found {
		c.bytes -= entry.bytes
		delete(c.entries, id)
	}
	// Oversized histories still render correctly; preserve warm entries instead of
	// clearing the entire cache and reparsing every message on every frame.
	// ponytail: beyond this budget, measure uncached messages; add block virtualization
	// if profiles show very large individual replies dominate the remaining work.
	if len(c.entries) < chatMarkdownCacheEntries && c.bytes+len(value) <= chatMarkdownCacheBytes {
		if c.entries == nil {
			c.entries = make(map[string]chatMarkdownEntry)
		}
		c.entries[id] = chatMarkdownEntry{hash: hash, bytes: len(value), document: props.Document, layout: layout, size: size}
		c.bytes += len(value)
	}
	return props, size
}
