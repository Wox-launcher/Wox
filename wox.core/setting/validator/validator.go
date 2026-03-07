package validator

import (
	"encoding/json"
	"errors"
	"fmt"
)

type PluginSettingValidatorType string

const (
	PluginSettingValidatorTypeIsNumber PluginSettingValidatorType = "is_number"
	PluginSettingValidatorTypeNotEmpty PluginSettingValidatorType = "not_empty"
)

type PluginSettingValidator struct {
	Type  PluginSettingValidatorType
	Value PluginSettingValidatorValue
}

type PluginSettingValidatorValue interface {
	GetValidatorType() PluginSettingValidatorType
}

func (p *PluginSettingValidator) UnmarshalJSON(b []byte) error {
	var raw struct {
		Type  PluginSettingValidatorType
		Value json.RawMessage
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if raw.Type == "" {
		return errors.New("validator must have Type property")
	}

	p.Type = raw.Type

	switch raw.Type {
	case PluginSettingValidatorTypeIsNumber:
		if len(raw.Value) == 0 || string(raw.Value) == "null" {
			return errors.New("is_number validator must have Value property")
		}

		var value PluginSettingValidatorIsNumber
		if err := json.Unmarshal(raw.Value, &value); err != nil {
			return fmt.Errorf("failed to parse is_number validator value: %w", err)
		}
		p.Value = &value
	case PluginSettingValidatorTypeNotEmpty:
		value := &PluginSettingValidatorNotEmpty{}
		if len(raw.Value) != 0 && string(raw.Value) != "null" {
			if err := json.Unmarshal(raw.Value, value); err != nil {
				return fmt.Errorf("failed to parse not_empty validator value: %w", err)
			}
		}
		p.Value = value
	default:
		return fmt.Errorf("unknown validator type: %s", raw.Type)
	}

	return nil
}
