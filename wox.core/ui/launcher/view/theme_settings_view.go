package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const ThemeListRowHeight = float32(72)

// ThemeCatalogItem contains resolved presentation data for one theme.
type ThemeCatalogItem struct {
	SourceIndex       int
	ID                string
	Name              string
	Author            string
	URL               string
	Version           string
	Description       string
	IsSystem          bool
	IsInstalled       bool
	IsUpgradable      bool
	IsAuto            bool
	Active            bool
	Selected          bool
	PreviewTheme      woxcomponent.Theme
	LightPreviewTheme woxcomponent.Theme
	DarkPreviewTheme  woxcomponent.Theme
}

// ThemeSettingsProps contains theme catalog state, localized labels, and actions.
type ThemeSettingsProps struct {
	Width                 float32
	Height                float32
	Theme                 woxcomponent.Theme
	Mode                  string
	Message               string
	MessageError          bool
	Error                 string
	Operation             string
	UninstallArmed        string
	Items                 []ThemeCatalogItem
	Detail                *ThemeCatalogItem
	Search                woxui.TextEditingState
	SearchFocused         bool
	SearchPlaceholder     string
	LocateLabel           string
	EmptyLabel            string
	WebsiteLabel          string
	InstallLabel          string
	ApplyLabel            string
	UninstallLabel        string
	UpdateLabel           string
	PreviewLabel          string
	DescriptionLabel      string
	SystemLabel           string
	AutoAppearanceHint    string
	AutoAppearanceAccent  woxui.Color
	PreviewTitle          string
	PreviewTexts          []string
	PreviewSubtitles      []string
	PreviewOpenLabel      string
	ActiveDetailTab       string
	Window                *woxui.Window
	LocateIcon            *woxui.Image
	ExternalIcon          *woxui.Image
	InstalledIcon         *woxui.Image
	InstalledSelectedIcon *woxui.Image
	AutoAppearanceIcon    *woxui.Image
	OnSelect              func(int)
	OnSearchKey           func(woxui.KeyEvent) bool
	OnSearchFocusChange   func(bool)
	OnSearchChanged       func(string)
	OnSetSearchValue      func(string) error
	OnClear               func()
	OnLocateCurrent       func()
	OnSelectDetailTab     func(string)
	OnOpenWebsite         func()
	OnOperation           func(string)
}

// ThemeSettingsView mirrors Flutter's fixed-width catalog, divider, and expanded detail pane.
func ThemeSettingsView(props ThemeSettingsProps) woxwidget.Widget {
	const listWidth = float32(260)
	const dividerGutter = float32(21)
	detailWidth := max(float32(0), props.Width-listWidth-dividerGutter)
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		themeList(props, listWidth, props.Height),
		woxwidget.Container{Width: dividerGutter, Height: props.Height, Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: woxwidget.Container{Width: 1, Height: props.Height, Color: props.Theme.PreviewSplit}},
		themeDetail(props, detailWidth, props.Height),
	}}
}

