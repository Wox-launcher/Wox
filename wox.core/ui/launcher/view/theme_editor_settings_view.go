package view

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const themeEditorControlPaneHeight = float32(140)
const themeEditorColorWheelSize = float32(220)

var themeEditorColorWheelImage = buildThemeEditorColorWheelImage()

// ThemeEditorColorPickerProps contains the live color controls shown in the token dialog.
type ThemeEditorColorPickerProps struct {
	Color              woxui.Color
	Hue                float64
	Saturation         float64
	Brightness         float64
	Opacity            float64
	BrightnessLabel    string
	OpacityLabel       string
	ColorField         woxwidget.Widget
	Theme              woxcomponent.Theme
	OnHueSaturation    func(hue, saturation float64)
	OnBrightnessChange func(value float64)
	OnOpacityChange    func(value float64)
}

// ThemeEditorColorPicker mirrors Flutter's wheel, swatch, CSS input, and sliders.
func ThemeEditorColorPicker(props ThemeEditorColorPickerProps) woxwidget.Widget {
	const contentWidth = float32(360)
	wheel := themeEditorColorWheel(props)
	colorRow := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 48, Height: 36, Radius: 6, Color: props.Color, BorderColor: themeAlpha(props.Theme.PreviewSplit, 200), BorderWidth: 1},
		props.ColorField,
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Align{Width: contentWidth, Height: themeEditorColorWheelSize, Horizontal: 0.5, Child: wheel},
		woxwidget.Align{Width: contentWidth, Height: 36, Horizontal: 0.5, Child: colorRow},
		themeEditorColorSlider("theme-editor-brightness", props.BrightnessLabel, props.Brightness, props.Theme, props.OnBrightnessChange),
		themeEditorColorSlider("theme-editor-opacity", props.OpacityLabel, props.Opacity, props.Theme, props.OnOpacityChange),
	}}
}

// themeEditorColorWheel maps pointer positions to Flutter-compatible hue and saturation.
func themeEditorColorWheel(props ThemeEditorColorPickerProps) woxwidget.Widget {
	radius := themeEditorColorWheelSize / 2
	angle := props.Hue * math.Pi / 180
	thumbX := radius + float32(math.Cos(angle)*props.Saturation)*radius
	thumbY := radius + float32(math.Sin(angle)*props.Saturation)*radius
	thumb := woxwidget.Stack{Width: 20, Height: 20, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: 20, Height: 20, Radius: 10, BorderColor: woxui.Color{A: 128}, BorderWidth: 1.5}},
		{Left: 2, Top: 2, Child: woxwidget.Container{Width: 16, Height: 16, Radius: 8, Color: props.Color, BorderColor: woxui.Color{R: 255, G: 255, B: 255, A: 255}, BorderWidth: 2}},
	}}
	setPosition := func(position woxui.Point) {
		if props.OnHueSaturation == nil {
			return
		}
		x := float64(position.X - radius)
		y := float64(position.Y - radius)
		distance := math.Min(math.Hypot(x, y), float64(radius))
		hue := math.Atan2(y, x) * 180 / math.Pi
		if hue < 0 {
			hue += 360
		}
		props.OnHueSaturation(hue, distance/float64(radius))
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key("theme-editor-color-wheel"), AutomationID: "theme-editor-color-wheel", Role: woxui.AccessibilityRoleGroup,
		Label: "Color wheel", Value: fmt.Sprintf("%.0f°, %.0f%%", props.Hue, props.Saturation*100),
		Child: woxwidget.Gesture{
			ID: "theme-editor-color-wheel-pointer", OnPanStart: setPosition, OnPanUpdate: setPosition,
			Child: woxwidget.Stack{Width: themeEditorColorWheelSize, Height: themeEditorColorWheelSize, Children: []woxwidget.StackChild{
				{Child: woxwidget.Image{Source: themeEditorColorWheelImage, Width: themeEditorColorWheelSize, Height: themeEditorColorWheelSize}},
				{Left: thumbX - 10, Top: thumbY - 10, Child: thumb},
			}},
		},
	}
}

