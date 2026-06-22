package plugin

import (
	"sort"
	"strings"
	"unicode/utf8"
	"wox/setting"
)

type QueryCompletionSource = string

const (
	QueryCompletionSourceCommand QueryCompletionSource = "command"
	QueryCompletionSourceHistory QueryCompletionSource = "history"
)

const (
	// QueryCompletionHistoryLimit bounds the history window scanned for inline hints.
	QueryCompletionHistoryLimit = 100

	queryCompletionVariableMarker       = "{wox:"
	queryCompletionGlobalHistoryMinLen  = 3
	queryCompletionPluginHistoryMinLen  = 2
	queryCompletionCommandScoreBase     = 20000
	queryCompletionHistoryScoreBase     = 10000
	queryCompletionFeedbackScoreBase    = 5000
	queryCompletionFeedbackAcceptBonus  = 100
	queryCompletionFeedbackAcceptMax    = 20
	queryCompletionRankBonusMax         = 100
	queryCompletionHistoryInputBonusMax = 20
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

// BuildQueryCompletionHintWithFeedback applies accepted history feedback to inline completion ranking.
func BuildQueryCompletionHintWithFeedback(query Query, queryPlugin *Instance, histories []setting.QueryHistory, feedbacks []setting.QueryCompletionFeedback) *QueryCompletionHint {
	return BuildQueryCompletionHintForInputPrefixWithFeedback(query, queryPlugin, histories, feedbacks, query.RawQuery)
}

// BuildQueryCompletionHintForInputPrefix uses the UI's original input as the stale-response prefix.
func BuildQueryCompletionHintForInputPrefix(query Query, queryPlugin *Instance, histories []setting.QueryHistory, inputPrefix string) *QueryCompletionHint {
	return BuildQueryCompletionHintForInputPrefixWithFeedback(query, queryPlugin, histories, nil, inputPrefix)
}

// BuildQueryCompletionHintForInputPrefixWithFeedback uses accepted history feedback without changing command priority.
func BuildQueryCompletionHintForInputPrefixWithFeedback(query Query, queryPlugin *Instance, histories []setting.QueryHistory, feedbacks []setting.QueryCompletionFeedback, inputPrefix string) *QueryCompletionHint {
	if query.Type != QueryTypeInput || query.RawQuery == "" {
		return nil
	}

	var best *QueryCompletionHint
	accept := func(candidate QueryCompletionHint) {
		if candidate.Suffix == "" ||
			!strings.HasPrefix(candidate.CompletionText, candidate.InputPrefix) ||
			strings.Contains(candidate.CompletionText, queryCompletionVariableMarker) {
			return
		}
		if best == nil || candidate.Score > best.Score {
			best = &candidate
		}
	}

	for _, candidate := range buildCommandCompletionHints(query, queryPlugin, inputPrefix) {
		accept(candidate)
	}
	for _, candidate := range buildHistoryCompletionHints(query, queryPlugin, histories, feedbacks, inputPrefix) {
		accept(candidate)
	}

	return best
}

func buildCommandCompletionHints(query Query, queryPlugin *Instance, inputPrefix string) []QueryCompletionHint {
	if queryPlugin == nil || query.TriggerKeyword == "" || query.Command != "" || query.Search == "" || strings.Contains(query.Search, " ") || strings.HasSuffix(query.RawQuery, " ") {
		return nil
	}

	type commandMatch struct {
		command MetadataCommand
		index   int
	}
	var matches []commandMatch
	for index, command := range queryPlugin.GetQueryCommands() {
		if command.Command == "" || command.Command == query.Search || !strings.HasPrefix(command.Command, query.Search) {
			continue
		}

		matches = append(matches, commandMatch{command: command, index: index})
	}
	if len(matches) != 1 {
		return nil
	}

	match := matches[0]
	completionText := query.TriggerKeyword + " " + match.command.Command + " "
	if !strings.HasPrefix(completionText, inputPrefix) {
		return nil
	}

	return []QueryCompletionHint{
		{
			InputPrefix:    inputPrefix,
			CompletionText: completionText,
			Suffix:         completionText[len(inputPrefix):],
			Source:         QueryCompletionSourceCommand,
			Score:          queryCompletionCommandScoreBase + effectiveCompletionInputLen(query.Search)*100 + rankBonus(match.index),
		},
	}
}

func buildHistoryCompletionHints(query Query, queryPlugin *Instance, histories []setting.QueryHistory, feedbacks []setting.QueryCompletionFeedback, inputPrefix string) []QueryCompletionHint {
	if len(histories) == 0 || !hasEnoughHistoryCompletionInput(query) || shouldDelayHistoryCompletionForCommandPrefix(query, queryPlugin) {
		return nil
	}

	feedbackByCompletionText := queryCompletionFeedbackByText(feedbacks)
	var hints []QueryCompletionHint
	for index, history := range latestQueryCompletionHistories(histories) {
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
			Score:          queryCompletionHistoryScoreBase + queryCompletionFeedbackBonus(completionText, feedbackByCompletionText) + historyInputBonus(query) + rankBonus(index),
		})
	}
	return hints
}

