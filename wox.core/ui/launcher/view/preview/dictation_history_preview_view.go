package preview

import (
	"fmt"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// DictationAudioPlayback contains the visible transport state for one track.
type DictationAudioPlayback struct {
	Playing  bool
	Position time.Duration
	Duration time.Duration
}

// DictationHistoryPreviewProps contains the comparison text and optional diagnostic audio.
type DictationHistoryPreviewProps struct {
	ID                    string
	Width                 float32
	Height                float32
	Scale                 float32
	Theme                 woxcomponent.Theme
	RefinedText           string
	OriginalText          string
	RefinedLabel          string
	OriginalLabel         string
	StatusLabel           string
	IsChanged             bool
	RefinedLayout         woxwidget.TextBlockLayout
	OriginalLayout        woxwidget.TextBlockLayout
	StatusWidth           float32
	AudioLabel            string
	RawAudioLabel         string
	RawAudioPath          string
	ProcessedAudioLabel   string
	ProcessedAudioPath    string
	RawPlayback           DictationAudioPlayback
	ProcessedPlayback     DictationAudioPlayback
	PlayLabel             string
	PauseLabel            string
	OnPlayDiagnosticAudio func(string)
}

// DictationHistoryPreviewView builds the Flutter-aligned transcript comparison surface.
func DictationHistoryPreviewView(props DictationHistoryPreviewProps) woxwidget.Widget {
	scale := props.Scale
	if scale <= 0 {
		scale = 1
	}
	scaled := func(value float32) float32 { return float32(int(value*scale + 0.5)) }
	innerWidth := max(float32(0), props.Width-scaled(52))
	muted := props.Theme.PreviewPropertyContent
	if muted.A == 0 {
		muted = props.Theme.PreviewText
	}
	accent := props.Theme.Cursor
	if accent.A == 0 {
		accent = props.Theme.PreviewText
	}

	children := []woxwidget.Widget{
		dictationSectionHeader(props.RefinedLabel, props.StatusLabel, props.IsChanged, dictationTranscriptIcon(strings.TrimSpace(props.OriginalText) != "", scaled(16), accent, muted), innerWidth, props.StatusWidth, scaled, muted),
		woxwidget.Container{Width: innerWidth, Height: scaled(16)},
		dictationRefinedText(props, innerWidth, scaled, accent),
	}

	if strings.TrimSpace(props.OriginalText) != "" {
		children = append(children,
			dictationDivider(innerWidth, scaled, props.Theme.PreviewSplit),
			dictationSectionHeader(props.OriginalLabel, "", false, woxcomponent.WaveformGlyph(scaled(16), dictationColorAlpha(muted, 0.7)), innerWidth, 0, scaled, muted),
			woxwidget.Container{Width: innerWidth, Height: scaled(12)},
			woxwidget.TextBlock{Value: props.OriginalText, Width: innerWidth, Height: props.OriginalLayout.Size.Height, Style: woxui.TextStyle{Size: scaled(14)}, LineHeight: scaled(22), Color: dictationColorAlpha(props.Theme.PreviewText, dictationOriginalTextOpacity(props.IsChanged)), Layout: &props.OriginalLayout},
		)
	}

	// Flutter only exposes diagnostics for a complete raw/processed pair.
	if strings.TrimSpace(props.RawAudioPath) != "" && strings.TrimSpace(props.ProcessedAudioPath) != "" {
		children = append(children,
			dictationDivider(innerWidth, scaled, props.Theme.PreviewSplit),
			dictationSectionHeader(props.AudioLabel, "", false, woxcomponent.MultitrackAudioGlyph(scaled(16), dictationColorAlpha(accent, 0.78)), innerWidth, 0, scaled, muted),
			woxwidget.Container{Width: innerWidth, Height: scaled(14)},
			dictationAudioTrack("raw", props.RawAudioLabel, props.RawAudioPath, props.PlayLabel, props.PauseLabel, props.RawPlayback, innerWidth, scaled, props.Theme, muted, props.OnPlayDiagnosticAudio),
			woxwidget.Container{Width: innerWidth, Height: scaled(12)},
			dictationAudioTrack("processed", props.ProcessedAudioLabel, props.ProcessedAudioPath, props.PlayLabel, props.PauseLabel, props.ProcessedPlayback, innerWidth, scaled, props.Theme, muted, props.OnPlayDiagnosticAudio),
		)
	}

	content := woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: scaled(26), Top: scaled(26), Right: scaled(26), Bottom: scaled(32)},
		Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: woxwidget.Key("dictation-history-preview-" + props.ID), FillWidth: true, FillHeight: true, Content: content,
			ThumbColor: dictationColorAlpha(muted, 0.58),
		}),
	}
}

// dictationSectionHeader keeps the label and optional AI status on one compact row.
func dictationSectionHeader(label, status string, changed bool, icon woxwidget.Widget, width, statusWidth float32, scaled func(float32) float32, muted woxui.Color) woxwidget.Widget {
	children := []woxwidget.StackChild{
		{Top: scaled(1), Child: icon},
		{Left: scaled(24), Right: statusWidth, StretchWidth: true, Child: woxwidget.Container{Height: scaled(18), Child: woxwidget.TextBlock{Value: label, Height: scaled(18), MaxLines: 1, Style: woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold}, LineHeight: scaled(18), Color: dictationColorAlpha(muted, 0.82)}}},
	}
	if strings.TrimSpace(status) != "" {
		statusIcon := woxcomponent.CheckGlyph(scaled(13), dictationColorAlpha(muted, 0.58))
		if changed {
			statusIcon = woxcomponent.SparklesGlyph(scaled(13), dictationColorAlpha(muted, 0.58))
		}
		children = append(children, woxwidget.StackChild{AnchorRight: true, Child: woxwidget.Container{Width: statusWidth, Height: scaled(18), Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaled(4), Children: []woxwidget.Widget{
			woxwidget.Container{Width: scaled(13), Height: scaled(18), Padding: woxwidget.Insets{Top: scaled(2)}, Child: statusIcon},
			woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: scaled(11)}, Color: dictationColorAlpha(muted, 0.62)},
		}}}})
	}
	return woxwidget.Stack{Width: width, Height: scaled(18), Children: children}
}

