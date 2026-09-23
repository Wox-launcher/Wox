package ui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/util"

	"github.com/fsnotify/fsnotify"
)

const (
	themeWatchDebounce      = 500 * time.Millisecond
	themeReadRetryWindow    = time.Second
	themeManagedWriteWindow = 2 * time.Second
)

// startUserThemeMonitoring watches the user theme root and every theme package directory below it.
// fsnotify is not recursive, so each package directory is its own subscription on the shared
// watcher; new package directories subscribe when created and removed ones unsubscribe.
func (m *Manager) startUserThemeMonitoring(ctx context.Context, directory string) {
	m.ensureThemeWatchMaps()
	var watchesMu sync.Mutex
	watches := map[string]*util.DirectoryWatch{}
	var handleEvent func(event fsnotify.Event)
	addDirectory := func(name string) {
		watchesMu.Lock()
		defer watchesMu.Unlock()
		if _, exists := watches[name]; exists {
			return
		}
		watch, err := util.WatchDirectory(name, handleEvent, func(watchErr error) {
			util.GetLogger().Warn(ctx, fmt.Sprintf("theme watch: %v", watchErr))
		})
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("watch theme directory %s: %v", name, err))
			return
		}
		watches[name] = watch
	}
	// removeDirectory releases the subscription for name and everything below it. Deleting or
	// renaming a theme package emits one event for the package directory, while its assets
	// subdirectory was subscribed separately; leaving that entry behind would make a later
	// addDirectory treat it as already watched and asset edits would never arrive.
	removeDirectory := func(name string) {
		watchesMu.Lock()
		defer watchesMu.Unlock()
		prefix := name + string(filepath.Separator)
		for path, watch := range watches {
			if path == name || strings.HasPrefix(path, prefix) {
				watch.Close()
				delete(watches, path)
			}
		}
	}
	addDirectories := func(root string) {
		_ = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") && name != directory {
					return filepath.SkipDir
				}
				addDirectory(name)
			}
			return nil
		})
	}
	handleEvent = func(event fsnotify.Event) {
		relative, err := filepath.Rel(directory, event.Name)
		if err != nil {
			return
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if len(parts) == 0 || strings.HasPrefix(parts[0], ".") {
			return
		}
		info, statErr := os.Stat(event.Name)
		if event.Op&fsnotify.Create != 0 && statErr == nil && info.IsDir() {
			addDirectories(event.Name)
		}
		if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			removeDirectory(filepath.Clean(event.Name))
		}
		_, knownPackage := m.themeFileIDs.Load(themeWatchKey(filepath.Join(directory, parts[0], "theme.json")))
		if len(parts) > 1 || knownPackage || (statErr == nil && info.IsDir()) {
			m.debounceUserThemeSync(ctx, filepath.Join(directory, parts[0], "theme.json"))
		} else {
			m.handleUserThemeFileEvent(ctx, event)
		}
	}
	addDirectories(directory)
	if ctx.Done() == nil {
		return
	}
	<-ctx.Done()
	watchesMu.Lock()
	defer watchesMu.Unlock()
	for name, watch := range watches {
		watch.Close()
		delete(watches, name)
	}
}

func (m *Manager) startEmbedThemeMonitoring(ctx context.Context, directory string) {
	if _, err := os.Stat(directory); err != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("skip embed theme watch, directory missing: %s", directory))
		return
	}
	if _, err := util.WatchDirectoryChanges(ctx, directory, func(event fsnotify.Event) {
		m.handleEmbedThemeFileEvent(ctx, event)
	}); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to watch embed themes directory: %s", err.Error()))
	}
}

func (m *Manager) handleUserThemeFileEvent(ctx context.Context, event fsnotify.Event) {
	if event.Op&fsnotify.Chmod != 0 && event.Op&^fsnotify.Chmod == 0 {
		return
	}
	if shouldIgnoreThemeWatchName(filepath.Base(event.Name)) {
		return
	}
	m.debounceUserThemeSync(ctx, event.Name)
}

