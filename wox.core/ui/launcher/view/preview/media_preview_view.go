package preview

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

var mediaAccent = woxui.Color{R: 255, G: 107, B: 53, A: 255}
var mediaVinylLabel = woxui.Color{R: 150, G: 0, B: 0, A: 255}

const (
	mediaRecordBoundaryKey   woxwidget.Key = "media-preview-record-boundary"
	mediaStatusBoundaryKey   woxwidget.Key = "media-preview-status-boundary"
	mediaMetadataBoundaryKey woxwidget.Key = "media-preview-metadata-boundary"
	mediaProgressBoundaryKey woxwidget.Key = "media-preview-progress-boundary"
	mediaControlsBoundaryKey woxwidget.Key = "media-preview-controls-boundary"
)

// MediaPreviewProps contains normalized media metadata and transport actions.
type MediaPreviewProps struct {
	Width          float32
	Height         float32
	Title          string
	Artist         string
	Album          string
	AppName        string
	Artwork        *woxui.Image
	Position       int64
	Duration       int64
	Playing        bool
	Theme          woxcomponent.Theme
	PreviousLabel  string
	ToggleLabel    string
	NextLabel      string
	ActionIdentity string
	Window         *woxui.Window
	OnPrevious     func()
	OnPlay         func()
	OnPause        func()
	OnNext         func()
}

type mediaRecordBoundaryProps struct {
	Size    float32
	Artwork *woxui.Image
	Playing bool
}

func (p mediaRecordBoundaryProps) Equal(other mediaRecordBoundaryProps) bool {
	return p == other
}

type mediaStatusBoundaryProps struct {
	Width   float32
	AppName string
	Playing bool
	Theme   woxcomponent.Theme
	Window  *woxui.Window
}

func (p mediaStatusBoundaryProps) Equal(other mediaStatusBoundaryProps) bool {
	return p == other
}

type mediaMetadataBoundaryProps struct {
	Width   float32
	Title   string
	Artist  string
	Album   string
	Compact bool
	Theme   woxcomponent.Theme
}

func (p mediaMetadataBoundaryProps) Equal(other mediaMetadataBoundaryProps) bool {
	return p == other
}

type mediaProgressBoundaryProps struct {
	Width     float32
	Position  int64
	Duration  int64
	Secondary woxui.Color
	Theme     woxcomponent.Theme
}

func (p mediaProgressBoundaryProps) Equal(other mediaProgressBoundaryProps) bool {
	return p == other
}

type mediaControlsBoundaryProps struct {
	Playing        bool
	Theme          woxcomponent.Theme
	PreviousLabel  string
	ToggleLabel    string
	NextLabel      string
	ActionIdentity string
	OnPrevious     func() `boundary:"stable"`
	OnPlay         func() `boundary:"stable"`
	OnPause        func() `boundary:"stable"`
	OnNext         func() `boundary:"stable"`
}

func (p mediaControlsBoundaryProps) Equal(other mediaControlsBoundaryProps) bool {
	return p.Playing == other.Playing && p.Theme == other.Theme && p.PreviousLabel == other.PreviousLabel && p.ToggleLabel == other.ToggleLabel && p.NextLabel == other.NextLabel && p.ActionIdentity == other.ActionIdentity
}

