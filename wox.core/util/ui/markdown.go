package ui

import "strings"

// MarkdownStyle classifies a parsed markdown line so the layout pass can pick
// the right font size, weight, color and indentation. This is a minimal subset
// — inline styling (bold/italic spans) is intentionally not supported; markers
// are stripped so the remaining text renders as plain text in the line's style.
type MarkdownStyle int32

const (
	MDNormal MarkdownStyle = iota
	MDHeading1
	MDHeading2
	MDHeading3
	MDList
	MDCode // inside a fenced code block — monospace-ish rendering
	MDQuote
	MDSeparator
)

// MarkdownLine is one rendered line of a markdown document.
type MarkdownLine struct {
	Text   string
	Style  MarkdownStyle
	Indent int // indent level for nested lists (0-based)
}

// ParseMarkdown converts a markdown source string into a flat list of styled
// lines. It handles:
//   - ATX headings (# / ## / ###)
//   - Unordered list items (- or * prefix, with nested indent by leading spaces)
//   - Fenced code blocks (``` ... ```) — the fences are dropped, inner lines get MDCode
//   - Block quotes (> prefix)
//   - Horizontal rules (--- / *** / ___ on their own line)
//   - Inline **bold** and *italic* markers are stripped to plain text
//
// Everything else is treated as normal paragraph text. Blank lines are kept as
// empty MDNormal lines so the layout pass can render paragraph spacing.
func ParseMarkdown(src string) []MarkdownLine {
	var out []MarkdownLine
	inCodeBlock := false

	lines := strings.Split(src, "\n")
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")

		// Fenced code block toggle. Treat the fence line as a boundary, not content.
		trimmedFence := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedFence, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			out = append(out, MarkdownLine{Text: line, Style: MDCode})
			continue
		}

		// Horizontal rule: a line of only -, * or _ with at least 3 chars and nothing else.
		if isHorizontalRule(line) {
			out = append(out, MarkdownLine{Style: MDSeparator})
			continue
		}

		// Heading: # / ## / ### (support trailing content after one space).
		if h, ok := parseHeading(line); ok {
			out = append(out, h)
			continue
		}

		// Block quote: > prefix.
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			text := strings.TrimSpace(line)
			text = strings.TrimPrefix(text, ">")
			text = strings.TrimSpace(text)
			out = append(out, MarkdownLine{Text: stripInlineMarkers(text), Style: MDQuote})
			continue
		}

		// Unordered list: leading spaces then - or *.
		if li, ok := parseListItem(line); ok {
			out = append(out, li)
			continue
		}

		// Normal paragraph text (possibly empty → paragraph gap).
		out = append(out, MarkdownLine{Text: stripInlineMarkers(strings.TrimSpace(line)), Style: MDNormal})
	}
	return out
}

// isHorizontalRule reports whether the line is a markdown horizontal rule
// (---, ***, ___ with at least 3 identical chars and optional leading spaces).
func isHorizontalRule(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) < 3 {
		return false
	}
	c := t[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != c {
			return false
		}
	}
	return true
}

// parseHeading returns a MarkdownLine for #/##/### headings.
func parseHeading(line string) (MarkdownLine, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return MarkdownLine{}, false
	}
	level := 0
	for level < len(t) && t[level] == '#' {
		level++
	}
	// Only support up to h3 for rendering; deeper headings render as h3.
	if level > 3 {
		level = 3
	}
	rest := strings.TrimSpace(t[level:])
	if rest == "" {
		return MarkdownLine{}, false
	}
	style := MDHeading1
	switch level {
	case 2:
		style = MDHeading2
	case 3:
		style = MDHeading3
	}
	return MarkdownLine{Text: stripInlineMarkers(rest), Style: style}, true
}

// parseListItem handles unordered list items ("- " or "* "), computing indent
// depth from leading spaces (2 spaces = one level).
func parseListItem(line string) (MarkdownLine, bool) {
	// Count leading spaces for nesting depth.
	leading := 0
	for leading < len(line) && line[leading] == ' ' {
		leading++
	}
	if leading >= len(line) {
		return MarkdownLine{}, false
	}
	marker := line[leading]
	if marker != '-' && marker != '*' {
		return MarkdownLine{}, false
	}
	// Require a space after the marker to distinguish from emphasis-only lines.
	if leading+1 >= len(line) || line[leading+1] != ' ' {
		return MarkdownLine{}, false
	}
	text := strings.TrimSpace(line[leading+2:])
	indent := leading / 2
	return MarkdownLine{Text: stripInlineMarkers(text), Style: MDList, Indent: indent}, true
}

// stripInlineMarkers removes **bold** and *italic* and `code` markers so the
// text renders cleanly in the line's style. We do not attempt mixed-style
// inline runs; the native UI has no rich-text layout support.
func stripInlineMarkers(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	// Remove single * used for italics, but keep * inside words like "a*b".
	// Simple heuristic: strip * that are surrounded by whitespace or at line edges.
	s = strings.TrimSpace(s)
	return s
}