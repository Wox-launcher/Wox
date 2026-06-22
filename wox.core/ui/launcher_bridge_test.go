package ui

import (
	"context"
	"testing"
	"wox/common"
	"wox/launcher"
)

type fakeLauncherRuntime struct {
	showCalls         []common.ShowContext
	hideCallCount     int
	toggleCalls       []common.ShowContext
	changeQueries     []common.PlainQuery
	refreshSelections []bool
	changeThemes      []common.Theme
}

func (f *fakeLauncherRuntime) Start(ctx context.Context) error { return nil }
func (f *fakeLauncherRuntime) Stop(ctx context.Context) error  { return nil }
func (f *fakeLauncherRuntime) Show(ctx context.Context, showContext common.ShowContext) {
	f.showCalls = append(f.showCalls, showContext)
}
func (f *fakeLauncherRuntime) Hide(ctx context.Context) {
	f.hideCallCount++
}
func (f *fakeLauncherRuntime) Toggle(ctx context.Context, showContext common.ShowContext) {
	f.toggleCalls = append(f.toggleCalls, showContext)
}
func (f *fakeLauncherRuntime) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	f.changeQueries = append(f.changeQueries, query)
}
func (f *fakeLauncherRuntime) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	f.refreshSelections = append(f.refreshSelections, preserveSelectedIndex)
}
func (f *fakeLauncherRuntime) ChangeTheme(ctx context.Context, theme common.Theme) {
	f.changeThemes = append(f.changeThemes, theme)
}
func (f *fakeLauncherRuntime) PushResults(ctx context.Context, payload interface{}) bool {
	return true
}

type fakeUI struct {
	openSettingCalls []common.SettingWindowContext
	changeThemeCalls []common.Theme
}

func (f *fakeUI) ChangeQuery(ctx context.Context, query common.PlainQuery)      {}
func (f *fakeUI) RefreshQuery(ctx context.Context, preserveSelectedIndex bool)  {}
func (f *fakeUI) HideApp(ctx context.Context)                                   {}
func (f *fakeUI) ShowApp(ctx context.Context, showContext common.ShowContext)   {}
func (f *fakeUI) ToggleApp(ctx context.Context, showContext common.ShowContext) {}
func (f *fakeUI) RecordHotkey(ctx context.Context, hotkey string)               {}
func (f *fakeUI) OpenSettingWindow(ctx context.Context, windowContext common.SettingWindowContext) {
	f.openSettingCalls = append(f.openSettingCalls, windowContext)
}
func (f *fakeUI) OpenOnboardingWindow(ctx context.Context)                              {}
func (f *fakeUI) PickFiles(ctx context.Context, params common.PickFilesParams) []string { return nil }
func (f *fakeUI) CaptureScreenshot(ctx context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	return common.CaptureScreenshotResult{}, nil
}
func (f *fakeUI) WriteClipboardImageFile(ctx context.Context, filePath string) error { return nil }
func (f *fakeUI) GetActiveWindowSnapshot(ctx context.Context) common.ActiveWindowSnapshot {
	return common.ActiveWindowSnapshot{}
}
func (f *fakeUI) GetServerPort(ctx context.Context) int           { return 0 }
func (f *fakeUI) GetAllThemes(ctx context.Context) []common.Theme { return nil }
func (f *fakeUI) ChangeTheme(ctx context.Context, theme common.Theme) {
	f.changeThemeCalls = append(f.changeThemeCalls, theme)
}
func (f *fakeUI) InstallTheme(ctx context.Context, theme common.Theme)             {}
func (f *fakeUI) UninstallTheme(ctx context.Context, theme common.Theme)           {}
func (f *fakeUI) RestoreTheme(ctx context.Context)                                 {}
func (f *fakeUI) Notify(ctx context.Context, msg common.NotifyMsg)                 {}
func (f *fakeUI) UpdateAttentionUnreadCount(ctx context.Context, unreadCount int)  {}
func (f *fakeUI) UpdateDiagnosticStatus(ctx context.Context, enabled bool)         {}
func (f *fakeUI) ShowToolbarMsg(ctx context.Context, msg interface{})              {}
func (f *fakeUI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string)         {}
func (f *fakeUI) UpdateResult(ctx context.Context, result interface{}) bool        { return false }
func (f *fakeUI) PushResults(ctx context.Context, payload interface{}) bool        { return false }
func (f *fakeUI) IsVisible(ctx context.Context) bool                               { return false }
func (f *fakeUI) FocusToChatInput(ctx context.Context)                             {}
func (f *fakeUI) SendChatResponse(ctx context.Context, chatData common.AIChatData) {}
func (f *fakeUI) ReloadChatResources(ctx context.Context, resourceName string)     {}
func (f *fakeUI) ReloadSettingPlugins(ctx context.Context)                         {}
func (f *fakeUI) ReloadSetting(ctx context.Context)                                {}
func (f *fakeUI) ReloadSettingThemes(ctx context.Context)                          {}
func (f *fakeUI) CloudSyncProgressChanged(ctx context.Context, progress any)       {}
func (f *fakeUI) RefreshGlance(ctx context.Context, pluginId string, ids []string) {}

