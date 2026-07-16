package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type mediaPreviewData struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AppName   string `json:"appName"`
	Artwork   string `json:"artwork"`
	Position  int64  `json:"position"`
	Duration  int64  `json:"duration"`
	IsPlaying bool   `json:"isPlaying"`
}

func decodeMediaPreview(value string) (mediaPreviewData, error) {
	var data mediaPreviewData
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return mediaPreviewData{}, err
	}
	return data, nil
}

func formatMediaDuration(seconds int64) string {
	seconds = max(int64(0), seconds)
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// buildMediaPreview renders metadata, artwork, progress, and result-backed controls in shared Go widgets.
func (a *App) buildMediaPreview(result queryResult, data mediaPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-36)
	innerHeight := max(float32(0), height-28)
	artSize := min(float32(220), max(float32(104), min(innerWidth*0.4, innerHeight-30)))
	var artwork woxwidget.Widget = woxwidget.Container{Width: artSize, Height: artSize, Radius: artSize / 2, Color: woxui.Color{R: 18, G: 18, B: 22, A: 255}, Padding: woxwidget.Insets{Left: artSize*0.4 - 8, Top: artSize*0.4 - 2}, Child: woxwidget.Text{
		Value: "♪", Style: woxui.TextStyle{Size: max(float32(28), artSize*0.22), Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 107, B: 53, A: 255},
	}}
	if source, ok := parsePreviewImage(data.Artwork); ok {
		if image := a.imageFor(source); image != nil {
			artwork = woxwidget.Container{Width: artSize, Height: artSize, Radius: 14, Color: palette.queryBackground, Child: woxwidget.Image{Source: image, Width: artSize, Height: artSize}}
		}
	}
	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "Unknown track"
	}
	artist := strings.TrimSpace(data.Artist)
	if artist == "" {
		artist = "Unknown artist"
	}
	details := make([]string, 0, 2)
	if album := strings.TrimSpace(data.Album); album != "" {
		details = append(details, album)
	}
	if appName := strings.TrimSpace(data.AppName); appName != "" {
		details = append(details, appName)
	}
	metadataWidth := max(float32(120), innerWidth-artSize-24)
	duration := max(int64(0), data.Duration)
	position := min(max(int64(0), data.Position), duration)
	progress := float32(0)
	if duration > 0 {
		progress = float32(position) / float32(duration)
	}
	progressWidth := max(float32(80), metadataWidth-12)
	progressBar := woxwidget.Container{Width: progressWidth, Height: 5, Radius: 3, Color: palette.previewSplit, Child: woxwidget.Container{Width: progressWidth * progress, Height: 5, Radius: 3, Color: woxui.Color{R: 255, G: 107, B: 53, A: 255}}}
	control := func(id, label string, primary bool) woxwidget.Widget {
		background := palette.queryBackground
		if primary {
			background = woxui.Color{R: 255, G: 107, B: 53, A: 255}
		}
		return woxwidget.Gesture{ID: "media-preview-" + id, OnTap: func() { a.activateResultActionByID(result.QueryID, result.ID, id) }, Child: woxwidget.Container{
			Width: 46, Height: 38, Radius: 19, Color: background, Padding: woxwidget.Insets{Left: 15, Top: 10}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
		}}
	}
	toggleID := "media-control-play"
	toggleLabel := "▶"
	if data.IsPlaying {
		toggleID = "media-control-pause"
		toggleLabel = "Ⅱ"
	}
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		control("media-control-previous", "◀", false),
		control(toggleID, toggleLabel, true),
		control("media-control-next", "▶", false),
	}}
	info := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 9, Children: []woxwidget.Widget{
		woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
		woxwidget.Text{Value: artist, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.resultSubtitle},
		woxwidget.Text{Value: strings.Join(details, "  ·  "), Style: woxui.TextStyle{Size: 11}, Color: palette.resultSubtitle},
		woxwidget.Painter{Width: metadataWidth, Height: 8},
		progressBar,
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Text{Value: formatMediaDuration(position), Style: woxui.TextStyle{Size: 10}, Color: palette.resultSubtitle},
			woxwidget.Painter{Width: max(float32(0), progressWidth-72), Height: 14},
			woxwidget.Text{Value: formatMediaDuration(duration), Style: woxui.TextStyle{Size: 10}, Color: palette.resultSubtitle},
		}},
		woxwidget.Painter{Width: metadataWidth, Height: 6},
		controls,
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 12, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 24, Children: []woxwidget.Widget{
		artwork,
		woxwidget.Container{Width: metadataWidth, Height: innerHeight, Padding: woxwidget.Insets{Top: max(float32(0), (innerHeight-190)/2)}, Child: info},
	}}}
}
