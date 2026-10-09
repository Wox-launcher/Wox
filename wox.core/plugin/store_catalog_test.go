package plugin

import "testing"

func TestAssignPluginStoreLeavesEarlierCatalogs(t *testing.T) {
	manifests := []StorePluginManifest{
		{Id: "wox-one", Store: OfficialPluginStoreID},
		{Id: "flow-one"},
	}
	assignPluginStore(manifests, "flow", pluginStoreManifestIDs([]StorePluginManifest{{Id: "wox-one"}}))
	if manifests[0].Store != OfficialPluginStoreID || manifests[1].Store != "flow" {
		t.Fatalf("stores = %#v", manifests)
	}
}

func TestPluginStorePresentation(t *testing.T) {
	label, icon := PluginStorePresentation("")
	if label != "i18n:ui_plugin_store_wox" || icon.IsEmpty() {
		t.Fatalf("official presentation = %q empty=%v", label, icon.IsEmpty())
	}
	label, icon = PluginStorePresentation("acme")
	if label != "Acme Store" || icon.IsEmpty() {
		t.Fatalf("unknown presentation = %q empty=%v", label, icon.IsEmpty())
	}
}
