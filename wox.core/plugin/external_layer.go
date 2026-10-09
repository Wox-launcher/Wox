package plugin

import (
	"context"
	"strings"

	"wox/common"
	"wox/common/icons"
)

// Layer is one compatibility implementation in plugin/thirdparty/<name>.
// The package registers itself from init. Wox discovers it through the blank
// imports in plugin/thirdparty/imports.go.
type Layer interface {
	Name() string
	// Directory is the reserved child of the user plugins directory.
	// An empty name means this layer does not own a collection directory.
	Directory() string
	Hosts() []Host
	// Store is nil when the layer has no plugin catalog.
	Store() ExternalStore
}

// ExternalStore is a plugin catalog whose install path is not the official store.
// Owns reports which manifests and installed plugins belong to that catalog.
// Label and Icon are the settings section for that catalog.
type ExternalStore interface {
	Name() string
	Label() string
	Icon() common.WoxImage
	AppendManifests(ctx context.Context, manifests []StorePluginManifest) []StorePluginManifest
	Owns(manifest StorePluginManifest) bool
	Install(ctx context.Context, manifest StorePluginManifest, progress InstallProgressCallback) error
	OwnsInstance(instance *Instance) bool
	Uninstall(ctx context.Context, instance *Instance, skipCleanSetting bool, preserveCache bool, progress UninstallProgressCallback) (bool, error)
}

var (
	externalStores []ExternalStore

	// ResolvePythonPath and ResolveNodePath are filled by the built-in interpreter
	// hosts. Compatibility layers read them when a host starts, after init has finished.
	ResolvePythonPath func(ctx context.Context) (string, error)
	ResolveNodePath   func(ctx context.Context) (string, error)

	reservedUserPluginDirectories = map[string]struct{}{}
)

// RegisterThirdParty adds a compatibility layer's hosts, store, and directory.
// init calls this once. Owns implementations must not claim the same plugin.
func RegisterThirdParty(layer Layer) {
	if layer == nil {
		return
	}
	RegisterReservedPluginDirectory(layer.Directory())
	for _, host := range layer.Hosts() {
		if host != nil {
			AllHosts = append(AllHosts, host)
		}
	}
	if store := layer.Store(); store != nil {
		externalStores = append(externalStores, store)
	}
}

// RegisterReservedPluginDirectory keeps a user-plugins child out of the packaged plugin scan.
func RegisterReservedPluginDirectory(name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return
	}
	reservedUserPluginDirectories[name] = struct{}{}
}

func isRegisteredReservedPluginDirectory(name string) bool {
	_, ok := reservedUserPluginDirectories[name]
	return ok
}

// pluginStoreManifestIDs records manifests already claimed by an earlier catalog.
func pluginStoreManifestIDs(manifests []StorePluginManifest) map[string]struct{} {
	known := make(map[string]struct{}, len(manifests))
	for _, manifest := range manifests {
		if id := strings.TrimSpace(manifest.Id); id != "" {
			known[id] = struct{}{}
		}
	}
	return known
}

// assignPluginStore marks manifests this catalog added. Rows already present keep their store.
func assignPluginStore(manifests []StorePluginManifest, storeID string, known map[string]struct{}) {
	storeID = strings.TrimSpace(storeID)
	if storeID == "" {
		return
	}
	for index := range manifests {
		if manifests[index].Store != "" {
			continue
		}
		if _, found := known[manifests[index].Id]; found {
			continue
		}
		manifests[index].Store = storeID
	}
}

// PluginStorePresentation returns the settings section title and mark for one catalog.
// An unknown id keeps a readable title and the generic store glyph.
func PluginStorePresentation(id string) (label string, icon common.WoxImage) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" || id == OfficialPluginStoreID {
		return "i18n:ui_plugin_store_wox", icons.Get(icons.BrandWox)
	}
	for _, store := range externalStores {
		if store == nil || !strings.EqualFold(store.Name(), id) {
			continue
		}
		label = strings.TrimSpace(store.Label())
		icon = store.Icon()
		if label == "" {
			label = fallbackPluginStoreLabel(id)
		}
		if icon.IsEmpty() {
			icon = icons.Get(icons.PluginWPM)
		}
		return label, icon
	}
	return fallbackPluginStoreLabel(id), icons.Get(icons.PluginWPM)
}

func fallbackPluginStoreLabel(id string) string {
	if id == "" {
		return "i18n:ui_plugin_store_wox"
	}
	return strings.ToUpper(id[:1]) + id[1:] + " Store"
}