func themeList(props ThemeSettingsProps, width, height float32) woxwidget.Widget {
	const searchHeight = float32(42)
	const searchGap = float32(20)
	viewportHeight := max(float32(0), height-searchHeight-searchGap)

	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		background := woxui.Color{}
		titleColor := props.Theme.ResultTitle
		subtitleColor := props.Theme.ResultSubtitle
		if item.Selected {
			background = props.Theme.SelectedBackground
			titleColor = props.Theme.ActionSelectedText
			subtitleColor = props.Theme.ActionSelectedText
		}
		trailing, trailingWidth := themeListTrailing(props, item, subtitleColor)
		textWidth := max(float32(0), width-32-10-trailingWidth-18)
		status := strings.TrimSpace(item.Version + "  " + item.Author)
		var swatch woxwidget.Widget = themeSwatch(item.PreviewTheme, 32)
		if item.IsAuto {
			swatch = themeAutoSwatch(item.LightPreviewTheme, item.DarkPreviewTheme, 32)
		}
		rowChildren := []woxwidget.Widget{
			swatch,
			woxwidget.Container{Width: textWidth, Height: 44, Child: woxwidget.Clip{Width: textWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
				woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 15}, Color: titleColor},
				woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 12}, Color: subtitleColor},
			}}}},
		}
		if trailing != nil {
			rowChildren = append(rowChildren, trailing)
		}
		radius := float32(4)
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: "theme-list-" + item.ID, Label: item.Name, Width: width, Height: ThemeListRowHeight - 8, Radius: &radius,
			Background: &background, Selected: item.Selected, Padding: woxwidget.Insets{Left: 6, Top: 10, Right: 6, Bottom: 10}, Theme: props.Theme,
			OnTap: func() {
				if props.OnSelect != nil {
					props.OnSelect(item.SourceIndex)
				}
			}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: rowChildren},
		}))
	}

	var list woxwidget.Widget
	if props.Message != "" {
		color := props.Theme.ResultSubtitle
		if props.MessageError {
			color = props.Theme.ErrorText
		}
		list = woxwidget.Container{Width: width, Height: viewportHeight, Padding: woxwidget.Insets{Top: 18}, Child: woxwidget.TextBlock{
			Value: props.Message, Width: width, Height: min(float32(80), viewportHeight), MaxLines: 3, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: color,
		}}
	} else if len(rows) == 0 {
		list = woxwidget.Container{Width: width, Height: viewportHeight, Padding: woxwidget.Insets{Top: 18}, Child: woxwidget.Text{Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}}
	} else {
		var keepVisible *woxwidget.ScrollRange
		for index, item := range props.Items {
			if item.Selected {
				start := float32(index) * ThemeListRowHeight
				keepVisible = &woxwidget.ScrollRange{Start: start, End: start + ThemeListRowHeight}
				break
			}
		}
		list = woxwidget.ScrollView{
			Key: "theme-list-scroll", ID: "theme-list-scroll", Width: width, Height: viewportHeight,
			ContentHeight: max(viewportHeight, float32(len(rows))*ThemeListRowHeight), KeepVisible: keepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows},
		}
	}

	actions := make([]woxcomponent.SearchFieldAction, 0, 1)
	if props.Mode != "store" {
		actions = append(actions, woxcomponent.SearchFieldAction{ID: "theme-locate-current", Label: props.LocateLabel, Icon: props.LocateIcon, Width: 32, IconSize: 18, OnTap: props.OnLocateCurrent})
	}
	searchField := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "theme-search", Label: props.SearchPlaceholder, Width: width, Value: props.Search.Text, Focused: props.SearchFocused, Autofocus: true,
		Actions: actions, Window: props.Window, Theme: props.Theme, OnClear: props.OnClear,
		OnKey: props.OnSearchKey, OnFocusChange: props.OnSearchFocusChange, OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearchValue,
	})
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: searchGap, Children: []woxwidget.Widget{searchField, list}}
}

func themeListTrailing(props ThemeSettingsProps, item ThemeCatalogItem, tagColor woxui.Color) (woxwidget.Widget, float32) {
	if props.Mode == "store" && item.IsInstalled {
		icon := props.InstalledIcon
		if item.Selected {
			icon = props.InstalledSelectedIcon
		}
		return woxwidget.Align{Width: 26, Height: 44, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: icon, Width: 20, Height: 20}}, 26
	}
	if props.Mode != "store" && item.IsSystem {
		const width = float32(44)
		return woxwidget.Align{Width: width, Height: 44, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxTag(props.SystemLabel, tagColor)}, width
	}
	return nil, 0
}

