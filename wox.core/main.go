package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"wox/entity"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/selection"

	"golang.design/x/hotkey/mainthread"

	_ "wox/plugin/host"

	// import all hosts

	// import all system plugins
	_ "wox/plugin/system"

	_ "wox/plugin/system/app"

	_ "wox/plugin/system/calculator"

	_ "wox/plugin/system/converter"

	_ "wox/plugin/system/file"
)

func main() {
	// logger depends on location, so location must be initialized first
	locationErr := util.GetLocation().Init()
	if locationErr != nil {
		panic(locationErr)
	}

	defer util.GoRecover(context.Background(), "main panic", func(err error) {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("main panic: %s", err.Error()))
	})

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
	ui.GetUIManager().UpdateServerPort(serverPort)
	entity.SetServerPort(serverPort)

	// check if there is existing instance running
	if existingPort := getExistingInstancePort(ctx); existingPort > 0 {
		util.GetLogger().Error(ctx, fmt.Sprintf("there is existing instance running, port: %d", existingPort))

		// if args has deeplink, post it to the existing instance
		if len(os.Args) > 1 {
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "wox://") {
					_, postDeepLinkErr := util.HttpPost(ctx, fmt.Sprintf("http://localhost:%d/deeplink", existingPort), map[string]string{
						"deeplink": arg,
					})
					if postDeepLinkErr != nil {
						util.GetLogger().Error(ctx, fmt.Sprintf("failed to post deeplink to existing instance: %s", postDeepLinkErr.Error()))
					} else {
						util.GetLogger().Info(ctx, "post deeplink to existing instance successfully, bye~")
						return
					}
				}
			}
		}

		// show existing instance if no deeplink is provided
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

	util.EnsureDeepLinkProtocolHandler(ctx)

	settingErr := setting.GetSettingManager().Init(ctx)
	if settingErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize settings: %s", settingErr.Error()))
		return
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	util.UpdateHTTPProxy(ctx, woxSetting.HttpProxyUrl.Get())

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

	selection.InitSelection()

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

		if util.IsProd() {
			util.Go(ctx, "start ui", func() {
				time.Sleep(time.Millisecond * 200) // wait websocket server start
				appErr := ui.GetUIManager().StartUIApp(ctx)
				if appErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to start ui app: %s", appErr.Error()))
					return
				}
			})
		}

		ui.GetUIManager().StartWebsocketAndWait(ctx)
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