func (m *Manager) handleEmbedThemeFileEvent(ctx context.Context, event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
		return
	}
	if shouldIgnoreThemeWatchName(filepath.Base(event.Name)) {
		return
	}
	m.reloadEmbedThemeFile(ctx, event.Name)
}

func (m *Manager) debounceUserThemeSync(ctx context.Context, themePath string) {
	key := themeWatchKey(themePath)
	debounceThemeWatchTimer(m.themeReloadTimers, key, themeWatchDebounce, func() {
		if m.isThemeWatchIgnored(themePath) {
			return
		}
		m.syncUserThemeFile(util.NewTraceContext(), themePath)
	})
}

// syncUserThemeFile loads a dropped or edited user theme JSON, or unloads it when the file is gone.
func (m *Manager) syncUserThemeFile(ctx context.Context, themePath string) {
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		currentID := m.currentThemeID(ctx)
		removedID, ok := m.removeUserThemeByPath(themePath)
		if ok && currentID != "" && currentID == removedID {
			util.GetLogger().Info(ctx, fmt.Sprintf("active user theme removed, restoring default: %s", removedID))
			m.ChangeToDefaultTheme(ctx)
		}
		return
	}

	theme, err := m.upsertUserThemeFromFile(ctx, themePath)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to load user theme %s: %s", themePath, err.Error()))
		return
	}
	if m.currentThemeID(ctx) == theme.ThemeId {
		util.GetLogger().Info(ctx, fmt.Sprintf("active user theme changed, applying: %s", theme.ThemeName))
		m.ChangeTheme(ctx, theme)
	}
}

func (m *Manager) reloadEmbedThemeFile(ctx context.Context, themePath string) {
	data, err := readThemeFileWithRetry(themePath, themeReadRetryWindow)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to read embed theme: %s, %s", themePath, err.Error()))
		return
	}
	theme, err := m.parseTheme(string(data))
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse embed theme: %s, %s", themePath, err.Error()))
		return
	}
	existing, ok := m.themes.Load(theme.ThemeId)
	if !ok {
		return
	}
	theme.IsInstalled = true
	theme.IsSystem = existing.IsSystem
	m.themes.Store(theme.ThemeId, theme)
	util.GetLogger().Info(ctx, fmt.Sprintf("embed theme updated: %s", theme.ThemeName))
	if m.currentThemeID(ctx) == theme.ThemeId {
		m.ChangeTheme(ctx, theme)
	}
}

// upsertUserThemeFromFile parses one user theme JSON and stores it without applying it.
func (m *Manager) upsertUserThemeFromFile(ctx context.Context, themePath string) (common.Theme, error) {
	m.ensureThemeWatchMaps()
	data, err := readThemeFileWithRetry(themePath, themeReadRetryWindow)
	if err != nil {
		return common.Theme{}, err
	}
	theme, err := m.parseTheme(string(data))
	if err != nil {
		return common.Theme{}, err
	}
	if m.IsSystemTheme(theme.ThemeId) {
		return common.Theme{}, fmt.Errorf("theme id %s belongs to a system theme", theme.ThemeId)
	}
	// Load validates the package; the stored copy drops the bytes and reloads them on demand.
	if err := theme.LoadThemeAssets(filepath.Dir(themePath)); err != nil {
		return common.Theme{}, err
	}
	theme.IsInstalled = true
	theme.IsSystem = false
	key := themeWatchKey(themePath)
	if oldID, ok := m.themeFileIDs.Load(key); ok && oldID != theme.ThemeId {
		m.themes.Delete(oldID)
		m.themePackageDirs.Delete(oldID)
	}
	m.rememberUserThemeFile(themePath, theme.ThemeId)
	m.themes.Store(theme.ThemeId, stripThemeAssets(theme))
	util.GetLogger().Info(ctx, fmt.Sprintf("user theme loaded: %s (%s)", theme.ThemeName, theme.ThemeId))
	return theme, nil
}

