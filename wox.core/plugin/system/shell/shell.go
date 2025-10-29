package shell

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"

	"github.com/google/uuid"
)

const (
	shellInterpreterSettingKey = "shell_interpreter"
)

var shellIcon = common.NewWoxImageEmoji("💻")

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ShellPlugin{})
}

type ShellPlugin struct {
	api            plugin.API
	historyManager *ShellHistoryManager
}

type shellContextData struct {
	Command     string `json:"command"`
	Interpreter string `json:"interpreter"`
}

type shellExecutionState struct {
	output       strings.Builder
	isRunning    bool
	isFinished   bool
	exitCode     int
	errorMessage string
	startTime    time.Time
	endTime      time.Time
	cmd          *exec.Cmd
	mutex        sync.RWMutex
}

// isProcessRunning checks if a process is still running
func isProcessRunning(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	// Send signal 0 to check if process exists
	// This works on Unix-like systems (macOS, Linux)
	// On Windows, we need to use a different approach
	if util.IsWindows() {
		// On Windows, Process.Signal is not supported
		// We check if the process has exited by checking ProcessState
		return cmd.ProcessState == nil || !cmd.ProcessState.Exited()
	}

	// On Unix-like systems, send signal 0 to check if process exists
	err := cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

func (s *ShellPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d",
		Name:          "Shell",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Execute shell commands directly from Wox",
		Icon:          shellIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			">",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]string{
					"WidthRatio": "0.25",
				},
			},
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type:               definition.PluginSettingDefinitionTypeSelect,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueSelect{
					Key:          shellInterpreterSettingKey,
					Label:        "i18n:plugin_shell_interpreter",
					Tooltip:      "i18n:plugin_shell_interpreter_tooltip",
					DefaultValue: getDefaultInterpreter(),
					Options:      getInterpreterOptions(),
				},
			},
		},
	}
}

func getDefaultInterpreter() string {
	if util.IsWindows() {
		return "powershell"
	} else if util.IsMacOS() {
		return "bash"
	} else if util.IsLinux() {
		return "bash"
	}
	return "bash"
}

func getInterpreterOptions() []definition.PluginSettingValueSelectOption {
	if util.IsWindows() {
		return []definition.PluginSettingValueSelectOption{
			{Label: "PowerShell", Value: "powershell"},
			{Label: "CMD", Value: "cmd"},
			{Label: "Bash (WSL)", Value: "bash"},
			{Label: "Python", Value: "python"},
			{Label: "Node.js", Value: "node"},
		}
	} else if util.IsMacOS() {
		return []definition.PluginSettingValueSelectOption{
			{Label: "Bash", Value: "bash"},
			{Label: "Zsh", Value: "zsh"},
			{Label: "Sh", Value: "sh"},
			{Label: "Python", Value: "python3"},
			{Label: "Node.js", Value: "node"},
		}
	} else if util.IsLinux() {
		return []definition.PluginSettingValueSelectOption{
			{Label: "Bash", Value: "bash"},
			{Label: "Sh", Value: "sh"},
			{Label: "Zsh", Value: "zsh"},
			{Label: "Python", Value: "python3"},
			{Label: "Node.js", Value: "node"},
		}
	}
	return []definition.PluginSettingValueSelectOption{}
}

func (s *ShellPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	s.api = initParams.API
	s.historyManager = NewShellHistoryManager()

	// Initialize history table
	err := s.historyManager.Init(ctx)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to initialize shell history: %s", err.Error()))
	}

	// Enforce max history count (keep last 1000 records)
	deleted, err := s.historyManager.EnforceMaxCount(ctx, 1000)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to enforce max history count: %s", err.Error()))
	} else if deleted > 0 {
		s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Deleted %d old shell history records", deleted))
	}
}

