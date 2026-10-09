package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/plugin/thirdparty/flow/brand"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/plugin/thirdparty/migrate"
	"wox/setting"
	"wox/util"
)

const flowMigrationID = "flow"

type flowMigrationSource struct{}

func (flowMigrationSource) ID() string { return flowMigrationID }

// Detect reads Flow's data directory. Scoop and other portable installs keep it
// beside the app; the roaming directory is the fallback.
func (flowMigrationSource) Detect(ctx context.Context) (migrate.Installation, error) {
	root := flowUserDataRoot()
	if root == "" {
		return nil, nil
	}
	settingsPath := filepath.Join(root, "Settings", "Settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	settings, err := readFlowLauncherSettings(settingsPath)
	if err != nil {
		return nil, err
	}
	return flowInstallation{root: root, settings: settings}, nil
}

type flowInstallation struct {
	root     string
	settings flowLauncherSettings
}

func (f flowInstallation) ID() string            { return flowMigrationID }
func (f flowInstallation) Name() string          { return "Flow Launcher" }
func (f flowInstallation) Version() string       { return "" }
func (f flowInstallation) Location() string      { return f.root }
func (f flowInstallation) Icon() common.WoxImage { return brand.Image() }
func (f flowInstallation) Hotkey() string        { return strings.TrimSpace(f.settings.Hotkey) }

func (f flowInstallation) Plugins(ctx context.Context) ([]migrate.Plugin, error) {
	entries, err := os.ReadDir(filepath.Join(f.root, "Plugins"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	installed := installedFlowPluginIDs(ctx)
	pythonMissing := flowPythonMissing(ctx)
	var plugins []migrate.Plugin
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		directory := filepath.Join(f.root, "Plugins", entry.Name())
		if _, err := os.Stat(filepath.Join(directory, "plugin.json")); err != nil {
			continue
		}
		plugins = append(plugins, f.pluginFromDirectory(ctx, directory, installed, pythonMissing))
	}
	return plugins, nil
}

func (f flowInstallation) pluginFromDirectory(ctx context.Context, directory string, installed map[string]struct{}, pythonMissing bool) migrate.Plugin {
	document, err := readFlowPluginDocument(directory)
	if err != nil {
		name := filepath.Base(directory)
		return migrate.Plugin{ID: name, Name: name, Status: migrate.PluginUnsupported}
	}
	id := migrationFieldString(document, "ID", "Id")
	name := migrationFieldString(document, "Name")
	if name == "" {
		name = filepath.Base(directory)
	}
	item := migrate.Plugin{
		ID:          id,
		Name:        name,
		Description: migrationFieldString(document, "Description"),
		Version:     migrationFieldString(document, "Version"),
		Keywords:    migrationTriggerKeywords(document),
		Status:      migrate.PluginUnsupported,
	}
	if id == "" || !flowMigrationIDSafe(id) {
		return item
	}
	if override, ok := f.settings.plugin(id); ok {
		if len(override.keywords) > 0 {
			item.Keywords = override.keywords
		}
	}
	if _, ok := installed[strings.ToLower(id)]; ok {
		item.Status = migrate.PluginImported
		return item
	}
	if _, err := os.Stat(filepath.Join(manifest.CollectionDirectory(), id)); err == nil {
		item.Status = migrate.PluginImported
		return item
	}
	languageKind := manifest.LanguageKind(migrationFieldString(document, "Language"))
	if languageKind == "" || (languageKind == manifest.KindDotNet && runtime.GOOS != "windows") {
		return item
	}
	descriptor, err := manifest.Parse(directory)
	if err != nil {
		return item
	}
	item.Name = descriptor.Metadata.GetName(ctx)
	item.Description = descriptor.Metadata.GetDescription(ctx)
	item.Version = descriptor.Metadata.Version
	item.Icon = woxImageFromString(descriptor.Metadata.Icon)
	item.Status = migrate.PluginReady
	item.Selectable = true
	if pythonMissing && strings.EqualFold(descriptor.Language, "python") {
		item.Detail = migrate.DetailPythonMissing
	}
	return item
}

func (f flowInstallation) Import(ctx context.Context, itemIDs []string) (migrate.ImportResult, error) {
	var pluginIDs []string
	var settingIDs []string
	for _, requested := range itemIDs {
		requested = strings.TrimSpace(requested)
		if requested == "" {
			continue
		}
		if strings.HasPrefix(requested, "setting:") {
			settingIDs = append(settingIDs, requested)
			continue
		}
		pluginIDs = append(pluginIDs, requested)
	}
	result, err := f.importPlugins(ctx, pluginIDs)
	if err != nil {
		return result, err
	}
	writes, failures := f.settingImport(settingIDs)
	result.Settings = writes
	result.Plugins = append(result.Plugins, failures...)
	return result, nil
}

func (f flowInstallation) importPlugins(ctx context.Context, pluginIDs []string) (migrate.ImportResult, error) {
	plugins, err := f.Plugins(ctx)
	if err != nil {
		return migrate.ImportResult{}, err
	}
	byID := map[string]string{}
	selectable := map[string]flowPluginOverride{}
	entries, readErr := os.ReadDir(filepath.Join(f.root, "Plugins"))
	if readErr != nil && !os.IsNotExist(readErr) {
		return migrate.ImportResult{}, readErr
	}
	directories := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(f.root, "Plugins", entry.Name())
		document, err := readFlowPluginDocument(directory)
		if err != nil {
			continue
		}
		id := migrationFieldString(document, "ID", "Id")
		if id != "" {
			directories[strings.ToLower(id)] = directory
		}
	}
	for _, item := range plugins {
		byID[strings.ToLower(item.ID)] = item.ID
		if item.Selectable {
			override, _ := f.settings.plugin(item.ID)
			selectable[strings.ToLower(item.ID)] = override
		}
	}
	result := migrate.ImportResult{Plugins: make([]migrate.PluginResult, 0, len(pluginIDs))}
	for _, requested := range pluginIDs {
		key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(requested), "plugin:"))
		id := byID[key]
		if id == "" {
			id = requested
		}
		outcome := migrate.PluginResult{ID: requested}
		override, ok := selectable[key]
		directory := directories[key]
		if !ok || directory == "" {
			outcome.Error = "plugin cannot be imported"
			result.Plugins = append(result.Plugins, outcome)
			continue
		}
		if err := copyFlowPlugin(ctx, directory, id, override); err != nil {
			outcome.Error = err.Error()
		}
		result.Plugins = append(result.Plugins, outcome)
	}
	return result, nil
}

