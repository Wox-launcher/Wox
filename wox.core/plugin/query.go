package plugin

import (
	"context"
	"strings"
	"wox/common"
	"wox/setting/definition"
	"wox/util/selection"

	"github.com/samber/lo"
)

type QueryResultActionType = string
type QueryType = string
type QueryVariable = string
type QueryResultTailType = string
type QueryResultTailTextCategory = string
type QueryRefinementType = string

const (
	QueryTypeInput     QueryType = "input"     // user input query
	QueryTypeSelection QueryType = "selection" // user selection query
)

const (
	QueryResultActionTypeExecute QueryResultActionType = "execute"
	QueryResultActionTypeForm    QueryResultActionType = "form"
)

const (
	QueryVariableSelectedText     QueryVariable = "{wox:selected_text}"
	QueryVariableSelectedFile     QueryVariable = "{wox:selected_file}"
	QueryVariableActiveBrowserUrl QueryVariable = "{wox:active_browser_url}"
	QueryVariableFileExplorerPath QueryVariable = "{wox:file_explorer_path}"
)

const (
	QueryResultTailTypeText  QueryResultTailType = "text"  // string type
	QueryResultTailTypeImage QueryResultTailType = "image" // WoxImage type
)

const (
	QueryResultTailTextCategoryDefault QueryResultTailTextCategory = "default"
	QueryResultTailTextCategoryDanger  QueryResultTailTextCategory = "danger"
	QueryResultTailTextCategoryWarning QueryResultTailTextCategory = "warning"
	QueryResultTailTextCategorySuccess QueryResultTailTextCategory = "success"
)

const (
	QueryRefinementTypeSingleSelect QueryRefinementType = "singleSelect"
	QueryRefinementTypeMultiSelect  QueryRefinementType = "multiSelect"
	QueryRefinementTypeToggle       QueryRefinementType = "toggle"
	QueryRefinementTypeSort         QueryRefinementType = "sort"
)

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Id identifies the current query session from UI.
	// It can be used to correlate async updates with the active query.
	Id string

	// SessionId identifies the UI session that owns this query.
	// It stays stable for the lifetime of a UI instance.
	SessionId string

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

	// Refinements carries query-scoped UI state selected by the user. Values are
	// strings to keep the plugin-facing API close to Env-style key/value data;
	// multi-select refinements are encoded as comma-separated strings.
	Refinements map[string]string

	// ContextData carries hidden query-scoped data. Unlike Refinements, this is
	// not rendered by the UI and is intended for plugin handoffs such as a shell
	// working directory.
	ContextData common.ContextData
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

	// These fields are core-only for built-in system plugins. Do not add them
	// to SDK models or public plugin docs unless we explicitly decide to expose
	// the native window handle/dialog state as public plugin API.
	ActiveWindowId               string `json:"-"` // exact top-level window id; Windows HWND, macOS CGWindowID
	ActiveWindowIsOpenSaveDialog bool   `json:"-"` // active window is open/save dialog when user query
}

// QueryResponse is the complete plugin answer for one query execution.
// The old API returned only []QueryResult, which forced query-scoped UI data
// such as refinements and layout hints into side channels. Keeping them in one
// response lets the UI apply the controls and results from the same query id.
type QueryResponse struct {
	Results     []QueryResult
	Refinements []QueryRefinement
	Layout      QueryLayout
	Context     QueryContext
}

func NewQueryResponse(results []QueryResult) QueryResponse {
	return QueryResponse{Results: results}
}

// QueryLayout carries query-scoped presentation hints.
// These fields are pointers because zero is a meaningful value for some hints:
// ResultPreviewWidthRatio=0 intentionally gives the preview the full result
// area. The old metadata side request could return that value explicitly; the
// QueryResponse path must keep the same distinction between unset and zero.
type QueryLayout struct {
	Icon                    *common.WoxImage                 `json:"Icon,omitempty"`
	ResultPreviewWidthRatio *float64                         `json:"ResultPreviewWidthRatio,omitempty"`
	GridLayout              *MetadataFeatureParamsGridLayout `json:"GridLayout,omitempty"`
}

// QueryContext carries the backend's canonical classification for a query.
// The UI can make a quick local guess for immediate rendering, but only core
// has the final parser state after shortcuts and trigger-keyword matching.
type QueryContext struct {
	IsGlobalQuery bool   `json:"IsGlobalQuery"`
	PluginId      string `json:"PluginId"`
}

// BuildQueryContext centralizes the parser result that the UI cannot reproduce
// cheaply. In particular, shortcuts and trigger-keyword parsing happen in core,
// so the response carries the resolved global/plugin classification back.
func BuildQueryContext(query Query, queryPlugin *Instance) QueryContext {
	queryContext := QueryContext{IsGlobalQuery: query.IsGlobalQuery()}
	if !queryContext.IsGlobalQuery && queryPlugin != nil {
		queryContext.PluginId = queryPlugin.Metadata.Id
	}
	return queryContext
}

