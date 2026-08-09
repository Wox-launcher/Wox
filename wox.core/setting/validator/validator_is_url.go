package validator

type PluginSettingValidatorIsURL struct{}

func (p *PluginSettingValidatorIsURL) GetValidatorType() PluginSettingValidatorType {
	return PluginSettingValidatorTypeIsURL
}