// MediaPreviewView builds the responsive Flutter media-preview composition.
func MediaPreviewView(props MediaPreviewProps) woxwidget.Widget {
	outerWidth := max(float32(0), props.Width-34)
	outerHeight := max(float32(0), props.Height-30)
	wide := outerWidth >= 690 && outerHeight >= 330
	horizontalPadding := float32(22)
	verticalPadding := float32(20)
	if wide {
		horizontalPadding = 34
		verticalPadding = 28
	}
	contentWidth := max(float32(0), outerWidth-horizontalPadding*2)
	contentHeight := max(float32(0), outerHeight-verticalPadding*2)

	var content woxwidget.Widget
	if wide {
		const gap = float32(38)
		columnsWidth := max(float32(0), contentWidth-gap)
		stageWidth := columnsWidth * 11 / 21
		detailsWidth := max(float32(0), columnsWidth-stageWidth)
		recordSize := min(float32(330), min(stageWidth, contentHeight))
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Align{Width: stageWidth, Height: contentHeight, Horizontal: 0.5, Vertical: 0.5, Child: mediaRecordBoundary(recordSize, props)},
			mediaTrackDetails(props, detailsWidth, contentHeight, false),
		}}
	} else {
		const gap = float32(14)
		stageHeight := max(float32(0), contentHeight*0.55-gap)
		detailsHeight := max(float32(0), contentHeight-stageHeight-gap)
		recordSize := min(float32(250), min(contentWidth, stageHeight))
		content = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Align{Width: contentWidth, Height: stageHeight, Horizontal: 0.5, Vertical: 0.5, Child: mediaRecordBoundary(recordSize, props)},
			mediaTrackDetails(props, contentWidth, detailsHeight, true),
		}}
	}

	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 16, Bottom: 14},
		Child: woxwidget.Container{Width: outerWidth, Height: outerHeight, Padding: woxwidget.Insets{Left: horizontalPadding, Top: verticalPadding, Right: horizontalPadding, Bottom: verticalPadding}, Child: content},
	}
}

// mediaRecordBoundary confines continuous rotation and tonearm animation damage to the record stage.
func mediaRecordBoundary(size float32, props MediaPreviewProps) woxwidget.Widget {
	boundaryProps := mediaRecordBoundaryProps{Size: size, Artwork: props.Artwork, Playing: props.Playing}
	return woxwidget.Boundary[mediaRecordBoundaryProps]{
		Key: mediaRecordBoundaryKey, Label: "media:record", Props: boundaryProps,
		Build: func(props mediaRecordBoundaryProps) woxwidget.Widget {
			return mediaRecordStage(props.Size, MediaPreviewProps{Artwork: props.Artwork, Playing: props.Playing})
		},
	}
}

// mediaRecordStage paints the vinyl, centered artwork, grooves, and playback-positioned tonearm.
func mediaRecordStage(size float32, props MediaPreviewProps) woxwidget.Widget {
	record := woxwidget.LoopAnimation{
		Key: "media-preview-record-rotation", Duration: 12 * time.Second, Paused: !props.Playing,
		Builder: func(progress float32) woxwidget.Widget { return mediaRecordPainter(size, props, progress) },
	}
	tonearmTarget := float32(0)
	if props.Playing {
		tonearmTarget = 1
	}
	tonearm := woxwidget.AnimatedFloat{
		Key: "media-preview-tonearm", Target: tonearmTarget, Duration: 480 * time.Millisecond, Curve: woxwidget.AnimationEaseInOutCubic,
		Builder: func(progress float32) woxwidget.Widget {
			return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				paintMediaTonearm(displayList, mediaRecordBounds(bounds), progress)
			}}
		},
	}
	return woxwidget.Stack{Width: size, Height: size, Children: []woxwidget.StackChild{{Child: record}, {Child: tonearm}}}
}