// themeEditorColorSlider keeps pointer and accessibility value changes on the same normalized path.
func themeEditorColorSlider(id, label string, value float64, theme woxcomponent.Theme, onChanged func(float64)) woxwidget.Widget {
	const trackWidth = float32(194)
	const thumbSize = float32(18)
	normalized := min(float64(1), max(float64(0), value))
	activeWidth := float32(normalized) * trackWidth
	setPosition := func(position woxui.Point) {
		if onChanged != nil {
			onChanged(min(float64(1), max(float64(0), float64(position.X)/float64(trackWidth))))
		}
	}
	track := woxwidget.Gesture{ID: id + "-pointer", OnPanStart: setPosition, OnPanUpdate: setPosition, Child: woxwidget.Stack{
		Width: trackWidth, Height: thumbSize, Children: []woxwidget.StackChild{
			{Top: 7, Child: woxwidget.Container{Width: trackWidth, Height: 4, Radius: 2, Color: themeAlpha(theme.PreviewSplit, 150)}},
			{Top: 7, Child: woxwidget.Container{Width: activeWidth, Height: 4, Radius: 2, Color: theme.SelectedBackground}},
			{Left: max(float32(0), min(trackWidth-thumbSize, activeWidth-thumbSize/2)), Child: woxwidget.Container{Width: thumbSize, Height: thumbSize, Radius: thumbSize / 2, Color: theme.ResultSubtitle}},
		},
	}}
	semanticTrack := woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleProgressBar, Label: label, Value: fmt.Sprintf("%.0f%%", normalized*100),
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionSetValue, woxui.AccessibilityActionIncrement, woxui.AccessibilityActionDecrement},
		OnAction: func(action woxui.AccessibilityAction, raw string) error {
			next := normalized
			switch action {
			case woxui.AccessibilityActionIncrement:
				next += 0.01
			case woxui.AccessibilityActionDecrement:
				next -= 0.01
			case woxui.AccessibilityActionSetValue:
				parsed, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(raw), "%"), 64)
				if err != nil {
					return err
				}
				if parsed > 1 {
					parsed /= 100
				}
				next = parsed
			}
			if onChanged != nil {
				onChanged(min(float64(1), max(float64(0), next)))
			}
			return nil
		},
		Child: track,
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 70, Height: 24, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}},
		woxwidget.Align{Width: trackWidth, Height: 24, Vertical: 0.5, Child: semanticTrack},
		woxwidget.Container{Width: 46, Height: 24, Padding: woxwidget.Insets{Left: 10, Top: 5}, Child: woxwidget.Text{Value: fmt.Sprintf("%.0f%%", normalized*100), Style: woxui.TextStyle{Size: 12}, Color: theme.ResultTitle}},
	}}
}

// buildThemeEditorColorWheelImage rasterizes the static sweep/radial gradient once for every dialog.
func buildThemeEditorColorWheelImage() *woxui.Image {
	const size = 440
	center := float64(size-1) / 2
	radius := center
	source := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			deltaX := float64(x) - center
			deltaY := float64(y) - center
			distance := math.Hypot(deltaX, deltaY)
			if distance > radius {
				continue
			}
			hue := math.Atan2(deltaY, deltaX) * 180 / math.Pi
			if hue < 0 {
				hue += 360
			}
			red, green, blue := themeEditorWheelRGB(hue, distance/radius)
			source.SetNRGBA(x, y, color.NRGBA{R: red, G: green, B: blue, A: 255})
		}
	}
	result, _ := woxui.NewImage(source)
	return result
}

// themeEditorWheelRGB renders a full-brightness HSV sample for the color wheel.
func themeEditorWheelRGB(hue, saturation float64) (uint8, uint8, uint8) {
	chroma := saturation
	section := hue / 60
	offset := chroma * (1 - math.Abs(math.Mod(section, 2)-1))
	var red, green, blue float64
	switch int(section) {
	case 0:
		red, green = chroma, offset
	case 1:
		red, green = offset, chroma
	case 2:
		green, blue = chroma, offset
	case 3:
		green, blue = offset, chroma
	case 4:
		red, blue = offset, chroma
	default:
		red, blue = chroma, offset
	}
	match := 1 - chroma
	return uint8(math.Round((red + match) * 255)), uint8(math.Round((green + match) * 255)), uint8(math.Round((blue + match) * 255))
}

// ThemeEditorColorToken contains one editable color and its resolved preview swatch.
type ThemeEditorColorToken struct {
	Key   string
	Label string
	Color woxui.Color
}

// ThemeEditorColorGroup contains the tokens shown together in the bottom editor pane.
type ThemeEditorColorGroup struct {
	Label      string
	LabelWidth float32
	Tokens     []ThemeEditorColorToken
}

// ThemeEditorPreviewGeometry carries the non-editable theme measurements used by the real launcher surface.
type ThemeEditorPreviewGeometry struct {
	AppPadding             woxwidget.Insets
	QueryRadius            float32
	ResultContainerPadding woxwidget.Insets
	ResultItemPadding      woxwidget.Insets
	ResultItemRadius       float32
	ToolbarPadding         woxwidget.Insets
}

