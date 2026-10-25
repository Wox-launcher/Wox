package launcher

import (
	"sync/atomic"

	woxui "wox/ui/runtime"
)

// resultController owns the published result collection and its selection and scroll state.
// Frame snapshots borrow these slices only until the current UI build completes.
type resultController struct {
	refinementTooltipRevision  atomic.Uint64
	results                    []queryResult
	resultRevision             uint64
	resultsSectionRevision     uint64
	resultsQueryID             string
	resultTailTooltipRevision  atomic.Uint64
	selected                   int
	hoveredResult              int
	pendingSelection           *pendingResultSelection
	resultScroll               scrollController
	resultScrollDetached       bool
	layout                     queryLayout
	refinements                []queryRefinement
	refinementsSectionRevision uint64
	refinementOpen             bool
	refinementScope            string
	// retainedResultIcons is the last decoded bitmap for each result row.
	// A replacement icon keeps that bitmap until its own decode finishes.
	retainedResultIcons        map[string]*woxui.Image
	retainedResultIconRevision uint64
	retainedResultIconsPruned  bool
}

func (a *resultController) resultNavigationColumns() int {
	layout := a.layout.GridLayout
	if layout == nil {
		return 1
	}
	return normalizedGridLayout(layout).Columns
}

func (a *resultController) hoverResult(index int, inside bool) {
	if inside {
		if index >= 0 && index < len(a.results) && !a.results[index].IsGroup {
			a.hoveredResult = index
		}
	} else if a.hoveredResult == index {
		a.hoveredResult = -1
	}
}

// publishRows assigns stable invalidation generations before exposing a result batch.
func (a *resultController) publishRows(queryID string, results []queryResult) {
	for index := range results {
		a.resultRevision++
		results[index].Revision = a.resultRevision
	}
	a.results = results
	a.resultsSectionRevision++
	a.resultsQueryID = queryID
	a.hoveredResult = -1
}

// clearRows drops every published row together with the selection, scroll position and
// layout that described them.
func (a *resultController) clearRows() {
	a.results = nil
	a.resultsSectionRevision++
	a.resultsQueryID = ""
	a.selected = -1
	a.hoveredResult = -1
	a.resultScroll.reset()
	a.resultScrollDetached = false
	a.layout = queryLayout{}
}

func (a *resultController) clearRefinements() {
	a.refinements = nil
	a.refinementsSectionRevision++
	a.refinementOpen = false
	a.refinementScope = ""
}

// restoreSelection retains a requested row across partial refresh snapshots.
func (a *resultController) restoreSelection(queryID string, complete bool) {
	selected, preserved, pending := restoreRefreshSelection(a.results, a.pendingSelection, queryID, complete)
	a.selected = selected
	if !pending {
		a.pendingSelection = nil
	}
	if !preserved {
		a.resultScrollDetached = false
	}
}
