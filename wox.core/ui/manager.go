package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"wox/analytics"
	"wox/common"
	"wox/diagnostic"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/resource"
	"wox/setting"
	"wox/updater"
	"wox/util"
	"wox/util/appearance"
	"wox/util/autostart"
	"wox/util/hotkey"
	"wox/util/ime"
	"wox/util/keyboard"
	"wox/util/screen"
	"wox/util/selection"
	"wox/util/shell"
	"wox/util/tray"
	"wox/util/window"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	cp "github.com/otiai10/copy"
	"github.com/samber/lo"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	mainHotkey       *hotkey.Hotkey
	selectionHotkey  *hotkey.Hotkey
	queryHotkeys     []*hotkey.Hotkey
	ui               common.UI
	serverPort       int
	uiProcess        *os.Process
	uiStopRequested  atomic.Bool
	themes           *util.HashMap[string, common.Theme]
	systemThemeIds   []string
	isUIReadyHandled bool
	isSystemDark     bool
	exitOnce         sync.Once

	activeWindowSnapshot    common.ActiveWindowSnapshot // cached active window snapshot
	activeWindowSnapshotMu  sync.RWMutex
	activeWindowSnapshotSeq uint64
	pendingStartupNotify    *common.NotifyMsg
}

func GetUIManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		managerInstance.mainHotkey = &hotkey.Hotkey{}
		managerInstance.selectionHotkey = &hotkey.Hotkey{}
		managerInstance.ui = &uiImpl{
			requestMap:             util.NewHashMap[string, chan WebsocketMsg](),
			sessionStates:          map[string]*uiSessionState{},
			isVisible:              false, // Initially hidden
			isSettingWindowOpen:    false,
			isOnboardingWindowOpen: false,
		}
		terminal.GetSessionManager().SetEmitter(func(ctx context.Context, uiSessionID string, method string, data any) {
			responseUI(ctx, WebsocketMsg{
				RequestId: uuid.NewString(),
				TraceId:   util.GetContextTraceId(ctx),
				SessionId: uiSessionID,
				Method:    method,
				Success:   true,
				Data:      data,
			})
		})
		managerInstance.themes = util.NewHashMap[string, common.Theme]()
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Start(ctx context.Context) error {
	//load embed themes
	embedThemes := resource.GetEmbedThemes(ctx)
	for _, themeJson := range embedThemes {
		theme, themeErr := m.parseTheme(themeJson)
		if themeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse theme: %s", themeErr.Error()))
			continue
		}
		theme.IsInstalled = true
		theme.IsSystem = true
		m.themes.Store(theme.ThemeId, theme)
		m.systemThemeIds = append(m.systemThemeIds, theme.ThemeId)
	}

	//load user themes
	userThemesDirectory := util.GetLocation().GetThemeDirectory()
	dirEntry, readErr := os.ReadDir(userThemesDirectory)
	if readErr != nil {
		return readErr
	}
	for _, entry := range dirEntry {
		if entry.IsDir() {
			continue
		}

		themeData, readThemeErr := os.ReadFile(userThemesDirectory + "/" + entry.Name())
		if readThemeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to read user theme: %s, %s", entry.Name(), readThemeErr.Error()))
			continue
		}

		theme, themeErr := m.parseTheme(string(themeData))
		if themeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse user theme: %s, %s", entry.Name(), themeErr.Error()))
			continue
		}
		m.themes.Store(theme.ThemeId, theme)
	}

	if util.IsDev() {
		var onThemeChange = func(e fsnotify.Event) {
			var themePath = e.Name

			//skip temp file
			if strings.HasSuffix(themePath, ".json~") {
				return
			}

			if e.Op == fsnotify.Write || e.Op == fsnotify.Create {
				logger.Info(ctx, fmt.Sprintf("user theme changed: %s", themePath))
				themeData, readThemeErr := os.ReadFile(themePath)
				if readThemeErr != nil {
					logger.Error(ctx, fmt.Sprintf("failed to read user theme: %s, %s", themePath, readThemeErr.Error()))
					return
				}

				changedTheme, themeErr := m.parseTheme(string(themeData))
				if themeErr != nil {
					logger.Error(ctx, fmt.Sprintf("failed to parse user theme: %s, %s", themePath, themeErr.Error()))
					return
				}

				//replace theme if current theme is the same
				if _, ok := m.themes.Load(changedTheme.ThemeId); ok {
					m.themes.Store(changedTheme.ThemeId, changedTheme)
					logger.Info(ctx, fmt.Sprintf("theme updated: %s", changedTheme.ThemeName))
					if m.GetCurrentTheme(ctx).ThemeId == changedTheme.ThemeId {
						m.ChangeTheme(ctx, changedTheme)
					}
				}
			}
		}

		//watch embed themes folder
		util.Go(ctx, "watch embed themes", func() {
			workingDirectory, wdErr := os.Getwd()
			if wdErr == nil {
				util.WatchDirectoryChanges(ctx, filepath.Join(workingDirectory, "resource", "ui", "themes"), onThemeChange)
			}
		})

		//watch user themes folder and reload themes
		util.Go(ctx, "watch user themes", func() {
			util.WatchDirectoryChanges(ctx, userThemesDirectory, onThemeChange)
		})
	}

	util.Go(ctx, "start store manager", func() {
		GetStoreManager().Start(util.NewTraceContext())
	})

	// Start watching system appearance changes for auto theme switching
	m.isSystemDark = appearance.IsDark()
	util.Go(ctx, "watch system appearance", func() {
		appearance.WatchSystemAppearance(func(isDark bool) {
			if m.isSystemDark != isDark {
				m.isSystemDark = isDark
				logger.Info(ctx, fmt.Sprintf("system appearance changed: isDark=%v", isDark))
				m.applyAutoAppearanceThemeIfNeed(ctx)
			}
		})
	})

	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	if util.IsDev() {
		logger.Info(ctx, "skip stopping ui app in dev mode")
		return
	}
	if m.uiProcess == nil {
		logger.Info(ctx, "skip stopping ui app because no ui process is tracked")
		return
	}

	logger.Info(ctx, "start stopping ui app")
	m.uiStopRequested.Store(true)
	var pid = m.uiProcess.Pid
	killErr := m.uiProcess.Kill()
	if killErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to kill ui process(%d): %s", pid, killErr))
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("killed ui process(%d)", pid))
	}
}

func (m *Manager) RegisterMainHotkey(ctx context.Context, combineKey string) error {
	if combineKey == "" {
		// remove hotkey
		logger.Info(ctx, "remove main hotkey")
		if m.mainHotkey != nil {
			m.mainHotkey.Unregister(ctx)
		}
		return nil
	}

	logger.Info(ctx, fmt.Sprintf("register main hotkey: %s", combineKey))
	// unregister previous hotkey
	if m.mainHotkey != nil {
		m.mainHotkey.Unregister(ctx)
	}

	managerInstance.mainHotkey = &hotkey.Hotkey{}
	return m.mainHotkey.Register(ctx, combineKey, func() {
		triggerCtx := util.NewTraceContext()
		logger.Info(triggerCtx, fmt.Sprintf("main hotkey callback received: hotkey=%s recordingActive=%t", combineKey, m.isHotkeyRecordingActive()))
		if m.recordHotkeyIfRecording(triggerCtx, combineKey) {
			return
		}
		if m.shouldIgnoreHotkeyTrigger(triggerCtx) {
			return
		}
		activationStartedAt := util.GetSystemTimestamp()
		m.ui.ToggleApp(triggerCtx, common.ShowContext{
			SelectAll:           true,
			ShowSource:          common.ShowSourceDefault,
			ActivationStartedAt: activationStartedAt,
		})
	})
}

func (m *Manager) RegisterSelectionHotkey(ctx context.Context, combineKey string) error {
	if combineKey == "" {
		// remove hotkey
		logger.Info(ctx, "remove selection hotkey")
		if m.selectionHotkey != nil {
			m.selectionHotkey.Unregister(ctx)
		}
		return nil
	}

	logger.Info(ctx, fmt.Sprintf("register selection hotkey: %s", combineKey))
	// unregister previous hotkey
	if m.selectionHotkey != nil {
		m.selectionHotkey.Unregister(ctx)
	}

	managerInstance.selectionHotkey = &hotkey.Hotkey{}
	return m.selectionHotkey.Register(ctx, combineKey, func() {
		triggerCtx := util.NewTraceContext()
		logger.Info(triggerCtx, fmt.Sprintf("selection hotkey callback received: hotkey=%s recordingActive=%t", combineKey, m.isHotkeyRecordingActive()))
		if m.recordHotkeyIfRecording(triggerCtx, combineKey) {
			return
		}
		if m.shouldIgnoreHotkeyTrigger(triggerCtx) {
			return
		}
		m.QuerySelection(triggerCtx)
	})
}

