package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSidebarPanelAndSplit(t *testing.T) {
	for _, scale := range []float32{.9, 1, 1.1} {
		theme := Theme{Controls: ControlTheme{DensityScale: scale}, ResultSubtitle: woxui.Color{R: 100, G: 120, B: 140, A: 200}}
		content := woxwidget.Text{Value: "content"}
		panel := SidebarPanel(240, 400, woxwidget.UniformInsets(8), theme, content).(woxwidget.Container)
		if panel.Radius != theme.Controls.Scaled(12) || panel.Color.A != 8 || panel.BorderColor.A != 24 || panel.Floating {
			t.Fatalf("unexpected sidebar surface: %+v", panel)
		}
		split := SidebarSplit(panel, content).(woxwidget.Flex)
		if split.Axis != woxwidget.Horizontal || len(split.Children) != 2 {
			t.Fatal("sidebar was not placed beside content")
		}
		if _, ok := SidebarSplit(nil, content).(woxwidget.Text); !ok {
			t.Fatal("closed sidebar changed content")
		}
	}
}

func TestSidebarMinimumWidth(t *testing.T) {
	if SidebarMinimumWidth(460, 256, false) != 460 || SidebarMinimumWidth(460, 256, true) != 716 {
		t.Fatal("sidebar minimum width is incorrect")
	}
}

func TestSidebarToggleReflectsOpenState(t *testing.T) {
	for _, colors := range []struct{ idle, active woxui.Color }{
		{woxui.Color{R: 150, G: 150, B: 150, A: 255}, woxui.Color{R: 255, G: 255, B: 255, A: 255}},
		{woxui.Color{R: 100, G: 100, B: 100, A: 255}, woxui.Color{A: 255}},
	} {
		icons := make([]*woxui.Image, 2)
		for i, open := range []bool{false, true} {
			button := SidebarToggle(IconButtonProps{}, 20, colors.idle, colors.active, open).(woxwidget.Stateful).Widget.(IconButtonProps)
			icons[i] = button.Icon.(woxwidget.Image).Source
		}
		closed, open := icons[0], icons[1]
		if closed == nil || open == nil || closed.Width != open.Width || closed.Height != open.Height {
			t.Fatal("sidebar state icons must have matching dimensions")
		}
		// The center of the left pane is transparent only when the sidebar is closed.
		offset := (closed.Width*(closed.Height/2) + closed.Width/4) * 4
		if closed.NativePixels()[offset+3] != 0 || open.NativePixels()[offset+3] != 255 {
			t.Fatal("open sidebar must fill the left pane while closed remains hollow")
		}
		pixels := open.NativePixels()
		if pixels[offset] != colors.active.R || pixels[offset+1] != colors.active.G || pixels[offset+2] != colors.active.B {
			t.Fatal("open sidebar fill must use the active theme color")
		}
	}
}
