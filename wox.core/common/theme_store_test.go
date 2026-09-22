package common

import (
	"strings"
	"testing"
)

func TestClassifyThemeArtifact(t *testing.T) {
	kind, err := ClassifyThemeArtifact("https://gist.githubusercontent.com/user/id/raw/theme.json?cache=1")
	if err != nil || kind != ThemeArtifactSingleFile {
		t.Fatalf("json kind = %s, %v", kind, err)
	}
	kind, err = ClassifyThemeArtifact("https://github.com/example/releases/download/v1/knit.wox-theme")
	if err != nil || kind != ThemeArtifactPackage {
		t.Fatalf("package kind = %s, %v", kind, err)
	}
	if _, err := ClassifyThemeArtifact("https://example.com/theme"); err == nil {
		t.Fatal("expected extension error")
	}
}

func TestThemeDocumentForStoreInstallRejectsImageAssets(t *testing.T) {
	manifest := StoreThemeManifest{Id: "theme-1", Name: "Knit", Version: "1.0.0", DownloadUrl: "https://example.com/theme.json"}
	document := []byte(`{"SchemaVersion":2,"ThemeId":"theme-1","ThemeName":"Knit","Version":"1.0.0","MinWoxVersion":"2.0.0","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ffffff","Surfaces":{"App":{"Background":{"Source":"frame.png","Mode":"stretch"}}}}`)
	if _, err := ThemeDocumentForStoreInstall(document, manifest, "2.4.5"); err == nil {
		t.Fatal("image theme was accepted as a single file")
	}
	plain := []byte(`{"SchemaVersion":2,"ThemeId":"theme-1","ThemeName":"Knit","Version":"1.0.0","MinWoxVersion":"2.0.0","BaseBackgroundColor":"#111111","BaseTextColor":"#ffffff","BaseAccentColor":"#ffffff"}`)
	theme, err := ThemeDocumentForStoreInstall(plain, manifest, "2.4.5")
	if err != nil || theme.ThemeId != "theme-1" {
		t.Fatalf("plain theme = %s, %v", theme.ThemeId, err)
	}
}

func TestStoreThemeColorsSwatch(t *testing.T) {
	colors := StoreThemeIconColors{Background: "#2D343A", Query: "transparent", Selected: "#394144", Outline: "#C7C2B3", OutlineWidth: 2}
	if !colors.HasColors() {
		t.Fatal("expected swatch colors")
	}
	image := colors.SwatchImage()
	if !strings.Contains(image.ImageData, "#2D343A") || !strings.Contains(image.ImageData, "#394144") || !strings.Contains(image.ImageData, "stroke=") {
		t.Fatalf("swatch = %s", image.ImageData)
	}
}

func TestStoreThemeManifestValidate(t *testing.T) {
	colors := StoreThemeIconColors{Background: "#111111", Query: "#222222", Selected: "#333333"}
	if err := (StoreThemeManifest{Id: "a", Name: "A", Version: "1.0.0", DownloadUrl: "https://example.com/a.json", IconColors: colors}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (StoreThemeManifest{Id: "a", Name: "A", Version: "1.0.0", DownloadUrl: "https://example.com/a.json", IconColors: StoreThemeIconColors{Background: "#111111", Query: "#222222"}}).Validate(); err == nil {
		t.Fatal("expected missing selected color")
	}
	if err := (StoreThemeManifest{Id: "a", Name: "A", Version: "1.0.0", DownloadUrl: "https://example.com/a.zip", IconColors: colors}).Validate(); err == nil {
		t.Fatal("expected download extension error")
	}
}

func TestThemeEnglishNames(t *testing.T) {
	translations := map[string]map[string]string{
		"en_US": {"theme_name": "Knit"},
		"zh_CN": {"theme_name": "织"},
	}
	manifest := StoreThemeManifest{Name: "i18n:theme_name", I18n: translations}
	theme := Theme{ThemeName: manifest.Name, I18n: translations}
	if manifest.GetNameEnUs() != "Knit" || theme.GetNameEnUs() != "Knit" {
		t.Fatal("store and installed themes must expose the English search name")
	}
	if got := (StoreThemeManifest{Name: "Omarchy"}).GetNameEnUs(); got != "Omarchy" {
		t.Fatalf("plain name = %q", got)
	}
}