func mediaRecordPainter(size float32, props MediaPreviewProps, progress float32) woxwidget.Widget {
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return
		}
		disc := mediaRecordBounds(bounds)
		if disc.Width <= 0 {
			return
		}
		displayList.FillRoundedRect(woxui.Rect{X: disc.X + 3, Y: disc.Y + 7, Width: disc.Width, Height: disc.Height}, disc.Width/2, woxui.Color{A: 76})
		displayList.FillRoundedRect(disc, disc.Width/2, woxui.Color{R: 8, G: 8, B: 10, A: 255})
		radius := disc.Width / 2
		for index := 0; index < 9; index++ {
			grooveRadius := radius * (0.5 + float32(index)*0.052)
			alpha := uint8(7)
			if index%2 == 0 {
				alpha = 14
			}
			displayList.StrokeRoundedRect(woxui.Rect{X: disc.X + radius - grooveRadius, Y: disc.Y + radius - grooveRadius, Width: grooveRadius * 2, Height: grooveRadius * 2}, grooveRadius, 0.8, woxui.Color{R: 255, G: 255, B: 255, A: alpha})
		}

		labelSize := disc.Width * 0.43
		label := woxui.Rect{X: disc.X + (disc.Width-labelSize)/2, Y: disc.Y + (disc.Height-labelSize)/2, Width: labelSize, Height: labelSize}
		displayList.FillRoundedRect(label, labelSize/2, mediaVinylLabel)
		if props.Artwork != nil {
			artworkInset := labelSize * 0.06
			artwork := woxui.Rect{X: label.X + artworkInset, Y: label.Y + artworkInset, Width: label.Width - artworkInset*2, Height: label.Height - artworkInset*2}
			displayList.DrawRotatedRoundedImage(props.Artwork, artwork, progress*2*math.Pi, artwork.Width/2)
		} else {
			stemWidth := max(float32(2), labelSize*0.055)
			stemHeight := labelSize * 0.34
			stemX := label.X + labelSize*0.55
			stemY := label.Y + labelSize*0.27
			displayList.FillRoundedRect(woxui.Rect{X: stemX, Y: stemY, Width: stemWidth, Height: stemHeight}, stemWidth/2, mediaAccent)
			displayList.FillRoundedRect(woxui.Rect{X: stemX, Y: stemY, Width: labelSize * 0.18, Height: stemWidth}, stemWidth/2, mediaAccent)
			noteSize := labelSize * 0.17
			displayList.FillRoundedRect(woxui.Rect{X: stemX - noteSize + stemWidth, Y: stemY + stemHeight - noteSize/2, Width: noteSize, Height: noteSize}, noteSize/2, mediaAccent)
		}
		displayList.StrokeRoundedRect(label, labelSize/2, 1, woxui.Color{R: 255, G: 255, B: 255, A: 46})
	}}
}

func mediaRecordBounds(bounds woxui.Rect) woxui.Rect {
	size := min(bounds.Width, bounds.Height)
	if size <= 8 {
		return woxui.Rect{}
	}
	return woxui.Rect{X: bounds.X + (bounds.Width-size)/2 + 4, Y: bounds.Y + (bounds.Height-size)/2 + 2, Width: size - 8, Height: size - 8}
}

