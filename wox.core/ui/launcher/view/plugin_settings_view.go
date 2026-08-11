package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PluginSettingsPageProps contains the data required by the plugin list and detail views.
type PluginSettingsPageProps struct {
	Width       float32
	Height      float32
	List        PluginListProps
	Detail      PluginDetailProps
	FilterPanel *PluginFilterPanelProps
	Theme       woxcomponent.Theme
}

// PluginSettingsPage builds the split plugin management route.
func PluginSettingsPage(props PluginSettingsPageProps) woxwidget.Widget {
	innerHeight := max(float32(0), props.Height-40)
	content := woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(20), Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.List.Width, Height: innerHeight, Child: PluginList(props.List)},
		woxwidget.Container{Width: 10, Height: innerHeight},
		woxwidget.Container{Width: 1, Height: innerHeight, Color: props.Theme.PreviewSplit},
		woxwidget.Container{Width: 10, Height: innerHeight},
		woxwidget.Container{Width: props.Detail.Width, Height: innerHeight, Child: PluginDetail(props.Detail)},
	}}}
	if props.FilterPanel == nil {
		return content
	}
	// Flutter anchors the panel before the two 30px search actions, 8px below the button, and shifts it only when the 360px minimum cannot fit.
	panelLeft := 20 + max(float32(0), props.List.Width-64)
	panelProps := *props.FilterPanel
	panelProps.Width = min(panelProps.Width, max(float32(0), props.Width-panelLeft-12))
	if panelProps.Width < 360 {
		panelProps.Width = min(props.FilterPanel.Width, max(float32(0), props.Width-24))
		panelLeft = min(max(float32(12), panelLeft), max(float32(12), props.Width-panelProps.Width-12))
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: content},
		{Child: woxwidget.Gesture{ID: "plugin-filter-dismiss", OnTap: props.FilterPanel.OnDismiss, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: panelLeft, Top: 64, Child: PluginFilterPanel(panelProps)},
	}}
}

// PluginListItem contains one rendered plugin catalog entry.
type PluginListItem struct {
	ID                string
	Name              string
	Status            string
	Badge             string
	ShowInstalledIcon bool
	Icon              *woxui.Image
	FallbackColor     woxui.Color
	Selected          bool
	Highlighted       bool
	OnSelect          func()
}

// PluginListProps contains plugin catalog data and search state.
type PluginListProps struct {
	Width                 float32
	Height                float32
	Items                 []PluginListItem
	Message               string
	MessageError          bool
	Placeholder           string
	Search                woxui.TextEditingState
	Focused               bool
	Window                *woxui.Window
	FilterIcon            *woxui.Image
	RefreshIcon           *woxui.Image
	InstalledIcon         *woxui.Image
	InstalledSelectedIcon *woxui.Image
	FilterLabel           string
	RefreshLabel          string
	FilterActive          bool
	Refreshing            bool
	EmptyLabel            string
	EmptyTitle            string
	EmptyDescription      string
	EmptyIcon             *woxui.Image
	Theme                 woxcomponent.Theme
	OnClear               func()
	OnSearchKey           func(woxui.KeyEvent) bool
	OnSearchFocusChange   func(bool)
	OnSearchChanged       func(string)
	OnSetSearchValue      func(string) error
	OnFilter              func()
	OnRefresh             func()
}

