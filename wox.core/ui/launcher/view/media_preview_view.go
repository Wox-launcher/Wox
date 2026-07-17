package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// MediaPreviewProps contains normalized media metadata and transport actions.
type MediaPreviewProps struct {
	Width      float32
	Height     float32
	Title      string
	Artist     string
	Details    string
	Artwork    *woxui.Image
	Position   int64
	Duration   int64
	Playing    bool
	Theme      woxcomponent.Theme
	OnPrevious func()
	OnPlay     func()
	OnPause    func()
	OnNext     func()
}

// MediaPreviewView builds artwork, metadata, progress, and transport controls.
func MediaPreviewView(props MediaPreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-36)
	innerHeight := max(float32(0), props.Height-28)
	artSize := min(float32(220), max(float32(104), min(innerWidth*0.4, innerHeight-30)))
	var artwork woxwidget.Widget = woxwidget.Container{Width: artSize, Height: artSize, Radius: artSize / 2, Color: woxui.Color{R: 18, G: 18, B: 22, A: 255}, Padding: woxwidget.Insets{Left: artSize*0.4 - 8, Top: artSize*0.4 - 2}, Child: woxwidget.Text{
		Value: "♪", Style: woxui.TextStyle{Size: max(float32(28), artSize*0.22), Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 107, B: 53, A: 255},
	}}
	if props.Artwork != nil {
		artwork = woxwidget.Container{Width: artSize, Height: artSize, Radius: 14, Color: props.Theme.QueryBackground, Child: woxwidget.Image{Source: props.Artwork, Width: artSize, Height: artSize}}
	}
	metadataWidth := max(float32(120), innerWidth-artSize-24)
	progress := float32(0)
	if props.Duration > 0 {
		progress = float32(props.Position) / float32(props.Duration)
	}
	progressWidth := max(float32(80), metadataWidth-12)
	progressBar := woxwidget.Container{Width: progressWidth, Height: 5, Radius: 3, Color: props.Theme.PreviewSplit, Child: woxwidget.Container{Width: progressWidth * progress, Height: 5, Radius: 3, Color: woxui.Color{R: 255, G: 107, B: 53, A: 255}}}
	toggleLabel := "▶"
	toggleAction := props.OnPlay
	toggleControlID := "media-control-play"
	if props.Playing {
		toggleLabel = "Ⅱ"
		toggleAction = props.OnPause
		toggleControlID = "media-control-pause"
	}
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		mediaControl("media-control-previous", "◀", false, props.OnPrevious, props.Theme),
		mediaControl(toggleControlID, toggleLabel, true, toggleAction, props.Theme),
		mediaControl("media-control-next", "▶", false, props.OnNext, props.Theme),
	}}
	info := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 9, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
		woxwidget.Text{Value: props.Artist, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: props.Details, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
		woxwidget.Painter{Width: metadataWidth, Height: 8},
		progressBar,
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Text{Value: formatMediaDuration(props.Position), Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
			woxwidget.Painter{Width: max(float32(0), progressWidth-72), Height: 14},
			woxwidget.Text{Value: formatMediaDuration(props.Duration), Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
		}},
		woxwidget.Painter{Width: metadataWidth, Height: 6},
		controls,
	}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 12, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 24, Children: []woxwidget.Widget{
		artwork,
		woxwidget.Container{Width: metadataWidth, Height: innerHeight, Padding: woxwidget.Insets{Top: max(float32(0), (innerHeight-190)/2)}, Child: info},
	}}}
}

// mediaControl keeps the three transport controls visually consistent.
func mediaControl(id, label string, primary bool, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	background := theme.QueryBackground
	if primary {
		background = woxui.Color{R: 255, G: 107, B: 53, A: 255}
	}
	return woxwidget.Gesture{ID: "media-preview-" + id, OnTap: onTap, Child: woxwidget.Container{
		Width: 46, Height: 38, Radius: 19, Color: background, Padding: woxwidget.Insets{Left: 15, Top: 10}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.PreviewText},
	}}
}

func formatMediaDuration(seconds int64) string {
	seconds = max(int64(0), seconds)
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