func (s *ShellPlugin) queryHistory(ctx context.Context, interpreter string) []plugin.QueryResult {
	// Get recent history from database
	histories, err := s.historyManager.GetRecentHistory(ctx, 10)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to get history: %s", err.Error()))
		return []plugin.QueryResult{
			{
				Title:    "i18n:plugin_shell_enter_command",
				SubTitle: "i18n:plugin_shell_enter_command_subtitle",
				Icon:     shellIcon,
				Score:    100,
			},
		}
	}

	if len(histories) == 0 {
		return []plugin.QueryResult{
			{
				Title:    "i18n:plugin_shell_enter_command",
				SubTitle: "i18n:plugin_shell_enter_command_subtitle",
				Icon:     shellIcon,
				Score:    100,
			},
		}
	}

	var results []plugin.QueryResult
	for i, history := range histories {
		// Check if running status is still valid (process might have died)
		// This handles cases where Wox was restarted or process died unexpectedly
		if history.Status == "running" {
			// Mark as failed since we can't track the process anymore
			history.Status = "failed"
			history.ExitCode = -1
			history.EndTime = util.GetSystemTimestamp()
			history.Duration = history.EndTime - history.StartTime
			err := s.historyManager.UpdateStatus(ctx, history.ID, "failed", -1, history.EndTime, history.Duration)
			if err != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update history status: %s", err.Error()))
			}
		}

		// Format status for subtitle (simple)
		var statusIcon string
		var statusText string
		switch history.Status {
		case "completed":
			statusIcon = "✅"
			statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_success")
		case "failed":
			statusIcon = "❌"
			statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed")
		case "killed":
			statusIcon = "🛑"
			statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_killed")
		default:
			statusIcon = "⏱️"
			statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_running")
		}

		subtitle := fmt.Sprintf("%s %s", statusIcon, statusText)

		// Build preview properties with detailed information
		previewProperties := map[string]string{
			i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_status"):      statusText,
			i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter"): history.Interpreter,
			i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_duration"):    s.formatDuration(history.Duration),
			i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_start_time"):  time.Unix(history.StartTime/1000, 0).Format("2006-01-02 15:04:05"),
		}

		// Add exit code for completed/failed commands
		if history.Status == "completed" || history.Status == "failed" {
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_exit_code")] = fmt.Sprintf("%d", history.ExitCode)
		}

		// Create context data for re-execution
		contextData := shellContextData{
			Command:     history.Command,
			Interpreter: interpreter,
		}
		contextDataJson, _ := json.Marshal(contextData)

		// Create execution state for this history item
		state := &shellExecutionState{}
		hasStarted := false

		results = append(results, plugin.QueryResult{
			Title:           history.Command,
			SubTitle:        subtitle,
			Icon:            shellIcon,
			Score:           int64(100 - i), // Recent commands have higher scores
			ContextData:     string(contextDataJson),
			RefreshInterval: 100, // Enable refresh for re-execution
			Preview: plugin.WoxPreview{
				PreviewType:       plugin.WoxPreviewTypeText,
				PreviewData:       history.Output,
				PreviewProperties: previewProperties,
			},
			OnRefresh: s.createOnRefreshCallback(contextData, state, &hasStarted),
			Actions:   s.buildActions(ctx, contextData, state, &hasStarted),
		})
	}

	return results
}

func (s *ShellPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	// Get the command from the query
	command := strings.TrimSpace(query.Search)

	// Get the configured interpreter
	interpreter := s.api.GetSetting(ctx, shellInterpreterSettingKey)
	if interpreter == "" {
		interpreter = getDefaultInterpreter()
	}

	// If no command entered, show history
	if command == "" {
		return s.queryHistory(ctx, interpreter)
	}

	// Create context data
	contextData := shellContextData{
		Command:     command,
		Interpreter: interpreter,
	}
	contextDataJson, _ := json.Marshal(contextData)

	// Build subtitle with interpreter info
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_with"), interpreter, command)

	// Create execution state for this command
	executionState := &shellExecutionState{}
	hasStarted := false

	return []plugin.QueryResult{
		{
			Title:           command,
			SubTitle:        subtitle,
			Icon:            shellIcon,
			Score:           100,
			ContextData:     string(contextDataJson),
			RefreshInterval: 100, // Refresh every 100ms to show real-time output
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeText,
				PreviewData:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_enter_to_execute"),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			OnRefresh: s.createOnRefreshCallback(contextData, executionState, &hasStarted),
			Actions:   s.buildActions(ctx, contextData, executionState, &hasStarted),
		},
	}
}

