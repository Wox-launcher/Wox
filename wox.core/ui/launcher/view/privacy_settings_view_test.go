package view

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"

	woxwidget "wox/ui/widget"
)

func TestPrivacyViewSampleButtonUsesSymmetricPadding(t *testing.T) {
	page := PrivacySettingsView(PrivacySettingsProps{
		Width: 900, Height: 400, ViewSampleLabel: "View data sample", TelemetryTitle: "Anonymous Usage Statistics",
	})
	fields := page.(woxwidget.Container).Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
	telemetry := fields.Children[3].(woxwidget.Container).Child.(woxwidget.Flex)
	controls := telemetry.Children[1].(woxwidget.Align).Child.(woxwidget.Flex)
	button := focusedControlGesture(controls.Children[0]).Child.(woxwidget.Container)
	if button.Padding.Left != 12 || button.Padding.Right != 12 {
		t.Fatalf("view sample padding = %+v, want the shared 12px button insets", button.Padding)
	}
}

// TestPrivacyOfflineDisablesTelemetry verifies the accessibility action cannot change a saved preference.
func TestPrivacyOfflineDisablesTelemetry(t *testing.T) {
	toggles := 0
	page := PrivacySettingsView(PrivacySettingsProps{Width: 1000, Height: 700, OfflineEnabled: true, TelemetryEnabled: true, OnToggleTelemetry: func() { toggles++ }})
	fields := page.(woxwidget.Container).Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
	telemetry := fields.Children[3].(woxwidget.Container).Child.(woxwidget.Flex)
	controls := telemetry.Children[1].(woxwidget.Align).Child.(woxwidget.Flex)
	control := controls.Children[1].(woxwidget.Semantics)
	if !control.Disabled || control.Checked || len(control.Actions) != 0 {
		t.Fatal("telemetry did not show its effective offline state")
	}
	_ = control.OnAction(woxui.AccessibilityActionToggle, "")
	if toggles != 0 {
		t.Fatal("offline telemetry preference changed")
	}
}

// TestPrivacyNarrowLayoutKeepsHelpUnclipped covers logical width and interface scaling independently.
func TestPrivacyNarrowLayoutKeepsHelpUnclipped(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1, 2} {
		props := PrivacySettingsProps{Width: 480, Height: 400, Theme: woxcomponent.ControlTheme{DensityScale: scale}}
		row := privacySettingField(props, "Clear local data on exit", "Long translated description", woxwidget.Container{})
		content := row.(woxwidget.Container).Child.(woxwidget.Flex)
		if content.Axis != woxwidget.Vertical {
			t.Fatal("narrow layout did not stack controls")
		}
		text := content.Children[1].(woxwidget.TextBlock)
		if text.MaxLines != 0 || text.Width > props.Width {
			t.Fatal("translated help was clipped")
		}
	}
}

// TestPrivacyRowsUseSharedGeometry keeps trailing switches aligned with ordinary settings controls.
func TestPrivacyRowsUseSharedGeometry(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1, 2} {
		theme := woxcomponent.ControlTheme{DensityScale: scale}
		props := PrivacySettingsProps{Width: 2000, Height: 800, Theme: theme}
		row := privacySettingField(props, "Offline mode", "Description", woxwidget.Container{}).(woxwidget.Container)
		content := row.Child.(woxwidget.Flex)
		label := content.Children[0].(woxwidget.Container).Child.(woxwidget.Constrained)
		control := content.Children[1].(woxwidget.Align)
		if label.MinHeight != theme.Scaled(woxcomponent.SettingsControlHeight) {
			t.Fatalf("row minimum height = %v", label.MinHeight)
		}
		if row.Padding.Bottom != theme.Scaled(24) {
			t.Fatal("row separation must be independent of text height")
		}
		if control.Height != theme.Scaled(woxcomponent.SettingsControlHeight) {
			t.Fatalf("control frame height = %v", control.Height)
		}
	}
}

// TestPrivacyWrappedHelpAddsHeight preserves the same trailing space for one and two lines.
func TestPrivacyWrappedHelpAddsHeight(t *testing.T) {
	for _, density := range []float32{0.9, 1, 1.1} {
		props := PrivacySettingsProps{Width: 1200, Height: 800, Theme: woxcomponent.ControlTheme{DensityScale: density}}
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
				woxwidget.Keyed{Key: "one", Child: privacySettingField(props, "Title", "One line", woxwidget.Container{})},
				woxwidget.Keyed{Key: "two", Child: privacySettingField(props, "Title", "One line\nSecond line", woxwidget.Container{})},
			}}
		})
		host.AttachServices(actionSearchHostServices{})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 1200, Height: 800}, Scale: 1})
		one, ok := host.BoundsForKey("one")
		two, okTwo := host.BoundsForKey("two")
		difference := two.Height - one.Height - props.Theme.Scaled(16)
		if !ok || !okTwo || difference < -0.1 || difference > 0.1 {
			t.Fatalf("density %v: adding a help line must add its full height: one=%+v two=%+v", density, one, two)
		}
		host.Dispose()
	}
}