// ThemeEditorSettingsProps contains the Flutter-aligned live preview and editor actions.
type ThemeEditorSettingsProps struct {
	Width                float32
	Height               float32
	Theme                woxcomponent.Theme
	DraftTheme           woxcomponent.Theme
	ResultTail           woxui.Color
	SelectedTail         woxui.Color
	Groups               []ThemeEditorColorGroup
	ActiveGroup          int
	Dirty                bool
	Saving               bool
	CanOverwrite         bool
	Error                string
	Wallpaper            *woxui.Image
	WallpaperBlurred     *woxui.Image
	PreviewGeometry      ThemeEditorPreviewGeometry
	FlashToken           string
	LocateIcon           *woxui.Image
	DiscardIcon          *woxui.Image
	OverwriteIcon        *woxui.Image
	SaveAsIcon           *woxui.Image
	LocateLabel          string
	DiscardLabel         string
	OverwriteLabel       string
	SaveAsLabel          string
	SavingLabel          string
	PreviewResultTitle   string
	PreviewResultState   string
	PreviewTailP1Width   float32
	PreviewTail4msWidth  float32
	PreviewTail13msWidth float32
	Window               *woxui.Window
	QueryBoxLabel        string
	ResultsLabel         string
	ToolbarCopyLabel     string
	ToolbarMoreLabel     string
	Dialog               woxwidget.Widget
	OnSelectGroup        func(int)
	OnEditToken          func(string)
	OnLocateToken        func(string)
	OnDiscard            func()
	OnOverwrite          func()
	OnSaveAs             func()
}

// ThemeEditorSettingsView mirrors Flutter's large desktop preview and compact bottom control pane.
func ThemeEditorSettingsView(props ThemeEditorSettingsProps) woxwidget.Widget {
	previewHeight := max(float32(0), props.Height-themeEditorControlPaneHeight)
	base := woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		themeEditorLivePreview(props, props.Width, previewHeight),
		themeEditorControlPane(props, props.Width, themeEditorControlPaneHeight),
	}}
	if props.Dialog == nil {
		return base
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: base}, {Child: props.Dialog}}}
}

func themeEditorLivePreview(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	stageWidth := min(float32(900), max(float32(0), width-36))
	stageHeight := min(float32(420), max(float32(0), height-20))
	stageLeft := max(float32(0), (width-stageWidth)/2)
	stageTop := max(float32(0), (height-stageHeight)/2)
	windowWidth := min(float32(780), max(float32(320), stageWidth*0.78))
	windowHeight := min(float32(360), max(float32(240), stageHeight*0.82))
	windowLeft := max(float32(0), (stageWidth-windowWidth)/2)
	windowTop := max(float32(0), (stageHeight-windowHeight)/2)

	stageColor := props.Theme.QueryBackground
	stage := woxwidget.Stack{Width: stageWidth, Height: stageHeight, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, Color: stageColor}},
	}}
	if props.Wallpaper != nil {
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Clip{Width: stageWidth, Height: stageHeight, Child: woxwidget.Image{Source: props.Wallpaper, Width: stageWidth, Height: stageHeight}}})
	} else {
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Color: woxui.Color{A: 255}}})
	}
	stage.Children = append(stage.Children,
		woxwidget.StackChild{Left: windowLeft, Top: windowTop, Child: themeEditorPreviewWindow(props, windowWidth, windowHeight)},
		woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, BorderColor: themeAlpha(props.Theme.PreviewSplit, 150), BorderWidth: 1}},
	)
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Left: stageLeft, Top: stageTop, Child: stage}}}
}

func themeEditorPreviewWindow(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	const queryHeight = float32(55)
	const toolbarHeight = float32(40)
	contentWidth := max(float32(0), width-props.PreviewGeometry.AppPadding.Left-props.PreviewGeometry.AppPadding.Right)
	contentHeight := max(float32(0), height-toolbarHeight-props.PreviewGeometry.AppPadding.Top-props.PreviewGeometry.AppPadding.Bottom)
	bodyHeight := max(float32(0), contentHeight-queryHeight)
	body := themeEditorPreviewResults(props, contentWidth, bodyHeight)
	if props.ActiveGroup == 3 {
		body = themeEditorPreviewWithTextPanel(props, contentWidth, bodyHeight)
	} else if props.ActiveGroup == 4 {
		body = themeEditorPreviewWithActionPanel(props, contentWidth, bodyHeight)
	}
	borderColor := themeAlpha(props.DraftTheme.PreviewSplit, 150)
	borderWidth := float32(1)
	if props.FlashToken == "AppBackgroundColor" {
		borderColor = themeEditorFlashColor()
		borderWidth = 2
	}
	children := []woxwidget.StackChild{}
	if props.WallpaperBlurred != nil {
		children = append(children, woxwidget.StackChild{Child: woxwidget.Image{Source: props.WallpaperBlurred, Width: width, Height: height}})
	} else {
		children = append(children, woxwidget.StackChild{Child: woxwidget.Container{Width: width, Height: height, Color: woxui.Color{A: 255}}})
	}
	children = append(children,
		woxwidget.StackChild{Child: woxwidget.Container{Width: width, Height: height, Radius: 12, Color: themeEditorMicaSurfaceColor(props.DraftTheme.Background)}},
		woxwidget.StackChild{Left: props.PreviewGeometry.AppPadding.Left, Top: props.PreviewGeometry.AppPadding.Top, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			themeEditorPreviewQuery(props, contentWidth, queryHeight), body,
		}}},
		woxwidget.StackChild{AnchorBottom: true, Child: themeEditorPreviewToolbar(props, width, toolbarHeight)},
		woxwidget.StackChild{Child: woxwidget.Container{Width: width, Height: height, Radius: 12, BorderColor: borderColor, BorderWidth: borderWidth}},
	)
	return woxwidget.Stack{Width: width, Height: height, Children: children}
}

