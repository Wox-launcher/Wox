package validator

type PluginSettingValidatorNotEmpty struct {
}

func (p *PluginSettingValidatorNotEmpty) GetValidatorType() PluginSettingValidatorType {
	return PluginSettingValidatorTypeNotEmpty
}
