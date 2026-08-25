package view

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"
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
		{Child: woxwidget.Container{Width: 20, Height: 20, Radius: 10, BorderColor: woxui.Color{A: 128}, BorderWidth: 2}},
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
		woxwidget.Align{Width: 70, Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}},
		woxwidget.Align{Width: trackWidth, Height: 24, Vertical: 0.5, Child: semanticTrack},
		woxwidget.Container{Width: 46, Height: 24, Padding: woxwidget.Insets{Left: 10}, Child: woxwidget.Align{Width: 36, Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: fmt.Sprintf("%.0f%%", normalized*100), Style: woxui.TextStyle{Size: 12}, Color: theme.ResultTitle}}},
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

// ThemeEditorSettingsProps contains the Flutter-aligned live preview and editor actions.
type ThemeEditorSettingsProps struct {
	Width              float32
	Height             float32
	Theme              woxcomponent.Theme
	DraftTheme         woxcomponent.Theme
	Groups             []ThemeEditorColorGroup
	ActiveGroup        int
	Dirty              bool
	Saving             bool
	CanOverwrite       bool
	Error              string
	Wallpaper          *woxui.Image
	WallpaperBlurred   *woxui.Image
	FlashToken         string
	LocateIcon         *woxui.Image
	DiscardIcon        *woxui.Image
	OverwriteIcon      *woxui.Image
	SaveAsIcon         *woxui.Image
	LocateLabel        string
	DiscardLabel       string
	OverwriteLabel     string
	SaveAsLabel        string
	SavingLabel        string
	PreviewResultTitle string
	PreviewResultState string
	Window             *woxui.Window
	QueryBoxLabel      string
	ResultsLabel       string
	ToolbarCopyLabel   string
	ToolbarMoreLabel   string
	Dialog             woxwidget.Widget
	OnSelectGroup      func(int)
	OnEditToken        func(string)
	OnLocateToken      func(string)
	OnDiscard          func()
	OnOverwrite        func()
	OnSaveAs           func()
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
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Image{Source: props.Wallpaper, Width: stageWidth, Height: stageHeight, Radius: 18}})
	} else {
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, Color: woxui.Color{A: 255}}})
	}
	stage.Children = append(stage.Children,
		woxwidget.StackChild{Left: windowLeft, Top: windowTop, Child: themeEditorPreviewWindow(props, windowWidth, windowHeight)},
		woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, BorderColor: themeAlpha(props.Theme.PreviewSplit, 150), BorderWidth: 1}},
	)
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Left: stageLeft, Top: stageTop, Child: stage}}}
}

func themeEditorPreviewWindow(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	selection := woxui.Color{}
	selectionText := props.DraftTheme.QueryText
	if props.ActiveGroup == 1 {
		selection, selectionText = props.DraftTheme.SelectionBackground, props.DraftTheme.SelectionText
	}
	results := []woxcomponent.LauncherDemoResult{
		{Title: props.PreviewResultTitle, Subtitle: props.PreviewResultState, Tail: "Live", Glyph: "⚙", GlyphColor: woxui.Color{R: 139, G: 92, B: 246, A: 255}, Selected: true},
		{Title: props.QueryBoxLabel, Subtitle: "QueryBoxBackgroundColor", Glyph: "⌕", GlyphColor: woxui.Color{R: 14, G: 165, B: 233, A: 255}},
		{Title: props.ResultsLabel, Subtitle: "ResultItemActiveBackgroundColor", Tail: "3 items", Glyph: "≡", GlyphColor: woxui.Color{R: 34, G: 197, B: 94, A: 255}},
	}
	var preview woxwidget.Widget
	resultWidth := float32(0)
	if props.ActiveGroup == 3 {
		resultWidth = width * .58
		preview = themeEditorTextPreviewPanel(props, max(float32(0), width-resultWidth-16), max(float32(0), height-119))
	}
	return woxcomponent.WoxLauncherDemo(woxcomponent.LauncherDemoProps{
		Width: width, Height: height, Backdrop: props.WallpaperBlurred, Background: props.DraftTheme.Background, Theme: props.DraftTheme, Opacity: 1,
		Query: "wox search", QueryParts: []woxcomponent.LauncherDemoQueryPart{
			{Text: "wox ", Color: props.DraftTheme.QueryText}, {Text: "search", Color: selectionText, Background: selection}, {Color: props.DraftTheme.Cursor, Caret: true},
		},
		QueryAccessory: woxwidget.Container{Width: 78, Height: 30, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.ClockGlyph(16, themeAlpha(props.DraftTheme.QueryText, 178)),
			woxwidget.Text{Value: time.Now().Format("15:04"), Style: woxui.TextStyle{Size: woxcomponent.GlanceFontSize}, Color: themeAlpha(props.DraftTheme.QueryText, 178)},
		}}},
		Results: results, ResultWidth: resultWidth, Preview: preview, ShowQuery: true, ShowToolbar: true,
		PrimaryAction: props.ToolbarCopyLabel, ActionCopy: props.ToolbarCopyLabel, ActionMore: props.ToolbarMoreLabel,
		ActionProgress: themeBoolFloat(props.ActiveGroup == 4), HighlightColor: themeEditorFlashColor(),
		HighlightTarget: themeEditorDemoHighlightTarget(props.FlashToken),
	})
}

