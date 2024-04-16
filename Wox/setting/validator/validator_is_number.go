package validator

type PluginSettingValidatorIsNumber struct {
	Type string // Type of the validator, will auto set, no need to set

	IsInteger bool
	IsFloat   bool
}

func (p *PluginSettingValidatorIsNumber) SetValidatorType() {
	p.Type = "is_number"
}
