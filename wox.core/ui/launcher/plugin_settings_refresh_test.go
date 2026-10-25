package launcher

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"wox/setting/definition"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type pluginDefinitionRefreshServices struct {
	contract.Services
	plugins    []contract.PluginCatalogItem
	duringLoad func()
}

// Plugins completes another save after the refresh snapshot has been taken.
func (s pluginDefinitionRefreshServices) Plugins(context.Context, string, contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	if s.duringLoad != nil {
		s.duringLoad()
	}
	return s.plugins, nil
}

// TestPluginDefinitionRefreshPreservesNewerSave forces an older refresh to arrive after a newer save.
func TestPluginDefinitionRefreshPreservesNewerSave(t *testing.T) {
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.uiCall = nil
	a.pluginSettings.SetPlugins([]pluginSettingsPlugin{{
		ID: "calculator", Name: "Calculator",
		SettingDefinitions: []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "precision"}}},
		Setting: pluginSettingsData{
			TriggerKeywords: []string{"*"}, Settings: map[string]string{"precision": "10"},
		},
	}})
	a.pluginSettings.setPluginSelectionLocked(0)
	a.services = pluginDefinitionRefreshServices{
		plugins: []contract.PluginCatalogItem{{
			ID: "calculator",
			Setting: contract.PluginSetting{
				TriggerKeywords: []string{"*"}, Settings: map[string]string{"precision": "10"},
			},
		}},
		duringLoad: func() {
			a.pluginSettings.reconcilePluginSettingSave(&pluginSettingsFormState{pluginID: "calculator"}, "calculator", map[string]string{
				"TriggerKeywords": encodePluginTriggerKeywordRows([]string{"*", "calculator"}), "precision": "20",
			}, map[string]string{"TriggerKeywords": "*,calculator", "precision": "20"}, nil)
		},
	}

	a.bindPluginSettings()
	a.pluginSettings.refreshPluginFormDefinitions("calculator", a.pluginSettings.settingSaveRevisions["calculator"])

	keywords, err := decodePluginTriggerKeywordRows(a.pluginSettings.Form().values["TriggerKeywords"])
	if err != nil || !reflect.DeepEqual(keywords, []string{"*", "calculator"}) {
		t.Fatalf("keywords after older refresh = %q (err %v), want [* calculator]", keywords, err)
	}
	if got := a.pluginSettings.Form().values["precision"]; got != "20" {
		t.Fatalf("precision after older refresh = %q, want 20", got)
	}
	plugins, _ := a.pluginSettings.CachedPlugins(false)
	if len(plugins) != 1 || !reflect.DeepEqual(plugins[0].Setting.TriggerKeywords, []string{"*", "calculator"}) || plugins[0].Setting.Settings["precision"] != "20" {
		t.Fatalf("cached settings after older refresh = %+v", plugins)
	}
}

// TestPluginDefinitionRefreshAppliesCurrentSave keeps dynamic definitions and normalized values refreshing.
func TestPluginDefinitionRefreshAppliesCurrentSave(t *testing.T) {
	for _, otherPluginSave := range []bool{false, true} {
		t.Run(fmt.Sprintf("otherPluginSave=%t", otherPluginSave), func(t *testing.T) {
			a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
			defer a.cancel()
			a.uiCall = nil
			a.pluginSettings.SetPlugins([]pluginSettingsPlugin{{ID: "calculator", TriggerKeywords: []string{"*"}}})
			a.pluginSettings.setPluginSelectionLocked(0)
			a.services = pluginDefinitionRefreshServices{
				plugins: []contract.PluginCatalogItem{{
					ID: "calculator",
					SettingDefinitions: definition.PluginSettingDefinitions{{
						Type:  definition.PluginSettingDefinitionTypeTextBox,
						Value: &definition.PluginSettingValueTextBox{Key: "precision", Label: "Precision"},
					}},
					Setting: contract.PluginSetting{
						TriggerKeywords: []string{"*", "calculator"}, Settings: map[string]string{"precision": "20"},
					},
				}},
				duringLoad: func() {
					if otherPluginSave {
						a.pluginSettings.reconcilePluginSettingSave(&pluginSettingsFormState{pluginID: "other"}, "other", nil, nil, nil)
					}
				},
			}

			a.bindPluginSettings()
			a.pluginSettings.refreshPluginFormDefinitions("calculator", a.pluginSettings.settingSaveRevisions["calculator"])

			form := a.pluginSettings.Form()
			if len(form.definitions) != 2 || form.definitions[1].Value.Label != "Precision" || form.values["precision"] != "20" {
				t.Fatalf("current refresh did not apply dynamic definition and value: %+v", form)
			}
		})
	}
}