// createOnRefreshCallback creates the OnRefresh callback for shell command execution
func (s *ShellPlugin) createOnRefreshCallback(contextData shellContextData, executionState *shellExecutionState, hasStarted *bool) func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
	return func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
		if !*hasStarted {
			return current
		}

		executionState.mutex.Lock()

		// Check if process is still running (for running state)
		if executionState.isRunning && !isProcessRunning(executionState.cmd) {
			// Process died unexpectedly
			executionState.isRunning = false
			executionState.isFinished = true
			executionState.endTime = time.Now()
			executionState.errorMessage = "Process terminated unexpectedly"
			s.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Shell command process died unexpectedly: %s", contextData.Command))
		}

		// Build preview content
		var previewBuilder strings.Builder
		previewBuilder.WriteString(fmt.Sprintf("$ %s\n\n", contextData.Command))

		var statusText string
		var statusIcon string
		previewProperties := make(map[string]string)

		if executionState.isRunning {
			elapsed := time.Since(executionState.startTime)
			previewBuilder.WriteString(fmt.Sprintf("⏱️ Running... (%.1fs)\n\n", elapsed.Seconds()))
			statusIcon = "⏱️"
			statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_running")

			// Add properties for running state
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_status")] = statusText
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter")] = contextData.Interpreter
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_duration")] = fmt.Sprintf("%.1fs", elapsed.Seconds())
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_start_time")] = executionState.startTime.Format("2006-01-02 15:04:05")
		} else if executionState.isFinished {
			duration := executionState.endTime.Sub(executionState.startTime)
			if executionState.exitCode == 0 {
				previewBuilder.WriteString(fmt.Sprintf("✅ Completed in %.2fs\n\n", duration.Seconds()))
				statusIcon = "✅"
				statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_success")
			} else {
				previewBuilder.WriteString(fmt.Sprintf("❌ Failed with exit code %d (%.2fs)\n\n", executionState.exitCode, duration.Seconds()))
				statusIcon = "❌"
				statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed")
			}

			// Add properties for finished state
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_status")] = statusText
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter")] = contextData.Interpreter
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_duration")] = fmt.Sprintf("%.2fs", duration.Seconds())
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_start_time")] = executionState.startTime.Format("2006-01-02 15:04:05")
			previewProperties[i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_exit_code")] = fmt.Sprintf("%d", executionState.exitCode)
		}

		// Add output
		output := executionState.output.String()
		if output != "" {
			previewBuilder.WriteString(output)
		} else if executionState.isFinished {
			previewBuilder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_no_output"))
		}

		// Add error message if any
		if executionState.errorMessage != "" {
			previewBuilder.WriteString("\n\n❌ Error:\n")
			previewBuilder.WriteString(executionState.errorMessage)
		}

		executionState.mutex.Unlock()

		// Update subtitle with simple status
		current.SubTitle = fmt.Sprintf("%s %s", statusIcon, statusText)

		// Update preview
		current.Preview.PreviewData = previewBuilder.String()
		current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom
		current.Preview.PreviewProperties = previewProperties

		// Stop refreshing when finished
		if executionState.isFinished {
			current.RefreshInterval = 0
		}

		// Update actions based on execution state
		current.Actions = s.buildActions(ctx, contextData, executionState, hasStarted)

		return current
	}
}

func (s *ShellPlugin) buildActions(ctx context.Context, data shellContextData, state *shellExecutionState, hasStarted *bool) []plugin.QueryResultAction {
	state.mutex.RLock()
	isRunning := state.isRunning
	isFinished := state.isFinished
	state.mutex.RUnlock()

	var actions []plugin.QueryResultAction

	if !*hasStarted {
		// Not started yet - show Execute and Execute in Background actions
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_shell_execute",
			Icon:                   plugin.CorrectIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if !*hasStarted {
					*hasStarted = true
					util.Go(ctx, "execute shell command", func() {
						s.executeCommandWithState(ctx, data, state)
					})
				}
			},
		})

		// Add "Execute in Background" action
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_shell_execute_background",
			Icon:                   plugin.OpenIcon,
			PreventHideAfterAction: false, // Hide Wox after execution
			Hotkey:                 "ctrl+enter",
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				util.Go(ctx, "execute shell command in background", func() {
					s.executeCommandInBackground(ctx, data)
				})
			},
		})
	} else if isRunning {
		// Running - show Stop action
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_shell_stop",
			Icon:                   plugin.TerminateAppIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				state.mutex.Lock()
				if state.cmd != nil && state.cmd.Process != nil {
					state.cmd.Process.Kill()
					s.api.Log(ctx, plugin.LogLevelInfo, "Command killed by user")
				}
				state.mutex.Unlock()
			},
		})
	} else if isFinished {
		// Finished - show Re-execute action
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_shell_reexecute",
			Icon:                   plugin.UpdateIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				// Reset state
				state.mutex.Lock()
				state.output.Reset()
				state.isRunning = false
				state.isFinished = false
				state.exitCode = 0
				state.errorMessage = ""
				state.cmd = nil
				state.mutex.Unlock()

				// Re-execute
				util.Go(ctx, "re-execute shell command", func() {
					s.executeCommandWithState(ctx, data, state)
				})
			},
		})
	}

	return actions
}