func themeEditorPreviewQuery(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	selection := woxui.Color{}
	selectionText := props.DraftTheme.QueryText
	if props.ActiveGroup == 1 {
		selection = props.DraftTheme.SelectionBackground
		selectionText = props.DraftTheme.SelectionText
	}
	queryContentWidth := max(float32(0), width-14)
	memoryWidth := float32(90)
	queryWidth := max(float32(0), queryContentWidth-memoryWidth)
	themeTextWidth := float32(66)
	editTextWidth := float32(34)
	themeText := themeEditorFlashOverlay(
		woxwidget.Text{Value: "theme ", Style: woxui.TextStyle{Size: 20}, Color: props.DraftTheme.QueryText},
		themeTextWidth, 26, 3, props.FlashToken == "QueryBoxFontColor",
	)
	editText := themeEditorFlashOverlay(
		woxwidget.Container{Width: editTextWidth, Height: 26, Color: selection, Child: woxwidget.Text{Value: "edit", Style: woxui.TextStyle{Size: 20}, Color: selectionText}},
		editTextWidth, 26, 3, props.FlashToken == "QueryBoxTextSelectionBackgroundColor",
	)
	cursor := themeEditorFlashOverlay(
		woxwidget.Align{Width: 8, Height: 26, Horizontal: 0.5, Child: woxwidget.Container{Width: 2, Height: 26, Color: props.DraftTheme.Cursor}},
		8, 26, 2, props.FlashToken == "QueryBoxCursorColor",
	)
	query := woxwidget.Container{Width: queryWidth, Height: height, Padding: woxwidget.Insets{Top: 17}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		themeText, editText, cursor,
	}}}
	memory := woxwidget.Align{Width: memoryWidth, Height: height, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{Value: "⚙  761 MB", Style: woxui.TextStyle{Size: 12}, Color: themeAlpha(props.DraftTheme.QueryText, 178)}}
	box := woxwidget.Container{Width: width, Height: height, Radius: props.PreviewGeometry.QueryRadius, Color: props.DraftTheme.QueryBackground, Padding: woxwidget.Insets{Left: 8, Right: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{query, memory},
	}}
	return themeEditorFlashOverlay(box, width, height, props.PreviewGeometry.QueryRadius, props.FlashToken == "QueryBoxBackgroundColor")
}

func themeEditorPreviewResults(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	padding := props.PreviewGeometry.ResultContainerPadding
	return woxwidget.Container{Width: width, Height: height, Padding: padding, Child: themeEditorPreviewResultRows(props, max(float32(0), width-padding.Left-padding.Right), max(float32(0), height-padding.Top-padding.Bottom))}
}

