package validator

type PluginSettingValidatorNotEmpty struct {
	Type string // Type of the validator, will auto set, no need to set
}

func (p *PluginSettingValidatorNotEmpty) SetValidatorType() {
	p.Type = "not_empty"
}
