// Package manifest reads plugin.json for the flow plugin hosts.
// Script plugins and .NET plugins share this parser and then diverge:
// the script host runs Python, Node, and executable plugins, and the
// dotnet host is the place for the C# and F# host.
package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"

	"gopkg.in/yaml.v3"
)

const (
	// DirectoryName is the reserved child of the user plugins directory.
	DirectoryName = "flow-jsonrpc"
	// RuntimeJSONRPC is the script host runtime. It is not a Wox plugin.json runtime.
	RuntimeJSONRPC plugin.Runtime = "FLOWJSONRPC"
	// RuntimeDotNet is the C# and F# host runtime. Its host is not registered yet.
	RuntimeDotNet plugin.Runtime = "FLOWDOTNET"
	// KindScript is a Python, Node, or executable plugin.
	KindScript = "script"
	// KindDotNet is a C# or F# plugin.
	KindDotNet = "dotnet"
	// SettingBool is a checkbox value sent to the plugin as JSON true or false.
	SettingBool = "bool"
	// SettingString is a text or dropdown value sent as a JSON string.
	SettingString = "string"
	// minWoxVersion matches plugin.defaultMinWoxVersion. The flow hosts set it
	// because their generated metadata does not come from ParseMetadata.
	minWoxVersion = "2.0.0"
)

// CollectionDirectory is plugins/flow-jsonrpc. Each child directory is one plugin.
func CollectionDirectory() string {
	return filepath.Clean(filepath.Join(util.GetLocation().GetPluginDirectory(), DirectoryName))
}

// Descriptor is one plugin directory translated into Wox metadata.
type Descriptor struct {
	Metadata     plugin.Metadata
	Language     string
	Kind         string
	SettingKinds map[string]string
}

type unsupportedLanguageError struct {
	Language string
}

func (e *unsupportedLanguageError) Error() string {
	return fmt.Sprintf("language %q is not supported by the flow plugin loader", e.Language)
}

// LanguageKind reports which flow host owns a plugin.json Language value.
func LanguageKind(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "python", "javascript", "typescript", "executable":
		return KindScript
	case "csharp", "c#", "fsharp", "f#":
		return KindDotNet
	default:
		return ""
	}
}

// Parse reads plugin.json and an optional settings template from one plugin directory.
func Parse(directory string) (Descriptor, error) {
	raw, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		return Descriptor{}, fmt.Errorf("read plugin.json: %w", err)
	}
	if len(raw) > 0 && raw[0] == 0xEF && len(raw) >= 3 {
		raw = raw[3:]
	}

	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return Descriptor{}, fmt.Errorf("parse plugin.json: %w", err)
	}

	language := strings.ToLower(flowFieldString(document, "Language"))
	kind := LanguageKind(language)
	if kind == "" {
		if language == "" {
			return Descriptor{}, errors.New("plugin.json is missing Language")
		}
		return Descriptor{}, &unsupportedLanguageError{Language: language}
	}

	id := flowFieldString(document, "ID", "Id")
	if id == "" {
		return Descriptor{}, errors.New("plugin.json is missing ID")
	}
	name := flowFieldString(document, "Name")
	if name == "" {
		return Descriptor{}, errors.New("plugin.json is missing Name")
	}
	entry := flowFieldString(document, "ExecuteFileName", "ExecuteFilePath")
	if entry == "" {
		return Descriptor{}, errors.New("plugin.json is missing ExecuteFileName")
	}
	entryPath, err := flowPathInside(directory, entry)
	if err != nil {
		return Descriptor{}, err
	}
	if _, err := os.Stat(entryPath); err != nil {
		return Descriptor{}, fmt.Errorf("entry %s: %w", entry, err)
	}

	version := flowFieldString(document, "Version")
	if version == "" {
		version = "1.0.0"
	}
	keywords := flowTriggerKeywords(document)
	icon := flowPluginIcon(directory, flowFieldString(document, "IcoPath", "IconPath"))
	settings, kinds, err := parseFlowSettingsTemplate(directory)
	if err != nil {
		return Descriptor{}, err
	}
	runtimeName := string(RuntimeJSONRPC)
	if kind == KindDotNet {
		runtimeName = string(RuntimeDotNet)
	}

	return Descriptor{
		Metadata: plugin.Metadata{
			Id:              id,
			Name:            common.I18nString(name),
			Author:          flowFieldString(document, "Author"),
			Version:         version,
			MinWoxVersion:   minWoxVersion,
			Runtime:         runtimeName,
			Description:     common.I18nString(flowFieldString(document, "Description")),
			Icon:            icon,
			Website:         flowFieldString(document, "Website"),
			Entry:           entry,
			TriggerKeywords: keywords,
			SupportedOS:     []string{"Windows", "Darwin", "Linux"},
			Features: []plugin.MetadataFeature{{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			}},
			SettingDefinitions: settings,
			Directory:          directory,
		},
		Language:     language,
		Kind:         kind,
		SettingKinds: kinds,
	}, nil
}

