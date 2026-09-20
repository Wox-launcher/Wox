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
	Theme       woxcomponent.ControlTheme
}

// PluginSettingsPage builds the split plugin management route.
func PluginSettingsPage(props PluginSettingsPageProps) woxwidget.Widget {
	innerHeight := max(float32(0), props.Height-40)
	content := woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(20), Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.List.Width, Height: innerHeight, Child: PluginList(props.List)},
		woxwidget.Container{Width: 10, Height: innerHeight},
		woxwidget.Container{Width: 1, Height: innerHeight, Color: props.Theme.Border},
		woxwidget.Container{Width: 10, Height: innerHeight},
		woxwidget.Container{Width: props.Detail.Width, Height: innerHeight, Child: PluginDetail(props.Detail)},
	}}}
	if props.FilterPanel == nil {
		return content
	}
	// Anchor to the trailing 30px filter action (plus the 4px search inset), 4px below the 40px search field.
	panelLeft := 20 + max(float32(0), props.List.Width-34)
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
	Disabled          bool
	OnSelect          func()
}

// PluginListEntry is a catalog section header or one plugin row.
type PluginListEntry struct {
	ID     string
	Header string
	Item   PluginListItem
}

// PluginListProps contains plugin catalog data and search state.
type PluginListProps struct {
	Width                 float32
	Height                float32
	Entries               []PluginListEntry
	Message               string
	MessageError          bool
	Placeholder           string
	Search                woxui.TextEditingState
	Focused               bool
	Window                *woxui.Window
	FilterIcon            *woxui.Image
	InstalledIcon         *woxui.Image
	InstalledSelectedIcon *woxui.Image
	FilterLabel           string
	FilterActive          bool
	EmptyLabel            string
	EmptyTitle            string
	EmptyDescription      string
	EmptyIcon             *woxui.Image
	Theme                 woxcomponent.ControlTheme
	OnClear               func()
	OnSearchKey           func(woxui.KeyEvent) bool
	OnSearchFocusChange   func(bool)
	OnSearchChanged       func(string)
	OnSetSearchValue      func(string) error
	OnFilter              func()
}

// PluginList builds the searchable plugin catalog.
func PluginList(props PluginListProps) woxwidget.Widget {
	if props.Message != "" {
		if !props.MessageError {
			return woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.WoxLoadingIndicator(24, props.Theme.Focus)}
		}
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: props.Message, Width: max(float32(0), props.Width-32), Height: max(float32(0), props.Height-32), Style: woxui.TextStyle{Size: 12}, Color: props.Theme.Error,
		}}
	}

	const headerHeight = float32(62)
	viewportHeight := max(float32(0), props.Height-headerHeight)
	entries := props.Entries

	var list woxwidget.Widget
	if len(entries) == 0 {
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
		catalog := woxwidget.LazyList{
			Width: props.Width, Viewport: viewportHeight, ItemCount: len(entries), ItemExtent: pluginListRowHeight,
			ItemKey: func(index int) woxwidget.Key { return pluginListEntryKey(entries[index]) },
			ItemBuilder: func(index int) woxwidget.Widget {
				return pluginListEntry(entries[index], pluginListSectionHasLead(entries, index), props)
			},
		}
		if pluginListHasSectionHeader(entries) {
			catalog.ItemExtentAt = func(index int) float32 { return pluginListEntryExtent(entries, index) }
		}
		list = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "plugin-list-scroll", Content: catalog, Width: props.Width, Height: viewportHeight,
			KeepVisible: pluginListSelectedRange(entries), Theme: props.Theme, ThumbColor: props.Theme.Text,
		})
	}
	searchFieldWidth := max(float32(80), props.Width)
	searchTheme := props.Theme
	// Catalog search chrome sits with plugin titles. Keep the placeholder and field
	// outline on Text so TextSecondary cannot restyle this box.
	searchTheme.TextSecondary = props.Theme.Text
	searchField := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "plugin-search", Label: props.Placeholder, Width: searchFieldWidth, Value: props.Search.Text, Focused: props.Focused, Autofocus: props.Focused,
		Actions: []woxcomponent.SearchFieldAction{
			{ID: "plugin-filter", Label: props.FilterLabel, Icon: props.FilterIcon, Active: props.FilterActive, OnTap: props.OnFilter},
		},
		Window: props.Window, Theme: searchTheme, OnClear: props.OnClear, OnKey: props.OnSearchKey,
		OnFocusChange: props.OnSearchFocusChange, OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearchValue,
	})
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 20, Children: []woxwidget.Widget{searchField, list}}}
}

