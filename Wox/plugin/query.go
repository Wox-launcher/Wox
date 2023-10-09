package plugin

import (
	"github.com/samber/lo"
	"strings"
	"wox/util"
)

var actionsForUIMap util.HashMap[string, func()]

type WoxImage struct {
	ImageType string
	ImageData string
}

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Raw query, this includes trigger keyword if it has.
	// We didn't recommend use this property directly. You should always use Search property.
	RawQuery string

	// Trigger keyword of a query. It can be empty if user is using global trigger keyword.
	// Empty trigger keyword means this query will be a global query.
	TriggerKeyword string

	// Command part of a query.
	// Empty command means this query doesn't have a command.
	Command string

	// Search part of a query.
	// Empty search means this query doesn't have a search part.
	Search string
}

// Query result return from plugin
type QueryResult struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id                     string
	Title                  string
	SubTitle               string
	Icon                   WoxImage
	Score                  int
	PreventHideAfterAction bool // If true, Wox will not hide after user select this result
	Action                 func()
}

type QueryResultEx struct {
	QueryResult     QueryResult
	PluginInstance  *Instance
	AssociatedQuery Query
}

type QueryResultForUI struct {
	Id              string
	Title           string
	SubTitle        string
	Icon            WoxImage
	Score           int
	AssociatedQuery string
}

func NewQueryResultForUI(ex QueryResultEx) QueryResultForUI {
	actionsForUIMap.Store(ex.QueryResult.Id, ex.QueryResult.Action)

	return QueryResultForUI{
		Id:              ex.QueryResult.Id,
		Title:           ex.QueryResult.Title,
		SubTitle:        ex.QueryResult.SubTitle,
		Icon:            ex.QueryResult.Icon,
		Score:           ex.QueryResult.Score,
		AssociatedQuery: ex.AssociatedQuery.RawQuery,
	}
}

func NewQueryResultForUIs(exs []QueryResultEx) []QueryResultForUI {
	var results []QueryResultForUI
	for _, ex := range exs {
		results = append(results, NewQueryResultForUI(ex))
	}
	return results
}

func GetActionForResult(resultId string) func() {
	action, found := actionsForUIMap.Load(resultId)
	if found {
		return action
	}

	return nil
}

func NewQuery(query string) Query {
	var terms = strings.Split(query, " ")
	if len(terms) == 0 {
		return Query{
			RawQuery: query,
		}
	}

	var rawQuery = query
	var triggerKeyword, command, search string
	var possibleTriggerKeyword = terms[0]
	var mustContainSpace = strings.Contains(query, " ")

	pluginInstance, found := lo.Find(GetPluginManager().GetPluginInstances(), func(instance *Instance) bool {
		return lo.Contains(instance.GetTriggerKeywords(), possibleTriggerKeyword)
	})
	if found && mustContainSpace {
		// non global trigger keyword
		triggerKeyword = possibleTriggerKeyword

		if len(terms) == 1 {
			// no command and search
			command = ""
			search = ""
		} else {
			var possibleCommand = terms[1]
			if lo.ContainsBy(pluginInstance.Metadata.Commands, func(item MetadataCommand) bool {
				return item.Command == possibleCommand
			}) {
				// command and search
				command = possibleCommand
				search = strings.Join(terms[2:], " ")
			} else {
				// no command, only search
				command = ""
				search = strings.Join(terms[1:], " ")
			}
		}
	} else {
		// non trigger keyword
		triggerKeyword = ""
		command = ""
		search = rawQuery
	}

	return Query{
		RawQuery:       query,
		TriggerKeyword: triggerKeyword,
		Command:        command,
		Search:         search,
	}
}
