package speech

import (
	"testing"
)

func TestModelDisplayNameUsesCatalogLabel(t *testing.T) {
	if got := ModelDisplayName(LocalModel{ID: "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", DisplayName: "dir"}); got != "Qwen3-ASR 0.6B" {
		t.Fatalf("display name = %q", got)
	}
	if got := ModelDisplayName(LocalModel{ID: "custom", DisplayName: "Local custom"}); got != "Local custom" {
		t.Fatalf("custom display name = %q", got)
	}
}

func TestOrderOfflineLocalModelsSkipsStreamingAndFollowsCatalog(t *testing.T) {
	got := orderOfflineLocalModels([]LocalModel{
		{ID: "custom-offline", ModelType: "sense_voice"},
		{ID: "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17", ModelType: "sense_voice"},
		{ID: "zipformer-stream", ModelType: "zipformer2"},
		{ID: "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25", ModelType: "qwen3_asr"},
	})
	if len(got) != 3 || got[0].ID != "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25" || got[1].ID != "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17" || got[2].ID != "custom-offline" {
		t.Fatalf("ordered models = %+v", got)
	}
}
