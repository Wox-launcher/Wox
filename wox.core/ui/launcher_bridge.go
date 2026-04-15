package ui

import (
	"context"
	"wox/common"
	"wox/launcher"
)

type launcherBridge struct {
	common.UI
	runtime launcher.Runtime
}

func (b *launcherBridge) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	b.runtime.ChangeQuery(ctx, query)
}

func (b *launcherBridge) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	b.runtime.RefreshQuery(ctx, preserveSelectedIndex)
}

func (b *launcherBridge) HideApp(ctx context.Context) {
	b.runtime.Hide(ctx)
}

func (b *launcherBridge) ShowApp(ctx context.Context, showContext common.ShowContext) {
	b.runtime.Show(ctx, showContext)
}

func (b *launcherBridge) ToggleApp(ctx context.Context, showContext common.ShowContext) {
	b.runtime.Toggle(ctx, showContext)
}

func (b *launcherBridge) ChangeTheme(ctx context.Context, theme common.Theme) {
	b.runtime.ChangeTheme(ctx, theme)
	b.UI.ChangeTheme(ctx, theme)
}

func (b *launcherBridge) PushResults(ctx context.Context, payload interface{}) bool {
	return b.runtime.PushResults(ctx, payload)
}

func (m *Manager) UseLauncherRuntime(runtime launcher.Runtime) {
	if runtime == nil {
		return
	}

	if bridge, ok := m.ui.(*launcherBridge); ok {
		bridge.runtime = runtime
		return
	}

	m.ui = &launcherBridge{
		UI:      m.ui,
		runtime: runtime,
	}
}
