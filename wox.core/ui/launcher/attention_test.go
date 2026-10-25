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
	app := bindTestApp(&App{
		querySession: queryController{
			editor: woxui.NewTextEditor(""),
			query:  newInputQuery(""),
		},
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	})
	if err := app.UpdateAttentionUnreadCount(context.Background(), 3); err != nil {
		t.Fatalf("update unread count: %v", err)
	}
	if app.attention.attentionUnreadCount != 3 {
		t.Fatalf("unread count = %d, want 3", app.attention.attentionUnreadCount)
	}
	if snapshot := app.snapshot(); snapshot.attentionUnreadCount != 3 || !snapshot.attentionVisible {
		t.Fatalf("snapshot unread = count %d visible %v, want 3/true", snapshot.attentionUnreadCount, snapshot.attentionVisible)
	}
	if err := app.UpdateAttentionUnreadCount(context.Background(), -2); err != nil {
		t.Fatalf("update negative unread count: %v", err)
	}
	if app.attention.attentionUnreadCount != 0 {
		t.Fatalf("negative unread count = %d, want 0", app.attention.attentionUnreadCount)
	}
}

func TestAttentionUnreadTooltipUsesPlatformHotkey(t *testing.T) {
	app := bindTestApp(&App{
		language: languageCatalog{
			translations: map[string]string{
				"ui_attention_unread_tooltip": "Attention items ({hotkey})",
			},
		},
	})
	want := "Attention items (" + strings.Join(formatHotkeyLabels(primaryHotkey("u")), "+") + ")"
	if got := app.attention.attentionUnreadTooltip(); got != want {
		t.Fatalf("attention tooltip = %q, want %q", got, want)
	}
}

func TestBuildAttentionUnreadExposesInboxTap(t *testing.T) {
	app := bindTestApp(&App{})
	widget := app.buildAttentionUnread(2, defaultPalette(), 30, launcherDensityMetricsFor(""))
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
	app := bindTestApp(&App{
		attention:    attentionController{attentionUnreadCount: 2},
		querySession: queryController{query: newInputQuery("")},
	})
	if !app.attention.attentionEligibleLocked() {
		t.Fatal("empty global query should show the unread badge")
	}
	app.querySession.query = newInputQuery("plugin ")
	app.querySession.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "plugin"}}}
	if app.attention.attentionEligibleLocked() {
		t.Fatal("plugin-scoped query should hide the unread badge")
	}
}

func TestAttentionEligibleLockedWaitsForGlobalQuery(t *testing.T) {
	app := bindTestApp(&App{attention: attentionController{attentionUnreadCount: 2}})
	for _, text := range []string{"chrome", "app tel", "app tele", " "} {
		app.querySession.query = newInputQuery(text)
		for _, known := range []bool{false, true} {
			app.querySession.queryContextKnown = known
			for _, global := range []bool{false, true} {
				app.querySession.queryContext = queryContext{IsGlobalQuery: global}
				want := known && global
				if got := app.attention.attentionEligibleLocked(); got != want {
					t.Fatalf("query %q eligibility = %v, want %v (known=%v, global=%v)", text, got, want, known, global)
				}
				if !want && app.attention.activateAttentionUnread() {
					t.Fatalf("hidden badge should leave the hotkey for query %q", text)
				}
			}
		}
	}
	app.querySession.query = newInputQuery("")
	if !app.attention.attentionEligibleLocked() {
		t.Fatal("clearing the query should restore the unread badge")
	}
}

func TestAttentionEligibleLockedHidesWhenPluginDisabled(t *testing.T) {
	original := attentionPluginDisabled
	attentionPluginDisabled = func() bool { return true }
	t.Cleanup(func() { attentionPluginDisabled = original })

	app := bindTestApp(&App{
		attention:    attentionController{attentionUnreadCount: 2},
		querySession: queryController{query: newInputQuery("")},
	})
	if app.attention.attentionEligibleLocked() {
		t.Fatal("disabled Attention plugin should hide the unread badge")
	}
}

// TestAttentionContinuousTyping preserves visibility across pending generations, including rapid edits.
func TestAttentionContinuousTyping(t *testing.T) {
	app := bindTestApp(&App{
		attention:    attentionController{attentionUnreadCount: 2},
		querySession: queryController{query: newInputQuery("")},
	})
	for _, text := range []string{"a", "as", "ass", "assd"} {
		app.applyQueryTextChangeLocked(text)
		if !app.attention.attentionEligibleLocked() {
			t.Fatalf("global typing hid the badge while %q was pending", text)
		}
	}
	app.querySession.queryContextKnown = true
	app.querySession.queryContext = queryContext{IsGlobalQuery: true}
	app.applyQueryTextChangeLocked("app tel")
	app.querySession.queryContextKnown = true
	app.querySession.queryContext = queryContext{PluginID: "apps"}
	if app.attention.attentionEligibleLocked() {
		t.Fatal("confirmed plugin query should hide the badge")
	}
	for _, text := range []string{"app tele", "app teleg", "app telegr"} {
		app.applyQueryTextChangeLocked(text)
		if app.attention.attentionEligibleLocked() {
			t.Fatalf("plugin typing flashed the badge while %q was pending", text)
		}
	}
	app.applyQueryTextChangeLocked("")
	if !app.attention.attentionEligibleLocked() {
		t.Fatal("clearing input should restore the badge")
	}
}

func TestActivateAttentionUnreadLeavesHotkeyWhenBadgeHidden(t *testing.T) {
	app := bindTestApp(&App{querySession: queryController{query: newInputQuery("")}})
	if app.attention.activateAttentionUnread() {
		t.Fatal("no unread count should not consume primary+U")
	}

	app.attention.attentionUnreadCount = 2
	app.querySession.query = newInputQuery("plugin ")
	app.querySession.query.QueryScope = queryScope{Plugins: []queryScopePlugin{{PluginID: "plugin"}}}
	if app.attention.activateAttentionUnread() {
		t.Fatal("plugin-scoped query should not consume primary+U")
	}

	app.querySession.query = newInputQuery("")
	app.show.HideQueryBox = true
	if app.attention.activateAttentionUnread() {
		t.Fatal("hidden query box should not consume primary+U")
	}

	original := attentionPluginDisabled
	attentionPluginDisabled = func() bool { return true }
	t.Cleanup(func() { attentionPluginDisabled = original })
	app.show.HideQueryBox = false
	if app.attention.activateAttentionUnread() {
		t.Fatal("disabled Attention plugin should not consume primary+U")
	}
}
