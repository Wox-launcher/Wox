package common

import (
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
)

type themeSchema struct {
	colors  func(Theme) map[string]string
	parse   func([]byte) (Theme, error)
	marshal func(Theme) ([]byte, error)
	resolve func(Theme, string, string) (Theme, error)
}

var themeSchemas = map[int]themeSchema{}

// registerThemeSchema is called only during package initialization by each schema file.
func registerThemeSchema(version int, schema themeSchema) {
	if version < 1 || schema.parse == nil || schema.marshal == nil {
		panic("invalid theme schema registration")
	}
	if _, exists := themeSchemas[version]; exists {
		panic("duplicate theme schema registration")
	}
	themeSchemas[version] = schema
}

// UnmarshalJSON dispatches by version; absent and historical zero versions mean v1.
func (t *Theme) UnmarshalJSON(data []byte) error {
	var header struct {
		SchemaVersion int
		MinWoxVersion string
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.SchemaVersion == 0 {
		header.SchemaVersion = 1
	}
	schema, ok := themeSchemas[header.SchemaVersion]
	if !ok {
		return fmt.Errorf("unsupported theme SchemaVersion %d", header.SchemaVersion)
	}
	if header.MinWoxVersion != "" {
		if _, err := semver.NewVersion(header.MinWoxVersion); err != nil {
			return fmt.Errorf("invalid theme MinWoxVersion %q: %w", header.MinWoxVersion, err)
		}
	}
	parsed, err := schema.parse(data)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// MarshalJSON delegates storage semantics to the originating schema.
func (t Theme) MarshalJSON() ([]byte, error) {
	version := t.SchemaVersion
	if version == 0 {
		version = 1
	}
	schema, ok := themeSchemas[version]
	if !ok {
		return nil, fmt.Errorf("unsupported theme SchemaVersion %d", version)
	}
	return schema.marshal(t)
}

// HasAuthoredStyles distinguishes sparse authored themes from legacy flat values.
func (t Theme) HasAuthoredStyles() bool { return t.source != nil }

// UsesCustomWindowChrome reports authored AppBorderColor, AppBorderWidth, or
// AppBorderRadius. Resolved v2 color defaults do not count, so ordinary themes
// keep system window material.
func (t Theme) UsesCustomWindowChrome() bool {
	if source, ok := t.source.(*themeV2Source); ok {
		return source.appWindowChrome
	}
	return t.AppBorderWidth != nil || t.AppBorderRadius != nil
}

// ResolveForTarget lets each schema apply platform overrides before its own defaults.
func (t Theme) ResolveForTarget(platform, variant string) (Theme, error) {
	version := t.SchemaVersion
	if version == 0 {
		version = 1
	}
	schema, ok := themeSchemas[version]
	if !ok {
		return Theme{}, fmt.Errorf("unsupported theme SchemaVersion %d", version)
	}
	if schema.resolve == nil {
		return t, nil
	}
	return schema.resolve(t, platform, variant)
}

// EnsureWoxVersionSupported uses the same semantic-version floor as plugins.
func (t Theme) EnsureWoxVersionSupported(current string) error {
	minimum := t.MinWoxVersion
	if minimum == "" {
		minimum = "2.0.0"
	}
	required, err := semver.NewVersion(minimum)
	if err != nil {
		return fmt.Errorf("theme %s has invalid MinWoxVersion %q: %w", t.ThemeName, minimum, err)
	}
	running, err := semver.NewVersion(current)
	if err != nil {
		return fmt.Errorf("invalid current Wox version %q: %w", current, err)
	}
	if required.GreaterThan(running) {
		return fmt.Errorf("theme %s requires Wox %s or later, current Wox version is %s", t.ThemeName, required, running)
	}
	return nil
}

// ResolvedColors exposes additional schema colors without extending the frozen v1 wire format.
// Callers must treat the returned map as read-only.
func (t Theme) ResolvedColors() map[string]string {
	if schema, ok := themeSchemas[t.SchemaVersion]; ok && schema.colors != nil {
		return schema.colors(t)
	}
	return nil
}
