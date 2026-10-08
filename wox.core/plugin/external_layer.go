package plugin

import (
	"context"
	"strings"
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
type ExternalStore interface {
	Name() string
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
