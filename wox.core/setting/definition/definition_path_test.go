package definition

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnmarshalPathType(t *testing.T) {
	type metadataForTest struct {
		SettingDefinitions PluginSettingDefinitions
	}

	jsonStr := `
{
    "SettingDefinitions":[
        {
            "Type":"path",
            "Value":{
                "Key":"ConfigDir",
                "DefaultValue":"/home/user/.config",
                "Label":"Config Directory: ",
                "Suffix":" (select a folder)",
                "Tooltip":"Choose the configuration directory"
            }
        }
    ]
}
`

	var metadata metadataForTest
	err := json.Unmarshal([]byte(jsonStr), &metadata)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(metadata.SettingDefinitions))
	assert.Equal(t, PluginSettingDefinitionTypePath, metadata.SettingDefinitions[0].Type)

	pathVal, ok := metadata.SettingDefinitions[0].Value.(*PluginSettingValuePath)
	assert.True(t, ok)
	assert.Equal(t, "ConfigDir", pathVal.GetKey())
	assert.Equal(t, "/home/user/.config", pathVal.GetDefaultValue())
	assert.Equal(t, "Config Directory: ", pathVal.Label)
	assert.Equal(t, " (select a folder)", pathVal.Suffix)
	assert.Equal(t, "Choose the configuration directory", pathVal.Tooltip)
}

func TestPathSettingTranslate(t *testing.T) {
	pathVal := &PluginSettingValuePath{
		Key:          "TestPath",
		Label:        "label.key",
		Suffix:       "suffix.key",
		DefaultValue: "/tmp",
		Tooltip:      "tooltip.key",
	}

	translator := func(ctx context.Context, key string) string {
		translations := map[string]string{
			"label.key":   "Translated Label",
			"suffix.key":  "Translated Suffix",
			"tooltip.key": "Translated Tooltip",
		}
		return translations[key]
	}

	translated := pathVal.Translate(translator).(*PluginSettingValuePath)
	assert.Equal(t, "Translated Label", translated.Label)
	assert.Equal(t, "Translated Suffix", translated.Suffix)
	assert.Equal(t, "Translated Tooltip", translated.Tooltip)
	assert.Equal(t, "label.key", pathVal.Label)
}

func TestPathSettingGetKeyAndGetDefaultValue(t *testing.T) {
	pathVal := &PluginSettingValuePath{
		Key:          "MyDir",
		DefaultValue: "/some/path",
	}
	assert.Equal(t, "/some/path", pathVal.GetDefaultValue())
	assert.Equal(t, "MyDir", pathVal.GetKey())
}

func TestPathSettingInDefinitionsGetDefaultValue(t *testing.T) {
	type metadataForTest struct {
		SettingDefinitions PluginSettingDefinitions
	}

	jsonStr := `
{
    "SettingDefinitions":[
        {
            "Type":"path",
            "Value":{
                "Key":"OutputDir",
                "DefaultValue":"/output",
                "Label":"Output Directory"
            }
        }
    ]
}
`

	var metadata metadataForTest
	err := json.Unmarshal([]byte(jsonStr), &metadata)
	assert.Nil(t, err)

	val, exist := metadata.SettingDefinitions.GetDefaultValue("OutputDir")
	assert.True(t, exist)
	assert.Equal(t, "/output", val)
}

func TestPathSettingMarshal(t *testing.T) {
	type metadataForTest struct {
		SettingDefinitions PluginSettingDefinitions
	}

	original := metadataForTest{
		SettingDefinitions: PluginSettingDefinitions{
			{
				Type: PluginSettingDefinitionTypePath,
				Value: &PluginSettingValuePath{
					Key:          "TestDir",
					Label:        "Test Directory",
					Suffix:       "suffix",
					DefaultValue: "/default",
					Tooltip:      "tooltip",
				},
			},
		},
	}

	data, err := json.Marshal(original)
	assert.Nil(t, err)

	var roundtripped metadataForTest
	err = json.Unmarshal(data, &roundtripped)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(roundtripped.SettingDefinitions))
	assert.Equal(t, PluginSettingDefinitionTypePath, roundtripped.SettingDefinitions[0].Type)

	pathVal := roundtripped.SettingDefinitions[0].Value.(*PluginSettingValuePath)
	assert.Equal(t, "TestDir", pathVal.Key)
	assert.Equal(t, "/default", pathVal.DefaultValue)
}
