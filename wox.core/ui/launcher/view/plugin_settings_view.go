package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PluginSettingsPageProps contains the data required by the plugin list and detail views.
type PluginSettingsPageProps struct {
	Width  float32
	Height float32
	List   PluginListProps
	Detail PluginDetailProps
	Theme  woxcomponent.Theme
}

// PluginSettingsPage builds the split plugin management route.
func PluginSettingsPage(props PluginSettingsPageProps) woxwidget.Widget {
	innerHeight := max(float32(0), props.Height-24)
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.List.Width, Height: innerHeight, Child: PluginList(props.List)},
		woxwidget.Container{Width: 1, Height: innerHeight, Color: props.Theme.PreviewSplit},
		woxwidget.Container{Width: props.Detail.Width, Height: innerHeight, Child: PluginDetail(props.Detail)},
	}}}
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
	Width        float32
	Height       float32
	Items        []PluginListItem
	Message      string
	MessageError bool
	Scroll       float32
	Placeholder  string
	Search       woxui.TextEditingState
	Focused      bool
	Window       *woxui.Window
	EmptyLabel   string
	Theme        woxcomponent.Theme
	OnViewport   func(float32)
	OnScroll     func(float32)
	OnCaret      func(int)
	OnClear      func()
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
	if props.OnViewport != nil {
		props.OnViewport(viewportHeight)
	}
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		item := item
		background := woxui.Color{}
		titleColor := props.Theme.ResultTitle
		if item.Selected {
			background = props.Theme.SelectedBackground
			titleColor = props.Theme.SelectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 36, Height: 36, Radius: 8, Color: item.FallbackColor}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 36, Height: 36}
		}
		textWidth := max(float32(0), props.Width-80)
		rowChildren := []woxwidget.Widget{icon}
		if item.Badge != "" {
			textWidth = max(float32(0), props.Width-134)
		}
		rowChildren = append(rowChildren, woxwidget.Container{Width: textWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: item.Status, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
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
		list = woxwidget.Gesture{ID: "plugin-list-scroll", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: props.Width - 16, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*rowHeight), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	clearWidth := float32(0)
	if props.Search.Text != "" {
		clearWidth = 24
	}
	searchWidth := max(float32(40), props.Width-70)
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "plugin-search", Label: props.Placeholder, Hint: props.Placeholder, Width: searchWidth, Height: 40,
		Padding: woxwidget.Insets{Left: 2, Right: 6}, Transparent: true, Style: woxui.TextStyle{Size: 13}, State: props.Search,
		Focused: props.Focused, MaxLines: 1, Window: props.Window, Theme: props.Theme, ControllerManagedFocus: true, OnCaret: props.OnCaret,
	})
	searchChildren := []woxwidget.Widget{
		woxwidget.Container{Width: 30, Height: 42, Padding: woxwidget.Insets{Left: 9, Top: 11}, Child: woxwidget.Text{Value: "⌕", Style: woxui.TextStyle{Size: 17}, Color: props.Theme.ResultSubtitle}},
		search,
	}
	if clearWidth > 0 {
		searchChildren = append(searchChildren, woxwidget.Gesture{ID: "plugin-search-clear", OnTap: props.OnClear, Child: woxwidget.Container{Width: clearWidth, Height: 42, Padding: woxwidget.Insets{Left: 5, Top: 10}, Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 16}, Color: props.Theme.ResultSubtitle}}})
	}
	searchField := woxwidget.Container{Width: props.Width - 16, Height: 44, Radius: 6, Color: props.Theme.QueryBackground, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: searchChildren}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{searchField, list}}}
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
	Scroll        float32
	OnViewport    func(float32)
	OnScroll      func(float32)
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

// PluginStoreDetailProps contains the store-only plugin detail card.
type PluginStoreDetailProps struct {
	Name            string
	Subtitle        string
	Description     string
	Icon            *woxui.Image
	FallbackColor   woxui.Color
	MetadataActions []PluginAction
	Management      []PluginAction
	Error           string
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
	const tabHeight = float32(46)
	header := pluginDetailHeader(props.Header, innerWidth, headerHeight, theme)
	tabs := PluginTabs(PluginTabsProps{Width: innerWidth, Height: tabHeight, Active: props.ActiveTab, Tabs: props.Tabs, Theme: theme, OnSelect: props.OnSelectTab})
	children := []woxwidget.Widget{header, tabs}
	if props.Metadata != nil {
		children = append(children, pluginMetadataTab(*props.Metadata, innerWidth, max(float32(0), innerHeight-headerHeight-tabHeight), theme))
	} else if props.Form != nil {
		statusHeight := float32(0)
		if props.Status != "" {
			statusHeight = 28
		}
		const footerHeight = float32(48)
		bodyHeight := max(float32(48), innerHeight-headerHeight-tabHeight-footerHeight-statusHeight)
		if props.Form.OnViewport != nil {
			props.Form.OnViewport(bodyHeight)
		}
		children = append(children, woxwidget.Gesture{ID: "plugin-settings-scroll", OnScroll: func(delta woxui.Point) {
			if props.Form.OnScroll != nil {
				props.Form.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: bodyHeight, ContentHeight: max(bodyHeight, props.Form.ContentHeight), Offset: props.Form.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Form.Rows},
		}})
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
		color := props.Theme.ResultSubtitle
		if tab.ID == props.Active {
			underline = props.Theme.Cursor
			color = props.Theme.QueryText
		}
		children = append(children, woxwidget.Gesture{ID: "plugin-detail-tab-" + tab.ID, OnTap: func() {
			if props.OnSelect != nil {
				props.OnSelect(tab.ID)
			}
		}, Child: woxwidget.Container{Width: tab.Width, Height: props.Height - 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: tab.Width, Height: props.Height - 3, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{Value: tab.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: color}},
			woxwidget.Container{Width: tab.Width, Height: 2, Color: underline},
		}}}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
		woxwidget.Container{Width: props.Width, Height: 1, Color: props.Theme.PreviewSplit},
	}}}
}

// pluginMetadataTab renders description, empty, or tabular metadata in one scroll surface.
func pluginMetadataTab(props PluginMetadataProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
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
		Width: width, Height: max(float32(1), height-18), ContentHeight: max(height-24, contentHeight), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
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

// pluginStoreDetail renders metadata and management actions before a plugin has settings.
func pluginStoreDetail(props PluginStoreDetailProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	innerWidth := max(float32(0), width-36)
	var icon woxwidget.Widget = woxwidget.Container{Width: 54, Height: 54, Radius: 12, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 54, Height: 54}
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.ActionBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 16, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
				icon,
				woxwidget.Container{Width: max(float32(0), innerWidth-68), Height: 60, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
					woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: theme.QueryText},
					woxwidget.Text{Value: props.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
				}}},
			}},
			woxwidget.TextBlock{Value: props.Description, Width: innerWidth, Height: 120, MaxLines: 6, Style: woxui.TextStyle{Size: 12}, LineHeight: 19, Color: theme.ResultSubtitle},
			pluginActions(props.MetadataActions, theme),
			pluginActions(props.Management, theme),
			woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 60, MaxLines: 3, Style: woxui.TextStyle{Size: 11}, Color: theme.ErrorText},
		},
	}}
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
