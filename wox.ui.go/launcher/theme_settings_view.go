package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildThemeCatalog uses the same list/detail composition for installed and store themes.
func (a *App) buildThemeCatalog(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	listWidth := min(float32(300), max(float32(240), width*0.32))
	detailWidth := max(float32(0), width-listWidth-16)
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
		a.buildThemeList(snapshot, listWidth, height),
		a.buildThemeDetail(snapshot, detailWidth, height),
	}}
}

// buildThemeList renders core-owned theme metadata with local resolved-color swatches.
func (a *App) buildThemeList(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	headerHeight := float32(42)
	viewportHeight := max(float32(0), height-headerHeight-20)
	a.setThemeListViewport(viewportHeight)
	if snapshot.themesLoading && len(snapshot.themes) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Text{
			Value: "Loading themes…", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	if snapshot.themesError != "" && len(snapshot.themes) == 0 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.TextBlock{
			Value: snapshot.themesError, Width: max(float32(0), width-32), Height: max(float32(0), height-32), Style: woxui.TextStyle{Size: 12}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.themes))
	for index, theme := range snapshot.themes {
		index := index
		theme := theme
		background := woxui.Color{}
		titleColor := snapshot.palette.resultTitle
		if index == snapshot.themeSelected {
			background = snapshot.palette.selectedBackground
			titleColor = snapshot.palette.selectedTitle
		}
		status := theme.Author + " · " + theme.Version
		if snapshot.themesMode == "store" && !theme.IsInstalled {
			status = "Available · " + status
		} else if theme.ID == snapshot.data.ThemeID && theme.IsUpgradable {
			status = "Active · upgrade available · " + status
		} else if theme.IsUpgradable {
			status = "Upgrade available · " + status
		} else if theme.ID == snapshot.data.ThemeID {
			status = "Active · " + status
		}
		if theme.IsAuto {
			status = "Auto appearance · " + status
		}
		rows = append(rows, woxwidget.Gesture{ID: "theme-list-" + theme.ID, OnTap: func() { a.selectTheme(index) }, Child: woxwidget.Container{
			Width: width - 16, Height: themeSettingsListRowHeight, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 8, Bottom: 8},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				a.buildThemeSwatch(theme, 34),
				woxwidget.Container{Width: max(float32(0), width-78), Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
					woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
				}}},
			}},
		}})
	}
	contentHeight := max(viewportHeight, float32(len(rows))*themeSettingsListRowHeight)
	label := "Installed"
	if snapshot.themesMode == "store" {
		label = "Store"
	}
	list := woxwidget.Gesture{ID: "theme-list-scroll", OnScroll: func(delta woxui.Point) { a.scrollThemeList(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: width - 16, Height: viewportHeight, ContentHeight: contentHeight, Offset: snapshot.themeListScroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(8), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 16, Height: headerHeight, Padding: woxwidget.Insets{Left: 10, Top: 10}, Child: woxwidget.Text{
				Value: fmt.Sprintf("%s · %d", label, len(snapshot.themes)), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle,
			}},
			list,
		},
	}}
}

// buildThemeDetail previews a theme without applying it and exposes explicit lifecycle actions.
func (a *App) buildThemeDetail(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.themeSelected < 0 || snapshot.themeSelected >= len(snapshot.themes) {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Text{
			Value: "No theme selected", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	theme := snapshot.themes[snapshot.themeSelected]
	innerWidth := max(float32(0), width-36)
	actions := a.buildThemeActions(snapshot, theme)
	var website woxwidget.Widget = woxwidget.Painter{Width: 0, Height: 36}
	websiteWidth := float32(0)
	if strings.TrimSpace(theme.URL) != "" {
		website = a.buildFormTableButton("theme-website", "Website", 104, snapshot.themeOperation == "", false, a.openSelectedThemeWebsite, snapshot.palette)
		websiteWidth = 104
	}
	description := theme.Description
	if strings.TrimSpace(description) == "" {
		description = "No description provided."
	}
	errorColor := snapshot.palette.resultSubtitle
	if snapshot.themesError != "" {
		errorColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.actionBackground, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
				a.buildThemeSwatch(theme, 54),
				woxwidget.Container{Width: max(float32(0), innerWidth-54-websiteWidth-28), Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
					woxwidget.Text{Value: theme.Name, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
					woxwidget.Text{Value: theme.Author + " · " + theme.Version, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
				}}},
				website,
			}},
			woxwidget.TextBlock{Value: description, Width: innerWidth, Height: 58, MaxLines: 3, Style: woxui.TextStyle{Size: 11}, LineHeight: 18, Color: snapshot.palette.resultSubtitle},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: actions},
			a.buildThemeDraftSample(theme.previewValues, innerWidth, min(float32(260), max(float32(130), height-230))),
			woxwidget.TextBlock{Value: snapshot.themesError, Width: innerWidth, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, Color: errorColor},
		},
	}}
}

// buildThemeActions mirrors core's install, apply, and uninstall lifecycle without mutating catalog DTOs.
func (a *App) buildThemeActions(snapshot settingsSnapshot, theme themeSettingsTheme) []woxwidget.Widget {
	busy := snapshot.themeOperation != ""
	if !theme.IsInstalled {
		return []woxwidget.Widget{a.buildFormTableButton("theme-install", themeOperationLabel(snapshot, "install", theme.ID, "Install"), 104, !busy, true, func() { a.runThemeOperation("install") }, snapshot.palette)}
	}
	buttons := make([]woxwidget.Widget, 0, 3)
	if theme.IsUpgradable {
		buttons = append(buttons, a.buildFormTableButton("theme-upgrade", themeOperationLabel(snapshot, "upgrade", theme.ID, "Upgrade"), 104, !busy, true, func() { a.runThemeOperation("upgrade") }, snapshot.palette))
	}
	if theme.ID != snapshot.data.ThemeID {
		buttons = append(buttons, a.buildFormTableButton("theme-apply", themeOperationLabel(snapshot, "apply", theme.ID, "Apply"), 96, !busy, true, func() { a.runThemeOperation("apply") }, snapshot.palette))
	}
	if !theme.IsSystem {
		label := "Uninstall"
		if snapshot.themeUninstallArmed == theme.ID {
			label = "Confirm uninstall"
		}
		buttons = append(buttons, a.buildFormTableButton("theme-uninstall", themeOperationLabel(snapshot, "uninstall", theme.ID, label), 124, !busy, false, func() { a.runThemeOperation("uninstall") }, snapshot.palette))
	}
	return buttons
}

func themeOperationLabel(snapshot settingsSnapshot, kind, themeID, idle string) string {
	if snapshot.themeOperation == kind+":"+themeID {
		return idle + "…"
	}
	return idle
}

// buildThemeSwatch creates a compact GPU-rendered identity from two resolved theme colors.
func (a *App) buildThemeSwatch(theme themeSettingsTheme, size float32) woxwidget.Widget {
	palette := themeEditorPalette(theme.previewValues)
	inner := max(float32(8), size*0.44)
	return woxwidget.Container{Width: size, Height: size, Radius: size * 0.22, Color: palette.background, Padding: woxwidget.UniformInsets((size - inner) * 0.5), Child: woxwidget.Container{
		Width: inner, Height: inner, Radius: inner * 0.28, Color: palette.selectedBackground,
	}}
}