// PluginList builds the searchable plugin catalog.
func PluginList(props PluginListProps) woxwidget.Widget {
	if props.Message != "" {
		color := props.Theme.ResultSubtitle
		if props.MessageError {
			color = props.Theme.ErrorText
		}
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: props.Message, Width: max(float32(0), props.Width-32), Height: max(float32(0), props.Height-32), Style: woxui.TextStyle{Size: 12}, Color: color,
		}}
	}

	const headerHeight = float32(62)
	const rowHeight = float32(62)
	viewportHeight := max(float32(0), props.Height-headerHeight)
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
		border := woxui.Color{}
		if item.Highlighted {
			border = props.Theme.SelectedBackground
			border.A = 122
			if !item.Selected {
				background = props.Theme.SelectedBackground
				background.A = 41
			}
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: item.FallbackColor}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 32, Height: 32, Fit: woxwidget.ImageFitContain}
		}
		textWidth := max(float32(0), props.Width-12-32-10)
		rowChildren := []woxwidget.Widget{icon}
		if item.Badge != "" {
			textWidth = max(float32(0), textWidth-10-44)
		}
		if item.ShowInstalledIcon {
			textWidth = max(float32(0), textWidth-10-26)
		}
		rowChildren = append(rowChildren, woxwidget.Container{Width: textWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 15}, Color: titleColor},
			woxwidget.Text{Value: item.Status, Style: woxui.TextStyle{Size: 12}, Color: subtitleColor},
		}}})
		if item.Badge != "" {
			badgeColor := props.Theme.ResultSubtitle
			if item.Selected {
				badgeColor = props.Theme.ActionSelectedText
			}
			badge := woxcomponent.WoxTag(item.Badge, badgeColor)
			rowChildren = append(rowChildren, woxwidget.Align{Width: 44, Height: 44, Horizontal: 1, Vertical: 0.5, Child: badge})
		}
		if item.ShowInstalledIcon {
			installedIcon := props.InstalledIcon
			if item.Selected {
				installedIcon = props.InstalledSelectedIcon
			}
			rowChildren = append(rowChildren, woxwidget.Align{Width: 26, Height: 44, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: installedIcon, Width: 20, Height: 20}})
		}
		radius := float32(4)
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: "plugin-list-" + item.ID, Label: item.Name, Width: props.Width, Height: rowHeight, Radius: &radius,
			Background: &background, BorderColor: border, BorderWidth: 1, Selected: item.Selected, OnTap: item.OnSelect, Theme: props.Theme,
			Padding: woxwidget.Insets{Left: 6, Top: 9, Right: 6, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: rowChildren},
		}))
	}

	var list woxwidget.Widget
	if len(rows) == 0 {
		title := props.EmptyTitle
		description := props.EmptyDescription
		if title == "" && description == "" {
			title = props.EmptyLabel
		}
		list = CatalogListEmptyState(CatalogListEmptyProps{
			Width: props.Width, Height: viewportHeight, Title: title, Description: description,
			Icon: props.EmptyIcon, Window: props.Window, Theme: props.Theme,
		})
	} else {
		var keepVisible *woxwidget.ScrollRange
		for index, item := range props.Items {
			if item.Selected {
				start := float32(index) * rowHeight
				keepVisible = &woxwidget.ScrollRange{Start: start, End: start + rowHeight}
				break
			}
		}
		list = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "plugin-list-scroll", Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, Width: props.Width, Height: viewportHeight,
			KeepVisible: keepVisible, ThumbColor: props.Theme.ResultSubtitle,
		})
	}
	searchFieldWidth := max(float32(80), props.Width)
	searchField := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "plugin-search", Label: props.Placeholder, Width: searchFieldWidth, Value: props.Search.Text, Focused: props.Focused, Autofocus: props.Focused,
		Actions: []woxcomponent.SearchFieldAction{
			{ID: "plugin-filter", Label: props.FilterLabel, Icon: props.FilterIcon, Active: props.FilterActive, OnTap: props.OnFilter},
			{ID: "plugin-refresh", Label: props.RefreshLabel, Icon: props.RefreshIcon, Disabled: props.Refreshing, OnTap: props.OnRefresh},
		},
		Window: props.Window, Theme: props.Theme, OnClear: props.OnClear, OnKey: props.OnSearchKey,
		OnFocusChange: props.OnSearchFocusChange, OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearchValue,
	})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 20, Children: []woxwidget.Widget{searchField, list}}}
}

// PluginFilterOption describes one advanced catalog filter.
type PluginFilterOption struct {
	ID    string
	Label string
	Value bool
}

// PluginFilterPanelProps contains the anchored advanced-filter surface.
type PluginFilterPanelProps struct {
	Width        float32
	LabelWidth   float32
	RuntimeTitle string
	Options      []PluginFilterOption
	Runtimes     []PluginFilterOption
	Theme        woxcomponent.Theme
	OnToggle     func(string)
	OnDismiss    func()
}