type flowPluginOverride struct {
	disabled bool
	keywords []string
	known    bool
}

func copyFlowPlugin(ctx context.Context, source, id string, override flowPluginOverride) error {
	if !flowMigrationIDSafe(id) {
		return fmt.Errorf("plugin id %q is not a safe directory name", id)
	}
	collection := manifest.CollectionDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(collection); err != nil {
		return err
	}
	dest := filepath.Join(collection, id)
	if !flowMigrationDestinationSafe(collection, dest) {
		return fmt.Errorf("plugin destination escapes the flow collection")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin %s is already in Wox", id)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := copyMigrationDirectory(source, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	descriptor, err := manifest.Parse(dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	if err := applyFlowPluginSettings(id, descriptor, override); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow migration settings %s: %s", id, err.Error()))
	}
	if database.GetDB() == nil {
		return nil
	}
	// A copied plugin is still imported when the host cannot start it in this session.
	if err := plugin.GetPluginManager().LoadRuntimePlugin(ctx, descriptor.Metadata); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow migration load %s: %s", id, err.Error()))
	}
	return nil
}

func applyFlowPluginSettings(id string, descriptor manifest.Descriptor, override flowPluginOverride) error {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	pluginSetting := setting.NewPluginSetting(setting.NewPluginSettingStore(db, id), descriptor.Metadata.SettingDefinitions.ToMap())
	if err := pluginSetting.Disabled.Set(override.known && override.disabled); err != nil {
		return err
	}
	keywords := descriptor.Metadata.TriggerKeywords
	if override.known && len(override.keywords) > 0 {
		keywords = override.keywords
	}
	if len(keywords) == 0 {
		return nil
	}
	return pluginSetting.TriggerKeywords.Set(keywords)
}

