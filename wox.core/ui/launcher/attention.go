package launcher

import (
	"log"
	"strings"

	"wox/common/icons"
	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// attentionPluginDisabled reports that the Attention inbox plugin is turned off.
// Tests replace this so eligibility can be checked without a plugin manager instance.
var attentionPluginDisabled = plugin.IsAttentionPluginDisabled

// buildAttentionUnread tints the inbox glyph and sizes the query-box accessory beside Glance.
func (a *App) buildAttentionUnread(unreadCount int, palette uiPalette, width, imageScale float32, densityMetrics launcherDensityMetrics) woxwidget.Widget {
	theme := palette.componentTheme()
	iconTint, _, _, _ := theme.AttentionBadgeColors(false)
	icon := a.imageForTint(fromCoreImage(icons.Get(icons.ControlInbox)), &iconTint, physicalImageSize(int(densityMetrics.scaled(15)), imageScale))
	return launcherview.AttentionUnreadBoundary(launcherview.AttentionUnreadProps{
		Width: width, Icon: icon, Tooltip: a.attentionUnreadTooltip(), CountText: launcherview.AttentionUnreadCountText(unreadCount), UnreadCount: unreadCount,
		Theme: theme, DensityScale: densityMetrics.scale,
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
	if a.query.QueryType != "" && a.query.QueryType != "input" {
		return false
	}
	if len(a.query.QueryScope.Plugins) > 0 {
		return false
	}
	if a.query.QueryText == "" || !a.queryContextKnown {
		// Keep the badge up while a new query is in flight. queryContext is cleared
		// on every keystroke, and hiding here remounts the badge on each result.
		return true
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
