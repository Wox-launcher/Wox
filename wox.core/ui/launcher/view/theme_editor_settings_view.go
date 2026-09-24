package view

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const themeEditorColorWheelSize = float32(220)

// Only the token dialog shows the wheel, but building it at package init kept its 440x440 raster
// on the Go heap for every session, including the ones that never open the theme editor.
var themeEditorColorWheelImage = sync.OnceValue(buildThemeEditorColorWheelImage)

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
	Theme              woxcomponent.ControlTheme
	OnHueSaturation    func(hue, saturation float64)
	OnBrightnessChange func(value float64)
	OnOpacityChange    func(value float64)
}

// ThemeEditorColorPicker mirrors Flutter's wheel, swatch, CSS input, and sliders.
func ThemeEditorColorPicker(props ThemeEditorColorPickerProps) woxwidget.Widget {
	const contentWidth = float32(360)
	wheel := themeEditorColorWheel(props)
	colorRow := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 48, Height: 36, Radius: 6, Color: props.Color, BorderColor: themeAlpha(props.Theme.Border, 200), BorderWidth: 1},
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
				{Child: woxwidget.Image{Source: themeEditorColorWheelImage(), Width: themeEditorColorWheelSize, Height: themeEditorColorWheelSize}},
				{Left: thumbX - 10, Top: thumbY - 10, Child: thumb},
			}},
		},
	}
}

// themeEditorColorSlider keeps pointer and accessibility value changes on the same normalized path.
func themeEditorColorSlider(id, label string, value float64, theme woxcomponent.ControlTheme, onChanged func(float64)) woxwidget.Widget {
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
			{Top: 7, Child: woxwidget.Container{Width: trackWidth, Height: 4, Radius: 2, Color: themeAlpha(theme.Border, 150)}},
			{Top: 7, Child: woxwidget.Container{Width: activeWidth, Height: 4, Radius: 2, Color: theme.SelectionBackground}},
			{Left: max(float32(0), min(trackWidth-thumbSize, activeWidth-thumbSize/2)), Child: woxwidget.Container{Width: thumbSize, Height: thumbSize, Radius: thumbSize / 2, Color: theme.TextSecondary}},
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
		Child: woxwidget.Focusable{Key: woxwidget.Key(id + "-focus"), FocusRingColor: theme.Focus, OnKey: func(event woxui.KeyEvent) bool {
			if !event.Down || onChanged == nil {
				return false
			}
			next := normalized
			switch event.Key {
			case woxui.KeyArrowLeft, woxui.KeyArrowDown:
				next -= 0.01
			case woxui.KeyArrowRight, woxui.KeyArrowUp:
				next += 0.01
			case woxui.KeyHome:
				next = 0
			case woxui.KeyEnd:
				next = 1
			default:
				return false
			}
			onChanged(min(float64(1), max(float64(0), next)))
			return true
		}, Child: track},
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Align{Width: 70, Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: theme.Scaled(12)}, Color: theme.TextSecondary}},
		woxwidget.Align{Width: trackWidth, Height: 24, Vertical: 0.5, Child: semanticTrack},
		woxwidget.Container{Width: 46, Height: 24, Padding: woxwidget.Insets{Left: 10}, Child: woxwidget.Align{Width: 36, Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: fmt.Sprintf("%.0f%%", normalized*100), Style: woxui.TextStyle{Size: theme.Scaled(12)}, Color: theme.Text}}},
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
	Key       string
	Label     string
	Color     woxui.Color
	Numeric   bool
	Optional  bool
	Value     string
	Effective string
	Subgroup  string
	Error     string
}

// ThemeEditorColorGroup contains one collapsible inspector section.
type ThemeEditorColorGroup struct {
	Label  string
	Tokens []ThemeEditorColorToken
}

