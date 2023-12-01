package setting

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"wox/setting/definition"
	"wox/util"
)

func TestUnMarshalPluginSettingItem(t *testing.T) {
	type metadataForTest struct {
		SettingDefinitions definition.PluginSettingDefinitions
	}

	jsonStr := `
{
    "SettingDefinitions":[
        {
            "Type":"head",
            "Value":{
                "Content":"This is head title"
            }
        },
        {
            "Type":"textbox",
            "Value":{
                "Key":"IndexDirectories",
                "Value":"test;test1",
                "Label":"Index Directories: ",
                "Suffix":" (separate by ';')"
            }
        },
        {
            "Type":"checkbox",
            "Value":{
                "Key":"OnlyIndexTxt",
				"Value": "true",
                "Label":", Only Index Txt"
            }
        },
		{
					"Type":"select",
					"Value":{	
						"Key":"IndexPrograms",	
						"Value":"true",		
						"Label":"Index Programs: ",	
						"Options":[
							{"Label":"true", "Value":"true"},
							{"Label":"false", "Value":"false"}	
						]		
					}	
		},
        {
            "Type":"newline",
			"Value":{}
        },
        {
            "Type":"label",
            "Value":{
                "Content":"IndexPrograms"
            }
        }
    ]
}
`

	var metadata metadataForTest
	err := json.Unmarshal([]byte(jsonStr), &metadata)
	if err != nil {
		t.Log(err.Error())
	}

	assert.Nil(t, err)
	assert.Equal(t, len(metadata.SettingDefinitions), 6)
	assert.Equal(t, metadata.SettingDefinitions[0].Type, definition.PluginSettingDefinitionTypeHead)
	assert.Equal(t, metadata.SettingDefinitions[1].Type, definition.PluginSettingDefinitionTypeTextBox)
	assert.Equal(t, metadata.SettingDefinitions[2].Type, definition.PluginSettingDefinitionTypeCheckBox)
	assert.Equal(t, metadata.SettingDefinitions[3].Type, definition.PluginSettingDefinitionTypeSelect)
	assert.Equal(t, metadata.SettingDefinitions[4].Type, definition.PluginSettingDefinitionTypeNewLine)
	assert.Equal(t, metadata.SettingDefinitions[5].Type, definition.PluginSettingDefinitionTypeLabel)
	assert.Equal(t, len(metadata.SettingDefinitions[3].Value.(*definition.PluginSettingValueSelect).Options), 2)

	val, exist := metadata.SettingDefinitions.GetDefaultValue("IndexDirectories")
	assert.True(t, exist)
	assert.Equal(t, val, "test;test1")

	marshalData, marshalErr := json.Marshal(metadata)
	assert.Nil(t, marshalErr)
	t.Log(string(marshalData))
}

func TestMarshalPluginSetting(t *testing.T) {
	var h util.HashMap[string, string]
	h.Store("test", "test")
	h.Store("test1", "test")

	ps := PluginSetting{
		Disabled:        true,
		TriggerKeywords: nil,
		Settings:        &h,
	}

	marshalData, marshalErr := json.Marshal(ps)
	assert.Nil(t, marshalErr)
	t.Log(string(marshalData))

	var ps1 PluginSetting
	err := json.Unmarshal(marshalData, &ps1)
	assert.Nil(t, err)
	assert.Equal(t, ps.Disabled, ps1.Disabled)
	assert.Equal(t, ps1.Settings.Len(), int64(2))
}
