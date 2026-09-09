package launcher

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Layout heights are logical units; native display/font changes invalidate reuse.
type terminalLayoutKey struct {
	session, font            string
	window                   *woxui.Window
	width, scale, lineHeight float32
	style                    woxui.TextStyle
}

// Keep only the current bounded terminal window, not 128 full output revisions.
// Complete paragraphs are stable during append; the final paragraph can rewrap.
// ponytail: a single unbroken paragraph still reflows in full; chunk it only if profiling warrants it.
type terminalLayoutCache struct {
	key         terminalLayoutKey
	value       string
	layout      woxwidget.TextBlockLayout
	prefixEnd   int
	prefixLines int
}

// measure reflows the changed tail, falling back to full layout after history replacement or resize.
func (c *terminalLayoutCache) measure(value string, key terminalLayoutKey) woxwidget.TextBlockLayout {
	if c.layout.Lines != nil && c.key == key && c.value == value {
		return c.layout
	}
	prefixEnd, prefixLines := 0, 0
	if c.key == key && c.prefixEnd <= len(value) && value[:c.prefixEnd] == c.value[:c.prefixEnd] {
		prefixEnd, prefixLines = c.prefixEnd, c.prefixLines
	}
	layout := woxwidget.LayoutTextBlock(key.window, value[prefixEnd:], key.style, key.width, 0, key.lineHeight)
	if prefixLines > 0 {
		// Previously published layouts can still be used by a retained frame.
		lines := make([]string, 0, prefixLines+len(layout.Lines))
		lines = append(lines, c.layout.Lines[:prefixLines]...)
		layout.Lines = append(lines, layout.Lines...)
		layout.Size.Height = float32(len(layout.Lines)) * layout.LineHeight
	}
	end := len(value)
	if end > 0 && value[end-1] == '\r' {
		// A trailing CR may receive its LF in the next transport chunk.
		end--
	}
	nextPrefix := strings.LastIndexAny(value[:end], "\r\n") + 1
	if nextPrefix != prefixEnd {
		tail := woxwidget.LayoutTextBlock(key.window, value[nextPrefix:], key.style, key.width, 0, key.lineHeight)
		prefixLines = len(layout.Lines) - len(tail.Lines)
	}
	*c = terminalLayoutCache{key: key, value: value, layout: layout, prefixEnd: nextPrefix, prefixLines: prefixLines}
	return layout
}