func themeDetail(props ThemeSettingsProps, width, height float32) woxwidget.Widget {
	if props.Detail == nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 16, Top: 20}, Child: woxwidget.Text{
			Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
	}
	theme := *props.Detail
	const headerHeight = float32(120)
	const tabHeight = float32(46)
	innerWidth := max(float32(0), width-32)
	var website woxwidget.Widget = woxwidget.Container{Width: 104, Height: 28}
	if strings.TrimSpace(theme.URL) != "" && props.OnOpenWebsite != nil {
		websiteChildren := make([]woxwidget.Widget, 0, 2)
		if props.ExternalIcon != nil {
			websiteChildren = append(websiteChildren, woxwidget.Image{Source: props.ExternalIcon, Width: 13, Height: 13})
		}
		websiteChildren = append(websiteChildren, woxwidget.Text{Value: props.WebsiteLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle})
		website = woxwidget.Gesture{ID: "theme-website", OnTap: props.OnOpenWebsite, Child: woxwidget.Align{Width: 104, Height: 28, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 7, Children: websiteChildren,
		}}}
	}
	versionWidth := float32(84)
	nameWidth := max(float32(80), innerWidth-versionWidth-10)
	header := woxwidget.Container{Width: width, Height: headerHeight, Padding: woxwidget.Insets{Left: 16, Right: 16}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 40, Padding: woxwidget.Insets{Left: 2}, Child: woxwidget.Clip{Width: innerWidth, Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: nameWidth, Height: 32, Child: woxwidget.Clip{Width: nameWidth, Height: 32, Child: woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 20}, Color: props.Theme.QueryText}}},
			woxwidget.Container{Width: versionWidth, Height: 32, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: theme.Version, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}},
		}}}},
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), innerWidth-104), Height: 28, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: theme.Author, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}},
			website,
		}},
		woxwidget.Container{Width: innerWidth, Height: 52, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: themeActions(props, theme)}},
	}}}
	tabs := PluginTabs(PluginTabsProps{Width: width, Height: tabHeight, Active: props.ActiveDetailTab, Tabs: []PluginTab{
		{ID: "preview", Label: props.PreviewLabel, Width: 76},
		{ID: "description", Label: props.DescriptionLabel, Width: 96},
	}, Theme: props.Theme, OnSelect: props.OnSelectDetailTab})
	bodyHeight := max(float32(0), height-headerHeight-tabHeight)
	var body woxwidget.Widget
	if props.ActiveDetailTab == "description" {
		body = themeDescriptionTab(theme, width, bodyHeight, props.Theme)
	} else {
		body = themePreviewTab(props, theme, width, bodyHeight)
	}
	if props.Error != "" {
		body = woxwidget.Stack{Width: width, Height: bodyHeight, Children: []woxwidget.StackChild{
			{Child: body},
			{Left: 16, Top: max(float32(0), bodyHeight-48), Child: woxwidget.TextBlock{Value: props.Error, Width: max(float32(0), width-32), Height: 44, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText}},
		}}
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, tabs, body}}
}

func themeDescriptionTab(theme ThemeCatalogItem, width, height float32, colors woxcomponent.Theme) woxwidget.Widget {
	description := theme.Description
	if strings.TrimSpace(description) == "" {
		description = "—"
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
		Value: description, Width: max(float32(0), width-32), Height: max(float32(0), height-32), MaxLines: 30, Style: woxui.TextStyle{Size: 13}, LineHeight: 21, Color: colors.ResultTitle,
	}}
}

func themePreviewTab(props ThemeSettingsProps, theme ThemeCatalogItem, width, height float32) woxwidget.Widget {
	const horizontalPadding = float32(20)
	const topPadding = float32(20)
	const bottomPadding = float32(200)
	hintHeight := float32(0)
	children := make([]woxwidget.Widget, 0, 2)
	if theme.IsAuto && props.AutoAppearanceHint != "" {
		hintHeight = 58
		accent := props.AutoAppearanceAccent
		backgroundAlpha := uint8(26)
		borderAlpha := uint8(77)
		if int(props.Theme.Background.R)*299+int(props.Theme.Background.G)*587+int(props.Theme.Background.B)*114 < 128000 {
			backgroundAlpha = 36
			borderAlpha = 89
		}
		icon := woxwidget.Widget(woxwidget.Painter{Width: 16, Height: 16})
		if props.AutoAppearanceIcon != nil {
			icon = woxwidget.Image{Source: props.AutoAppearanceIcon, Width: 16, Height: 16}
		}
		children = append(children, woxwidget.Container{
			Width: max(float32(0), width-horizontalPadding*2), Height: 46, Radius: 10, Color: settingsColorAlpha(accent, backgroundAlpha),
			BorderColor: settingsColorAlpha(accent, borderAlpha), BorderWidth: 1, Padding: woxwidget.Insets{Left: 12, Top: 14, Right: 12},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				icon,
				woxwidget.TextBlock{Value: props.AutoAppearanceHint, Width: max(float32(0), width-horizontalPadding*2-50), Height: 18, MaxLines: 1, LineHeight: 18, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle},
			}},
		})
	}
	previewHeight := max(float32(0), height-topPadding-bottomPadding-hintHeight)
	previewWidth := max(float32(0), width-horizontalPadding*2)
	preview := themeCatalogPreview(props, theme.PreviewTheme, previewWidth, previewHeight)
	if theme.IsAuto {
		preview = themeAutoCatalogPreview(props, theme.LightPreviewTheme, theme.DarkPreviewTheme, previewWidth, previewHeight)
	}
	children = append(children, preview)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: horizontalPadding, Top: topPadding, Right: horizontalPadding, Bottom: bottomPadding}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 12, Children: children,
	}}
}

