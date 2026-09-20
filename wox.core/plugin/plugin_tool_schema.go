package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

type compiledPluginToolSchema struct {
	raw      map[string]any
	resolved *jsonschema.Resolved
}

func compilePluginToolSchema(raw map[string]any, rejectAdditionalProperties bool) (*compiledPluginToolSchema, *PluginToolError) {
	copied, err := copyJSONObject(raw)
	if err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, err.Error())
	}
	if err := validatePluginToolSchemaDocument(copied); err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, err.Error())
	}

	data, err := json.Marshal(copied)
	if err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, fmt.Sprintf("invalid schema: %s", err.Error()))
	}
	if len(data) > pluginToolMaxSchemaBytes {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, fmt.Sprintf("schema exceeds %d bytes", pluginToolMaxSchemaBytes))
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, fmt.Sprintf("invalid JSON Schema: %s", err.Error()))
	}
	if schema.Type != "" && schema.Type != "object" {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, "schema type must be object")
	}
	schema.Type = "object"
	if rejectAdditionalProperties && schema.AdditionalProperties == nil {
		schema.AdditionalProperties = jsonschemaFalse()
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, fmt.Sprintf("schema is not supported: %s", err.Error()))
	}

	normalized, err := schemaToObject(&schema)
	if err != nil {
		return nil, newPluginToolError(PluginToolErrorInvalidRegistration, err.Error())
	}
	return &compiledPluginToolSchema{raw: normalized, resolved: resolved}, nil
}

func jsonschemaFalse() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

func schemaToObject(schema *jsonschema.Schema) (map[string]any, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("schema cannot be copied: %w", err)
	}
	var copied map[string]any
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, fmt.Errorf("schema cannot be copied: %w", err)
	}
	if copied == nil {
		copied = map[string]any{}
	}
	return copied, nil
}

func validatePluginToolSchemaDocument(raw map[string]any) error {
	if raw == nil {
		return fmt.Errorf("schema is required")
	}
	if err := rejectExternalSchemaRefs(raw, 0); err != nil {
		return err
	}
	return nil
}

func rejectExternalSchemaRefs(value any, depth int) error {
	if depth > pluginToolMaxSchemaDepth {
		return fmt.Errorf("schema exceeds max depth %d", pluginToolMaxSchemaDepth)
	}
	switch typed := value.(type) {
	case map[string]any:
		if ref, ok := typed["$ref"].(string); ok && strings.TrimSpace(ref) != "" && !strings.HasPrefix(ref, "#") {
			return fmt.Errorf("schema $ref must be an in-document pointer: %s", ref)
		}
		for _, child := range typed {
			if err := rejectExternalSchemaRefs(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectExternalSchemaRefs(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyJSONObject(raw map[string]any) (map[string]any, error) {
	if raw == nil {
		return nil, fmt.Errorf("schema is required")
	}
	normalized, err := normalizeJSONObject(raw)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeJSONObject(raw map[string]any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("value is not valid JSON: %w", err)
	}
	if len(data) > pluginToolMaxArgumentsBytes {
		return nil, fmt.Errorf("JSON payload exceeds %d bytes", pluginToolMaxArgumentsBytes)
	}
	var copied map[string]any
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, fmt.Errorf("value is not a JSON object: %w", err)
	}
	if copied == nil {
		copied = map[string]any{}
	}
	return copied, nil
}

func validateAgainstSchema(resolved *jsonschema.Resolved, value map[string]any, invalidCode string) *PluginToolError {
	if resolved == nil {
		return newPluginToolError(invalidCode, "schema is not compiled")
	}
	if err := resolved.Validate(value); err != nil {
		return newPluginToolError(invalidCode, err.Error())
	}
	return nil
}
