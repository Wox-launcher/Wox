package script

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util/shell"
)

const flowRPCPrefix = "flow.launcher."

// flowBridge receives calls a JSON-RPC plugin makes back into the host.
type flowBridge interface {
	ChangeQuery(ctx context.Context, query string)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, title string, subtitle string)
	CopyText(ctx context.Context, text string)
	OpenPath(ctx context.Context, target string) error
	OpenDirectory(ctx context.Context, directory string, fileName string) error
	ShellRun(ctx context.Context, command string) error
	Log(ctx context.Context, message string)
}

type flowAction struct {
	Method     string
	Parameters []any
	DontHide   bool
}

type flowResult struct {
	Title       string
	SubTitle    string
	IconPath    string
	Score       int64
	CopyText    string
	PreviewText string
	PreviewFile string
	RoundIcon   bool
	Action      *flowAction
}

type flowReply struct {
	Results     []flowResult
	Settings    map[string]any
	HasSettings bool
}

// handleFlowMethod runs a Flow.Launcher.* call. Plugin methods return false.
func handleFlowMethod(ctx context.Context, bridge flowBridge, method string, params []any) bool {
	if bridge == nil {
		return false
	}
	name := canonicalFlowAPI(method)
	if name == "" {
		return false
	}
	if ctx.Err() != nil {
		return true
	}
	switch name {
	case "changequery":
		bridge.ChangeQuery(ctx, flowParamString(params, 0))
	case "shellrun":
		_ = bridge.ShellRun(ctx, flowParamString(params, 0))
	case "copytoclipboard":
		bridge.CopyText(ctx, flowParamString(params, 0))
	case "openurl", "openappuri":
		_ = bridge.OpenPath(ctx, flowParamString(params, 0))
	case "opendirectory":
		_ = bridge.OpenDirectory(ctx, flowParamString(params, 0), flowParamString(params, 1))
	case "showmsg", "showmsgerror":
		bridge.Notify(ctx, flowParamString(params, 0), flowParamString(params, 1))
	case "hideapp", "closeapp":
		bridge.HideApp(ctx)
	case "showapp":
		bridge.ShowApp(ctx)
	case "log", "logdebug", "loginfo", "logwarn", "logerror":
		bridge.Log(ctx, flowParamString(params, 0))
	case "restartapp", "saveappallsettings", "checkfornewupdate", "reloadallplugindata", "startloadingbar", "stoploadingbar", "opensettingdialog", "getallplugins", "gettranslation":
		bridge.Log(ctx, "unsupported host method "+method)
	default:
		return false
	}
	return true
}

func canonicalFlowAPI(method string) string {
	name := strings.ToLower(strings.TrimSpace(method))
	name = strings.TrimPrefix(name, flowRPCPrefix)
	if name == strings.ToLower(strings.TrimSpace(method)) {
		return ""
	}
	return strings.ReplaceAll(name, ".", "")
}

func flowParamString(params []any, index int) string {
	if index < 0 || index >= len(params) || params[index] == nil {
		return ""
	}
	switch typed := params[index].(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

// parseFlowDocument reads one JSON value from a plugin process.
// API calls return handled=true and an empty reply.
func parseFlowDocument(raw []byte) (flowReply, string, []any, bool, error) {
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return flowReply{}, "", nil, false, err
	}
	object, isObject := document.(map[string]any)
	if isObject {
		if method := flowObjectString(object, "method"); method != "" && flowObject(object, "result") == nil && flowObject(object, "error") == nil {
			return flowReply{}, method, flowObjectParams(object), true, nil
		}
	}
	reply, err := flowReplyFromValue(document)
	return reply, "", nil, false, err
}

// parseFlowOutput reads a one-shot process transcript. Host API lines are dispatched
// before the result document is returned.
func parseFlowOutput(ctx context.Context, bridge flowBridge, output string) (flowReply, error) {
	lines := strings.Split(output, "\n")
	var reply flowReply
	sawReply := false
	parsedAny := false
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		if line == "" {
			continue
		}
		part, method, params, isAPI, err := parseFlowDocument([]byte(line))
		if err != nil {
			continue
		}
		parsedAny = true
		if isAPI {
			handleFlowMethod(ctx, bridge, method, params)
			continue
		}
		reply = mergeFlowReply(reply, part)
		sawReply = true
	}
	if sawReply {
		return reply, nil
	}
	if !parsedAny {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			return flowReply{}, nil
		}
		part, method, params, isAPI, err := parseFlowDocument([]byte(trimmed))
		if err != nil {
			return flowReply{}, fmt.Errorf("plugin output is not JSON: %w", err)
		}
		if isAPI {
			handleFlowMethod(ctx, bridge, method, params)
			return flowReply{}, nil
		}
		return part, nil
	}
	return reply, nil
}

