package launcher

import (
	"sync/atomic"

	woxui "wox/ui/runtime"
)

// attentionController owns inbox counts and their visibility during query classification.
type attentionController struct {
	attentionUnreadCount     int
	attentionQueryWasGlobal  bool
	attentionTooltipRevision atomic.Uint64
	querySession             *queryController
	resultList               *resultController
	language                 *languageCatalog
	tooltips                 *tooltipController
	hideQueryBox             func() bool
	window                   func() *woxui.Window
	setQuery                 func(plainQuery)
	sendCurrentQuery         func() error
}

// bindAttention connects inbox navigation to the query coordinator.
func (a *App) bindAttention() {
	c := &a.attention
	c.querySession = &a.querySession
	c.resultList = &a.resultList
	c.language = &a.language
	c.tooltips = &a.tooltips
	c.hideQueryBox = func() bool { return a.show.HideQueryBox }
	c.window = func() *woxui.Window { return a.window }
	c.setQuery = a.setQuery
	c.sendCurrentQuery = a.sendCurrentQuery
}