func themeEditorPreviewResultRows(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	rowHeight := max(float32(44), 50+props.PreviewGeometry.ResultItemPadding.Top+props.PreviewGeometry.ResultItemPadding.Bottom)
	titles := []string{props.PreviewResultTitle, props.QueryBoxLabel, props.ResultsLabel}
	subtitles := []string{props.PreviewResultState, "QueryBoxBackgroundColor", "ResultItemActiveBackgroundColor"}
	icons := []struct {
		glyph string
		color woxui.Color
	}{{"⚙", woxui.Color{R: 139, G: 92, B: 246, A: 255}}, {"⌕", woxui.Color{R: 14, G: 165, B: 233, A: 255}}, {"≡", woxui.Color{R: 34, G: 197, B: 94, A: 255}}}
	rows := make([]woxwidget.Widget, 0, len(titles))
	for index := range titles {
		background := woxui.Color{}
		titleColor := props.DraftTheme.ResultTitle
		subtitleColor := props.DraftTheme.ResultSubtitle
		if index == 0 {
			background = props.DraftTheme.SelectedBackground
			titleColor = props.DraftTheme.SelectedTitle
			subtitleColor = props.DraftTheme.SelectedSubtitle
		}
		innerHeight := max(float32(0), rowHeight-props.PreviewGeometry.ResultItemPadding.Top-props.PreviewGeometry.ResultItemPadding.Bottom)
		titleFlash := (index == 0 && props.FlashToken == "ResultItemActiveTitleColor") || (index > 0 && props.FlashToken == "ResultItemTitleColor")
		subtitleFlash := index > 0 && props.FlashToken == "ResultItemSubTitleColor"
		tailColor := props.ResultTail
		if index == 0 {
			tailColor = props.SelectedTail
		}
		duration := map[bool]LauncherResultTail{
			true:  {Text: "13ms", Width: props.PreviewTail13msWidth, Height: 22},
			false: {Text: "4ms", Width: props.PreviewTail4msWidth, Height: 22},
		}[index == 0]
		tailWidth := 20 + props.PreviewTailP1Width + duration.Width
		innerWidth := max(float32(0), width-props.PreviewGeometry.ResultItemPadding.Left-props.PreviewGeometry.ResultItemPadding.Right)
		textWidth := max(float32(0), innerWidth-34-20-tailWidth)
		flashTextWidth := min(float32(210), max(float32(80), textWidth))
		texts := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			themeEditorFlashOverlay(woxwidget.Text{Value: titles[index], Style: woxui.TextStyle{Size: 13}, Color: titleColor}, flashTextWidth, 18, 3, titleFlash),
			themeEditorFlashOverlay(woxwidget.Text{Value: subtitles[index], Style: woxui.TextStyle{Size: 10}, Color: subtitleColor}, flashTextWidth, 15, 3, subtitleFlash),
		}}
		tails := launcherResultTails([]LauncherResultTail{
			{Text: "P1", Width: props.PreviewTailP1Width, Height: 22},
			duration,
		}, tailWidth, 30, tailColor, index == 0)
		var tailView woxwidget.Widget = tails
		tailView = themeEditorFlashOverlay(tailView, tailWidth, 30, 4, index > 0 && props.FlashToken == "ResultItemTailTextColor")
		tail := woxwidget.Align{Width: tailWidth, Height: innerHeight, Vertical: 0.5, Child: tailView}
		row := woxwidget.Container{Width: width, Height: rowHeight, Radius: props.PreviewGeometry.ResultItemRadius, Color: background, Padding: props.PreviewGeometry.ResultItemPadding, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Align{Width: 34, Height: innerHeight, Vertical: 0.5, Child: woxwidget.Container{Width: 28, Height: 28, Radius: 6, Color: icons[index].color, Padding: woxwidget.Insets{Left: 7, Top: 4}, Child: woxwidget.Text{Value: icons[index].glyph, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}},
				woxwidget.Align{Width: textWidth, Height: innerHeight, Vertical: 0.5, Child: texts},
				tail,
			},
		}}
		rows = append(rows, themeEditorFlashOverlay(row, width, rowHeight, props.PreviewGeometry.ResultItemRadius, index == 0 && props.FlashToken == "ResultItemActiveBackgroundColor"))
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}
}

func themeEditorPreviewWithTextPanel(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(180), width*0.42)
	resultWidth := max(float32(0), width-panelWidth)
	results := woxwidget.Container{Width: resultWidth, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 4}, Child: themeEditorPreviewResultRows(props, max(float32(0), resultWidth-16), max(float32(0), height-12))}
	layout := previewview.ResolvePreviewLayout(panelWidth, height, true)
	contentWidth := max(float32(0), layout.BodyWidth-24)
	title := themeEditorFlashOverlay(woxwidget.Text{Value: "Theme Preview", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.DraftTheme.PreviewText}, contentWidth, 18, 3, props.FlashToken == "PreviewFontColor")
	body := themeEditorFlashOverlay(woxwidget.TextBlock{Value: "Colors update immediately in this live preview.", Width: contentWidth, Height: 30, MaxLines: 2, Style: woxui.TextStyle{Size: 10}, LineHeight: 15, Color: themeAlpha(props.DraftTheme.PreviewText, 210)}, contentWidth, 30, 3, props.FlashToken == "PreviewFontColor")
	selectionWidth := min(float32(106), contentWidth)
	selection := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "select ", Style: woxui.TextStyle{Size: 9}, Color: props.DraftTheme.PreviewText},
		themeEditorFlashOverlay(woxwidget.Container{Width: 42, Height: 16, Color: props.DraftTheme.SelectionBackground, Child: woxwidget.Text{Value: "preview", Style: woxui.TextStyle{Size: 9}, Color: props.DraftTheme.PreviewText}}, 42, 16, 3, props.FlashToken == "PreviewTextSelectionColor"),
	}}
	previewBody := woxwidget.Container{Width: layout.BodyWidth, Height: layout.BodyHeight, Padding: woxwidget.UniformInsets(12), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		title, body, woxwidget.Align{Width: selectionWidth, Height: 16, Child: selection},
	}}}
	panelBody := previewview.PreviewView(previewview.PreviewProps{
		Width: panelWidth, Height: height, Tags: []previewview.PreviewTag{{Label: "2026-05-26 10:47:08"}, {Label: "2074x679"}, {Label: "702.7 KB"}, {Label: "OCR"}},
		Body: previewBody, Theme: props.DraftTheme, Window: props.Window,
	})
	panelChildren := []woxwidget.StackChild{
		{Child: panelBody},
		{Child: woxwidget.Container{Width: 1, Height: height, Color: props.DraftTheme.PreviewSplit}},
	}
	if props.FlashToken == "PreviewPropertyTitleColor" || props.FlashToken == "PreviewPropertyContentColor" {
		panelChildren = append(panelChildren, woxwidget.StackChild{Left: 14, Bottom: 8, AnchorBottom: true, Child: themeEditorFlashOverlay(woxwidget.Container{Width: layout.InnerWidth, Height: 26}, layout.InnerWidth, 26, 8, true)})
	}
	if props.FlashToken == "PreviewSplitLineColor" {
		panelChildren = append(panelChildren, woxwidget.StackChild{Child: themeEditorFlashOverlay(woxwidget.Container{Width: 3, Height: height, Color: props.DraftTheme.PreviewSplit}, 3, height, 0, true)})
	}
	panel := woxwidget.Stack{Width: panelWidth, Height: height, Children: panelChildren}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{results, panel}}
}