// PluginFilterPanel builds the catalog filter popover above the split view.
func PluginFilterPanel(props PluginFilterPanelProps) woxwidget.Widget {
	const rowHeight = float32(18)
	const rowGap = float32(10)
	innerWidth := max(float32(0), props.Width-28)
	labelWidth := min(max(float32(50), props.LabelWidth), float32(180))
	rows := make([]woxwidget.Widget, 0, len(props.Options)+1)
	for _, option := range props.Options {
		rows = append(rows, pluginFilterRow(option, labelWidth, rowHeight, props))
	}
	runtimeOptions := make([]woxwidget.Widget, 0, len(props.Runtimes))
	for _, option := range props.Runtimes {
		runtimeOptions = append(runtimeOptions, pluginRuntimeFilterOption(option, rowHeight, props))
	}
	runtimeWidth := max(float32(0), innerWidth-labelWidth-10)
	rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: rowHeight, Child: woxwidget.Text{Value: props.RuntimeTitle, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle}},
		woxwidget.ScrollView{Key: "plugin-filter-runtime-scroll", Width: runtimeWidth, Height: rowHeight, Horizontal: true, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: runtimeOptions}},
	}})
	height := float32(24) + float32(len(rows))*rowHeight + float32(max(0, len(rows)-1))*rowGap
	background := props.Theme.Background
	background.A = 255
	return woxwidget.FocusScope{Key: "plugin-filter-panel", Modal: true, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: 8, Color: background, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: rowGap, Children: rows},
	}}
}

func pluginFilterRow(option PluginFilterOption, labelWidth, height float32, props PluginFilterPanelProps) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: height, Child: woxwidget.Text{Value: option.Label, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle}},
		woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{ID: "plugin-filter-" + option.ID, Label: option.Label, Value: option.Value, OnChange: func(bool) { props.OnToggle(option.ID) }, Theme: props.Theme}),
	}}
}

func pluginRuntimeFilterOption(option PluginFilterOption, height float32, props PluginFilterPanelProps) woxwidget.Widget {
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{ID: "plugin-filter-" + option.ID, Label: option.Label, Value: option.Value, OnChange: func(bool) { props.OnToggle(option.ID) }, Theme: props.Theme}),
		woxwidget.Container{Height: height, Child: woxwidget.Text{Value: option.Label, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultTitle}},
	}}
}

// PluginAction describes one plugin management or metadata action.
type PluginAction struct {
	ID      string
	Label   string
	Icon    *woxui.Image
	Width   float32
	Enabled bool
	Primary bool
	OnTap   func()
}

// PluginHeaderProps contains the selected plugin identity and actions.
type PluginHeaderProps struct {
	Name            string
	Version         string
	Author          string
	Icon            *woxui.Image
	FallbackColor   woxui.Color
	MetadataActions []PluginAction
	Management      []PluginAction
}

// PluginMetadataItem contains one metadata title and description pair.
type PluginMetadataItem struct {
	Title       string
	Description string
}

// PluginMetadataProps contains one non-editing detail tab.
type PluginMetadataProps struct {
	DescriptionOnly  bool
	Description      string
	Header           string
	Items            []PluginMetadataItem
	EmptyTitle       string
	EmptyDescription string
}

// PluginFormProps contains the shared form rows and scroll actions.
type PluginFormProps struct {
	Rows             []woxwidget.Widget
	KeepVisibleKey   woxwidget.Key
	Intro            string
	IntroIcon        *woxui.Image
	IntroAccent      woxui.Color
	EmptyTitle       string
	EmptyDescription string
}

// PluginEditorProps contains the selected plugin detail and editable state.
type PluginEditorProps struct {
	Header            PluginHeaderProps
	ActiveTab         string
	Tabs              []PluginTab
	DescriptionDetail *PluginStoreDetailProps
	Metadata          *PluginMetadataProps
	Form              *PluginFormProps
	OnSelectTab       func(string)
}

