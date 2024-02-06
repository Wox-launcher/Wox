package main

import (
	"context"
	"github.com/getlantern/systray"
	"os"
	"wox/plugin"
	"wox/ui"
	"wox/util"
)

func startTray(ctx context.Context) {
	systray.Register(onReady, nil)
}

func onReady() {
	ctx := util.NewTraceContext()
	systray.SetIcon(IconData)
	systray.SetTooltip("Wox")
	mToggle := systray.AddMenuItem("Toggle Wox", "")
	mQuit := systray.AddMenuItem("Quit", "")

	for {
		select {
		case <-mToggle.ClickedCh:
			ui.GetUIManager().GetUI(ctx).ToggleApp(ctx)
		case <-mQuit.ClickedCh:
			ExitApp(util.NewTraceContext())
		}
	}
}

func ExitApp(ctx context.Context) {
	util.GetLogger().Info(ctx, "start quitting")
	plugin.GetPluginManager().Stop(ctx)
	ui.GetUIManager().Stop(ctx)
	util.GetLogger().Info(ctx, "bye~")
	os.Exit(0)
}