func installedFlowPluginIDs(ctx context.Context) map[string]struct{} {
	ids := map[string]struct{}{}
	descriptors, err := manifest.LoadDirectory(ctx, manifest.CollectionDirectory())
	if err != nil {
		return ids
	}
	for _, descriptor := range descriptors {
		ids[strings.ToLower(descriptor.Metadata.Id)] = struct{}{}
	}
	return ids
}

func flowPythonMissing(ctx context.Context) bool {
	if plugin.ResolvePythonPath == nil {
		return false
	}
	_, err := plugin.ResolvePythonPath(ctx)
	return err != nil
}

// flowUserDataRoot is the Flow data directory that contains Settings and Plugins.
// Portable installs, including Scoop, keep that directory beside the app.
// The roaming directory is used only when no portable UserData is present.
func flowUserDataRoot() string {
	if root := flowPortableUserData(); root != "" {
		return root
	}
	return flowSettingsRoot(filepath.Join(strings.TrimSpace(os.Getenv("APPDATA")), "FlowLauncher"))
}

func flowSettingsRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, "Settings", "Settings.json")); err != nil {
		return ""
	}
	return root
}

func flowPortableUserData() string {
	var parents []string
	if home := strings.TrimSpace(os.Getenv("USERPROFILE")); home != "" {
		parents = append(parents, filepath.Join(home, "scoop", "apps", "flow-launcher", "current"))
	}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		parents = append(parents, filepath.Join(local, "FlowLauncher"))
	}
	var best string
	var bestVersion string
	consider := func(appDir string) {
		userData := flowSettingsRoot(filepath.Join(appDir, "UserData"))
		if userData == "" {
			return
		}
		version := flowAppVersion(appDir)
		if best == "" || flowVersionLess(bestVersion, version) {
			best = userData
			bestVersion = version
		}
	}
	for _, parent := range parents {
		consider(parent)
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "app-") {
				consider(filepath.Join(parent, entry.Name()))
			}
		}
	}
	return best
}

func flowAppVersion(appDir string) string {
	name := filepath.Base(appDir)
	if len(name) > 4 && strings.EqualFold(name[:4], "app-") {
		return name[4:]
	}
	return name
}

func flowVersionLess(left, right string) bool {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	count := len(leftParts)
	if len(rightParts) > count {
		count = len(rightParts)
	}
	for index := 0; index < count; index++ {
		var leftNumber, rightNumber int
		if index < len(leftParts) {
			leftNumber, _ = strconv.Atoi(leftParts[index])
		}
		if index < len(rightParts) {
			rightNumber, _ = strconv.Atoi(rightParts[index])
		}
		if leftNumber == rightNumber {
			continue
		}
		return leftNumber < rightNumber
	}
	return false
}

type flowLauncherSettings struct {
	Hotkey                           string                `json:"Hotkey"`
	OpenContextMenuHotkey            string                `json:"OpenContextMenuHotkey"`
	IgnoreHotkeysOnFullscreen        bool                  `json:"IgnoreHotkeysOnFullscreen"`
	CustomPluginHotkeys              []flowCustomHotkey    `json:"CustomPluginHotkeys"`
	CustomShortcuts                  []flowCustomShortcut  `json:"CustomShortcuts"`
	LastQueryMode                    string                `json:"LastQueryMode"`
	ShowHistoryResultsForHomePage    bool                  `json:"ShowHistoryResultsForHomePage"`
	ShouldUsePinyin                  bool                  `json:"ShouldUsePinyin"`
	AlwaysStartEn                    bool                  `json:"AlwaysStartEn"`
	StartFlowLauncherOnSystemStartup bool                  `json:"StartFlowLauncherOnSystemStartup"`
	HideOnStartup                    bool                  `json:"HideOnStartup"`
	HideWhenDeactivated              bool                  `json:"HideWhenDeactivated"`
	HideNotifyIcon                   bool                  `json:"HideNotifyIcon"`
	WindowSize                       int                   `json:"WindowSize"`
	MaxResultsToShow                 int                   `json:"MaxResultsToShow"`
	SearchWindowScreen               string                `json:"SearchWindowScreen"`
	Language                         string                `json:"Language"`
	QueryBoxFont                     string                `json:"QueryBoxFont"`
	AutoUpdates                      bool                  `json:"AutoUpdates"`
	Proxy                            flowProxySettings     `json:"Proxy"`
	PluginSettings                   flowPluginRuntimePath `json:"PluginSettings"`
}

