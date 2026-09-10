package icons

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"wox/common"
	woxsvg "wox/util/svg"
)

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

func TestActionIconsAreThemeAdaptiveSVGs(t *testing.T) {
	for name, icon := range defaultActionIcons {
		if icon.ImageType != common.WoxImageTypeSvg || !strings.HasPrefix(icon.ImageData, "<svg") {
			t.Fatalf("action icon %q is not an SVG", name)
		}
		if !strings.Contains(icon.ImageData, "var(--wox-theme-icon-color)") {
			t.Fatalf("action icon %q must use var(--wox-theme-icon-color)", name)
		}
		if err := renderCatalogSVG(icon.ImageData, 24, 24); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
	}
}

func renderCatalogSVG(data string, width, height int) error {
	_, err := woxsvg.Render(strings.ReplaceAll(data, "var(--wox-theme-icon-color)", "#000000"), width, height)
	return err
}