// LoadDirectory reads every plugin directory under root.
// The collection root itself is not a plugin. Unsupported languages are skipped.
func LoadDirectory(ctx context.Context, root string) ([]Descriptor, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read flow plugin directory: %w", err)
	}

	var descriptors []Descriptor
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		directory := filepath.Join(root, entry.Name())
		descriptor, parseErr := Parse(directory)
		if parseErr != nil {
			var unsupported *unsupportedLanguageError
			if errors.As(parseErr, &unsupported) {
				util.GetLogger().Warn(ctx, fmt.Sprintf("skip flow plugin %s: %s", directory, parseErr.Error()))
				continue
			}
			util.GetLogger().Error(ctx, fmt.Sprintf("skip flow plugin %s: %s", directory, parseErr.Error()))
			continue
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("found flow plugin %s (%s)", descriptor.Metadata.GetName(ctx), descriptor.Language))
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

func flowTriggerKeywords(document map[string]any) []string {
	var keywords []string
	seen := map[string]bool{}
	add := func(keyword string) {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" || keyword == "*" {
			keyword = "*"
		}
		if seen[keyword] {
			return
		}
		seen[keyword] = true
		keywords = append(keywords, keyword)
	}
	for _, keyword := range flowFieldStrings(document, "ActionKeyword") {
		add(keyword)
	}
	for _, keyword := range flowFieldStrings(document, "ActionKeywords") {
		add(keyword)
	}
	if len(keywords) == 0 {
		return []string{"*"}
	}
	return keywords
}

func flowPluginIcon(directory, icoPath string) string {
	resolved := resolveFlowAssetPath(directory, icoPath)
	if resolved == "" {
		return ""
	}
	if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
		return common.NewWoxImageUrl(resolved).String()
	}
	return common.NewWoxImageAbsolutePath(resolved).String()
}

func resolveFlowAssetPath(directory, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(directory, filepath.FromSlash(value))
}

func flowPathInside(directory, name string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(directory, filepath.FromSlash(name)))
	relative, err := filepath.Rel(filepath.Clean(directory), cleaned)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("entry %s escapes the plugin directory", name)
	}
	return cleaned, nil
}

