package launcher

import (
	"log"
	"strings"

	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// attentionPluginDisabled reports that the Attention inbox plugin is turned off.
// Tests replace this so eligibility can be checked without a plugin manager instance.
var attentionPluginDisabled = plugin.IsAttentionPluginDisabled

// buildAttentionUnread sizes the query-box accessory beside Glance.
func (a *App) buildAttentionUnread(unreadCount int, palette uiPalette, width float32, densityMetrics launcherDensityMetrics) woxwidget.Widget {
	return launcherview.AttentionUnreadBoundary(launcherview.AttentionUnreadProps{
		Width: width, Tooltip: a.attentionUnreadTooltip(), CountText: launcherview.AttentionUnreadCountText(unreadCount), UnreadCount: unreadCount,
		Theme: palette.componentTheme(), DensityScale: densityMetrics.scale,
		OnTap: func() { a.activateAttentionUnread() }, OnHover: a.setAttentionUnreadHover,
	})
}

func (a *App) attentionUnreadTooltip() string {
	hotkey := strings.Join(formatHotkeyLabels(primaryHotkey("u")), "+")
	return strings.ReplaceAll(a.translate("i18n:ui_attention_unread_tooltip"), "{hotkey}", hotkey)
}

func (a *App) attentionEligibleLocked() bool {
	if a.attentionUnreadCount <= 0 || a.show.HideQueryBox || attentionPluginDisabled() {
		return false
	}
	if a.query.QueryType != "input" || a.layout.Icon.ImageData != "" || len(a.layout.ScopeIcons) > 0 {
		return false
	}
	if len(a.query.QueryScope.Plugins) > 0 {
		return false
	}
	if a.query.QueryText == "" {
		return true
	}
	// Like Glance's retained item, keep the last state while classification is pending.
	if !a.queryContextKnown {
		return a.attentionQueryWasGlobal
	}
	return a.queryContext.IsGlobalQuery
}

func (a *App) setAttentionUnreadHover(inside bool, text string, anchor woxui.Rect) {
	a.setNativeHoverTooltip(&a.attentionTooltipRevision, "go-ui-attention", "update attention tooltip", inside, text, anchor, "top", func() *woxui.Window { return a.window })
}

// activateAttentionUnread opens the Attention inbox from the query-box badge or primary+U.
// It returns false when the badge is hidden so the hotkey is left for other handlers, like refinements.
func (a *App) activateAttentionUnread() bool {
	if !a.attentionEligibleLocked() {
		return false
	}
	a.setQuery(newInputQuery("attention "))
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send attention query: %v", err)
	}
	return true
}
