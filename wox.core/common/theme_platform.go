package common

import (
	"encoding/json"
	"fmt"
)

const themePlatformOverrideVariantsField = "variants"
const themeBackgroundBlurField = "backgroundBlur"

// ThemePlatformOverride retains authored platform styles for storage and sync.
type ThemePlatformOverride map[string]json.RawMessage

// validateThemePlatformOverride applies a schema's own allowlist to a platform node.
func validateThemePlatformOverride(raw map[string]json.RawMessage, platformName string, styleFields map[string]bool) error {
	value, ok := raw[platformName]
	if !ok {
		return nil
	}
	if string(value) == "null" {
		return nil
	}

	var overrides map[string]json.RawMessage
	if err := json.Unmarshal(value, &overrides); err != nil {
		return fmt.Errorf("platform theme override %q must be a JSON object: %w", platformName, err)
	}

	for fieldName := range overrides {
		if fieldName == themePlatformOverrideVariantsField {
			if err := validateThemePlatformOverrideVariants(overrides[fieldName], platformName, styleFields); err != nil {
				return err
			}
			continue
		}
		if !styleFields[fieldName] {
			return fmt.Errorf("platform theme override %q contains non-style field %q", platformName, fieldName)
		}
	}

	return nil
}

func validateThemePlatformOverrideVariants(value json.RawMessage, platformName string, styleFields map[string]bool) error {
	if string(value) == "null" {
		return fmt.Errorf("platform theme override %q variants must be a JSON object", platformName)
	}

	var variants map[string]json.RawMessage
	if err := json.Unmarshal(value, &variants); err != nil {
		return fmt.Errorf("platform theme override %q variants must be a JSON object: %w", platformName, err)
	}
	if variants == nil {
		return fmt.Errorf("platform theme override %q variants must be a JSON object", platformName)
	}

	for variantName, variantValue := range variants {
		if string(variantValue) == "null" {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object", platformName, variantName)
		}

		var overrides map[string]json.RawMessage
		if err := json.Unmarshal(variantValue, &overrides); err != nil {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object: %w", platformName, variantName, err)
		}
		if overrides == nil {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object", platformName, variantName)
		}

		for fieldName := range overrides {
			if fieldName == themeBackgroundBlurField {
				if err := validateThemeCapabilityOverride(overrides[fieldName], platformName+"/"+variantName+"/"+fieldName, styleFields); err != nil {
					return err
				}
				continue
			}
			if !styleFields[fieldName] {
				return fmt.Errorf("platform theme override %q variant %q contains non-style field %q", platformName, variantName, fieldName)
			}
		}
	}

	return nil
}

// themeVariantLayers selects only capabilities nested under the matched desktop/OS variant.
// Capability order is supplied by the runtime, never by JSON map iteration.
func themeVariantLayers(node map[string]json.RawMessage, variant string, capabilities []string) ([]map[string]json.RawMessage, error) {
	layers := []map[string]json.RawMessage{node}
	var variants map[string]map[string]json.RawMessage
	if data := node["variants"]; len(data) > 0 {
		if err := json.Unmarshal(data, &variants); err != nil {
			return nil, err
		}
	}
	selected := variants[variant]
	layers = append(layers, selected)
	for _, capability := range capabilities {
		if capability != themeBackgroundBlurField {
			continue
		}
		var fields map[string]json.RawMessage
		if data := selected[capability]; len(data) > 0 {
			if err := json.Unmarshal(data, &fields); err != nil {
				return nil, err
			}
			layers = append(layers, fields)
		}
	}

	return layers, nil
}

// validateThemeCapabilityOverride accepts style fields directly below a desktop capability.
func validateThemeCapabilityOverride(data json.RawMessage, path string, styleFields map[string]bool) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("theme capability %q must be a JSON object: %w", path, err)
	}
	if fields == nil {
		return fmt.Errorf("theme capability %q must be a JSON object", path)
	}
	for name := range fields {
		if !styleFields[name] {
			return fmt.Errorf("theme capability %q contains non-style field %q", path, name)
		}
	}
	return nil
}
