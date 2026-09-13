package launcher

import (
	"context"
	"strings"
	"testing"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestUpdateAttentionUnreadCountStoresCount(t *testing.T) {
	app := &App{
		editor:          woxui.NewTextEditor(""),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	}
	if err := app.UpdateAttentionUnreadCount(context.Background(), 3); err != nil {
		t.Fatalf("update unread count: %v", err)
	}
	if app.attentionUnreadCount != 3 {
		t.Fatalf("unread count = %d, want 3", app.attentionUnreadCount)
	}
	if snapshot := app.snapshot(); snapshot.attentionUnreadCount != 3 || !snapshot.attentionVisible {
		t.Fatalf("snapshot unread = count %d visible %v, want 3/true", snapshot.attentionUnreadCount, snapshot.attentionVisible)
	}
	if err := app.UpdateAttentionUnreadCount(context.Background(), -2); err != nil {
		t.Fatalf("update negative unread count: %v", err)
	}
	if app.attentionUnreadCount != 0 {
		t.Fatalf("negative unread count = %d, want 0", app.attentionUnreadCount)
	}
}

func TestAttentionUnreadTooltipUsesPlatformHotkey(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_attention_unread_tooltip": "Attention items ({hotkey})",
	}}
	want := "Attention items (" + strings.Join(formatHotkeyLabels(primaryHotkey("u")), "+") + ")"
	if got := app.attentionUnreadTooltip(); got != want {
		t.Fatalf("attention tooltip = %q, want %q", got, want)
	}
}

func TestBuildAttentionUnreadExposesInboxTap(t *testing.T) {
	app := &App{
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	widget := app.buildAttentionUnread(2, defaultPalette(), 30, 1, launcherDensityMetricsFor(""))
	boundary, ok := widget.(woxwidget.Boundary[launcherview.AttentionUnreadProps])
	if !ok {
		t.Fatalf("attention widget = %T, want boundary", widget)
	}
	if boundary.Key != launcherview.AttentionUnreadBoundaryKey {
		t.Fatalf("attention boundary key = %q, want %q", boundary.Key, launcherview.AttentionUnreadBoundaryKey)
	}
	if boundary.Props.OnTap == nil || boundary.Props.OnHover == nil {
		t.Fatal("attention unread should expose tap and hover handlers")
	}
	if boundary.Props.UnreadCount != 2 || boundary.Props.Width != 30 || boundary.Props.CountText != "2" || boundary.Props.Tooltip == "" {
		t.Fatalf("attention unread props = %#v", boundary.Props)
	}
}

func TestAttentionEligibleLockedRequiresGlobalQuery(t *testing.T) {
	app := &App{attentionUnreadCount: 2, query: newInputQuery("")}
	if !app.attentionEligibleLocked() {
		t.Fatal("empty global query should show the unread badge")
	}
	app.query = newInputQuery("plugin ")
	app.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "plugin"}}}
	if app.attentionEligibleLocked() {
		t.Fatal("plugin-scoped query should hide the unread badge")
	}
}

func TestAttentionEligibleLockedKeepsBadgeWhileQueryContextPending(t *testing.T) {
	app := &App{attentionUnreadCount: 2, query: newInputQuery("chrome")}
	if !app.attentionEligibleLocked() {
		t.Fatal("pending query classification should keep the unread badge")
	}
	app.queryContextKnown = true
	app.queryContext = queryContext{IsGlobalQuery: false, PluginID: "notes"}
	if app.attentionEligibleLocked() {
		t.Fatal("known plugin query should hide the unread badge")
	}
	app.queryContext = queryContext{IsGlobalQuery: true}
	if !app.attentionEligibleLocked() {
		t.Fatal("known global query should show the unread badge")
	}
}

func TestAttentionEligibleLockedHidesWhenPluginDisabled(t *testing.T) {
	original := attentionPluginDisabled
	attentionPluginDisabled = func() bool { return true }
	t.Cleanup(func() { attentionPluginDisabled = original })

	app := &App{attentionUnreadCount: 2, query: newInputQuery("")}
	if app.attentionEligibleLocked() {
		t.Fatal("disabled Attention plugin should hide the unread badge")
	}
}

func TestActivateAttentionUnreadLeavesHotkeyWhenBadgeHidden(t *testing.T) {
	app := &App{query: newInputQuery("")}
	if app.activateAttentionUnread() {
		t.Fatal("no unread count should not consume primary+U")
	}

	app.attentionUnreadCount = 2
	app.query = newInputQuery("plugin ")
	app.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "plugin"}}}
	if app.activateAttentionUnread() {
		t.Fatal("plugin-scoped query should not consume primary+U")
	}

	app.query = newInputQuery("")
	app.show.HideQueryBox = true
	if app.activateAttentionUnread() {
		t.Fatal("hidden query box should not consume primary+U")
	}

	original := attentionPluginDisabled
	attentionPluginDisabled = func() bool { return true }
	t.Cleanup(func() { attentionPluginDisabled = original })
	app.show.HideQueryBox = false
	if app.activateAttentionUnread() {
		t.Fatal("disabled Attention plugin should not consume primary+U")
	}
}
