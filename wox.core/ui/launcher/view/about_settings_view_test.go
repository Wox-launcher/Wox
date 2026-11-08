package view

import (
	"image"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestAboutSettingsActionLayout keeps the license link beside the existing actions.
func TestAboutSettingsActionLayout(t *testing.T) {
	icon, err := woxui.NewImage(image.NewRGBA(image.Rect(0, 0, 24, 24)))
	if err != nil {
		t.Fatal(err)
	}
	openedLicenses := false
	props := AboutSettingsProps{
		Width: woxcomponent.SettingsWindowWidth - woxcomponent.SettingsRailMaxWidth, Height: 600,
		AppIcon: icon, Version: "2.4.9", Theme: woxcomponent.ControlTheme{DensityScale: 1.1},
		Description: "Wox é um launcher para Windows, macOS e Linux que simplesmente funciona.",
		Links: []AboutLink{
			{ID: "about-open-onboarding-button", Label: "Guia inicial", Icon: icon},
			{ID: "about-link-documentation", Label: "Docs", Icon: icon},
			{ID: "about-link-github", Label: "GitHub", Icon: icon},
			{ID: "about-open-licenses", Label: "Licenças de código aberto", Icon: icon, OnTap: func() { openedLicenses = true }},
		},
	}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return AboutSettingsView(props) })
	defer host.Dispose()
	host.AttachServices(actionSearchHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: props.Width, Height: props.Height}, Scale: 1.5})

	var previous woxui.Rect
	for i, link := range props.Links {
		bounds, ok := host.BoundsForKey(woxwidget.Key(link.ID))
		if !ok || bounds.Width <= 0 || bounds.X < 0 || bounds.X+bounds.Width > props.Width {
			t.Fatalf("%s has invalid bounds %+v", link.ID, bounds)
		}
		if i > 0 && (bounds.Y != previous.Y || bounds.X < previous.X+previous.Width) {
			t.Fatalf("%s is outside the action row: previous=%+v current=%+v", link.ID, previous, bounds)
		}
		previous = bounds
	}
	point := woxui.Point{X: previous.X + previous.Width/2, Y: previous.Y + previous.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	if !openedLicenses {
		t.Fatal("licenses action did not activate")
	}
}
