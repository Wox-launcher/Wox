package main

import (
	"context"
	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
	"os"
	"wox/plugin"
	"wox/ui"
	"wox/util"
)

func startTray(ctx context.Context) {
	systray.Register(onReady, nil)
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTooltip("Wox")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	mQuit.SetIcon(icon.Data)

	for range mQuit.ClickedCh {
		ExitApp(util.NewTraceContext())
	}
}

func ExitApp(ctx context.Context) {
	util.GetLogger().Info(ctx, "start quitting")
	plugin.GetPluginManager().Stop(ctx)
	ui.GetUIManager().Stop(ctx)
	util.GetLogger().Info(ctx, "bye~")
	os.Exit(0)
}
