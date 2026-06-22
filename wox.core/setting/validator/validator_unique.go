package validator

type PluginSettingValidatorUnique struct {
}

func (p *PluginSettingValidatorUnique) GetValidatorType() PluginSettingValidatorType {
	return PluginSettingValidatorTypeUnique
}