// paintMediaTonearm interpolates Flutter's parked and playing needle positions independently of record rotation.
func paintMediaTonearm(displayList *woxui.DisplayList, bounds woxui.Rect, playbackProgress float32) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	scale := min(bounds.Width, bounds.Height)
	pivot := woxui.Point{X: bounds.X + bounds.Width*0.80, Y: bounds.Y + bounds.Height*0.135}
	parked := woxui.Point{X: bounds.X + bounds.Width*0.84, Y: bounds.Y + bounds.Height*0.33}
	playing := woxui.Point{X: bounds.X + bounds.Width*0.69, Y: bounds.Y + bounds.Height*0.34}
	playbackProgress = min(max(float32(0), playbackProgress), float32(1))
	needle := woxui.Point{X: parked.X + (playing.X-parked.X)*playbackProgress, Y: parked.Y + (playing.Y-parked.Y)*playbackProgress}
	dx := needle.X - pivot.X
	dy := needle.Y - pivot.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return
	}
	directionX := dx / length
	directionY := dy / length
	perpendicularX := -directionY
	perpendicularY := directionX
	armEnd := woxui.Point{X: needle.X - directionX*scale*0.044, Y: needle.Y - directionY*scale*0.044}
	line := func(start, end woxui.Point, width float32, color woxui.Color) {
		half := width / 2
		displayList.FillConvexPolygon([]woxui.Point{
			{X: start.X + perpendicularX*half, Y: start.Y + perpendicularY*half},
			{X: end.X + perpendicularX*half, Y: end.Y + perpendicularY*half},
			{X: end.X - perpendicularX*half, Y: end.Y - perpendicularY*half},
			{X: start.X - perpendicularX*half, Y: start.Y - perpendicularY*half},
		}, color)
	}
	shadowOffset := woxui.Point{X: scale * 0.004, Y: scale * 0.007}
	line(woxui.Point{X: pivot.X + shadowOffset.X, Y: pivot.Y + shadowOffset.Y}, woxui.Point{X: armEnd.X + shadowOffset.X, Y: armEnd.Y + shadowOffset.Y}, scale*0.017, woxui.Color{A: 112})
	line(pivot, armEnd, scale*0.011, woxui.Color{R: 196, G: 190, B: 182, A: 255})
	line(pivot, armEnd, scale*0.0024, woxui.Color{R: 255, G: 255, B: 255, A: 96})

	shellWidth := scale * 0.012
	shellEnd := woxui.Point{X: needle.X - directionX*scale*0.007, Y: needle.Y - directionY*scale*0.007}
	displayList.FillConvexPolygon([]woxui.Point{
		{X: armEnd.X + perpendicularX*shellWidth, Y: armEnd.Y + perpendicularY*shellWidth},
		{X: shellEnd.X + perpendicularX*shellWidth*0.58, Y: shellEnd.Y + perpendicularY*shellWidth*0.58},
		{X: shellEnd.X - perpendicularX*shellWidth*0.58, Y: shellEnd.Y - perpendicularY*shellWidth*0.58},
		{X: armEnd.X - perpendicularX*shellWidth, Y: armEnd.Y - perpendicularY*shellWidth},
	}, woxui.Color{R: 88, G: 84, B: 81, A: 255})
	needleRadius := max(float32(1), scale*0.004)
	displayList.FillRoundedRect(woxui.Rect{X: needle.X - needleRadius, Y: needle.Y - needleRadius, Width: needleRadius * 2, Height: needleRadius * 2}, needleRadius, woxui.Color{R: 238, G: 112, B: 72, A: 255})
	pivotRadius := scale * 0.035
	displayList.FillRoundedRect(woxui.Rect{X: pivot.X - pivotRadius + shadowOffset.X, Y: pivot.Y - pivotRadius + shadowOffset.Y, Width: pivotRadius * 2, Height: pivotRadius * 2}, pivotRadius, woxui.Color{A: 107})
	displayList.FillRoundedRect(woxui.Rect{X: pivot.X - pivotRadius, Y: pivot.Y - pivotRadius, Width: pivotRadius * 2, Height: pivotRadius * 2}, pivotRadius, woxui.Color{R: 92, G: 88, B: 84, A: 255})
	innerRadius := scale * 0.019
	displayList.FillRoundedRect(woxui.Rect{X: pivot.X - innerRadius, Y: pivot.Y - innerRadius, Width: innerRadius * 2, Height: innerRadius * 2}, innerRadius, woxui.Color{R: 200, G: 189, B: 174, A: 255})
}

