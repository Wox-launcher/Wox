package main

import (
	"fmt"
	"golang.design/x/hotkey/mainthread"
	"runtime"
	"strings"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/share"
	"wox/ui"
	"wox/util"
)

import _ "wox/plugin/host"   // import all hosts
import _ "wox/plugin/system" // import all system plugins

func main() {
	mainthread.Init(func() {
		// logger depends on location, so location must be initialized first
		locationErr := util.GetLocation().Init()
		if locationErr != nil {
			panic(locationErr)
		}

		ctx := util.NewTraceContext()
		util.GetLogger().Info(ctx, "------------------------------")
		util.GetLogger().Info(ctx, "Wox starting")
		util.GetLogger().Info(ctx, fmt.Sprintf("golang version: %s", strings.ReplaceAll(runtime.Version(), "go", "")))

		share.ExitApp = ExitApp

		extractErr := resource.Extract(ctx)
		if extractErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to extract embed file: %s", extractErr.Error()))
			return
		}

		clipboardErr := util.ClipboardInit()
		if clipboardErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize clipboard: %s", clipboardErr.Error()))
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

		serverPort := 34987
		if util.IsProd() {
			availablePort, portErr := util.GetAvailableTcpPort(ctx)
			if portErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize lang(%s): %s", woxSetting.LangCode, langErr.Error()))
				return
			}
			serverPort = availablePort
		}

		if woxSetting.ShowTray {
			startTray(ctx)
		}

		//demo
		woxSetting.QueryHotkeys = []setting.QueryHotkey{
			{
				Hotkey: "command+shift+v",
				Query:  "cb ",
			},
		}

		shareUI := ui.GetUIManager().GetUI(ctx)
		plugin.GetPluginManager().Start(ctx, shareUI)

		registerMainHotkeyErr := ui.GetUIManager().RegisterMainHotkey(ctx, woxSetting.MainHotkey)
		if registerMainHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register main hotkey: %s", registerMainHotkeyErr.Error()))
		}
		for _, queryHotkey := range woxSetting.QueryHotkeys {
			registerQueryHotkeyErr := ui.GetUIManager().RegisterQueryHotkey(ctx, queryHotkey)
			if registerQueryHotkeyErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to register query hotkey: %s", registerQueryHotkeyErr.Error()))
			}
		}

		t := util.Hotkey{}
		t.Register(ctx, "command+m", func() {
			data, selectedErr := ui.GetSelectedText()
			if selectedErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to get selected text: %s", selectedErr.Error()))
			} else {
				util.GetLogger().Info(ctx, fmt.Sprintf("selected text: %s", data.Data))
			}
		})

		if util.IsProd() {
			appErr := ui.GetUIManager().StartUIApp(ctx, serverPort)
			if appErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to start ui app: %s", appErr.Error()))
				return
			}
		}

		ui.GetUIManager().StartWebsocketAndWait(ctx, serverPort)
	})
}