func (m *Manager) QuerySelection(ctx context.Context) {
	newCtx := util.NewTraceContext()
	start := util.GetSystemTimestamp()
	selection, err := selection.GetSelected(newCtx)
	logger.Debug(newCtx, fmt.Sprintf("took %d ms to get selection", util.GetSystemTimestamp()-start))
	if err != nil {
		logger.Error(newCtx, fmt.Sprintf("failed to get selected: %s", err.Error()))
		return
	}
	if selection.IsEmpty() {
		logger.Info(newCtx, "no selection")
		return
	}

	if err := m.triggerSelectionQuery(newCtx, selection); err != nil {
		logger.Error(newCtx, fmt.Sprintf("failed to trigger selection query: %s", err.Error()))
	}
}

func (m *Manager) triggerSelectionQuery(ctx context.Context, selected selection.Selection) error {
	if selected.IsEmpty() {
		return errors.New("selection is empty")
	}

	m.RefreshActiveWindowSnapshot(ctx)
	m.openSecondaryInstance(ctx, string(common.ShowSourceSelection), common.PlainQuery{
		QueryType:      plugin.QueryTypeSelection,
		QuerySelection: selected,
	}, common.ShowContext{
		ShowSource: common.ShowSourceSelection,
	})
	return nil
}

// openSecondaryInstance routes query-owned transient UI into a secondary window so the primary launcher query is left untouched.
func (m *Manager) openSecondaryInstance(ctx context.Context, instanceName string, query common.PlainQuery, showContext common.ShowContext) {
	if query.QueryId == "" {
		query.QueryId = uuid.NewString()
	}

	m.ui.OpenWoxInstance(ctx, common.OpenWoxInstanceRequest{
		Role:         common.WoxInstanceRoleSecondary,
		InstanceName: instanceName,
		Query:        query,
		ShowApp:      showContext,
	})
}

func (m *Manager) triggerQueryHotkey(ctx context.Context, queryHotkey setting.QueryHotkey) error {
	queryCtx := util.WithCoreSessionContext(ctx)
	queryCtx = util.WithShowSourceContext(queryCtx, string(common.ShowSourceQueryHotkey))
	plainQuery := plugin.GetPluginManager().ReplaceQueryVariable(queryCtx, queryHotkey.Query)
	plainQuery.QueryId = uuid.NewString()

	// Query hotkeys build the plugin query immediately, so they keep the
	// blocking snapshot path while normal launcher activation can refresh slow
	// details in the background.
	m.RefreshActiveWindowSnapshotBlocking(queryCtx)
	q, _, err := plugin.GetPluginManager().NewQuery(queryCtx, plainQuery)
	if err != nil {
		return err
	}

	if queryHotkey.IsSilentExecution {
		success := plugin.GetPluginManager().QuerySilent(queryCtx, q)
		if !success {
			return fmt.Errorf("failed to execute silent query: %s", plainQuery.String())
		}
		logger.Info(queryCtx, fmt.Sprintf("silent query executed: %s", plainQuery.String()))
		return nil
	}

	isQueryFocus := false
	if plugin.GetPluginManager().IsTriggerKeywordAIChat(ctx, q.TriggerKeyword) {
		if plugin.GetPluginManager().GetAIChatPluginChater(ctx).IsAutoFocusToChatInputWhenOpenWithQueryHotkey(ctx) {
			isQueryFocus = true
		}
	}

	showContext := common.ShowContext{
		SelectAll:      false,
		IsQueryFocus:   isQueryFocus,
		HideQueryBox:   queryHotkey.HideQueryBox,
		HideToolbar:    queryHotkey.HideToolbar,
		WindowWidth:    normalizedWindowWidth(queryHotkey.Width),
		MaxResultCount: normalizedMaxResultCount(queryHotkey.MaxResultCount),
		ShowSource:     common.ShowSourceQueryHotkey,
	}
	if position, ok := m.getQueryHotkeyWindowPosition(queryCtx, queryHotkey); ok {
		showContext.WindowPosition = &position
	}

	if queryHotkey.HideQueryBox && queryHotkey.HideToolbar {
		normalizedHotkey := normalizeHotkeyForCompare(queryHotkey.Hotkey)
		m.openSecondaryInstance(queryCtx, "query-hotkey:"+normalizedHotkey, plainQuery, showContext)
		return nil
	}

	m.ui.ChangeQuery(queryCtx, plainQuery)
	m.ui.ShowApp(queryCtx, showContext)
	return nil
}

func (m *Manager) RegisterQueryHotkey(ctx context.Context, queryHotkey setting.QueryHotkey) error {
	combineKey := strings.TrimSpace(queryHotkey.Hotkey)
	if queryHotkey.Disabled || combineKey == "" {
		logger.Info(ctx, fmt.Sprintf("skip register query hotkey: disabled=%t hotkey=%s", queryHotkey.Disabled, queryHotkey.Hotkey))
		return nil
	}

	hk := &hotkey.Hotkey{}

	err := hk.Register(ctx, combineKey, func() {
		queryCtx := util.WithCoreSessionContext(util.NewTraceContext())
		logger.Info(queryCtx, fmt.Sprintf("query hotkey callback received: hotkey=%s query=%s recordingActive=%t", combineKey, queryHotkey.Query, m.isHotkeyRecordingActive()))
		if m.recordHotkeyIfRecording(queryCtx, combineKey) {
			return
		}
		if m.shouldIgnoreHotkeyTrigger(queryCtx) {
			return
		}
		if err := m.triggerQueryHotkey(queryCtx, queryHotkey); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to trigger query hotkey: %s", err.Error()))
		}
	})
	if err != nil {
		return err
	}

	m.queryHotkeys = append(m.queryHotkeys, hk)
	return nil
}

func (m *Manager) unregisterQueryHotkeys(ctx context.Context) {
	for _, hk := range m.queryHotkeys {
		hk.Unregister(ctx)
	}
	m.queryHotkeys = nil
}

type HotkeyAvailability struct {
	Available     bool
	ConflictType  string
	ConflictValue string
}

const (
	hotkeyConflictTypeMain      = "main"
	hotkeyConflictTypeSelection = "selection"
	hotkeyConflictTypeQuery     = "query"
	hotkeyConflictTypeSystem    = "system"
)

// CheckHotkeyAvailability checks Wox-owned settings before probing the platform registry.
func (m *Manager) CheckHotkeyAvailability(ctx context.Context, hotkeyStr string) HotkeyAvailability {
	if conflict := m.findConfiguredHotkeyConflict(ctx, hotkeyStr); conflict.ConflictType != "" {
		logger.Info(ctx, fmt.Sprintf("hotkey availability check: hotkey=%s available=false reason=wox_setting conflictType=%s conflictValue=%s", hotkeyStr, conflict.ConflictType, conflict.ConflictValue))
		return conflict
	}

	isAvailable := hotkey.IsHotkeyAvailable(ctx, hotkeyStr)
	logger.Info(ctx, fmt.Sprintf("hotkey availability check: hotkey=%s available=%t reason=platform_probe", hotkeyStr, isAvailable))
	if !isAvailable {
		return HotkeyAvailability{Available: false, ConflictType: hotkeyConflictTypeSystem}
	}
	return HotkeyAvailability{Available: true}
}

// IsHotkeyAvailable keeps the existing bool endpoint compatible with callers that only need availability.
func (m *Manager) IsHotkeyAvailable(ctx context.Context, hotkeyStr string) bool {
	return m.CheckHotkeyAvailability(ctx, hotkeyStr).Available
}

