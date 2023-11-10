package plugin

import (
	"github.com/samber/lo"
	"strings"
	"wox/util"
)

type QueryType = string
type QueryVariable = string

const (
	QueryTypeText QueryType = "text"
	QueryTypeFile QueryType = "file"
)

const (
	QueryVariableSelectedText QueryVariable = "{wox:selected_text}"
)

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Query type, can be "text" or "file"
	// if query type is "file", search property will be a absolute file path(split by , if have multiple files).
	// note: you need to add features "queryFile" in plugin.json to support query type of file
	Type QueryType

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
	Id string
	// Title support i18n
	Title string
	// SubTitle support i18n
	SubTitle string
	Icon     WoxImage
	Preview  WoxPreview
	Score    int
	// Additional data associate with this result, can be retrieved in Action function
	ContextData string
	Actions     []QueryResultAction
	// refresh result after specified interval, in milliseconds. If this value is 0, Wox will not refresh this result
	// interval can only divisible by 100, if not, Wox will use the nearest number which is divisible by 100
	// E.g. if you set 123, Wox will use 200, if you set 1234, Wox will use 1300
	RefreshInterval int
	// refresh result by calling OnRefresh function
	OnRefresh func(current RefreshableResult) RefreshableResult
}

type QueryResultAction struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id string
	// Name support i18n
	Name string
	Icon WoxImage
	// If true, Wox will use this action as default action. There can be only one default action in results
	// This can be omitted, if you don't set it, Wox will use the first action as default action
	IsDefault bool
	// If true, Wox will not hide after user select this result
	PreventHideAfterAction bool
	Action                 func(actionContext ActionContext)
}

type ActionContext struct {
	// Additional data associate with this result
	ContextData string
}

func (q *QueryResult) ToUI(associatedQuery string) QueryResultUI {
	return QueryResultUI{
		Id:              q.Id,
		Title:           q.Title,
		SubTitle:        q.SubTitle,
		Icon:            q.Icon,
		Preview:         q.Preview,
		Score:           q.Score,
		AssociatedQuery: associatedQuery,
		ContextData:     q.ContextData,
		Actions: lo.Map(q.Actions, func(action QueryResultAction, index int) QueryResultActionUI {
			return QueryResultActionUI{
				Id:                     action.Id,
				Name:                   action.Name,
				Icon:                   action.Icon,
				IsDefault:              action.IsDefault,
				PreventHideAfterAction: action.PreventHideAfterAction,
			}
		}),
		RefreshInterval: q.RefreshInterval,
	}
}

type QueryResultUI struct {
	Id              string
	Title           string
	SubTitle        string
	Icon            WoxImage
	Preview         WoxPreview
	Score           int
	ContextData     string
	AssociatedQuery string
	Actions         []QueryResultActionUI
	RefreshInterval int
}

type QueryResultActionUI struct {
	Id                     string
	Name                   string
	Icon                   WoxImage
	IsDefault              bool
	PreventHideAfterAction bool
}

// store latest result value after query/refresh, so we can retrieve data later in action/refresh
type QueryResultCache struct {
	ResultId       string
	ResultTitle    string
	ResultSubTitle string
	ContextData    string
	Refresh        func(RefreshableResult) RefreshableResult
	PluginInstance *Instance
	Query          Query
	Actions        *util.HashMap[string, func(actionContext ActionContext)]
}

func newQueryWithPlugins(query string, queryType QueryType, pluginInstances []*Instance) Query {
	if queryType == QueryTypeFile {
		return Query{
			Type:     QueryTypeFile,
			RawQuery: query,
			Search:   query,
		}
	}

	var terms = strings.Split(query, " ")
	if len(terms) == 0 {
		return Query{
			Type:     QueryTypeText,
			RawQuery: query,
		}
	}

	var rawQuery = query
	var triggerKeyword, command, search string
	var possibleTriggerKeyword = terms[0]
	var mustContainSpace = strings.Contains(query, " ")

	pluginInstance, found := lo.Find(pluginInstances, func(instance *Instance) bool {
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
			if len(terms) == 2 {
				// e.g "wpm install", we treat "install" as search, only "wpm install " will be treated as command
				command = ""
				search = terms[1]
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
		}
	} else {
		// non trigger keyword
		triggerKeyword = ""
		command = ""
		search = rawQuery
	}

	return Query{
		Type:           QueryTypeText,
		RawQuery:       query,
		TriggerKeyword: triggerKeyword,
		Command:        command,
		Search:         search,
	}
}

func NewQuery(query string, queryType QueryType) Query {
	return newQueryWithPlugins(query, queryType, GetPluginManager().GetPluginInstances())
}
