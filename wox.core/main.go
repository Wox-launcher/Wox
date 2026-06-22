package main

import (
	"os"
	"wox/app"
	"wox/diagnostic"
	"wox/util"
	"wox/util/mainthread"

	_ "wox/plugin/host"
	_ "wox/plugin/system"
	_ "wox/plugin/system/app"
	_ "wox/plugin/system/browser_bookmark"
	_ "wox/plugin/system/calculator"
	_ "wox/plugin/system/clipboard"
	_ "wox/plugin/system/converter"
	_ "wox/plugin/system/emoji"
	_ "wox/plugin/system/explorer"
	_ "wox/plugin/system/file_search"
	_ "wox/plugin/system/glance"
	_ "wox/plugin/system/mediaplayer"
	_ "wox/plugin/system/shell"
	_ "wox/plugin/system/sys"
	_ "wox/plugin/system/window_manager"
)

func main() {
	if diagnostic.GetManager().IsSupervisorArg(os.Args) {
		ctx := util.NewTraceContext()
		if locationErr := util.GetLocation().Init(); locationErr != nil {
			os.Exit(1)
		}
		os.Exit(diagnostic.GetManager().RunSupervisor(ctx, os.Args))
	}
	mainthread.Init(app.Run)
}