// QueryRefinement describes one query-scoped control such as type filters or
// sort modes. Options are deliberately simple values because plugins, not core,
// interpret the selected values when the next query is executed.
type QueryRefinement struct {
	Id           string
	Title        string
	Type         QueryRefinementType
	Options      []QueryRefinementOption
	DefaultValue []string
	Hotkey       string
	Persist      bool
}

type QueryRefinementOption struct {
	Value    string
	Title    string
	Icon     common.WoxImage
	Keywords []string
	Count    *int
}

// RefreshQueryParam contains parameters for refreshing a query
type RefreshQueryParam struct {
	// PreserveSelectedIndex controls whether to maintain the previously selected item index after refresh
	// When true, the user's current selection index in the results list is preserved
	// When false, the selection resets to the first item (index 0)
	PreserveSelectedIndex bool
}

const QueryResultDragDataTypeFiles = "files"

// QueryResultDragData declares data the UI can export through a native drag session.
type QueryResultDragData struct {
	Type  string
	Files []string
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
	// ScoreKey is an optional stable identity for actioned-result scoring when title or subtitle is dynamic.
	ScoreKey string
	// Group results, Wox will group results by group name
	Group string
	// Score of the group, the higher the score, the more relevant the group is, more likely to be displayed on top
	GroupScore int64
	// Tails are additional results associate with this result, can be displayed in result detail view
	Tails   []QueryResultTail
	Actions []QueryResultAction
	// DragData declares what can be dragged out of Wox for this result.
	DragData *QueryResultDragData
}

type QueryResultTail struct {
	// Tail id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id           string
	Type         QueryResultTailType
	Text         string                      // only available when type is QueryResultTailTypeText
	TextCategory QueryResultTailTextCategory // optional semantic category when type is QueryResultTailTypeText
	Image        common.WoxImage             // only available when type is QueryResultTailTypeImage
	ImageWidth   *float64                    // optional width for image tails
	ImageHeight  *float64                    // optional height for image tails
	// Tooltip is hover-only context for compact tails. This keeps visual-only tails
	// readable without forcing plugins to trade result width for explanatory text.
	Tooltip string
	// Additional data associate with this tail, can be retrieved later
	ContextData map[string]string

	// internal use
	IsSystemTail bool
}

func NewQueryResultTailText(text string) QueryResultTail {
	return NewQueryResultTailTextWithCategory(text, QueryResultTailTextCategoryDefault)
}

func NewQueryResultTailTextWithCategory(text string, category QueryResultTailTextCategory) QueryResultTail {
	if category == "" {
		category = QueryResultTailTextCategoryDefault
	}

	return QueryResultTail{
		Type:         QueryResultTailTypeText,
		Text:         text,
		TextCategory: category,
	}
}

func NewQueryResultTailTexts(texts ...string) []QueryResultTail {
	return lo.Map(texts, func(text string, index int) QueryResultTail {
		return NewQueryResultTailText(text)
	})
}

type QueryResultAction struct {
	// Action id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id string
	// Action type, use QueryResultActionTypeExecute for immediate execution and QueryResultActionTypeForm for form submission
	Type QueryResultActionType
	// Name support i18n
	Name string
	Icon common.WoxImage
	// If true, Wox will use this action as default action. There can be only one default action in results
	// This can be omitted, if you don't set it, Wox will use the first action as default action
	IsDefault bool
	// If true, Wox will not hide after user select this result
	PreventHideAfterAction bool
	// Hotkey to trigger this action. E.g. "ctrl+Shift+Space", "Ctrl+1", "Command+K"
	// Case insensitive, space insensitive
	// If IsDefault is true, Hotkey will be set to enter key by default
	// Wox will normalize the hotkey to platform specific format. E.g. "ctrl" will be converted to "control" on macOS
	Hotkey string
	// Additional data associate with this action, can be retrieved later
	ContextData map[string]string

	// For execute action
	Action func(ctx context.Context, actionContext ActionContext) `json:"-"` // Exclude from JSON serialization

	// For form action
	Form     definition.PluginSettingDefinitions
	OnSubmit func(ctx context.Context, actionContext FormActionContext) `json:"-"` // Exclude from JSON serialization

	// internal use
	IsSystemAction bool
}

type ActionContext struct {
	// The ID of the result that triggered this action
	// This is automatically set by Wox when the action is invoked
	// Useful for calling UpdateResult API to update the result's UI
	ResultId string

	// The ID of the action that was triggered
	// This is automatically set by Wox when the action is invoked
	// Useful for calling UpdateResul API to update this specific action's UI
	ResultActionId string

	// Additional data associate with this action
	ContextData common.ContextData
}

type FormActionContext struct {
	ActionContext
	Values map[string]string
}

