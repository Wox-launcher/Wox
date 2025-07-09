package host

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/shell"
)

func init() {
	host := &ScriptHost{}
	plugin.AllHosts = append(plugin.AllHosts, host)
}

type ScriptHost struct {
	// Script host doesn't need persistent connections like websocket hosts
}

func (s *ScriptHost) GetRuntime(ctx context.Context) plugin.Runtime {
	return plugin.PLUGIN_RUNTIME_SCRIPT
}

func (s *ScriptHost) Start(ctx context.Context) error {
	// Script host doesn't need to start any background processes
	util.GetLogger().Info(ctx, "Script host started")
	return nil
}

func (s *ScriptHost) Stop(ctx context.Context) {
	// Script host doesn't need to stop any background processes
	util.GetLogger().Info(ctx, "Script host stopped")
}

func (s *ScriptHost) IsStarted(ctx context.Context) bool {
	// Script host is always "started" since it doesn't maintain persistent connections
	return true
}

func (s *ScriptHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	// For script plugins, the actual script file is in the user script plugins directory
	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
	scriptPath := filepath.Join(userScriptPluginDirectory, metadata.Entry)

	// Check if script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("script file not found: %s", scriptPath)
	}

	// Make sure script is executable
	if err := os.Chmod(scriptPath, 0755); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to make script executable: %s", err.Error()))
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Loaded script plugin: %s", metadata.Name))
	return NewScriptPlugin(metadata, scriptPath), nil
}

func (s *ScriptHost) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	// Script plugins don't need explicit unloading since they're not persistent
	util.GetLogger().Info(ctx, fmt.Sprintf("Unloaded script plugin: %s", metadata.Name))
}

// ScriptPlugin represents a script-based plugin
type ScriptPlugin struct {
	metadata   plugin.Metadata
	scriptPath string
}

func NewScriptPlugin(metadata plugin.Metadata, scriptPath string) *ScriptPlugin {
	return &ScriptPlugin{
		metadata:   metadata,
		scriptPath: scriptPath,
	}
}

func (sp *ScriptPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	// Script plugins don't need initialization since they're executed on-demand
	util.GetLogger().Debug(ctx, fmt.Sprintf("Script plugin %s initialized", sp.metadata.Name))
}

func (sp *ScriptPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	// Prepare JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "query",
		"params": map[string]interface{}{
			"search":          query.Search,
			"trigger_keyword": query.TriggerKeyword,
			"command":         query.Command,
			"raw_query":       query.RawQuery,
		},
		"id": util.GetContextTraceId(ctx),
	}

	// Execute script and get results
	results, err := sp.executeScript(ctx, request)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s query failed: %s", sp.metadata.Name, err.Error()))
		return []plugin.QueryResult{}
	}

	return results
}

// executeScript executes the script with the given JSON-RPC request and returns the results
func (sp *ScriptPlugin) executeScript(ctx context.Context, request map[string]interface{}) ([]plugin.QueryResult, error) {
	// Execute script and get raw response
	response, err := sp.executeScriptRaw(ctx, request)
	if err != nil {
		return nil, err
	}

	// Extract results
	result, exists := response["result"]
	if !exists {
		return []plugin.QueryResult{}, nil
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	items, exists := resultMap["items"]
	if !exists {
		return []plugin.QueryResult{}, nil
	}

	itemsArray, ok := items.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid items format")
	}

	// Convert items to QueryResult
	var queryResults []plugin.QueryResult
	for _, item := range itemsArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		queryResult := plugin.QueryResult{
			Title:    getStringFromMap(itemMap, "title"),
			SubTitle: getStringFromMap(itemMap, "subtitle"),
			Score:    int64(getFloatFromMap(itemMap, "score")),
		}

		// Handle action if present
		if actionData, exists := itemMap["action"]; exists {
			if actionMap, ok := actionData.(map[string]interface{}); ok {
				queryResult.Actions = []plugin.QueryResultAction{
					{
						Name: "Execute",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							sp.executeAction(ctx, actionMap)
						},
					},
				}
			}
		}

		queryResults = append(queryResults, queryResult)
	}

	return queryResults, nil
}

// executeAction executes an action from a script plugin result
func (sp *ScriptPlugin) executeAction(ctx context.Context, actionData map[string]interface{}) {
	// Prepare JSON-RPC request for action
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "action",
		"params": map[string]interface{}{
			"id":   getStringFromMap(actionData, "id"),
			"data": getStringFromMap(actionData, "data"),
		},
		"id": util.GetContextTraceId(ctx),
	}

	// Execute script for action
	err := sp.executeScriptAction(ctx, request)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s action failed: %s", sp.metadata.Name, err.Error()))
	}
}

