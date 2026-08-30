package system

import (
	"fmt"
	"testing"

	"wox/plugin"
	"wox/util/fuzzymatch"
)

func TestIndicatorDescriptionMatchRejectsScatteredCharacters(t *testing.T) {
	pattern := fuzzymatch.PreparePattern("confetti")
	scattered := fuzzymatch.FuzzyMatchPrepared(fuzzymatch.PrepareText("Actions and previews for selected text or files"), pattern, false)
	if !scattered.IsMatch {
		t.Fatal("test fixture no longer exercises a fuzzy match")
	}
	if strongIndicatorDescriptionMatch(scattered, "confetti").IsMatch {
		t.Fatalf("scattered description match unexpectedly passed with score %d", scattered.Score)
	}

	strong := fuzzymatch.FuzzyMatchPrepared(fuzzymatch.PrepareText("Launch confetti effects"), pattern, false)
	if !strongIndicatorDescriptionMatch(strong, "confetti").IsMatch {
		t.Fatalf("contiguous description match unexpectedly failed with score %d", strong.Score)
	}

	exactSubtitle := "Activate the Selection plugin"
	entry := indicatorSearchEntry{
		preparedPluginName:     fuzzymatch.PrepareText("Selection"),
		preparedDescription:    fuzzymatch.PrepareText("Actions and previews for selected text or files"),
		preparedResultSubtitle: fuzzymatch.PrepareText(exactSubtitle),
	}
	matched, score := matchIndicatorPluginText(entry, fuzzymatch.PreparePattern(exactSubtitle), exactSubtitle, false)
	if !matched {
		t.Fatalf("exact result subtitle unexpectedly failed with score %d", score)
	}
	matched, score = matchIndicatorPluginText(entry, pattern, "confetti", false)
	if matched {
		t.Fatalf("scattered plugin text unexpectedly passed with score %d", score)
	}
}

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