// PluginStoreDetailProps contains the store-only plugin detail page.
type PluginStoreDetailProps struct {
	Name              string
	Version           string
	Author            string
	Description       string
	Runtime           string
	WebsiteLabel      string
	WebsiteChipLabel  string
	Icon              *woxui.Image
	ExternalIcon      *woxui.Image
	RuntimeIcon       *woxui.Image
	WebsiteIcon       *woxui.Image
	FallbackColor     woxui.Color
	Management        []PluginAction
	ActiveTab         string
	Tabs              []PluginTab
	TabForm           *PluginFormProps
	Metadata          *PluginMetadataProps
	Screenshot        *woxui.Image
	ScreenshotLoading bool
	Error             string
	OnWebsite         func()
	OnScreenshot      func()
	OnSelectTab       func(string)
}

// PluginDetailProps selects the empty, store, or editable detail view.
type PluginDetailProps struct {
	Width      float32
	Height     float32
	EmptyLabel string
	Store      *PluginStoreDetailProps
	Editor     *PluginEditorProps
	Theme      woxcomponent.Theme
}

// PluginDetail builds the selected plugin detail route.
func PluginDetail(props PluginDetailProps) woxwidget.Widget {
	if props.Store != nil {
		return pluginStoreDetail(*props.Store, props.Width, props.Height, props.Theme)
	}
	if props.Editor != nil {
		return pluginEditor(*props.Editor, props.Width, props.Height, props.Theme)
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(24), Child: woxwidget.Text{
		Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
	}}
}

// PluginDetailTabBodyProps selects the shared description, form, or metadata body for one plugin tab.
type PluginDetailTabBodyProps struct {
	ActiveTab   string
	Description *PluginStoreDetailProps
	Form        *PluginFormProps
	Metadata    *PluginMetadataProps
	Width       float32
	Height      float32
	ScrollID    string
	Theme       woxcomponent.Theme
}

// pluginDetailTabBody renders one plugin detail tab through the shared form or metadata surfaces.
func pluginDetailTabBody(props PluginDetailTabBodyProps) woxwidget.Widget {
	if props.ActiveTab == "description" && props.Description != nil {
		// Keep description flush with trigger-keyword/form tabs; the outer plugin detail
		// already owns the shared 16px inset, so an extra 24px horizontal pad only skewed this tab.
		return pluginStoreDescription(*props.Description, props.Width, props.Height, props.Theme)
	}
	if props.Form != nil {
		return pluginFormTabBody(props.Form, props.Width, props.Height, props.ScrollID, props.Theme)
	}
	if props.Metadata != nil {
		return pluginMetadataTab(*props.Metadata, props.Width, props.Height, props.ScrollID, props.Theme)
	}
	return nil
}

// pluginEditor composes the shared identity, tabs, metadata, and auto-saving form body.
func pluginEditor(props PluginEditorProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := height
	const headerHeight = float32(112)
	const tabHeight = float32(44)
	header := pluginDetailHeader(props.Header, innerWidth, headerHeight, theme)
	tabs := PluginTabs(PluginTabsProps{Width: innerWidth, Height: tabHeight, Active: props.ActiveTab, Tabs: props.Tabs, Theme: theme, OnSelect: props.OnSelectTab})
	children := []woxwidget.Widget{header, tabs}
	bodyHeight := max(float32(48), innerHeight-headerHeight-tabHeight)
	children = append(children, pluginDetailTabBody(PluginDetailTabBodyProps{
		ActiveTab: props.ActiveTab, Description: props.DescriptionDetail, Form: props.Form, Metadata: props.Metadata,
		Width: innerWidth, Height: bodyHeight, ScrollID: "plugin-detail-" + props.ActiveTab, Theme: theme,
	}))
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 16, Right: 16}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// pluginFormTabBody renders the shared hint box and scrollable form rows used by plugin detail tabs.
func pluginFormTabBody(form *PluginFormProps, width, height float32, scrollID string, theme woxcomponent.Theme) woxwidget.Widget {
	if form == nil {
		return nil
	}
	if len(form.Rows) == 0 {
		return pluginEmptySettings(form.EmptyTitle, form.EmptyDescription, width, height, theme)
	}
	formRows := form.Rows
	if form.Intro != "" {
		intro := woxcomponent.WoxHintBox(woxcomponent.HintBoxProps{
			Text: form.Intro, Width: width, MaxLines: 2, Icon: form.IntroIcon, Accent: form.IntroAccent, Theme: theme,
		})
		formRows = append([]woxwidget.Widget{intro, woxwidget.Container{Height: 6}}, formRows...)
	}
	return woxwidget.ScrollView{
		Key: woxwidget.Key(scrollID), ID: scrollID, Width: width, Height: height,
		KeepVisibleKey: form.KeepVisibleKey,
		Child:          woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: formRows}},
	}
}

