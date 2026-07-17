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
	innerHeight := max(float32(0), props.Height-24)
	content := woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.List.Width, Height: innerHeight, Child: PluginList(props.List)},
		woxwidget.Container{Width: 1, Height: innerHeight, Color: props.Theme.PreviewSplit},
		woxwidget.Container{Width: props.Detail.Width, Height: innerHeight, Child: PluginDetail(props.Detail)},
	}}}
	if props.FilterPanel == nil {
		return content
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: content},
		{Child: woxwidget.Gesture{ID: "plugin-filter-dismiss", OnTap: props.FilterPanel.OnDismiss, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: 28, Top: 66, Child: PluginFilterPanel(*props.FilterPanel)},
	}}
}

// PluginListItem contains one rendered plugin catalog entry.
type PluginListItem struct {
	ID            string
	Name          string
	Status        string
	Badge         string
	Icon          *woxui.Image
	FallbackColor woxui.Color
	Selected      bool
	OnSelect      func()
}

// PluginListProps contains plugin catalog data and search state.
type PluginListProps struct {
	Width               float32
	Height              float32
	Items               []PluginListItem
	Message             string
	MessageError        bool
	Placeholder         string
	Search              woxui.TextEditingState
	Focused             bool
	Window              *woxui.Window
	FilterIcon          *woxui.Image
	RefreshIcon         *woxui.Image
	FilterActive        bool
	Refreshing          bool
	EmptyLabel          string
	Theme               woxcomponent.Theme
	OnClear             func()
	OnSearchKey         func(woxui.KeyEvent) bool
	OnSearchFocusChange func(bool)
	OnSearchChanged     func(string)
	OnSetSearchValue    func(string) error
	OnFilter            func()
	OnRefresh           func()
}

// PluginList builds the searchable plugin catalog.
func PluginList(props PluginListProps) woxwidget.Widget {
	if props.Message != "" {
		color := props.Theme.ResultSubtitle
		if props.MessageError {
			color = props.Theme.ErrorText
		}
		return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: props.Message, Width: max(float32(0), props.Width-32), Height: max(float32(0), props.Height-32), Style: woxui.TextStyle{Size: 12}, Color: color,
		}}
	}

	const headerHeight = float32(58)
	const rowHeight = float32(62)
	viewportHeight := max(float32(0), props.Height-headerHeight)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		item := item
		background := woxui.Color{}
		titleColor := props.Theme.ResultTitle
		if item.Selected {
			background = props.Theme.SelectedBackground
			titleColor = props.Theme.SelectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: item.FallbackColor}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 32, Height: 32}
		}
		textWidth := max(float32(0), props.Width-80)
		rowChildren := []woxwidget.Widget{icon}
		if item.Badge != "" {
			textWidth = max(float32(0), props.Width-134)
		}
		rowChildren = append(rowChildren, woxwidget.Container{Width: textWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: item.Status, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		}}})
		if item.Badge != "" {
			badgeColor := props.Theme.ToolbarBackground
			badgeColor.A = 210
			rowChildren = append(rowChildren, woxwidget.Container{Width: 44, Height: 22, Radius: 5, Color: badgeColor, Padding: woxwidget.Insets{Left: 7, Top: 4}, Child: woxwidget.Text{
				Value: item.Badge, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle,
			}})
		}
		rows = append(rows, woxwidget.Gesture{ID: "plugin-list-" + item.ID, OnTap: item.OnSelect, Child: woxwidget.Container{
			Width: props.Width - 16, Height: rowHeight, Radius: 6, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 8, Bottom: 8},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: rowChildren},
		}})
	}

	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: props.Width - 16, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18}, Child: woxwidget.Text{Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}}
	} else {
		var keepVisible *woxwidget.ScrollRange
		for index, item := range props.Items {
			if item.Selected {
				start := float32(index) * rowHeight
				keepVisible = &woxwidget.ScrollRange{Start: start, End: start + rowHeight}
				break
			}
		}
		list = woxwidget.ScrollView{
			Key: "plugin-list-scroll", ID: "plugin-list-scroll", Width: props.Width - 16, Height: viewportHeight,
			ContentHeight: max(viewportHeight, float32(len(rows))*rowHeight), KeepVisible: keepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}
	}
	searchFieldWidth := max(float32(80), props.Width-16)
	searchField := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "plugin-search", Label: props.Placeholder, Width: searchFieldWidth, Value: props.Search.Text, Focused: props.Focused, Autofocus: props.Focused,
		Actions: []woxcomponent.SearchFieldAction{
			{ID: "plugin-filter", Icon: props.FilterIcon, Active: props.FilterActive, OnTap: props.OnFilter},
			{ID: "plugin-refresh", Icon: props.RefreshIcon, Disabled: props.Refreshing, OnTap: props.OnRefresh},
		},
		Window: props.Window, Theme: props.Theme, OnClear: props.OnClear, OnKey: props.OnSearchKey,
		OnFocusChange: props.OnSearchFocusChange, OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearchValue,
	})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{searchField, list}}}
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
	Title        string
	RuntimeTitle string
	Options      []PluginFilterOption
	Runtimes     []PluginFilterOption
	Theme        woxcomponent.Theme
	OnToggle     func(string)
	OnDismiss    func()
}