func mergeFlowReply(base flowReply, extra flowReply) flowReply {
	if len(extra.Results) > 0 {
		base.Results = extra.Results
	}
	if extra.HasSettings {
		base.Settings = extra.Settings
		base.HasSettings = true
	}
	return base
}

func flowReplyFromValue(document any) (flowReply, error) {
	if document == nil {
		return flowReply{}, nil
	}
	switch typed := document.(type) {
	case []any:
		return flowReply{Results: flowResultsFromArray(typed)}, nil
	case map[string]any:
		if _, ok := typed["jsonrpc"]; ok {
			if errorObject := flowObject(typed, "error"); errorObject != nil {
				message := "plugin call failed"
				if object, ok := errorObject.(map[string]any); ok {
					if text := flowObjectString(object, "message"); text != "" {
						message = text
					}
				}
				return flowReply{}, fmt.Errorf("%s", message)
			}
			if result, ok := flowLookup(typed, "result"); ok {
				reply, err := flowReplyFromValue(result)
				if err != nil {
					return flowReply{}, err
				}
				reply = attachFlowSettings(reply, typed)
				return reply, nil
			}
		}
		reply := flowReply{}
		if result, ok := flowLookup(typed, "result"); ok {
			switch nested := result.(type) {
			case []any:
				reply.Results = flowResultsFromArray(nested)
			case map[string]any:
				if _, isResultObject := flowLookup(nested, "title"); isResultObject {
					reply.Results = []flowResult{flowResultFromObject(nested)}
				} else if inner, ok := flowLookup(nested, "result"); ok {
					if items, ok := inner.([]any); ok {
						reply.Results = flowResultsFromArray(items)
					}
				}
			}
		} else if _, ok := flowLookup(typed, "title"); ok {
			reply.Results = []flowResult{flowResultFromObject(typed)}
		}
		reply = attachFlowSettings(reply, typed)
		return reply, nil
	default:
		return flowReply{}, fmt.Errorf("plugin output must be a JSON object or array")
	}
}

func attachFlowSettings(reply flowReply, object map[string]any) flowReply {
	for _, name := range []string{"settings", "settingsChange"} {
		value, ok := flowLookup(object, name)
		if !ok {
			continue
		}
		settings, ok := value.(map[string]any)
		if !ok {
			continue
		}
		reply.Settings = settings
		reply.HasSettings = true
		break
	}
	return reply
}

func flowResultsFromArray(items []any) []flowResult {
	results := make([]flowResult, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		results = append(results, flowResultFromObject(object))
	}
	return results
}

func flowResultFromObject(object map[string]any) flowResult {
	result := flowResult{
		Title:     flowObjectString(object, "title"),
		SubTitle:  flowObjectString(object, "subtitle"),
		IconPath:  flowObjectString(object, "icopath", "iconpath"),
		Score:     flowObjectInt(object, "score"),
		CopyText:  flowObjectString(object, "copytext"),
		RoundIcon: flowObjectBool(object, "roundedicon"),
	}
	preview, ok := flowLookup(object, "preview")
	if ok {
		switch typed := preview.(type) {
		case string:
			result.PreviewText = typed
		case map[string]any:
			result.PreviewFile = flowObjectString(typed, "filepath", "path")
			result.PreviewText = flowObjectString(typed, "description")
		}
	}
	actionValue, ok := flowLookup(object, "jsonrpcaction")
	if ok {
		if actionObject, ok := actionValue.(map[string]any); ok {
			method := flowObjectString(actionObject, "method")
			if method != "" {
				result.Action = &flowAction{
					Method:     method,
					Parameters: flowObjectParams(actionObject),
					DontHide:   flowObjectBool(actionObject, "donthideafteraction"),
				}
			}
		}
	}
	return result
}