func pluginDetailHeader(props PluginHeaderProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 32, Height: 32, Fit: woxwidget.ImageFitContain}
	}
	actionsWidth := float32(0)
	for index, action := range props.MetadataActions {
		actionsWidth += action.Width
		if index > 0 {
			actionsWidth += 4
		}
	}
	identity := woxwidget.Expanded{Child: woxwidget.Container{
		Height: 40,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: theme.QueryText},
			woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle},
		}},
	}}
	author := woxwidget.Expanded{Child: woxwidget.Container{
		Height: 30, Padding: woxwidget.Insets{Left: 8, Top: 7},
		Child: woxwidget.Text{Value: props.Author, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle},
	}}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{icon, identity}},
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			author,
			woxwidget.Container{Width: actionsWidth, Height: 30, Child: pluginTextActions(props.MetadataActions, theme)},
		}},
		woxwidget.Container{Width: width, Height: 42, Padding: woxwidget.Insets{Left: 8, Top: 3}, Child: pluginOutlineActions(props.Management, theme)},
	}}}
}

// pluginEmptySettings matches Flutter's centered empty-state hierarchy without
// coupling the plugin view to a platform icon.
func pluginEmptySettings(title, description string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	contentWidth := min(float32(430), max(float32(0), width-32))
	children := []woxwidget.Widget{
		catalogCenteredText(contentWidth, title, woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, theme.ResultTitle),
	}
	if description != "" {
		children = append(children, woxwidget.TextBlock{
			Value: description, Width: contentWidth, MaxLines: 2, LineHeight: 19, Centered: true,
			Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle,
		})
	}
	content := woxwidget.Container{
		Width: contentWidth,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
	}
	return woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.45, Child: content}
}

// PluginTab contains one plugin detail destination.
type PluginTab struct {
	ID    string
	Label string
	Width float32
}

// PluginTabsProps contains the available tabs and selection action.
type PluginTabsProps struct {
	Width    float32
	Height   float32
	Active   string
	Tabs     []PluginTab
	Theme    woxcomponent.Theme
	OnSelect func(string)
}

// PluginTabs builds the plugin detail tab strip.
func PluginTabs(props PluginTabsProps) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(props.Tabs))
	for _, tab := range props.Tabs {
		underline := woxui.Color{}
		color := props.Theme.ResultTitle
		if tab.ID == props.Active {
			underline = props.Theme.Cursor
			color = props.Theme.QueryText
		}
		indicatorWidth := max(float32(32), tab.Width-24)
		children = append(children, woxwidget.Gesture{ID: "plugin-detail-tab-" + tab.ID, OnTap: func() {
			if props.OnSelect != nil {
				props.OnSelect(tab.ID)
			}
		}, Child: woxwidget.Container{Width: tab.Width, Height: props.Height - 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Align{Width: tab.Width, Height: props.Height - 3, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: tab.Label, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: color}},
			woxwidget.Align{Width: tab.Width, Height: 2, Horizontal: 0.5, Child: woxwidget.Container{Width: indicatorWidth, Height: 2, Color: underline}},
		}}}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
		woxwidget.Container{Width: props.Width, Height: 1, Color: props.Theme.PreviewSplit},
	}}}
}

