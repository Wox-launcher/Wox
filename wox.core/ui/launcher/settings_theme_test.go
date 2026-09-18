package launcher

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"reflect"
	"runtime"
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestSettingsPaletteIgnoresLauncherTheme keeps form chrome stable while theme
// previews and the launcher can still resolve arbitrary user colors.
func TestSettingsPaletteIgnoresLauncherTheme(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	previousAppearance := woxui.DefaultAppearanceIsDark()
	t.Cleanup(func() { woxui.SetDefaultAppearance(previousAppearance) })
	want := settingsPalette()
	wantAlpha := uint8(191)
	if runtime.GOOS == "linux" {
		wantAlpha = 255
	}
	if want.Background.A != wantAlpha || (want.Surface.A == 0 || want.Surface.A == 255) || !themeColorIsDark(want.Background) {
		t.Fatal("Settings must own a dark window tint, opaque on Linux, with translucent popup tints")
	}
	for _, color := range []string{"#FFFFFFFF", "#FF0000FF", "#00000000"} {
		app.applyTheme(themeData{AppBackgroundColor: color, ResultItemTitleColor: color, PreviewSplitLineColor: color})
		if got := app.settingsSnapshot().palette; !reflect.DeepEqual(got, want) {
			t.Fatalf("Settings inherited launcher theme %s", color)
		}
		draftTheme, err := themeEditorDraftTheme(map[string]any{"SchemaVersion": 2, "ThemeId": "settings-palette-test", "ThemeName": "Preview", "AppBackgroundColor": color, "BaseBackgroundColor": color, "BaseTextColor": "#F7F7F8FF", "BaseAccentColor": "#F7F7F8FF"}, map[string]string{})
		if err != nil {
			t.Fatal(err)
		}
		draft := paletteForTheme(draftTheme)
		if draft.background != app.palette.background {
			t.Fatalf("preview color %v does not match theme %v", draft.background, app.palette.background)
		}
	}
}

// TestSettingsImagesKeepTheirOwnAppearance checks both image-cache directions
// at fractional and integer display scales without recoloring brand artwork.
func TestSettingsImagesKeepTheirOwnAppearance(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.palette = paletteForTheme(themeData{AppBackgroundColor: "#FFFFFFFF"})
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path fill="var(--wox-theme-icon-color)" d="M0 0h8v16H0z"/><path fill="#FF0000" d="M8 0h8v16H8z"/></svg>`}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		size := physicalImageSize(16, scale)
		light, dark := &woxui.Image{Width: size}, &woxui.Image{Width: size}
		key := fmt.Sprintf("%s-svg-%d", imageKey(source), size)
		app.images[key], app.images[key+"-dark"] = light, dark
		if app.imageForSurface(source, size, settingsPalette().Background) != dark {
			t.Fatalf("Settings used light SVG at scale %v", scale)
		}
		if app.imageForSize(source, size) != light {
			t.Fatalf("Settings changed launcher SVG at scale %v", scale)
		}
	}
}

// TestSettingsAccentRemainsReadable exercises the actual resting control colors.
func TestSettingsAccentRemainsReadable(t *testing.T) {
	theme := settingsPalette()
	if theme.Accent.R != theme.Accent.G || max(theme.Accent.R, theme.Accent.B)-min(theme.Accent.R, theme.Accent.B) > 3 {
		t.Fatal("Settings active controls must keep the neutral Glass accent")
	}
	luminance := func(c woxui.Color) float64 {
		values := []uint8{c.R, c.G, c.B}
		weights := []float64{.2126, .7152, .0722}
		value := float64(0)
		for i, v := range values {
			channel := float64(v) / 255
			if channel <= .04045 {
				channel /= 12.92
			} else {
				channel = math.Pow((channel+.055)/1.055, 2.4)
			}
			value += channel * weights[i]
		}
		return value
	}
	contrast := func(a, b woxui.Color) float64 {
		x, y := luminance(a), luminance(b)
		return (max(x, y) + .05) / (min(x, y) + .05)
	}
	if theme.Accent.A != 255 || theme.Accent == theme.SelectionBackground || contrast(theme.Accent, theme.AccentText) < 4.5 || contrast(theme.Accent, theme.Background) < 3 {
		t.Fatal("Settings accent must distinguish active controls and support readable labels")
	}
	checkbox := woxcomponent.WoxCheckbox(woxcomponent.CheckboxProps{Value: true, Theme: theme}).(woxwidget.Container)
	switchVisual := woxcomponent.WoxSwitch(woxcomponent.SwitchProps{Value: true, Theme: theme}).(woxwidget.AnimatedFloat).Builder(1).(woxwidget.Stack)
	track := switchVisual.Children[0].Child.(woxwidget.Container)
	thumb := switchVisual.Children[1].Child.(woxwidget.Container)
	if checkbox.Color != theme.Accent || track.Color != theme.Accent || contrast(track.Color, thumb.Color) < 4.5 {
		t.Fatal("checked controls lost their active contrast")
	}
	button := woxcomponent.WoxButton(woxcomponent.ButtonProps{Label: "Save", Width: 80, Variant: woxcomponent.ButtonPrimary, Theme: theme}).(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Stateful)
	body := button.CreateState().Build(woxwidget.StateContext{}, button.Widget).(woxwidget.Gesture).Child.(woxwidget.Container)
	label := body.Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if body.Color != theme.Accent || label.Color != theme.AccentText {
		t.Fatal("primary button lost the Settings accent pair")
	}
}

// TestManagementWindowAppearanceContract guards native setup without opening desktop windows.
func TestManagementWindowAppearanceContract(t *testing.T) {
	for _, spec := range []struct {
		file, function string
		expected       int
	}{
		{"windows.go", "ensureSettingsWindow", 2}, {"windows.go", "ensureOnboardingWindow", 2}, {"theme.go", "applyTheme", 2},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), spec.file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != spec.function {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if method.Sel.Name != "SetAppearance" && method.Sel.Name != "SetWindowChrome" {
					return true
				}
				if spec.function == "applyTheme" {
					window, ok := method.X.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := window.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					owner, ok := selector.X.(*ast.Ident)
					if !ok || (owner.Name != "settingsView" && owner.Name != "onboardingView") {
						return true
					}
				}
				enabled, ok := call.Args[0].(*ast.Ident)
				want := "true"
				if method.Sel.Name == "SetWindowChrome" {
					want = "false"
				}
				if !ok || enabled.Name != want {
					t.Errorf("%s must use %s(%s) for fixed dark system material", spec.function, method.Sel.Name, want)
				}
				count++
				return true
			})
		}
		if count != spec.expected {
			t.Errorf("%s management appearance calls = %d, want %d", spec.function, count, spec.expected)
		}
	}
}