var _ launcher.Runtime = (*fakeLauncherRuntime)(nil)
var _ common.UI = (*fakeUI)(nil)

func TestUseLauncherRuntimeDelegatesLauncherCommands(t *testing.T) {
	t.Parallel()

	runtime := &fakeLauncherRuntime{}
	fallback := &fakeUI{}
	manager := &Manager{ui: fallback}

	manager.UseLauncherRuntime(runtime)

	query := common.PlainQuery{QueryId: "q1", QueryType: "input", QueryText: "abc"}
	showContext := common.ShowContext{HideOnBlur: true}
	toggleContext := common.ShowContext{HideToolbar: true}

	manager.GetUI(context.Background()).ChangeQuery(context.Background(), query)
	manager.GetUI(context.Background()).RefreshQuery(context.Background(), true)
	manager.GetUI(context.Background()).ShowApp(context.Background(), showContext)
	manager.GetUI(context.Background()).ToggleApp(context.Background(), toggleContext)
	manager.GetUI(context.Background()).HideApp(context.Background())

	if len(runtime.changeQueries) != 1 ||
		runtime.changeQueries[0].QueryId != query.QueryId ||
		runtime.changeQueries[0].QueryType != query.QueryType ||
		runtime.changeQueries[0].QueryText != query.QueryText {
		t.Fatal("ChangeQuery should delegate to launcher runtime")
	}

	if len(runtime.refreshSelections) != 1 || !runtime.refreshSelections[0] {
		t.Fatal("RefreshQuery should delegate to launcher runtime")
	}

	if len(runtime.showCalls) != 1 || runtime.showCalls[0] != showContext {
		t.Fatal("ShowApp should delegate to launcher runtime")
	}

	if len(runtime.toggleCalls) != 1 || runtime.toggleCalls[0] != toggleContext {
		t.Fatal("ToggleApp should delegate to launcher runtime")
	}

	if runtime.hideCallCount != 1 {
		t.Fatal("HideApp should delegate to launcher runtime")
	}
}

func TestUseLauncherRuntimeKeepsFallbackForSettingsOperations(t *testing.T) {
	t.Parallel()

	runtime := &fakeLauncherRuntime{}
	fallback := &fakeUI{}
	manager := &Manager{ui: fallback}
	manager.UseLauncherRuntime(runtime)

	windowContext := common.SettingWindowContext{Path: "/plugins"}
	theme := common.Theme{ThemeId: "theme-1"}

	manager.GetUI(context.Background()).OpenSettingWindow(context.Background(), windowContext)
	manager.GetUI(context.Background()).ChangeTheme(context.Background(), theme)

	if len(fallback.openSettingCalls) != 1 || fallback.openSettingCalls[0] != windowContext {
		t.Fatal("OpenSettingWindow should continue to use fallback UI implementation")
	}

	if len(fallback.changeThemeCalls) != 1 || fallback.changeThemeCalls[0].ThemeId != theme.ThemeId {
		t.Fatal("ChangeTheme should continue to use fallback UI implementation")
	}

	if len(runtime.changeThemes) != 1 || runtime.changeThemes[0].ThemeId != theme.ThemeId {
		t.Fatal("ChangeTheme should also update the launcher runtime theme")
	}
}
