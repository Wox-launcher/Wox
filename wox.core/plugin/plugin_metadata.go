package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
)

func (l *pluginLifecycle) ParseMetadata(ctx context.Context, pluginDirectory string) (Metadata, error) {
	configPath := path.Join(pluginDirectory, "plugin.json")
	if _, statErr := os.Stat(configPath); statErr != nil {
		return Metadata{}, fmt.Errorf("missing plugin.json file in %s", configPath)
	}

	metadataJson, err := os.ReadFile(configPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read plugin.json file: %w", err)
	}

	var metadata Metadata
	unmarshalErr := json.Unmarshal(metadataJson, &metadata)
	if unmarshalErr != nil {
		return Metadata{}, fmt.Errorf("failed to unmarshal plugin.json file (%s): %w", pluginDirectory, unmarshalErr)
	}

	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing trigger keywords in plugin.json file (%s)", pluginDirectory)
	}
	if !IsSupportedRuntime(metadata.Runtime) {
		return Metadata{}, fmt.Errorf("unsupported runtime in plugin.json file (%s), runtime=%s", pluginDirectory, metadata.Runtime)
	}
	if !IsAllSupportedOS(metadata.SupportedOS) {
		return Metadata{}, fmt.Errorf("unsupported os in plugin.json file (%s), os=%s", pluginDirectory, metadata.SupportedOS)
	}
	if err := metadata.ValidateGlances(); err != nil {
		// Global Glance selections are persisted as PluginId + GlanceId, so the
		// backend must reject ambiguous plugin-local ids before settings can point
		// at a candidate that cannot be resolved deterministically.
		return Metadata{}, fmt.Errorf("invalid glances in plugin.json file (%s): %w", pluginDirectory, err)
	}

	metadata.Directory = pluginDirectory
	metadata.LoadPluginI18nFromDirectory(ctx)

	return metadata, nil
}

// validateAndSetScriptMetadataDefaults validates required fields and sets default values
func (l *pluginLifecycle) validateAndSetScriptMetadataDefaults(metadata Metadata) (Metadata, error) {
	// Validate required fields
	if metadata.Id == "" {
		return Metadata{}, fmt.Errorf("missing required field: Id")
	}
	if metadata.GetName(context.Background()) == "" {
		return Metadata{}, fmt.Errorf("missing required field: Name")
	}
	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing required field: TriggerKeywords")
	}

	// Set default values
	if metadata.Author == "" {
		metadata.Author = "Unknown"
	}
	if metadata.GetDescription(context.Background()) == "" {
		metadata.Description = "A script plugin"
	}
	if metadata.Icon == "" {
		metadata.Icon = "emoji:📝"
	}
	if metadata.MinWoxVersion == "" {
		// Script plugins can omit MinWoxVersion in their inline metadata. Use the
		// same default as packaged plugins so the later compatibility check has a
		// concrete semantic floor instead of treating the field as absent.
		metadata.MinWoxVersion = defaultMinWoxVersion
	}
	if metadata.Version == "" {
		metadata.Version = "1.0.0"
	}

	// Set supported OS to all platforms by default
	if len(metadata.SupportedOS) == 0 {
		metadata.SupportedOS = []string{"Windows", "Linux", "Macos"}
	}

	return metadata, nil
}