const pluginListRowHeight = float32(62)

func pluginListEntryKey(entry PluginListEntry) woxwidget.Key {
	if entry.Header != "" {
		return woxwidget.Key("plugin-list-section-" + entry.ID)
	}
	return woxwidget.Key("plugin-list-" + entry.Item.ID)
}

func pluginListHasSectionHeader(entries []PluginListEntry) bool {
	for _, entry := range entries {
		if entry.Header != "" {
			return true
		}
	}
	return false
}

func pluginListSectionHasLead(entries []PluginListEntry, index int) bool {
	for i := 0; i < index; i++ {
		if entries[i].Header != "" {
			return true
		}
	}
	return false
}

// pluginListEntryExtent keeps section headers shorter than plugin rows, and adds
// lead only before a following group so the first header sits flush under search.
func pluginListEntryExtent(entries []PluginListEntry, index int) float32 {
	if index < 0 || index >= len(entries) || entries[index].Header == "" {
		return pluginListRowHeight
	}
	height := woxcomponent.SettingsNavGroupHeight
	if pluginListSectionHasLead(entries, index) {
		height += woxcomponent.SettingsNavGroupLead
	}
	return height
}

// pluginListSelectedRange accounts for mixed header and row heights when revealing the selection.
func pluginListSelectedRange(entries []PluginListEntry) *woxwidget.ScrollRange {
	var offset float32
	for index, entry := range entries {
		extent := pluginListEntryExtent(entries, index)
		if entry.Header == "" && entry.Item.Selected {
			return &woxwidget.ScrollRange{Start: offset, End: offset + extent}
		}
		offset += extent
	}
	return nil
}

func pluginListEntry(entry PluginListEntry, lead bool, props PluginListProps) woxwidget.Widget {
	if entry.Header != "" {
		return pluginListSectionHeader(entry, lead, props)
	}
	return pluginListRow(entry.Item, props, pluginListRowHeight)
}

// pluginListSectionHeader uses the compact Settings rail group chrome, not the wide page divider.
func pluginListSectionHeader(entry PluginListEntry, lead bool, props PluginListProps) woxwidget.Widget {
	label, size := woxcomponent.SettingsChromeLabel(entry.Header)
	row := woxwidget.Container{
		Width: props.Width, Height: woxcomponent.SettingsNavGroupHeight, Padding: woxwidget.Insets{Left: 6, Right: 6},
		Child: woxwidget.Align{Height: woxcomponent.SettingsNavGroupHeight, Vertical: 0.5, Child: woxwidget.Text{
			Value: label, Style: woxui.TextStyle{Size: size, Weight: woxui.FontWeightSemibold}, Color: props.Theme.TextSecondary,
		}},
	}
	if lead {
		row = woxwidget.Container{Width: props.Width, Padding: woxwidget.Insets{Top: woxcomponent.SettingsNavGroupLead}, Child: row}
	}
	return woxwidget.Semantics{
		Key: pluginListEntryKey(entry), AutomationID: string(pluginListEntryKey(entry)),
		Role: woxui.AccessibilityRoleGroup, Label: entry.Header, Child: row,
	}
}

// pluginListRowTextColors keeps disabled names on secondary text so the inactive
// group stays gray even while the row is selected and still tappable.
func pluginListRowTextColors(item PluginListItem, theme woxcomponent.ControlTheme) (title, subtitle woxui.Color) {
	if item.Disabled {
		return theme.TextSecondary, theme.TextSecondary
	}
	if item.Selected {
		return theme.SelectionText, theme.SelectionText
	}
	return theme.Text, theme.TextSecondary
}