// ThemeEditorSettingsProps carries an immutable draft and stable editing callbacks.
type ThemeEditorSettingsProps struct {
	ModeLabel          string
	AILabel            string
	AIIcon             *woxui.Image
	OnSelectAI         func(bool)
	AIAssistant        woxwidget.Widget
	AIExpanded         bool
	Key                string
	Title              string
	DefaultLabel       string
	ResetLabel         string
	LinkPaddingLabel   string
	PaddingLabel       string
	NoPropertiesLabel  string
	ChromeHelp         string
	ShowChromeHelp     bool
	OpacityLabel       string
	OnChangeToken      func(string, string)
	OnChangeTokens     func(map[string]string)
	Width              float32
	Height             float32
	Theme              woxcomponent.ControlTheme
	DraftTheme         woxcomponent.Theme
	Geometry           *woxcomponent.LauncherDemoGeometry
	Groups             []ThemeEditorColorGroup
	ActiveGroup        int
	Dirty              bool
	AIBusy             bool
	Saving             bool
	CanOverwrite       bool
	Error              string
	Wallpaper          *woxui.Image
	WallpaperBlurred   *woxui.Image
	FlashToken         string
	DialogToken        string
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
	PropertyLabel      string
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

// ThemeEditorSettingsView keeps inspector interaction state below the application controller.
func ThemeEditorSettingsView(props ThemeEditorSettingsProps) woxwidget.Widget {
	return woxwidget.Stateful{Key: woxwidget.Key("theme-editor-" + props.Key), Type: (*themeEditorSettingsState)(nil), Widget: props,
		CreateState: func() woxwidget.State {
			return &themeEditorSettingsState{expanded: map[int]bool{0: true}, linked: map[string]bool{}}
		}}
}

// Expansion and filtering stay local; opening a section selects its preview scene.
type themeEditorSettingsState struct {
	expanded       map[int]bool
	linked         map[string]bool
	scrollOffset   float32
	scrollRevision int
}

func (s *themeEditorSettingsState) InitState(_ woxwidget.StateContext, _ any)          {}
func (s *themeEditorSettingsState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}
func (s *themeEditorSettingsState) Dispose()                                           {}

// Build reserves a full-height inspector, falling back to stacked panes on narrow windows.
func (s *themeEditorSettingsState) Build(ctx woxwidget.StateContext, input any) woxwidget.Widget {
	props := input.(ThemeEditorSettingsProps)
	headerHeight := float32(44)
	header := woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: props.Theme.Scaled(22), Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text}
	bodyHeight := max(float32(0), props.Height-headerHeight)
	if props.Error != "" {
		bodyHeight = max(float32(0), bodyHeight-40)
	}
	var body woxwidget.Widget
	if props.Width >= 760 {
		inspectorWidth := float32(340)
		previewWidth := max(float32(0), props.Width-inspectorWidth-16)
		left := []woxwidget.Widget{woxwidget.Container{Width: previewWidth, Height: headerHeight, Child: header}}
		if props.Error != "" {
			left = append(left, woxwidget.TextBlock{Value: props.Error, Width: previewWidth, Height: 40, MaxLines: 2, LineHeight: 20, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.Error})
		}
		left = append(left, themeEditorLivePreview(props, previewWidth, bodyHeight))
		body = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Vertical, Children: left}, s.editorPane(ctx, props, inspectorWidth, props.Height),
		}}
	} else {
		previewHeight := min(float32(300), float32(math.Floor(float64(bodyHeight*.35))))
		body = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
			themeEditorLivePreview(props, props.Width, previewHeight), s.editorPane(ctx, props, props.Width, max(float32(0), bodyHeight-previewHeight-12)),
		}}
	}
	children := []woxwidget.Widget{woxwidget.Container{Width: props.Width, Height: headerHeight, Child: header}}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{Value: props.Error, Width: props.Width, Height: 40, MaxLines: 2, LineHeight: 20, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.Error})
	}
	children = append(children, body)
	if props.Width >= 760 {
		children = []woxwidget.Widget{body}
	}
	var base woxwidget.Widget = woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
	if props.Dialog != nil {
		base = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: base}, {Child: props.Dialog}}}
	}
	return base
}

