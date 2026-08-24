package common

import (
	"strings"
	"testing"
	woxsvg "wox/util/svg"
)

func TestStaticActivityIconsRenderAsSVG(t *testing.T) {
	for name, icon := range map[string]WoxImage{"media playing": MediaPlayingIcon, "loading": LoadingIcon} {
		if icon.ImageType != WoxImageTypeSvg {
			t.Fatalf("%s icon type = %q, want svg", name, icon.ImageType)
		}
		if _, err := woxsvg.Render(icon.ImageData, 48, 48); err != nil {
			t.Fatalf("render %s icon: %v", name, err)
		}
	}
}

func TestUIIconsAreCategorizedSVGs(t *testing.T) {
	categories := map[string]bool{}
	for name, icon := range uiIcons {
		category, _, ok := strings.Cut(name, ".")
		if !ok || icon.ImageType != WoxImageTypeSvg || !strings.HasPrefix(icon.ImageData, "<svg") {
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
	maximize := UIIcon("control.window-maximize").ImageData
	restore := UIIcon("control.window-restore").ImageData
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
	if UIIcon("settings.themes.edit") != UIIcon("control.tune") {
		t.Fatal("theme editor icon does not reuse the tune icon")
	}
}