// pluginListRow builds one catalog entry so LazyList can materialize only the visible slice.
func pluginListRow(item PluginListItem, props PluginListProps, rowHeight float32) woxwidget.Widget {
	background := woxui.Color{}
	titleColor, subtitleColor := pluginListRowTextColors(item, props.Theme)
	if item.Selected {
		background = props.Theme.SelectionBackground
	}
	border := woxui.Color{}
	if item.Highlighted {
		border = props.Theme.SelectionBackground
		border.A = 122
		if !item.Selected {
			background = props.Theme.SelectionBackground
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
		// Keep the System badge on secondary text so a selected row cannot invert it.
		badge := woxcomponent.WoxTag(item.Badge, props.Theme.TextSecondary)
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
	return woxcomponent.WoxListItem(woxcomponent.ListItemProps{
		ID: "plugin-list-" + item.ID, Label: item.Name, Width: props.Width, Height: rowHeight, Radius: &radius,
		Background: &background, BorderColor: border, BorderWidth: 1, Selected: item.Selected, OnTap: item.OnSelect, Theme: props.Theme,
		Padding: woxwidget.Insets{Left: 6, Right: 6}, Child: woxwidget.Align{Height: rowHeight, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: rowChildren}},
	})
}

// PluginFilterField describes one exclusive catalog filter dropdown.
type PluginFilterField struct {
	ID    string
	Label string
	Value string
}

// PluginFilterPanelProps contains the anchored advanced-filter surface.
type PluginFilterPanelProps struct {
	Width        float32
	LabelWidth   float32
	Fields       []PluginFilterField
	ResetLabel   string
	ResetEnabled bool
	Theme        woxcomponent.ControlTheme
	OnOpen       func(string, woxui.Rect)
	OnReset      func()
	OnDismiss    func()
}

// PluginFilterPanel builds the catalog filter popover above the split view.
func PluginFilterPanel(props PluginFilterPanelProps) woxwidget.Widget {
	const rowGap = float32(12)
	const horizontalPadding = float32(16)
	rowHeight := woxcomponent.SettingsControlHeight
	innerWidth := max(float32(0), props.Width-horizontalPadding*2)
	labelWidth := min(max(float32(50), props.LabelWidth), innerWidth-woxcomponent.SettingsChoiceControlWidth-12)
	controlWidth := max(float32(120), innerWidth-labelWidth-12)
	rows := make([]woxwidget.Widget, 0, len(props.Fields))
	for _, field := range props.Fields {
		rows = append(rows, pluginFilterSelectRow(field, labelWidth, controlWidth, rowHeight, props))
	}
	if props.ResetLabel != "" || props.OnReset != nil {
		rows = append(rows, woxwidget.Align{Width: innerWidth, Height: rowHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "plugin-filter-reset", Label: props.ResetLabel, Disabled: !props.ResetEnabled, Variant: woxcomponent.ButtonSecondary,
			OnTap: props.OnReset, Theme: props.Theme,
		})})
	}
	height := horizontalPadding*2 + float32(len(rows))*rowHeight + float32(max(0, len(rows)-1))*rowGap
	return woxwidget.FocusScope{Key: "plugin-filter-panel", Modal: true, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: 8, Floating: true, Color: props.Theme.Surface, BorderColor: props.Theme.Border, BorderWidth: 1,
		Padding: woxwidget.UniformInsets(horizontalPadding), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: rowGap, Children: rows},
	}}
}