// findConfiguredHotkeyConflict keeps availability checks aligned with Wox-owned hotkey settings.
func (m *Manager) findConfiguredHotkeyConflict(ctx context.Context, hotkeyStr string) HotkeyAvailability {
	normalized := normalizeHotkeyForCompare(hotkeyStr)
	if normalized == "" {
		return HotkeyAvailability{Available: true}
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if normalizeHotkeyForCompare(woxSetting.MainHotkey.Get()) == normalized {
		return HotkeyAvailability{Available: false, ConflictType: hotkeyConflictTypeMain}
	}
	if normalizeHotkeyForCompare(woxSetting.SelectionHotkey.Get()) == normalized {
		return HotkeyAvailability{Available: false, ConflictType: hotkeyConflictTypeSelection}
	}

	for _, queryHotkey := range woxSetting.QueryHotkeys.Get() {
		if queryHotkey.Disabled {
			continue
		}
		if normalizeHotkeyForCompare(queryHotkey.Hotkey) == normalized {
			return HotkeyAvailability{Available: false, ConflictType: hotkeyConflictTypeQuery, ConflictValue: queryHotkey.Query}
		}
	}

	return HotkeyAvailability{Available: true}
}

// normalizeHotkeyForCompare canonicalizes common aliases so stored settings and recorder output compare consistently.
func normalizeHotkeyForCompare(hotkeyStr string) string {
	tokens := []string{}
	for _, token := range strings.Split(hotkeyStr, "+") {
		normalizedToken := normalizeHotkeyToken(token)
		if normalizedToken != "" {
			tokens = append(tokens, normalizedToken)
		}
	}

	if len(tokens) == 2 && tokens[0] == tokens[1] && isHotkeyModifierToken(tokens[0]) {
		return strings.Join(tokens, "+")
	}

	modifiers := map[string]bool{}
	key := ""
	for _, token := range tokens {
		if isHotkeyModifierToken(token) {
			modifiers[token] = true
			continue
		}
		if key == "" {
			key = token
		}
	}

	parts := []string{}
	for _, modifier := range []string{"ctrl", "shift", "alt", "meta"} {
		if modifiers[modifier] {
			parts = append(parts, modifier)
		}
	}
	if key != "" {
		parts = append(parts, key)
	}

	return strings.Join(parts, "+")
}

// normalizeHotkeyToken maps platform and UI aliases to one comparison token.
func normalizeHotkeyToken(token string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "", " ":
		return ""
	case "control":
		return "ctrl"
	case "option":
		return "alt"
	case "cmd", "command", "win", "windows", "super":
		return "meta"
	case "return":
		return "enter"
	case "arrowleft":
		return "left"
	case "arrowright":
		return "right"
	case "arrowup":
		return "up"
	case "arrowdown":
		return "down"
	default:
		return strings.ToLower(strings.TrimSpace(token))
	}
}

func isHotkeyModifierToken(token string) bool {
	return token == "ctrl" || token == "shift" || token == "alt" || token == "meta"
}

func (m *Manager) StartWebsocketAndWait(ctx context.Context) {
	serveAndWait(ctx, m.serverPort)
}

func (m *Manager) UpdateServerPort(port int) {
	m.serverPort = port
}

func (m *Manager) getUILaunchEnvs(ctx context.Context) []string {
	if !util.IsLinux() {
		return nil
	}

	if os.Getenv("GDK_BACKEND") != "" {
		return nil
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" || os.Getenv("DISPLAY") == "" {
		return nil
	}

	// Bug fix: native Wayland ignores the GTK positioning APIs that Wox uses to
	// place the launcher window, so the first show falls back to the compositor's
	// top-left placement. Prefer XWayland for the UI child process when both
	// Wayland and DISPLAY are present; this keeps the existing X11 move/resize
	// path working without changing the user's global desktop session.
	logger.Info(ctx, "start ui with GDK_BACKEND=x11 so Linux launcher positioning uses the X11 path under Wayland")
	return []string{"GDK_BACKEND=x11"}
}

func (m *Manager) StartUIApp(ctx context.Context) error {
	var appPath = util.GetLocation().GetUIAppPath()
	if fileInfo, statErr := os.Stat(appPath); os.IsNotExist(statErr) {
		logger.Info(ctx, "UI app not exist: "+appPath)
		return errors.New("UI app not exist")
	} else {
		if !util.IsFileExecAny(fileInfo.Mode()) {
			// add execute permission
			chmodErr := os.Chmod(appPath, 0755)
			if chmodErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to add execute permission to ui app: %s", chmodErr.Error()))
				return chmodErr
			} else {
				logger.Info(ctx, "added execute permission to ui app")
			}
		}
	}

	// Bug fix: on a fresh Windows 10 install the Flutter runner can fail before
	// Dart code starts if the MSVC runtime is absent. Check the native runtime
	// dependencies while the Go backend can still explain the cause and direct
	// the user to Microsoft's installer instead of launching an opaque failing
	// child process.
	if dependencyErr := ensureUIRuntimeDependencies(ctx, appPath); dependencyErr != nil {
		m.ExitApp(ctx)
		return dependencyErr
	}

	logger.Info(ctx, fmt.Sprintf("start ui, path=%s, port=%d, pid=%d", appPath, m.serverPort, os.Getpid()))
	cmd, cmdErr := shell.RunWithEnv(appPath, m.getUILaunchEnvs(ctx),
		fmt.Sprintf("%d", m.serverPort),
		fmt.Sprintf("%d", os.Getpid()),
		fmt.Sprintf("%t", util.IsDev()),
	)
	if cmdErr != nil {
		return cmdErr
	}

	m.uiProcess = cmd.Process
	m.uiStopRequested.Store(false)
	pid := cmd.Process.Pid
	// Debug Glance reads this PID to report combined core + Flutter memory.
	// Prod launches the UI from core, while dev mode can later replace it with
	// the PID reported by Flutter's ready callback.
	util.SetWoxUIProcessPid(pid)
	util.GetLogger().Info(ctx, fmt.Sprintf("ui app pid: %d", pid))

	util.Go(ctx, "watch ui app", func() {
		waitErr := cmd.Wait()
		// Clear only this exited process so a restarted UI keeps its newer PID.
		util.ClearWoxUIProcessPid(pid)
		waitCtx := util.NewTraceContext()
		diagnostic.GetManager().RecordUIExit(waitCtx, pid, waitErr, m.uiStopRequested.Load())
		if waitErr != nil {
			logger.Warn(waitCtx, fmt.Sprintf("ui app process(%d) exited with error: %s", pid, waitErr.Error()))
			handleUIRuntimeLaunchFailure(waitCtx, waitErr)
		} else {
			logger.Info(waitCtx, fmt.Sprintf("ui app process(%d) exited", pid))
		}
		logger.Warn(waitCtx, "ui app exited, quitting backend")
		m.ExitApp(waitCtx)
	})

	return nil
}

func (m *Manager) GetCurrentTheme(ctx context.Context) common.Theme {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if v, ok := m.themes.Load(woxSetting.ThemeId.Get()); ok {
		// If it's an auto appearance theme, return the actual applied theme (light or dark)
		if v.IsAutoAppearance {
			return m.getActualTheme(ctx, v)
		}
		return m.resolvePlatformTheme(ctx, v)
	}

	return common.Theme{}
}

// getActualTheme returns the actual theme to apply based on system appearance
// It copies the auto theme's ID and flags but uses the light/dark theme's properties
func (m *Manager) getActualTheme(ctx context.Context, autoTheme common.Theme) common.Theme {
	var targetThemeId string
	if m.isSystemDark {
		targetThemeId = autoTheme.DarkThemeId
	} else {
		targetThemeId = autoTheme.LightThemeId
	}

	if targetTheme, ok := m.themes.Load(targetThemeId); ok {
		// Copy the target theme's properties but keep auto theme's identity
		result := targetTheme
		result.ThemeId = autoTheme.ThemeId
		result.IsAutoAppearance = autoTheme.IsAutoAppearance
		result.DarkThemeId = autoTheme.DarkThemeId
		result.LightThemeId = autoTheme.LightThemeId
		return m.resolvePlatformTheme(ctx, result)
	}

	// Fallback to auto theme if target not found
	return m.resolvePlatformTheme(ctx, autoTheme)
}

func (m *Manager) GetAllThemes(ctx context.Context) []common.Theme {
	var themes []common.Theme
	m.themes.Range(func(key string, value common.Theme) bool {
		themes = append(themes, m.resolvePlatformTheme(ctx, value))
		return true
	})
	return themes
}

func (m *Manager) AddTheme(ctx context.Context, theme common.Theme) {
	m.themes.Store(theme.ThemeId, theme)
	m.ChangeTheme(ctx, theme)
}

func (m *Manager) RemoveTheme(ctx context.Context, theme common.Theme) {
	m.themes.Delete(theme.ThemeId)
}

