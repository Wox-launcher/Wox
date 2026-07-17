package view

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const themeEditorControlPaneHeight = float32(140)

// ThemeEditorColorToken contains one editable color and its resolved preview swatch.
type ThemeEditorColorToken struct {
	Key   string
	Label string
	Color woxui.Color
}

// ThemeEditorColorGroup contains the tokens shown together in the bottom editor pane.
type ThemeEditorColorGroup struct {
	Label  string
	Tokens []ThemeEditorColorToken
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
	PreviewGeometry    ThemeEditorPreviewGeometry
	FlashToken         string
	LocateIcon         *woxui.Image
	DiscardIcon        *woxui.Image
	OverwriteIcon      *woxui.Image
	SaveAsIcon         *woxui.Image
	DiscardLabel       string
	OverwriteLabel     string
	SaveAsLabel        string
	SavingLabel        string
	PreviewResultTitle string
	PreviewResultState string
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
		stage.Children = append(stage.Children, woxwidget.StackChild{Child: woxwidget.Clip{Width: stageWidth, Height: stageHeight, Child: woxwidget.Image{Source: props.Wallpaper, Width: stageWidth, Height: stageHeight}}})
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
	}
	children = append(children,
		woxwidget.StackChild{Child: woxwidget.Container{Width: width, Height: height, Radius: 12, Color: themeEditorMicaSurfaceColor(props.DraftTheme.Background)}},
		woxwidget.StackChild{Left: props.PreviewGeometry.AppPadding.Left, Top: props.PreviewGeometry.AppPadding.Top, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			themeEditorPreviewQuery(props, contentWidth, queryHeight), body,
		}}},
		woxwidget.StackChild{Top: height - toolbarHeight, Child: themeEditorPreviewToolbar(props, width, toolbarHeight)},
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
	queryContentWidth := max(float32(0), width-28)
	memoryWidth := float32(90)
	queryWidth := max(float32(0), queryContentWidth-memoryWidth)
	query := woxwidget.Container{Width: queryWidth, Height: height, Padding: woxwidget.Insets{Top: 17}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "theme ", Style: woxui.TextStyle{Size: 20}, Color: props.DraftTheme.QueryText},
		woxwidget.Container{Height: 26, Color: selection, Child: woxwidget.Text{Value: "edit", Style: woxui.TextStyle{Size: 20}, Color: selectionText}},
		woxwidget.Container{Width: 2, Height: 26, Color: props.DraftTheme.Cursor},
	}}}
	memory := woxwidget.Container{Width: memoryWidth, Height: height, Padding: woxwidget.Insets{Top: 19}, Child: woxwidget.Text{Value: "⚙  761 MB", Style: woxui.TextStyle{Size: 12}, Color: themeAlpha(props.DraftTheme.QueryText, 178)}}
	borderColor, borderWidth := themeEditorFlashBorder(props.FlashToken, "QueryBox")
	return woxwidget.Container{Width: width, Height: height, Radius: props.PreviewGeometry.QueryRadius, Color: props.DraftTheme.QueryBackground, BorderColor: borderColor, BorderWidth: borderWidth, Padding: woxwidget.Insets{Left: 8, Right: 6}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{query, memory},
	}}
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
		borderColor := woxui.Color{}
		borderWidth := float32(0)
		if strings.HasPrefix(props.FlashToken, "ResultItem") && (index == 0 || props.FlashToken == "ResultItemTitleColor" || props.FlashToken == "ResultItemSubTitleColor" || props.FlashToken == "ResultItemTailTextColor") {
			borderColor = themeEditorFlashColor()
			borderWidth = 2
		}
		textWidth := max(float32(0), width-132)
		innerHeight := max(float32(0), rowHeight-props.PreviewGeometry.ResultItemPadding.Top-props.PreviewGeometry.ResultItemPadding.Bottom)
		rows = append(rows, woxwidget.Container{Width: width, Height: rowHeight, Radius: props.PreviewGeometry.ResultItemRadius, Color: background, BorderColor: borderColor, BorderWidth: borderWidth, Padding: props.PreviewGeometry.ResultItemPadding, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Align{Width: 34, Height: innerHeight, Vertical: 0.5, Child: woxwidget.Container{Width: 28, Height: 28, Radius: 6, Color: icons[index].color, Padding: woxwidget.Insets{Left: 7, Top: 4}, Child: woxwidget.Text{Value: icons[index].glyph, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}}}},
				woxwidget.Align{Width: textWidth, Height: innerHeight, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
					woxwidget.Text{Value: titles[index], Style: woxui.TextStyle{Size: 13}, Color: titleColor},
					woxwidget.Text{Value: subtitles[index], Style: woxui.TextStyle{Size: 10}, Color: subtitleColor},
				}}},
				themeEditorTailBadge("P1", titleColor),
				themeEditorTailBadge(map[bool]string{true: "13ms", false: "4ms"}[index == 0], titleColor),
			},
		}})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}
}

