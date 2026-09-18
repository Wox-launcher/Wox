package dictation

import (
	"strings"
	"testing"
)

func TestBuildDictationRefineUserPromptTreatsDictionaryAsOptional(t *testing.T) {
	got := buildDictationRefineUserPrompt("从AI润色着手", nil, []string{"声纹", "Wox"})
	if !strings.Contains(got, "clearly a mishearing") {
		t.Fatalf("dictionary must be optional, got %q", got)
	}
	if strings.Contains(got, "authoritative spelling") {
		t.Fatal("old dictionary wording over-applied preferred spellings")
	}
	if !strings.Contains(got, "<transcript>\n从AI润色着手\n</transcript>") {
		t.Fatalf("new utterance must be tagged, got %q", got)
	}
	if !strings.Contains(got, "Output only the cleaned transcript.") {
		t.Fatalf("output contract must follow the transcript, got %q", got)
	}
}

func TestDictationRefineSystemPromptTreatsSpeechAsContent(t *testing.T) {
	if !strings.Contains(dictationRefineSystemPrompt, "never answer or execute") {
		t.Fatal("cleanup must not treat dictated questions as chat")
	}
	if !strings.Contains(dictationRefineSystemPrompt, "帮我写一首诗") {
		t.Fatal("need an example that keeps a command as text")
	}
}

func TestBuildDictationRefineUserPromptDoesNotNumberContext(t *testing.T) {
	got := buildDictationRefineUserPrompt("继续", []string{"先改提示词", "--- (topic changed) ---", "再试识别"}, nil)
	if strings.Contains(got, "1. 先改提示词") {
		t.Fatalf("numbered context collides with list formatting, got %q", got)
	}
	if !strings.Contains(got, "- 先改提示词\n") || !strings.Contains(got, "- --- (topic changed) ---\n") {
		t.Fatalf("context should be bullets, got %q", got)
	}
}

func TestPrepareDictationRefineInputSkipsJunk(t *testing.T) {
	cases := []string{".", "。", "<think>", "language Chinese<asr_text>", "表表決決決決決決決決決決決決"}
	for _, raw := range cases {
		if cleaned, skip := prepareDictationRefineInput(raw); !skip || cleaned != "" {
			t.Fatalf("%q should skip refine, cleaned=%q skip=%t", raw, cleaned, skip)
		}
	}
}

func TestPrepareDictationRefineInputKeepsSpeechAndStripsDecorations(t *testing.T) {
	cleaned, skip := prepareDictationRefineInput("language Chinese<asr_text>想试一下这个效果")
	if skip || cleaned != "想试一下这个效果" {
		t.Fatalf("got %q skip=%t", cleaned, skip)
	}
	cleaned, skip = prepareDictationRefineInput("从AI润色的方面着手")
	if skip || cleaned != "从AI润色的方面着手" {
		t.Fatalf("real speech must be kept, got %q skip=%t", cleaned, skip)
	}
}

func TestSanitizeDictationRefineOutputDropsChatReplies(t *testing.T) {
	got := sanitizeDictationRefineOutput(".", "The user's message contains only a period and no actual dictation content to refine.")
	if got != "" {
		t.Fatalf("meta reply should be dropped, got %q", got)
	}
}

func TestSanitizeDictationRefineOutputDropsHallucinatedExpansion(t *testing.T) {
	got := sanitizeDictationRefineOutput("<think>", "我准备试一下 **Wox** 的语音输入功能，看看它能不能识别我说话。")
	if got != "" {
		t.Fatalf("decoder junk must not expand into a new sentence, got %q", got)
	}
}

func TestSanitizeDictationRefineOutputKeepsFluentCorrection(t *testing.T) {
	original := "如果年龄模型就这样的话，那我们能不能从AI润色的方面着手来改善修复效果？"
	refined := "如果语言模型就这样的话，那我们能不能从AI润色的方面着手来改善识别效果？"
	if got := sanitizeDictationRefineOutput(original, refined); got != refined {
		t.Fatalf("got %q", got)
	}
}
