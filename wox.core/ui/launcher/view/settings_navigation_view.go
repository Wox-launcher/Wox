package view

import (
	"fmt"

	"wox/util"

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
	KeepVisible *woxwidget.ScrollRange
	SearchBox   woxwidget.Widget
	SearchPanel woxwidget.Widget
	ShowSearch  bool
	Theme       woxcomponent.Theme
}

// settingsRailBackground is the rail wash. Opaque Linux matches the page surface.
// Linux compositor blur must stay empty: the Windows toolbar overlay is invisible
// on Acrylic, but the same fill reads as a sidebar color mismatch on a colorful desktop.
func settingsRailBackground(theme woxcomponent.Theme, linux, nativeMaterial bool) woxui.Color {
	if linux {
		if nativeMaterial {
			return woxui.Color{}
		}
		return theme.Background
	}
	return settingsColorAlpha(theme.ToolbarText, 9)
}

// SettingsRail builds the settings navigation rail.
func SettingsRail(props SettingsRailProps) woxwidget.Widget {
	railColor := settingsRailBackground(props.Theme, util.IsLinux(), woxui.HasNativeWindowMaterial())
	rail := woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: railColor, Padding: woxwidget.UniformInsets(14),
		Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
			items := make([]woxwidget.Widget, 0, len(props.Items))
			for _, item := range props.Items {
				color := woxui.Color{}
				border := woxui.Color{}
				foreground := props.Theme.ToolbarText
				if item.Selected {
					color = props.Theme.SelectedBackground
					border = settingsColorAlpha(props.Theme.SelectedBackground, 82)
					foreground = props.Theme.SelectedTitle
				}
				labelStyle := woxui.TextStyle{Size: 13}
				if item.Parent {
					labelStyle.Weight = woxui.FontWeightSemibold
				}
				leftPadding := float32(10 + item.Depth*18)
				var icon woxwidget.Widget = woxwidget.Text{Value: item.FallbackIcon, Style: woxui.TextStyle{Size: 15}, Color: foreground}
				if item.Icon != nil {
					icon = woxwidget.Image{Source: item.Icon, Width: 18, Height: 18}
				}
				radius := float32(6)
				onTap := item.OnTap
				if item.Parent {
					onTap = nil
				}
				hoverBackground := settingsColorAlpha(props.Theme.ToolbarText, 25)
				items = append(items, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
					ID: "settings-nav-" + item.ID, Label: item.Label, Width: size.Width, Height: 46, Radius: &radius,
					Background: &color, HoverBackground: &hoverBackground, BorderColor: border, BorderWidth: 1, Selected: item.Selected, SkipFocus: item.Parent, OnTap: onTap, Theme: props.Theme,
					Padding: woxwidget.Insets{Left: leftPadding, Right: 10}, Child: woxwidget.Align{Height: 46, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
						woxwidget.Align{Width: 22, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon},
						woxwidget.Expanded{Child: woxwidget.Align{Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: item.Label, Style: labelStyle, Color: foreground}}},
					}}},
				}))
			}
			const searchAreaHeight = float32(58)
			viewportHeight := max(float32(1), size.Height-searchAreaHeight)
			nav := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
				Key: "settings-rail-scroll", KeepVisible: props.KeepVisible, Width: size.Width, Height: viewportHeight,
				Content:    woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: items},
				ThumbColor: props.Theme.ResultSubtitle, HideScrollbar: true,
			})
			stackChildren := []woxwidget.StackChild{{Child: nav}}
			if props.ShowSearch {
				stackChildren = append(stackChildren, woxwidget.StackChild{Child: props.SearchPanel})
			}
			return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
				props.SearchBox,
				woxwidget.Stack{Width: size.Width, Height: viewportHeight, Children: stackChildren},
			}}
		}},
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: rail}, {AnchorRight: true, StretchHeight: true, Child: woxwidget.Container{Width: 1, Color: settingsColorAlpha(props.Theme.ToolbarText, 26)}}}}
}

// SettingsSearchBoxProps contains the search editing state and actions.
type SettingsSearchBoxProps struct {
	Width         float32
	Placeholder   string
	State         woxui.TextEditingState
	Focused       bool
	Controller    *woxwidget.TextEditingController
	SearchIcon    *woxui.Image
	Window        *woxui.Window
	Theme         woxcomponent.Theme
	OnFocus       func()
	OnClear       func()
	OnKey         func(woxui.KeyEvent) bool
	OnFocusChange func(bool)
	OnChanged     func(string)
	OnSetValue    func(string) error
}

