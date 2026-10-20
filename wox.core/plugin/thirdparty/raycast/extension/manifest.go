// Package extension adapts locally built Raycast commands to existing Wox plugin capabilities.
package extension

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/plugin/thirdparty/raycast/brand"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
)

const DirectoryName = "raycast"
const Runtime plugin.Runtime = "RAYCAST"

var safeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

type Preference struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    *bool  `json:"required"`
	Optional    bool   `json:"optional"`
	Placeholder string `json:"placeholder"`
	Default     any    `json:"default"`
	Data        []struct {
		Title string `json:"title"`
		Value string `json:"value"`
	} `json:"data"`
}

type Command struct {
	Name              string       `json:"name"`
	Title             string       `json:"title"`
	Description       string       `json:"description"`
	Mode              string       `json:"mode"`
	Icon              string       `json:"icon"`
	Interval          string       `json:"interval"`
	DisabledByDefault bool         `json:"disabledByDefault"`
	Preferences       []Preference `json:"preferences"`
	Arguments         []Preference `json:"arguments"`
}

// usesSearchArgument maps a single text argument onto the ordinary Wox search input.
func (c Command) usesSearchArgument() bool {
	return len(c.Arguments) == 1 && c.Arguments[0].Type == "text"
}

func (c Command) needsSearchInput(search string) bool {
	return c.usesSearchArgument() && (c.Arguments[0].Required == nil || *c.Arguments[0].Required) && strings.TrimSpace(search) == ""
}

