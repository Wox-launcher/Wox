package host

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/clipboard"
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
	api        plugin.API // API for accessing plugin settings
}

func NewScriptPlugin(metadata plugin.Metadata, scriptPath string) *ScriptPlugin {
	return &ScriptPlugin{
		metadata:   metadata,
		scriptPath: scriptPath,
	}
}

func (s *ScriptPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	// Save API reference for accessing settings
	s.api = initParams.API
	util.GetLogger().Debug(ctx, fmt.Sprintf("Script plugin %s initialized", s.metadata.Name))
}

func (s *ScriptPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
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
	results, err := s.executeScript(ctx, request)
	if err != nil {
		requestJSON, _ := json.Marshal(request)
		util.GetLogger().Error(ctx, fmt.Sprintf("script plugin query failed for %s: %s, raw request: %s", s.metadata.Name, err.Error(), requestJSON))
		return []plugin.QueryResult{
			plugin.GetPluginManager().GetResultForFailedQuery(ctx, s.metadata, query, err),
		}
	}

	return results
}

// executeScript executes the script with the given JSON-RPC request and returns the results
func (s *ScriptPlugin) executeScript(ctx context.Context, request map[string]interface{}) ([]plugin.QueryResult, error) {
	// Execute script and get raw response
	response, err := s.executeScriptRaw(ctx, request)
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

		// Icon: WoxImage.String() format, e.g. "base64:data:image/png;base64,xxx" or "emoji:🧮"
		if iconStr := getStringFromMap(itemMap, "icon"); iconStr != "" {
			if img, err := common.ParseWoxImage(iconStr); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("script plugin %s returned invalid icon: %s, err: %s", s.metadata.Name, iconStr, err.Error()))
			} else {
				// Normalize base64 without data URI header to png
				if img.ImageType == common.WoxImageTypeBase64 && !strings.Contains(img.ImageData, ",") {
					img.ImageData = fmt.Sprintf("data:image/png;base64,%s", img.ImageData)
				}
				queryResult.Icon = img
			}
		}

		// Handle actions - must be an array
		if actionsData, exists := itemMap["actions"]; exists {
			if actionsArray, ok := actionsData.([]interface{}); ok {
				for _, actionItem := range actionsArray {
					if actionMap, ok := actionItem.(map[string]interface{}); ok {
						actionName := getStringFromMap(actionMap, "name")
						if actionName == "" {
							actionName = "Execute"
						}

						// Capture actionMap in closure
						actionMapCopy := actionMap
						queryResult.Actions = append(queryResult.Actions, plugin.QueryResultAction{
							Name: actionName,
							Action: func(ctx context.Context, actionContext plugin.ActionContext) {
								s.executeAction(ctx, actionMapCopy)
							},
						})
					}
				}
			}
		}

		queryResults = append(queryResults, queryResult)
	}

	return queryResults, nil
}

// executeAction executes an action from a script plugin result
func (s *ScriptPlugin) executeAction(ctx context.Context, actionData map[string]interface{}) {
	actionId := getStringFromMap(actionData, "id")

	// Check if this is a built-in action that can be handled directly
	if s.handleBuiltInAction(ctx, actionId, actionData) {
		// Built-in action was handled, still call script action as a hook (optional for script to handle)
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "action",
			"params": map[string]interface{}{
				"id":   actionId,
				"data": getStringFromMap(actionData, "data"),
			},
			"id": util.GetContextTraceId(ctx),
		}

		// Call script action as a hook, but ignore errors since it's optional
		_ = s.executeScriptAction(ctx, request)
		return
	}

	// Custom action - must be handled by script
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "action",
		"params": map[string]interface{}{
			"id":   actionId,
			"data": getStringFromMap(actionData, "data"),
		},
		"id": util.GetContextTraceId(ctx),
	}

	// Execute script for custom action
	err := s.executeScriptAction(ctx, request)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s action failed: %s", s.metadata.Name, err.Error()))
	}
}

