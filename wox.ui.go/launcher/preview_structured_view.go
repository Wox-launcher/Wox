package launcher

import (
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildPluginDetailPreview keeps store metadata and screenshots on the shared GPU widget path.
func (a *App) buildPluginDetailPreview(data pluginDetailPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	headerHeight := min(float32(108), height)
	iconWidth := float32(0)
	var icon woxwidget.Widget
	if data.Icon.ImageType != "" && data.Icon.ImageData != "" {
		iconWidth = 62
		if image := a.imageFor(data.Icon); image != nil {
			icon = woxwidget.Container{Width: 50, Height: 50, Radius: 9, Color: palette.background, Padding: woxwidget.UniformInsets(7), Child: woxwidget.Image{Source: image, Width: 36, Height: 36}}
		} else {
			icon = woxwidget.Container{Width: 50, Height: 50, Radius: 9, Color: palette.background}
		}
	}
	metadata := make([]string, 0, 4)
	if data.Author != "" {
		metadata = append(metadata, data.Author)
	}
	if data.Version != "" {
		metadata = append(metadata, "v"+data.Version)
	}
	if data.Runtime != "" {
		metadata = append(metadata, data.Runtime)
	}
	if data.Website != "" {
		metadata = append(metadata, data.Website)
	}
	textWidth := max(float32(0), width-iconWidth-28)
	text := woxwidget.Container{Width: textWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Text{Value: data.Name, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
		woxwidget.TextBlock{Value: data.Description, Width: textWidth, Height: 40, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: palette.resultSubtitle},
		woxwidget.Text{Value: strings.Join(metadata, "  ·  "), Style: woxui.TextStyle{Size: 10}, Color: palette.actionHeader},
	}}}
	headerChildren := make([]woxwidget.Widget, 0, 2)
	if icon != nil {
		headerChildren = append(headerChildren, icon)
	}
	headerChildren = append(headerChildren, text)
	header := woxwidget.Container{Width: width, Height: headerHeight, Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: headerChildren}}
	if len(data.ScreenshotURLs) == 0 || height <= headerHeight+20 {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Child: header}
	}
	// ponytail: The first screenshot covers the store preview now; add carousel state only when navigation is exposed in the shared widget API.
	screenshotHeight := max(float32(0), height-headerHeight-10)
	screenshot := a.buildPreviewImage(woxImage{ImageType: "url", ImageData: data.ScreenshotURLs[0]}, woxImage{ImageType: "url", ImageData: data.ScreenshotURLs[0]}, palette, width, screenshotHeight)
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{header, screenshot}}}
}