func themeCatalogPreview(props ThemeSettingsProps, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	if width <= 0 || height <= 0 {
		return woxwidget.Container{Width: max(float32(0), width), Height: max(float32(0), height)}
	}
	const queryAreaHeight = float32(60)
	const toolbarHeight = float32(34)
	rowsHeight := max(float32(0), height-queryAreaHeight-toolbarHeight)
	rowWidgets := make([]woxwidget.Widget, 0, len(props.PreviewTexts))
	for index, title := range props.PreviewTexts {
		subtitle := ""
		if index < len(props.PreviewSubtitles) {
			subtitle = props.PreviewSubtitles[index]
		}
		selected := index == 1
		background := woxui.Color{}
		titleColor := theme.ResultTitle
		subtitleColor := theme.ResultSubtitle
		if selected {
			background = theme.SelectedBackground
			titleColor = theme.SelectedTitle
			subtitleColor = theme.SelectedSubtitle
		}
		rowWidgets = append(rowWidgets, woxwidget.Container{Width: max(float32(0), width-20), Height: 60, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 10}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				woxwidget.Align{Width: 30, Height: 42, Vertical: 0.5, Child: woxwidget.Text{Value: "📁", Style: woxui.TextStyle{Size: 22}, Color: titleColor}},
				woxwidget.Container{Width: max(float32(0), width-84), Height: 42, Child: woxwidget.Clip{Width: max(float32(0), width-84), Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
					woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 13}, Color: titleColor},
					woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 11}, Color: subtitleColor},
				}}}},
			},
		}})
	}
	query := woxwidget.Container{Width: max(float32(0), width-20), Height: 40, Radius: 7, Color: theme.QueryBackground, Padding: woxwidget.Insets{Left: 10, Top: 11}, Child: woxwidget.Text{
		Value: props.PreviewTitle, Style: woxui.TextStyle{Size: 13}, Color: theme.QueryText,
	}}
	rows := woxwidget.ScrollView{Width: max(float32(0), width-20), Height: rowsHeight, ContentHeight: max(rowsHeight, float32(len(rowWidgets))*60), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rowWidgets}}
	openWidth := max(float32(0), width-108)
	toolbarContent := woxwidget.Container{Width: width, Height: toolbarHeight - 1, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: openWidth, Height: 24},
		woxwidget.Text{Value: props.PreviewOpenLabel, Style: woxui.TextStyle{Size: 11}, Color: theme.ToolbarText},
		woxwidget.Container{Width: 8, Height: 24},
		woxwidget.Container{Width: 30, Height: 22, Radius: 4, BorderColor: theme.ToolbarText, BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 3}, Child: woxwidget.Text{Value: "↵", Style: woxui.TextStyle{Size: 12}, Color: theme.ToolbarText}},
	}}}
	toolbar := woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 1, Color: theme.PreviewSplit},
		toolbarContent,
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 8, Color: theme.Background, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: queryAreaHeight, Padding: woxwidget.UniformInsets(10), Child: query},
		woxwidget.Container{Width: width, Height: rowsHeight, Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: rows},
		toolbar,
	}}}
}