func (m *Manager) ChangeToDefaultTheme(ctx context.Context) {
	if v, ok := m.themes.Load(setting.DefaultThemeId); ok {
		m.ChangeTheme(ctx, v)
	}
}

func (m *Manager) RestoreTheme(ctx context.Context) {
	var uninstallThemes = m.themes.FilterList(func(key string, theme common.Theme) bool {
		return !theme.IsSystem
	})

	for _, theme := range uninstallThemes {
		GetStoreManager().Uninstall(ctx, theme)
	}

	m.ChangeToDefaultTheme(ctx)
}

func (m *Manager) GetThemeById(themeId string) common.Theme {
	if v, ok := m.themes.Load(themeId); ok {
		return v
	}
	return common.Theme{}
}

func (m *Manager) parseTheme(themeJson string) (common.Theme, error) {
	var theme common.Theme
	parseErr := json.Unmarshal([]byte(themeJson), &theme)
	if parseErr != nil {
		return common.Theme{}, parseErr
	}
	return theme, nil
}

func (m *Manager) resolvePlatformTheme(ctx context.Context, theme common.Theme) common.Theme {
	platformName, platformOverride := m.getThemePlatformOverride(theme)
	if platformOverride == nil || len(*platformOverride) == 0 {
		return clearThemePlatformOverrides(theme)
	}

	// New feature: platform nodes are preserved on the stored Theme, but Flutter
	// still expects the old flat payload. Merge the current OS override here so
	// every caller receives the same effective style without teaching the UI about
	// platform-specific schema details.
	themeJSON, marshalErr := json.Marshal(theme)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal theme %s for platform override %s: %s", theme.ThemeId, platformName, marshalErr.Error()))
		return clearThemePlatformOverrides(theme)
	}

	var merged map[string]json.RawMessage
	unmarshalErr := json.Unmarshal(themeJSON, &merged)
	if unmarshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to prepare theme %s for platform override %s: %s", theme.ThemeId, platformName, unmarshalErr.Error()))
		return clearThemePlatformOverrides(theme)
	}

	// Legacy border aliases must still work inside platform overrides. If an
	// override uses the old alias, remove the canonical base value first so the
	// existing alias parser can treat the alias as the effective value.
	if _, ok := (*platformOverride)["ResultItemBorderLeft"]; ok {
		delete(merged, "ResultItemBorderLeftWidth")
	}
	if _, ok := (*platformOverride)["ResultItemActiveBorderLeft"]; ok {
		delete(merged, "ResultItemActiveBorderLeftWidth")
	}
	for fieldName, value := range *platformOverride {
		merged[fieldName] = value
	}

	delete(merged, "windows")
	delete(merged, "macos")
	delete(merged, "linux")

	resolvedJSON, marshalErr := json.Marshal(merged)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to encode resolved theme %s for platform override %s: %s", theme.ThemeId, platformName, marshalErr.Error()))
		return clearThemePlatformOverrides(theme)
	}

	var resolvedTheme common.Theme
	unmarshalErr = json.Unmarshal(resolvedJSON, &resolvedTheme)
	if unmarshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to resolve theme %s for platform override %s: %s", theme.ThemeId, platformName, unmarshalErr.Error()))
		return clearThemePlatformOverrides(theme)
	}

	return clearThemePlatformOverrides(resolvedTheme)
}

func (m *Manager) getThemePlatformOverride(theme common.Theme) (string, *common.ThemePlatformOverride) {
	switch util.GetCurrentPlatform() {
	case util.PlatformWindows:
		return "windows", theme.Windows
	case util.PlatformMacOS:
		return "macos", theme.MacOS
	case util.PlatformLinux:
		return "linux", theme.Linux
	default:
		return util.GetCurrentPlatform(), nil
	}
}

func clearThemePlatformOverrides(theme common.Theme) common.Theme {
	theme.Windows = nil
	theme.MacOS = nil
	theme.Linux = nil
	return theme
}

func (m *Manager) ChangeTheme(ctx context.Context, theme common.Theme) {
	// If it's an auto appearance theme, save the auto theme ID but apply the appropriate light/dark theme
	if theme.IsAutoAppearance {
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		woxSetting.ThemeId.Set(theme.ThemeId)

		// Update system dark state and apply the appropriate theme
		m.isSystemDark = appearance.IsDark()
		m.applyAutoAppearanceThemeIfNeed(ctx)
	} else {
		m.GetUI(ctx).ChangeTheme(ctx, m.resolvePlatformTheme(ctx, theme))
	}
}

func (m *Manager) GetUI(ctx context.Context) common.UI {
	return m.ui
}

// called after UI is ready to show, and will execute only once
func (m *Manager) PostUIReady(ctx context.Context) {
	logger.Info(ctx, "app is ready to show")
	if m.isUIReadyHandled {
		logger.Warn(ctx, "app is already handled ready to show event")
		return
	}
	m.isUIReadyHandled = true
	if impl, ok := m.ui.(*uiImpl); ok {
		impl.setPrimarySession(util.GetContextSessionId(ctx))
	}

	// Apply auto appearance theme on startup
	m.applyAutoAppearanceThemeIfNeed(ctx)

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !woxSetting.OnboardingFinished.Get() {
		// The first-run guide must win over HideOnStart so every user data
		// directory gets one skippable setup pass before normal launcher startup.
		m.ui.OpenOnboardingWindow(ctx)
		return
	}

	if !woxSetting.HideOnStart.Get() {
		m.ui.ShowApp(ctx, common.ShowContext{})
	}
}

func (m *Manager) PostOnShow(ctx context.Context) {
	// Update cached visibility state
	if impl, ok := m.ui.(*uiImpl); ok {
		sessionId := util.GetContextSessionId(ctx)
		impl.sessionStatesMu.Lock()
		if sessionId != "" {
			state := impl.getOrCreateSessionStateLocked(sessionId)
			state.isVisible = true
			state.isRecordingHotkey = false
		}
		if impl.isPrimarySession(sessionId) {
			impl.isVisible = true
			impl.isRecordingHotkey = false
		}
		impl.sessionStatesMu.Unlock()
	}

	analytics.TrackUIOpened(ctx)

	if m.pendingStartupNotify != nil {
		logger.Info(ctx, "showing pending startup notify")
		m.ui.Notify(ctx, *m.pendingStartupNotify)
		m.pendingStartupNotify = nil
	}
}

func (m *Manager) SetStartupNotify(msg common.NotifyMsg) {
	logger.Info(util.NewTraceContext(), "setting pending startup notify")
	m.pendingStartupNotify = &msg
}

func (m *Manager) PostOnQueryBoxFocus(ctx context.Context) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.SwitchInputMethodABC.Get() {
		util.GetLogger().Info(ctx, "switch input method to ABC on query box focus")
		switchErr := ime.SwitchInputMethodABC()
		if switchErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to switch input method to ABC: %s", switchErr.Error()))
		}
	}
}

func (m *Manager) PostOnHide(ctx context.Context) {
	// Update cached visibility state
	if impl, ok := m.ui.(*uiImpl); ok {
		sessionId := util.GetContextSessionId(ctx)
		impl.sessionStatesMu.Lock()
		if sessionId != "" {
			state := impl.getOrCreateSessionStateLocked(sessionId)
			state.isVisible = false
			state.isRecordingHotkey = false
		}
		if impl.isPrimarySession(sessionId) {
			impl.isVisible = false
			impl.isRecordingHotkey = false
		}
		impl.sessionStatesMu.Unlock()
	}
}

func (m *Manager) PostOnSetting(ctx context.Context, isSettingWindowOpen bool) {
	if impl, ok := m.ui.(*uiImpl); ok {
		sessionId := util.GetContextSessionId(ctx)
		impl.sessionStatesMu.Lock()
		if sessionId != "" {
			state := impl.getOrCreateSessionStateLocked(sessionId)
			state.isSettingWindowOpen = isSettingWindowOpen
			if !isSettingWindowOpen {
				state.isRecordingHotkey = false
			}
			if isSettingWindowOpen {
				state.isOnboardingWindowOpen = false
			}
		}
		if impl.isPrimarySession(sessionId) {
			impl.isSettingWindowOpen = isSettingWindowOpen
			if !isSettingWindowOpen {
				impl.isRecordingHotkey = false
			}
			if isSettingWindowOpen {
				impl.isOnboardingWindowOpen = false
			}
		}
		impl.sessionStatesMu.Unlock()
	}
}