// executeScriptRaw executes the script with the given JSON-RPC request and returns the raw response
func (s *ScriptPlugin) executeScriptRaw(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	// Convert request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Determine the interpreter based on file extension
	interpreter, err := s.getInterpreter(ctx)
	if err != nil {
		return nil, err
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Using interpreter: '%s' for script: %s", interpreter, s.scriptPath))

	// Set timeout for script execution
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Prepare command
	var cmd *exec.Cmd
	if interpreter != "" {
		cmd = exec.CommandContext(timeoutCtx, interpreter, s.scriptPath)
		util.GetLogger().Debug(ctx, fmt.Sprintf("Executing command: %s %s", interpreter, s.scriptPath))
	} else {
		cmd = exec.CommandContext(timeoutCtx, s.scriptPath)
		util.GetLogger().Debug(ctx, fmt.Sprintf("Executing command: %s", s.scriptPath))
	}

	// Set up environment variables for script plugins
	envVars := []string{
		"WOX_DIRECTORY_USER_SCRIPT_PLUGINS=" + util.GetLocation().GetUserScriptPluginsDirectory(),
		"WOX_DIRECTORY_USER_DATA=" + util.GetLocation().GetUserDataDirectory(),
		"WOX_DIRECTORY_WOX_DATA=" + util.GetLocation().GetWoxDataDirectory(),
		"WOX_DIRECTORY_PLUGINS=" + util.GetLocation().GetPluginDirectory(),
		"WOX_DIRECTORY_THEMES=" + util.GetLocation().GetThemeDirectory(),
		"WOX_PLUGIN_ID=" + s.metadata.Id,
		"WOX_PLUGIN_NAME=" + s.metadata.Name,
	}

	// Add plugin settings as environment variables
	// Settings are prefixed with WOX_SETTING_ to avoid conflicts
	if s.api != nil {
		// Iterate through setting definitions to get all setting values
		for _, settingDef := range s.metadata.SettingDefinitions {
			if settingDef.Value != nil {
				key := settingDef.Value.GetKey()
				value := s.api.GetSetting(ctx, key)

				// Convert setting key to uppercase and replace special characters for env var name
				// e.g., "api_key" -> "WOX_SETTING_API_KEY"
				envKey := "WOX_SETTING_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
				envVars = append(envVars, envKey+"="+value)
			}
		}
	}

	cmd.Env = append(os.Environ(), envVars...)

	// Set up stdin with the JSON-RPC request
	cmd.Stdin = strings.NewReader(string(requestJSON))

	// Execute script
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("script execution failed: %s, stderr: %s", exitError.Error(), string(exitError.Stderr))
		}

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
func (s *ScriptPlugin) executeScriptAction(ctx context.Context, request map[string]interface{}) error {
	// Execute script and get raw response
	response, err := s.executeScriptRaw(ctx, request)
	if err != nil {
		return err
	}

	// Handle action result if present
	if result, exists := response["result"]; exists {
		if resultMap, ok := result.(map[string]interface{}); ok {
			s.handleActionResult(ctx, resultMap)
		}
	}

	return nil
}

// handleBuiltInAction handles built-in actions directly without calling script
// Returns true if the action was a built-in action (handled or not), false if it's a custom action
func (s *ScriptPlugin) handleBuiltInAction(ctx context.Context, actionId string, actionData map[string]interface{}) bool {
	switch actionId {
	case "copy-to-clipboard":
		// Support both "text" and "data" fields
		text := getStringFromMap(actionData, "text")
		if text == "" {
			text = getStringFromMap(actionData, "data")
		}
		if text != "" {
			if err := clipboard.WriteText(text); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s failed to copy to clipboard: %s", s.metadata.Name, err.Error()))
			} else {
				util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s copied to clipboard: %s", s.metadata.Name, text))
			}
		}
		return true

	case "open-url":
		url := getStringFromMap(actionData, "url")
		if url != "" {
			// TODO: Implement URL opening functionality
			util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s requested to open URL: %s", s.metadata.Name, url))
		}
		return true

	case "open-directory":
		path := getStringFromMap(actionData, "path")
		if path != "" {
			// Open directory using shell.Open
			if err := shell.Open(path); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Script plugin %s failed to open directory %s: %s", s.metadata.Name, path, err.Error()))
			} else {
				util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s opened directory: %s", s.metadata.Name, path))
			}
		}
		return true

	case "notify":
		message := getStringFromMap(actionData, "message")
		if message != "" {
			// TODO: Implement notification functionality
			util.GetLogger().Info(ctx, fmt.Sprintf("Script plugin %s notification: %s", s.metadata.Name, message))
		}
		return true

	default:
		// Not a built-in action
		return false
	}
}

// handleActionResult handles the result from an action execution (for custom actions that return results)
func (s *ScriptPlugin) handleActionResult(ctx context.Context, result map[string]interface{}) {
	actionType := getStringFromMap(result, "action")

	// This is for backward compatibility - if script returns an action result, handle it
	if actionType != "" {
		util.GetLogger().Warn(ctx, fmt.Sprintf("Script plugin %s returned action result (deprecated): %s", s.metadata.Name, actionType))
	}
}