// SettingsSearchBox builds the rail search field.
func SettingsSearchBox(props SettingsSearchBoxProps) woxwidget.Widget {
	search := woxcomponent.WoxSearchField(woxcomponent.SearchFieldProps{
		ID: "settings-search-field", Label: props.Placeholder, Width: props.Width, Value: props.State.Text, Focused: props.Focused, Autofocus: props.Focused, Controller: props.Controller,
		SearchIcon: props.SearchIcon, Window: props.Window, Theme: props.Theme, OnFocus: props.OnFocus, OnClear: props.OnClear,
		OnKey: props.OnKey, OnFocusChange: props.OnFocusChange, OnChanged: props.OnChanged, OnSetValue: props.OnSetValue,
	})
	return woxwidget.Container{Width: props.Width, Height: 50, Child: search}
}

// SettingsSearchResult contains one prepared settings search destination.
type SettingsSearchResult struct {
	Title    string
	Subtitle string
	Icon     *woxui.Image
	OnHover  func()
	OnTap    func()
}

// SettingsSearchResultsProps contains the search panel state and rows.
type SettingsSearchResultsProps struct {
	Width           float32
	AvailableHeight float32
	Results         []SettingsSearchResult
	Selected        int
	EmptyMessage    string
	Theme           woxcomponent.Theme
}

// SettingsSearchResults builds the rail search result overlay.
func SettingsSearchResults(props SettingsSearchResultsProps) woxwidget.Widget {
	const rowHeight = float32(54)
	const panelRadius = float32(6)
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
	background := props.Theme.ToolbarBackground
	background.A = 255
	if len(props.Results) == 0 {
		return woxwidget.Container{Width: props.Width, Height: panelHeight, Radius: panelRadius, Color: background, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.Insets{Left: 12, Top: 18, Right: 12}, Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: woxcomponent.SettingsSearchTitleFontSize}, Color: props.Theme.ResultSubtitle}}
	}
	start := float32(selected) * rowHeight
	return woxwidget.Container{Width: props.Width, Height: panelHeight, Radius: panelRadius, Color: background, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(6), Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
		rows := make([]woxwidget.Widget, 0, len(props.Results))
		showIcons := size.Width-20 >= 72
		for index, result := range props.Results {
			rowBackground := background
			titleColor := props.Theme.ResultTitle
			subtitleColor := props.Theme.ResultSubtitle
			if index == selected {
				rowBackground = props.Theme.SelectedBackground
				titleColor = props.Theme.SelectedTitle
				subtitleColor = props.Theme.SelectedSubtitle
			}
			textColumn := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
				woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: woxcomponent.SettingsSearchTitleFontSize, Weight: woxui.FontWeightSemibold}, Color: titleColor},
				woxwidget.Text{Value: result.Subtitle, Style: woxui.TextStyle{Size: woxcomponent.SettingsSearchSubtitleFontSize}, Color: subtitleColor},
			}}
			var content woxwidget.Widget = textColumn
			if showIcons && result.Icon != nil {
				content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
					woxwidget.Align{Width: 24, Height: 38, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: result.Icon, Width: 24, Height: 24}},
					woxwidget.Expanded{Child: woxwidget.Align{Height: 38, Vertical: 0.5, Child: textColumn}},
				}}
			}
			rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("settings-search-result-%d", index), OnHover: func(inside bool) {
				if inside && result.OnHover != nil {
					result.OnHover()
				}
			}, OnTap: result.OnTap, Child: woxwidget.Container{Width: size.Width, Height: rowHeight, Radius: 5, Color: rowBackground, Padding: woxwidget.Insets{Left: 10, Right: 10}, Child: woxwidget.Align{Height: rowHeight, Vertical: 0.5, Child: content}}})
		}
		return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "settings-search-results", Width: size.Width, Height: size.Height,
			KeepVisible: &woxwidget.ScrollRange{Start: start, End: start + rowHeight},
			Content:     woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle,
		})
	}}}
}

func settingsColorAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