type flowPluginRuntimePath struct {
	PythonExecutablePath string                               `json:"PythonExecutablePath"`
	NodeExecutablePath   string                               `json:"NodeExecutablePath"`
	Plugins              map[string]flowLauncherPluginSetting `json:"Plugins"`
}

type flowCustomHotkey struct {
	Hotkey        string `json:"Hotkey"`
	ActionKeyword string `json:"ActionKeyword"`
}

type flowCustomShortcut struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type flowProxySettings struct {
	Enabled bool   `json:"Enabled"`
	Server  string `json:"Server"`
	Port    int    `json:"Port"`
}

type flowLauncherPluginSetting struct {
	ID             string   `json:"ID"`
	Disabled       bool     `json:"Disabled"`
	ActionKeyword  string   `json:"ActionKeyword"`
	ActionKeywords []string `json:"ActionKeywords"`
}

func (s flowLauncherSettings) plugin(id string) (flowPluginOverride, bool) {
	if s.PluginSettings.Plugins == nil {
		return flowPluginOverride{}, false
	}
	for key, item := range s.PluginSettings.Plugins {
		if !strings.EqualFold(key, id) && !strings.EqualFold(item.ID, id) {
			continue
		}
		keywords := append([]string(nil), item.ActionKeywords...)
		if len(keywords) == 0 && strings.TrimSpace(item.ActionKeyword) != "" {
			keywords = []string{strings.TrimSpace(item.ActionKeyword)}
		}
		return flowPluginOverride{disabled: item.Disabled, keywords: keywords, known: true}, true
	}
	return flowPluginOverride{}, false
}

func readFlowLauncherSettings(path string) (flowLauncherSettings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return flowLauncherSettings{}, err
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	var settings flowLauncherSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return flowLauncherSettings{}, fmt.Errorf("parse Flow Launcher settings: %w", err)
	}
	return settings, nil
}

func readFlowPluginDocument(directory string) (map[string]any, error) {
	raw, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		return nil, err
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func migrationFieldString(document map[string]any, names ...string) string {
	for _, name := range names {
		value, ok := document[name]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		default:
			if text := strings.TrimSpace(fmt.Sprint(typed)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func migrationTriggerKeywords(document map[string]any) []string {
	value, ok := document["ActionKeywords"]
	if !ok {
		value, ok = document["ActionKeyword"]
	}
	if !ok || value == nil {
		return []string{"*"}
	}
	var keywords []string
	switch typed := value.(type) {
	case string:
		keywords = []string{strings.TrimSpace(typed)}
	case []any:
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				keywords = append(keywords, text)
			}
		}
	}
	if len(keywords) == 0 {
		return []string{"*"}
	}
	return keywords
}

func woxImageFromString(value string) common.WoxImage {
	imageType, imageData, ok := strings.Cut(value, ":")
	if !ok || imageType == "" || imageData == "" {
		return common.WoxImage{}
	}
	return common.WoxImage{ImageType: imageType, ImageData: imageData}
}

func flowMigrationIDSafe(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	return filepath.Base(id) == id
}

func flowMigrationDestinationSafe(root, directory string) bool {
	root = filepath.Clean(root)
	directory = filepath.Clean(directory)
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == "." || relative == "" {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !strings.ContainsRune(relative, filepath.Separator)
}

func copyMigrationDirectory(src, dest string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyMigrationFile(path, target)
	})
}

func copyMigrationFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
