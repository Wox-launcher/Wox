package common

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"wox/i18n"

	"github.com/Masterminds/semver/v3"
)

// ThemeArtifactKind is how a store theme is downloaded.
type ThemeArtifactKind string

const (
	ThemeArtifactSingleFile ThemeArtifactKind = "singleFile"
	ThemeArtifactPackage    ThemeArtifactKind = "package"
)

// StoreThemeManifest is one theme-store catalog entry.
// The theme document itself is downloaded from DownloadUrl.
type StoreThemeManifest struct {
	Id             string
	Name           string
	Author         string
	Version        string
	MinWoxVersion  string
	Description    string
	ImageTheme     bool
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	DateCreated    string
	DateUpdated    string
	IconColors     StoreThemeIconColors
	I18n           map[string]map[string]string
}

// StoreThemeIconColors is the palette used to draw a catalog icon without downloading the theme.
type StoreThemeIconColors struct {
	Background   string
	Query        string
	Selected     string
	Outline      string `json:",omitempty"`
	OutlineWidth int    `json:",omitempty"`
}

func (m StoreThemeManifest) GetName(ctx context.Context) string {
	return translateStoreThemeText(ctx, m.Name, m.I18n)
}

func (m StoreThemeManifest) GetDescription(ctx context.Context) string {
	return translateStoreThemeText(ctx, m.Description, m.I18n)
}

func (m StoreThemeManifest) GetNameEnUs() string {
	return themeEnglishName(m.Name, m.I18n)
}

// themeEnglishName keeps website queries searchable regardless of the active language.
func themeEnglishName(name string, translations map[string]map[string]string) string {
	if !strings.HasPrefix(name, "i18n:") {
		return name
	}
	if translated := translations["en_US"][strings.TrimPrefix(name, "i18n:")]; translated != "" {
		return translated
	}
	return i18n.GetI18nManager().TranslateWoxEnUs(context.Background(), name)
}

// HasColors reports whether Background, Query, and Selected are all present.
// A partial palette is rejected so the catalog icon is not filled with defaults.
func (c StoreThemeIconColors) HasColors() bool {
	return strings.TrimSpace(c.Background) != "" && strings.TrimSpace(c.Query) != "" && strings.TrimSpace(c.Selected) != ""
}

// SwatchImage draws the Settings catalog icon from the store palette.
func (c StoreThemeIconColors) SwatchImage() WoxImage {
	return NewWoxImageSvg(themeSwatchSVG(themeSwatchColors{
		Background:   c.Background,
		Query:        c.Query,
		Selected:     c.Selected,
		Outline:      c.Outline,
		OutlineWidth: ThemeSwatchOutlineWidth(c.OutlineWidth),
	}))
}

func translateStoreThemeText(ctx context.Context, text string, translations map[string]map[string]string) string {
	if !strings.HasPrefix(text, "i18n:") {
		return text
	}
	manager := i18n.GetI18nManager()
	if translated := manager.TranslateI18nMap(ctx, text, translations); translated != text {
		return translated
	}
	return manager.TranslateWox(ctx, text)
}

// Validate checks the catalog fields required before a download starts.
func (m StoreThemeManifest) Validate() error {
	if strings.TrimSpace(m.Id) == "" || strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.DownloadUrl) == "" {
		return fmt.Errorf("theme store entry %q is missing id, name, version, or download URL", m.Id)
	}
	if _, err := semver.NewVersion(m.Version); err != nil {
		return fmt.Errorf("theme %s has invalid version %q", m.Id, m.Version)
	}
	if !m.IconColors.HasColors() {
		return fmt.Errorf("theme %s is missing IconColors.Background, IconColors.Query, or IconColors.Selected", m.Id)
	}
	_, err := ClassifyThemeArtifact(m.DownloadUrl)
	return err
}

// ClassifyThemeArtifact decides single-file JSON versus a .wox-theme package from the URL path.
// Query strings are ignored. Image themes cannot use the JSON form.
func ClassifyThemeArtifact(downloadURL string) (ThemeArtifactKind, error) {
	switch themeArtifactExtension(downloadURL) {
	case ".json":
		return ThemeArtifactSingleFile, nil
	case ".wox-theme":
		return ThemeArtifactPackage, nil
	default:
		return "", fmt.Errorf("theme download URL must end with .json or .wox-theme")
	}
}

func themeArtifactExtension(downloadURL string) string {
	trimmed := strings.TrimSpace(downloadURL)
	if trimmed == "" {
		return ""
	}
	name := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		name = path.Base(parsed.Path)
	}
	ext := strings.ToLower(path.Ext(name))
	if ext == "." {
		return ""
	}
	return ext
}

// ThemeDocumentForStoreInstall parses a single-file theme and rejects documents that need packaged images.
func ThemeDocumentForStoreInstall(data []byte, manifest StoreThemeManifest, currentVersion string) (Theme, error) {
	theme, err := ParseThemeDocument(data, currentVersion)
	if err != nil {
		return Theme{}, err
	}
	if strings.TrimSpace(theme.ThemeId) == "" {
		theme.ThemeId = manifest.Id
	}
	if theme.ThemeId != manifest.Id {
		return Theme{}, fmt.Errorf("theme id %s does not match store entry %s", theme.ThemeId, manifest.Id)
	}
	sources, err := theme.ThemeAssetSources()
	if err != nil {
		return Theme{}, err
	}
	if len(sources) > 0 {
		return Theme{}, fmt.Errorf("theme %s references image assets and must be published as a .wox-theme package", manifest.Id)
	}
	return theme, nil
}
