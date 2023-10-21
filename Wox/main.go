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

		extractErr := resource.ExtractHosts(ctx)
		if extractErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to extract embed host file: %s", extractErr.Error()))
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

		if woxSetting.ShowTray {
			startTray(ctx)
		}

		shareUI := ui.GetUIManager().GetUI(ctx)
		plugin.GetPluginManager().Start(ctx, shareUI)

		registerMainHotkeyErr := ui.GetUIManager().RegisterMainHotkey(ctx, woxSetting.MainHotkey)
		if registerMainHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register main hotkey: %s", registerMainHotkeyErr.Error()))
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

		ui.GetUIManager().StartWebsocketAndWait(ctx, 34987)
	})
}
