package view

import (
	"testing"
	component "wox/ui/launcher/component"
	widget "wox/ui/widget"
)

// TestOfflinePageCentersContent checks catalog recovery placement at different widths and densities.
func TestOfflinePageCentersContent(t *testing.T) {
	for _, width := range []float32{320, 900} {
		for _, scale := range []float32{0.9, 1, 1.1} {
			page := OfflineSettingsView(OfflineSettingsProps{Width: width, Height: 600, Theme: component.ControlTheme{DensityScale: scale}}).(widget.ScrollView)
			alignment := page.Child.(widget.Align)
			if alignment.Horizontal != 0.5 || alignment.Vertical != 0.5 {
				t.Fatal("offline content must be centered on both axes")
			}
			content := alignment.Child.(widget.Container)
			if content.Width > width-48 {
				t.Fatal("offline content exceeds the available reading width")
			}
			column := content.Child.(widget.Flex)
			if column.CrossAxisAlignment != widget.CrossAxisCenter || len(column.Children) != 4 {
				t.Fatal("expected centered icon, title, description and settings action")
			}
		}
	}
}
