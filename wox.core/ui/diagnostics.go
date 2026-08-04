package ui

import (
	"context"
	"time"

	"wox/diagnostic"
	"wox/setting"
	"wox/util"
)

// EnableDiagnosticsMonitorAndRestart enables bug-aware monitoring before restarting the primary instance.
func EnableDiagnosticsMonitorAndRestart(ctx context.Context) (diagnostic.State, error) {
	state, err := enableDiagnosticsMonitor(ctx)
	if err != nil {
		return diagnostic.State{}, err
	}
	if err := diagnostic.GetManager().StartSupervisorDetached(ctx, true); err != nil {
		return diagnostic.State{}, err
	}
	util.Go(ctx, "restart wox for bug aware monitor", func() {
		time.Sleep(200 * time.Millisecond)
		GetUIManager().ExitApp(util.NewTraceContext())
	})
	return state, nil
}

func enableDiagnosticsMonitor(ctx context.Context) (diagnostic.State, error) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	previousLogLevel := util.NormalizeLogLevel(woxSetting.LogLevel.Get())
	state, err := diagnostic.GetManager().Enable(ctx, previousLogLevel)
	if err != nil {
		return diagnostic.State{}, err
	}
	woxSetting.LogLevel.Set(setting.LogLevelDebug)
	util.GetLogger().SetLevel(setting.LogLevelDebug)
	GetUIManager().GetUI(ctx).UpdateDiagnosticStatus(ctx, true)
	return state, nil
}
