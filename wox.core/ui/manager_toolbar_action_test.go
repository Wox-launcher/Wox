package ui

import (
	"context"
	"testing"

	"wox/common"
	corehotkey "wox/hotkey"
	"wox/i18n"
	"wox/ui/contract"
	"wox/util"
)

type toolbarActionTestUI struct {
	common.UI
	message  common.SystemToolbarMsg
	notice   common.ToastMsg
	settings common.SettingWindowContext
}

func (u *toolbarActionTestUI) ShowSystemToolbarMsg(_ context.Context, message common.SystemToolbarMsg) {
	u.message = message
}
func (u *toolbarActionTestUI) ShowToast(_ context.Context, message common.ToastMsg) {
	u.notice = message
}
func (u *toolbarActionTestUI) OpenSettingWindow(_ context.Context, target common.SettingWindowContext) {
	u.settings = target
}

type toolbarWarningTestView struct {
	contract.View
	id string
}

func (v *toolbarWarningTestView) SessionID() string { return v.id }

type toolbarWarningSessionsUI struct {
	common.UI
	shown   map[string]bool
	cleared map[string]bool
}

func (u *toolbarWarningSessionsUI) ShowSystemToolbarMsg(ctx context.Context, _ common.SystemToolbarMsg) {
	u.shown[util.GetContextSessionId(ctx)] = true
}

func (u *toolbarWarningSessionsUI) ClearSystemToolbarMsg(ctx context.Context, _ string) {
	u.cleared[util.GetContextSessionId(ctx)] = true
}

func TestMainHotkeyWarningSynchronizesAllSessions(t *testing.T) {
	ui := &toolbarWarningSessionsUI{shown: map[string]bool{}, cleared: map[string]bool{}}
	primary := &toolbarWarningTestView{id: "primary"}
	secondary := &toolbarWarningTestView{id: "secondary"}
	manager := &Manager{
		ui: ui, hotkeyService: corehotkey.NewService(corehotkey.Callbacks{}), primaryView: primary,
		views: map[string]contract.View{"primary": primary, "secondary": secondary},
	}
	manager.syncMainHotkeyToolbarWarning(t.Context(), "alt+space", nil)
	manager.syncMainHotkeyToolbarWarning(t.Context(), "", nil)
	for _, id := range []string{"primary", "secondary"} {
		if !ui.shown[id] || !ui.cleared[id] {
			t.Errorf("session %q: shown=%v cleared=%v", id, ui.shown[id], ui.cleared[id])
		}
	}
}

func TestMainHotkeyWarningOffersSettingsAction(t *testing.T) {
	ui := &toolbarActionTestUI{}
	manager := &Manager{ui: ui, mainHotkeyWarning: "alt+space"}
	manager.showMainHotkeyToolbarWarning(context.Background())
	if ui.message.Priority != common.SystemToolbarPriorityWarning || len(ui.message.Actions) != 1 {
		t.Fatalf("warning = %+v", ui.message)
	}
	action := ui.message.Actions[0]
	if action.SettingsPath != "/hotkeys" || action.SettingsParam != "MainHotkey" || action.Icon.IsEmpty() {
		t.Fatalf("warning action = %+v", action)
	}
}

func TestResultHotkeyWarningOpensBindings(t *testing.T) {
	ui := &toolbarActionTestUI{}
	manager := &Manager{ui: ui}
	manager.notifyResultHotkeyRegistrationFailure(context.Background())
	if ui.notice.Action == nil || ui.notice.Action.Action == nil {
		t.Fatal("notification lacks recovery action")
	}
	ui.notice.Action.Action(context.Background())
	if ui.settings.Path != "/hotkeys" || ui.settings.Param != "ResultBindings" {
		t.Fatalf("settings target = %+v", ui.settings)
	}
}

// TestPendingUpdateNoticeIsDeliveredOnce preserves the startup update confirmation without a toast.
func TestPendingUpdateNoticeIsDeliveredOnce(t *testing.T) {
	ui := &toolbarActionTestUI{}
	manager := &Manager{ui: ui}
	manager.SetStartupConfetti()
	if !manager.showPendingUpdateNotice(t.Context()) {
		t.Fatal("missing update notice")
	}
	want := i18n.GetI18nManager().TranslateWox(t.Context(), "ui_update_success")
	if ui.message.Title != want || ui.message.Id != "wox-update-success" || ui.message.Priority != common.SystemToolbarPriorityConfirmation || ui.message.DisplaySeconds != 5 || ui.message.Icon.IsEmpty() {
		t.Fatalf("update notice = %+v", ui.message)
	}
	if manager.showPendingUpdateNotice(t.Context()) {
		t.Fatal("update notice was delivered twice")
	}
	if ui.notice.Text != "" {
		t.Fatalf("update notice unexpectedly used toast: %+v", ui.notice)
	}
}

// TestSecondaryShowDoesNotConsumeUpdateNotice keeps the confirmation for the primary launcher.
func TestSecondaryShowDoesNotConsumeUpdateNotice(t *testing.T) {
	impl := &uiImpl{primarySessionID: "primary"}
	manager := &Manager{ui: impl, pendingStartupConfetti: true}
	manager.PostOnShow(util.WithSessionContext(context.Background(), "secondary"))
	if !manager.pendingStartupConfetti {
		t.Fatal("secondary launcher consumed the update notice")
	}
	if impl.isVisible {
		t.Fatal("secondary launcher was treated as primary")
	}
}
