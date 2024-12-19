package validator

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
