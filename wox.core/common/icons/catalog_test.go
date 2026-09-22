package icons

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"wox/common"
	woxsvg "wox/util/svg"
)

// TestPluginIconsAreVectorOnly keeps built-in identities scalable without embedded raster images.
func TestPluginIconsAreVectorOnly(t *testing.T) {
	for name, icon := range defaultPluginIcons {
		if icon.ImageType != common.WoxImageTypeSvg || strings.Contains(icon.ImageData, "base64,") || strings.Contains(icon.ImageData, "<image") {
			t.Fatalf("plugin icon %s must contain SVG geometry only", name)
		}
		for _, size := range []int{18, 48} {
			img, err := woxsvg.Render(icon.ImageData, size, size)
			if err != nil {
				t.Fatalf("render %s at %d: %v", name, size, err)
			}
			visible := false
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					visible = visible || img.RGBAAt(x, y).A > 0
				}
			}
			if !visible {
				t.Fatalf("plugin icon %s is blank at %d", name, size)
			}
		}
	}
}

// TestPluginStoreIconFillsCanvas prevents transparent padding from shrinking its menu identity.
func TestPluginStoreIconFillsCanvas(t *testing.T) {
	icon := Get(PluginWPM)
	if icon.ImageType != common.WoxImageTypeSvg {
		t.Fatalf("plugin store icon type = %q, want svg", icon.ImageType)
	}
	img, err := woxsvg.Render(icon.ImageData, 18, 18)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]int{{0, 9}, {17, 9}, {9, 0}, {9, 17}} {
		if img.RGBAAt(point[0], point[1]).A == 0 {
			t.Fatalf("plugin store icon has transparent padding at %v", point)
		}
	}
}

func TestStaticActivityIconsRenderAsSVG(t *testing.T) {
	for name, icon := range map[string]common.WoxImage{
		"media playing": Get(StatusPlaying),
		"running":       Get(StatusRunning),
		"loading":       Get(StatusLoading),
	} {
		if icon.ImageType != common.WoxImageTypeSvg {
			t.Fatalf("%s icon type = %q, want svg", name, icon.ImageType)
		}
		if _, err := woxsvg.Render(icon.ImageData, 48, 48); err != nil {
			t.Fatalf("render %s icon: %v", name, err)
		}
	}
}

func TestCommunityBrandIconsRenderFromSvgl(t *testing.T) {
	for _, name := range []string{BrandReddit, BrandDiscord} {
		icon := Get(name)
		if strings.Contains(icon.ImageData, "var(--wox-theme-icon-color)") {
			t.Fatalf("community brand icon %s must keep authored colors from svgl", name)
		}
		img, err := woxsvg.Render(icon.ImageData, 22, 22)
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		colored := false
		for y := 0; y < 22; y++ {
			for x := 0; x < 22; x++ {
				pixel := img.RGBAAt(x, y)
				colored = colored || pixel.A > 0 && (pixel.R != pixel.G || pixel.G != pixel.B)
			}
		}
		if !colored {
			t.Fatalf("community brand icon %s rendered blank or monochrome", name)
		}
	}
}

func TestChatPickerIconsRenderInColor(t *testing.T) {
	for _, name := range []string{ChatSelectFile, ChatSelectFolder} {
		icon := Get(name)
		if strings.Contains(icon.ImageData, "var(--wox-theme-icon-color)") {
			t.Fatalf("chat catalog icon %s must retain its authored colors", name)
		}
		img, err := woxsvg.Render(icon.ImageData, 18, 18)
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		colored := false
		for y := 0; y < 18; y++ {
			for x := 0; x < 18; x++ {
				pixel := img.RGBAAt(x, y)
				colored = colored || pixel.A > 0 && (pixel.R != pixel.G || pixel.G != pixel.B)
			}
		}
		if !colored {
			t.Fatalf("chat catalog icon %s rendered blank or monochrome", name)
		}
	}
}

