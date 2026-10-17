// Package migrate is the shared discovery API for third-party launcher imports.
// A compatibility layer registers one Source from init. Onboarding asks Detect
// which launchers are installed and never names a specific product.
package migrate

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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

	// CategoryHotkeys, CategoryQueries, CategoryGeneral, CategoryThemes, and CategoryPlugins
	// are the stable group ids onboarding translates. A launcher omits a group that has no items.
	CategoryHotkeys    = "hotkeys"
	CategoryQueries    = "queries"
	CategoryGeneral    = "general"
	CategoryThemes     = "themes"
	CategoryPlugins    = "plugins"
	CategoryQuicklinks = "quicklinks"
	CategoryShortcuts  = "shortcuts"
	CategorySnippets   = "snippets"
	CategoryNotes      = "notes"
	CategoryClipboard  = "clipboard"
	// CategoryPrepare is the locked first group. It quits the source launcher before hotkeys are registered.
	CategoryPrepare = "prepare"

	ItemKindSetting       = "setting"
	ItemKindPlugin        = "plugin"
	ItemKindQuicklink     = "quicklink"
	ItemKindFavorite      = "favorite"
	ItemKindNote          = "note"
	ItemKindClipboard     = "clipboard"
	ItemKindResultBinding = "result_binding"
	// ItemKindCloseLauncher quits the source launcher before the rest of the import.
	ItemKindCloseLauncher = "close_launcher"

	// ClipboardSummaryID is the single catalog row for a clipboard history import.
	ClipboardSummaryID = "clipboard:summary"
	// CloseLauncherItemID is the locked row that quits the source launcher.
	CloseLauncherItemID = "prepare:close_launcher"

	// StatusConflict means the same identity already points at different Wox content.
	StatusConflict = "conflict"

	// CodeExportRequired means Catalog was called before an encrypted export was unlocked.
	CodeExportRequired = "export_required"

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
	// Setting changes and mapped themes are returned for the caller to persist.
	Import(ctx context.Context, itemIDs []string) (ImportResult, error)
}

// RunningLauncher can release the global hotkeys it still owns by exiting.
// Onboarding quits it before registering the imported shortcuts.
type RunningLauncher interface {
	// CloseRunning stops the launcher process. A nil error means it is no longer running.
	CloseRunning(ctx context.Context) error
}

// ExportSource is an installation whose rows stay hidden until the user unlocks an export file.
type ExportSource interface {
	Installation
	// Recognize checks the signature without decrypting the file.
	Recognize(path string) (ok bool, detail string)
	// Unlock retains mapped data for Catalog and Import until Reset.
	Unlock(ctx context.Context, path, passphrase string) error
	// Reset cancels session work and releases retained export data. It is safe to call more than once.
	Reset()
	// Generation changes on every Unlock and Reset so a late result can be dropped.
	Generation() uint64
	// SetKeepKeyword changes one snippet's body option inside the unlocked session.
	SetKeepKeyword(itemID string, keep bool) bool
}

// Category is one group of migration choices, such as hotkeys or plugins.
type Category struct {
	ID    string
	Items []Item
}

// CloseLauncherCategory is the locked first group. Detail carries the launcher name for {name}.
func CloseLauncherCategory(name string) Category {
	return Category{ID: CategoryPrepare, Items: []Item{{
		ID: CloseLauncherItemID, Kind: ItemKindCloseLauncher, TitleKey: "close_launcher",
		DetailKey: "close_launcher", Detail: name, Required: true,
	}}}
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
	// Kind selects how onboarding applies the row. Empty keeps the older setting-prefix rule.
	Kind string
	// Required stays selected. The user cannot clear it, because the import depends on it.
	Required bool
	// OptionKey is an extra checkbox, such as keeping a snippet keyword in the note body.
	OptionKey string
	OptionOn  bool
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
// Themes are new Wox themes mapped from the source. The caller installs each one and selects it.
type ImportResult struct {
	Plugins        []PluginResult
	Settings       []SettingWrite
	Themes         []ThemeWrite
	Quicklinks     []QuicklinkWrite
	Favorites      []FavoriteWrite
	Notes          []NoteWrite
	Clipboard      []ClipboardWrite
	ResultBindings []ResultBindingWrite
	Results        []ItemResult
}

// ResultBindingWrite hangs an alias and hotkey on one result found by a plugin query.
type ResultBindingWrite struct {
	ItemID              string
	PluginID            string
	Search              string
	Command             string
	TriggerKeyword      string
	IdentityKey         string
	Path                string
	Title               string
	Keyword             string
	RequireSettingsPath bool
	Alias               string
	Hotkey              string
}

// ThemeWrite is one Wox theme produced by mapping a source launcher theme.
// Installing it and selecting its id is the whole theme migration.
type ThemeWrite struct {
	ItemIDs []string
	Theme   common.Theme
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

// QuicklinkWrite is one Web Search row mapped from a link quicklink.
type QuicklinkWrite struct {
	ItemID  string
	Title   string
	Keyword string
	URL     string
	Browser string
}

// FavoriteWrite is one file or folder quicklink mapped onto a Folder favorite.
type FavoriteWrite struct {
	ItemID string
	Name   string
	Path   string
}

// NoteWrite is one note mapped from a Raycast snippet or note.
// Document is set for a Raycast note. Snippets leave it empty and use Title and Body.
type NoteWrite struct {
	ItemID      string
	SourceID    string
	Fingerprint string
	Title       string
	Body        string
	Document    common.NoteDocument
}

// ClipboardWrite is mapped clipboard content. The clipboard package copies images and commits records.
type ClipboardWrite struct {
	ItemID    string
	Text      string
	ImagePath string
	CreatedAt time.Time
	Pinned    bool
}

// ItemResult is one non-plugin import outcome.
type ItemResult struct {
	ID        string
	Status    string
	ErrorCode string
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
