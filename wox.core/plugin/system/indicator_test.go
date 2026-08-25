package system

import (
	"fmt"
	"testing"

	"wox/plugin"
)

func TestLimitIndicatorQueryResultsKeepsHighestScoresStable(t *testing.T) {
	results := []plugin.QueryResult{{Title: "first", Score: 100}, {Title: "second", Score: 100}}
	for score := int64(31); score >= 0; score-- {
		results = append(results, plugin.QueryResult{Title: fmt.Sprintf("score-%d", score), Score: score})
	}

	limited := limitIndicatorQueryResults(results)
	if len(limited) != indicatorQueryResultLimit {
		t.Fatalf("result count = %d, want %d", len(limited), indicatorQueryResultLimit)
	}
	if limited[0].Title != "first" || limited[1].Title != "second" || limited[len(limited)-1].Score != 4 {
		t.Fatalf("limited results did not preserve the highest scores and stable ties: %#v", limited)
	}
}
