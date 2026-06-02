package plugin

import (
	"strings"
	"wox/setting"
)

type QueryCompletionSource = string

const (
	QueryCompletionSourceCommand QueryCompletionSource = "command"
	QueryCompletionSourceHistory QueryCompletionSource = "history"
)

// QueryCompletionHint carries the single inline completion candidate for the current query.
type QueryCompletionHint struct {
	InputPrefix    string
	CompletionText string
	Suffix         string
	Source         QueryCompletionSource
	Score          int
}

// BuildQueryCompletionHint selects the best inline completion from command metadata and query history.
func BuildQueryCompletionHint(query Query, queryPlugin *Instance, histories []setting.QueryHistory) *QueryCompletionHint {
	return BuildQueryCompletionHintForInputPrefix(query, queryPlugin, histories, query.RawQuery)
}

// BuildQueryCompletionHintForInputPrefix uses the UI's original input as the stale-response prefix.
func BuildQueryCompletionHintForInputPrefix(query Query, queryPlugin *Instance, histories []setting.QueryHistory, inputPrefix string) *QueryCompletionHint {
	if query.Type != QueryTypeInput || query.RawQuery == "" {
		return nil
	}

	var best *QueryCompletionHint
	accept := func(candidate QueryCompletionHint) {
		if candidate.Suffix == "" || !strings.HasPrefix(candidate.CompletionText, candidate.InputPrefix) {
			return
		}
		if best == nil || candidate.Score > best.Score {
			best = &candidate
		}
	}

	for _, candidate := range buildCommandCompletionHints(query, queryPlugin, inputPrefix) {
		accept(candidate)
	}
	for _, candidate := range buildHistoryCompletionHints(query, queryPlugin, histories, inputPrefix) {
		accept(candidate)
	}

	return best
}

func buildCommandCompletionHints(query Query, queryPlugin *Instance, inputPrefix string) []QueryCompletionHint {
	if queryPlugin == nil || query.TriggerKeyword == "" || query.Command != "" || query.Search == "" || strings.Contains(query.Search, " ") || strings.HasSuffix(query.RawQuery, " ") {
		return nil
	}

	var hints []QueryCompletionHint
	for index, command := range queryPlugin.GetQueryCommands() {
		if command.Command == "" || command.Command == query.Search || !strings.HasPrefix(command.Command, query.Search) {
			continue
		}

		completionText := query.TriggerKeyword + " " + command.Command + " "
		if !strings.HasPrefix(completionText, inputPrefix) {
			continue
		}

		hints = append(hints, QueryCompletionHint{
			InputPrefix:    inputPrefix,
			CompletionText: completionText,
			Suffix:         completionText[len(inputPrefix):],
			Source:         QueryCompletionSourceCommand,
			Score:          10000 + len(query.Search)*100 - index,
		})
	}
	return hints
}

func buildHistoryCompletionHints(query Query, queryPlugin *Instance, histories []setting.QueryHistory, inputPrefix string) []QueryCompletionHint {
	if len(histories) == 0 || shouldDelayHistoryCompletionForCommandPrefix(query, queryPlugin) {
		return nil
	}

	minTimestamp, maxTimestamp := historyTimestampRange(histories)
	var hints []QueryCompletionHint
	for index, history := range histories {
		if history.Query.QueryType != QueryTypeInput {
			continue
		}

		completionText := history.Query.QueryText
		if completionText == "" || completionText == inputPrefix || !strings.HasPrefix(completionText, inputPrefix) {
			continue
		}

		hints = append(hints, QueryCompletionHint{
			InputPrefix:    inputPrefix,
			CompletionText: completionText,
			Suffix:         completionText[len(inputPrefix):],
			Source:         QueryCompletionSourceHistory,
			Score:          9000 + len(inputPrefix)*200 + historyRecencyScore(history.Timestamp, minTimestamp, maxTimestamp) + index,
		})
	}
	return hints
}

func shouldDelayHistoryCompletionForCommandPrefix(query Query, queryPlugin *Instance) bool {
	return queryPlugin != nil && query.TriggerKeyword != "" && query.Command == "" && !strings.HasSuffix(query.RawQuery, " ") && len(query.Search) < 3
}

func historyTimestampRange(histories []setting.QueryHistory) (int64, int64) {
	minTimestamp := int64(0)
	maxTimestamp := int64(0)
	for _, history := range histories {
		if history.Timestamp <= 0 {
			continue
		}
		if minTimestamp == 0 || history.Timestamp < minTimestamp {
			minTimestamp = history.Timestamp
		}
		if history.Timestamp > maxTimestamp {
			maxTimestamp = history.Timestamp
		}
	}
	return minTimestamp, maxTimestamp
}

func historyRecencyScore(timestamp int64, minTimestamp int64, maxTimestamp int64) int {
	if timestamp <= 0 || minTimestamp <= 0 || maxTimestamp <= minTimestamp {
		return 0
	}
	return int((timestamp - minTimestamp) * 100 / (maxTimestamp - minTimestamp))
}