func themeEditorTailBadge(label string, color woxui.Color) woxwidget.Widget {
	width := max(float32(30), float32(utf8.RuneCountInString(label))*6+12)
	return woxwidget.Align{Width: width, Height: 30, Vertical: 0.5, Child: woxwidget.Container{Width: width, Height: 22, Radius: 11, BorderColor: themeAlpha(color, 110), BorderWidth: 1, Padding: woxwidget.Insets{Left: 7, Top: 4}, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: 9}, Color: themeAlpha(color, 190),
	}}}
}

func themeEditorPreviewWithTextPanel(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(180), width*0.42)
	resultWidth := max(float32(0), width-panelWidth)
	results := woxwidget.Container{Width: resultWidth, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 4}, Child: themeEditorPreviewResultRows(props, max(float32(0), resultWidth-16), max(float32(0), height-12))}
	panelBorder, panelBorderWidth := themeEditorFlashBorder(props.FlashToken, "Preview")
	if panelBorderWidth == 0 {
		panelBorder = props.DraftTheme.PreviewSplit
		panelBorderWidth = 1
	}
	panel := woxwidget.Container{Width: panelWidth, Height: height, BorderColor: panelBorder, BorderWidth: panelBorderWidth, Padding: woxwidget.Insets{Left: 12, Top: 12, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 9, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "Theme Preview", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.DraftTheme.PreviewText},
		woxwidget.TextBlock{Value: "Colors update immediately in this live preview.", Width: max(float32(0), panelWidth-22), Height: 46, MaxLines: 2, Style: woxui.TextStyle{Size: 10}, LineHeight: 15, Color: themeAlpha(props.DraftTheme.PreviewText, 210)},
		woxwidget.Container{Width: min(float32(126), panelWidth-22), Height: 24, Radius: 7, BorderColor: props.DraftTheme.PreviewPropertyTitle, BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 5}, Child: woxwidget.Text{Value: "Theme editor", Style: woxui.TextStyle{Size: 9}, Color: props.DraftTheme.PreviewPropertyContent}},
	}}}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{results, panel}}
}

func themeEditorPreviewWithActionPanel(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(230), width*0.38)
	panelHeight := min(float32(170), height-12)
	panelLeft := max(float32(0), width-panelWidth-12)
	panelTop := max(float32(0), height-panelHeight-8)
	panelBorder, panelBorderWidth := themeEditorFlashBorder(props.FlashToken, "Action")
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: themeEditorPreviewResults(props, width, height)},
		{Left: panelLeft, Top: panelTop, Child: woxwidget.Container{Width: panelWidth, Height: panelHeight, Radius: 8, Color: props.DraftTheme.ActionBackground, BorderColor: panelBorder, BorderWidth: panelBorderWidth, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Actions", Style: woxui.TextStyle{Size: 11}, Color: props.DraftTheme.ActionHeader},
			woxwidget.Container{Width: panelWidth - 20, Height: 38, Radius: 5, Color: props.DraftTheme.ActionSelected, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: woxwidget.Text{Value: props.ToolbarCopyLabel, Style: woxui.TextStyle{Size: 10}, Color: props.DraftTheme.ActionSelectedText}},
			woxwidget.Container{Width: panelWidth - 20, Height: 38, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: woxwidget.Text{Value: props.ToolbarMoreLabel, Style: woxui.TextStyle{Size: 10}, Color: props.DraftTheme.ActionText}},
			woxwidget.Container{Width: panelWidth - 20, Height: 28, Radius: 5, Color: props.DraftTheme.QueryBackground, Padding: woxwidget.Insets{Left: 9, Top: 7}, Child: woxwidget.Text{Value: props.QueryBoxLabel, Style: woxui.TextStyle{Size: 9}, Color: themeAlpha(props.DraftTheme.ActionText, 170)}},
		}}}},
	}}
}

func themeEditorPreviewToolbar(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	copyWidth := max(float32(120), float32(utf8.RuneCountInString(props.ToolbarCopyLabel))*7+46)
	moreWidth := max(float32(108), float32(utf8.RuneCountInString(props.ToolbarMoreLabel))*7+66)
	spacerWidth := max(float32(0), width-copyWidth-moreWidth-20)
	borderColor, borderWidth := themeEditorFlashBorder(props.FlashToken, "Toolbar")
	if borderWidth == 0 {
		borderColor = themeAlpha(props.DraftTheme.ToolbarText, 32)
		borderWidth = 1
	}
	padding := props.PreviewGeometry.ToolbarPadding
	padding.Top = 9
	return woxwidget.Container{Width: width, Height: height, Color: props.DraftTheme.ToolbarBackground, BorderColor: borderColor, BorderWidth: borderWidth, Padding: padding, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: spacerWidth, Height: 22},
		themeEditorToolbarAction(props.ToolbarCopyLabel, "Enter", copyWidth, props.DraftTheme.ToolbarText),
		themeEditorToolbarAction(props.ToolbarMoreLabel, "Cmd  J", moreWidth, props.DraftTheme.ToolbarText),
	}}}
}