// PluginFilterPanel builds the catalog filter popover above the split view.
func PluginFilterPanel(props PluginFilterPanelProps) woxwidget.Widget {
	const rowHeight = float32(34)
	innerWidth := max(float32(0), props.Width-28)
	rows := make([]woxwidget.Widget, 0, len(props.Options)+len(props.Runtimes)+2)
	rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 30, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}})
	for _, option := range props.Options {
		option := option
		rows = append(rows, pluginFilterRow(option, innerWidth, rowHeight, props))
	}
	rows = append(rows, woxwidget.Container{Width: innerWidth, Height: 30, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{Value: props.RuntimeTitle, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}})
	for _, option := range props.Runtimes {
		option := option
		rows = append(rows, pluginFilterRow(option, innerWidth, rowHeight, props))
	}
	height := float32(28) + float32(len(rows))*rowHeight
	return woxwidget.FocusScope{Key: "plugin-filter-panel", Modal: true, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: 8, Color: props.Theme.Background, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
}

func pluginFilterRow(option PluginFilterOption, width, height float32, props PluginFilterPanelProps) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), width-54), Height: height, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{Value: option.Label, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultTitle}},
		woxwidget.Container{Width: 54, Height: height, Padding: woxwidget.Insets{Top: 6}, Child: woxcomponent.WoxSwitch(woxcomponent.SwitchProps{ID: "plugin-filter-" + option.ID, Label: option.Label, Value: option.Value, OnChange: func(bool) { props.OnToggle(option.ID) }, Theme: props.Theme})},
	}}}
}

// PluginAction describes one plugin management or metadata action.
type PluginAction struct {
	ID      string
	Label   string
	Width   float32
	Enabled bool
	Primary bool
	OnTap   func()
}

// PluginHeaderProps contains the selected plugin identity and actions.
type PluginHeaderProps struct {
	Title           string
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
	DescriptionOnly bool
	Description     string
	Header          string
	Items           []PluginMetadataItem
	EmptyLabel      string
}

// PluginFormProps contains the shared form rows and scroll actions.
type PluginFormProps struct {
	Rows          []woxwidget.Widget
	ContentHeight float32
	KeepVisible   *woxwidget.ScrollRange
}

// PluginEditorProps contains the selected plugin detail and editable state.
type PluginEditorProps struct {
	Header        PluginHeaderProps
	ActiveTab     string
	Tabs          []PluginTab
	Metadata      *PluginMetadataProps
	Form          *PluginFormProps
	Status        string
	StatusError   bool
	SaveLabel     string
	SaveHighlight bool
	OnSelectTab   func(string)
	OnSave        func()
}

