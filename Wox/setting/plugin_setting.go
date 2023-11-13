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

	CustomizedSettings *util.HashMap[string, string]
}

func (p *PluginSetting) GetCustomizedSetting(key string) (string, bool) {
	if p.CustomizedSettings == nil {
		return "", false
	}
	return p.CustomizedSettings.Load(key)
}

type CustomizedPluginSettings []PluginSettingItem

func (c CustomizedPluginSettings) GetValue(key string) (string, bool) {
	for _, item := range c {
		if item.Value.GetKey() == key {
			return item.Value.GetValue(), true
		}
	}

	return "", false
}

func (c CustomizedPluginSettings) GetAll() (settings *util.HashMap[string, string]) {
	settings = util.NewHashMap[string, string]()
	for _, item := range c {
		settings.Store(item.Value.GetKey(), item.Value.GetValue())
	}
	return
}

type PluginSettingType string

const (
	PluginSettingTypeHead     PluginSettingType = "head"
	PluginSettingTypeTextBox  PluginSettingType = "textbox"
	PluginSettingTypeCheckBox PluginSettingType = "checkbox"
	PluginSettingTypeSelect   PluginSettingType = "select"
	PluginSettingTypeLabel    PluginSettingType = "label"
	PluginSettingTypeNewLine  PluginSettingType = "newline"
)

type PluginSettingItem struct {
	Type  PluginSettingType
	Value PluginSettingValue
}

func (n *PluginSettingItem) UnmarshalJSON(b []byte) error {
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
		n.Type = PluginSettingTypeHead
		var v PluginSettingValueHead
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "textbox":
		n.Type = PluginSettingTypeTextBox
		var v PluginSettingValueTextBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "checkbox":
		n.Type = PluginSettingTypeCheckBox
		var v PluginSettingValueCheckBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "select":
		n.Type = PluginSettingTypeSelect
		var v PluginSettingValueSelect
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "label":
		n.Type = PluginSettingTypeLabel
		var v PluginSettingValueLabel
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = v
	case "newline":
		n.Type = PluginSettingTypeNewLine
		n.Value = PluginSettingValueNewLine{}
	default:
		return errors.New("unknown setting type: " + value.String())
	}

	return nil
}

type PluginSettingValue interface {
	GetPluginSettingType() PluginSettingType
	GetKey() string
	GetValue() string
}

type PluginSettingValueHead struct {
	Content string
}

func (p PluginSettingValueHead) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeHead
}

func (p PluginSettingValueHead) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueHead) GetValue() string {
	return ""
}

type PluginSettingValueTextBox struct {
	Key    string
	Label  string
	Suffix string
	Value  string
}

func (p PluginSettingValueTextBox) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeTextBox
}

func (p PluginSettingValueTextBox) GetKey() string {
	return p.Key
}

func (p PluginSettingValueTextBox) GetValue() string {
	return p.Value
}

type PluginSettingValueCheckBox struct {
	Key   string
	Label string
	Value string
}

func (p PluginSettingValueCheckBox) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeCheckBox
}

func (p PluginSettingValueCheckBox) GetKey() string {
	return p.Key
}

func (p PluginSettingValueCheckBox) GetValue() string {
	return p.Value
}

type PluginSettingValueSelect struct {
	Key     string
	Label   string
	Suffix  string
	Value   string
	Options []PluginSettingValueSelectOption
}

type PluginSettingValueSelectOption struct {
	Label string
	Value string
}

func (p PluginSettingValueSelect) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeSelect
}

func (p PluginSettingValueSelect) GetKey() string {
	return p.Key
}

func (p PluginSettingValueSelect) GetValue() string {
	return p.Value
}

type PluginSettingValueLabel struct {
	Content string
}

func (p PluginSettingValueLabel) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeLabel
}

func (p PluginSettingValueLabel) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueLabel) GetValue() string {
	return ""
}

type PluginSettingValueNewLine struct {
}

func (p PluginSettingValueNewLine) GetPluginSettingType() PluginSettingType {
	return PluginSettingTypeNewLine
}

func (p PluginSettingValueNewLine) GetKey() string {
	return uuid.NewString()
}

func (p PluginSettingValueNewLine) GetValue() string {
	return ""
}
