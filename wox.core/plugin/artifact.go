package plugin

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type PluginArtifactKind string

const (
	PluginArtifactUnknown    PluginArtifactKind = ""
	PluginArtifactSingleFile PluginArtifactKind = "singleFile"
	PluginArtifactPackage    PluginArtifactKind = "package"
	PluginArtifactScript     PluginArtifactKind = "script"
)

const (
	PluginEntryModePackage    = "package"
	PluginEntryModeSingleFile = "singleFile"
)

// ClassifyPluginArtifact maps a store runtime and download URL to an install type.
// URL query strings are ignored; only the path suffix is used.
func ClassifyPluginArtifact(runtime Runtime, downloadURL string) (PluginArtifactKind, error) {
	ext := artifactURLExtension(downloadURL)
	normalizedRuntime := ConvertToRuntime(string(runtime))
	if normalizedRuntime == PLUGIN_RUNTIME_SCRIPT {
		if ext == ".wox" {
			return PluginArtifactUnknown, fmt.Errorf("SCRIPT plugins cannot be delivered as .wox packages")
		}
		return PluginArtifactScript, nil
	}
	if ext == "" {
		return PluginArtifactUnknown, fmt.Errorf("plugin download URL has no file extension: %s", downloadURL)
	}

	switch normalizedRuntime {
	case PLUGIN_RUNTIME_PYTHON:
		switch ext {
		case ".py":
			return PluginArtifactSingleFile, nil
		case ".wox":
			return PluginArtifactPackage, nil
		case ".js":
			return PluginArtifactUnknown, fmt.Errorf("PYTHON plugins cannot be delivered as .js files")
		default:
			return PluginArtifactUnknown, fmt.Errorf("unsupported PYTHON plugin download extension %s", ext)
		}
	case PLUGIN_RUNTIME_NODEJS:
		switch ext {
		case ".js":
			return PluginArtifactSingleFile, nil
		case ".wox":
			return PluginArtifactPackage, nil
		case ".py":
			return PluginArtifactUnknown, fmt.Errorf("NODEJS plugins cannot be delivered as .py files")
		default:
			return PluginArtifactUnknown, fmt.Errorf("unsupported NODEJS plugin download extension %s", ext)
		}
	case PLUGIN_RUNTIME_GO:
		if ext != ".wox" {
			return PluginArtifactUnknown, fmt.Errorf("GO plugins must be delivered as .wox packages")
		}
		return PluginArtifactPackage, nil
	default:
		return PluginArtifactUnknown, fmt.Errorf("unsupported plugin runtime: %s", runtime)
	}
}

func artifactURLExtension(downloadURL string) string {
	name := artifactFileName(downloadURL)
	if name == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "." {
		return ""
	}
	return ext
}

func artifactFileName(downloadURL string) string {
	trimmed := strings.TrimSpace(downloadURL)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" && (parsed.Scheme != "" || parsed.Host != "") {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	replaced := strings.ReplaceAll(trimmed, "\\", "/")
	return path.Base(replaced)
}

// ValidateStorePluginManifest rejects store rows that older Wox builds must not install.
func ValidateStorePluginManifest(manifest StorePluginManifest) error {
	kind, err := ClassifyPluginArtifact(manifest.Runtime, manifest.DownloadUrl)
	if err != nil {
		return err
	}
	if kind != PluginArtifactSingleFile {
		return nil
	}
	if strings.TrimSpace(manifest.MinWoxVersion) == "" {
		return fmt.Errorf("single-file plugin %s must declare MinWoxVersion", manifest.Id)
	}
	required, err := semver.NewVersion(manifest.MinWoxVersion)
	if err != nil {
		return fmt.Errorf("single-file plugin %s has invalid MinWoxVersion %q: %w", manifest.Id, manifest.MinWoxVersion, err)
	}
	floor, err := semver.NewVersion(SingleFilePluginMinWoxVersion)
	if err != nil {
		return fmt.Errorf("invalid SingleFilePluginMinWoxVersion %q: %w", SingleFilePluginMinWoxVersion, err)
	}
	if required.LessThan(floor) {
		return fmt.Errorf("single-file plugin %s MinWoxVersion %s is below %s", manifest.Id, required.String(), floor.String())
	}
	return nil
}

// ValidateSingleFileHeaderMatchesManifest checks ID, runtime, and version agreement.
func ValidateSingleFileHeaderMatchesManifest(header Metadata, manifest StorePluginManifest) error {
	if !strings.EqualFold(header.Id, manifest.Id) {
		return fmt.Errorf("single-file plugin header Id %s does not match store manifest Id %s", header.Id, manifest.Id)
	}
	if ConvertToRuntime(header.Runtime) != ConvertToRuntime(string(manifest.Runtime)) {
		return fmt.Errorf("single-file plugin header Runtime %s does not match store manifest Runtime %s", header.Runtime, manifest.Runtime)
	}
	headerVersion, headerErr := semver.NewVersion(header.Version)
	manifestVersion, manifestErr := semver.NewVersion(manifest.Version)
	if headerErr != nil || manifestErr != nil || headerVersion == nil || manifestVersion == nil || !headerVersion.Equal(manifestVersion) {
		return fmt.Errorf("single-file plugin header Version %s does not match store manifest Version %s", header.Version, manifest.Version)
	}
	return nil
}

// PluginEntryMode is the internal host load mode derived from plugin location.
func PluginEntryMode(metadata Metadata) string {
	if IsSingleFilePlugin(metadata) {
		return PluginEntryModeSingleFile
	}
	return PluginEntryModePackage
}

// InstalledPluginArtifactKind reports how an already-loaded plugin was delivered.
func InstalledPluginArtifactKind(instance *Instance) PluginArtifactKind {
	if instance == nil {
		return PluginArtifactUnknown
	}
	if IsSingleFilePlugin(instance.Metadata) {
		return PluginArtifactSingleFile
	}
	if strings.EqualFold(instance.Metadata.Runtime, string(PLUGIN_RUNTIME_SCRIPT)) {
		return PluginArtifactScript
	}
	return PluginArtifactPackage
}
