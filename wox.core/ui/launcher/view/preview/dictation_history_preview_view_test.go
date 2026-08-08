package preview

import (
	"testing"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestDictationHistoryPreviewMatchesFlutterComparisonStructure(t *testing.T) {
	theme := woxcomponent.Theme{
		Cursor: woxui.Color{R: 50, G: 150, B: 250, A: 255}, PreviewText: woxui.Color{R: 230, G: 235, B: 240, A: 255},
		PreviewSplit: woxui.Color{R: 120, G: 130, B: 140, A: 255}, PreviewPropertyContent: woxui.Color{R: 180, G: 185, B: 190, A: 255},
	}
	view := DictationHistoryPreviewView(DictationHistoryPreviewProps{
		ID: "result", Width: 500, Height: 180, Scale: 1, Theme: theme, RefinedText: "Refined", OriginalText: "Original",
		RefinedLabel: "AI refined", OriginalLabel: "Original transcript", StatusLabel: "Changed", IsChanged: true, StatusWidth: 72,
		RefinedLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 400, Height: 28}}, OriginalLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 400, Height: 22}},
	}).(woxwidget.Container)

	if view.Padding != (woxwidget.Insets{Left: 26, Top: 26, Right: 26, Bottom: 32}) {
		t.Fatalf("preview padding = %#v, want Flutter 26/26/26/32", view.Padding)
	}
	scroll := view.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	content := scroll.Content.(woxwidget.Flex)
	if len(content.Children) != 7 {
		t.Fatalf("comparison children = %d, want refined header/text plus divider and original section", len(content.Children))
	}
	refined := content.Children[2].(woxwidget.Stack)
	bar := refined.Children[0].Child.(woxwidget.Container)
	if bar.Width != 2 || bar.Color.A != 184 {
		t.Fatalf("refined accent bar = width %.0f alpha %d, want Flutter 2px at 0.72", bar.Width, bar.Color.A)
	}
	divider := content.Children[3].(woxwidget.Container).Child.(woxwidget.Container)
	if divider.Height != 1 || divider.Color.A != 82 {
		t.Fatalf("divider = height %.0f alpha %d, want Flutter 1px at 0.32", divider.Height, divider.Color.A)
	}
}

func TestDictationHistoryPreviewRequiresCompleteDiagnosticAudioPair(t *testing.T) {
	playedPath := ""
	base := DictationHistoryPreviewProps{
		ID: "audio", Width: 500, Height: 100, Scale: 1, Theme: woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}, RefinedText: "Text",
		RefinedLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 400, Height: 28}}, RawAudioPath: "/tmp/raw.wav", RawAudioLabel: "Raw",
		OnPlayDiagnosticAudio: func(path string) { playedPath = path },
	}
	incomplete := DictationHistoryPreviewView(base).(woxwidget.Container).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex)
	if len(incomplete.Children) != 3 {
		t.Fatalf("incomplete audio children = %d, want diagnostics hidden like Flutter", len(incomplete.Children))
	}

	base.ProcessedAudioPath = "/tmp/processed.wav"
	base.ProcessedAudioLabel = "Processed"
	base.RawPlayback = DictationAudioPlayback{Playing: true, Position: time.Second, Duration: 4 * time.Second}
	complete := DictationHistoryPreviewView(base).(woxwidget.Container).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex)
	if len(complete.Children) != 9 {
		t.Fatalf("complete audio children = %d, want diagnostic header and two tracks", len(complete.Children))
	}
	rawTrack := complete.Children[6].(woxwidget.Container).Child.(woxwidget.Stack)
	playButton := rawTrack.Children[2].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if playButton.ID != "dictation-audio-raw" || playButton.Disabled || playButton.OnTap == nil {
		t.Fatalf("raw audio control = %+v, want an enabled accessible icon button", playButton)
	}
	progressTrack := rawTrack.Children[3].Child.(woxwidget.Container)
	progressFill := progressTrack.Child.(woxwidget.Container)
	if progressFill.Width != progressTrack.Width*0.25 {
		t.Fatalf("progress width = %.2f/%.2f, want real 25%% playback position", progressFill.Width, progressTrack.Width)
	}
	playButton.OnTap()
	if playedPath != "/tmp/raw.wav" {
		t.Fatalf("played path = %q, want raw diagnostic audio", playedPath)
	}
}