// getInterpreter determines the appropriate interpreter for the script based on file extension or shebang
func (s *ScriptPlugin) getInterpreter(ctx context.Context) (string, error) {
	ext := strings.ToLower(filepath.Ext(s.scriptPath))

	// Try to determine interpreter from file extension first
	interpreter, err := s.getInterpreterFromExtension(ctx, ext)
	if err == nil {
		return interpreter, nil
	}

	// If extension is unknown or missing, try to read shebang from file
	shebangInterpreter, shebangErr := s.getInterpreterFromShebang(ctx)
	if shebangErr == nil && shebangInterpreter != "" {
		return shebangInterpreter, nil
	}

	// If no extension and no valid shebang, assume it's executable
	if ext == "" {
		return "", nil
	}

	return "", fmt.Errorf("unsupported script type: %s", ext)
}

// getInterpreterFromExtension determines interpreter based on file extension
func (s *ScriptPlugin) getInterpreterFromExtension(ctx context.Context, ext string) (string, error) {
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
	case "":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported extension: %s", ext)
	}
}

// getInterpreterFromShebang reads the first line of the script to determine the interpreter
func (s *ScriptPlugin) getInterpreterFromShebang(ctx context.Context) (string, error) {
	file, err := os.Open(s.scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open script file: %w", err)
	}
	defer file.Close()

	// Read first line using scanner
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read script file: %w", err)
		}
		return "", fmt.Errorf("script file is empty")
	}

	shebangLine := strings.TrimSpace(scanner.Text())

	// Check if it starts with shebang
	if !strings.HasPrefix(shebangLine, "#!") {
		return "", fmt.Errorf("no shebang found")
	}

	// Extract interpreter path from shebang
	interpreterPath := strings.TrimSpace(shebangLine[2:])
	util.GetLogger().Debug(ctx, fmt.Sprintf("Raw shebang line: %s", shebangLine))
	util.GetLogger().Debug(ctx, fmt.Sprintf("Extracted interpreter path: %s", interpreterPath))

	// Handle common shebang patterns
	// e.g., "#!/usr/bin/env python3" -> "python3"
	// e.g., "#!/usr/bin/python3" -> "python3"
	// e.g., "#!/bin/bash" -> "bash"
	parts := strings.Fields(interpreterPath)
	util.GetLogger().Debug(ctx, fmt.Sprintf("Shebang parts: %v (count: %d)", parts, len(parts)))

	if len(parts) == 0 {
		return "", fmt.Errorf("empty shebang interpreter")
	}

	// Check if using env
	if len(parts) >= 2 && (filepath.Base(parts[0]) == "env") {
		// Format: /usr/bin/env python3
		interpreterPath = parts[1]
		util.GetLogger().Debug(ctx, fmt.Sprintf("Detected env-based shebang, using: %s", interpreterPath))
	} else {
		// Format: /usr/bin/python3 or /bin/bash
		// Extract just the interpreter name from full path
		interpreterPath = filepath.Base(parts[0])
		util.GetLogger().Debug(ctx, fmt.Sprintf("Detected direct path shebang, using: %s", interpreterPath))
	}

	// Map common interpreter names and apply custom paths if configured
	interpreterPath = s.mapInterpreterWithCustomPath(ctx, interpreterPath)

	util.GetLogger().Debug(ctx, fmt.Sprintf("Final interpreter after mapping: %s", interpreterPath))
	return interpreterPath, nil
}

// mapInterpreterWithCustomPath maps interpreter names to custom paths if configured
func (s *ScriptPlugin) mapInterpreterWithCustomPath(ctx context.Context, interpreter string) string {
	// Normalize interpreter name
	interpreter = strings.ToLower(interpreter)

	// Check for Python interpreters
	if strings.HasPrefix(interpreter, "python") {
		customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomPythonPath.Get()
		if customPath != "" && util.IsFileExists(customPath) {
			return customPath
		}
		// Normalize to python3
		return "python3"
	}

	// Check for Node.js interpreters
	if interpreter == "node" || interpreter == "nodejs" {
		customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomNodejsPath.Get()
		if customPath != "" && util.IsFileExists(customPath) {
			return customPath
		}
		return "node"
	}

	// Return as-is for other interpreters (bash, ruby, perl, etc.)
	return interpreter
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