func (s *ShellPlugin) executeCommandWithState(ctx context.Context, data shellContextData, state *shellExecutionState) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command: %s with interpreter: %s", data.Command, data.Interpreter))

	// Build command based on interpreter
	var cmd *exec.Cmd
	switch data.Interpreter {
	case "powershell":
		cmd = exec.CommandContext(ctx, "powershell", "-Command", data.Command)
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/C", data.Command)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", data.Command)
	case "zsh":
		cmd = exec.CommandContext(ctx, "zsh", "-c", data.Command)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", data.Command)
	case "python", "python3":
		cmd = exec.CommandContext(ctx, data.Interpreter, "-c", data.Command)
	case "node":
		cmd = exec.CommandContext(ctx, "node", "-e", data.Command)
	default:
		cmd = exec.CommandContext(ctx, data.Interpreter, "-c", data.Command)
	}

	// Create history record
	historyID := uuid.NewString()
	historyRecord := &ShellHistory{
		ID:          historyID,
		Command:     data.Command,
		Interpreter: data.Interpreter,
		Status:      "running",
		StartTime:   util.GetSystemTimestamp(),
	}
	err := s.historyManager.Create(ctx, historyRecord)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create shell history: %s", err.Error()))
	}

	// Start history tracker for periodic output saving
	tracker := newShellHistoryTracker(s.historyManager, historyID, state)
	tracker.start(ctx)

	// Mark as running and save cmd
	state.mutex.Lock()
	state.isRunning = true
	state.startTime = time.Now()
	state.cmd = cmd
	state.mutex.Unlock()

	// Get stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to create stdout pipe: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.mutex.Unlock()
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to create stderr pipe: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.mutex.Unlock()
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to start command: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.exitCode = 1
		state.mutex.Unlock()

		// Stop tracker and save failed state
		tracker.stop(ctx, "failed", 1)
		return
	}

	// Read output in real-time
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			state.mutex.Lock()
			state.output.WriteString(line)
			state.output.WriteString("\n")
			state.mutex.Unlock()
		}
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			state.mutex.Lock()
			state.output.WriteString(line)
			state.output.WriteString("\n")
			state.mutex.Unlock()
		}
	}()

	// Wait for output reading to complete
	wg.Wait()

	// Wait for command to finish
	err = cmd.Wait()

	// Update state
	state.mutex.Lock()
	state.isRunning = false
	state.isFinished = true
	state.endTime = time.Now()

	var historyStatus string
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			state.exitCode = exitErr.ExitCode()
		} else {
			state.exitCode = 1
			state.errorMessage = err.Error()
		}
		historyStatus = "failed"
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Command failed: %s", err.Error()))
	} else {
		state.exitCode = 0
		historyStatus = "completed"
		s.api.Log(ctx, plugin.LogLevelInfo, "Command completed successfully")
	}
	exitCode := state.exitCode
	state.mutex.Unlock()

	// Stop history tracker and save final state
	tracker.stop(ctx, historyStatus, exitCode)
}

func (s *ShellPlugin) executeCommandInBackground(ctx context.Context, data shellContextData) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command in background: %s with interpreter: %s", data.Command, data.Interpreter))

	// Build command based on interpreter
	var cmd *exec.Cmd
	switch data.Interpreter {
	case "powershell":
		cmd = exec.CommandContext(ctx, "powershell", "-Command", data.Command)
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/C", data.Command)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", data.Command)
	case "zsh":
		cmd = exec.CommandContext(ctx, "zsh", "-c", data.Command)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", data.Command)
	case "python", "python3":
		cmd = exec.CommandContext(ctx, data.Interpreter, "-c", data.Command)
	case "node":
		cmd = exec.CommandContext(ctx, "node", "-e", data.Command)
	default:
		cmd = exec.CommandContext(ctx, data.Interpreter, "-c", data.Command)
	}

	// Start command in background without waiting
	if err := cmd.Start(); err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to start background command: %s", err.Error()))
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_failed"), err.Error()))
		return
	}

	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Background command started with PID: %d", cmd.Process.Pid))
	s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_background_started"))

	// Optionally wait for completion in background and log result
	util.Go(ctx, "wait for background command", func() {
		err := cmd.Wait()
		if err != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Background command failed: %s", err.Error()))
		} else {
			s.api.Log(ctx, plugin.LogLevelInfo, "Background command completed successfully")
		}
	})
}

// formatDuration formats duration in milliseconds to human-readable string
func (s *ShellPlugin) formatDuration(durationMs int64) string {
	duration := time.Duration(durationMs) * time.Millisecond

	if duration < time.Second {
		return fmt.Sprintf("%dms", durationMs)
	} else if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
}
