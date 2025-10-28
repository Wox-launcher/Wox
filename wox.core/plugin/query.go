package plugin

import (
	"context"
	"strings"
	"wox/common"
	"wox/util"
	"wox/util/selection"

	"github.com/samber/lo"
)

type QueryType = string
type QueryVariable = string
type QueryResultTailType = string

const (
	QueryTypeInput     QueryType = "input"     // user input query
	QueryTypeSelection QueryType = "selection" // user selection query
)

const (
	QueryVariableSelectedText     QueryVariable = "{wox:selected_text}"
	QueryVariableActiveBrowserUrl QueryVariable = "{wox:active_browser_url}"
	QueryVariableFileExplorerPath QueryVariable = "{wox:file_explorer_path}"
)

const (
	QueryResultTailTypeText  QueryResultTailType = "text"  // string type
	QueryResultTailTypeImage QueryResultTailType = "image" // WoxImage type
)

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// By default, Wox will only pass QueryTypeInput query to plugin.
	// plugin author need to enable MetadataFeatureQuerySelection feature to handle QueryTypeSelection query
	Type QueryType

	// Raw query, this includes trigger keyword if it has.
	// We didn't recommend use this property directly. You should always use Search property.
	RawQuery string

	// Trigger keyword of a query. It can be empty if user is using global trigger keyword.
	// Empty trigger keyword means this query will be a global query, see IsGlobalQuery.
	//
	// NOTE: Only available when query type is QueryTypeInput
	TriggerKeyword string

	// Command part of a query.
	// Empty command means this query doesn't have a command.
	//
	// NOTE: Only available when query type is QueryTypeInput
	Command string

	// Search part of a query.
	// Empty search means this query doesn't have a search part.
	Search string

	// User selected or drag-drop data, can be text or file or image etc
	//
	// NOTE: Only available when query type is QueryTypeSelection
	Selection selection.Selection

	// additional query environment data
	// expose more context env data to plugin, E.g. plugin A only show result when active window title is "Chrome"
	Env QueryEnv
}

func (q *Query) IsGlobalQuery() bool {
	return q.Type == QueryTypeInput && q.TriggerKeyword == ""
}

func (q *Query) String() string {
	if q.Type == QueryTypeInput {
		return q.RawQuery
	}
	if q.Type == QueryTypeSelection {
		return q.Selection.String()
	}
	return ""
}

type QueryEnv struct {
	ActiveWindowTitle string          // active window title when user query, empty if not available
	ActiveWindowPid   int             // active window pid when user query, 0 if not available
	ActiveWindowIcon  common.WoxImage // active window icon when user query, empty if not available

	// active browser url when user query
	// Only available when active window is browser and https://github.com/Wox-launcher/Wox.Chrome.Extension is installed
	ActiveBrowserUrl string
}

// Query result return from plugin
type QueryResult struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id string
	// Title support i18n
	Title string
	// SubTitle support i18n
	SubTitle string
	Icon     common.WoxImage
	Preview  WoxPreview
	// Score of the result, the higher the score, the more relevant the result is, more likely to be displayed on top
	Score int64
	// Group results, Wox will group results by group name
	Group string
	// Score of the group, the higher the score, the more relevant the group is, more likely to be displayed on top
	GroupScore int64
	// Tails are additional results associate with this result, can be displayed in result detail view
	Tails []QueryResultTail
	// Additional data associate with this result, can be retrieved in Action function
	ContextData string
	Actions     []QueryResultAction
	// refresh result after specified interval, in milliseconds. If this value is 0, Wox will not refresh this result
	// interval can only divisible by 100, if not, Wox will use the nearest number which is divisible by 100
	// E.g. if you set 123, Wox will use 200, if you set 1234, Wox will use 1300
	RefreshInterval int
	// refresh result by calling OnRefresh function
	OnRefresh func(ctx context.Context, current RefreshableResult) RefreshableResult
}

