package definition

import (
	"context"
	"encoding/json"
	"errors"
	"wox/util"

	"github.com/tidwall/gjson"
)

type PluginSettingDefinitionType string

const (
	PluginSettingDefinitionTypeHead          PluginSettingDefinitionType = "head"
	PluginSettingDefinitionTypeTextBox       PluginSettingDefinitionType = "textbox"
	PluginSettingDefinitionTypeCheckBox      PluginSettingDefinitionType = "checkbox"
	PluginSettingDefinitionTypeSelect        PluginSettingDefinitionType = "select"
	PluginSettingDefinitionTypeSelectAIModel PluginSettingDefinitionType = "selectAIModel"
	PluginSettingDefinitionTypeLabel         PluginSettingDefinitionType = "label"
	PluginSettingDefinitionTypeNewLine       PluginSettingDefinitionType = "newline"
	PluginSettingDefinitionTypeTable         PluginSettingDefinitionType = "table"

	// dynamic setting will be replaced by the actual setting when retrieved
	// this is useful when the setting is dynamic. E.g. a list of plugins for select
	// if user define the dynamic setting, user should use api.
	PluginSettingDefinitionTypeDynamic PluginSettingDefinitionType = "dynamic"
)

type PluginSettingDefinitionValue interface {
	GetKey() string
	GetDefaultValue() string
	Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue
}

type PluginSettingDefinitionItem struct {
	Type                PluginSettingDefinitionType
	Value               PluginSettingDefinitionValue
	DisabledInPlatforms []util.Platform
	IsPlatformSpecific  bool // if true, this setting may be different in different platforms
}

type PluginSettingValueStyle struct {
	PaddingLeft   int
	PaddingTop    int
	PaddingRight  int
	PaddingBottom int

	Width      int
	LabelWidth int // if has label, E.g. select, checkbox, textbox
}

func (n *PluginSettingDefinitionItem) UnmarshalJSON(b []byte) error {
	value := gjson.GetBytes(b, "Type")
	if !value.Exists() {
		return errors.New("setting must have Type property")
	}

	contentResult := gjson.GetBytes(b, "Value")
	if value.String() != "newline" {
		if !contentResult.Exists() {
			return errors.New("setting type must have Value property")
		}
	}

	switch value.String() {
	case "head":
		n.Type = PluginSettingDefinitionTypeHead
		var v PluginSettingValueHead
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "textbox":
		n.Type = PluginSettingDefinitionTypeTextBox
		var v PluginSettingValueTextBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "checkbox":
		n.Type = PluginSettingDefinitionTypeCheckBox
		var v PluginSettingValueCheckBox
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "select":
		n.Type = PluginSettingDefinitionTypeSelect
		var v PluginSettingValueSelect
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "label":
		n.Type = PluginSettingDefinitionTypeLabel
		var v PluginSettingValueLabel
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "newline":
		n.Type = PluginSettingDefinitionTypeNewLine
		n.Value = &PluginSettingValueNewLine{}
	case "table":
		n.Type = PluginSettingDefinitionTypeTable
		var v PluginSettingValueTable
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	case "selectAIModel":
		n.Type = PluginSettingDefinitionTypeSelectAIModel
		var v PluginSettingValueSelectAIModel
		unmarshalErr := json.Unmarshal([]byte(contentResult.String()), &v)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		n.Value = &v
	default:
		return errors.New("unknown setting type: " + value.String())
	}

	return nil
}

type PluginSettingDefinitions []PluginSettingDefinitionItem

func (c PluginSettingDefinitions) ToMap() map[string]string {
	m := make(map[string]string)
	for _, item := range c {
		if item.Value != nil {
			m[item.Value.GetKey()] = item.Value.GetDefaultValue()
		}
	}
	return m
}

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
		if item.Value != nil {
			settings.Store(item.Value.GetKey(), item.Value.GetDefaultValue())
		}
	}
	return
}
