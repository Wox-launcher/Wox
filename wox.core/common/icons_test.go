package common

import (
	"strings"
	"testing"
)

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
