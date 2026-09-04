package view

import (
	"slices"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWindowGroupSelectedLayoutCardUsesStrongActiveTreatment(t *testing.T) {
	theme := woxcomponent.Theme{ActionBackground: woxui.Color{R: 30, G: 32, B: 36, A: 255}}
	surface := windowGroupLayoutCard(WindowGroupEditorProps{Theme: theme}, WindowGroupLayoutOptionProps{ID: "split", Selected: true}).(woxwidget.Gesture).Child.(woxwidget.Stack)
	halo := surface.Children[0].Child.(woxwidget.Container)
	card := surface.Children[1].Child.(woxwidget.Container)

	if card.BorderColor != windowGroupSelectionColor() || card.BorderWidth != 2 {
		t.Fatalf("selected layout border = %+v/%.0f, want green/2", card.BorderColor, card.BorderWidth)
	}
	if card.Color == theme.ActionBackground {
		t.Fatal("selected layout background should have a visible green tint")
	}
	if halo.BorderWidth != 3 || halo.BorderColor.A == 0 {
		t.Fatalf("selected layout halo = %.0f/%d, want visible 3px ring", halo.BorderWidth, halo.BorderColor.A)
	}
}

func TestNormalizeWindowGroupURLMatchesFlutterSaveContract(t *testing.T) {
	inputs := []string{" example.com ", "http://wox.one", "HTTPS://wox.one/docs", "ftp://invalid", "  "}
	want := []string{"https://example.com", "http://wox.one", "HTTPS://wox.one/docs", "", ""}
	got := make([]string, len(inputs))
	for index, input := range inputs {
		got[index] = normalizeWindowGroupURL(input)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("normalized URLs = %v, want %v", got, want)
	}
}

func TestWindowGroupExtensionStatusOnlyLinksWhenDisconnected(t *testing.T) {
	props := WindowGroupUrlEditorProps{ExtensionDisconnectedLabel: "Disconnected", ExtensionInstallLabel: "Install", Theme: woxcomponent.Theme{}}
	if _, ok := windowGroupExtensionStatus(props, 480).(woxwidget.Gesture); !ok {
		t.Fatal("disconnected extension status should open the install link")
	}
	props.ExtensionConnected = true
	if _, ok := windowGroupExtensionStatus(props, 480).(woxwidget.Container); !ok {
		t.Fatal("connected extension status should be informational")
	}
}

func TestWindowGroupExtensionStatusVerticallyCentersContent(t *testing.T) {
	status := windowGroupExtensionStatus(WindowGroupUrlEditorProps{ExtensionConnected: true, ExtensionConnectedLabel: "Connected", Theme: woxcomponent.Theme{}}, 480).(woxwidget.Container)
	fill, ok := status.Child.(woxwidget.Constrained)
	if !ok || !fill.FillWidth {
		t.Fatalf("extension status fill = %#v, want parent-width constraint", status.Child)
	}
	align, ok := fill.Child.(woxwidget.Align)
	if !ok || align.Vertical != 0.5 || align.Height != status.Height {
		t.Fatalf("extension status alignment = %#v, want full-height vertical center", fill.Child)
	}
}

func TestWindowGroupURLDialogUsesCompactScrollableHeight(t *testing.T) {
	state := &windowGroupURLState{rowEditor: -2}
	dialog := state.buildDialog(woxwidget.StateContext{}, WindowGroupUrlEditorProps{Width: 1200, Height: 800, Theme: woxcomponent.Theme{}}).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	if props.Height != 344 {
		t.Fatalf("URL dialog height = %.0f, want compact 344", props.Height)
	}
}