// inspector uses one vertical scroll region so adding properties does not squeeze the preview.
func (s *themeEditorSettingsState) inspector(ctx woxwidget.StateContext, props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-16)
	rows := []woxwidget.Widget{}
	var pinned woxwidget.Widget
	for index, group := range props.Groups {
		tokens := []ThemeEditorColorToken{}
		for _, token := range group.Tokens {
			if strings.HasPrefix(token.Key, "Base") {
				continue
			}
			tokens = append(tokens, token)
		}
		if len(tokens) == 0 {
			continue
		}
		expanded := s.expanded[index]
		chevron := woxcomponent.ChevronGlyph(16, props.Theme.TextSecondary, expanded).(woxwidget.Image)
		headerProps := woxcomponent.ButtonProps{ID: "theme-editor-group-" + strconv.Itoa(index), Label: group.Label, Icon: chevron.Source, Width: contentWidth, Padding: woxwidget.Insets{Top: 7, Bottom: 7}, AlignLeading: true, Variant: woxcomponent.ButtonText, Theme: props.Theme, OnTap: func() {
			ctx.SetState(func() {
				s.expanded = map[int]bool{index: !expanded}
				s.scrollOffset = 0
				s.scrollRevision++
			})
			if !expanded && props.OnSelectGroup != nil {
				props.OnSelectGroup(index)
			}
		}}
		// With one expanded section, every preceding row is a 32-unit header plus its gap.
		if expanded && s.scrollOffset > float32(len(rows))*48 {
			headerProps.ID = "theme-editor-pinned-group-" + strconv.Itoa(index)
			pinned = woxcomponent.WoxButton(headerProps)
			headerProps.ID = "theme-editor-group-" + strconv.Itoa(index)
		}
		rows = append(rows, woxcomponent.WoxButton(headerProps))
		if !expanded {
			continue
		}
		// Keep related properties together even when geometry was appended after colors.
		sections := []string{}
		buckets := map[string][]ThemeEditorColorToken{}
		for _, token := range tokens {
			if _, exists := buckets[token.Subgroup]; !exists {
				sections = append(sections, token.Subgroup)
			}
			buckets[token.Subgroup] = append(buckets[token.Subgroup], token)
		}
		tokens = nil
		for _, section := range sections {
			tokens = append(tokens, buckets[section]...)
		}
		previousSection := ""
		for i := 0; i < len(tokens); i++ {
			token := tokens[i]
			token.Label = themeEditorPropertyLabel(token.Label, group.Label, token.Subgroup)
			if token.Subgroup != "" && token.Subgroup != previousSection {
				rows = append(rows, woxwidget.Container{Width: contentWidth, Padding: woxwidget.Insets{Top: 12, Bottom: 4}, Child: woxwidget.Text{Value: token.Subgroup, Style: woxui.TextStyle{Size: props.Theme.Scaled(13), Weight: woxui.FontWeightSemibold}, Color: props.Theme.TextSecondary}})
				previousSection = token.Subgroup
				if strings.HasPrefix(token.Key, "AppBorder") && props.ShowChromeHelp {
					rows = append(rows, woxwidget.TextBlock{Value: props.ChromeHelp, Width: contentWidth, LineHeight: 18, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.TextSecondary})
				}
			}
			rowProps := props
			if strings.HasSuffix(token.Key, "PaddingLeft") && i+3 < len(tokens) {
				prefix := strings.TrimSuffix(token.Key, "Left")
				sides := tokens[i : i+4]
				if sides[1].Key == prefix+"Top" && sides[2].Key == prefix+"Right" && sides[3].Key == prefix+"Bottom" {
					linked, chosen := s.linked[prefix]
					if !chosen {
						linked = true
						for _, side := range sides {
							linked = linked && side.Value == token.Value && side.Effective == token.Effective
						}
					}
					rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
						woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{ID: "theme-editor-link-" + prefix, Label: props.LinkPaddingLabel, Value: linked, Theme: props.Theme, OnChange: func(value bool) {
							ctx.SetState(func() {
								if s.linked == nil {
									s.linked = map[string]bool{}
								}
								s.linked[prefix] = value
							})
						}}),
						woxwidget.Text{Value: props.LinkPaddingLabel, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.TextSecondary},
					}})
					if linked {
						token.Label = props.PaddingLabel
						rowProps.OnChangeToken = func(_ string, value string) {
							if props.OnChangeTokens != nil {
								props.OnChangeTokens(map[string]string{prefix + "Left": value, prefix + "Top": value, prefix + "Right": value, prefix + "Bottom": value})
							}
						}
						i += 3
					}
				}
			}
			rows = append(rows, themeEditorPropertyRow(rowProps, token, contentWidth))
		}
		rows = append(rows, woxwidget.Container{Height: 1, Width: contentWidth, Color: props.Theme.Border})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.TextBlock{Value: props.NoPropertiesLabel, Width: contentWidth, LineHeight: 20, Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.TextSecondary})
	}
	viewportHeight := max(float32(0), height)
	var scroll woxwidget.Widget = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: woxwidget.Key("theme-editor-properties-" + strconv.Itoa(s.scrollRevision)), Width: width, Height: viewportHeight, ContentWidth: width, Theme: props.Theme,
		OnOffsetChanged: func(offset float32) { ctx.SetState(func() { s.scrollOffset = offset }) },
		Content:         woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: rows}})
	layers := []woxwidget.StackChild{{Child: scroll}}
	if pinned != nil {
		layers = append(layers, woxwidget.StackChild{Child: woxwidget.Container{Width: contentWidth, Height: 32, Color: woxui.Color{R: 36, G: 38, B: 42, A: 255}, Child: pinned}})
	}
	scroll = woxwidget.Stack{Width: width, Height: viewportHeight, Children: layers}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{scroll}}
}

