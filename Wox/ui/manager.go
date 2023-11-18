package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"os"
	"path"
	"sync"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/share"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	mainHotkey      *util.Hotkey
	selectionHotkey *util.Hotkey
	queryHotkeys    []*util.Hotkey
	ui              share.UI
	serverPort      int
	uiProcess       *os.Process
	themes          []Theme
}

func GetUIManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		managerInstance.mainHotkey = &util.Hotkey{}
		managerInstance.selectionHotkey = &util.Hotkey{}
		managerInstance.ui = &uiImpl{}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Send(ctx context.Context) error {
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
		m.ui.ToggleApp(ctx)
	})
}

func (m *Manager) RegisterSelectionHotkey(ctx context.Context, combineKey string) error {
	return m.selectionHotkey.Register(ctx, combineKey, func() {
		selection, err := util.GetSelected()
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get selected: %s", err.Error()))
			return
		}

		m.ui.ChangeQuery(ctx, share.ChangedQuery{
			QueryType:      plugin.QueryTypeSelection,
			QuerySelection: selection,
		})
		m.ui.ToggleApp(ctx)
	})
}

func (m *Manager) RegisterQueryHotkey(ctx context.Context, queryHotkey setting.QueryHotkey) error {
	hotkey := &util.Hotkey{}
	err := hotkey.Register(ctx, queryHotkey.Hotkey, func() {
		query := plugin.GetPluginManager().ReplaceQueryVariable(ctx, queryHotkey.Query)
		m.ui.ChangeQuery(ctx, share.ChangedQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: query,
		})
		m.ui.ShowApp(ctx, share.ShowContext{SelectAll: false})
	})
	if err != nil {
		return err
	}

	m.queryHotkeys = append(m.queryHotkeys, hotkey)
	return nil
}

func (m *Manager) StartWebsocketAndWait(ctx context.Context, port int) {
	m.serverPort = port
	serveAndWait(ctx, port)
}

func (m *Manager) StartUIApp(ctx context.Context, port int) error {
	//check if electron exist
	var electronExecutablePath = util.GetLocation().GetUIAppPath()
	if _, statErr := os.Stat(electronExecutablePath); os.IsNotExist(statErr) {
		logger.Info(ctx, "electron not exist, download it")
		//download electron
		isDownloaded := false
		urls := m.getElectronDownloadUrl(ctx)
		if len(urls) == 0 {
			return fmt.Errorf("failed to get electron download urls")
		}
		for _, url := range urls {
			downloadErr := util.HttpDownload(ctx, url, path.Join(util.GetLocation().GetElectronDirectory(), "electron.zip"))
			if downloadErr != nil {
				continue
			}
			isDownloaded = true
		}
		if !isDownloaded {
			return fmt.Errorf("failed to download electron")
		}

		logger.Info(ctx, "unzip electron")
		//unzip electron
		unzipErr := util.Unzip(util.GetLocation().GetElectronDirectory()+"/electron.zip", path.Join(util.GetLocation().GetElectronBinDirectory()))
		if unzipErr != nil {
			return unzipErr
		}
	}

	url := fmt.Sprintf("http://localhost:%d/index.html", port)
	if util.IsDev() {
		url = fmt.Sprintf("http://localhost:%d", 1420)
	}
	logger.Info(ctx, fmt.Sprintf("start ui app, path=%s", electronExecutablePath))
	logger.Info(ctx, fmt.Sprintf("url: %s, port=%d, pid=%d", url, port, os.Getpid()))
	logger.Info(ctx, fmt.Sprintf("main.js: %s", util.GetLocation().GetElectronMainJsPath()))
	logger.Info(ctx, fmt.Sprintf("preload.js: %s", util.GetLocation().GetElectronPreloadJsPath()))
	cmd, cmdErr := util.ShellRun(electronExecutablePath,
		util.GetLocation().GetElectronMainJsPath(),
		util.GetLocation().GetElectronPreloadJsPath(),
		fmt.Sprintf("%d", port),
		fmt.Sprintf("%d", os.Getpid()),
		url,
	)
	if cmdErr != nil {
		return cmdErr
	}

	m.uiProcess = cmd.Process
	util.GetLogger().Info(ctx, fmt.Sprintf("ui app pid: %d", cmd.Process.Pid))
	return nil
}

func (m *Manager) getElectronDownloadUrl(ctx context.Context) []string {
	if util.IsMacOS() {
		if util.IsArm64() {
			return []string{"https://registry.npmmirror.com/-/binary/electron/27.1.0/electron-v27.1.0-darwin-arm64.zip"}
		}
		if util.IsAmd64() {
			return []string{"https://registry.npmmirror.com/-/binary/electron/27.1.0/electron-v27.1.0-darwin-x64.zip"}
		}
	}
	if util.IsWindows() {
		if util.IsAmd64() {
			return []string{"https://registry.npmmirror.com/-/binary/electron/27.1.0/electron-v27.1.0-win32-x64.zip"}
		}
	}

	return []string{}
}

func (m *Manager) LoadThemes(ctx context.Context) error {
	//load embed themes
	embedThemes := resource.GetEmbedThemes(ctx)
	for _, themeJson := range embedThemes {
		theme, themeErr := m.parseTheme(themeJson)
		if themeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse theme: %s", themeErr.Error()))
			continue
		}
		m.themes = append(m.themes, theme)
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
		m.themes = append(m.themes, theme)
	}

	if util.IsDev() {
		//watch user themes folder and reload themes
		util.Go(ctx, "watch user themes", func() {
			watchErr := util.WatchDirectories(ctx, userThemesDirectory, func(e fsnotify.Event) {
				var themePath = e.Name
				if e.Op == fsnotify.Write || e.Op == fsnotify.Chmod {
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
					for i, theme := range m.themes {
						if theme.ThemeId == changedTheme.ThemeId {
							m.themes[i] = changedTheme
							logger.Info(ctx, fmt.Sprintf("replaced theme: %s", theme.ThemeName))
							m.OnThemeChange(ctx, changedTheme)
							return
						}
					}
				}
			})
			if watchErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to watch user themes: %s", watchErr.Error()))
			}
		})
	}

	return nil
}

func (m *Manager) GetCurrentTheme(ctx context.Context) Theme {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	for _, theme := range m.themes {
		if theme.ThemeId == woxSetting.ThemeId {
			return theme
		}
	}

	return Theme{}
}

func (m *Manager) parseTheme(themeJson string) (Theme, error) {
	var theme Theme
	parseErr := json.Unmarshal([]byte(themeJson), &theme)
	if parseErr != nil {
		return Theme{}, parseErr
	}
	return theme, nil
}

func (m *Manager) OnThemeChange(ctx context.Context, theme Theme) {
	themeJson, marshalErr := json.Marshal(theme)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal theme and send to ui: %s", marshalErr.Error()))
		return
	}

	m.GetUI(ctx).ChangeTheme(ctx, string(themeJson))
}

func (m *Manager) ToggleWindow() {
	ctx := util.NewTraceContext()
	logger.Info(ctx, "[UI] toggle window")
	requestUI(ctx, WebsocketMsg{
		Id:     uuid.NewString(),
		Method: "toggleWindow",
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