func pluginFilterSelectRow(field PluginFilterField, labelWidth, controlWidth, height float32, props PluginFilterPanelProps) woxwidget.Widget {
	id := field.ID
	onOpen := func(anchor woxui.Rect) {
		if props.OnOpen != nil {
			props.OnOpen(id, anchor)
		}
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelWidth, Height: height, Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Text{
			Value: field.Label, Style: woxui.TextStyle{Size: woxcomponent.SettingsLabelFontSize}, Color: props.Theme.Text,
		}}},
		woxwidget.Keyed{Key: SettingChoiceAnchorKey("plugin-filter-" + field.ID), Child: woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
			ID: "plugin-filter-" + field.ID, Label: field.Label, Value: field.Value, Width: controlWidth, Height: height,
			Foreground: props.Theme.Text, Secondary: props.Theme.TextSecondary, Theme: props.Theme, OnTapBounds: onOpen,
		})},
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
	SectionLabel     string
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
	ScrollID          string
	Error             string
	DescriptionDetail *PluginStoreDetailProps
	Metadata          *PluginMetadataProps
	Form              *PluginFormProps
	Keywords          *PluginFormProps
	Commands          *PluginFormProps
	Tools             *PluginFormProps
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
	OnRuntimeHover    func(bool, woxui.Rect)
	RuntimeIcon       *woxui.Image
	WebsiteIcon       *woxui.Image
	FallbackColor     woxui.Color
	Management        []PluginAction
	ScrollID          string
	Keywords          *PluginFormProps
	Commands          *PluginFormProps
	Metadata          *PluginMetadataProps
	Screenshot        *woxui.Image
	ScreenshotLoading bool
	Error             string
	OnWebsite         func()
	OnScreenshot      func()
}

// PluginDetailProps selects the empty, store, or editable detail view.
type PluginDetailProps struct {
	Width            float32
	Height           float32
	EmptyLabel       string
	EmptyTitle       string
	EmptyDescription string
	EmptyIcon        *woxui.Image
	Window           *woxui.Window
	Store            *PluginStoreDetailProps
	Editor           *PluginEditorProps
	Theme            woxcomponent.ControlTheme
}

// PluginDetail builds the selected plugin detail route.
func PluginDetail(props PluginDetailProps) woxwidget.Widget {
	if props.Store != nil {
		return pluginStoreDetail(*props.Store, props.Width, props.Height, props.Theme)
	}
	if props.Editor != nil {
		return pluginEditor(*props.Editor, props.Width, props.Height, props.Theme)
	}
	title := props.EmptyTitle
	if title == "" && props.EmptyDescription == "" {
		title = props.EmptyLabel
	}
	return CatalogListEmptyState(CatalogListEmptyProps{
		Width: props.Width, Height: props.Height, Title: title, Description: props.EmptyDescription,
		Icon: props.EmptyIcon, Window: props.Window, Theme: props.Theme,
	})
}

// pluginEditor shares the description-first layout between installed plugins and the store.
func pluginEditor(props PluginEditorProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	content := []woxwidget.Widget{}
	var trailingScreenshot woxwidget.Widget
	if props.DescriptionDetail != nil {
		description := *props.DescriptionDetail
		// Installed forms prioritize configuration; store previews stay beside the description.
		if props.Form != nil {
			trailingScreenshot = pluginStoreScreenshot(description, innerWidth, theme)
			description.Screenshot = nil
			description.ScreenshotLoading = false
		}
		content = append(content, pluginStoreDescriptionContent(description, innerWidth, theme))
	}
	for _, form := range []*PluginFormProps{props.Form, props.Keywords, props.Commands, props.Tools} {
		if form == nil || len(form.Rows) == 0 {
			continue
		}
		content = append(content, woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: form.SectionLabel, Width: innerWidth, Theme: theme}), pluginFormContent(form, innerWidth, theme))
	}
	if props.Metadata != nil {
		metadata := props.Metadata
		rows := []woxwidget.Widget{}
		if metadata.EmptyTitle != "" {
			rows = append(rows, pluginDetailCopy(metadata.EmptyTitle, innerWidth, theme))
		}
		for _, item := range metadata.Items {
			rows = append(rows, pluginMetadataRow(item, innerWidth, theme))
		}
		content = append(content, woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: metadata.Header, Width: innerWidth, Theme: theme}), woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows})
	}
	if props.Error != "" {
		content = append(content, woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Style: woxui.TextStyle{Size: woxcomponent.SettingsHelpFontSize}, Color: theme.Error})
	}
	if trailingScreenshot != nil {
		content = append(content, trailingScreenshot)
	}
	var keepVisible woxwidget.Key
	if props.Form != nil {
		keepVisible = props.Form.KeepVisibleKey
	}
	// Keep the scrollbar and its pointer target in the right inset, clear of tables and actions.
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 16}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		pluginDetailHeader(props.Header, innerWidth, 80, theme),
		woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: woxwidget.Key(props.ScrollID), Width: max(float32(0), width-16), Height: max(float32(1), height-80), KeepVisibleKey: keepVisible,
			Content: woxwidget.Container{Width: innerWidth, Padding: woxwidget.Insets{Top: 8, Bottom: 16}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: content}}, Theme: theme, ThumbColor: theme.Text}),
	}}}
}

