package launcher

import (
	"context"
	"log"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// reloadAboutVersion reads the running core version instead of duplicating build metadata in the UI binary.
func (a *App) reloadAboutVersion() {
	a.mu.Lock()
	if a.aboutLoading {
		a.mu.Unlock()
		return
	}
	a.aboutLoading = true
	a.aboutError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var version string
	err := a.client.Post(ctx, "/version", map[string]any{}, &version)

	a.mu.Lock()
	a.aboutLoading = false
	if err != nil {
		a.aboutError = err.Error()
	} else {
		a.aboutVersion = version
		a.aboutLoaded = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// buildAboutSettingsPage presents core-owned version data and platform-neutral browser actions.
func (a *App) buildAboutSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := min(float32(640), max(float32(0), width-96))
	left := max(float32(48), (width-contentWidth)*0.5)
	version := snapshot.aboutVersion
	if version == "" && snapshot.aboutLoading {
		version = "Loading version…"
	}
	if version == "" {
		version = "Version unavailable"
	}
	status := ""
	if snapshot.aboutError != "" {
		status = "Could not read version: " + snapshot.aboutError
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: left, Top: 92, Right: left, Bottom: 40}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 22, Children: []woxwidget.Widget{
			woxwidget.Container{Width: contentWidth, Height: 86, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "WOX", Style: woxui.TextStyle{Size: 42, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: version, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.cursor},
			}}},
			woxwidget.TextBlock{Value: "A cross-platform launcher that keeps plugins, search, automation, and AI workflows one keystroke away.", Width: contentWidth, Height: 64, Style: woxui.TextStyle{Size: 16}, LineHeight: 24, Color: snapshot.palette.resultTitle},
			woxwidget.Container{Width: contentWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
				a.buildAboutLink("Documentation", "https://wox-launcher.github.io/Wox/#/", snapshot.palette),
				a.buildAboutLink("GitHub", "https://github.com/Wox-launcher/Wox", snapshot.palette),
			}}},
			woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 12}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}},
		},
	}}
}

func (a *App) buildAboutLink(label, target string, palette uiPalette) woxwidget.Widget {
	return woxwidget.Gesture{ID: "about-link-" + label, OnTap: func() {
		if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
			log.Printf("open About link: %v", err)
			a.mu.Lock()
			a.aboutError = err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}, Child: woxwidget.Container{
		Width: 150, Height: 42, Radius: 9, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 12},
		Child: woxwidget.Text{Value: label + "  ↗", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.cursor},
	}}
}
