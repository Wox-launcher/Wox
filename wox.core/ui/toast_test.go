package ui

import (
	"context"
	"image/color"
	"testing"

	"wox/common"
	"wox/common/icons"
	"wox/util/overlay"
)

// TestPrepareToastPreservesContentAndAction verifies the internal notification boundary.
func TestPrepareToastPreservesContentAndAction(t *testing.T) {
	called := 0
	toast := prepareToast(t.Context(), common.ToastMsg{
		Text: "System error", DisplaySeconds: 8,
		Action: &common.ToastAction{Name: "Open settings", Action: func(context.Context) { called++ }},
	})
	if toast.Message != "System error" || toast.DisplaySeconds != 8 || toast.ActionLabel != "Open settings" || toast.OnDone == nil {
		t.Fatalf("unexpected toast: %+v", toast)
	}
	toast.OnDone(false)
	if called != 0 {
		t.Fatal("dismissal invoked the action")
	}
	toast.OnDone(true)
	if called != 1 {
		t.Fatalf("action calls = %d", called)
	}
}

// TestPrepareToastUpdateIconFollowsAppearance exercises the icon that crashed manual update checks.
func TestPrepareToastUpdateIconFollowsAppearance(t *testing.T) {
	t.Cleanup(func() { overlay.SetThemeProvider(nil) })
	for _, background := range []string{"#181d26", "#ffffff"} {
		t.Run(background, func(t *testing.T) {
			overlay.SetThemeProvider(func() common.Theme { return common.Theme{OverlayBackgroundColor: background} })
			toast := prepareToast(t.Context(), common.ToastMsg{Text: "Checking for updates", Icon: icons.Get(icons.ActionUpdate).String()})
			if toast.Icon == nil {
				t.Fatal("update icon was discarded")
			}
			visible := false
			for y := 0; y < toast.Icon.Bounds().Dy(); y++ {
				for x := 0; x < toast.Icon.Bounds().Dx(); x++ {
					pixel := color.RGBAModel.Convert(toast.Icon.At(x, y)).(color.RGBA)
					if pixel.A == 0 {
						continue
					}
					visible = true
					want := uint8(0)
					if background == "#181d26" {
						want = pixel.A
					}
					if pixel.R != want || pixel.G != want || pixel.B != want {
						t.Fatalf("update icon pixel=%v, want channel=%d", pixel, want)
					}
				}
			}
			if !visible {
				t.Fatal("update icon is transparent")
			}
		})
	}
}

// TestPrepareToastInvalidIconUsesFallback keeps decoder failures out of the notification renderer.
func TestPrepareToastInvalidIconUsesFallback(t *testing.T) {
	icon := common.NewWoxImageSvg(`<svg><path fill="var(--missing-color)" d="M0 0h8v8H0z"/></svg>`)
	toast := prepareToast(t.Context(), common.ToastMsg{Text: "Checking for updates", Icon: icon.String()})
	if toast.Icon != nil || toast.Message != "Checking for updates" {
		t.Fatalf("invalid icon did not leave the notification ready for its default icon: %+v", toast)
	}
}