func flowObjectParams(object map[string]any) []any {
	value, ok := flowLookup(object, "parameters", "params")
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func flowObject(object map[string]any, name string) any {
	value, _ := flowLookup(object, name)
	return value
}

func flowObjectString(object map[string]any, names ...string) string {
	value, ok := flowLookup(object, names...)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func flowObjectInt(object map[string]any, names ...string) int64 {
	value, ok := flowLookup(object, names...)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func flowObjectBool(object map[string]any, names ...string) bool {
	value, ok := flowLookup(object, names...)
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	case float64:
		return typed != 0
	default:
		return false
	}
}

func flowLookup(object map[string]any, names ...string) (any, bool) {
	folded := make(map[string]any, len(object))
	for key, value := range object {
		folded[strings.ToLower(key)] = value
	}
	for _, name := range names {
		value, ok := folded[strings.ToLower(name)]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func flowAssetPath(directory, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(directory, filepath.FromSlash(value))
}

func flowImage(directory, value string) common.WoxImage {
	resolved := flowAssetPath(directory, value)
	if resolved == "" {
		return common.WoxImage{}
	}
	if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
		return common.NewWoxImageUrl(resolved)
	}
	return common.NewWoxImageAbsolutePath(resolved)
}

func flowActionName(method string) string {
	switch canonicalFlowAPI(method) {
	case "copytoclipboard":
		return "Copy"
	case "openurl", "openappuri", "opendirectory":
		return "Open"
	case "shellrun":
		return "Run"
	}
	name := method
	if slash := strings.LastIndex(name, "."); slash >= 0 {
		name = name[slash+1:]
	}
	name = strings.NewReplacer("_", " ", "-", " ").Replace(name)
	if name == "" {
		return "Run"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func flowActionIcon(method string) common.WoxImage {
	switch canonicalFlowAPI(method) {
	case "copytoclipboard":
		return icons.Get(icons.ActionCopy)
	case "openurl", "openappuri", "opendirectory":
		return icons.Get(icons.ActionOpen)
	default:
		if strings.EqualFold(method, "copy") || strings.Contains(strings.ToLower(method), "clipboard") {
			return icons.Get(icons.ActionCopy)
		}
		return icons.Get(icons.ActionExecute)
	}
}

// flowQueryResults turns plugin rows into launcher results. run executes one action.
func flowQueryResults(directory string, fallbackIcon common.WoxImage, results []flowResult, run func(ctx context.Context, method string, params []any)) []plugin.QueryResult {
	rows := make([]plugin.QueryResult, 0, len(results))
	for _, result := range results {
		icon := flowImage(directory, result.IconPath)
		if icon.IsEmpty() {
			icon = fallbackIcon
		}
		row := plugin.QueryResult{
			Title:             result.Title,
			SubTitle:          result.SubTitle,
			Icon:              icon,
			IconShowContainer: result.RoundIcon,
			Score:             result.Score,
		}
		if result.PreviewFile != "" {
			row.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: flowAssetPath(directory, result.PreviewFile),
			}
		} else if result.PreviewText != "" {
			row.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeText,
				PreviewData: result.PreviewText,
			}
		}
		if result.Action != nil {
			method := result.Action.Method
			params := append([]any{}, result.Action.Parameters...)
			row.Actions = append(row.Actions, plugin.QueryResultAction{
				Name:                   flowActionName(method),
				Type:                   plugin.QueryResultActionTypeExecute,
				Icon:                   flowActionIcon(method),
				IsDefault:              true,
				PreventHideAfterAction: result.Action.DontHide,
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					run(ctx, method, params)
				},
			})
		}
		if text := strings.TrimSpace(result.CopyText); text != "" {
			copied := text
			row.Actions = append(row.Actions, plugin.QueryResultAction{
				Name: "Copy",
				Type: plugin.QueryResultActionTypeExecute,
				Icon: icons.Get(icons.ActionCopy),
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					run(ctx, "Flow.Launcher.CopyToClipboard", []any{copied})
				},
			})
		}
		rows = append(rows, row)
	}
	return rows
}

func openFlowDirectory(directory, fileName string) error {
	fileName = strings.TrimSpace(fileName)
	if fileName != "" {
		target := fileName
		if !filepath.IsAbs(fileName) {
			target = filepath.Join(directory, fileName)
		}
		if err := shell.OpenFileInFolder(target); err == nil {
			return nil
		}
	}
	if strings.TrimSpace(directory) == "" {
		return fmt.Errorf("directory is empty")
	}
	return shell.Open(directory)
}