func flowFieldString(document map[string]any, names ...string) string {
	value, ok := flowField(document, names...)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func flowFieldStrings(document map[string]any, names ...string) []string {
	value, ok := flowField(document, names...)
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	case []any:
		var items []string
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				items = append(items, text)
			}
		}
		return items
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func flowField(document map[string]any, names ...string) (any, bool) {
	folded := make(map[string]any, len(document))
	for key, value := range document {
		folded[strings.ToLower(key)] = value
	}
	for _, name := range names {
		value, ok := folded[strings.ToLower(name)]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func parseFlowSettingsTemplate(directory string) (definition.PluginSettingDefinitions, map[string]string, error) {
	var raw []byte
	var err error
	for _, name := range []string{"SettingsTemplate.yaml", "SettingsTemplate.yml"} {
		raw, err = os.ReadFile(filepath.Join(directory, name))
		if err == nil {
			break
		}
		if os.IsNotExist(err) {
			raw = nil
			continue
		}
		return nil, nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(raw) == 0 {
		return nil, map[string]string{}, nil
	}

	var document map[string]any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, nil, fmt.Errorf("parse settings template: %w", err)
	}
	body, _ := flowField(document, "body")
	items, _ := body.([]any)
	definitions := definition.PluginSettingDefinitions{}
	kinds := map[string]string{}
	for _, item := range items {
		object, _ := item.(map[string]any)
		if len(object) == 0 {
			continue
		}
		control, kind, ok := flowSettingControl(object)
		if !ok {
			continue
		}
		definitions = append(definitions, control)
		kinds[control.Value.GetKey()] = kind
	}
	return definitions, kinds, nil
}

func flowSettingControl(item map[string]any) (definition.PluginSettingDefinitionItem, string, bool) {
	controlType := strings.ToLower(flowFieldString(item, "type"))
	attributes, _ := flowField(item, "attributes")
	fields, _ := attributes.(map[string]any)
	if len(fields) == 0 {
		return definition.PluginSettingDefinitionItem{}, "", false
	}
	name := flowFieldString(fields, "name", "key")
	if name == "" {
		return definition.PluginSettingDefinitionItem{}, "", false
	}
	label := flowFieldString(fields, "label", "title")
	if label == "" {
		label = name
	}
	tooltip := flowFieldString(fields, "description", "tooltip")
	defaultValue, hasDefault := flowField(fields, "defaultValue", "default_value", "default")

	switch controlType {
	case "checkbox":
		checked := false
		if hasDefault {
			checked = flowTruthy(defaultValue)
		}
		value := "false"
		if checked {
			value = "true"
		}
		return definition.PluginSettingDefinitionItem{
			Type: definition.PluginSettingDefinitionTypeCheckBox,
			Value: &definition.PluginSettingValueCheckBox{
				Key:          name,
				Label:        label,
				Tooltip:      tooltip,
				DefaultValue: value,
			},
		}, SettingBool, true
	case "dropdown", "select", "combobox":
		options := flowSelectOptions(fields)
		if len(options) == 0 {
			return definition.PluginSettingDefinitionItem{}, "", false
		}
		selected := ""
		if hasDefault {
			selected = flowScalarString(defaultValue)
		}
		if selected == "" {
			selected = options[0].Value
		}
		return definition.PluginSettingDefinitionItem{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          name,
				Label:        label,
				Tooltip:      tooltip,
				DefaultValue: selected,
				Options:      options,
			},
		}, SettingString, true
	case "password", "passwordbox":
		return definition.PluginSettingDefinitionItem{
			Type: definition.PluginSettingDefinitionTypePassword,
			Value: &definition.PluginSettingValueTextBox{
				Key:          name,
				Label:        label,
				Tooltip:      tooltip,
				DefaultValue: flowScalarString(defaultValue),
			},
		}, SettingString, true
	case "textarea":
		return definition.PluginSettingDefinitionItem{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          name,
				Label:        label,
				Tooltip:      tooltip,
				DefaultValue: flowScalarString(defaultValue),
				MaxLines:     6,
			},
		}, SettingString, true
	case "textbox", "input", "text", "inputwithfolderbtn":
		return definition.PluginSettingDefinitionItem{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          name,
				Label:        label,
				Tooltip:      tooltip,
				DefaultValue: flowScalarString(defaultValue),
			},
		}, SettingString, true
	default:
		return definition.PluginSettingDefinitionItem{}, "", false
	}
}

func flowSelectOptions(fields map[string]any) []definition.PluginSettingValueSelectOption {
	raw, ok := flowField(fields, "options")
	if !ok {
		return nil
	}
	items, _ := raw.([]any)
	var options []definition.PluginSettingValueSelectOption
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text == "" {
				continue
			}
			options = append(options, definition.PluginSettingValueSelectOption{Label: text, Value: text})
		case map[string]any:
			value := flowFieldString(typed, "value", "name")
			label := flowFieldString(typed, "label", "title")
			if value == "" {
				value = label
			}
			if label == "" {
				label = value
			}
			if value == "" {
				continue
			}
			options = append(options, definition.PluginSettingValueSelectOption{Label: label, Value: value})
		}
	}
	return options
}

func flowTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || typed == "1"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func flowScalarString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprint(typed)
	default:
		return fmt.Sprint(typed)
	}
}