// pluginMetadataTab renders description, empty, or tabular metadata in one scroll surface.
func pluginMetadataTab(props PluginMetadataProps, width, height float32, scrollID string, theme woxcomponent.Theme) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Items)+1)
	if props.DescriptionOnly {
		rows = append(rows, woxwidget.TextBlock{Value: props.Description, Width: width, Height: max(float32(100), height-30), MaxLines: 20, Style: woxui.TextStyle{Size: 13}, LineHeight: 21, Color: theme.ResultSubtitle})
	} else if props.EmptyTitle != "" {
		return pluginEmptySettings(props.EmptyTitle, props.EmptyDescription, width, height, theme)
	} else {
		if props.Header != "" {
			rows = append(rows, woxwidget.Container{Width: width, Height: 46, Padding: woxwidget.Insets{Top: 16}, Child: woxwidget.Text{Value: props.Header, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}})
		}
		for _, item := range props.Items {
			rows = append(rows, pluginMetadataRow(item, width, theme))
		}
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 18}, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key(scrollID), FillWidth: true, FillHeight: true,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: theme.ResultSubtitle,
	})}
}

func pluginMetadataRow(item PluginMetadataItem, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width * 0.32, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 18, Right: 8}, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
			woxwidget.Container{Width: width * 0.68, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 14, Right: 8}, Child: woxwidget.TextBlock{
				Value: item.Description, MaxLines: 2, LineHeight: 16, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle,
			}},
		}},
		woxwidget.Container{Width: width, Height: 1, Color: theme.PreviewSplit},
	}}}
}

// pluginStoreDetail mirrors the identity, actions, tabs, and content hierarchy of the Flutter store route.
func pluginStoreDetail(props PluginStoreDetailProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := max(float32(0), height-24)
	var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 32, Height: 32, Fit: woxwidget.ImageFitContain}
	}
	const headerHeight = float32(120)
	const tabHeight = float32(44)
	identity := woxwidget.Container{Width: innerWidth, Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		icon,
		woxwidget.Expanded{Child: woxwidget.Container{Height: 38, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 20}, Color: theme.QueryText},
			woxwidget.Container{Height: 25, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}},
		}}}},
	}}}
	websiteWidth := float32(104)
	var website woxwidget.Widget = woxwidget.Container{Width: websiteWidth, Height: 28}
	if props.WebsiteLabel != "" && props.OnWebsite != nil {
		website = woxwidget.Gesture{ID: "plugin-website", OnTap: props.OnWebsite, Child: woxwidget.Align{Width: websiteWidth, Height: 28, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Image{Source: props.ExternalIcon, Width: 13, Height: 13},
			woxwidget.Text{Value: props.WebsiteLabel, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultTitle},
		}}}}
	}
	header := woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		identity,
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Expanded{Child: woxwidget.Container{Height: 28, Padding: woxwidget.Insets{Left: 6, Top: 6}, Child: woxwidget.Text{Value: props.Author, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}}},
			website,
		}},
		woxwidget.Container{Width: innerWidth, Height: 52, Padding: woxwidget.Insets{Left: 6, Top: 6}, Child: pluginOutlineActions(props.Management, theme)},
	}}}
	tabs := PluginTabs(PluginTabsProps{Width: innerWidth, Height: tabHeight, Active: props.ActiveTab, Tabs: props.Tabs, Theme: theme, OnSelect: props.OnSelectTab})
	bodyHeight := max(float32(1), innerHeight-headerHeight-tabHeight)
	description := (*PluginStoreDetailProps)(nil)
	if props.ActiveTab == "description" {
		description = &props
	}
	body := pluginDetailTabBody(PluginDetailTabBodyProps{
		ActiveTab: props.ActiveTab, Description: description, Form: props.TabForm, Metadata: props.Metadata,
		Width: innerWidth, Height: bodyHeight, ScrollID: "plugin-detail-" + props.ActiveTab, Theme: theme,
	})
	children := []woxwidget.Widget{header, tabs, body}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 44, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, Color: theme.ErrorText})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 16, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// pluginStoreDescription renders the description metadata and the first manifest screenshot.