func (m *Manager) PostOnOnboarding(ctx context.Context, isOnboardingWindowOpen bool) {
	if impl, ok := m.ui.(*uiImpl); ok {
		sessionId := util.GetContextSessionId(ctx)
		impl.sessionStatesMu.Lock()
		if sessionId != "" {
			state := impl.getOrCreateSessionStateLocked(sessionId)
			state.isOnboardingWindowOpen = isOnboardingWindowOpen
			if !isOnboardingWindowOpen {
				state.isRecordingHotkey = false
			}
			if isOnboardingWindowOpen {
				state.isSettingWindowOpen = false
			}
		}
		if impl.isPrimarySession(sessionId) {
			impl.isOnboardingWindowOpen = isOnboardingWindowOpen
			if !isOnboardingWindowOpen {
				impl.isRecordingHotkey = false
			}
			if isOnboardingWindowOpen {
				impl.isSettingWindowOpen = false
			}
		}
		impl.sessionStatesMu.Unlock()
	}
}

// PostOnHotkeyRecording tracks recorder focus so global hotkey callbacks can feed the active recorder.
func (m *Manager) PostOnHotkeyRecording(ctx context.Context, isRecording bool) {
	if impl, ok := m.ui.(*uiImpl); ok {
		sessionId := util.GetContextSessionId(ctx)
		impl.sessionStatesMu.Lock()
		if sessionId != "" {
			impl.getOrCreateSessionStateLocked(sessionId).isRecordingHotkey = isRecording
		}
		if impl.isPrimarySession(sessionId) {
			impl.isRecordingHotkey = isRecording
		}
		impl.sessionStatesMu.Unlock()
		logger.Info(ctx, fmt.Sprintf("hotkey recording state changed: %t", isRecording))
	}
}

func (m *Manager) PostOnInstanceDestroyed(ctx context.Context) {
	sessionId := util.GetContextSessionId(ctx)
	if sessionId == "" {
		return
	}
	if impl, ok := m.ui.(*uiImpl); ok {
		impl.removeSession(sessionId)
	}
	plugin.GetPluginManager().ClearSessionState(ctx, sessionId)
}

func (m *Manager) IsSystemTheme(id string) bool {
	return lo.Contains(m.systemThemeIds, id)
}

func (m *Manager) IsThemeUpgradable(id string, version string) bool {
	theme := m.GetThemeById(id)
	if theme.ThemeId != "" {
		existingVersion, existingErr := semver.NewVersion(theme.Version)
		currentVersion, currentErr := semver.NewVersion(version)
		if existingErr != nil && currentErr != nil && existingVersion != nil && currentVersion != nil {
			if existingVersion.GreaterThan(currentVersion) {
				return true
			}
		}
	}
	return false
}

func (m *Manager) ShowTray() {
	ctx := util.NewTraceContext()

	tray.CreateTray(resource.GetAppIcon(), func() {
		m.GetUI(ctx).ToggleApp(ctx, common.ShowContext{
			SelectAll: true,
		})
	},
		tray.MenuItem{
			Title: i18n.GetI18nManager().TranslateWox(ctx, "ui_tray_toggle_app"),
			Callback: func() {
				m.GetUI(ctx).ToggleApp(ctx, common.ShowContext{
					SelectAll: true,
				})
			},
		}, tray.MenuItem{
			Title: i18n.GetI18nManager().TranslateWox(ctx, "ui_tray_open_setting_window"),
			Callback: func() {
				m.GetUI(ctx).OpenSettingWindow(ctx, common.SettingWindowContext{Source: common.SettingWindowSourceTray})
			},
		}, tray.MenuItem{
			Title: i18n.GetI18nManager().TranslateWox(ctx, "ui_tray_quit"),
			Callback: func() {
				m.ExitApp(util.NewTraceContext())
			},
		})

	m.refreshTrayQueryIcons(ctx)
}

func (m *Manager) HideTray() {
	tray.RemoveTray()
}

func (m *Manager) PostSettingUpdate(ctx context.Context, key string, value string) {
	var vb bool
	var vs = value
	if vb1, err := strconv.ParseBool(vs); err == nil {
		vb = vb1
	}

	switch key {
	case "ShowTray":
		if vb {
			m.ShowTray()
		} else {
			m.HideTray()
		}
	case "MainHotkey":
		m.RegisterMainHotkey(ctx, vs)
	case "SelectionHotkey":
		m.RegisterSelectionHotkey(ctx, vs)
	case "LogLevel":
		util.GetLogger().SetLevel(vs)
	case "QueryHotkeys":
		// unregister previous hotkeys
		logger.Info(ctx, "post update query hotkeys, unregister previous query hotkeys")
		m.unregisterQueryHotkeys(ctx)

		queryHotkeys := setting.GetSettingManager().GetWoxSetting(ctx).QueryHotkeys.Get()
		for _, queryHotkey := range queryHotkeys {
			m.RegisterQueryHotkey(ctx, queryHotkey)
		}
	case "TrayQueries":
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		if woxSetting.ShowTray.Get() {
			m.refreshTrayQueryIcons(ctx)
		}
	case "LangCode":
		langCode := vs
		langErr := i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCode(langCode))
		if langErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to update lang: %s", langErr.Error()))
		}
	case "EnableAutostart":
		enabled := vb
		err := autostart.SetAutostart(ctx, enabled)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to set autostart: %s", err.Error()))
		}
	case "EnableAutoUpdate":
		updater.CheckForUpdatesWithCallback(ctx, nil)
	case "AIProviders":
		plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "models")
	}
}