// PluginStoreDetailProps contains the store-only plugin detail page.
type PluginStoreDetailProps struct {
	Name             string
	Version          string
	Author           string
	Description      string
	Runtime          string
	WebsiteLabel     string
	WebsiteChipLabel string
	Icon             *woxui.Image
	ExternalIcon     *woxui.Image
	RuntimeIcon      *woxui.Image
	WebsiteIcon      *woxui.Image
	FallbackColor    woxui.Color
	Management       []PluginAction
	ActiveTab        string
	Tabs             []PluginTab
	Metadata         PluginMetadataProps
	Screenshot       *woxui.Image
	ScreenshotHeight float32
	Error            string
	OnWebsite        func()
	OnScreenshot     func()
	OnSelectTab      func(string)
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

// pluginEditor composes the shared identity, tabs, metadata or form body, and save footer.
func pluginEditor(props PluginEditorProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-48)
	innerHeight := max(float32(0), height-24)
	const headerHeight = float32(104)
	const tabHeight = float32(48)
	header := pluginDetailHeader(props.Header, innerWidth, headerHeight, theme)
	tabs := PluginTabs(PluginTabsProps{Width: innerWidth, Height: tabHeight, Active: props.ActiveTab, Tabs: props.Tabs, Theme: theme, OnSelect: props.OnSelectTab})
	children := []woxwidget.Widget{header, tabs}
	if props.Metadata != nil {
		children = append(children, pluginMetadataTab(*props.Metadata, innerWidth, max(float32(0), innerHeight-headerHeight-tabHeight), "plugin-metadata-"+props.ActiveTab, theme))
	} else if props.Form != nil {
		statusHeight := float32(0)
		if props.Status != "" {
			statusHeight = 28
		}
		const footerHeight = float32(48)
		bodyHeight := max(float32(48), innerHeight-headerHeight-tabHeight-footerHeight-statusHeight)
		children = append(children, woxwidget.ScrollView{
			Key: "plugin-settings-scroll", ID: "plugin-settings-scroll", Width: innerWidth, Height: bodyHeight,
			ContentHeight: max(bodyHeight, props.Form.ContentHeight), KeepVisible: props.Form.KeepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Form.Rows},
		})
		if props.Status != "" {
			color := theme.ResultSubtitle
			if props.StatusError {
				color = theme.ErrorText
			}
			children = append(children, woxwidget.Container{Width: innerWidth, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 11}, Color: color}})
		}
		variant := woxcomponent.ButtonSelected
		if props.SaveHighlight {
			variant = woxcomponent.ButtonPrimary
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Painter{Width: max(float32(0), innerWidth-128), Height: footerHeight},
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "plugin-settings-save", Label: props.SaveLabel, Width: 128, Height: 36, Variant: variant, OnTap: props.OnSave, Theme: theme}),
		}})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 12, Right: 24, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

func pluginDetailHeader(props PluginHeaderProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	var icon woxwidget.Widget = woxwidget.Container{Width: 44, Height: 44, Radius: 10, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 44, Height: 44}
	}
	const actionsWidth = float32(224)
	identityWidth := max(float32(120), width-44-14-actionsWidth)
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
			icon,
			woxwidget.Container{Width: identityWidth, Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 19, Weight: woxui.FontWeightSemibold}, Color: theme.QueryText},
				woxwidget.Text{Value: props.Author, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle},
			}}},
			woxwidget.Container{Width: actionsWidth, Height: 58, Padding: woxwidget.Insets{Top: 4}, Child: pluginActions(props.MetadataActions, theme)},
		}},
		woxwidget.Container{Width: width, Height: 46, Child: pluginActions(props.Management, theme)},
	}}}
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
		tab := tab
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
	contentHeight := float32(0)
	if props.DescriptionOnly {
		contentHeight = max(float32(100), height-30)
		rows = append(rows, woxwidget.TextBlock{Value: props.Description, Width: width, Height: contentHeight, MaxLines: 20, Style: woxui.TextStyle{Size: 13}, LineHeight: 21, Color: theme.ResultSubtitle})
	} else if props.EmptyLabel != "" {
		contentHeight = 100
		rows = append(rows, woxwidget.Container{Width: width, Height: 100, Padding: woxwidget.Insets{Top: 26}, Child: woxwidget.Text{Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}})
	} else {
		if props.Header != "" {
			contentHeight += 46
			rows = append(rows, woxwidget.Container{Width: width, Height: 46, Padding: woxwidget.Insets{Top: 16}, Child: woxwidget.Text{Value: props.Header, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}})
		}
		for _, item := range props.Items {
			rows = append(rows, pluginMetadataRow(item, width, theme))
			contentHeight += 62
		}
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 18}, Child: woxwidget.ScrollView{
		Key: woxwidget.Key(scrollID), ID: scrollID, Width: width, Height: max(float32(1), height-18),
		ContentHeight: max(height-24, contentHeight), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
}

func pluginMetadataRow(item PluginMetadataItem, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width * 0.32, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 18, Right: 8}, Child: woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
			woxwidget.Container{Width: width * 0.68, Height: 61, Padding: woxwidget.Insets{Left: 8, Top: 18, Right: 8}, Child: woxwidget.Text{Value: item.Description, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}},
		}},
		woxwidget.Container{Width: width, Height: 1, Color: theme.PreviewSplit},
	}}}
}