// themeEditorPropertyLabel removes scope already supplied by visible section headings.
func themeEditorPropertyLabel(label, group, subgroup string) string {
	for _, scope := range []string{subgroup, group} {
		if scope != "" && strings.HasPrefix(label, scope) {
			shortened := strings.TrimLeft(strings.TrimPrefix(label, scope), " ·")
			if shortened != "" {
				label = shortened
			}
		}
	}
	runes := []rune(label)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

// themeEditorPropertyRow keeps numeric edits inline and colors in the existing live picker.
func themeEditorPropertyRow(props ThemeEditorSettingsProps, token ThemeEditorColorToken, width float32) woxwidget.Widget {
	label := token.Label
	if token.Optional && token.Value == "" {
		label += " · " + props.DefaultLabel
	}
	reset := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "theme-editor-reset-" + token.Key, Label: props.ResetLabel + ": " + token.Label,
		Icon: woxwidget.Image{Source: props.DiscardIcon, Width: 16, Height: 16}, Width: 32, Height: 32, Disabled: props.Saving || props.AIBusy || !token.Optional || token.Value == "", FocusRingColor: props.Theme.Focus, HoverBackground: themeAlpha(props.Theme.Text, 25),
		OnTap: func() {
			if props.OnChangeToken != nil {
				props.OnChangeToken(token.Key, "")
			}
		}})
	controls := []woxwidget.Widget{}
	if token.Numeric {
		locateWidth := float32(0)
		if strings.HasSuffix(token.Key, "Radius") {
			locateWidth = 36
		}
		hint := token.Effective
		if hint == "" {
			hint = props.DefaultLabel
		}
		border := props.Theme.Border
		if value := strings.TrimSpace(token.Value); value != "" {
			if number, err := strconv.Atoi(value); err != nil || number < 0 {
				border = props.Theme.Error
			}
		}
		field := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{ID: "theme-editor-value-" + token.Key, Label: token.Label, Width: max(float32(0), width-108-locateWidth), Height: 32, Radius: 4, Padding: woxwidget.Insets{Left: 8, Right: 8, Top: 6, Bottom: 6}, Value: token.Value, Hint: hint, BorderColor: border, Window: props.Window, Theme: props.Theme, Disabled: props.Saving || props.AIBusy,
			OnFocusChange: func(focused bool) {
				if focused && props.OnSelectGroup != nil {
					for i, g := range props.Groups {
						for _, t := range g.Tokens {
							if t.Key == token.Key {
								props.OnSelectGroup(i)
								return
							}
						}
					}
				}
			},
			OnChanged: func(value string) {
				if props.OnChangeToken != nil {
					props.OnChangeToken(token.Key, value)
				}
			}})
		controls = append(controls, field)
		for _, delta := range []int{-1, 1} {
			value := token.Value
			if value == "" {
				value = token.Effective
			}
			number, err := strconv.Atoi(value)
			if value == "" {
				number = 0
				err = nil
			}
			caption := "−"
			if delta > 0 {
				caption = "+"
			}
			controls = append(controls, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-step-" + token.Key + caption, Label: caption, Width: 32, Disabled: props.Saving || props.AIBusy || err != nil || (delta < 0 && number == 0) || (delta > 0 && number == int(^uint(0)>>1)), Theme: props.Theme,
				OnTap: func() {
					if props.OnChangeToken != nil {
						props.OnChangeToken(token.Key, strconv.Itoa(max(0, number+delta)))
					}
				}}))
		}
	} else {
		controls = append(controls, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "theme-editor-swatch-" + token.Key, Label: token.Label, Width: 32, Height: 32, Disabled: props.Saving || props.AIBusy, FocusRingColor: props.Theme.Focus, Icon: woxwidget.Container{Width: 32, Height: 32, Radius: 4, Color: token.Color, BorderWidth: 1, BorderColor: props.Theme.Border}, OnTap: func() {
			if props.OnEditToken != nil {
				props.OnEditToken(token.Key)
			}
		}}),
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-token-" + token.Key, Label: token.Effective, Width: max(float32(0), width-108), Disabled: props.Saving || props.AIBusy, Theme: props.Theme, OnTap: func() {
				if props.OnEditToken != nil {
					props.OnEditToken(token.Key)
				}
			}}),
			woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "theme-editor-locate-" + token.Key, Label: props.LocateLabel + ": " + token.Label, Icon: woxwidget.Image{Source: props.LocateIcon, Width: 16, Height: 16}, Width: 32, Height: 32, Disabled: props.Saving || props.AIBusy, FocusRingColor: props.Theme.Focus, HoverBackground: themeAlpha(props.Theme.Text, 25), OnTap: func() {
				if props.OnLocateToken != nil {
					props.OnLocateToken(token.Key)
				}
			}}))
	}
	if token.Numeric && strings.HasSuffix(token.Key, "Radius") {
		controls = append(controls, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "theme-editor-locate-" + token.Key, Label: props.LocateLabel + ": " + token.Label, Icon: woxwidget.Image{Source: props.LocateIcon, Width: 16, Height: 16}, Width: 32, Height: 32, Disabled: props.Saving || props.AIBusy, OnTap: func() {
			if props.OnLocateToken != nil {
				props.OnLocateToken(token.Key)
			}
		}}))
	}
	controls = append(controls, reset)
	children := []woxwidget.Widget{woxwidget.TextBlock{Value: label, Width: width, LineHeight: 18, Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.Text}, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: controls}}
	if token.Error != "" {
		children = append(children, woxwidget.TextBlock{Value: token.Error, Width: width, LineHeight: 18, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.Error})
	}
	return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: children}}
}

func themeEditorLivePreview(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	stageWidth := min(float32(900), max(float32(0), width))
	stageHeight := min(float32(420), max(float32(0), height-20))
	windowWidth := min(float32(780), max(float32(0), stageWidth-24))
	overlaySpace := float32(0)
	if themeEditorOverlayFocused(props) {
		overlaySpace = 78
	}
	windowHeight := min(float32(360), max(float32(0), stageHeight-24-overlaySpace))
	demo := woxwidget.Widget(themeEditorPreviewWindow(props, windowWidth, windowHeight))
	if sample, ok := themeEditorOverlaySample(props, min(windowWidth, 320)); ok {
		demo = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{demo, sample}}
	}

	stageColor := props.Theme.InputBackground
	stage := woxwidget.Stack{Width: stageWidth, Height: stageHeight, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, Color: stageColor}},
	}}
	if props.Wallpaper != nil {
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Image{Source: props.Wallpaper, Width: stageWidth, Height: stageHeight, Radius: 18}})
	} else {
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, Color: woxui.Color{A: 255}}})
	}
	stage.Children = append(stage.Children,
		woxwidget.StackChild{Child: woxwidget.Align{Width: stageWidth, Height: stageHeight, Horizontal: 0.5, Vertical: 0.5, Child: demo}},
		woxwidget.StackChild{Child: woxwidget.Container{Width: stageWidth, Height: stageHeight, Radius: 18, BorderColor: themeAlpha(props.Theme.Border, 150), BorderWidth: 1}},
	)
	return woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: stage}
}

