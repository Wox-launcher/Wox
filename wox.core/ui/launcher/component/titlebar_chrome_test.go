package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestMacTrafficLightUsesInactiveGrayWhileUnfocused(t *testing.T) {
	dark := Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}}
	native := woxui.Color{R: 255, G: 92, B: 95, A: 255}
	control := MacTrafficLight("close", native, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255}, false, false, false, dark, func() {}, nil, nil)
	if fill := macTrafficLightFill(control); fill != MacTrafficLightInactiveColor(dark) {
		t.Fatalf("unfocused traffic light = %#v, want inactive gray %#v", fill, MacTrafficLightInactiveColor(dark))
	}
	symbol := macTrafficLightSymbol(control)
	if empty, ok := symbol.(woxwidget.Container); !ok || empty.Width != 14 || empty.Height != 14 {
		t.Fatalf("unfocused traffic light glyph = %#v, want empty 14x14 container", symbol)
	}
}

func TestMacTrafficLightRestoresNativeColorOnHoverWhileUnfocused(t *testing.T) {
	dark := Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}}
	native := woxui.Color{R: 255, G: 92, B: 95, A: 255}
	glyph := woxui.Color{R: 128, G: 47, B: 49, A: 255}
	control := MacTrafficLight("close", native, "×", glyph, true, false, false, dark, func() {}, nil, nil)
	if fill := macTrafficLightFill(control); fill != native {
		t.Fatalf("hovered unfocused traffic light = %#v, want native %#v", fill, native)
	}
	if _, ok := macTrafficLightSymbol(control).(woxwidget.Painter); !ok {
		t.Fatal("hovered unfocused close control should reveal its glyph")
	}
}

func TestMacTrafficLightKeepsNativeColorWhileFocused(t *testing.T) {
	dark := Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}}
	native := woxui.Color{R: 250, G: 200, B: 0, A: 255}
	control := MacTrafficLight("minimize", native, "−", woxui.Color{}, false, false, true, dark, func() {}, nil, nil)
	if fill := macTrafficLightFill(control); fill != native {
		t.Fatalf("focused traffic light = %#v, want native %#v", fill, native)
	}
}

func TestMacTrafficLightInactiveColorFollowsAppearance(t *testing.T) {
	dark := MacTrafficLightInactiveColor(Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}})
	light := MacTrafficLightInactiveColor(Theme{Background: woxui.Color{R: 245, G: 245, B: 245, A: 255}})
	if dark != (woxui.Color{R: 94, G: 94, B: 96, A: 255}) {
		t.Fatalf("dark inactive fill = %#v, want #5E5E60", dark)
	}
	if light != (woxui.Color{R: 222, G: 222, B: 222, A: 255}) {
		t.Fatalf("light inactive fill = %#v, want #DEDEDE", light)
	}
}

func macTrafficLightFill(control woxwidget.Widget) woxui.Color {
	if gesture, ok := control.(woxwidget.Gesture); ok {
		control = gesture.Child
	}
	return control.(woxwidget.Align).Child.(woxwidget.Container).Color
}

func macTrafficLightSymbol(control woxwidget.Widget) woxwidget.Widget {
	if gesture, ok := control.(woxwidget.Gesture); ok {
		control = gesture.Child
	}
	return control.(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Align).Child
}
