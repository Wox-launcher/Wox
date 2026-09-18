package speech

import (
	"strings"
	"testing"
)

func TestCleanRecognizerTextStripsQwen3Markers(t *testing.T) {
	got := cleanRecognizerText("language Chinese<asr_text>在提示词里面尽量不要写上太多的。")
	if got != "在提示词里面尽量不要写上太多的。" {
		t.Fatalf("got %q", got)
	}

	got = cleanRecognizerText("language English<asr_text>hello world")
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}

	got = cleanRecognizerText("<|audio_start|>hello<|audio_end|>")
	if got != "hello" {
		t.Fatalf("got %q", got)
	}

	got = cleanRecognizerText("¶ationToken")
	if got != "ationToken" {
		t.Fatalf("got %q", got)
	}
}

func TestCleanRecognizerTextKeepsPlainLanguage(t *testing.T) {
	const input = "language is important in this sentence."
	if got := cleanRecognizerText(input); got != input {
		t.Fatalf("got %q", got)
	}
}

// TestCleanRecognizerTextRejectsPunctuationOnly preserves meaningful short dictation.
func TestCleanRecognizerTextRejectsPunctuationOnly(t *testing.T) {
	for _, input := range []string{".", "。", " …！？\n", "<|audio_start|>.<|audio_end|>"} {
		if got := cleanRecognizerText(input); got != "" {
			t.Errorf("punctuation-only input %q produced %q", input, got)
		}
	}
	for _, input := range []string{"好", "OK", "1", "好。", "C++", "👍"} {
		if got := cleanRecognizerText(input); got != input {
			t.Errorf("short input %q changed to %q", input, got)
		}
	}
}

func TestCleanRecognizerTextDropsCollapsedDecoderLoops(t *testing.T) {
	got := cleanRecognizerText("表表" + strings.Repeat("決", 80))
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}

	got = cleanRecognizerText("优化一下，变成列表形式。")
	if got != "优化一下，变成列表形式。" {
		t.Fatalf("got %q", got)
	}

	got = cleanRecognizerText("好好好")
	if got != "好好好" {
		t.Fatalf("got %q", got)
	}
}

func TestFinalizeRecognizerTextPreservesNumbersAndIdentifiers(t *testing.T) {
	for _, input := range []string{"1000000", "订单号111111", "１１１１１１", "getUserName", "iPhone", "CancellationToken", "ationToken"} {
		text, reason := finalizeRecognizerText(input)
		if text != input || reason != "" {
			t.Errorf("input=%q got text=%q reason=%q", input, text, reason)
		}
	}
}

func TestFinalizeRecognizerTextIdempotent(t *testing.T) {
	const input = "language Chinese<asr_text>就差不多，不需要写太长。"
	first, _ := finalizeRecognizerText(input)
	second, reason := finalizeRecognizerText(first)
	if first != "就差不多，不需要写太长。" || second != first || reason != "" {
		t.Fatalf("first=%q second=%q reason=%q", first, second, reason)
	}
}