// themeEditorOverlayFocused is true while the Overlay group is open, or one of its colors is located.
func themeEditorOverlayFocused(props ThemeEditorSettingsProps) bool {
	if props.ActiveGroup >= 0 && props.ActiveGroup < len(props.Groups) {
		for _, token := range props.Groups[props.ActiveGroup].Tokens {
			if token.Key == "OverlayBackgroundColor" || token.Key == "OverlayFontColor" {
				return true
			}
		}
	}
	for _, token := range []string{props.FlashToken, props.DialogToken} {
		if token == "OverlayBackgroundColor" || token == "OverlayFontColor" {
			return true
		}
	}
	return false
}

// themeEditorOverlaySample shows the desktop overlay under the launcher. It is not part of the launcher frame.
func themeEditorOverlaySample(props ThemeEditorSettingsProps, width float32) (woxwidget.Widget, bool) {
	if !themeEditorOverlayFocused(props) {
		return nil, false
	}
	backgroundToken := props.FlashToken == "OverlayBackgroundColor" || props.DialogToken == "OverlayBackgroundColor"
	textToken := props.FlashToken == "OverlayFontColor" || props.DialogToken == "OverlayFontColor"
	background := props.DraftTheme.Background
	if props.DraftTheme.OverlayBackground != nil {
		background = *props.DraftTheme.OverlayBackground
	}
	foreground := props.DraftTheme.QueryText
	if props.DraftTheme.OverlayText != nil {
		foreground = *props.DraftTheme.OverlayText
	}
	cardWidth := max(float32(180), width)
	const cardHeight float32 = 56
	text := themeEditorFlashOverlay(woxwidget.Text{Value: "Overlay", Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: foreground}, 72, 20, 3, textToken)
	card := woxwidget.Container{
		Width: cardWidth, Height: cardHeight, Radius: 12, Color: background,
		Child: woxwidget.Align{Width: cardWidth, Height: cardHeight, Horizontal: 0.5, Vertical: 0.5, Child: text},
	}
	return themeEditorFlashOverlay(card, cardWidth, cardHeight, 12, backgroundToken), true
}

func themeEditorPreviewWindow(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	selection := woxui.Color{}
	selectionText := props.DraftTheme.QueryText
	if props.ActiveGroup == 1 {
		selection, selectionText = props.DraftTheme.SelectionBackground, props.DraftTheme.SelectionText
	}
	results := []woxcomponent.LauncherDemoResult{
		{Title: props.PreviewResultTitle, Subtitle: props.PreviewResultState, Tail: "Live", Glyph: "⚙", GlyphColor: woxui.Color{R: 139, G: 92, B: 246, A: 255}, Selected: true},
		{Hovered: props.FlashToken == "ResultItemHoverBackgroundColor", Title: props.QueryBoxLabel, Subtitle: "QueryBoxBackgroundColor", Glyph: "⌕", GlyphColor: woxui.Color{R: 14, G: 165, B: 233, A: 255}},
		{Title: props.ResultsLabel, Subtitle: "ResultItemActiveBackgroundColor", Tail: "3 items", Glyph: "≡", GlyphColor: woxui.Color{R: 34, G: 197, B: 94, A: 255}},
	}
	var preview woxwidget.Widget
	resultWidth := float32(0)
	if props.ActiveGroup == 3 {
		for i := range results {
			results[i].Tail = ""
		}
		resultWidth = width * .4
		// Match the demo's 77-unit preview top and 50-unit footer clearance.
		previewHeight := height - 127
		previewWidth := width - resultWidth - 16
		if props.Geometry != nil {
			bounds := woxcomponent.LauncherContentBounds(width, height, props.DraftTheme.AppContentInset, props.DraftTheme.Surfaces)
			resultWidth = bounds.Width * .4
			previewWidth = bounds.Width - resultWidth - 16
			previewHeight = bounds.Height - props.Geometry.AppPadding.Top - 55 - props.Geometry.ResultPadding.Top - 4 - 40 - props.Geometry.AppPadding.Bottom
		}
		preview = themeEditorTextPreviewPanel(props, max(float32(0), previewWidth), max(float32(0), previewHeight))
	}
	backdrop := props.WallpaperBlurred
	if props.DraftTheme.AppWindowChrome {
		backdrop = props.Wallpaper
	}
	return woxcomponent.WoxLauncherDemo(woxcomponent.LauncherDemoProps{
		Width: width, Height: height, Backdrop: backdrop, Background: props.DraftTheme.Background, Theme: props.DraftTheme, Opacity: 1,
		Geometry: props.Geometry, Window: props.Window,
		Query: "wox search", QueryParts: []woxcomponent.LauncherDemoQueryPart{
			{Text: "wox ", Color: props.DraftTheme.QueryText}, {Text: "search", Color: selectionText, Background: selection, Selected: props.ActiveGroup == 1}, {Color: props.DraftTheme.Cursor, Caret: true},
		},
		QueryAccessory: themeEditorQueryAccessories(props),
		Results:        results, ResultWidth: resultWidth, Preview: preview, ShowQuery: true, ShowToolbar: true,
		PrimaryAction: props.ToolbarCopyLabel, ActionCopy: props.ToolbarCopyLabel, ActionMore: props.ToolbarMoreLabel,
		ActionProgress: themeBoolFloat(props.ActiveGroup == 4), HighlightColor: themeEditorFlashColor(),
		HighlightTarget:  themeEditorDemoHighlightTarget(props.FlashToken),
		HighlightCorners: strings.HasSuffix(props.FlashToken, "Radius"),
	})
}

