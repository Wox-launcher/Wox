package launcher

import (
	"context"
	"testing"
	"time"

	previewview "wox/ui/launcher/view/preview"
)

func TestFormatDictationCompareDuration(t *testing.T) {
	if got := formatDictationCompareDuration(0); got != "" {
		t.Fatalf("zero duration = %q", got)
	}
	if got := formatDictationCompareDuration(1200 * time.Millisecond); got != "1.2s" {
		t.Fatalf("duration = %q", got)
	}
}

func TestEnsureDictationModelCompareReusesSession(t *testing.T) {
	app := &App{isDev: true, lifecycleCtx: context.Background()}
	first := app.ensureDictationModelCompare("/tmp/raw.wav")
	ctx, cancel := context.WithCancel(app.lifecycleCtx)
	defer cancel()
	first.cancel = cancel
	first.models = []dictationModelCompareRow{{ID: "kept", Status: previewview.DictationModelCompareDone, Text: "cached"}}
	second := app.ensureDictationModelCompare("/tmp/raw.wav")
	if second != first || len(second.models) != 1 || second.models[0].Text != "cached" {
		t.Fatalf("session should reuse in-progress comparison results: %+v", second)
	}
	if ctx.Err() != nil {
		t.Fatal("reusing the recording should keep its comparison running")
	}
	third := app.ensureDictationModelCompare("/tmp/other-raw.wav")
	if third == first || third.sessionKey == first.sessionKey {
		t.Fatal("a different diagnostic pair should start a new comparison session")
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("switching recordings should cancel the previous comparison")
	}
}