func shouldDelayHistoryCompletionForCommandPrefix(query Query, queryPlugin *Instance) bool {
	if queryPlugin == nil || query.TriggerKeyword == "" || query.Command != "" || strings.HasSuffix(query.RawQuery, " ") {
		return false
	}

	// In the command position, command metadata is more reliable than history.
	// Delay history while the input still looks like a command name so Tab does
	// not jump from a partial command straight into old command arguments.
	for _, command := range queryPlugin.GetQueryCommands() {
		if command.Command == "" {
			continue
		}
		if command.Command == query.Search || strings.HasPrefix(command.Command, query.Search) {
			return true
		}
	}
	return false
}

// hasEnoughHistoryCompletionInput keeps history hints quiet until the user gives a clear prefix.
func hasEnoughHistoryCompletionInput(query Query) bool {
	minLen := queryCompletionGlobalHistoryMinLen
	effectiveInput := query.RawQuery
	if query.TriggerKeyword != "" {
		minLen = queryCompletionPluginHistoryMinLen
		effectiveInput = query.Search
	}

	return effectiveCompletionInputLen(effectiveInput) >= minLen
}

// latestQueryCompletionHistories returns the newest bounded history window for stable hint ranking.
func latestQueryCompletionHistories(histories []setting.QueryHistory) []setting.QueryHistory {
	latest := append([]setting.QueryHistory(nil), histories...)
	sort.SliceStable(latest, func(i, j int) bool {
		return latest[i].Timestamp > latest[j].Timestamp
	})
	if len(latest) > QueryCompletionHistoryLimit {
		return latest[:QueryCompletionHistoryLimit]
	}
	return latest
}

// historyInputBonus rewards clearer prefixes without letting history outrank command hints.
func historyInputBonus(query Query) int {
	inputLen := effectiveCompletionInputLen(query.RawQuery)
	if query.TriggerKeyword != "" {
		inputLen = effectiveCompletionInputLen(query.Search)
	}
	if inputLen > queryCompletionHistoryInputBonusMax {
		inputLen = queryCompletionHistoryInputBonusMax
	}
	return inputLen * 20
}

// queryCompletionFeedbackByText keeps the latest accepted feedback for each completion text.
func queryCompletionFeedbackByText(feedbacks []setting.QueryCompletionFeedback) map[string]setting.QueryCompletionFeedback {
	result := map[string]setting.QueryCompletionFeedback{}
	for _, feedback := range feedbacks {
		if feedback.CompletionText == "" || feedback.AcceptCount <= 0 {
			continue
		}

		existing, exists := result[feedback.CompletionText]
		if !exists || feedback.LastAcceptedTimestamp > existing.LastAcceptedTimestamp {
			result[feedback.CompletionText] = feedback
		}
	}
	return result
}

// queryCompletionFeedbackBonus converts accepted hint feedback into a bounded history-only score bonus.
func queryCompletionFeedbackBonus(completionText string, feedbacks map[string]setting.QueryCompletionFeedback) int {
	feedback, exists := feedbacks[completionText]
	if !exists {
		return 0
	}

	acceptCount := feedback.AcceptCount
	if acceptCount > queryCompletionFeedbackAcceptMax {
		acceptCount = queryCompletionFeedbackAcceptMax
	}
	return queryCompletionFeedbackScoreBase + acceptCount*queryCompletionFeedbackAcceptBonus
}

func effectiveCompletionInputLen(value string) int {
	return utf8.RuneCountInString(strings.TrimSpace(value))
}

func rankBonus(index int) int {
	if index >= queryCompletionRankBonusMax {
		return 0
	}
	return queryCompletionRankBonusMax - index
}