func pluginDetailCopy(text string, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	return woxwidget.TextBlock{Value: text, Width: width, LineHeight: 18, Style: woxui.TextStyle{Size: woxcomponent.SettingsHelpFontSize}, Color: theme.TextSecondary}
}

// pluginFormContent lets sections grow naturally inside the page's shared scroll surface.
func pluginFormContent(form *PluginFormProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	if form == nil {
		return nil
	}
	if len(form.Rows) == 0 {
		return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{pluginDetailCopy(form.EmptyTitle, width, theme), pluginDetailCopy(form.EmptyDescription, width, theme)}}
	}
	rows := form.Rows
	if form.Intro != "" {
		rows = append([]woxwidget.Widget{woxcomponent.WoxHintBox(woxcomponent.HintBoxProps{Text: form.Intro, Width: width, Icon: form.IntroIcon, Accent: form.IntroAccent, Theme: theme})}, rows...)
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}
}

func pluginDetailHeader(props PluginHeaderProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	var icon woxwidget.Widget = woxwidget.Container{Width: 32, Height: 32, Radius: 7, Color: props.FallbackColor}
	if props.Icon != nil {
		icon = woxwidget.Image{Source: props.Icon, Width: 32, Height: 32, Fit: woxwidget.ImageFitContain}
	}
	// Keep the icon and title as siblings. A 40-high title-only box top-aligns
	// the text while CrossAxisCenter lifts the 32 icon against that taller box.
	identity := woxwidget.Container{Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		icon,
		woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: theme.InputText},
			woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13}, Color: theme.TextSecondary},
		}}},
	}}}
	author := woxwidget.Container{Height: 32, Padding: woxwidget.Insets{Left: 8}, Child: woxwidget.Align{
		Height: 32, Vertical: 0.5, Child: woxwidget.Text{Value: props.Author, Style: woxui.TextStyle{Size: 12}, Color: theme.TextSecondary},
	}}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{identity, author}}},
		pluginOutlineActions(props.Management, theme),
	}}}
}

// pluginMetadataRow keeps access names and explanations readable at narrow detail widths.
func pluginMetadataRow(item PluginMetadataItem, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	border := theme.Border
	border.A /= 2
	return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: 16, Bottom: 16}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: item.Title, Width: width, LineHeight: 20, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.Text},
			woxwidget.TextBlock{Value: item.Description, Width: width, LineHeight: 18, Style: woxui.TextStyle{Size: 12}, Color: theme.TextSecondary},
		}}},
		woxwidget.Container{Width: width, Height: 1, Color: border},
	}}}
}

// pluginStoreDetail supplies read-only content to the same detail surface as installed plugins.
func pluginStoreDetail(props PluginStoreDetailProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	return pluginEditor(PluginEditorProps{
		Header:   PluginHeaderProps{Name: props.Name, Version: props.Version, Author: props.Author, Icon: props.Icon, FallbackColor: props.FallbackColor, Management: props.Management},
		ScrollID: props.ScrollID, DescriptionDetail: &props, Keywords: props.Keywords, Commands: props.Commands, Metadata: props.Metadata, Error: props.Error,
	}, width, height, theme)
}

