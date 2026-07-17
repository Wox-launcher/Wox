package launcher

import (
	"strings"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildMediaPreview maps media metadata and result actions into the shared preview view.
func (a *App) buildMediaPreview(result queryResult, data mediaPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
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
	duration := max(int64(0), data.Duration)
	position := min(max(int64(0), data.Position), duration)
	var artwork *woxui.Image
	if source, ok := parsePreviewImage(data.Artwork); ok {
		artwork = a.imageFor(source)
	}
	action := func(id string) func() {
		return func() { a.activateResultActionByID(result.QueryID, result.ID, id) }
	}
	return previewview.MediaPreviewView(previewview.MediaPreviewProps{
		Width: width, Height: height, Title: title, Artist: artist, Details: strings.Join(details, "  ·  "), Artwork: artwork,
		Position: position, Duration: duration, Playing: data.IsPlaying, Theme: palette.componentTheme(),
		OnPrevious: action("media-control-previous"), OnPlay: action("media-control-play"), OnPause: action("media-control-pause"), OnNext: action("media-control-next"),
	})
}