func themeEditorDemoHighlightTarget(token string) woxcomponent.LauncherDemoHighlightTarget {
	switch token {
	case "ScrollbarBorderRadius":
		return woxcomponent.LauncherDemoHighlightScrollbar
	case "ResultItemActiveIndicatorBorderRadius":
		return woxcomponent.LauncherDemoHighlightIndicator
	case "AppBorderRadius":
		return woxcomponent.LauncherDemoHighlightSurface
	case "AppContentBorderRadius":
		return woxcomponent.LauncherDemoHighlightContent
	case "QueryBoxBorderRadius":
		return woxcomponent.LauncherDemoHighlightQueryBackground
	case "ResultItemBorderRadius":
		return woxcomponent.LauncherDemoHighlightSelectedBackground
	case "ActionContainerBorderRadius":
		return woxcomponent.LauncherDemoHighlightActionBackground
	case "ActionItemBorderRadius":
		return woxcomponent.LauncherDemoHighlightActionSelectedBackground
	case "ActionQueryBoxBorderRadius":
		return woxcomponent.LauncherDemoHighlightActionQueryBackground
	case "ResultItemHoverBackgroundColor":
		return woxcomponent.LauncherDemoHighlightResultHover
	case "ActionContainerDividerColor":
		return woxcomponent.LauncherDemoHighlightActionDivider
	case "ActionItemHotkeyFontColor", "ActionItemHotkeyBackgroundColor", "ActionItemHotkeyBorderColor":
		return woxcomponent.LauncherDemoHighlightActionHotkey
	case "ActionItemActiveHotkeyFontColor", "ActionItemActiveHotkeyBackgroundColor", "ActionItemActiveHotkeyBorderColor":
		return woxcomponent.LauncherDemoHighlightActionActiveHotkey
	case "ToolbarPrimaryFontColor":
		return woxcomponent.LauncherDemoHighlightToolbarPrimaryText
	case "ToolbarPrimaryHotkeyFontColor", "ToolbarPrimaryHotkeyBackgroundColor", "ToolbarPrimaryHotkeyBorderColor":
		return woxcomponent.LauncherDemoHighlightToolbarPrimaryHotkey
	case "ToolbarHotkeyFontColor", "ToolbarHotkeyBackgroundColor", "ToolbarHotkeyBorderColor":
		return woxcomponent.LauncherDemoHighlightHotkey
	case "AppBackgroundColor", "BaseBackgroundColor", "AppBorderColor":
		return woxcomponent.LauncherDemoHighlightSurface
	case "AppContentBackgroundColor":
		return woxcomponent.LauncherDemoHighlightContent
	case "QueryBoxBackgroundColor", "QueryBoxBorderBottomColor":
		return woxcomponent.LauncherDemoHighlightQueryBackground
	case "QueryBoxFontColor", "BaseTextColor":
		return woxcomponent.LauncherDemoHighlightQueryText
	case "QueryBoxCursorColor", "BaseAccentColor":
		return woxcomponent.LauncherDemoHighlightQueryCaret
	case "QueryBoxTextSelectionBackgroundColor":
		return woxcomponent.LauncherDemoHighlightQuerySelection
	case "ResultItemTitleColor":
		return woxcomponent.LauncherDemoHighlightResultTitle
	case "ResultItemSubTitleColor":
		return woxcomponent.LauncherDemoHighlightResultSubtitle
	case "ResultItemTailTextColor":
		return woxcomponent.LauncherDemoHighlightResultTail
	case "ResultItemActiveBackgroundColor", "ResultItemActiveIndicatorColor":
		return woxcomponent.LauncherDemoHighlightSelectedBackground
	case "ResultItemActiveTitleColor":
		return woxcomponent.LauncherDemoHighlightSelectedTitle
	case "ResultItemActiveTailTextColor":
		return woxcomponent.LauncherDemoHighlightSelectedTail
	case "ActionContainerBackgroundColor", "ActionContainerBorderColor":
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
	case "ToolbarBackgroundColor", "ToolbarBorderColor":
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
	selectionColor := props.DraftTheme.SelectionBackground
	for _, group := range props.Groups {
		for _, token := range group.Tokens {
			if token.Key == "PreviewTextSelectionColor" {
				selectionColor = token.Color
			}
		}
	}
	title := themeEditorFlashOverlay(woxwidget.Text{Value: "Theme Preview", Style: woxui.TextStyle{Size: props.Theme.Scaled(13), Weight: woxui.FontWeightSemibold}, Color: props.DraftTheme.PreviewText}, contentWidth, 18, 3, props.FlashToken == "PreviewFontColor")
	body := themeEditorFlashOverlay(woxwidget.TextBlock{Value: "Colors update immediately in this live preview.", Width: contentWidth, Height: 30, MaxLines: 2, Style: woxui.TextStyle{Size: props.Theme.Scaled(10)}, LineHeight: props.Theme.Scaled(15), Color: themeAlpha(props.DraftTheme.PreviewText, 210)}, contentWidth, 30, 3, props.FlashToken == "PreviewFontColor")
	selection := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "select ", Style: woxui.TextStyle{Size: props.Theme.Scaled(9)}, Color: props.DraftTheme.PreviewText},
		themeEditorFlashOverlay(woxwidget.Container{Width: 42, Height: 16, Color: selectionColor, Child: woxwidget.Text{Value: "preview", Style: woxui.TextStyle{Size: props.Theme.Scaled(9)}, Color: props.DraftTheme.PreviewText}}, 42, 16, 3, props.FlashToken == "PreviewTextSelectionColor"),
	}}
	// Properties describe body content; footer metadata uses PreviewTag colors in v2.
	propertyToken := props.FlashToken == "PreviewPropertyTitleColor" || props.FlashToken == "PreviewPropertyContentColor"
	if propertyToken && props.DraftTheme.PreviewTagFontColor != nil {
		labelWidth := min(float32(88), contentWidth/2)
		valueWidth := max(float32(0), contentWidth-labelWidth-10)
		selection = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			themeEditorFlashOverlay(woxwidget.Text{Value: props.PropertyLabel, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.DraftTheme.PreviewPropertyTitle}, labelWidth, 18, 3, props.FlashToken == "PreviewPropertyTitleColor"),
			themeEditorFlashOverlay(woxwidget.Text{Value: "702.7 KB", Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.DraftTheme.PreviewPropertyContent}, valueWidth, 18, 3, props.FlashToken == "PreviewPropertyContentColor"),
		}}
	}
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
	if props.FlashToken == "PreviewBackgroundColor" || props.FlashToken == "PreviewBorderColor" {
		children = append(children, woxwidget.StackChild{Left: 14, Top: 12, Child: themeEditorFlashOverlay(woxwidget.Container{Width: layout.InnerWidth, Height: layout.BodyHeight + 2}, layout.InnerWidth, layout.BodyHeight+2, 8, true)})
	}
	if props.FlashToken == "PreviewTagFontColor" || props.FlashToken == "PreviewTagBackgroundColor" || props.FlashToken == "PreviewTagBorderColor" || (props.DraftTheme.PreviewTagFontColor == nil && (props.FlashToken == "PreviewPropertyTitleColor" || props.FlashToken == "PreviewPropertyContentColor")) {
		children = append(children, woxwidget.StackChild{Left: 14, Top: 12 + layout.BodyHeight + 2 + 10, Child: themeEditorFlashOverlay(woxwidget.Container{Width: layout.InnerWidth, Height: 26}, layout.InnerWidth, 26, 8, true)})
	}
	if props.FlashToken == "PreviewBorderRadius" {
		radius := float32(8)
		if props.DraftTheme.PreviewBorderRadius != nil {
			radius = float32(*props.DraftTheme.PreviewBorderRadius)
		}
		children = append(children, woxwidget.StackChild{Left: 14, Top: 12, Child: woxcomponent.CornerRadiusHighlight(layout.InnerWidth, layout.BodyHeight+2, radius, themeEditorFlashColor())})
	}
	if props.FlashToken == "PreviewTagBorderRadius" {
		metrics, _ := props.Window.MeasureText("2026-05-26 10:47:08", woxui.TextStyle{Size: props.Theme.Scaled(11), Weight: woxui.FontWeightSemibold})
		chipWidth := min(max(float32(36), metrics.Size.Width+18), min(float32(220), max(float32(36), layout.InnerWidth)))
		radius := float32(8)
		if props.DraftTheme.PreviewTagBorderRadius != nil {
			radius = float32(*props.DraftTheme.PreviewTagBorderRadius)
		}
		children = append(children, woxwidget.StackChild{Left: 14, Top: 12 + layout.BodyHeight + 2 + 10, Child: woxcomponent.CornerRadiusHighlight(chipWidth, 26, radius, themeEditorFlashColor())})
	}
	if props.FlashToken == "PreviewSplitLineColor" {
		children = append(children, woxwidget.StackChild{Child: themeEditorFlashOverlay(woxwidget.Container{Width: 3, Height: height, Color: props.DraftTheme.PreviewSplit}, 3, height, 0, true)})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: children}
}