// mediaTrackDetails builds Flutter's status, metadata, progress, and controls stack.
func mediaTrackDetails(props MediaPreviewProps, width, height float32, compact bool) woxwidget.Widget {
	secondary := mediaColorAlpha(props.Theme.PreviewPropertyContent, 0.72)
	if props.Theme.PreviewPropertyContent.A == 0 {
		secondary = mediaColorAlpha(props.Theme.PreviewText, 0.72)
	}
	statusProps := mediaStatusBoundaryProps{Width: width, AppName: props.AppName, Playing: props.Playing, Theme: props.Theme, Window: props.Window}
	status := woxwidget.Boundary[mediaStatusBoundaryProps]{
		Key: mediaStatusBoundaryKey, Label: "media:status", Props: statusProps,
		Build: func(props mediaStatusBoundaryProps) woxwidget.Widget {
			return mediaPlaybackStatus(MediaPreviewProps{AppName: props.AppName, Playing: props.Playing, Theme: props.Theme, Window: props.Window}, props.Width)
		},
	}
	statusGap := float32(22)
	progressGap := float32(30)
	controlsGap := float32(22)
	crossAxisAlignment := woxwidget.CrossAxisStart
	horizontalAlignment := float32(0)
	if compact {
		statusGap = 10
		progressGap = 14
		controlsGap = 14
		crossAxisAlignment = woxwidget.CrossAxisCenter
		horizontalAlignment = 0.5
	}
	children := []woxwidget.Widget{status, woxwidget.Painter{Width: width, Height: statusGap}}
	metadataProps := mediaMetadataBoundaryProps{Width: width, Title: props.Title, Artist: props.Artist, Album: props.Album, Compact: compact, Theme: props.Theme}
	children = append(children, woxwidget.Boundary[mediaMetadataBoundaryProps]{
		Key: mediaMetadataBoundaryKey, Label: "media:metadata", Props: metadataProps,
		Build: func(props mediaMetadataBoundaryProps) woxwidget.Widget { return mediaMetadata(props) },
	})
	progressProps := mediaProgressBoundaryProps{Width: width, Position: props.Position, Duration: props.Duration, Secondary: secondary, Theme: props.Theme}
	controlsProps := mediaControlsBoundaryProps{
		Playing: props.Playing, Theme: props.Theme, PreviousLabel: props.PreviousLabel, ToggleLabel: props.ToggleLabel, NextLabel: props.NextLabel,
		ActionIdentity: props.ActionIdentity,
		OnPrevious:     props.OnPrevious, OnPlay: props.OnPlay, OnPause: props.OnPause, OnNext: props.OnNext,
	}
	children = append(children,
		woxwidget.Painter{Width: width, Height: progressGap},
		woxwidget.Boundary[mediaProgressBoundaryProps]{
			Key: mediaProgressBoundaryKey, Label: "media:progress", Props: progressProps,
			Build: func(props mediaProgressBoundaryProps) woxwidget.Widget {
				return mediaProgressBar(MediaPreviewProps{Position: props.Position, Duration: props.Duration, Theme: props.Theme}, props.Width, props.Secondary)
			},
		},
		woxwidget.Painter{Width: width, Height: controlsGap},
		woxwidget.Align{Width: width, Height: 48, Horizontal: 0.5, Child: woxwidget.Boundary[mediaControlsBoundaryProps]{
			Key: mediaControlsBoundaryKey, Label: "media:controls", Props: controlsProps,
			Build: func(props mediaControlsBoundaryProps) woxwidget.Widget {
				return mediaControls(MediaPreviewProps{
					Playing: props.Playing, Theme: props.Theme, PreviousLabel: props.PreviousLabel, ToggleLabel: props.ToggleLabel, NextLabel: props.NextLabel,
					OnPrevious: props.OnPrevious, OnPlay: props.OnPlay, OnPause: props.OnPause, OnNext: props.OnNext,
				})
			},
		}},
	)
	content := woxwidget.Flex{Axis: woxwidget.Vertical, CrossAxisAlignment: crossAxisAlignment, Children: children}
	return woxwidget.Align{Width: width, Height: height, Horizontal: horizontalAlignment, Vertical: 0.5, Child: content}
}

