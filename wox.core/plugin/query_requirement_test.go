package plugin

import (
	"testing"
	"wox/setting/validator"

	"github.com/stretchr/testify/assert"
)

func TestMetadataQueryRequirementsScopes(t *testing.T) {
	requirements := MetadataQueryRequirements{
		AnyQuery: []MetadataQueryRequirement{
			{SettingKey: "accessKey"},
		},
		QueryWithoutCommand: []MetadataQueryRequirement{
			{SettingKey: "defaultMode"},
		},
		QueryWithCommand: map[string][]MetadataQueryRequirement{
			"download": {
				{SettingKey: "downloadPath"},
			},
		},
	}

	withoutCommand := requirements.GetRequirementsForQuery(Query{Command: ""})
	assert.Equal(t, []string{"accessKey", "defaultMode"}, []string{withoutCommand[0].SettingKey, withoutCommand[1].SettingKey})

	withKnownCommand := requirements.GetRequirementsForQuery(Query{Command: "download"})
	assert.Equal(t, []string{"accessKey", "downloadPath"}, []string{withKnownCommand[0].SettingKey, withKnownCommand[1].SettingKey})

	withUnknownCommand := requirements.GetRequirementsForQuery(Query{Command: "search"})
	assert.Equal(t, []string{"accessKey"}, []string{withUnknownCommand[0].SettingKey})
}

func TestValidateQueryRequirementValue(t *testing.T) {
	notEmpty := validator.PluginSettingValidator{
		Type:  validator.PluginSettingValidatorTypeNotEmpty,
		Value: &validator.PluginSettingValidatorNotEmpty{},
	}
	integer := validator.PluginSettingValidator{
		Type: validator.PluginSettingValidatorTypeIsNumber,
		Value: &validator.PluginSettingValidatorIsNumber{
			IsInteger: true,
		},
	}

	assert.Equal(t, "i18n:ui_validator_value_can_not_be_empty", validateQueryRequirementValue(" ", []validator.PluginSettingValidator{notEmpty}))
	assert.Equal(t, "i18n:ui_validator_must_be_integer", validateQueryRequirementValue("1.2", []validator.PluginSettingValidator{integer}))
	assert.Empty(t, validateQueryRequirementValue("12", []validator.PluginSettingValidator{notEmpty, integer}))
}