func (q *QueryResult) ToUI() QueryResultUI {
	return QueryResultUI{
		Id:         q.Id,
		Title:      q.Title,
		SubTitle:   q.SubTitle,
		Icon:       q.Icon,
		Preview:    q.Preview,
		Score:      q.Score,
		Group:      q.Group,
		GroupScore: q.GroupScore,
		Tails:      q.Tails,
		DragData:   q.DragData,
		Actions: lo.Map(q.Actions, func(action QueryResultAction, index int) QueryResultActionUI {
			actionType := action.Type
			if actionType == "" {
				actionType = QueryResultActionTypeExecute
			}
			return QueryResultActionUI{
				Id:                     action.Id,
				Type:                   actionType,
				Name:                   action.Name,
				Icon:                   action.Icon,
				IsDefault:              action.IsDefault,
				PreventHideAfterAction: action.PreventHideAfterAction,
				Hotkey:                 action.Hotkey,
				Form:                   action.Form,
				ContextData:            action.ContextData,
				IsSystemAction:         action.IsSystemAction,
			}
		}),
	}
}

func (q *QueryResponse) ToUI() QueryResponseUI {
	return QueryResponseUI{
		Results: lo.Map(q.Results, func(result QueryResult, index int) QueryResultUI {
			return result.ToUI()
		}),
		Refinements: q.Refinements,
		Layout:      q.Layout,
		Context:     q.Context,
	}
}

type QueryResultUI struct {
	QueryId    string
	Id         string
	Title      string
	SubTitle   string
	Icon       common.WoxImage
	Preview    WoxPreview
	Score      int64
	Group      string
	GroupScore int64
	Tails      []QueryResultTail
	Actions    []QueryResultActionUI
	DragData   *QueryResultDragData
	IsGroup    bool
}

type QueryResponseUI struct {
	Results             []QueryResultUI
	Refinements         []QueryRefinement
	Layout              QueryLayout
	Context             QueryContext
	QueryStartTimestamp int64 // end-to-end query start timestamp, preferably from Flutter request send time
}

// PushResultsPayload is used to push additional results to UI for a query.
type PushResultsPayload struct {
	QueryId string
	Results []QueryResultUI
}

type QueryResultActionUI struct {
	Id                     string
	Type                   QueryResultActionType
	Name                   string
	Icon                   common.WoxImage
	IsDefault              bool
	PreventHideAfterAction bool
	Hotkey                 string
	Form                   definition.PluginSettingDefinitions
	ContextData            map[string]string

	// internal use
	IsSystemAction bool
}

// UpdatableResult is used to update a query result that is currently displayed in the UI.
//
// This struct serves two purposes:
// 1. As the return type of GetUpdatableResult() - contains the current state of the result
// 2. As the parameter of UpdateResult() - specifies which fields to update
//
// When returned by GetUpdatableResult():
// - All fields contain the current values from the result cache
// - You can modify any fields and pass it back to UpdateResult()
//
// When passed to UpdateResult():
// - All fields except Id are optional (pointers). Only non-nil fields will be updated.
// - This allows you to update specific fields without affecting others.
//
// Example usage:
//
//	// Get current result state
//	updatableResult := api.GetUpdatableResult(ctx, resultId)
//	if updatableResult != nil {
//	    // Modify any fields
//	    newTitle := "Downloading... 50%"
//	    updatableResult.Title = &newTitle
//	    updatableResult.Tails = append(*updatableResult.Tails, NewQueryResultTailText("50%"))
//
//	    // Update the result
//	    api.UpdateResult(ctx, *updatableResult)
//	}
type UpdatableResult struct {
	// Id is required - identifies which result to update
	Id string

	// Optional fields - only non-nil fields will be updated
	Title    *string
	SubTitle *string
	Icon     *common.WoxImage
	Preview  *WoxPreview
	Tails    *[]QueryResultTail
	Actions  *[]QueryResultAction
	DragData *QueryResultDragData
}

// store latest result value after query/refresh, so we can retrieve data later in action/refresh
type QueryResultCache struct {
	Result         QueryResult // store the full QueryResult including actions with callbacks
	PluginInstance *Instance
	Query          Query
	Layout         QueryLayout // query layout used when polishing this result, so later updates keep the same surface sizing
	// FlushBatch is the debouncer batch that first sent this result in a visible response.
	FlushBatch int
	// BatchQueueElapsed is the elapsed time when queryRun put this result into the debouncer queue.
	BatchQueueElapsed    int64
	BatchQueueElapsedSet bool
	// QueryElapsed is the elapsed time when queryRun received the plugin response, measured from the end-to-end query start.
	QueryElapsed    int64
	QueryElapsedSet bool
	// PluginQueryElapsed is only the raw Plugin.Query duration, excluding manager polish and UI conversion.
	PluginQueryElapsed    int64
	PluginQueryElapsedSet bool
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
	if found && (mustContainSpace) {
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
