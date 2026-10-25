package launcher

import (
	"context"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type queryCompletionServices interface {
	AcceptQueryCompletionHint(context.Context, string, string, string, string) error
}

// queryController owns editing, semantic hints, history, and query-generation timers.
// Results are a separate owner; preview and action handoffs are coordinator commands.
type queryController struct {
	services                      queryCompletionServices
	sessionID                     string
	lifecycleCtx                  context.Context
	resultList                    *resultController
	runOnUI                       func(string, func()) error
	window                        func() *woxui.Window
	host                          func() *woxwidget.Host
	visible                       func() bool
	startPage                     func() string
	isDestroyed                   func() bool
	completionEnabled             func() bool
	queryHintChangedWithSelection func(*queryResultSelection)
	applyQueryTextChangeLocked    func(string)
	reconcileSelectedPreview      func()
	sendCurrentQuery              func() error
	resultAreaHeightLocked        func([]queryResult, queryLayout) int
	applyResults                  func(string, []queryResult, *queryLayout, *[]queryRefinement, *queryContext, int64, bool)
	queryHintEditorState          queryHintEditor
	queryTabFeedback              uint64
	query                         plainQuery
	queryContext                  queryContext
	queryContextKnown             bool
	queryHistories                []plainQuery
	queryHistoryIndex             int
	canRecallHistory              bool
	editor                        *woxui.TextEditor
	// selectionAnchor holds the rune offset captured at query drag-selection start so extend updates only the focus.
	selectionAnchor int
	// queryMenu* retains the open query context-menu enablement snapshot for refresh/revalidation.
	queryMenuAnchor         woxui.Point
	queryMenuEnablement     queryMenuEnablement
	queryMenuTheme          woxcomponent.Theme
	queryComplete           bool
	queryTransitionTimer    *time.Timer
	queryRefreshHoldQueryID string
	queryRefreshHoldTimer   *time.Timer
	queryRefreshSnapshot    *refreshResultSnapshot
	queryLoading            bool
	queryLoadingTimer       *time.Timer
	completionHint          *queryCompletionHint
	// queryFocusNotifiedInActiveWindow prevents the initial native activation
	// from duplicating the query-focus notification already sent while laying out.
	queryFocusNotifiedInActiveWindow bool
}

// bindQuery connects generation ownership to result publication and UI effects.
func (a *App) bindQuery() {
	c := &a.querySession
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.resultList = &a.resultList
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.host = func() *woxwidget.Host { return a.host }
	c.visible = func() bool { return a.visible }
	c.startPage = func() string { return a.show.StartPage }
	c.isDestroyed = a.isDestroyed
	c.completionEnabled = func() bool { return a.generalSettings.Data().EnableQueryCompletionHint }
	c.queryHintChangedWithSelection = a.queryHintChangedWithSelection
	c.applyQueryTextChangeLocked = a.applyQueryTextChangeLocked
	c.reconcileSelectedPreview = a.previews.reconcileSelectedPreview
	c.sendCurrentQuery = a.sendCurrentQuery
	c.resultAreaHeightLocked = a.resultAreaHeightLocked
	c.applyResults = a.applyResults
}

// previousQueryHistory advances through the show-time history snapshot while history recall remains active.
func (a *queryController) previousQueryHistory() (*plainQuery, bool) {
	if !a.canRecallHistory {
		return nil, false
	}
	if a.queryHistoryIndex >= len(a.queryHistories)-1 {
		return nil, true
	}
	a.queryHistoryIndex++
	query := a.queryHistories[a.queryHistoryIndex]
	return &query, true
}