func themeEditorActions(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	gap := float32(8)
	count := 2
	if props.CanOverwrite {
		count = 3
	}
	buttonWidth := (width - gap*float32(count-1)) / float32(count)
	saveLabel := props.SaveAsLabel
	if props.Saving {
		saveLabel = props.SavingLabel
	}
	buttons := []woxwidget.Widget{woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-discard", Label: props.DiscardLabel, Width: buttonWidth, Disabled: props.Saving || props.AIBusy || !props.Dirty, Theme: props.Theme, OnTap: props.OnDiscard})}
	if props.CanOverwrite {
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-overwrite", Label: props.OverwriteLabel, Width: buttonWidth, Disabled: props.Saving || props.AIBusy || !props.Dirty, Theme: props.Theme, OnTap: props.OnOverwrite}))
	}
	buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-save-as", Label: saveLabel, Width: buttonWidth, Disabled: props.Saving || props.AIBusy, Variant: woxcomponent.ButtonPrimary, Theme: props.Theme, OnTap: props.OnSaveAs}))
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: buttons}
}

func themeEditorQueryAccessories(props ThemeEditorSettingsProps) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		themeEditorAttentionAccessory(props),
		themeEditorGlanceAccessory(props),
	}}
}

func themeEditorAttentionAccessory(props ThemeEditorSettingsProps) woxwidget.Widget {
	hovered := props.FlashToken == "AttentionHoverBackgroundColor" || props.FlashToken == "AttentionHoverBorderColor"
	icon, label, background, border := props.DraftTheme.AttentionBadgeColors(hovered)
	borderWidth := float32(0)
	if border.A > 0 {
		borderWidth = 1
	}
	return themeEditorFlashOverlay(woxwidget.Container{
		Width: 46, Height: 30, Radius: 5, Color: background, BorderColor: border, BorderWidth: borderWidth,
		Padding: woxwidget.Insets{Left: 8, Right: 8},
		Child: woxwidget.Align{Width: 30, Height: 30, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				themeEditorFlashOverlay(woxcomponent.NotificationGlyph(16, icon), 16, 16, 3, props.FlashToken == "AttentionIconColor"),
				themeEditorFlashOverlay(woxwidget.Text{Value: "1", Style: woxui.TextStyle{Size: woxcomponent.AttentionBadgeFontSize}, Color: label}, 10, 16, 3, props.FlashToken == "AttentionFontColor"),
			},
		}},
	}, 46, 30, 5, props.FlashToken == "AttentionBackgroundColor" || props.FlashToken == "AttentionBorderColor" || hovered)
}