func themeEditorDemoHighlightTarget(token string) woxcomponent.LauncherDemoHighlightTarget {
	switch token {
	case "AppBackgroundColor":
		return woxcomponent.LauncherDemoHighlightSurface
	case "QueryBoxBackgroundColor":
		return woxcomponent.LauncherDemoHighlightQueryBackground
	case "QueryBoxFontColor":
		return woxcomponent.LauncherDemoHighlightQueryText
	case "QueryBoxCursorColor":
		return woxcomponent.LauncherDemoHighlightQueryCaret
	case "QueryBoxTextSelectionBackgroundColor":
		return woxcomponent.LauncherDemoHighlightQuerySelection
	case "ResultItemTitleColor":
		return woxcomponent.LauncherDemoHighlightResultTitle
	case "ResultItemSubTitleColor":
		return woxcomponent.LauncherDemoHighlightResultSubtitle
	case "ResultItemTailTextColor":
		return woxcomponent.LauncherDemoHighlightResultTail
	case "ResultItemActiveBackgroundColor":
		return woxcomponent.LauncherDemoHighlightSelectedBackground
	case "ResultItemActiveTitleColor":
		return woxcomponent.LauncherDemoHighlightSelectedTitle
	case "ActionContainerBackgroundColor":
		return woxcomponent.LauncherDemoHighlightActionBackground
	case "ActionContainerHeaderFontColor":
		return woxcomponent.LauncherDemoHighlightActionHeader
	case "ActionItemFontColor":
		return woxcomponent.LauncherDemoHighlightActionText
	case "ActionItemActiveBackgroundColor":
		return woxcomponent.LauncherDemoHighlightActionSelectedBackground
	case "ActionItemActiveFontColor":
		return woxcomponent.LauncherDemoHighlightActionSelectedText
	case "ActionQueryBoxBackgroundColor":
		return woxcomponent.LauncherDemoHighlightActionQueryBackground
	case "ToolbarBackgroundColor":
		return woxcomponent.LauncherDemoHighlightToolbarBackground
	case "ToolbarFontColor":
		return woxcomponent.LauncherDemoHighlightToolbarText
	default:
		return woxcomponent.LauncherDemoHighlightNone
	}
}

