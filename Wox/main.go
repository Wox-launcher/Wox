package main

import (
	"context"
	"fmt"
	"golang.design/x/hotkey/mainthread"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/hotkey"
)

import _ "wox/plugin/host" // import all hosts

// import all system plugins
import _ "wox/plugin/system"
import _ "wox/plugin/system/app"
import _ "wox/plugin/system/calculator"
import _ "wox/plugin/system/file"

func main() {
	// logger depends on location, so location must be initialized first
	locationErr := util.GetLocation().Init()
	if locationErr != nil {
		panic(locationErr)
	}

	ctx := util.NewTraceContext()
	util.GetLogger().Info(ctx, "------------------------------")
	util.GetLogger().Info(ctx, "Wox starting")
	util.GetLogger().Info(ctx, fmt.Sprintf("golang version: %s", strings.ReplaceAll(runtime.Version(), "go", "")))
	util.GetLogger().Info(ctx, fmt.Sprintf("wox data location: %s", util.GetLocation().GetWoxDataDirectory()))
	util.GetLogger().Info(ctx, fmt.Sprintf("user data location: %s", util.GetLocation().GetUserDataDirectory()))

	serverPort := 34987
	if util.IsProd() {
		availablePort, portErr := util.GetAvailableTcpPort(ctx)
		if portErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to get server port: %s", portErr.Error()))
			return
		}
		serverPort = availablePort
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("server port: %d", serverPort))

	// check if there is existing instance running
	if existingPort := getExistingInstancePort(ctx); existingPort > 0 {
		util.GetLogger().Error(ctx, fmt.Sprintf("there is existing instance running, port: %d", existingPort))
		_, postShowErr := util.HttpPost(ctx, fmt.Sprintf("http://localhost:%d/show", existingPort), "")
		if postShowErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to show existing instance: %s", postShowErr.Error()))
		} else {
			util.GetLogger().Info(ctx, "show existing instance successfully, bye~")
		}
		return
	} else {
		util.GetLogger().Info(ctx, "no existing instance found")
		writeErr := os.WriteFile(util.GetLocation().GetAppLockPath(), []byte(fmt.Sprintf("%d", serverPort)), 0644)
		if writeErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to write lock file: %s", writeErr.Error()))
		}
	}

	extractErr := resource.Extract(ctx)
	if extractErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to extract embed file: %s", extractErr.Error()))
		return
	}

	settingErr := setting.GetSettingManager().Init(ctx)
	if settingErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize settings: %s", settingErr.Error()))
		return
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	langErr := i18n.GetI18nManager().UpdateLang(ctx, woxSetting.LangCode)
	if langErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize lang(%s): %s", woxSetting.LangCode, langErr.Error()))
		return
	}

	themeErr := ui.GetUIManager().Start(ctx)
	if themeErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize themes: %s", themeErr.Error()))
		return
	}

	if woxSetting.ShowTray {
		ui.GetUIManager().ShowTray()
	}

	shareUI := ui.GetUIManager().GetUI(ctx)
	plugin.GetPluginManager().Start(ctx, shareUI)

	util.InitSelection()

	// hotkey must be registered in main thread
	mainthread.Init(func() {
		registerMainHotkeyErr := ui.GetUIManager().RegisterMainHotkey(ctx, woxSetting.MainHotkey.Get())
		if registerMainHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register main hotkey: %s", registerMainHotkeyErr.Error()))
		}
		registerSelectionHotkeyErr := ui.GetUIManager().RegisterSelectionHotkey(ctx, woxSetting.SelectionHotkey.Get())
		if registerSelectionHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register selection hotkey: %s", registerSelectionHotkeyErr.Error()))
		}
		for _, queryHotkey := range woxSetting.QueryHotkeys.Get() {
			registerQueryHotkeyErr := ui.GetUIManager().RegisterQueryHotkey(ctx, queryHotkey)
			if registerQueryHotkeyErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to register query hotkey: %s", registerQueryHotkeyErr.Error()))
			}
		}

		t := hotkey.Hotkey{}
		t.Register(ctx, "ctrl+ctrl", func() {
			//files := plugin.GetPluginManager().GetUI().PickFiles(ctx, share.PickFilesParams{IsDirectory: true})
			//ui.GetUIManager().GetUI(ctx).Notify(ctx, "Picked files", fmt.Sprintf("%v", files))
			//util.GetLogger().Info(ctx, window.GetActiveWindowName())
		})
		if util.IsProd() {
			util.Go(ctx, "start ui", func() {
				time.Sleep(time.Millisecond * 200) // wait websocket server start
				appErr := ui.GetUIManager().StartUIApp(ctx, serverPort)
				if appErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to start ui app: %s", appErr.Error()))
					return
				}
			})
		}

		ui.GetUIManager().StartWebsocketAndWait(ctx, serverPort)
	})
}

// retrieves the instance port from the existing instance lock file.
// It returns 0 if the lock file doesn't exist or fails to read the file.
func getExistingInstancePort(ctx context.Context) int {
	filePath := util.GetLocation().GetAppLockPath()
	if !util.IsFileExists(filePath) {
		return 0
	}

	file, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	port, err := strconv.Atoi(string(file))
	if err != nil {
		return 0
	}

	//check if the port is valid
	response, err := util.HttpGet(ctx, fmt.Sprintf("http://localhost:%d/ping", port))
	if err != nil {
		return 0
	}

	if !strings.Contains(string(response), "pong") {
		return 0
	}

	return port
}
