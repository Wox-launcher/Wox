package woxview

import (
	"encoding/json"
	"fmt"
	"testing"

	"wox/common"
	"wox/common/icons"
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestCatalogIconColorMode renders every icon-bearing node in both appearances.
// It detects a multicolor brand turning into a solid tinted silhouette, and
// checks that original-color controls still visibly fade when disabled.
func TestCatalogIconColorMode(t *testing.T) {
	for _, kind := range []string{"image", "button", "icon-button", "submit"} {
		for _, mode := range []string{"", "tint", "original"} {
			for _, dark := range []bool{false, true} {
				for _, scale := range []float32{1, 1.5, 2} {
					t.Run(fmt.Sprintf("%s/%s/dark=%v/scale=%g", kind, mode, dark, scale), func(t *testing.T) {
						modeField := ""
						if mode != "" {
							modeField = fmt.Sprintf(`,"iconColorMode":%q`, mode)
						}
						props := fmt.Sprintf(`{"icon":"plugin.notes"%s,"label":"Example","style":{"width":48,"height":48}}`, modeField)
						if kind == "image" {
							props = fmt.Sprintf(`{"icon":"plugin.notes"%s,"style":{"width":48,"height":48}}`, modeField)
						}
						node := fmt.Sprintf(`{"type":%q,"props":%s}`, kind, props)
						if kind == "submit" {
							node = `{"type":"form","props":{"onSubmit":{"$handler":"submit"}},"children":[` + node + `]}`
						}
						v := NewView()
						v.SetTree(mustParse(t, `{"version":1,"root":`+node+`}`))
						color := ui.Color{A: 255}
						if dark {
							color.R, color.G, color.B = 255, 255, 255
						}
						theme := component.ControlTheme{Dark: dark, Text: color, ControlText: color}
						host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
							return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) {}})
						})
						host.AttachServices(fakeServices{})
						defer host.Dispose()
						img := animationFrameImage(t, host, scale)
						brand, tinted := false, false
						for i := 0; i < len(img.Pix); i += 4 {
							r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
							brand = brand || a > 200 && int(r) > int(g)+15 && int(g) > int(b)+50
							tinted = tinted || a > 200 && r == color.R && g == color.G && b == color.B
						}
						if mode != "tint" && !brand {
							t.Fatal("original brand colors were lost")
						}
						if mode == "tint" && (brand || !tinted) {
							t.Fatal("tint mode did not apply the foreground color")
						}
					})
				}
			}
		}
	}
}

// TestOriginalCatalogIconFollowsExplicitThemeVariable keeps brand colors fixed
// while resolving only the authored theme-dependent paint on appearance changes.
func TestOriginalCatalogIconFollowsExplicitThemeVariable(t *testing.T) {
	previous := icons.CurrentTheme()
	t.Cleanup(func() { icons.SetTheme(previous) })
	icons.SetTheme(icons.NewMapTheme("icon-color-test", map[string]common.WoxImage{
		icons.PluginNotes: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><rect width="10" height="10" fill="#f5c451"/><rect x="10" width="10" height="10" fill="var(--wox-theme-icon-color)"/></svg>`),
	}))
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"image","props":{"icon":"plugin.notes","style":{"width":40,"height":40}}}}`))
	dark := false
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{Dark: dark}})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, dark = range []bool{false, true, false} {
		frame := animationFrameImage(t, host, 1)
		brand, variable := frame.RGBAAt(70, 60), frame.RGBAAt(130, 60)
		if brand.R < 230 || brand.G < 180 || brand.B > 100 {
			t.Fatalf("brand color changed with theme: %+v", brand)
		}
		if dark && variable.R < 245 || !dark && variable.R > 10 || variable.A < 245 {
			t.Fatalf("theme variable did not follow dark=%v: %+v", dark, variable)
		}
	}
}

// TestDisabledOriginalIconFadesWithoutLosingColor covers both button types.
func TestDisabledOriginalIconFadesWithoutLosingColor(t *testing.T) {
	for _, kind := range []string{"button", "icon-button"} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":%q,"props":{"icon":"plugin.notes","label":"Example","disabled":true,"style":{"width":48,"height":48}}}}`, kind)))
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{Dark: true}})
		})
		host.AttachServices(fakeServices{})
		img := animationFrameImage(t, host, 1.5)
		host.Dispose()
		var maxAlpha uint8
		for i := 0; i < len(img.Pix); i += 4 {
			r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
			if int(r) > int(g)+4 && int(g) > int(b)+15 && a > maxAlpha {
				maxAlpha = a
			}
		}
		if maxAlpha < 70 || maxAlpha > 95 {
			t.Fatalf("%s disabled icon should retain muted brand color; max alpha %d", kind, maxAlpha)
		}
	}
}

// TestDisabledTintedButtonUsesLabelOpacity ensures an explicitly tinted glyph
// does not remain bright while its disabled label fades.
func TestDisabledTintedButtonUsesLabelOpacity(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"button","props":{"icon":"control.add","iconColorMode":"tint","label":"Example","disabled":true}}}`))
	white := ui.Color{R: 255, G: 255, B: 255, A: 255}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{Text: white, ControlText: white}})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	img := animationFrameImage(t, host, 1.5)
	var maxAlpha uint8
	for i := 3; i < len(img.Pix); i += 4 {
		maxAlpha = max(maxAlpha, img.Pix[i])
	}
	if maxAlpha < 70 || maxAlpha > 95 {
		t.Fatalf("disabled tinted icon is brighter than its label: max alpha %d", maxAlpha)
	}
}

// TestIconColorModeValidation keeps the JSON and Go contracts strict.
func TestIconColorModeValidation(t *testing.T) {
	for _, node := range []string{
		`{"type":"button","props":{"icon":"plugin.notes","iconColorMode":"invalid"}}`,
		`{"type":"icon-button","props":{"icon":"plugin.notes","iconColorMode":"invalid"}}`,
		`{"type":"image","props":{"icon":"plugin.notes","iconColorMode":"invalid"}}`,
		`{"type":"image","props":{"source":{"imageType":"svg","imageData":"x"},"iconColorMode":"original"}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + node + `}`)); err == nil {
			t.Fatalf("accepted invalid icon mode: %s", node)
		}
	}
}