// executeScriptRaw executes the script with the given JSON-RPC request and returns the raw response
func (sp *ScriptPlugin) executeScriptRaw(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	// Convert request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine the interpreter based on file extension
	interpreter, err := sp.getInterpreter(ctx)
	if err != nil {
		return nil, err
	}

	// Set timeout for script execution
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Prepare command
	var cmd *exec.Cmd
	if interpreter != "" {
		cmd = exec.CommandContext(timeoutCtx, interpreter, sp.scriptPath)
	} else {
		cmd = exec.CommandContext(timeoutCtx, sp.scriptPath)
	}

	// Set up environment variables for script plugins
	cmd.Env = append(os.Environ(),
		"WOX_DIRECTORY_USER_SCRIPT_PLUGINS="+util.GetLocation().GetUserScriptPluginsDirectory(),
		"WOX_DIRECTORY_USER_DATA="+util.GetLocation().GetUserDataDirectory(),
		"WOX_DIRECTORY_WOX_DATA="+util.GetLocation().GetWoxDataDirectory(),
		"WOX_DIRECTORY_PLUGINS="+util.GetLocation().GetPluginDirectory(),
		"WOX_DIRECTORY_THEMES="+util.GetLocation().GetThemeDirectory(),
	)

	// Set up stdin with the JSON-RPC request
	cmd.Stdin = strings.NewReader(string(requestJSON))

	// Execute script
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	// Parse JSON-RPC response
	var response map[string]interface{}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse script response: %w", err)
	}

	// Check for JSON-RPC error
	if errorData, exists := response["error"]; exists {
		return nil, fmt.Errorf("script returned error: %v", errorData)
	}

	return response, nil
}

// executeScriptAction executes the script for action requests
func (sp *ScriptPlugin) executeScriptAction(ctx context.Context, request map[string]interface{}) error {
	// Execute script and get raw response
	response, err := sp.executeScriptRaw(ctx, request)
	if err != nil {
		return err
	}

	// Handle action result if present
	if result, exists := response["result"]; exists {
		if resultMap, ok := result.(map[string]interface{}); ok {
			sp.handleActionResult(ctx, resultMap)
		}
	}

	return nil
}

// handleActionResult handles the result from an action execution
func (sp *ScriptPlugin) handleActionResult(ctx context.Context, result map[string]interface{}) {
	actionType := getStringFromMap(result, "action")

	switch actionType {
	case "open-url":
		url := getStringFromMap(result, "url")
		if url != "" {
			// TODO: Implement URL opening functionality
			util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s requested to open URL: %s", sp.metadata.Name, url))
		}
	case "open-directory":
		path := getStringFromMap(result, "path")
		if path != "" {
			// Open directory using shell.Open
			if err := shell.Open(path); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s failed to open directory %s: %s", sp.metadata.Name, path, err.Error()))
			} else {
				util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s opened directory: %s", sp.metadata.Name, path))
			}
		}
	case "notify":
		message := getStringFromMap(result, "message")
		if message != "" {
			// TODO: Implement notification functionality
			util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s notification: %s", sp.metadata.Name, message))
		}
	case "copy-to-clipboard":
		text := getStringFromMap(result, "text")
		if text != "" {
			// TODO: Implement clipboard functionality
			util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s requested to copy to clipboard: %s", sp.metadata.Name, text))
		}
	default:
		util.GetLogger().Warn(ctx, fmt.Sprintf("Script plugin %s returned unknown action type: %s", sp.metadata.Name, actionType))
	}
}

// getInterpreter determines the appropriate interpreter for the script based on file extension
func (sp *ScriptPlugin) getInterpreter(ctx context.Context) (string, error) {
	ext := strings.ToLower(filepath.Ext(sp.scriptPath))

	switch ext {
	case ".py":
		// Check if user has configured a custom Python path
		customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomPythonPath.Get()
		if customPath != "" && util.IsFileExists(customPath) {
			return customPath, nil
		}
		return "python3", nil
	case ".js":
		// Check if user has configured a custom Node.js path
		customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomNodejsPath.Get()
		if customPath != "" && util.IsFileExists(customPath) {
			return customPath, nil
		}
		return "node", nil
	case ".sh":
		return "bash", nil
	case ".rb":
		return "ruby", nil
	case ".pl":
		return "perl", nil
	case "": // No extension, assume it's executable
		return "", nil
	default:
		return "", fmt.Errorf("unsupported script type: %s", ext)
	}
}

// Helper functions to safely extract values from maps
func getStringFromMap(m map[string]interface{}, key string) string {
	if value, exists := m[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if value, exists := m[key]; exists {
		if num, ok := value.(float64); ok {
			return num
		}
		if num, ok := value.(int); ok {
			return float64(num)
		}
	}
	return 0
}
