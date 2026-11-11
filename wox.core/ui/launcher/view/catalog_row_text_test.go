package view

import (
	"math"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type catalogTallFontServices struct{ actionSearchHostServices }

func (services catalogTallFontServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	metrics, err := services.actionSearchHostServices.MeasureText(text, style)
	metrics.Size.Height = style.Size * 1.6
	return metrics, err
}

// TestCatalogRowTextFitsMeasuredLines checks both catalogs against tall fonts without losing horizontal clipping or row centering.
func TestCatalogRowTextFitsMeasuredLines(t *testing.T) {
	for _, density := range []float32{0.9, 1, 1.1, 1.2} {
		for _, dpi := range []float32{1, 1.5, 2.5} {
			for _, width := range []float32{80, 160} {
				theme := woxcomponent.ControlTheme{DensityScale: density}
				plugin := focusedControlGesture(pluginListRow(PluginListItem{Name: "LuxTranslate 文字g", Status: "0.1.8 stmluyuer ↓ 1817"}, PluginListProps{Width: 260, Theme: theme}, pluginListRowHeight)).Child.(woxwidget.Container)
				themeRow := themeListRow(ThemeSettingsProps{Theme: theme}, ThemeCatalogItem{Name: "Large テーマg", Version: "0.1.8", Author: "stmluyuer"}, 260).(woxwidget.Container)
				for _, row := range []woxwidget.Container{plugin, focusedControlGesture(themeRow.Child).Child.(woxwidget.Container)} {
					content := row.Child.(woxwidget.Align).Child.(woxwidget.Flex)
					clip := content.Children[1].(woxwidget.Expanded).Child.(woxwidget.Clip)
					texts := clip.Child.(woxwidget.Flex)
					title, status := texts.Children[0].(woxwidget.Text), texts.Children[1].(woxwidget.Text)
					texts.Children = []woxwidget.Widget{
						woxwidget.Keyed{Key: "title", Child: title}, woxwidget.Keyed{Key: "status", Child: status},
					}
					clip.Child = texts
					host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
						return woxwidget.Align{Width: width, Height: row.Height, Vertical: 0.5, Child: woxwidget.Keyed{Key: "text-clip", Child: clip}}
					})
					services := catalogTallFontServices{}
					host.AttachServices(services)
					host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: row.Height}, PixelSize: woxui.PixelSize{Width: int(width * dpi), Height: int(row.Height * dpi)}, Scale: dpi})
					textBounds, _ := host.BoundsForKey("text-clip")
					for _, line := range []struct {
						key  string
						text woxwidget.Text
					}{{"title", title}, {"status", status}} {
						bounds, found := host.BoundsForKey(woxwidget.Key(line.key))
						metrics, _ := services.MeasureText(line.text.Value, line.text.Style)
						if !found || bounds.Height < metrics.Size.Height || bounds.Y+bounds.Height > textBounds.Y+textBounds.Height+0.01 || bounds.Width > width {
							t.Errorf("density %g DPI %g width %g %s: line %+v clips measured height %g in %+v", density, dpi, width, line.key, bounds, metrics.Size.Height, textBounds)
						}
					}
					if textBounds.Width > width || textBounds.Height > row.Height || math.Abs(float64(textBounds.Y+textBounds.Height/2-row.Height/2)) > 0.01 {
						t.Errorf("density %g DPI %g: text %+v escaped its width or row centering", density, dpi, textBounds)
					}
					host.Dispose()
				}
			}
		}
	}
}