type QueryResultTail struct {
	Type  QueryResultTailType
	Text  string          // only available when type is QueryResultTailTypeText
	Image common.WoxImage // only available when type is QueryResultTailTypeImage
}

func NewQueryResultTailText(text string) QueryResultTail {
	return QueryResultTail{
		Type: QueryResultTailTypeText,
		Text: text,
	}
}

func NewQueryResultTailTexts(texts ...string) []QueryResultTail {
	return lo.Map(texts, func(text string, index int) QueryResultTail {
		return NewQueryResultTailText(text)
	})
}

type QueryResultAction struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id string
	// Name support i18n
	Name string
	Icon common.WoxImage
	// If true, Wox will use this action as default action. There can be only one default action in results
	// This can be omitted, if you don't set it, Wox will use the first action as default action
	IsDefault bool
	// If true, Wox will not hide after user select this result
	PreventHideAfterAction bool
	Action                 func(ctx context.Context, actionContext ActionContext)
	// Hotkey to trigger this action. E.g. "ctrl+Shift+Space", "Ctrl+1", "Command+K"
	// Case insensitive, space insensitive
	// If IsDefault is true, Hotkey will be set to enter key by default
	// Wox will normalize the hotkey to platform specific format. E.g. "ctrl" will be converted to "control" on macOS
	Hotkey string

	// internal use
	IsSystemAction bool
}

type ActionContext struct {
	// Additional data associate with this result
	ContextData string
}

func (q *QueryResult) ToUI() QueryResultUI {
	return QueryResultUI{
		Id:          q.Id,
		Title:       q.Title,
		SubTitle:    q.SubTitle,
		Icon:        q.Icon,
		Preview:     q.Preview,
		Score:       q.Score,
		Group:       q.Group,
		GroupScore:  q.GroupScore,
		Tails:       q.Tails,
		ContextData: q.ContextData,
		Actions: lo.Map(q.Actions, func(action QueryResultAction, index int) QueryResultActionUI {
			return QueryResultActionUI{
				Id:                     action.Id,
				Name:                   action.Name,
				Icon:                   action.Icon,
				IsDefault:              action.IsDefault,
				PreventHideAfterAction: action.PreventHideAfterAction,
				Hotkey:                 action.Hotkey,
				IsSystemAction:         action.IsSystemAction,
			}
		}),
		RefreshInterval: q.RefreshInterval,
	}
}

type QueryResultUI struct {
	QueryId         string
	Id              string
	Title           string
	SubTitle        string
	Icon            common.WoxImage
	Preview         WoxPreview
	Score           int64
	Group           string
	GroupScore      int64
	Tails           []QueryResultTail
	ContextData     string
	Actions         []QueryResultActionUI
	RefreshInterval int
}

type QueryResultActionUI struct {
	Id                     string
	Name                   string
	Icon                   common.WoxImage
	IsDefault              bool
	PreventHideAfterAction bool
	Hotkey                 string

	// internal use
	IsSystemAction bool
}

// store latest result value after query/refresh, so we can retrieve data later in action/refresh
type QueryResultCache struct {
	ResultId       string
	ResultTitle    string
	ResultSubTitle string
	ContextData    string
	Icon           common.WoxImage
	Refresh        func(context.Context, RefreshableResult) RefreshableResult
	PluginInstance *Instance
	Query          Query
	Preview        WoxPreview
	Actions        *util.HashMap[string, func(ctx context.Context, actionContext ActionContext)]
}

func newQueryInputWithPlugins(query string, pluginInstances []*Instance) (Query, *Instance) {
	var terms = strings.Split(query, " ")
	if len(terms) == 0 {
		return Query{
			Type:     QueryTypeInput,
			RawQuery: query,
		}, nil
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
				if lo.ContainsBy(pluginInstance.GetQueryCommands(), func(item MetadataCommand) bool {
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
		pluginInstance = nil
	}

	return Query{
		Type:           QueryTypeInput,
		RawQuery:       query,
		TriggerKeyword: triggerKeyword,
		Command:        command,
		Search:         search,
	}, pluginInstance
}
