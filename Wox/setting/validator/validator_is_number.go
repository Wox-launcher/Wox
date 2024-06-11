package validator

type PluginSettingValidatorIsNumber struct {
	IsInteger bool
	IsFloat   bool
}

func (p *PluginSettingValidatorIsNumber) GetValidatorType() PluginSettingValidatorType {
	return PluginSettingValidatorTypeIsNumber
}
