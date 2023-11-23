package setting

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"wox/util"
)

type PluginQueryCommand struct {
	Command     string
	Description string
}

type PluginSetting struct {
	// Is this plugin disabled by user
	Disabled bool

	// User defined keywords, will be used to trigger this plugin. User may not set custom trigger keywords, which will cause this property to be null
	// So don't use this property directly, use Instance.TriggerKeywords instead
	TriggerKeywords []string

	// plugin author can register query command dynamically
	// the final query command will be the combination of plugin's metadata commands defined in plugin.json and customized query command registered here
	CustomizedQueryCommands []PluginQueryCommand

	Settings *util.HashMap[string, string]
}

func (p *PluginSetting) GetSetting(key string) (string, bool) {
	if p.Settings == nil {
		return "", false
	}
	return p.Settings.Load(key)
}

type PluginSettingDefinitions []PluginSettingDefinitionItem

func (c PluginSettingDefinitions) GetDefaultValue(key string) (string, bool) {
	for _, item := range c {
		if item.Value.GetKey() == key {
			return item.Value.GetDefaultValue(), true
		}
	}

	return "", false
}

func (c PluginSettingDefinitions) GetAllDefaults() (settings *util.HashMap[string, string]) {
	settings = util.NewHashMap[string, string]()
	for _, item := range c {
		settings.Store(item.Value.GetKey(), item.Value.GetDefaultValue())
	}
	return
}

type PluginSettingDefinitionType string

const (
	PluginSettingDefinitionTypeHead     PluginSettingDefinitionType = "head"
	PluginSettingDefinitionTypeTextBox  PluginSettingDefinitionType = "textbox"
	PluginSettingDefinitionTypeCheckBox PluginSettingDefinitionType = "checkbox"
	PluginSettingDefinitionTypeSelect   PluginSettingDefinitionType = "select"
	PluginSettingDefinitionTypeLabel    PluginSettingDefinitionType = "label"
	PluginSettingDefinitionTypeNewLine  PluginSettingDefinitionType = "newline"
)

type PluginSettingDefinitionItem struct {
	Type  PluginSettingDefinitionType
	Value PluginSettingDefinitionValue
}

func (n *PluginSettingDefinitionItem) UnmarshalJSON(b []byte) error {
	value := gjson.GetBytes(b, "Type")
	if !value.Exists() {
		return errors.New("setting must have Type property")
	}
	contentResult := gjson.GetBytes(b, "Value")
	if !contentResult.Exists() {
		return errors.New("setting type must have Value property")
	}

	switch value.String() {
	case "head":
		n.Type = PluginSettingDefinitionTypeHead
		var v PluginSettingValueHead
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "textbox":
		n.Type = PluginSettingDefinitionTypeTextBox
		var v PluginSettingValueTextBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "checkbox":
		n.Type = PluginSettingDefinitionTypeCheckBox
		var v PluginSettingValueCheckBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "select":
		n.Type = PluginSettingDefinitionTypeSelect
		var v PluginSettingValueSelect
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "label":
		n.Type = PluginSettingDefinitionTypeLabel
		var v PluginSettingValueLabel
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "newline":
		n.Type = PluginSettingDefinitionTypeNewLine
		n.Value = PluginSettingValueNewLine{}
	default:
		return errors.New("unknown setting type: " + value.String())
	}

	return nil
}

type PluginSettingDefinitionValue interface {
	GetKey() string
	GetDefaultValue() string
}

type PluginSettingValueHead struct {
	Content string
}

func (p PluginSettingValueHead) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeHead
}

func (p PluginSettingValueHead) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueHead) GetDefaultValue() string {
	return ""
}

type PluginSettingValueTextBox struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
}

func (p PluginSettingValueTextBox) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeTextBox
}

func (p PluginSettingValueTextBox) GetKey() string {
	return p.Key
}

func (p PluginSettingValueTextBox) GetDefaultValue() string {
	return p.DefaultValue
}

type PluginSettingValueCheckBox struct {
	Key          string
	Label        string
	DefaultValue string
}

func (p PluginSettingValueCheckBox) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeCheckBox
}

func (p PluginSettingValueCheckBox) GetKey() string {
	return p.Key
}

func (p PluginSettingValueCheckBox) GetDefaultValue() string {
	return p.DefaultValue
}

type PluginSettingValueSelect struct {
	Key          string
	Label        string
	Suffix       string
	DefaultValue string
	Options      []PluginSettingValueSelectOption
}

type PluginSettingValueSelectOption struct {
	Label string
	Value string
}

func (p PluginSettingValueSelect) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeSelect
}

func (p PluginSettingValueSelect) GetKey() string {
	return p.Key
}

func (p PluginSettingValueSelect) GetDefaultValue() string {
	return p.DefaultValue
}

type PluginSettingValueLabel struct {
	Content string
}

func (p PluginSettingValueLabel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeLabel
}

func (p PluginSettingValueLabel) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueLabel) GetDefaultValue() string {
	return ""
}

type PluginSettingValueNewLine struct {
}

func (p PluginSettingValueNewLine) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeNewLine
}

func (p PluginSettingValueNewLine) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueNewLine) GetDefaultValue() string {
	return ""
}