func themeEditorToolbarAction(label, key string, width float32, color woxui.Color) woxwidget.Widget {
	keyWidth := max(float32(34), float32(utf8.RuneCountInString(key))*6+12)
	return woxwidget.Container{Width: width, Height: 22, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), width-keyWidth-8), Height: 22, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 9}, Color: color}},
		woxwidget.Container{Width: keyWidth, Height: 20, Radius: 4, BorderColor: themeAlpha(color, 180), BorderWidth: 1, Padding: woxwidget.Insets{Left: 7, Top: 3}, Child: woxwidget.Text{Value: key, Style: woxui.TextStyle{Size: 8}, Color: color}},
	}}}
}

func themeEditorControlPane(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-36)
	actionsWidth := min(float32(370), innerWidth*0.42)
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
	for index, group := range props.Groups {
		index := index
		chipWidth := max(float32(54), float32(utf8.RuneCountInString(group.Label))*7+24)
		background := woxui.Color{}
		border := woxui.Color{}
		foreground := themeAlpha(props.Theme.ResultTitle, 198)
		if index == props.ActiveGroup {
			background = themeAlpha(props.Theme.SelectedBackground, 42)
			border = themeAlpha(props.Theme.SelectedBackground, 96)
			foreground = props.Theme.ResultTitle
		}
		chips = append(chips, woxwidget.Gesture{ID: "theme-editor-group-" + strconv.Itoa(index), OnTap: func() {
			if props.OnSelectGroup != nil {
				props.OnSelectGroup(index)
			}
		}, Child: woxwidget.Container{Width: chipWidth, Height: 34, Radius: 6, Color: background, BorderColor: border, BorderWidth: themeBoolFloat(border.A != 0), Padding: woxwidget.Insets{Left: 12, Top: 9}, Child: woxwidget.Text{
			Value: group.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground,
		}}})
	}
	return woxwidget.Clip{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: chips}}
}

func themeEditorActions(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	gap := float32(10)
	buttonWidth := max(float32(74), (width-gap*2)/3)
	saveLabel := props.SaveAsLabel
	if props.Saving {
		saveLabel = props.SavingLabel
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-discard", Label: props.DiscardLabel, Icon: props.DiscardIcon, Width: buttonWidth, Height: height, Radius: 5, FontSize: 11, Disabled: props.Saving || !props.Dirty, Variant: woxcomponent.ButtonOutline, OnTap: props.OnDiscard, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-overwrite", Label: props.OverwriteLabel, Icon: props.OverwriteIcon, Width: buttonWidth, Height: height, Radius: 5, FontSize: 11, Disabled: props.Saving || !props.Dirty || !props.CanOverwrite, Variant: woxcomponent.ButtonOutline, OnTap: props.OnOverwrite, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-editor-save-as", Label: saveLabel, Icon: props.SaveAsIcon, Width: buttonWidth, Height: height, Radius: 5, FontSize: 11, Disabled: props.Saving, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSaveAs, Theme: props.Theme}),
	}}
}

func themeEditorTokens(props ThemeEditorSettingsProps, width, height float32) woxwidget.Widget {
	if props.ActiveGroup < 0 || props.ActiveGroup >= len(props.Groups) {
		return woxwidget.Container{Width: width, Height: height}
	}
	group := props.Groups[props.ActiveGroup]
	cards := make([]woxwidget.Widget, 0, len(group.Tokens))
	for _, token := range group.Tokens {
		token := token
		cards = append(cards, themeEditorTokenCard(props, token, 190, min(float32(58), height)))
	}
	return woxwidget.Clip{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: cards}}
}

func themeEditorTokenCard(props ThemeEditorSettingsProps, token ThemeEditorColorToken, width, height float32) woxwidget.Widget {
	labelWidth := max(float32(0), width-86)
	locate := woxwidget.Gesture{ID: "theme-editor-locate-" + token.Key, OnTap: func() {
		if props.OnLocateToken != nil {
			props.OnLocateToken(token.Key)
		}
	}, Child: woxwidget.Align{Width: 26, Height: height - 2, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.LocateIcon, Width: 15, Height: 15}}}
	card := woxwidget.Container{Width: width, Height: height, Radius: 7, BorderColor: themeAlpha(props.Theme.PreviewSplit, 148), BorderWidth: 1, Padding: woxwidget.Insets{Left: 12, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Align{Width: labelWidth, Height: height - 2, Vertical: 0.5, Child: woxwidget.Clip{Width: labelWidth, Height: 24, Child: woxwidget.Text{Value: token.Label, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle}}},
		locate,
		woxwidget.Container{Width: 8, Height: height - 2},
		woxwidget.Align{Width: 28, Height: height - 2, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Container{Width: 28, Height: 28, Radius: 6, Color: token.Color, BorderColor: themeAlpha(props.Theme.PreviewSplit, 190), BorderWidth: 1}},
	}}}
	return woxwidget.Gesture{ID: "theme-editor-token-" + token.Key, OnTap: func() {
		if props.OnEditToken != nil {
			props.OnEditToken(token.Key)
		}
	}, Child: card}
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

func themeEditorFlashBorder(token, prefix string) (woxui.Color, float32) {
	if strings.HasPrefix(token, prefix) {
		return themeEditorFlashColor(), 2
	}
	return woxui.Color{}, 0
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