func themeEditorTextPreviewPanel(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	layout := previewview.ResolvePreviewLayout(width, height, true)
	contentWidth := max(float32(0), layout.BodyWidth-24)
	title := themeEditorFlashOverlay(woxwidget.Text{Value: "Theme Preview", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.DraftTheme.PreviewText}, contentWidth, 18, 3, props.FlashToken == "PreviewFontColor")
	body := themeEditorFlashOverlay(woxwidget.TextBlock{Value: "Colors update immediately in this live preview.", Width: contentWidth, Height: 30, MaxLines: 2, Style: woxui.TextStyle{Size: 10}, LineHeight: 15, Color: themeAlpha(props.DraftTheme.PreviewText, 210)}, contentWidth, 30, 3, props.FlashToken == "PreviewFontColor")
	selection := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "select ", Style: woxui.TextStyle{Size: 9}, Color: props.DraftTheme.PreviewText},
		themeEditorFlashOverlay(woxwidget.Container{Width: 42, Height: 16, Color: props.DraftTheme.SelectionBackground, Child: woxwidget.Text{Value: "preview", Style: woxui.TextStyle{Size: 9}, Color: props.DraftTheme.PreviewText}}, 42, 16, 3, props.FlashToken == "PreviewTextSelectionColor"),
	}}
	previewBody := woxwidget.Container{Width: layout.BodyWidth, Height: layout.BodyHeight, Padding: woxwidget.UniformInsets(12), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		title, body, selection,
	}}}
	panelBody := previewview.PreviewView(previewview.PreviewProps{
		Width: width, Height: height, Tags: []previewview.PreviewTag{{Label: "2026-05-26 10:47:08"}, {Label: "2074x679"}, {Label: "702.7 KB"}, {Label: "OCR"}},
		Body: previewBody, Theme: props.DraftTheme, Window: props.Window,
	})
	children := []woxwidget.StackChild{
		{Child: panelBody},
		{Child: woxwidget.Container{Width: 1, Height: height, Color: props.DraftTheme.PreviewSplit}},
	}
	if props.FlashToken == "PreviewPropertyTitleColor" || props.FlashToken == "PreviewPropertyContentColor" {
		children = append(children, woxwidget.StackChild{Left: 14, Bottom: 8, AnchorBottom: true, Child: themeEditorFlashOverlay(woxwidget.Container{Width: layout.InnerWidth, Height: 26}, layout.InnerWidth, 26, 8, true)})
	}
	if props.FlashToken == "PreviewSplitLineColor" {
		children = append(children, woxwidget.StackChild{Child: themeEditorFlashOverlay(woxwidget.Container{Width: 3, Height: height, Color: props.DraftTheme.PreviewSplit}, 3, height, 0, true)})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: children}
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
		chip := themeEditorGroupChip(themeEditorGroupChipProps{
			ID: id, Label: group.Label, Width: chipWidth, Height: 34, Background: background, HoverBackground: themeAlpha(props.Theme.ResultTitle, 25),
			BorderColor: border, BorderWidth: themeBoolFloat(border.A != 0), Foreground: foreground, Selected: index == props.ActiveGroup, OnTap: func() {
				if props.OnSelectGroup != nil {
					props.OnSelectGroup(index)
				}
			},
		})
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

// themeEditorGroupChipProps contains one theme editor group tab and its hover colors.
type themeEditorGroupChipProps struct {
	ID              string
	Label           string
	Width           float32
	Height          float32
	Background      woxui.Color
	HoverBackground woxui.Color
	BorderColor     woxui.Color
	BorderWidth     float32
	Foreground      woxui.Color
	Selected        bool
	OnTap           func()
}

type themeEditorGroupChipState struct {
	hovered bool
}

// themeEditorGroupChip builds a selectable group tab with retained pointer hover state.
func themeEditorGroupChip(props themeEditorGroupChipProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*themeEditorGroupChipState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &themeEditorGroupChipState{} },
	}
}

func (s *themeEditorGroupChipState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *themeEditorGroupChipState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *themeEditorGroupChipState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(themeEditorGroupChipProps)
	background := props.Background
	if s.hovered && !props.Selected {
		background = props.HoverBackground
	}
	return woxwidget.Gesture{ID: props.ID + "-pointer", OnTap: props.OnTap, OnHoverAt: func(inside bool, _ woxui.Rect) {
		if inside != s.hovered {
			context.SetState(func() { s.hovered = inside })
		}
	}, Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 6, Color: background, BorderColor: props.BorderColor, BorderWidth: props.BorderWidth,
		Padding: woxwidget.Insets{Left: 12}, Child: woxwidget.Align{Height: props.Height, Vertical: 0.5, Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Foreground}},
	}}
}

func (s *themeEditorGroupChipState) Dispose() {}

func themeEditorActions(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	gap := float32(10)
	buttonWidth := max(float32(74), (width-gap*2)/3)
	buttonPadding := woxwidget.Insets{Left: 10, Right: 10}
	saveLabel := props.SaveAsLabel
	if props.Saving {
		saveLabel = props.SavingLabel
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-discard", Label: props.DiscardLabel, Icon: props.DiscardIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving || !props.Dirty, Variant: woxcomponent.ButtonOutline, OnTap: props.OnDiscard, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-overwrite", Label: props.OverwriteLabel, Icon: props.OverwriteIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving || !props.Dirty || !props.CanOverwrite, Variant: woxcomponent.ButtonOutline, OnTap: props.OnOverwrite, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-save-as", Label: saveLabel, Icon: props.SaveAsIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Radius: 5, Padding: buttonPadding, FontSize: 11, Disabled: props.Saving, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSaveAs, Theme: props.Theme}),
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
