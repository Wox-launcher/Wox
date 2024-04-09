package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"os"
	"path"
	"strings"
	"sync"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/share"
	"wox/util"
	"wox/util/hotkey"
	"wox/util/tray"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	mainHotkey      *hotkey.Hotkey
	selectionHotkey *hotkey.Hotkey
	queryHotkeys    []*hotkey.Hotkey
	ui              share.UI
	serverPort      int
	uiProcess       *os.Process
	themes          *util.HashMap[string, share.Theme]
	systemThemeIds  []string
}

func GetUIManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		managerInstance.mainHotkey = &hotkey.Hotkey{}
		managerInstance.selectionHotkey = &hotkey.Hotkey{}
		managerInstance.ui = &uiImpl{
			requestMap: util.NewHashMap[string, chan WebsocketMsg](),
		}
		managerInstance.themes = util.NewHashMap[string, share.Theme]()
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

			if e.Op == fsnotify.Write {
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

				//replace theme
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
				util.WatchDirectoryChanges(ctx, path.Join(workingDirectory, "resource", "ui", "themes"), onThemeChange)
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

	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	if util.IsDev() {
		logger.Info(ctx, "skip stopping ui app in dev mode")
		return
	}

	logger.Info(ctx, "start stopping ui app")
	var pid = m.uiProcess.Pid
	killErr := m.uiProcess.Kill()
	if killErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to kill ui process(%d): %s", pid, killErr))
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("killed ui process(%d)", pid))
	}
}

func (m *Manager) RegisterMainHotkey(ctx context.Context, combineKey string) error {
	return m.mainHotkey.Register(ctx, combineKey, func() {
		m.ui.ToggleApp(util.NewTraceContext())
	})
}

func (m *Manager) RegisterSelectionHotkey(ctx context.Context, combineKey string) error {
	return m.selectionHotkey.Register(ctx, combineKey, func() {
		newCtx := util.NewTraceContext()
		selection, err := util.GetSelected()
		if err != nil {
			logger.Error(newCtx, fmt.Sprintf("failed to get selected: %s", err.Error()))
			return
		}

		m.ui.ChangeQuery(newCtx, share.ChangedQuery{
			QueryType:      plugin.QueryTypeSelection,
			QuerySelection: selection,
		})
		m.ui.ShowApp(newCtx, share.ShowContext{SelectAll: false})
	})
}

func (m *Manager) RegisterQueryHotkey(ctx context.Context, queryHotkey setting.QueryHotkey) error {
	hk := &hotkey.Hotkey{}
	err := hk.Register(ctx, queryHotkey.Hotkey, func() {
		newCtx := util.NewTraceContext()
		query := plugin.GetPluginManager().ReplaceQueryVariable(newCtx, queryHotkey.Query)
		m.ui.ChangeQuery(newCtx, share.ChangedQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: query,
		})
		m.ui.ShowApp(newCtx, share.ShowContext{SelectAll: false})
	})
	if err != nil {
		return err
	}

	m.queryHotkeys = append(m.queryHotkeys, hk)
	return nil
}

func (m *Manager) StartWebsocketAndWait(ctx context.Context, port int) {
	m.serverPort = port
	serveAndWait(ctx, port)
}

func (m *Manager) StartUIApp(ctx context.Context, port int) error {
	var appPath = util.GetLocation().GetUIAppPath()
	if fileInfo, statErr := os.Stat(appPath); os.IsNotExist(statErr) {
		logger.Info(ctx, "UI app not exist")
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

	logger.Info(ctx, fmt.Sprintf("start ui, path=%s, port=%d, pid=%d", appPath, port, os.Getpid()))
	cmd, cmdErr := util.ShellRun(appPath,
		fmt.Sprintf("%d", port),
		fmt.Sprintf("%d", os.Getpid()),
		fmt.Sprintf("%t", util.IsDev()),
	)
	if cmdErr != nil {
		return cmdErr
	}

	m.uiProcess = cmd.Process
	util.GetLogger().Info(ctx, fmt.Sprintf("ui app pid: %d", cmd.Process.Pid))
	return nil
}

func (m *Manager) GetCurrentTheme(ctx context.Context) share.Theme {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if v, ok := m.themes.Load(woxSetting.ThemeId); ok {
		return v
	}

	return share.Theme{}
}

func (m *Manager) GetAllThemes(ctx context.Context) []share.Theme {
	var themes []share.Theme
	m.themes.Range(func(key string, value share.Theme) bool {
		themes = append(themes, value)
		return true
	})
	return themes
}

func (m *Manager) AddTheme(ctx context.Context, theme share.Theme) {
	m.themes.Store(theme.ThemeId, theme)
	m.ChangeTheme(ctx, theme)
}

func (m *Manager) RemoveTheme(ctx context.Context, theme share.Theme) {
	m.themes.Delete(theme.ThemeId)
	if v, ok := m.themes.Load("53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03"); ok {
		m.ChangeTheme(ctx, v)
	}
}

func (m *Manager) GetThemeById(themeId string) share.Theme {
	if v, ok := m.themes.Load(themeId); ok {
		return v
	}
	return share.Theme{}
}

func (m *Manager) parseTheme(themeJson string) (share.Theme, error) {
	var theme share.Theme
	parseErr := json.Unmarshal([]byte(themeJson), &theme)
	if parseErr != nil {
		return share.Theme{}, parseErr
	}
	return theme, nil
}

func (m *Manager) ChangeTheme(ctx context.Context, theme share.Theme) {
	m.GetUI(ctx).ChangeTheme(ctx, theme)
}

func (m *Manager) ToggleWindow() {
	ctx := util.NewTraceContext()
	logger.Info(ctx, "[UI] toggle window")
	requestUI(ctx, WebsocketMsg{
		RequestId: uuid.NewString(),
		Method:    "toggleWindow",
	})
}

func (m *Manager) GetUI(ctx context.Context) share.UI {
	return m.ui
}

func (m *Manager) PostAppStart(ctx context.Context) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !woxSetting.HideOnStart {
		m.ui.ShowApp(ctx, share.ShowContext{SelectAll: false})
	}
}

func (m *Manager) IsSystemTheme(id string) bool {
	return lo.Contains(m.systemThemeIds, id)
}

func (m *Manager) IsThemeUpgradable(id string, version string) bool {
	theme := m.GetThemeById(id)
	if theme.ThemeId != "" {
		existingVersion, existingErr := semver.NewVersion(theme.Version)
		currentVersion, currentErr := semver.NewVersion(version)
		if existingErr != nil && currentErr != nil {
			if existingVersion.GreaterThan(currentVersion) {
				return true
			}
		}
	}
	return false
}

func (m *Manager) ShowTray() {
	ctx := util.NewTraceContext()

	tray.CreateTray(resource.GetAppIcon(),
		tray.MenuItem{
			Title: "Toggle Wox",
			Callback: func() {
				m.GetUI(ctx).ToggleApp(ctx)
			},
		}, tray.MenuItem{
			Title: "Quit",
			Callback: func() {
				m.ExitApp(util.NewTraceContext())
			},
		})
}

func (m *Manager) HideTray() {
	tray.RemoveTray()
}

func (m *Manager) PostSettingUpdate(ctx context.Context, key, value string) {
	if key == "ShowTray" {
		if value == "true" {
			m.ShowTray()
		} else {
			m.HideTray()
		}
	}
}

func (m *Manager) ExitApp(ctx context.Context) {
	util.GetLogger().Info(ctx, "start quitting")
	plugin.GetPluginManager().Stop(ctx)
	m.Stop(ctx)
	util.GetLogger().Info(ctx, "bye~")
	os.Exit(0)
}