func TestUIIconsAreCategorizedSVGs(t *testing.T) {
	categories := map[string]bool{}
	for name, icon := range defaultUIIcons {
		category, _, ok := strings.Cut(name, ".")
		if !ok || icon.ImageType != common.WoxImageTypeSvg || !strings.HasPrefix(icon.ImageData, "<svg") {
			t.Fatalf("UI icon %q is not a categorized SVG", name)
		}
		categories[category] = true
	}
	for _, category := range []string{"settings", "control", "screenshot", "usage", "runtime", "plugin"} {
		if !categories[category] {
			t.Fatalf("UI icon category %q is empty", category)
		}
	}
}

func TestWindowChromeIconsMatchWindowsCaptionGeometry(t *testing.T) {
	maximize := Get(ControlWindowMaximize).ImageData
	restore := Get(ControlWindowRestore).ImageData
	if strings.Contains(maximize, "rx=") || strings.Contains(restore, "rx=") {
		t.Fatal("Windows caption icons should stay sharp-cornered")
	}
	if !strings.Contains(restore, "<path") || !strings.Contains(restore, "<rect") {
		t.Fatal("restore icon should be a front square plus the back top and right edges")
	}
	for name, source := range map[string]string{"maximize": maximize, "restore": restore} {
		if _, err := woxsvg.Render(source, 24, 24); err != nil {
			t.Fatalf("render %s caption icon: %v", name, err)
		}
	}
}

func TestThemeEditorReusesTuneIcon(t *testing.T) {
	if Get(SettingsThemesEdit) != Get(ControlTune) {
		t.Fatal("theme editor icon does not reuse the tune icon")
	}
}

func TestHotkeyAndAliasActionsShareKeyboardIcon(t *testing.T) {
	if Get(ActionHotkey) != Get(ActionQueryAlias) {
		t.Fatal("hotkey and query-alias actions must share the keyboard glyph")
	}
}

func TestDefaultThemeCoversNames(t *testing.T) {
	theme := DefaultTheme()
	for _, name := range Names() {
		icon, ok := theme.Icon(name)
		if !ok || icon.IsEmpty() {
			t.Fatalf("default theme missing %q", name)
		}
	}

	source, err := os.ReadFile("names.go")
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, name := range Names() {
		registered[name] = true
	}
	constants := map[string]bool{}
	for _, match := range regexp.MustCompile(`=\s+"([^"]+)"`).FindAllSubmatch(source, -1) {
		name := string(match[1])
		constants[name] = true
		if !registered[name] {
			t.Errorf("catalog name %q is not in the default theme", name)
		}
	}
	for _, name := range Names() {
		if !constants[name] {
			t.Errorf("default theme name %q has no catalog constant", name)
		}
	}
}

func TestGetFallsBackToDefaultTheme(t *testing.T) {
	original := CurrentTheme()
	t.Cleanup(func() { SetTheme(original) })

	override := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/></svg>`)
	SetTheme(NewMapTheme("partial", map[string]common.WoxImage{
		ActionCopy: override,
	}))

	if got := Get(ActionCopy); got != override {
		t.Fatal("Get should return the current theme override")
	}
	fallback, ok := DefaultTheme().Icon(ActionOpen)
	if !ok {
		t.Fatal("default theme should contain action.open")
	}
	if got := Get(ActionOpen); got != fallback {
		t.Fatal("Get should fall back to the default theme for missing names")
	}
}

func TestSetThemeReplacesLookup(t *testing.T) {
	original := CurrentTheme()
	t.Cleanup(func() { SetTheme(original) })

	override := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24"/></svg>`)
	SetTheme(NewMapTheme("override", map[string]common.WoxImage{
		ActionCopy: override,
	}))
	if got := Get(ActionCopy); got != override {
		t.Fatal("SetTheme should change Get results")
	}

	SetTheme(nil)
	fallback, ok := DefaultTheme().Icon(ActionCopy)
	if !ok {
		t.Fatal("default theme should contain action.copy")
	}
	if got := Get(ActionCopy); got != fallback {
		t.Fatal("SetTheme(nil) should restore the default pack")
	}
	if CurrentTheme().Name() != DefaultTheme().Name() {
		t.Fatalf("CurrentTheme after nil = %q, want default", CurrentTheme().Name())
	}
}