func (m *Manager) refreshTrayQueryIcons(ctx context.Context) {
	if util.IsLinux() {
		// tray query is not supported on linux yet
		return
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	queryItems := make([]tray.QueryIconItem, 0, len(woxSetting.TrayQueries.Get()))
	settingMenuTitle := i18n.GetI18nManager().TranslateWox(ctx, "ui_tray_open_setting_window")
	for trayQueryIndex, trayQuery := range woxSetting.TrayQueries.Get() {
		if trayQuery.Disabled {
			continue
		}

		query := strings.TrimSpace(trayQuery.Query)
		if query == "" {
			continue
		}

		iconBytes := m.toTrayIconBytes(ctx, trayQuery.Icon)
		tooltip := query
		if len(tooltip) > 80 {
			tooltip = tooltip[:80]
		}

		queryItems = append(queryItems, tray.QueryIconItem{
			Icon:             iconBytes,
			Tooltip:          tooltip,
			ContextMenuTitle: settingMenuTitle,
			Callback: func(rect tray.ClickRect) {
				m.executeTrayQuery(util.NewTraceContext(), trayQuery, rect)
			},
			ContextMenuCallback: func() {
				openSettingCtx := util.NewTraceContext()
				m.GetUI(openSettingCtx).OpenSettingWindow(openSettingCtx, common.SettingWindowContext{
					Path:   "/general",
					Param:  fmt.Sprintf("tray_queries:%d", trayQueryIndex),
					Source: common.SettingWindowSourceTray,
				})
			},
		})
	}

	tray.SetQueryIcons(queryItems)
}

func (m *Manager) executeTrayQuery(ctx context.Context, trayQuery setting.TrayQuery, rect tray.ClickRect) {
	queryCtx := util.WithCoreSessionContext(ctx)
	queryCtx = util.WithShowSourceContext(queryCtx, string(common.ShowSourceTrayQuery))
	// ReplaceQueryVariable returns a PlainQuery whose type may be QueryTypeSelection
	// when {wox:selected_file} was resolved, so we no longer hard-code QueryTypeInput here.
	plainQuery := plugin.GetPluginManager().ReplaceQueryVariable(queryCtx, trayQuery.Query)
	plainQuery.QueryId = uuid.NewString()

	// Tray queries create and execute a plugin query in this call stack, so they
	// need the fully-populated snapshot instead of the launcher fast path.
	m.RefreshActiveWindowSnapshotBlocking(queryCtx)
	q, _, err := plugin.GetPluginManager().NewQuery(queryCtx, plainQuery)
	if err != nil {
		logger.Error(queryCtx, fmt.Sprintf("failed to create tray query: %s", err.Error()))
		return
	}

	isQueryFocus := false
	if plugin.GetPluginManager().IsTriggerKeywordAIChat(queryCtx, q.TriggerKeyword) {
		if plugin.GetPluginManager().GetAIChatPluginChater(queryCtx).IsAutoFocusToChatInputWhenOpenWithQueryHotkey(queryCtx) {
			isQueryFocus = true
		}
	}

	windowWidth := m.getTrayQueryWindowWidth(queryCtx, trayQuery)
	screenRect := m.getTrayQueryScreenRect(queryCtx, rect)
	windowHeight := m.getTrayQueryInitialWindowHeight(queryCtx, trayQuery)
	windowAnchorBottom := m.getTrayQueryWindowAnchorBottom(rect, screenRect)
	position := m.getTrayQueryWindowPosition(queryCtx, rect, screenRect, windowWidth, windowHeight, windowAnchorBottom)
	var trayAnchor *common.TrayAnchor
	if util.IsWindows() {
		trayAnchor = &common.TrayAnchor{
			WindowX: position.X,
			Bottom:  windowAnchorBottom,
			ScreenRect: common.WindowRect{
				X:      screenRect.X,
				Y:      screenRect.Y,
				Width:  screenRect.Width,
				Height: screenRect.Height,
			},
		}
		logger.Debug(queryCtx, fmt.Sprintf("tray query anchor resolved: windowX=%d bottom=%d screen=(x=%d y=%d w=%d h=%d)", trayAnchor.WindowX, trayAnchor.Bottom, trayAnchor.ScreenRect.X, trayAnchor.ScreenRect.Y, trayAnchor.ScreenRect.Width, trayAnchor.ScreenRect.Height))
	}
	m.openSecondaryInstance(queryCtx, "tray-query:"+strings.TrimSpace(trayQuery.Query), plainQuery, common.ShowContext{
		SelectAll:        false,
		IsQueryFocus:     isQueryFocus,
		HideQueryBox:     trayQuery.HideQueryBox,
		HideToolbar:      trayQuery.HideToolbar,
		QueryBoxAtBottom: runtime.GOOS == "windows",
		HideOnBlur:       true,
		ShowSource:       common.ShowSourceTrayQuery,
		WindowPosition:   &position,
		TrayAnchor:       trayAnchor,
		WindowWidth:      windowWidth,
		MaxResultCount:   trayQuery.MaxResultCount,
	})
}

func (m *Manager) getTrayQueryWindowWidth(ctx context.Context, trayQuery setting.TrayQuery) int {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	windowWidth := trayQuery.Width
	if windowWidth <= 0 {
		windowWidth = woxSetting.AppWidth.Get() / 2
	}
	if windowWidth <= 0 {
		windowWidth = 400
	}
	return windowWidth
}

func (m *Manager) getTrayQueryWindowPosition(ctx context.Context, rect tray.ClickRect, screenRect common.WindowRect, windowWidth int, windowHeight int, windowAnchorBottom int) common.WindowPosition {
	margin := 8
	x := screenRect.X + (screenRect.Width-windowWidth)/2
	y := screenRect.Y + 10

	if rect.Width > 0 && rect.Height > 0 {
		x = rect.X + (rect.Width-windowWidth)/2
		if util.IsWindows() {
			y = windowAnchorBottom - windowHeight
		} else {
			y = rect.Y + rect.Height + margin
		}
	} else if util.IsWindows() {
		y = windowAnchorBottom - windowHeight
	}

	minX := screenRect.X + 10
	maxX := screenRect.X + screenRect.Width - windowWidth - 10
	x = clampInt(x, minX, maxX)

	minY := screenRect.Y + 10
	maxY := screenRect.Y + screenRect.Height - windowHeight - 10
	if maxY < minY {
		maxY = minY
	}
	y = clampInt(y, minY, maxY)

	return common.WindowPosition{X: x, Y: y}
}

func (m *Manager) getTrayQueryWindowAnchorBottom(rect tray.ClickRect, screenRect common.WindowRect) int {
	margin := 8
	if rect.Width > 0 && rect.Height > 0 {
		if util.IsWindows() {
			return rect.Y - margin
		}
		return rect.Y + rect.Height + margin
	}

	if util.IsWindows() {
		return screenRect.Y + screenRect.Height - margin
	}

	return screenRect.Y + 10
}

func (m *Manager) getTrayQueryInitialWindowHeight(ctx context.Context, trayQuery setting.TrayQuery) int {
	theme := m.GetCurrentTheme(ctx)
	// Tray query popups start before Flutter has measured content, so backend
	// positioning must use the same density-scaled base heights as the launcher
	// render path while leaving theme padding untouched.
	queryBoxHeight := DensityQueryBoxBaseHeight(ctx) + theme.AppPaddingTop + theme.AppPaddingBottom
	if queryBoxHeight <= 0 {
		queryBoxHeight = 80
	}

	if !trayQuery.HideQueryBox {
		return queryBoxHeight
	}

	resultItemHeight := DensityResultItemBaseHeight(ctx) + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom
	if resultItemHeight <= 0 {
		resultItemHeight = 50
	}

	windowHeight := resultItemHeight + theme.AppPaddingBottom
	if windowHeight <= 0 {
		windowHeight = resultItemHeight
	}

	return windowHeight
}

func (m *Manager) getQueryHotkeyWindowPosition(ctx context.Context, queryHotkey setting.QueryHotkey) (common.WindowPosition, bool) {
	positionType := queryHotkey.Position
	if positionType == "" || positionType == setting.QueryHotkeyPositionSystemDefault {
		return common.WindowPosition{}, false
	}

	screenSize := screen.GetMouseScreen()
	windowWidth := m.getResolvedQueryHotkeyWindowWidth(ctx, queryHotkey)
	maxResultCount := m.getResolvedQueryHotkeyMaxResultCount(ctx, queryHotkey)
	windowHeight := CalculateMaxWindowHeight(ctx, maxResultCount, !queryHotkey.HideQueryBox, !queryHotkey.HideToolbar)
	const margin = 20

	left := screenSize.X + margin
	centerX := screenSize.X + (screenSize.Width-windowWidth)/2
	right := screenSize.X + screenSize.Width - windowWidth - margin
	if right < left {
		right = left
	}

	top := screenSize.Y + margin
	centerY := screenSize.Y + (screenSize.Height-windowHeight)/2
	bottom := screenSize.Y + screenSize.Height - windowHeight - margin
	if bottom < top {
		bottom = top
	}

	x := centerX
	y := centerY

	switch positionType {
	case setting.QueryHotkeyPositionTopLeft:
		x = left
		y = top
	case setting.QueryHotkeyPositionTopCenter:
		x = centerX
		y = top
	case setting.QueryHotkeyPositionTopRight:
		x = right
		y = top
	case setting.QueryHotkeyPositionCenter:
		x = centerX
		y = centerY
	case setting.QueryHotkeyPositionBottomLeft:
		x = left
		y = bottom
	case setting.QueryHotkeyPositionBottomCenter:
		x = centerX
		y = bottom
	case setting.QueryHotkeyPositionBottomRight:
		x = right
		y = bottom
	default:
		return common.WindowPosition{}, false
	}

	return common.WindowPosition{X: x, Y: y}, true
}

func (m *Manager) getResolvedQueryHotkeyWindowWidth(ctx context.Context, queryHotkey setting.QueryHotkey) int {
	windowWidth := normalizedWindowWidth(queryHotkey.Width)
	if windowWidth > 0 {
		return windowWidth
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.AppWidth.Get() > 0 {
		return woxSetting.AppWidth.Get()
	}

	return 800
}

func (m *Manager) getResolvedQueryHotkeyMaxResultCount(ctx context.Context, queryHotkey setting.QueryHotkey) int {
	maxResultCount := normalizedMaxResultCount(queryHotkey.MaxResultCount)
	if maxResultCount > 0 {
		return maxResultCount
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.MaxResultCount.Get() > 0 {
		return woxSetting.MaxResultCount.Get()
	}

	return 10
}

func normalizedWindowWidth(windowWidth int) int {
	if windowWidth < 0 {
		return 0
	}
	return windowWidth
}

func normalizedMaxResultCount(maxResultCount int) int {
	if maxResultCount <= 0 {
		return 0
	}
	return clampInt(maxResultCount, 5, 15)
}

func (m *Manager) getTrayQueryScreenRect(ctx context.Context, rect tray.ClickRect) common.WindowRect {
	displays, err := screen.ListDisplays()
	if err == nil {
		pointX := rect.X
		pointY := rect.Y
		if rect.Width > 0 && rect.Height > 0 {
			pointX = rect.X + rect.Width/2
			pointY = rect.Y + rect.Height/2
		}

		for _, display := range displays {
			workArea := display.WorkArea
			if pointX >= workArea.X && pointX < workArea.Right() && pointY >= workArea.Y && pointY < workArea.Bottom() {
				return common.WindowRect{
					X:      workArea.X,
					Y:      workArea.Y,
					Width:  workArea.Width,
					Height: workArea.Height,
				}
			}
		}
	}

	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to get tray query screen rect from display list, fallback to mouse screen: %s", err.Error()))
	}

	screenSize := screen.GetMouseScreen()
	return common.WindowRect{
		X:      screenSize.X,
		Y:      screenSize.Y,
		Width:  screenSize.Width,
		Height: screenSize.Height,
	}
}

func (m *Manager) toTrayIconBytes(ctx context.Context, icon common.WoxImage) []byte {
	if icon.IsEmpty() {
		return resource.GetAppIcon()
	}

	if svgBytes, ok := m.toMacOSTrayVectorBytes(ctx, icon); ok {
		return svgBytes
	}

	img, err := icon.ToImageWithoutRemoteFetch()
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to parse tray query icon, fallback to app icon: %s", err.Error()))
		return resource.GetAppIcon()
	}

	buf := bytes.NewBuffer(nil)
	if err := png.Encode(buf, img); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to encode tray query icon, fallback to app icon: %s", err.Error()))
		return resource.GetAppIcon()
	}

	if util.IsWindows() {
		icoBytes, err := wrapPNGAsICO(buf.Bytes(), img.Bounds().Dx(), img.Bounds().Dy())
		if err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to convert tray query icon to ico, fallback to app icon: %s", err.Error()))
			return resource.GetAppIcon()
		}
		return icoBytes
	}

	return buf.Bytes()
}