func pluginStoreDescription(props PluginStoreDetailProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	const topPadding = float32(24)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 30, Child: woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
		woxwidget.TextBlock{Value: props.Description + " · " + props.Author, Width: width, Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: theme.ResultTitle},
		woxwidget.Container{Width: width, Height: 42, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			pluginStoreChip("v"+props.Version, nil, nil, theme),
			pluginStoreChip(props.Runtime, props.RuntimeIcon, nil, theme),
			pluginStoreChip(props.WebsiteChipLabel, props.WebsiteIcon, props.OnWebsite, theme),
		}}},
	}
	if shot := pluginStoreScreenshot(props, width, theme); shot != nil {
		// Match Flutter's SizedBox(height: 24) between metadata chips and the screenshot.
		children = append(children, woxwidget.Container{Height: 24}, shot)
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: topPadding}, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "plugin-store-description-scroll", FillWidth: true, FillHeight: true,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}, ThumbColor: theme.ResultSubtitle,
	})}
}

// pluginStoreScreenshot sizes the manifest preview to the description width so URL images keep their aspect ratio.
// While the URL is still resolving, Flutter only shows a compact spinner instead of a blank panel that later pops in.
func pluginStoreScreenshot(props PluginStoreDetailProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	if props.Screenshot != nil && props.Screenshot.Width > 0 && props.Screenshot.Height > 0 {
		screenshotHeight := width * float32(props.Screenshot.Height) / float32(props.Screenshot.Width)
		return woxwidget.Gesture{ID: "plugin-store-screenshot", OnTap: props.OnScreenshot, Child: woxwidget.Container{
			Width: width, Height: screenshotHeight, Radius: 8,
			Child: woxwidget.Image{Source: props.Screenshot, Width: width, Height: screenshotHeight, Fit: woxwidget.ImageFitContain},
		}}
	}
	if !props.ScreenshotLoading {
		return nil
	}
	return woxwidget.Align{
		Width: width, Height: 48, Horizontal: 0.5, Vertical: 0.5,
		Child: woxcomponent.WoxLoadingIndicator(24, theme.Cursor),
	}
}

// pluginStoreChip keeps version, runtime, and source metadata visually consistent.
func pluginStoreChip(label string, icon *woxui.Image, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	if label == "" {
		return nil
	}
	width := max(float32(58), float32(len([]rune(label)))*7+24)
	children := make([]woxwidget.Widget, 0, 2)
	if icon != nil {
		children = append(children, woxwidget.Image{Source: icon, Width: 14, Height: 14, Fit: woxwidget.ImageFitContain})
		width += 18
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultTitle})
	return woxwidget.Gesture{ID: "plugin-store-chip-" + label, OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 28, Radius: 7, Color: theme.ActionBackground, BorderColor: theme.ResultSubtitle, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 10, Right: 8}, Child: woxwidget.Align{Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
	}}
}

// pluginOutlineActions matches the compact store management controls used by the Flutter route.
func pluginOutlineActions(actions []PluginAction, theme woxcomponent.Theme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Icon: action.Icon, IconSize: 14, IntrinsicWidth: true, Height: 36, Radius: 4, FontSize: 13,
			Disabled: !action.Enabled, Variant: woxcomponent.ButtonOutline, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}

// pluginTextActions renders lightweight metadata links separately from lifecycle controls.
func pluginTextActions(actions []PluginAction, theme woxcomponent.Theme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Icon: action.Icon, IconSize: 13, IconGap: 6, Height: 30, FontSize: 12,
			Padding: woxwidget.Insets{Left: 6, Right: 4}, Disabled: !action.Enabled, Variant: woxcomponent.ButtonText, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: buttons}
}

func pluginActions(actions []PluginAction, theme woxcomponent.Theme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		variant := woxcomponent.ButtonSecondary
		if action.Primary {
			variant = woxcomponent.ButtonPrimary
		}
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Icon: action.Icon, Disabled: !action.Enabled, Variant: variant, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}
