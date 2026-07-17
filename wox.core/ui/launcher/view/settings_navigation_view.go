package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingsNavItem contains one prepared navigation destination.
type SettingsNavItem struct {
	ID           string
	Label        string
	FallbackIcon string
	Icon         *woxui.Image
	Depth        int
	Parent       bool
	Selected     bool
	OnTap        func()
}

// SettingsRailProps contains navigation and search actions.
type SettingsRailProps struct {
	Width       float32
	Height      float32
	Items       []SettingsNavItem
	Scroll      float32
	SearchBox   woxwidget.Widget
	SearchPanel woxwidget.Widget
	ShowSearch  bool
	Theme       woxcomponent.Theme
	OnScroll    func(float32)
}

// SettingsRail builds the settings navigation rail.
func SettingsRail(props SettingsRailProps) woxwidget.Widget {
	items := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		item := item
		color := woxui.Color{}
		border := woxui.Color{}
		foreground := props.Theme.ToolbarText
		if item.Selected {
			color = settingsColorAlpha(props.Theme.SelectedBackground, 41)
			border = settingsColorAlpha(props.Theme.SelectedBackground, 82)
			foreground = props.Theme.SelectedTitle
		}
		labelStyle := woxui.TextStyle{Size: 13}
		if item.Parent {
			labelStyle.Weight = woxui.FontWeightSemibold
		}
		leftPadding := float32(10 + item.Depth*18)
		trailing := ""
		if item.Parent {
			trailing = "⌄"
		}
		var icon woxwidget.Widget = woxwidget.Text{Value: item.FallbackIcon, Style: woxui.TextStyle{Size: 15}, Color: foreground}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 18, Height: 18}
		}
		row := woxwidget.Container{Width: props.Width - 28, Height: 46, Radius: 6, Color: color, BorderColor: border, BorderWidth: 1, Padding: woxwidget.Insets{Left: leftPadding, Top: 11, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Align{Width: 22, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon},
			woxwidget.Align{Width: max(float32(0), props.Width-leftPadding-98), Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: item.Label, Style: labelStyle, Color: foreground}},
			woxwidget.Align{Width: 18, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: trailing, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle}},
		}}}
		items = append(items, woxwidget.Gesture{ID: "settings-nav-" + item.ID, OnTap: item.OnTap, Child: row})
	}
	innerWidth := props.Width - 28
	const searchAreaHeight = float32(58)
	viewportHeight := max(float32(1), props.Height-searchAreaHeight-28)
	nav := woxwidget.Gesture{ID: "settings-rail-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(items))*50), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: items},
	}}
	stackChildren := []woxwidget.StackChild{{Child: nav}}
	if props.ShowSearch {
		stackChildren = append(stackChildren, woxwidget.StackChild{Child: props.SearchPanel})
	}
	railColor := settingsColorAlpha(props.Theme.ToolbarText, 9)
	rail := woxwidget.Container{Width: props.Width, Height: props.Height, Color: railColor, Padding: woxwidget.Insets{Left: 14, Top: 14, Right: 14, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		props.SearchBox,
		woxwidget.Stack{Width: innerWidth, Height: viewportHeight, Children: stackChildren},
	}}}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: rail}, {Left: props.Width - 1, Child: woxwidget.Container{Width: 1, Height: props.Height, Color: settingsColorAlpha(props.Theme.PreviewSplit, 128)}}}}
}

// SettingsSearchBoxProps contains the search editing state and actions.
type SettingsSearchBoxProps struct {
	Width         float32
	Placeholder   string
	State         woxui.TextEditingState
	Focused       bool
	SearchIcon    *woxui.Image
	Window        *woxui.Window
	Theme         woxcomponent.Theme
	OnFocus       func()
	OnClear       func()
	OnCaret       func(int)
	OnKey         func(woxui.KeyEvent) bool
	OnTextInput   func(woxui.TextInputEvent) bool
	OnFocusChange func(bool)
	OnSetValue    func(string) error
}

// SettingsSearchBox builds the rail search field.
func SettingsSearchBox(props SettingsSearchBoxProps) woxwidget.Widget {
	search := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "settings-search-field", Label: props.Placeholder, Width: props.Width, State: props.State, Focused: props.Focused, Autofocus: props.Focused,
		SearchIcon: props.SearchIcon, Window: props.Window, Theme: props.Theme, OnFocus: props.OnFocus, OnClear: props.OnClear, OnCaret: props.OnCaret,
		OnKey: props.OnKey, OnTextInput: props.OnTextInput, OnFocusChange: props.OnFocusChange, OnSetValue: props.OnSetValue,
	})
	return woxwidget.Container{Width: props.Width, Height: 50, Child: search}
}

// SettingsSearchResult contains one prepared settings search destination.
type SettingsSearchResult struct {
	Title    string
	Subtitle string
	OnHover  func()
	OnTap    func()
}

// SettingsSearchResultsProps contains the search panel state and rows.
type SettingsSearchResultsProps struct {
	Width           float32
	AvailableHeight float32
	Results         []SettingsSearchResult
	Selected        int
	Scroll          float32
	EmptyMessage    string
	Theme           woxcomponent.Theme
	OnSetViewport   func(float32)
	OnScroll        func(float32)
}

// SettingsSearchResults builds the rail search result overlay.
func SettingsSearchResults(props SettingsSearchResultsProps) woxwidget.Widget {
	const rowHeight = float32(54)
	selected := 0
	if len(props.Results) > 0 {
		selected = min(max(0, props.Selected), len(props.Results)-1)
	}
	panelHeight := min(float32(280), props.AvailableHeight)
	if len(props.Results) > 0 {
		panelHeight = min(panelHeight, float32(len(props.Results))*rowHeight+12)
	} else {
		panelHeight = min(panelHeight, float32(58))
	}
	viewportHeight := max(float32(1), panelHeight-12)
	if props.OnSetViewport != nil {
		props.OnSetViewport(viewportHeight)
	}
	background := props.Theme.ToolbarBackground
	background.A = 255
	if len(props.Results) == 0 {
		return woxwidget.Container{Width: props.Width, Height: panelHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 18, Right: 12}, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Results))
	for index, result := range props.Results {
		index := index
		rowBackground := background
		titleColor := props.Theme.ResultTitle
		if index == selected {
			rowBackground = props.Theme.SelectedBackground
			titleColor = props.Theme.SelectedTitle
		}
		rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("settings-search-result-%d", index), OnHover: func(inside bool) {
			if inside && result.OnHover != nil {
				result.OnHover()
			}
		}, OnTap: result.OnTap, Child: woxwidget.Container{Width: props.Width - 12, Height: rowHeight, Radius: 5, Color: rowBackground, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: result.Subtitle, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
		}}}})
	}
	return woxwidget.Container{Width: props.Width, Height: panelHeight, Radius: 7, Color: background, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Gesture{ID: "settings-search-results", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: props.Width - 12, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(props.Results))*rowHeight), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}}
}

func settingsColorAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