func (m *Manager) toMacOSTrayVectorBytes(ctx context.Context, icon common.WoxImage) ([]byte, bool) {
	if !util.IsMacOS() {
		return nil, false
	}

	if icon.ImageType == common.WoxImageTypeSvg {
		svgData := strings.TrimSpace(icon.ImageData)
		if svgData == "" {
			return nil, false
		}
		return []byte(svgData), true
	}

	if icon.ImageType == common.WoxImageTypeAbsolutePath && strings.EqualFold(filepath.Ext(icon.ImageData), ".svg") {
		svgData, err := os.ReadFile(icon.ImageData)
		if err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to read tray query svg icon, fallback to raster path: %s", err.Error()))
			return nil, false
		}

		return svgData, true
	}

	return nil, false
}

func wrapPNGAsICO(pngData []byte, width int, height int) ([]byte, error) {
	if len(pngData) == 0 {
		return nil, fmt.Errorf("empty png data")
	}

	if width <= 0 || width > 256 {
		width = 256
	}
	if height <= 0 || height > 256 {
		height = 256
	}

	widthByte := byte(width)
	if width == 256 {
		widthByte = 0
	}
	heightByte := byte(height)
	if height == 256 {
		heightByte = 0
	}

	buf := bytes.NewBuffer(nil)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // icon type
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // image count
	_ = buf.WriteByte(widthByte)
	_ = buf.WriteByte(heightByte)
	_ = buf.WriteByte(0) // color palette count
	_ = buf.WriteByte(0) // reserved
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(pngData)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(22)) // ICONDIR(6) + ICONDIRENTRY(16)
	_, _ = buf.Write(pngData)

	return buf.Bytes(), nil
}

func clampInt(v int, min int, max int) int {
	if min > max {
		return min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (m *Manager) ExitApp(ctx context.Context) {
	m.exitOnce.Do(func() {
		util.GetLogger().Info(ctx, "start quitting")
		plugin.GetPluginManager().Stop(ctx)
		m.Stop(ctx)
		diagnostic.GetManager().MarkCleanExit(ctx)
		util.GetLogger().Info(ctx, "bye~")
		os.Exit(0)
	})
}

func (m *Manager) GetActiveWindowSnapshot(ctx context.Context) common.ActiveWindowSnapshot {
	m.activeWindowSnapshotMu.RLock()
	defer m.activeWindowSnapshotMu.RUnlock()
	return m.activeWindowSnapshot
}

// RefreshActiveWindowSnapshot updates the cached active window snapshot without
// blocking launcher activation on expensive per-process details. The hotkey path
// only needs a stable foreground PID before Wox appears; name/icon/dialog state
// is filled later from that PID so macOS Accessibility calls cannot delay the
// first launcher frame.
func (m *Manager) RefreshActiveWindowSnapshot(ctx context.Context) {
	m.refreshActiveWindowSnapshot(ctx, false)
}

// RefreshActiveWindowSnapshotBlocking preserves the old fully-populated snapshot
// semantics for callers that immediately build or execute a plugin query. Those
// callers would otherwise read a PID-only snapshot before the background detail
// refresh has completed.
func (m *Manager) RefreshActiveWindowSnapshotBlocking(ctx context.Context) {
	m.refreshActiveWindowSnapshot(ctx, true)
}

func (m *Manager) refreshActiveWindowSnapshot(ctx context.Context, waitForDetails bool) {
	activeWindowPid := window.GetActiveWindowPid()
	activeWindowId := window.GetActiveWindowId()

	if activeWindowPid <= 0 {
		m.activeWindowSnapshotMu.Lock()
		m.activeWindowSnapshotSeq++
		m.activeWindowSnapshot = common.ActiveWindowSnapshot{}
		m.activeWindowSnapshotMu.Unlock()
		return
	}

	if m.isUIWindow("", activeWindowPid) {
		return
	}

	m.activeWindowSnapshotMu.Lock()
	m.activeWindowSnapshotSeq++
	snapshotSeq := m.activeWindowSnapshotSeq
	// Optimization: clear detail fields while keeping the PID immediately
	// available. Keeping old details with a new PID created mixed snapshots, and
	// blocking here made every launcher activation wait for icon and AX dialog
	// probes even when the UI only needed to become visible.
	m.activeWindowSnapshot = common.ActiveWindowSnapshot{Pid: activeWindowPid, WindowId: activeWindowId}
	m.activeWindowSnapshotMu.Unlock()

	if waitForDetails {
		m.refreshActiveWindowSnapshotDetails(activeWindowPid, snapshotSeq)
		return
	}

	util.Go(ctx, "refresh active window snapshot details", func() {
		m.refreshActiveWindowSnapshotDetails(activeWindowPid, snapshotSeq)
	})
}

func (m *Manager) refreshActiveWindowSnapshotDetails(activeWindowPid int, snapshotSeq uint64) {
	activeWindowName := window.GetWindowNameByPid(activeWindowPid)

	activeWindowIcon := common.WoxImage{}
	if icon, err := window.GetWindowIconByPid(activeWindowPid); err == nil {
		if woxIcon, convErr := common.NewWoxImage(icon); convErr == nil {
			activeWindowIcon = woxIcon
		}
	}

	activeWindowIsOpenSaveDialog := false
	if isDialog, err := window.IsOpenSaveDialogByPid(activeWindowPid); err == nil {
		activeWindowIsOpenSaveDialog = isDialog
	}

	m.activeWindowSnapshotMu.Lock()
	if m.activeWindowSnapshotSeq != snapshotSeq || m.activeWindowSnapshot.Pid != activeWindowPid {
		m.activeWindowSnapshotMu.Unlock()
		return
	}
	m.activeWindowSnapshot.Name = activeWindowName
	m.activeWindowSnapshot.Icon = activeWindowIcon
	m.activeWindowSnapshot.IsOpenSaveDialog = activeWindowIsOpenSaveDialog
	m.activeWindowSnapshotMu.Unlock()
}

func (m *Manager) shouldIgnoreHotkeyTrigger(ctx context.Context) bool {
	if m.isOnboardingViewActive() {
		// Bug fix: onboarding has its own hotkey setup UI and uses the shared
		// Wox window. The previous guard only checked ignored foreground apps,
		// so pressing a registered global hotkey during the guide could toggle
		// or replace the onboarding surface. Keeping the check in the common
		// hotkey gate blocks all global hotkey handlers while onboarding is active.
		logger.Info(ctx, "ignore hotkey trigger while onboarding is active")
		return true
	}

	ignoredApps := setting.GetSettingManager().GetWoxSetting(ctx).IgnoredHotkeyApps.Get()
	if len(ignoredApps) == 0 {
		return false
	}

	activeWindowName := window.GetActiveWindowName()
	activeWindowPid := window.GetActiveWindowPid()
	if m.isUIWindow(activeWindowName, activeWindowPid) {
		return false
	}

	identity := strings.TrimSpace(window.GetProcessIdentity(activeWindowPid))
	if identity == "" {
		return false
	}

	for _, app := range ignoredApps {
		if strings.EqualFold(strings.TrimSpace(app.Identity), identity) {
			logger.Info(ctx, fmt.Sprintf("ignore hotkey trigger for app identity=%s name=%s pid=%d", identity, activeWindowName, activeWindowPid))
			return true
		}
	}

	return false
}

// recordHotkeyIfRecording forwards Wox-owned global hotkey presses to the active recorder instead of executing them.
func (m *Manager) recordHotkeyIfRecording(ctx context.Context, hotkeyStr string) bool {
	if !m.isHotkeyRecordingActive() {
		return false
	}

	logger.Info(ctx, fmt.Sprintf("record registered hotkey while recording: %s", hotkeyStr))
	util.Go(ctx, "record global hotkey in UI", func() {
		m.ui.RecordHotkey(ctx, hotkeyStr)
	})
	return true
}

// isHotkeyRecordingActive reports whether the shared UI is currently capturing a hotkey.
func (m *Manager) isHotkeyRecordingActive() bool {
	if impl, ok := m.ui.(*uiImpl); ok {
		impl.sessionStatesMu.RLock()
		defer impl.sessionStatesMu.RUnlock()
		return impl.isRecordingHotkey
	}
	return false
}

func (m *Manager) isOnboardingViewActive() bool {
	if impl, ok := m.ui.(*uiImpl); ok {
		impl.sessionStatesMu.RLock()
		defer impl.sessionStatesMu.RUnlock()
		return impl.isOnboardingWindowOpen
	}
	return false
}

func (m *Manager) isUIWindow(activeWindowName string, activeWindowPid int) bool {
	if m.uiProcess != nil && activeWindowPid != 0 && m.uiProcess.Pid == activeWindowPid {
		return true
	}
	return strings.EqualFold(activeWindowName, "wox-ui")
}

func (m *Manager) ProcessDeeplink(ctx context.Context, deeplink string) {
	logger.Info(ctx, fmt.Sprintf("start processing deeplink: %s", deeplink))

	parts := strings.SplitN(deeplink, "?", 2)
	command := strings.TrimSuffix(strings.TrimPrefix(parts[0], "wox://"), "/")

	arguments := make(map[string]string)
	if len(parts) == 2 {
		queryParams := strings.Split(parts[1], "&")
		for _, param := range queryParams {
			keyValue := strings.SplitN(param, "=", 2)
			if len(keyValue) == 2 {
				key := keyValue[0]
				value, err := url.QueryUnescape(keyValue[1])
				if err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to unescape value: %s", err.Error()))
					continue
				}
				arguments[key] = value
			}
		}
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("parsed deeplink => command: %s, arguments: %v", command, arguments))

	if command == "query" {
		query := arguments["q"]
		if query != "" {
			m.ui.ChangeQuery(ctx, common.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: query,
			})
			m.ui.ShowApp(ctx, common.ShowContext{})
		}
	}

	if command == "select" {
		m.QuerySelection(ctx)
	}

	if command == "toggle" {
		m.ui.ToggleApp(ctx, common.ShowContext{
			SelectAll: true,
		})
	}

	// wox://gnome-hotkey?binding=<url-encoded-binding>
	// Invoked when a GNOME custom keybinding fires and the secondary wox
	// process forwards the deeplink to the already-running instance.
	// The binding parameter is the GNOME key string (e.g. "<Primary><Shift>k"),
	// URL-decoded by ProcessDeeplink before it reaches here.
	if command == "gnome-hotkey" {
		binding := arguments["binding"]
		if binding != "" {
			keyboard.InvokeGnomeHotkeyCallback(binding)
		}
	}

	// wox://plugin/{pluginID}?arg1=val1&arg2=val2
	if strings.HasPrefix(command, "plugin/") {
		pluginID := strings.TrimPrefix(command, "plugin/")
		if pluginID != "" {
			plugin.GetPluginManager().ExecutePluginDeeplink(ctx, pluginID, arguments)
		}
	}
}

