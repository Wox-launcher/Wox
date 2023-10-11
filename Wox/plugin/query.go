package plugin

import (
	"github.com/samber/lo"
	"strings"
)

type QueryType = string

const (
	QueryTypeText QueryType = "text"
	QueryTypeFile QueryType = "file"
)

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Query type, can be "text" or "file"
	// if query type is "file", search property will be a absolute file path. note: you need to add features "queryFile" in plugin.json to support query type of file
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
	Id                     string
	Title                  string
	SubTitle               string
	Icon                   WoxImage
	Preview                WoxPreview
	Score                  int
	PreventHideAfterAction bool // If true, Wox will not hide after user select this result
	Action                 func()
}

type QueryResultUI struct {
	Id              string
	Title           string
	SubTitle        string
	Icon            WoxImage
	Preview         WoxPreview
	Score           int
	AssociatedQuery string
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
