package launcher

import (
	"context"
	"fmt"
	"time"

	"wox/util"
)

// refreshQueryResultsHold waits longer than a typed-query grace period. Refresh
// re-runs the same query, and the first snapshot is often only the fastest
// plugins. Painting that snapshot collapses the window until the rest arrive.
const refreshQueryResultsHold = 400 * time.Millisecond

// refreshResultSnapshot owns the latest deferred response, including optional metadata,
// so releasing a refresh hold applies one coherent snapshot through the normal result pipeline.
type refreshResultSnapshot struct {
	results             []queryResult
	layout              *queryLayout
	refinements         *[]queryRefinement
	context             *queryContext
	queryStartTimestamp int64
}

// beginRefreshResultHoldLocked keeps the current list up until this refresh can
// replace it without shrinking the window, or until the hold expires.
func (a *App) beginRefreshResultHoldLocked() {
	a.releaseRefreshResultHoldLocked()
	// A typed-query clear still in flight would blank the list this refresh is keeping.
	a.resetQueryTransitionLocked()
	if !a.visible || len(a.results) == 0 {
		return
	}
	if a.query.QueryText == "" && len(a.query.QueryScope.Plugins) == 0 {
		return
	}
	queryID := a.query.QueryID
	a.queryRefreshHoldQueryID = queryID
	a.queryRefreshHoldTimer = time.AfterFunc(refreshQueryResultsHold, func() {
		if err := a.runOnUI("expire refresh result hold", func() {
			a.expireRefreshResultHold(queryID)
		}); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("dispatch refresh result hold: %v", err))
		}
	})
}

// holdRefreshResultsLocked buffers only the latest partial snapshot that would
// shrink the result area; group headers and grid rows use the actual layout geometry.
func (a *App) holdRefreshResultsLocked(queryID string, results []queryResult, layout *queryLayout, refinements *[]queryRefinement, context *queryContext, queryStartTimestamp int64, complete bool) bool {
	if complete || queryID == "" || a.queryRefreshHoldQueryID != queryID || a.resultsQueryID == queryID {
		return false
	}
	nextLayout := a.layout
	if layout != nil {
		nextLayout = *layout
	}
	if a.resultAreaHeightLocked(results, nextLayout) >= a.resultAreaHeightLocked(a.results, a.layout) {
		return false
	}
	a.queryRefreshSnapshot = &refreshResultSnapshot{
		results: results, layout: layout, refinements: refinements, context: context,
		queryStartTimestamp: queryStartTimestamp,
	}
	return true
}

// releaseRefreshResultHoldLocked drops the hold once a snapshot is safe to paint.
func (a *App) releaseRefreshResultHoldLocked() {
	if a.queryRefreshHoldTimer != nil {
		a.queryRefreshHoldTimer.Stop()
		a.queryRefreshHoldTimer = nil
	}
	a.queryRefreshHoldQueryID = ""
	a.queryRefreshSnapshot = nil
}

// expireRefreshResultHold publishes the newest buffered snapshot at the deadline.
// Only a refresh that has not received any snapshot falls back to the normal waiting state.
func (a *App) expireRefreshResultHold(queryID string) {
	if a.queryRefreshHoldQueryID != queryID {
		return
	}
	snapshot := a.queryRefreshSnapshot
	a.releaseRefreshResultHoldLocked()
	if a.destroyed.Load() || a.query.QueryID != queryID {
		return
	}
	if snapshot != nil {
		a.applyResults(queryID, snapshot.results, snapshot.layout, snapshot.refinements, snapshot.context, snapshot.queryStartTimestamp, false)
		return
	}
	a.showPendingQueryResults(queryID, true)
}