// mediaMetadata keeps track text independent from position and playback-state updates.
func mediaMetadata(props mediaMetadataBoundaryProps) woxwidget.Widget {
	titleSize := float32(30)
	artistSize := float32(17)
	titleLines := 3
	if props.Compact {
		titleSize = 22
		artistSize = 14
		titleLines = 1
	}
	children := []woxwidget.Widget{woxwidget.TextBlock{Value: props.Title, Width: props.Width, MaxLines: titleLines, LineHeight: titleSize * 1.14, Style: woxui.TextStyle{Size: titleSize, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}}
	if strings.TrimSpace(props.Artist) != "" {
		children = append(children, woxwidget.Painter{Width: props.Width, Height: 9}, woxwidget.Text{Value: props.Artist, Style: woxui.TextStyle{Size: artistSize, Weight: woxui.FontWeightSemibold}, Color: mediaColorAlpha(props.Theme.PreviewText, 0.78)})
	}
	if !props.Compact && strings.TrimSpace(props.Album) != "" {
		secondary := mediaColorAlpha(props.Theme.PreviewPropertyContent, 0.72)
		if props.Theme.PreviewPropertyContent.A == 0 {
			secondary = mediaColorAlpha(props.Theme.PreviewText, 0.72)
		}
		children = append(children, woxwidget.Painter{Width: props.Width, Height: 5}, woxwidget.Text{Value: props.Album, Style: woxui.TextStyle{Size: 13}, Color: secondary})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

// mediaPlaybackStatus builds the compact source and playback-state pill.
func mediaPlaybackStatus(props MediaPreviewProps, availableWidth float32) woxwidget.Widget {
	source := strings.TrimSpace(props.AppName)
	style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
	textWidth := float32(utf8.RuneCountInString(source)) * 6.5
	if props.Window != nil {
		if metrics, err := props.Window.MeasureText(source, style); err == nil {
			textWidth = metrics.Size.Width
		}
	}
	textWidth = min(float32(180), max(float32(0), textWidth))
	width := float32(35)
	if source != "" {
		width += 7 + textWidth
	}
	width = min(width, availableWidth)
	icon := woxwidget.Painter{Width: 15, Height: 15, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		if props.Playing {
			for index, height := range []float32{7, 13, 9} {
				x := bounds.X + float32(index)*5 + 1
				displayList.FillRoundedRect(woxui.Rect{X: x, Y: bounds.Y + (bounds.Height-height)/2, Width: 2.5, Height: height}, 1.25, mediaAccent)
			}
		} else {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 4, Y: bounds.Y + 3, Width: 2.5, Height: 9}, 1, mediaAccent)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 9, Y: bounds.Y + 3, Width: 2.5, Height: 9}, 1, mediaAccent)
		}
	}}
	children := []woxwidget.Widget{icon}
	if source != "" {
		children = append(children, woxwidget.Text{Value: source, Style: style, Color: mediaColorAlpha(props.Theme.PreviewText, 0.74)})
	}
	return woxwidget.Container{
		Width: width, Height: 28, Radius: 14, Color: mediaColorAlpha(props.Theme.PreviewText, 0.055), BorderColor: mediaColorAlpha(props.Theme.PreviewText, 0.09), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 10, Bottom: 6}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 7, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
	}
}

// mediaProgressBar keeps the elapsed and total time aligned to the progress ends.
func mediaProgressBar(props MediaPreviewProps, width float32, secondary woxui.Color) woxwidget.Widget {
	progress := float32(0)
	if props.Duration > 0 {
		progress = min(float32(1), max(float32(0), float32(props.Position)/float32(props.Duration)))
	}
	bar := woxwidget.Container{Width: width, Height: 5, Radius: 3, Color: mediaColorAlpha(props.Theme.PreviewText, 0.12), Child: woxwidget.Container{Width: width * progress, Height: 5, Radius: 3, Color: mediaAccent}}
	position := formatMediaDuration(props.Position)
	duration := formatMediaDuration(props.Duration)
	timeWidth := float32(utf8.RuneCountInString(position)+utf8.RuneCountInString(duration)) * 7
	times := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Text{Value: position, Style: woxui.TextStyle{Size: 12}, Color: secondary},
		woxwidget.Painter{Width: max(float32(0), width-timeWidth), Height: 14},
		woxwidget.Text{Value: duration, Style: woxui.TextStyle{Size: 12}, Color: secondary},
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{bar, times}}
}

