package plugin

import (
	"runtime"
	"strings"
	"testing"
)

func TestSelectCurrentOSManifestsKeepsOnlyThisMachine(t *testing.T) {
	manifests := []StorePluginManifest{
		{Id: "win", SupportedOS: []string{"Windows"}},
		{Id: "mac", SupportedOS: []string{"Darwin"}},
		{Id: "linux", SupportedOS: []string{"Linux"}},
		{Id: "all", SupportedOS: []string{"Windows", "Darwin", "Linux"}},
	}
	var ids []string
	for _, manifest := range selectCurrentOSManifests(manifests) {
		ids = append(ids, manifest.Id)
	}
	want := []string{"mac", "all"}
	if runtime.GOOS == "windows" {
		want = []string{"win", "all"}
	} else if runtime.GOOS == "linux" {
		want = []string{"linux", "all"}
	}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %#v, want %#v", ids, want)
	}
}

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
