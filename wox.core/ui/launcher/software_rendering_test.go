package launcher

import (
	"testing"
	"wox/ui/contract"
)

// TestSoftwareRenderingSettingVisibility verifies the persisted value reaches the control at the end of Interface settings.
func TestSoftwareRenderingSettingVisibility(t *testing.T) {
	data, err := settingsDataFromContract(contract.GeneralSettings{UseSoftwareRendering: true, SoftwareRenderingRestartRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if !data.SoftwareRenderingRestartRequired {
		t.Fatal("restart requirement was lost in the settings snapshot")
	}
	found := false
	for _, item := range settingItems("appearance", data) {
		if item.key == "UseSoftwareRendering" {
			found = true
			if item.value != "true" || len(item.choices) != 2 {
				t.Fatalf("unexpected control: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("software rendering control is missing")
	}
	items := settingItems("appearance", data)
	if items[len(items)-1].key != "UseSoftwareRendering" {
		t.Fatal("rendering setting must be last")
	}
	for _, item := range settingItems("general", data) {
		if item.key == "UseSoftwareRendering" {
			t.Fatal("rendering setting must not appear on General")
		}
	}
}
