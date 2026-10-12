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
	// SupportedOS lists the platforms whose hosts may be installed.
	// Names match plugin metadata: Windows, Darwin, and Linux.
	// An empty list means every platform. Runtime settings, host startup,
	// and the plugin runtime filter all read the installed host list, so a
	// layer that cannot run here must not contribute hosts.
	SupportedOS() []string
}

// PluginRemover deletes a plugin this layer copied onto disk during import.
// It is not a plugin catalog. A layer that does not implement it has nothing to remove.
type PluginRemover interface {
	Uninstall(ctx context.Context, instance *Instance, skipCleanSetting bool, preserveCache bool, progress UninstallProgressCallback) (bool, error)
}

var (
	pluginRemovers []PluginRemover

	// compatibilityHosts are runnable, but they stay off the runtime settings page.
	// A compatibility layer is not a first-class runtime the user configures there.
	compatibilityHosts = map[Host]struct{}{}

	// ResolvePythonPath and ResolveNodePath are filled by the built-in interpreter
	// hosts. Compatibility layers read them when a host starts, after init has finished.
	ResolvePythonPath func(ctx context.Context) (string, error)
	ResolveNodePath   func(ctx context.Context) (string, error)

	reservedUserPluginDirectories = map[string]struct{}{}
)

// RegisterThirdParty adds a compatibility layer's hosts and directory.
// init calls this once. The directory stays reserved on every system.
// Hosts are installed only where SupportedOS includes the current platform.
func RegisterThirdParty(layer Layer) {
	if layer == nil {
		return
	}
	RegisterReservedPluginDirectory(layer.Directory())
	if remover, ok := layer.(PluginRemover); ok && remover != nil {
		pluginRemovers = append(pluginRemovers, remover)
	}
	if !layerRunsOnCurrentOS(layer) {
		return
	}
	for _, host := range layer.Hosts() {
		if host == nil {
			continue
		}
		AllHosts = append(AllHosts, host)
		compatibilityHosts[host] = struct{}{}
	}
}

// RuntimeSettingsVisible reports whether a host belongs on the runtime settings page.
// Compatibility hosts still load imported plugins. They are omitted from that page
// and from the plugin runtime filter so they are not presented as Wox runtimes.
func RuntimeSettingsVisible(host Host) bool {
	if host == nil {
		return false
	}
	_, hidden := compatibilityHosts[host]
	return !hidden
}

// layerRunsOnCurrentOS reports whether this machine may install the layer's hosts.
// An empty SupportedOS list means the layer runs on every platform.
func layerRunsOnCurrentOS(layer Layer) bool {
	supported := layer.SupportedOS()
	if len(supported) == 0 {
		return true
	}
	return IsAnySupportedInCurrentOS(supported)
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

// PluginStorePresentation returns the settings section title and mark for one catalog.
// An unknown id keeps a readable title and the generic store glyph.
func PluginStorePresentation(id string) (label string, icon common.WoxImage) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" || id == OfficialPluginStoreID {
		return "i18n:ui_plugin_store_wox", icons.Get(icons.BrandWox)
	}
	return fallbackPluginStoreLabel(id), icons.Get(icons.PluginWPM)
}

func fallbackPluginStoreLabel(id string) string {
	if id == "" {
		return "i18n:ui_plugin_store_wox"
	}
	return strings.ToUpper(id[:1]) + id[1:] + " Store"
}