// themeAutoCatalogPreview layers shared preview content over diagonally split light and dark surfaces.
func themeAutoCatalogPreview(props ThemeSettingsProps, light, dark woxcomponent.Theme, width, height float32) woxwidget.Widget {
	if width <= 0 || height <= 0 {
		return woxwidget.Container{Width: max(float32(0), width), Height: max(float32(0), height)}
	}
	const queryAreaHeight = float32(60)
	const toolbarHeight = float32(34)
	rowsHeight := max(float32(0), height-queryAreaHeight-toolbarHeight)
	background := woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		fillThemeDiagonalRect(displayList, bounds, bounds, light.Background, dark.Background)
		query := woxui.Rect{X: bounds.X + 10, Y: bounds.Y + 10, Width: max(float32(0), bounds.Width-20), Height: 40}
		fillThemeDiagonalRect(displayList, query, bounds, light.QueryBackground, dark.QueryBackground)
		for index := range props.PreviewTexts {
			row := woxui.Rect{X: bounds.X + 10, Y: bounds.Y + queryAreaHeight + float32(index)*60, Width: max(float32(0), bounds.Width-20), Height: 60}
			lightColor, darkColor := light.Background, dark.Background
			if index == 1 {
				lightColor, darkColor = light.SelectedBackground, dark.SelectedBackground
			}
			fillThemeDiagonalRect(displayList, row, bounds, lightColor, darkColor)
		}
		toolbar := woxui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - toolbarHeight, Width: bounds.Width, Height: toolbarHeight}
		fillThemeDiagonalRect(displayList, toolbar, bounds, light.ToolbarBackground, dark.ToolbarBackground)
		fillThemeDiagonalRect(displayList, woxui.Rect{X: toolbar.X, Y: toolbar.Y, Width: toolbar.Width, Height: 1}, bounds, light.PreviewSplit, dark.PreviewSplit)
		drawThemeDiagonalLine(displayList, bounds, 2)
	}}
	rows := make([]woxwidget.Widget, 0, len(props.PreviewTexts))
	for index, title := range props.PreviewTexts {
		subtitle := ""
		if index < len(props.PreviewSubtitles) {
			subtitle = props.PreviewSubtitles[index]
		}
		titleColor, subtitleColor := light.ResultTitle, light.ResultSubtitle
		if index == 1 {
			titleColor, subtitleColor = light.SelectedTitle, light.SelectedSubtitle
		}
		rows = append(rows, woxwidget.Container{Width: max(float32(0), width-20), Height: 60, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 10}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				woxwidget.Align{Width: 30, Height: 42, Vertical: 0.5, Child: woxwidget.Text{Value: "📁", Style: woxui.TextStyle{Size: 22}, Color: titleColor}},
				woxwidget.Container{Width: max(float32(0), width-84), Height: 42, Child: woxwidget.Clip{Width: max(float32(0), width-84), Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
					woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 13}, Color: titleColor},
					woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 11}, Color: subtitleColor},
				}}}},
			},
		}})
	}
	query := woxwidget.Container{Width: max(float32(0), width-20), Height: 40, Padding: woxwidget.Insets{Left: 10, Top: 11}, Child: woxwidget.Text{
		Value: props.PreviewTitle, Style: woxui.TextStyle{Size: 13}, Color: light.QueryText,
	}}
	rowList := woxwidget.ScrollView{Width: max(float32(0), width-20), Height: rowsHeight, ContentHeight: max(rowsHeight, float32(len(rows))*60), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
	openWidth := max(float32(0), width-108)
	toolbar := woxwidget.Container{Width: width, Height: toolbarHeight, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: openWidth, Height: 24},
		woxwidget.Text{Value: props.PreviewOpenLabel, Style: woxui.TextStyle{Size: 11}, Color: dark.ToolbarText},
		woxwidget.Container{Width: 8, Height: 24},
		woxwidget.Container{Width: 30, Height: 22, Radius: 4, BorderColor: dark.ToolbarText, BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 3}, Child: woxwidget.Text{Value: "↵", Style: woxui.TextStyle{Size: 12}, Color: dark.ToolbarText}},
	}}}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: queryAreaHeight, Padding: woxwidget.UniformInsets(10), Child: query},
		woxwidget.Container{Width: width, Height: rowsHeight, Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: rowList},
		toolbar,
	}}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: background}, {Child: content}}}
}

