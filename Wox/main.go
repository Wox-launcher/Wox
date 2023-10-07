package main

import (
	"fmt"
	"golang.design/x/hotkey/mainthread"
	"runtime"
	"strings"
	"wox/plugin"
	"wox/resource"
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

		plugin.GetPluginManager().Start(ctx)

		registerMainHotkeyErr := mainHotkey.Register(ctx, "command+space", toggleWindow)
		if registerMainHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register main hotkey: %s", registerMainHotkeyErr.Error()))
		}

		ServeAndWait(ctx, 34987)
	})
}