// ChangeUserDataDirectory handles changing the user data directory location
// This includes creating the new directory structure and copying necessary data
func (m *Manager) ChangeUserDataDirectory(ctx context.Context, newDirectory string) error {
	location := util.GetLocation()
	oldDirectory := location.GetUserDataDirectory()

	// check if new directory is valid
	if _, err := os.Stat(newDirectory); os.IsNotExist(err) {
		return fmt.Errorf("new directory is not a valid directory: %s", newDirectory)
	}

	// Skip if old and new directories are the same
	if oldDirectory == newDirectory {
		logger.Info(ctx, "New directory is the same as current directory, skipping")
		return nil
	}

	// Expand tilde if present in the path
	expandedDir, expandErr := homedir.Expand(newDirectory)
	if expandErr != nil {
		return fmt.Errorf("failed to expand directory path: %w", expandErr)
	}
	newDirectory = expandedDir

	logger.Info(ctx, fmt.Sprintf("Changing user data directory from %s to %s", oldDirectory, newDirectory))

	// Create the new directory if it doesn't exist
	if err := os.MkdirAll(newDirectory, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create new directory: %w", err)
	}

	// Copy only necessary directories instead of the entire user data directory
	// This prevents recursive copying issues when new directory is inside old directory
	// #4192
	if oldDirectory != "" && oldDirectory != newDirectory {
		// Define the directories we need to copy
		directoriesToCopy := []struct {
			srcPath string
			dstPath string
		}{
			{
				srcPath: filepath.Join(oldDirectory, "plugins"),
				dstPath: filepath.Join(newDirectory, "plugins"),
			},
			{
				srcPath: filepath.Join(oldDirectory, "settings"),
				dstPath: filepath.Join(newDirectory, "settings"),
			},
			{
				srcPath: filepath.Join(oldDirectory, "themes"),
				dstPath: filepath.Join(newDirectory, "themes"),
			},
		}

		// Copy each directory if it exists
		for _, dir := range directoriesToCopy {
			if _, err := os.Stat(dir.srcPath); os.IsNotExist(err) {
				logger.Info(ctx, fmt.Sprintf("Source directory %s does not exist, skipping", dir.srcPath))
				continue
			}

			logger.Info(ctx, fmt.Sprintf("Copying directory from %s to %s", dir.srcPath, dir.dstPath))
			if err := cp.Copy(dir.srcPath, dir.dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", dir.srcPath, err)
			}
		}
	}

	// Update the shortcut file
	shortcutPath := location.GetUserDataDirectoryShortcutPath()
	file, err := os.OpenFile(shortcutPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open shortcut file for writing: %w", err)
	}
	defer file.Close()

	_, writeErr := file.WriteString(newDirectory)
	if writeErr != nil {
		return fmt.Errorf("failed to write new directory path to shortcut file: %w", writeErr)
	}

	// Update the location in memory
	location.UpdateUserDataDirectory(newDirectory)

	logger.Info(ctx, "User data directory successfully changed")
	return nil
}

// applyAutoAppearanceThemeIfNeed applies the appropriate theme based on system appearance
// when the current theme has IsAutoAppearance enabled
func (m *Manager) applyAutoAppearanceThemeIfNeed(ctx context.Context) {
	currentTheme := m.GetCurrentTheme(ctx)
	if !currentTheme.IsAutoAppearance {
		return
	}

	var targetThemeId string
	if m.isSystemDark {
		targetThemeId = currentTheme.DarkThemeId
	} else {
		targetThemeId = currentTheme.LightThemeId
	}

	if targetThemeId == "" {
		logger.Warn(ctx, "auto appearance theme is enabled but target theme id is empty")
		return
	}

	if targetTheme, ok := m.themes.Load(targetThemeId); ok {
		logger.Info(ctx, fmt.Sprintf("auto apply theme: %s (isDark=%v)", targetTheme.ThemeName, m.isSystemDark))
		// Apply the current-platform effective theme without saving to settings, so
		// auto appearance keeps storing the auto theme ID while the UI receives the
		// same flattened payload as normal theme changes.
		if impl, ok := m.ui.(*uiImpl); ok {
			impl.ChangeThemeWithoutSave(ctx, m.resolvePlatformTheme(ctx, targetTheme))
		}
	} else {
		logger.Warn(ctx, fmt.Sprintf("target theme not found: %s", targetThemeId))
	}
}