// themeAutoSwatch mirrors Flutter's compact diagonal AUTO theme icon.
func themeAutoSwatch(light, dark woxcomponent.Theme, size float32) woxwidget.Widget {
	return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		fillThemeDiagonalPolygon(displayList, themeRoundedRectPoints(bounds, 8), bounds, light.Background, dark.Background)
		query := woxui.Rect{X: bounds.X + 4, Y: bounds.Y + 5, Width: max(float32(0), bounds.Width-8), Height: 10}
		result := woxui.Rect{X: bounds.X + 4, Y: bounds.Y + 19, Width: max(float32(0), bounds.Width-8), Height: 5}
		fillThemeDiagonalRect(displayList, query, bounds, light.QueryBackground, dark.QueryBackground)
		fillThemeDiagonalRect(displayList, result, bounds, light.SelectedBackground, dark.SelectedBackground)
		drawThemeDiagonalLine(displayList, bounds, 1.5)
	}}
}

// themeRoundedRectPoints approximates Flutter's clipped 8px swatch corners with one portable convex path.
func themeRoundedRectPoints(bounds woxui.Rect, radius float32) []woxui.Point {
	radius = min(max(float32(0), radius), min(bounds.Width, bounds.Height)/2)
	if radius == 0 {
		return []woxui.Point{{X: bounds.X, Y: bounds.Y}, {X: bounds.X + bounds.Width, Y: bounds.Y}, {X: bounds.X + bounds.Width, Y: bounds.Y + bounds.Height}, {X: bounds.X, Y: bounds.Y + bounds.Height}}
	}
	const half = float32(0.5)
	const near = float32(0.134)
	const far = float32(0.866)
	left, top := bounds.X, bounds.Y
	right, bottom := bounds.X+bounds.Width, bounds.Y+bounds.Height
	return []woxui.Point{
		{X: right - radius, Y: top}, {X: right - radius*half, Y: top + radius*near}, {X: right - radius*near, Y: top + radius*half}, {X: right, Y: top + radius},
		{X: right, Y: bottom - radius}, {X: right - radius*near, Y: bottom - radius*half}, {X: right - radius*half, Y: bottom - radius*near}, {X: right - radius, Y: bottom},
		{X: left + radius, Y: bottom}, {X: left + radius*half, Y: bottom - radius*near}, {X: left + radius*near, Y: bottom - radius*half}, {X: left, Y: bottom - radius},
		{X: left, Y: top + radius}, {X: left + radius*near, Y: top + radius*half}, {X: left + radius*half, Y: top + radius*near}, {X: left + radius, Y: top},
	}
}

// fillThemeDiagonalRect clips one surface against both halves of the AUTO preview diagonal.
func fillThemeDiagonalRect(displayList *woxui.DisplayList, rect, splitBounds woxui.Rect, light, dark woxui.Color) {
	fillThemeDiagonalPolygon(displayList, themeDiagonalRectPoints(rect, splitBounds), splitBounds, light, dark)
}

// fillThemeDiagonalPolygon paints both AUTO colors inside one convex surface.
func fillThemeDiagonalPolygon(displayList *woxui.DisplayList, points []woxui.Point, splitBounds woxui.Rect, light, dark woxui.Color) {
	if points := themeDiagonalPolygon(points, splitBounds, true); len(points) >= 3 {
		displayList.FillConvexPolygon(points, light)
	}
	if points := themeDiagonalPolygon(points, splitBounds, false); len(points) >= 3 {
		displayList.FillConvexPolygon(points, dark)
	}
}

// themeDiagonalRectPoints limits a rectangular preview surface to its visible bounds.
func themeDiagonalRectPoints(rect, splitBounds woxui.Rect) []woxui.Point {
	if rect.Width <= 0 || rect.Height <= 0 || splitBounds.Width <= 0 || splitBounds.Height <= 0 {
		return nil
	}
	left := max(rect.X, splitBounds.X)
	top := max(rect.Y, splitBounds.Y)
	right := min(rect.X+rect.Width, splitBounds.X+splitBounds.Width)
	bottom := min(rect.Y+rect.Height, splitBounds.Y+splitBounds.Height)
	if right <= left || bottom <= top {
		return nil
	}
	rect = woxui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
	return []woxui.Point{{X: rect.X, Y: rect.Y}, {X: rect.X + rect.Width, Y: rect.Y}, {X: rect.X + rect.Width, Y: rect.Y + rect.Height}, {X: rect.X, Y: rect.Y + rect.Height}}
}

