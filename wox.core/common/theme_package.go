package common

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"
)

const ThemePackageMaxBytes = 32 << 20

// ParseThemeDocument checks the version floor before decoding optional newer fields.
func ParseThemeDocument(data []byte, currentVersion string) (Theme, error) {
	var header struct{ ThemeName, MinWoxVersion string }
	if err := json.Unmarshal(data, &header); err != nil {
		return Theme{}, err
	}
	if err := (Theme{ThemeName: header.ThemeName, MinWoxVersion: header.MinWoxVersion}).EnsureWoxVersionSupported(currentVersion); err != nil {
		return Theme{}, err
	}
	var theme Theme
	err := json.Unmarshal(data, &theme)
	return theme, err
}

// ReadThemePackage validates the entire archive in memory before any installation writes.
func ReadThemePackage(filePath, currentVersion string) (Theme, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return Theme{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > ThemePackageMaxBytes {
		return Theme{}, fmt.Errorf("theme archive must be a regular file up to 32 MiB")
	}
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return Theme{}, err
	}
	defer archive.Close()
	if len(archive.File) > 128 {
		return Theme{}, fmt.Errorf("theme package contains too many entries")
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	remaining := int64(ThemePackageMaxBytes)
	for _, entry := range archive.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if !ValidThemeAssetPath(name) || seen[strings.ToLower(name)] || entry.Mode()&os.ModeSymlink != 0 {
			return Theme{}, fmt.Errorf("unsafe or duplicate theme entry %q", entry.Name)
		}
		seen[strings.ToLower(name)] = true
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() || entry.UncompressedSize64 > uint64(remaining) {
			return Theme{}, fmt.Errorf("invalid or oversized theme entry %q", name)
		}
		reader, err := entry.Open()
		if err != nil {
			return Theme{}, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, remaining+1))
		reader.Close()
		if readErr != nil {
			return Theme{}, readErr
		}
		remaining -= int64(len(data))
		if remaining < 0 {
			return Theme{}, fmt.Errorf("theme package exceeds 32 MiB")
		}
		files[name] = data
	}
	document, ok := files["theme.json"]
	if !ok {
		return Theme{}, fmt.Errorf("theme package requires root theme.json")
	}
	theme, err := ParseThemeDocument(document, currentVersion)
	if err != nil {
		return Theme{}, err
	}
	delete(files, "theme.json")
	theme.AssetFiles = files
	if err := theme.ValidateAssets(); err != nil {
		return Theme{}, err
	}
	return theme, nil
}

// ThemeAssetSources includes inactive platform variants so packages remain portable.
func (t Theme) ThemeAssetSources() ([]string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var sources []string
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "Source" {
					if source, ok := child.(string); ok && !seen[source] {
						seen[source] = true
						sources = append(sources, source)
					}
				} else {
					visit(child)
				}
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		}
	}
	visit(root)
	return sources, nil
}

// CanSyncWithoutAssets reports whether the document carries no image files, including inactive platform assets.
// Invalid documents are reported as not asset-free.
func (t Theme) CanSyncWithoutAssets() bool {
	if len(t.AssetFiles) > 0 {
		return false
	}
	sources, err := t.ThemeAssetSources()
	return err == nil && len(sources) == 0
}

// ValidateAssets bounds compressed and decoded memory and requires every declared resource.
func (t Theme) ValidateAssets() error {
	if !ValidThemeAssetPath(t.ThemeId) || strings.Contains(t.ThemeId, "/") {
		return fmt.Errorf("invalid theme id %q", t.ThemeId)
	}
	if len(t.AssetFiles) > 127 {
		return fmt.Errorf("too many theme assets")
	}
	total, pixels := 0, int64(0)
	seen := map[string]bool{}
	for name, data := range t.AssetFiles {
		if !ValidThemeAssetPath(name) || strings.EqualFold(name, "theme.json") || seen[strings.ToLower(name)] {
			return fmt.Errorf("invalid asset path %q", name)
		}
		seen[strings.ToLower(name)] = true
		total += len(data)
		if total > ThemePackageMaxBytes {
			return fmt.Errorf("theme assets exceed 32 MiB")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 {
			return fmt.Errorf("invalid theme image %q", name)
		}
		pixels += int64(config.Width) * int64(config.Height)
		if pixels > 16<<20 {
			return fmt.Errorf("theme images exceed 16 megapixels")
		}
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			return fmt.Errorf("invalid theme image %q: %w", name, err)
		}
	}
	sources, err := t.ThemeAssetSources()
	if err != nil {
		return err
	}
	for _, source := range sources {
		if _, ok := t.AssetFiles[source]; !ok {
			return fmt.Errorf("missing theme asset %q", source)
		}
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	return validateThemeSlices(document, t.AssetFiles)
}

// validateThemeSlices also visits inactive platform layers before accepting a portable package.
func validateThemeSlices(value any, assets map[string][]byte) error {
	switch node := value.(type) {
	case map[string]any:
		if node["Mode"] == "nineSlice" {
			data, _ := json.Marshal(node)
			var layer ThemeSurfaceImage
			if err := json.Unmarshal(data, &layer); err != nil {
				return err
			}
			config, _, err := image.DecodeConfig(bytes.NewReader(assets[layer.Source]))
			if err != nil {
				return err
			}
			if layer.Slice == nil || layer.Slice.Left+layer.Slice.Right >= config.Width || layer.Slice.Top+layer.Slice.Bottom >= config.Height {
				return fmt.Errorf("nineSlice exceeds source image %q", layer.Source)
			}
		}
		for _, child := range node {
			if err := validateThemeSlices(child, assets); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := validateThemeSlices(child, assets); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadThemeAssets uses an OS-confined root so resource symlinks cannot escape the theme directory.
func (t *Theme) LoadThemeAssets(directory string) error {
	sources, err := t.ThemeAssetSources()
	if err != nil || len(sources) == 0 {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	t.AssetFiles = map[string][]byte{}
	remaining := int64(ThemePackageMaxBytes)
	for _, source := range sources {
		if !ValidThemeAssetPath(source) {
			return fmt.Errorf("invalid theme asset %q", source)
		}
		file, err := root.Open(source)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, remaining+1))
		file.Close()
		if readErr != nil {
			return readErr
		}
		remaining -= int64(len(data))
		if remaining < 0 {
			return fmt.Errorf("theme assets exceed 32 MiB")
		}
		t.AssetFiles[source] = data
	}
	return t.ValidateAssets()
}