func dictationTranscriptIcon(hasOriginal bool, size float32, accent, muted woxui.Color) woxwidget.Widget {
	if hasOriginal {
		return woxcomponent.SparklesGlyph(size, accent)
	}
	return woxcomponent.WaveformGlyph(size, dictationColorAlpha(muted, 0.7))
}

func dictationOriginalTextOpacity(changed bool) float32 {
	if changed {
		return 0.7
	}
	return 0.58
}

// dictationRefinedText draws the accent rail alongside the emphasized result.
func dictationRefinedText(props DictationHistoryPreviewProps, width float32, scaled func(float32) float32, accent woxui.Color) woxwidget.Widget {
	textWidth := max(float32(0), width-scaled(16))
	return woxwidget.Stack{Width: width, Height: props.RefinedLayout.Size.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: scaled(2), Height: props.RefinedLayout.Size.Height, Radius: scaled(1), Color: dictationColorAlpha(accent, 0.72)}},
		{Left: scaled(16), Child: woxwidget.TextBlock{Value: props.RefinedText, Width: textWidth, Height: props.RefinedLayout.Size.Height, Style: woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}, LineHeight: scaled(28), Color: dictationColorAlpha(props.Theme.PreviewText, 0.94), Layout: &props.RefinedLayout}},
	}}
}

func dictationDivider(width float32, scaled func(float32) float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: scaled(49), Padding: woxwidget.Insets{Top: scaled(24), Bottom: scaled(24)}, Child: woxwidget.Container{Width: width, Height: 1, Color: dictationColorAlpha(color, 0.32)}}
}

// dictationAudioTrack renders a compact player surface without exposing its local file path.
func dictationAudioTrack(id, label, path, playLabel, pauseLabel string, playback DictationAudioPlayback, width float32, scaled func(float32) float32, theme woxcomponent.Theme, muted woxui.Color, onPlay func(string)) woxwidget.Widget {
	innerWidth := max(float32(0), width-scaled(28))
	trackAccent := theme.Cursor
	if trackAccent.A == 0 {
		trackAccent = theme.PreviewText
	}
	controlColor := dictationColorAlpha(muted, 0.84)
	controlIcon := woxcomponent.PlayCircleGlyph(scaled(24), controlColor)
	if playback.Playing {
		controlIcon = woxcomponent.PauseGlyph(scaled(22), controlColor)
	}
	controlLabel := strings.TrimSpace(playLabel)
	if playback.Playing {
		controlLabel = strings.TrimSpace(pauseLabel)
	}
	if controlLabel == "" {
		controlLabel = label
	}
	playButton := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "dictation-audio-" + id, Label: controlLabel + " · " + label, Icon: controlIcon,
		Width: scaled(36), Height: scaled(36), Radius: scaled(18), Background: dictationColorAlpha(trackAccent, 0.13), HoverBackground: dictationColorAlpha(trackAccent, 0.24),
		FocusRingColor: trackAccent, Disabled: onPlay == nil, OnTap: func() {
			if onPlay != nil {
				onPlay(path)
			}
		},
	})
	progress := float32(0)
	if playback.Duration > 0 {
		progress = min(float32(1), max(float32(0), float32(playback.Position)/float32(playback.Duration)))
	}
	timeWidth := scaled(62)
	progressWidth := max(float32(0), innerWidth-scaled(120))
	timeLabel := ""
	if playback.Duration > 0 {
		timeLabel = formatDictationPlaybackTime(playback.Position) + " / " + formatDictationPlaybackTime(playback.Duration)
	}
	return woxwidget.Container{
		Width: width, Height: scaled(102), Radius: scaled(12), Color: dictationColorAlpha(theme.PreviewText, 0.025), BorderColor: dictationColorAlpha(theme.PreviewSplit, 0.32), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: scaled(14), Top: scaled(12), Right: scaled(14), Bottom: scaled(8)},
		Child: woxwidget.Stack{Width: innerWidth, Height: scaled(82), Children: []woxwidget.StackChild{
			{Child: woxcomponent.PlayCircleGlyph(scaled(15), dictationColorAlpha(muted, 0.62))},
			{Left: scaled(22), Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold}, Color: dictationColorAlpha(muted, 0.78)}},
			{Top: scaled(29), Child: playButton},
			{Left: scaled(50), Top: scaled(43), Child: woxwidget.Container{Width: progressWidth, Height: scaled(5), Radius: scaled(3), Color: dictationColorAlpha(theme.PreviewText, 0.12), Child: woxwidget.Container{Width: progressWidth * progress, Height: scaled(5), Radius: scaled(3), Color: trackAccent}}},
			{Left: innerWidth - timeWidth, Top: scaled(38), Child: woxwidget.Container{Width: timeWidth, Height: scaled(14), Child: woxwidget.Text{Value: timeLabel, Style: woxui.TextStyle{Size: scaled(9)}, Color: dictationColorAlpha(muted, 0.68)}}},
		}},
	}
}

func formatDictationPlaybackTime(value time.Duration) string {
	seconds := max(0, int(value.Round(time.Second)/time.Second))
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func dictationColorAlpha(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(max(float32(0), opacity), float32(1))*255 + 0.5)
	return color
}