// pluginStoreDetail mirrors the identity, actions, tabs, and content hierarchy of the Flutter store route.
func pluginStoreDetail(props PluginStoreDetailProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-48)
	innerHeight := max(float32(0), height-24)
	var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 32, Height: 32}
	}
	const headerHeight = float32(120)
	const tabHeight = float32(48)
	titleWidth := max(float32(100), innerWidth-52)
	identity := woxwidget.Container{Width: innerWidth, Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		icon,
		woxwidget.Container{Width: titleWidth, Height: 38, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 20}, Color: theme.QueryText},
			woxwidget.Container{Height: 25, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}},
		}}},
	}}}
	websiteWidth := float32(104)
	authorWidth := max(float32(0), innerWidth-websiteWidth)
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
			woxwidget.Container{Width: authorWidth, Height: 28, Padding: woxwidget.Insets{Left: 6, Top: 6}, Child: woxwidget.Text{Value: props.Author, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle}},
			website,
		}},
		woxwidget.Container{Width: innerWidth, Height: 52, Padding: woxwidget.Insets{Left: 6, Top: 6}, Child: pluginOutlineActions(props.Management, theme)},
	}}}
	tabs := PluginTabs(PluginTabsProps{Width: innerWidth, Height: tabHeight, Active: props.ActiveTab, Tabs: props.Tabs, Theme: theme, OnSelect: props.OnSelectTab})
	bodyHeight := max(float32(1), innerHeight-headerHeight-tabHeight)
	var body woxwidget.Widget
	if props.ActiveTab == "description" {
		body = pluginStoreDescription(props, innerWidth, bodyHeight, theme)
	} else {
		body = pluginMetadataTab(props.Metadata, innerWidth, bodyHeight, "plugin-store-metadata-"+props.ActiveTab, theme)
	}
	children := []woxwidget.Widget{header, tabs, body}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 44, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, Color: theme.ErrorText})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Right: 24, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// pluginStoreDescription renders the description metadata and the first manifest screenshot.
func pluginStoreDescription(props PluginStoreDetailProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	const topPadding = float32(22)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 30, Child: woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
		woxwidget.TextBlock{Value: props.Description + " · " + props.Author, Width: width, Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: theme.ResultTitle},
		woxwidget.Container{Width: width, Height: 42, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			pluginStoreChip("v"+props.Version, nil, nil, theme),
			pluginStoreChip(props.Runtime, props.RuntimeIcon, nil, theme),
			pluginStoreChip(props.WebsiteChipLabel, props.WebsiteIcon, props.OnWebsite, theme),
		}}},
	}
	contentHeight := float32(110)
	if props.Screenshot != nil && props.ScreenshotHeight > 0 {
		children = append(children, woxwidget.Gesture{ID: "plugin-store-screenshot", OnTap: props.OnScreenshot, Child: woxwidget.Container{
			Width: width, Height: props.ScreenshotHeight, Radius: 8, Child: woxwidget.Image{Source: props.Screenshot, Width: width, Height: props.ScreenshotHeight},
		}})
		contentHeight += props.ScreenshotHeight
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: topPadding}, Child: woxwidget.ScrollView{
		Key: "plugin-store-description-scroll", ID: "plugin-store-description-scroll", Width: width, Height: max(float32(1), height-topPadding),
		ContentHeight: max(height-topPadding, contentHeight), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}}
}

// pluginStoreChip keeps version, runtime, and source metadata visually consistent.
func pluginStoreChip(label string, icon *woxui.Image, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	if label == "" {
		return nil
	}
	width := max(float32(58), float32(len([]rune(label)))*7+24)
	children := make([]woxwidget.Widget, 0, 2)
	if icon != nil {
		children = append(children, woxwidget.Image{Source: icon, Width: 14, Height: 14})
		width += 18
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultTitle})
	return woxwidget.Gesture{ID: "plugin-store-chip-" + label, OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 28, Radius: 7, Color: theme.ActionBackground, BorderColor: theme.ResultSubtitle, BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 10, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
	}}
}

// pluginOutlineActions matches the compact store management controls used by the Flutter route.
func pluginOutlineActions(actions []PluginAction, theme woxcomponent.Theme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Width: action.Width, Height: 36, Radius: 4, FontSize: 13, Disabled: !action.Enabled, Variant: woxcomponent.ButtonOutline, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}

func pluginActions(actions []PluginAction, theme woxcomponent.Theme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		variant := woxcomponent.ButtonSecondary
		if action.Primary {
			variant = woxcomponent.ButtonPrimary
		}
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Width: action.Width, Disabled: !action.Enabled, Variant: variant, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}
}