func themeEditorPreviewWithActionPanel(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(230), width*0.38)
	panelHeight := min(float32(170), height-12)
	panelLeft := max(float32(0), width-panelWidth-12)
	panelTop := max(float32(0), height-panelHeight-8)
	actionWidth := panelWidth - 20
	header := themeEditorFlashOverlay(woxwidget.Text{Value: "Actions", Style: woxui.TextStyle{Size: 11}, Color: props.DraftTheme.ActionHeader}, actionWidth, 16, 3, props.FlashToken == "ActionContainerHeaderFontColor")
	divider := woxwidget.Container{Width: actionWidth, Height: ActionDividerHeight, Padding: woxwidget.Insets{Top: 7, Bottom: 8}, Child: woxwidget.Container{Width: actionWidth, Height: 1, Color: props.DraftTheme.PreviewSplit}}
	actionTextWidth := min(float32(150), max(float32(80), actionWidth-18))
	activeText := themeEditorFlashOverlay(woxwidget.Text{Value: props.ToolbarCopyLabel, Style: woxui.TextStyle{Size: 10}, Color: props.DraftTheme.ActionSelectedText}, actionTextWidth, 18, 3, props.FlashToken == "ActionItemActiveFontColor")
	activeRow := woxwidget.Container{Width: actionWidth, Height: 38, Radius: 5, Color: props.DraftTheme.ActionSelected, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: activeText}
	inactiveText := themeEditorFlashOverlay(woxwidget.Text{Value: props.ToolbarMoreLabel, Style: woxui.TextStyle{Size: 10}, Color: props.DraftTheme.ActionText}, actionTextWidth, 18, 3, props.FlashToken == "ActionItemFontColor")
	inactiveRow := woxwidget.Container{Width: actionWidth, Height: 38, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: inactiveText}
	query := woxwidget.Container{Width: actionWidth, Height: 28, Radius: 5, Color: props.DraftTheme.QueryBackground, Padding: woxwidget.Insets{Left: 9, Top: 7}, Child: woxwidget.Text{Value: props.QueryBoxLabel, Style: woxui.TextStyle{Size: 9}, Color: themeAlpha(props.DraftTheme.ActionText, 170)}}
	search := woxwidget.Container{Width: actionWidth, Height: 36, Padding: woxwidget.Insets{Top: 8}, Child: themeEditorFlashOverlay(query, actionWidth, 28, 5, props.FlashToken == "ActionQueryBoxBackgroundColor")}
	panel := woxwidget.Container{Width: panelWidth, Height: panelHeight, Radius: 8, Color: props.DraftTheme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		header,
		divider,
		themeEditorFlashOverlay(activeRow, actionWidth, 38, 5, props.FlashToken == "ActionItemActiveBackgroundColor"),
		inactiveRow,
		search,
	}}}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: themeEditorPreviewResults(props, width, height)},
		{Left: panelLeft, Top: panelTop, Child: themeEditorFlashOverlay(panel, panelWidth, panelHeight, 8, props.FlashToken == "ActionContainerBackgroundColor")},
	}}
}

func themeEditorPreviewToolbar(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	actions := []LauncherToolbarAction{
		{Label: props.ToolbarCopyLabel, HotkeyLabels: []string{"Enter"}},
		{Label: props.ToolbarMoreLabel, HotkeyLabels: []string{"Cmd", "J"}},
	}
	actionWidgets := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		widget, actionWidth := themeEditorToolbarActionView(action, props.DraftTheme, props.Window)
		actionWidgets = append(actionWidgets, themeEditorFlashOverlay(widget, actionWidth, 28, 4, props.FlashToken == "ToolbarFontColor"))
	}
	const actionGap = float32(16)
	padding := props.PreviewGeometry.ToolbarPadding
	body := woxwidget.Container{Width: width, Height: height, Color: props.DraftTheme.ToolbarBackground, Padding: woxwidget.Insets{
		Left: padding.Left, Top: max(float32(0), (height-28)/2), Right: padding.Right, Bottom: max(float32(0), (height-28)/2),
	}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: actionGap, MainAxisAlignment: woxwidget.MainAxisEnd, Children: actionWidgets}}
	toolbar := woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: body},
		{Child: woxwidget.Container{Width: width, Height: 1, Color: themeAlpha(props.DraftTheme.ToolbarText, 26)}},
	}}
	return themeEditorFlashOverlay(toolbar, width, height, 0, props.FlashToken == "ToolbarBackgroundColor")
}