// mediaControls builds Flutter's compact three-button transport pill.
func mediaControls(props MediaPreviewProps) woxwidget.Widget {
	toggleKind := "play"
	toggleLabel := props.ToggleLabel
	if toggleLabel == "" {
		toggleLabel = "Play"
	}
	toggleAction := props.OnPlay
	toggleID := "media-control-play"
	if props.Playing {
		toggleKind = "pause"
		if props.ToggleLabel == "" {
			toggleLabel = "Pause"
		}
		toggleAction = props.OnPause
		toggleID = "media-control-pause"
	}
	previousLabel := props.PreviousLabel
	if previousLabel == "" {
		previousLabel = "Previous"
	}
	nextLabel := props.NextLabel
	if nextLabel == "" {
		nextLabel = "Next"
	}
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 28, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		mediaControl("media-control-previous", previousLabel, "previous", false, props.OnPrevious, props.Theme),
		mediaControl(toggleID, toggleLabel, toggleKind, true, toggleAction, props.Theme),
		mediaControl("media-control-next", nextLabel, "next", false, props.OnNext, props.Theme),
	}}
	return woxwidget.Container{
		Width: 176, Height: 48, Radius: 24, Color: mediaColorAlpha(props.Theme.PreviewText, 0.035), BorderColor: mediaColorAlpha(props.Theme.PreviewText, 0.10), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 8, Top: 6, Right: 8, Bottom: 6}, Child: controls,
	}
}

// mediaControl maps one transport action onto the shared accessible icon button.
func mediaControl(id, label, kind string, emphasized bool, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	size := float32(34)
	color := mediaColorAlpha(theme.PreviewText, 0.72)
	background := woxui.Color{}
	hoverBackground := mediaColorAlpha(theme.PreviewText, 0.08)
	if emphasized {
		size = 36
		color = mediaAccent
		background = mediaColorAlpha(mediaAccent, 0.13)
		hoverBackground = mediaColorAlpha(mediaAccent, 0.22)
	}
	return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "media-preview-" + id, Label: label, Icon: mediaTransportGlyph(kind, 20, color), Width: size, Height: size, Radius: size / 2,
		Background: background, HoverBackground: hoverBackground, FocusRingColor: theme.Cursor, Disabled: onTap == nil, OnTap: onTap,
	})
}

// mediaTransportGlyph paints the small transport symbols without font-dependent glyphs.
func mediaTransportGlyph(kind string, size float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		centerX := bounds.X + bounds.Width/2
		centerY := bounds.Y + bounds.Height/2
		if kind == "pause" {
			displayList.FillRoundedRect(woxui.Rect{X: centerX - 4.5, Y: centerY - 7, Width: 3, Height: 14}, 1, color)
			displayList.FillRoundedRect(woxui.Rect{X: centerX + 1.5, Y: centerY - 7, Width: 3, Height: 14}, 1, color)
			return
		}
		points := []woxui.Point{{X: centerX - 5, Y: centerY - 7}, {X: centerX + 6, Y: centerY}, {X: centerX - 5, Y: centerY + 7}}
		if kind == "previous" {
			points = []woxui.Point{{X: centerX + 5, Y: centerY - 7}, {X: centerX - 6, Y: centerY}, {X: centerX + 5, Y: centerY + 7}}
			displayList.FillRoundedRect(woxui.Rect{X: centerX - 7, Y: centerY - 7, Width: 2, Height: 14}, 1, color)
		} else if kind == "next" {
			displayList.FillRoundedRect(woxui.Rect{X: centerX + 5, Y: centerY - 7, Width: 2, Height: 14}, 1, color)
		}
		displayList.FillConvexPolygon(points, color)
	}}
}

func mediaColorAlpha(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(max(float32(0), opacity), float32(1))*255 + 0.5)
	return color
}

func formatMediaDuration(seconds int64) string {
	seconds = max(int64(0), seconds)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
