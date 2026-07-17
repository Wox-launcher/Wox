package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const ThemeListRowHeight = float32(62)

// ThemeCatalogItem contains resolved presentation data for one theme.
type ThemeCatalogItem struct {
	ID           string
	Name         string
	Author       string
	URL          string
	Version      string
	Description  string
	IsSystem     bool
	IsInstalled  bool
	IsUpgradable bool
	IsAuto       bool
	Active       bool
	PreviewTheme woxcomponent.Theme
}

// ThemeSettingsProps contains theme catalog state and actions.
type ThemeSettingsProps struct {
	Width          float32
	Height         float32
	Theme          woxcomponent.Theme
	Mode           string
	Loading        bool
	Error          string
	Selected       int
	Scroll         float32
	Operation      string
	UninstallArmed string
	Items          []ThemeCatalogItem
	OnSelect       func(int)
	OnScroll       func(float32)
	OnSetViewport  func(float32)
	OnOpenWebsite  func()
	OnOperation    func(string)
}

// ThemeSettingsView builds the installed/store theme catalog.
func ThemeSettingsView(props ThemeSettingsProps) woxwidget.Widget {
	listWidth := min(float32(300), max(float32(240), props.Width*0.32))
	detailWidth := max(float32(0), props.Width-listWidth-16)
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
		themeList(props, listWidth, props.Height),
		themeDetail(props, detailWidth, props.Height),
	}}
}

func themeList(props ThemeSettingsProps, width, height float32) woxwidget.Widget {
	headerHeight := float32(42)
	viewportHeight := max(float32(0), height-headerHeight-20)
	if props.OnSetViewport != nil {
		props.OnSetViewport(viewportHeight)
	}
	if props.Loading && len(props.Items) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Text{
			Value: "Loading themes…", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
	}
	if props.Error != "" && len(props.Items) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: props.Error, Width: max(float32(0), width-32), Height: max(float32(0), height-32), Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText,
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for index, theme := range props.Items {
		index := index
		theme := theme
		background := woxui.Color{}
		titleColor := props.Theme.ResultTitle
		if index == props.Selected {
			background = props.Theme.SelectedBackground
			titleColor = props.Theme.SelectedTitle
		}
		status := theme.Author + " · " + theme.Version
		if props.Mode == "store" && !theme.IsInstalled {
			status = "Available · " + status
		} else if theme.Active && theme.IsUpgradable {
			status = "Active · upgrade available · " + status
		} else if theme.IsUpgradable {
			status = "Upgrade available · " + status
		} else if theme.Active {
			status = "Active · " + status
		}
		if theme.IsAuto {
			status = "Auto appearance · " + status
		}
		rows = append(rows, woxwidget.Gesture{ID: "theme-list-" + theme.ID, OnTap: func() {
			if props.OnSelect != nil {
				props.OnSelect(index)
			}
		}, Child: woxwidget.Container{
			Width: width - 16, Height: ThemeListRowHeight, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 8, Bottom: 8},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				themeSwatch(theme.PreviewTheme, 34),
				woxwidget.Container{Width: max(float32(0), width-78), Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
					woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle},
				}}},
			}},
		}})
	}
	contentHeight := max(viewportHeight, float32(len(rows))*ThemeListRowHeight)
	label := "Installed"
	if props.Mode == "store" {
		label = "Store"
	}
	list := woxwidget.Gesture{ID: "theme-list-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: width - 16, Height: viewportHeight, ContentHeight: contentHeight, Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(8), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 16, Height: headerHeight, Padding: woxwidget.Insets{Left: 10, Top: 10}, Child: woxwidget.Text{
				Value: fmt.Sprintf("%s · %d", label, len(props.Items)), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle,
			}},
			list,
		},
	}}
}

func themeDetail(props ThemeSettingsProps, width, height float32) woxwidget.Widget {
	if props.Selected < 0 || props.Selected >= len(props.Items) {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Text{
			Value: "No theme selected", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
	}
	theme := props.Items[props.Selected]
	innerWidth := max(float32(0), width-36)
	actions := themeActions(props, theme)
	var website woxwidget.Widget = woxwidget.Painter{Width: 0, Height: 36}
	websiteWidth := float32(0)
	if strings.TrimSpace(theme.URL) != "" {
		website = woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "theme-website", Label: "Website", Width: 104, Disabled: props.Operation != "", OnTap: props.OnOpenWebsite, Theme: props.Theme})
		websiteWidth = 104
	}
	description := theme.Description
	if strings.TrimSpace(description) == "" {
		description = "No description provided."
	}
	errorColor := props.Theme.ResultSubtitle
	if props.Error != "" {
		errorColor = props.Theme.ErrorText
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.ActionBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
				themeSwatch(theme.PreviewTheme, 54),
				woxwidget.Container{Width: max(float32(0), innerWidth-54-websiteWidth-28), Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
					woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
					woxwidget.Text{Value: theme.Author + " · " + theme.Version, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
				}}},
				website,
			}},
			woxwidget.TextBlock{Value: description, Width: innerWidth, Height: 58, MaxLines: 3, Style: woxui.TextStyle{Size: 11}, LineHeight: 18, Color: props.Theme.ResultSubtitle},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: actions},
			previewview.ThemeDraftSample(theme.PreviewTheme, innerWidth, min(float32(260), max(float32(130), height-230))),
			woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, Color: errorColor},
		},
	}}
}

func themeActions(props ThemeSettingsProps, theme ThemeCatalogItem) []woxwidget.Widget {
	busy := props.Operation != ""
	button := func(id, label, operation string, width float32, primary bool) woxwidget.Widget {
		variant := woxcomponent.ButtonSecondary
		if primary {
			variant = woxcomponent.ButtonPrimary
		}
		if props.Operation == operation+":"+theme.ID {
			label += "…"
		}
		return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Width: width, Disabled: busy, Variant: variant, OnTap: func() {
			if props.OnOperation != nil {
				props.OnOperation(operation)
			}
		}, Theme: props.Theme})
	}
	if !theme.IsInstalled {
		return []woxwidget.Widget{button("theme-install", "Install", "install", 104, true)}
	}
	buttons := make([]woxwidget.Widget, 0, 3)
	if theme.IsUpgradable {
		buttons = append(buttons, button("theme-upgrade", "Upgrade", "upgrade", 104, true))
	}
	if !theme.Active {
		buttons = append(buttons, button("theme-apply", "Apply", "apply", 96, true))
	}
	if !theme.IsSystem {
		label := "Uninstall"
		if props.UninstallArmed == theme.ID {
			label = "Confirm uninstall"
		}
		buttons = append(buttons, button("theme-uninstall", label, "uninstall", 124, false))
	}
	return buttons
}

func themeSwatch(theme woxcomponent.Theme, size float32) woxwidget.Widget {
	inner := max(float32(8), size*0.44)
	return woxwidget.Container{Width: size, Height: size, Radius: size * 0.22, Color: theme.Background, Padding: woxwidget.UniformInsets((size - inner) * 0.5), Child: woxwidget.Container{
		Width: inner, Height: inner, Radius: inner * 0.28, Color: theme.SelectedBackground,
	}}
}