// themeDiagonalPolygon clips a convex surface to one AUTO preview half-plane.
func themeDiagonalPolygon(points []woxui.Point, splitBounds woxui.Rect, light bool) []woxui.Point {
	if len(points) < 3 || splitBounds.Width <= 0 || splitBounds.Height <= 0 {
		return nil
	}
	value := func(point woxui.Point) float32 {
		return (point.X-splitBounds.X)*splitBounds.Height + (point.Y-splitBounds.Y)*splitBounds.Width - splitBounds.Width*splitBounds.Height
	}
	inside := func(v float32) bool {
		if light {
			return v <= 0
		}
		return v >= 0
	}
	clipped := make([]woxui.Point, 0, 5)
	appendPoint := func(point woxui.Point) {
		if len(clipped) == 0 || clipped[len(clipped)-1] != point {
			clipped = append(clipped, point)
		}
	}
	for index, current := range points {
		next := points[(index+1)%len(points)]
		currentValue, nextValue := value(current), value(next)
		currentInside, nextInside := inside(currentValue), inside(nextValue)
		if currentInside {
			appendPoint(current)
		}
		if currentInside != nextInside {
			t := currentValue / (currentValue - nextValue)
			appendPoint(woxui.Point{X: current.X + (next.X-current.X)*t, Y: current.Y + (next.Y-current.Y)*t})
		}
	}
	if len(clipped) > 1 && clipped[0] == clipped[len(clipped)-1] {
		clipped = clipped[:len(clipped)-1]
	}
	return clipped
}

// themeDiagonalRectPolygon retains the rectangle helper used by focused geometry checks.
func themeDiagonalRectPolygon(rect, splitBounds woxui.Rect, light bool) []woxui.Point {
	return themeDiagonalPolygon(themeDiagonalRectPoints(rect, splitBounds), splitBounds, light)
}

// drawThemeDiagonalLine separates the light and dark preview halves with Flutter's subtle dark rule.
func drawThemeDiagonalLine(displayList *woxui.DisplayList, bounds woxui.Rect, width float32) {
	half := width / 2
	displayList.FillConvexPolygon([]woxui.Point{
		{X: bounds.X + bounds.Width - half, Y: bounds.Y},
		{X: bounds.X + bounds.Width + half, Y: bounds.Y},
		{X: bounds.X + half, Y: bounds.Y + bounds.Height},
		{X: bounds.X - half, Y: bounds.Y + bounds.Height},
	}, woxui.Color{A: 38})
}

func themeActions(props ThemeSettingsProps, theme ThemeCatalogItem) []woxwidget.Widget {
	busy := props.Operation != ""
	button := func(id, label, operation string, disabled bool) woxwidget.Widget {
		if props.Operation == operation+":"+theme.ID {
			label += "…"
		}
		return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Height: 36, IntrinsicWidth: true, Disabled: busy || disabled, Variant: woxcomponent.ButtonOutline, OnTap: func() {
			if props.OnOperation != nil {
				props.OnOperation(operation)
			}
		}, Theme: props.Theme})
	}
	if !theme.IsInstalled {
		return []woxwidget.Widget{button("theme-install", props.InstallLabel, "install", false)}
	}
	buttons := make([]woxwidget.Widget, 0, 3)
	if theme.IsUpgradable {
		buttons = append(buttons, button("theme-upgrade", props.UpdateLabel, "upgrade", false))
	}
	buttons = append(buttons, button("theme-apply", props.ApplyLabel, "apply", theme.Active))
	if !theme.IsSystem {
		label := props.UninstallLabel
		if props.UninstallArmed == theme.ID {
			label = "Confirm " + props.UninstallLabel
		}
		buttons = append(buttons, button("theme-uninstall", label, "uninstall", false))
	}
	return buttons
}

func themeSwatch(theme woxcomponent.Theme, size float32) woxwidget.Widget {
	innerWidth := max(float32(0), size-8)
	return woxwidget.Container{Width: size, Height: size, Radius: 8, Color: theme.Background, Padding: woxwidget.Insets{Left: 4, Top: 5, Right: 4}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: 10, Radius: 4, Color: theme.QueryBackground},
			woxwidget.Container{Width: innerWidth, Height: 5, Radius: 2, Color: theme.SelectedBackground},
		},
	}}
}