// themeEditorToolbarActionView keeps the preview keycaps on Flutter's lighter toolbar treatment.
func themeEditorToolbarActionView(action LauncherToolbarAction, theme woxcomponent.Theme, window *woxui.Window) (woxwidget.Widget, float32) {
	labelStyle := woxui.TextStyle{Size: woxcomponent.ToolbarFontSize}
	labelMetrics, _ := window.MeasureText(action.Label, labelStyle)
	keycaps, keycapsWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
		Labels: action.HotkeyLabels, Foreground: theme.ToolbarText, Background: themeAlpha(theme.ToolbarText, 6),
		Border: themeAlpha(theme.ToolbarText, 184), FontSize: woxcomponent.TailFontSize, Window: window,
	})
	width := labelMetrics.Size.Width + 8 + keycapsWidth
	return woxwidget.Gesture{Child: woxwidget.Container{Width: width, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelMetrics.Size.Width, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: action.Label, Style: labelStyle, Color: theme.ToolbarText}},
		keycaps,
	}}}}, width
}

func themeEditorControlPane(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-36)
	minimumActionsWidth := min(float32(320), max(float32(0), innerWidth-14))
	actionsWidth := min(float32(370), max(minimumActionsWidth, innerWidth*0.42))
	groupsWidth := max(float32(0), innerWidth-actionsWidth-14)
	groups := themeEditorGroupSelector(props, groupsWidth, 40)
	actions := themeEditorActions(props, actionsWidth, 40)
	tokensTop := float32(62)
	if props.Error != "" {
		tokensTop = 78
	}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: width, Height: 1, Color: themeAlpha(props.Theme.PreviewSplit, 184)}},
		{Left: 18, Top: 12, Child: groups},
		{Left: 18 + groupsWidth + 14, Top: 12, Child: actions},
		{Left: 18, Top: tokensTop, Child: themeEditorTokens(props, innerWidth, max(float32(0), height-tokensTop-6))},
	}
	if props.Error != "" {
		children = append(children, woxwidget.StackChild{Left: 18, Top: 54, Child: woxwidget.Text{Value: props.Error, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ErrorText}})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: children}
}

func themeEditorGroupSelector(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	chips := make([]woxwidget.Widget, 0, len(props.Groups))
	contentWidth := float32(0)
	for index, group := range props.Groups {
		chipWidth := max(float32(54), group.LabelWidth+24)
		if group.LabelWidth <= 0 {
			chipWidth = max(float32(54), float32(utf8.RuneCountInString(group.Label))*7+24)
		}
		background := woxui.Color{}
		border := woxui.Color{}
		foreground := themeAlpha(props.Theme.ResultTitle, 198)
		if index == props.ActiveGroup {
			background = themeAlpha(props.Theme.ActionSelected, 42)
			border = themeAlpha(props.Theme.ActionSelected, 96)
			foreground = props.Theme.ResultTitle
		}
		id := "theme-editor-group-" + strconv.Itoa(index)
		chip := woxwidget.Gesture{ID: id + "-pointer", OnTap: func() {
			if props.OnSelectGroup != nil {
				props.OnSelectGroup(index)
			}
		}, Child: woxwidget.Container{Width: chipWidth, Height: 34, Radius: 6, Color: background, BorderColor: border, BorderWidth: themeBoolFloat(border.A != 0), Padding: woxwidget.Insets{Left: 12, Top: 8}, Child: woxwidget.Text{
			Value: group.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground,
		}}}
		chips = append(chips, woxwidget.Semantics{
			Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: group.Label, Selected: index == props.ActiveGroup,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate && props.OnSelectGroup != nil {
					props.OnSelectGroup(index)
				}
				return nil
			},
			Child: chip,
		})
		contentWidth += chipWidth
	}
	if len(chips) > 1 {
		contentWidth += float32(len(chips)-1) * 8
	}
	return woxwidget.ScrollView{
		Key: "theme-editor-group-scroll", Width: width, Height: height, ContentWidth: max(width, contentWidth), Horizontal: true,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: chips},
	}
}