// pluginStoreDescriptionContent shares intrinsic description layout with the installed page.
func pluginStoreDescriptionContent(props PluginStoreDetailProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	chips := make([]woxwidget.Widget, 0, 2)
	if props.Runtime != "" {
		chips = append(chips, woxwidget.Gesture{ID: "plugin-runtime", OnHoverAt: props.OnRuntimeHover, Child: pluginStoreChip(props.Runtime, props.RuntimeIcon, nil, theme)})
	}
	if chip := pluginStoreChip(props.WebsiteChipLabel, props.WebsiteIcon, props.OnWebsite, theme); chip != nil {
		chips = append(chips, chip)
	}
	metadata := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: chips}
	children := []woxwidget.Widget{woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Expanded{Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
			return woxwidget.TextBlock{Value: props.Description, Width: size.Width, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: theme.Text}
		}}}, metadata,
	}}}
	if shot := pluginStoreScreenshot(props, width, theme); shot != nil {
		children = append(children, shot)
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: children}
}

// pluginStoreScreenshot sizes the manifest preview to the description width so URL images keep their aspect ratio.
// While the URL is still resolving, Flutter only shows a compact spinner instead of a blank panel that later pops in.
func pluginStoreScreenshot(props PluginStoreDetailProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	if props.Screenshot != nil && props.Screenshot.Width > 0 && props.Screenshot.Height > 0 {
		screenshotHeight := width * float32(props.Screenshot.Height) / float32(props.Screenshot.Width)
		return woxwidget.Gesture{ID: "plugin-store-screenshot", OnTap: props.OnScreenshot, Child: woxwidget.Container{
			Width: width, Height: screenshotHeight, Radius: 8,
			Child: woxwidget.Image{Source: props.Screenshot, Width: width, Height: screenshotHeight, Radius: 8, Fit: woxwidget.ImageFitContain},
		}}
	}
	if !props.ScreenshotLoading {
		return nil
	}
	return woxwidget.Align{
		Width: width, Height: 48, Horizontal: 0.5, Vertical: 0.5,
		Child: woxcomponent.WoxLoadingIndicator(24, theme.Focus),
	}
}

// pluginStoreChip keeps version, runtime, and source metadata visually consistent.
func pluginStoreChip(label string, icon *woxui.Image, onTap func(), theme woxcomponent.ControlTheme) woxwidget.Widget {
	if label == "" {
		return nil
	}
	if onTap != nil {
		return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "plugin-store-chip-" + label, Label: label, Icon: icon, IconSize: 14, Variant: woxcomponent.ButtonText, OnTap: onTap, Theme: theme})
	}
	width := max(float32(58), float32(len([]rune(label)))*7+24)
	children := make([]woxwidget.Widget, 0, 2)
	if icon != nil {
		children = append(children, woxwidget.Image{Source: icon, Width: 14, Height: 14, Fit: woxwidget.ImageFitContain})
		width += 18
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: theme.TextSecondary})
	return woxwidget.Gesture{ID: "plugin-store-chip-" + label, OnTap: onTap, Child: woxwidget.Container{
		Width: width, Height: 32, Radius: 4,
		Padding: woxwidget.Insets{Left: 10, Right: 8}, Child: woxwidget.Align{Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
	}}
}

// pluginOutlineActions renders quiet management actions with intrinsic button widths.
func pluginOutlineActions(actions []PluginAction, theme woxcomponent.ControlTheme) woxwidget.Widget {
	return pluginActions(actions, theme)
}

// pluginTextActions renders lightweight metadata links separately from lifecycle controls.
func pluginTextActions(actions []PluginAction, theme woxcomponent.ControlTheme) woxwidget.Widget {
	buttons := make([]woxwidget.Widget, 0, len(actions))
	for _, action := range actions {
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Icon: action.Icon, IconSize: 13, IconGap: 6, FontSize: 12,
			Padding: woxwidget.Insets{Left: 6, Right: 4}, Disabled: !action.Enabled, Variant: woxcomponent.ButtonText, OnTap: action.OnTap, Theme: theme,
		}))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: buttons}
}

func pluginActions(actions []PluginAction, theme woxcomponent.ControlTheme) woxwidget.Widget {
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