func themeEditorGlanceAccessory(props ThemeEditorSettingsProps) woxwidget.Widget {
	glanceText, glanceBackground := themeAlpha(props.DraftTheme.QueryText, 178), woxui.Color{}
	glanceIcon := glanceText
	if props.DraftTheme.GlanceFontColor != nil {
		glanceText, glanceBackground = props.DraftTheme.GlanceColors(props.FlashToken == "GlanceHoverBackgroundColor")
	}
	if props.DraftTheme.GlanceIconColor != nil {
		glanceIcon = props.DraftTheme.GlanceIconTint()
	}
	return themeEditorFlashOverlay(woxwidget.Container{Width: 78, Height: 30, Radius: 5, Color: glanceBackground, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Align{Width: 62, Height: 30, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		themeEditorFlashOverlay(woxcomponent.ClockGlyph(16, glanceIcon), 16, 16, 3, props.FlashToken == "GlanceIconColor"),
		themeEditorFlashOverlay(woxwidget.Text{Value: time.Now().Format("15:04"), Style: woxui.TextStyle{Size: woxcomponent.GlanceFontSize}, Color: glanceText}, 41, 20, 3, props.FlashToken == "GlanceFontColor"),
	}}}}, 78, 30, 5, props.FlashToken == "GlanceBackgroundColor" || props.FlashToken == "GlanceHoverBackgroundColor")
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

// editorPane keeps mode navigation beside the content it controls and outside scrolling.
func (s *themeEditorSettingsState) editorPane(ctx woxwidget.StateContext, props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-16)
	buttons := []woxwidget.Widget{}
	for index, label := range []string{props.ModeLabel, props.AILabel} {
		ai := index == 1
		variant := woxcomponent.ButtonText
		weight := woxui.FontWeightRegular
		if ai == props.AIExpanded {
			variant = woxcomponent.ButtonSelected
			weight = woxui.FontWeightSemibold
		}
		var icon *woxui.Image
		if ai {
			icon = props.AIIcon
		}
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: fmt.Sprintf("theme-editor-mode-%d", index), Label: label, Icon: icon,
			Width: max(float32(0), (contentWidth-8)/2), Theme: props.Theme, Variant: variant, FontWeight: weight,
			OnTap: func() {
				if props.OnSelectAI != nil && props.AIExpanded != ai {
					props.OnSelectAI(ai)
				}
			},
		}))
	}
	modes := woxwidget.Container{Width: contentWidth, Height: 36, Radius: 6, Color: props.Theme.InputBackground, Padding: woxwidget.Insets{Left: 2, Right: 2, Top: 2, Bottom: 2}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: buttons}}
	contentHeight := max(float32(0), height-92)
	var content woxwidget.Widget
	if props.AIExpanded && props.AIAssistant != nil {
		content = props.AIAssistant
	} else {
		content = s.inspector(ctx, props, width, contentHeight)
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{modes, woxwidget.Container{Width: width, Height: contentHeight, Child: content}, themeEditorActions(props, contentWidth, 32)}}
}

// ThemeEditorInspectorSize shares the available chat bounds with the adapter.
func ThemeEditorInspectorSize(width, height float32, hasError bool) (float32, float32) {
	if width >= 760 {
		return 324, max(float32(0), height-92)
	}
	height = max(float32(0), height-44)
	if hasError {
		height = max(float32(0), height-40)
	}
	previewHeight := min(float32(300), float32(math.Floor(float64(height*.35))))
	return max(float32(0), width-16), max(float32(0), height-previewHeight-104)
}
