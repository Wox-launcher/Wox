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
		{Id: "mac-alias", SupportedOS: []string{"Macos"}},
		{Id: "linux", SupportedOS: []string{"Linux"}},
		{Id: "all", SupportedOS: []string{"Windows", "Darwin", "Linux"}},
	}
	var ids []string
	for _, manifest := range selectCurrentOSManifests(manifests) {
		ids = append(ids, manifest.Id)
	}
	want := []string{"mac", "mac-alias", "all"}
	if runtime.GOOS == "windows" {
		want = []string{"win", "all"}
	} else if runtime.GOOS == "linux" {
		want = []string{"linux", "all"}
	}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %#v, want %#v", ids, want)
	}
}

func TestPluginOSValidationAcceptsMacOSAlias(t *testing.T) {
	if !IsAllSupportedOS([]string{"Windows", "Linux", "Macos"}) {
		t.Fatal("Macos should be valid alongside the other platforms")
	}
	if IsAllSupportedOS([]string{"macOS", "unknown"}) {
		t.Fatal("an unknown platform should still invalidate the list")
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

func TestPluginStoreUpdatedDateChangesSignature(t *testing.T) {
	store := &Store{}
	before := []StorePluginManifest{{Id: "a", Version: "1", DateUpdated: "2025-01-01 00:00:00"}}
	after := []StorePluginManifest{{Id: "a", Version: "1", DateUpdated: "2026-01-01 00:00:00"}}
	if store.buildManifestSignature(before) == store.buildManifestSignature(after) {
		t.Fatal("updated date changes must refresh the store catalog")
	}
}
