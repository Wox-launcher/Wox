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

func TestPluginStoreStatisticsChangeSignature(t *testing.T) {
	store := &Store{}
	zero, one := 0, 1
	base := []StorePluginManifest{{Id: "a", Version: "1"}}
	withZero := []StorePluginManifest{{Id: "a", Version: "1", StarCount: &zero}}
	withOne := []StorePluginManifest{{Id: "a", Version: "1", StarCount: &one}}
	withDownloads := []StorePluginManifest{{Id: "a", Version: "1", DownloadCount: &zero}}
	if store.buildManifestSignature(base) == store.buildManifestSignature(withZero) || store.buildManifestSignature(withZero) == store.buildManifestSignature(withOne) || store.buildManifestSignature(base) == store.buildManifestSignature(withDownloads) {
		t.Fatal("statistics changes must refresh the store catalog")
	}
}
