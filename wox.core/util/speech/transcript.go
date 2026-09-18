package speech

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	repeatedRuneCollapseAfter = 6
	repeatedRuneKeep          = 2
)

var (
	qwen3LanguageAsrPrefix = regexp.MustCompile(`(?i)language\s+[A-Za-z][A-Za-z0-9_-]*\s*<asr_text>`)
	qwen3AsrTextTag        = regexp.MustCompile(`(?i)<asr_text>`)
	qwen3SpecialToken      = regexp.MustCompile(`<\|[^|>]+?\|>|<blank\d+>|<tts_[a-z_]+>`)
)

// cleanRecognizerText strips Qwen3 control markers and collapses decoder loops.
func cleanRecognizerText(text string) string {
	if text == "" {
		return ""
	}

	text = qwen3SpecialToken.ReplaceAllString(text, "")
	text = qwen3LanguageAsrPrefix.ReplaceAllString(text, "")
	text = qwen3AsrTextTag.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "¶", "")
	// Silence can decode to punctuation alone; it must not become an AI request.
	if strings.IndexFunc(text, func(r rune) bool { return !unicode.IsSpace(r) && !unicode.IsPunct(r) }) < 0 {
		return ""
	}

	collapsed, collapsedRun := collapseRepeatedRunes(text)
	collapsed = strings.TrimSpace(collapsed)
	if collapsedRun && isLowDiversityTranscript(collapsed) {
		return ""
	}
	return collapsed
}

// finalizeRecognizerText cleans model output and drops clips that are clearly
// decoder artifacts rather than speech.
func finalizeRecognizerText(text string) (string, string) {
	cleaned := cleanRecognizerText(text)
	if cleaned == "" {
		if strings.TrimSpace(text) != "" {
			return "", "empty_after_clean"
		}
		return "", ""
	}
	return cleaned, ""
}

func collapseRepeatedRunes(text string) (string, bool) {
	runes := []rune(text)
	if len(runes) == 0 {
		return text, false
	}

	var builder strings.Builder
	builder.Grow(len(text))
	collapsedAny := false
	for i := 0; i < len(runes); {
		j := i + 1
		for j < len(runes) && runes[j] == runes[i] {
			j++
		}
		run := j - i
		keep := run
		// Repeated digits are valid amounts and identifiers, not decoder loops.
		if run >= repeatedRuneCollapseAfter && !unicode.IsDigit(runes[i]) {
			keep = repeatedRuneKeep
			collapsedAny = true
		}
		for k := 0; k < keep; k++ {
			builder.WriteRune(runes[i])
		}
		i = j
	}
	return builder.String(), collapsedAny
}

func isLowDiversityTranscript(text string) bool {
	var content []rune
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		content = append(content, r)
	}
	if len(content) < 4 {
		return false
	}

	seen := map[rune]struct{}{}
	for _, r := range content {
		if unicode.IsLetter(r) && r < 0x80 {
			return false
		}
		if unicode.IsDigit(r) {
			return false
		}
		seen[r] = struct{}{}
	}
	return len(seen) <= 2
}