func (m *Manager) removeUserThemeByPath(themePath string) (string, bool) {
	m.ensureThemeWatchMaps()
	key := themeWatchKey(themePath)
	themeID, ok := m.themeFileIDs.Load(key)
	if !ok {
		themeID = themeIDFromFileName(themePath)
	}
	m.themeFileIDs.Delete(key)
	if themeID == "" || m.IsSystemTheme(themeID) {
		return "", false
	}
	m.themePackageDirs.Delete(themeID)
	if _, exists := m.themes.Load(themeID); !exists {
		return "", false
	}
	m.themes.Delete(themeID)
	util.GetLogger().Info(context.Background(), fmt.Sprintf("user theme unloaded: %s", themeID))
	return themeID, true
}

func (m *Manager) rememberUserThemeFile(themePath, themeID string) {
	m.ensureThemeWatchMaps()
	m.themeFileIDs.Store(themeWatchKey(themePath), themeID)
	m.themePackageDirs.Store(themeID, filepath.Dir(themeWatchKey(themePath)))
}

// IgnoreThemeWatch temporarily gives a managed install or uninstall sole ownership of reload.
func (m *Manager) IgnoreThemeWatch(themePath string) {
	m.ensureThemeWatchMaps()
	m.themeWatchIgnored.Store(themeWatchKey(themePath), time.Now().Add(themeManagedWriteWindow).UnixMilli())
}

func (m *Manager) isThemeWatchIgnored(themePath string) bool {
	m.ensureThemeWatchMaps()
	key := themeWatchKey(themePath)
	deadline, exists := m.themeWatchIgnored.Load(key)
	if !exists {
		return false
	}
	if time.Now().UnixMilli() <= deadline {
		return true
	}
	m.themeWatchIgnored.Delete(key)
	return false
}

func (m *Manager) currentThemeID(ctx context.Context) string {
	return m.GetCurrentTheme(ctx).ThemeId
}

func (m *Manager) ensureThemeWatchMaps() {
	if m.themes == nil {
		m.themes = util.NewHashMap[string, common.Theme]()
	}
	if m.themeFileIDs == nil {
		m.themeFileIDs = util.NewHashMap[string, string]()
	}
	if m.themePackageDirs == nil {
		m.themePackageDirs = util.NewHashMap[string, string]()
	}
	if m.themeReloadTimers == nil {
		m.themeReloadTimers = util.NewHashMap[string, *time.Timer]()
	}
	if m.themeWatchIgnored == nil {
		m.themeWatchIgnored = util.NewHashMap[string, int64]()
	}
}

func shouldIgnoreThemeWatchName(name string) bool {
	if name == "" || name == ".DS_Store" {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "~") || strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".bak") || strings.HasSuffix(lower, ".swp") {
		return true
	}
	return filepath.Ext(lower) != ".json"
}

func themeIDFromFileName(themePath string) string {
	base := filepath.Base(themePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func themeWatchKey(themePath string) string {
	abs, err := filepath.Abs(themePath)
	if err != nil {
		return filepath.Clean(themePath)
	}
	return abs
}

func debounceThemeWatchTimer(timers *util.HashMap[string, *time.Timer], key string, delay time.Duration, fn func()) {
	if timers == nil {
		fn()
		return
	}
	if timer, exists := timers.Load(key); exists {
		timer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		current, exists := timers.Load(key)
		if !exists || current != timer {
			return
		}
		timers.Delete(key)
		fn()
	})
	timers.Store(key, timer)
}

func readThemeFileWithRetry(filePath string, retryWindow time.Duration) ([]byte, error) {
	deadline := time.Now().Add(retryWindow)
	var lastData []byte
	var lastErr error
	for {
		data, err := os.ReadFile(filePath)
		if err != nil {
			lastErr = err
		} else if lastData != nil && len(data) == len(lastData) {
			return data, nil
		} else {
			lastData = data
			lastErr = nil
		}
		if !time.Now().Before(deadline) {
			if lastData != nil {
				return lastData, nil
			}
			return nil, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}
