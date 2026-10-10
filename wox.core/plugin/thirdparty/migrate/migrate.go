// Package migrate is the shared discovery API for third-party launcher imports.
// A compatibility layer registers one Source from init. Onboarding asks Detect
// which launchers are installed and never names a specific product.
package migrate

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"wox/common"
	"wox/util"
)

const (
	// PluginReady can be copied.
	PluginReady = "ready"
	// PluginImported is already present in Wox.
	PluginImported = "imported"
	// PluginUnsupported cannot be loaded by the owning compatibility layer.
	PluginUnsupported = "unsupported"
	// DetailPythonMissing means a Python plugin can be copied before Python is installed.
	DetailPythonMissing = "python_missing"

	// CategoryHotkeys, CategoryQueries, CategoryGeneral, and CategoryPlugins are the
	// stable group ids onboarding translates. A launcher omits a group that has no items.
	CategoryHotkeys = "hotkeys"
	CategoryQueries = "queries"
	CategoryGeneral = "general"
	CategoryPlugins = "plugins"

	// ValueOn and the other value ids are stable labels for a setting's current choice.
	ValueOn             = "on"
	ValueOff            = "off"
	ValuePositionCursor = "position_cursor"
	ValuePositionFocus  = "position_focus"
	ValueLaunchContinue = "launch_continue"
	ValueLaunchFresh    = "launch_fresh"
	ValueStartMRU       = "start_mru"
	ValueStartBlank     = "start_blank"
)

// Source is one third-party launcher that can be imported into Wox.
type Source interface {
	ID() string
	// Detect reports a local installation. Nil means this launcher is not installed.
	Detect(ctx context.Context) (Installation, error)
}

// Installation is one detected launcher and the settings and plugins that can be copied from it.
type Installation interface {
	ID() string
	Name() string
	Version() string
	Location() string
	Icon() common.WoxImage
	// Hotkey is the launcher's main shortcut, or empty when it has none.
	Hotkey() string
	Plugins(ctx context.Context) ([]Plugin, error)
	// Catalog groups every setting and plugin the user can choose. Empty groups are omitted.
	Catalog(ctx context.Context) ([]Category, error)
	// Import copies the requested item IDs. It does not remove or modify the source launcher.
	// Setting changes are returned for the caller to persist.
	Import(ctx context.Context, itemIDs []string) (ImportResult, error)
}

// Category is one group of migration choices, such as hotkeys or plugins.
type Category struct {
	ID    string
	Items []Item
}

// Item is one setting or plugin the user can include or leave behind.
// TitleKey and DetailKey are stable translation ids. Title and Detail are used when the text comes from the source launcher.
type Item struct {
	ID         string
	TitleKey   string
	Title      string
	DetailKey  string
	Detail     string
	DetailCode string
	Icon       common.WoxImage
	Selectable bool
	Status     string
}

// Plugin is one candidate shown before import.
type Plugin struct {
	ID          string
	Name        string
	Description string
	Version     string
	Keywords    []string
	Icon        common.WoxImage
	Selectable  bool
	Status      string
	Detail      string
}

// ImportResult is the outcome of one import.
// One SettingWrite can cover several items when Wox stores them as a single list.
type ImportResult struct {
	Plugins  []PluginResult
	Settings []SettingWrite
}

// SettingWrite is one Wox setting value produced by the selected items.
// QueryHotkeys and QueryAliases contain only the selected rows. Callers add those
// rows to the stored table and leave existing rows in place. Other keys replace the current value.
type SettingWrite struct {
	ItemIDs []string
	Key     string
	Value   string
}

// PluginResult records whether one requested plugin was copied.
type PluginResult struct {
	ID    string
	Error string
}

var (
	registryMu sync.Mutex
	registry   []Source
)

// Register adds a launcher source. init calls this once per compatibility layer.
// A repeated ID is ignored so tests and reloads do not stack duplicates.
func Register(source Source) {
	if source == nil || strings.TrimSpace(source.ID()) == "" {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, existing := range registry {
		if existing.ID() == source.ID() {
			return
		}
	}
	registry = append(registry, source)
}

// Detect returns every registered launcher that is installed on this machine.
// A source that fails to probe is skipped; the remaining installations are still returned.
func Detect(ctx context.Context) ([]Installation, error) {
	registryMu.Lock()
	sources := append([]Source(nil), registry...)
	registryMu.Unlock()

	var found []Installation
	for _, source := range sources {
		installation, err := source.Detect(ctx)
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("migration source %s: %s", source.ID(), err.Error()))
			continue
		}
		if installation == nil {
			continue
		}
		found = append(found, installation)
	}
	return found, nil
}