func themeEditorActions(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	gap := float32(10)
	buttonWidth := max(float32(74), (width-gap*2)/3)
	buttonPadding := woxwidget.Insets{Left: 10, Right: 10}
	saveLabel := props.SaveAsLabel
	if props.Saving {
		saveLabel = props.SavingLabel
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-discard", Label: props.DiscardLabel, Icon: props.DiscardIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Height: height, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving || !props.Dirty, Variant: woxcomponent.ButtonOutline, OnTap: props.OnDiscard, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-overwrite", Label: props.OverwriteLabel, Icon: props.OverwriteIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Height: height, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving || !props.Dirty || !props.CanOverwrite, Variant: woxcomponent.ButtonOutline, OnTap: props.OnOverwrite, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-save-as", Label: saveLabel, Icon: props.SaveAsIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Height: height, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSaveAs, Theme: props.Theme}),
	}}
}

func themeEditorTokens(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	if props.ActiveGroup < 0 || props.ActiveGroup >= len(props.Groups) {
		return woxwidget.Container{Width: width, Height: height}
	}
	group := props.Groups[props.ActiveGroup]
	cards := make([]woxwidget.Widget, 0, len(group.Tokens))
	for _, token := range group.Tokens {
		cards = append(cards, themeEditorTokenCard(props, token, 190, min(float32(44), height)))
	}
	contentWidth := max(width, float32(len(cards))*190+float32(max(0, len(cards)-1))*12)
	return woxwidget.ScrollView{
		Key: woxwidget.Key("theme-editor-token-scroll-" + strconv.Itoa(props.ActiveGroup)), Width: width, Height: height, ContentWidth: contentWidth, Horizontal: true,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: cards},
	}
}

func themeEditorTokenCard(props ThemeEditorSettingsProps, token ThemeEditorColorToken, width, height float32) woxwidget.Widget {
	labelWidth := max(float32(0), width-86)
	hoverBackground := props.Theme.ResultSubtitle
	hoverBackground.A = 26
	locate := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: "theme-editor-locate-" + token.Key, Label: props.LocateLabel + ": " + token.Label,
		Icon: woxwidget.Image{Source: props.LocateIcon, Width: 15, Height: 15}, Width: 26, Height: height - 2, Radius: 4,
		HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: func() {
			if props.OnLocateToken != nil {
				props.OnLocateToken(token.Key)
			}
		},
	})
	label := woxwidget.Clip{Width: labelWidth, Height: height - 2, Child: woxwidget.Align{
		Width: labelWidth, Height: height - 2, Vertical: 0.5,
		Child: woxwidget.Text{Value: token.Label, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultTitle},
	}}
	card := woxwidget.Container{Width: width, Height: height, Radius: 7, BorderColor: themeAlpha(props.Theme.PreviewSplit, 148), BorderWidth: 1, Padding: woxwidget.Insets{Left: 12, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		label,
		locate,
		woxwidget.Container{Width: 8, Height: height - 2},
		woxwidget.Align{Width: 28, Height: height - 2, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Container{Width: 28, Height: 28, Radius: 6, Color: token.Color, BorderColor: themeAlpha(props.Theme.PreviewSplit, 190), BorderWidth: 1}},
	}}}
	activate := func() {
		if props.OnEditToken != nil {
			props.OnEditToken(token.Key)
		}
	}
	id := "theme-editor-token-" + token.Key
	return woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: token.Label,
		Value:   fmt.Sprintf("#%02X%02X%02X%02X", token.Color.R, token.Color.G, token.Color.B, token.Color.A),
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				activate()
			}
			return nil
		},
		Child: woxwidget.Gesture{ID: id + "-pointer", OnTap: activate, Child: card},
	}
}

// themeEditorMicaSurfaceColor mirrors Flutter's translucent app-color tint over the blurred wallpaper.
func themeEditorMicaSurfaceColor(app woxui.Color) woxui.Color {
	if app.A >= 245 {
		return app
	}
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	luminance := 0.2126*linear(app.R) + 0.7152*linear(app.G) + 0.0722*linear(app.B)
	tint := float64(32)
	if luminance >= 0.5 {
		tint = 242
	}
	mix := func(value uint8) uint8 {
		return uint8(math.Round(float64(value)*0.82 + tint*0.18))
	}
	alpha := min(0.86, max(0.64, 0.64+float64(app.A)/255*0.18))
	return woxui.Color{R: mix(app.R), G: mix(app.G), B: mix(app.B), A: uint8(math.Round(alpha * 255))}
}

func themeAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// themeEditorFlashOverlay highlights the exact preview control owned by one color token.
func themeEditorFlashOverlay(child woxwidget.Widget, width, height, radius float32, visible bool) woxwidget.Widget {
	if !visible {
		return child
	}
	fill := themeEditorFlashColor()
	fill.A = 42
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: child},
		{Child: woxwidget.Container{Width: width, Height: height, Radius: radius, Color: fill, BorderColor: themeEditorFlashColor(), BorderWidth: 2}},
	}}
}

func themeEditorFlashColor() woxui.Color {
	return woxui.Color{R: 244, G: 63, B: 94, A: 230}
}

func themeBoolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}