type Manifest struct {
	Name        string       `json:"name"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	Owner       string       `json:"owner"`
	Version     string       `json:"version"`
	Icon        string       `json:"icon"`
	Platforms   []string     `json:"platforms"`
	Commands    []Command    `json:"commands"`
	Preferences []Preference `json:"preferences"`
}

// Descriptor keeps the original manifest alongside Wox's catalog representation.
type Descriptor struct {
	Manifest Manifest
	Metadata plugin.Metadata
	Commands []Command
}

func CollectionDirectory() string {
	return filepath.Join(util.GetLocation().GetPluginDirectory(), DirectoryName)
}

// Parse validates an installed extension directory and its built commands without executing code.
func Parse(path string) (Descriptor, error) {
	directory, err := filepath.Abs(path)
	if err != nil {
		return Descriptor{}, err
	}
	raw, err := readManifest(directory)
	if err != nil {
		return Descriptor{}, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Descriptor{}, err
	}
	if !safeName.MatchString(m.Name) || !safeName.MatchString(m.Author) || (m.Owner != "" && !safeName.MatchString(m.Owner)) || m.Title == "" {
		return Descriptor{}, fmt.Errorf("Raycast manifest requires a safe name, author and title")
	}
	owner := m.Owner
	if owner == "" {
		owner = m.Author
	}
	d := Descriptor{Manifest: m}
	seen := map[string]bool{}
	for _, c := range m.Commands {
		if !safeName.MatchString(c.Name) || seen[strings.ToLower(c.Name)] {
			return Descriptor{}, fmt.Errorf("invalid or repeated Raycast command: %s", c.Name)
		}
		seen[strings.ToLower(c.Name)] = true
		// Filter once so discovery, command hints and execution share the same enabled commands.
		// No background scheduler or menu bar is created.
		if c.DisabledByDefault || c.Interval != "" || (c.Mode != "view" && c.Mode != "no-view") {
			continue
		}
		entry := filepath.Join(directory, c.Name+".js")
		info, err := os.Lstat(entry)
		if err != nil || !info.Mode().IsRegular() {
			return Descriptor{}, fmt.Errorf("Raycast command %s needs its built %s.js file", c.Name, c.Name)
		}
		if c.Title == "" {
			c.Title = c.Name
		}
		if c.Icon == "" {
			c.Icon = m.Icon
		}
		for i := range c.Arguments {
			// Installed manifests use required; source manifests use optional (default false).
			if c.Arguments[i].Required == nil {
				required := !c.Arguments[i].Optional
				c.Arguments[i].Required = &required
			}
			if _, err := preferenceDefinition(c.Arguments[i], c.Arguments[i].Name); err != nil {
				return Descriptor{}, fmt.Errorf("command %s: %w", c.Name, err)
			}
		}
		d.Commands = append(d.Commands, c)
	}
	if len(d.Commands) == 0 {
		return Descriptor{}, fmt.Errorf("no supported Raycast view or no-view commands")
	}
	version := m.Version
	if version == "" {
		version = "1.0.0"
	}
	d.Metadata = plugin.Metadata{
		Id: "raycast." + strings.ToLower(owner) + "." + strings.ToLower(m.Name), Name: common.I18nString(m.Title),
		Author: m.Author, Version: version, MinWoxVersion: "2.0.0", Runtime: string(Runtime),
		Description: common.I18nString(m.Description), Icon: assetIcon(directory, m.Icon).String(),
		Directory: directory, Entry: "package.json", TriggerKeywords: []string{m.Name},
		Features: []plugin.MetadataFeature{{Name: plugin.MetadataFeatureIgnoreAutoScore}},
	}
	for _, platform := range m.Platforms {
		switch platform {
		case "macOS":
			d.Metadata.SupportedOS = append(d.Metadata.SupportedOS, "Darwin")
		case "Windows":
			d.Metadata.SupportedOS = append(d.Metadata.SupportedOS, "Windows")
		case "Linux":
			d.Metadata.SupportedOS = append(d.Metadata.SupportedOS, "Linux")
		default:
			return Descriptor{}, fmt.Errorf("unknown Raycast platform %q", platform)
		}
	}
	for _, pref := range m.Preferences {
		item, err := preferenceDefinition(pref, "preference:"+pref.Name)
		if err != nil {
			return Descriptor{}, err
		}
		d.Metadata.SettingDefinitions = append(d.Metadata.SettingDefinitions, item)
	}
	for _, command := range d.Commands {
		d.Metadata.Commands = append(d.Metadata.Commands, plugin.MetadataCommand{Command: command.Name, Description: common.I18nString(command.Title)})
		for _, pref := range command.Preferences {
			item, err := preferenceDefinition(pref, "command:"+command.Name+":"+pref.Name)
			if err != nil {
				return Descriptor{}, err
			}
			d.Metadata.SettingDefinitions = append(d.Metadata.SettingDefinitions, item)
		}
	}
	return d, nil
}

// readManifest bounds preview and discovery reads before allocating the manifest contents.
func readManifest(directory string) ([]byte, error) {
	file, err := os.Open(filepath.Join(directory, "package.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if len(raw) > 2<<20 {
		return nil, fmt.Errorf("Raycast manifest is too large")
	}
	return raw, err
}

// preferenceDefinition translates the fields Wox already knows how to edit.
func preferenceDefinition(p Preference, key string) (definition.PluginSettingDefinitionItem, error) {
	if !safeName.MatchString(p.Name) {
		return definition.PluginSettingDefinitionItem{}, fmt.Errorf("invalid preference name %q", p.Name)
	}
	label := p.Title
	if label == "" {
		label = p.Placeholder
	}
	if label == "" {
		label = p.Name
	}
	value := ""
	if p.Default != nil {
		value = fmt.Sprint(p.Default)
	}
	var checks []validator.PluginSettingValidator
	if p.Required != nil && *p.Required {
		checks = append(checks, validator.PluginSettingValidator{Type: validator.PluginSettingValidatorTypeNotEmpty, Value: &validator.PluginSettingValidatorNotEmpty{}})
	}
	switch p.Type {
	case "checkbox":
		if value == "" {
			value = "false"
		}
		return definition.PluginSettingDefinitionItem{Type: definition.PluginSettingDefinitionTypeCheckBox, Value: &definition.PluginSettingValueCheckBox{Key: key, Label: label, Tooltip: p.Description, DefaultValue: value}}, nil
	case "dropdown":
		options := []definition.PluginSettingValueSelectOption{}
		for _, option := range p.Data {
			options = append(options, definition.PluginSettingValueSelectOption{Label: option.Title, Value: option.Value})
		}
		return definition.PluginSettingDefinitionItem{Type: definition.PluginSettingDefinitionTypeSelect, Value: &definition.PluginSettingValueSelect{Key: key, Label: label, Tooltip: p.Description, DefaultValue: value, Options: options, Validators: checks}}, nil
	case "textfield", "text", "password":
		kind := definition.PluginSettingDefinitionTypeTextBox
		if p.Type == "password" {
			kind = definition.PluginSettingDefinitionTypePassword
		}
		return definition.PluginSettingDefinitionItem{Type: kind, Value: &definition.PluginSettingValueTextBox{Key: key, Label: label, Tooltip: p.Description, DefaultValue: value, Validators: checks}}, nil
	default:
		return definition.PluginSettingDefinitionItem{}, fmt.Errorf("unsupported Raycast preference type %q", p.Type)
	}
}

// assetIcon confines relative manifest assets to the copied extension directory.
func assetIcon(directory, name string) common.WoxImage {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "..") {
		return brand.Image()
	}
	for _, candidate := range []string{filepath.Join(directory, "assets", name), filepath.Join(directory, name)} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return common.NewWoxImageAbsolutePath(candidate)
		}
	}
	return brand.Image()
}