func TestGetUnknownNameIsEmpty(t *testing.T) {
	if !Get("does.not.exist").IsEmpty() {
		t.Fatal("unknown catalog names should return an empty image")
	}
}

func TestActionIconsUseSemanticSVGColors(t *testing.T) {
	for name, icon := range defaultActionIcons {
		if icon.ImageType != common.WoxImageTypeSvg || !strings.HasPrefix(icon.ImageData, "<svg") {
			t.Fatalf("action icon %q is not an SVG", name)
		}
		if name == ActionExit {
			if !strings.Contains(icon.ImageData, `stroke="#EF4444"`) || strings.Contains(icon.ImageData, "var(--wox-theme-icon-color)") {
				t.Fatal("exit must retain its fixed red stroke across appearances")
			}
		} else if !strings.Contains(icon.ImageData, "var(--wox-theme-icon-color)") {
			t.Fatalf("action icon %q must use var(--wox-theme-icon-color)", name)
		}
		if err := renderCatalogSVG(icon.ImageData, 24, 24); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
	}
}

// pluginCreatorActionAssets maps the SVGs bundled with the wox-plugin-creator
// skill to the catalog verbs they mirror. Third-party plugins cannot call
// icons.Get, so the skill ships copies; this keeps them from drifting.
var pluginCreatorActionAssets = map[string]string{
	"execute.svg":                ActionExecute,
	"copy.svg":                   ActionCopy,
	"open.svg":                   ActionOpen,
	"open-containing-folder.svg": ActionOpenContainingFolder,
	"delete.svg":                 ActionDelete,
	"edit.svg":                   ActionEdit,
	"paste.svg":                  ActionPaste,
	"add.svg":                    ActionAdd,
	"search.svg":                 ActionSearch,
	"settings.svg":               ActionSettings,
}

func TestPluginCreatorSkillActionAssetsMatchCatalog(t *testing.T) {
	dir := filepath.Join("..", "..", "..", ".agents", "skills", "wox-plugin-creator", "assets", "iconify", "action")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("skill assets not present: %v", err)
	}
	for file, name := range pluginCreatorActionAssets {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Errorf("read %s: %v", file, err)
			continue
		}
		if got, want := strings.TrimSpace(string(data)), Get(name).ImageData; got != want {
			t.Errorf("%s differs from catalog %q; copy the catalog SVG into the skill asset", file, name)
		}
	}
}

func renderCatalogSVG(data string, width, height int) error {
	_, err := woxsvg.Render(strings.ReplaceAll(data, "var(--wox-theme-icon-color)", "#000000"), width, height)
	return err
}

func TestAboutMenuMonochromeIconsRender(t *testing.T) {
	for _, name := range []string{ActionFeedback, BrandRedditMonochrome, BrandDiscordMonochrome, BrandGithubMonochrome} {
		for _, color := range []string{"#000000", "#ffffff"} {
			svg := strings.ReplaceAll(Get(name).ImageData, "var(--wox-theme-icon-color)", color)
			img, err := woxsvg.Render(svg, 22, 22)
			if err != nil {
				t.Fatalf("render %s: %v", name, err)
			}
			visible := false
			for y := 0; y < 22; y++ {
				for x := 0; x < 22; x++ {
					pixel := img.RGBAAt(x, y)
					visible = visible || pixel.A > 0
					if pixel.A > 0 && (pixel.R != pixel.G || pixel.G != pixel.B) {
						t.Fatalf("%s has colored pixels", name)
					}
				}
			}
			if !visible {
				t.Fatalf("%s rendered blank", name)
			}
		}
	}
}
