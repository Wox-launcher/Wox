package shell

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
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
	// Map to store execution states by result ID
	executionStates sync.Map // map[string]*shellExecutionState
}

type shellContextData struct {
	Command     string `json:"command"`
	Interpreter string `json:"interpreter"`
	HistoryID   string `json:"-"`
	FromHistory bool   `json:"-"`
}

type shellExecutionState struct {
	output         strings.Builder
	isRunning      bool
	isFinished     bool
	isKilledByUser bool // true if command was killed by user action
	exitCode       int
	errorMessage   string
	startTime      time.Time
	endTime        time.Time
	cmd            *exec.Cmd
	mutex          sync.RWMutex
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
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
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
	for _, history := range histories {
		s.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("History: %s, created_at:%s", history.Command, history.CreatedAt.String()))

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

		// Build actions based on status
		var actions []plugin.QueryResultAction

		// Always add re-execute action
		actions = append(actions, plugin.QueryResultAction{
			Id:                     "reexecute",
			Name:                   "i18n:plugin_shell_reexecute",
			Icon:                   plugin.UpdateIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				// Toggle: if running then stop; otherwise re-execute
				if stateVal, ok := s.executionStates.Load(actionContext.ResultId); ok {
					state := stateVal.(*shellExecutionState)
					state.mutex.Lock()
					if state.isRunning && state.cmd != nil && state.cmd.Process != nil {
						state.isKilledByUser = true
						state.cmd.Process.Kill()
						s.api.Log(ctx, plugin.LogLevelInfo, "Command killed by user via re-execute toggle")
						state.mutex.Unlock()
						return
					}
					state.mutex.Unlock()
				}
				// Start execution
				contextData := shellContextData{
					Command:     history.Command,
					Interpreter: interpreter,
					HistoryID:   history.ID,
					FromHistory: true,
				}
				executionState := &shellExecutionState{}
				s.executionStates.Store(actionContext.ResultId, executionState)
				util.Go(ctx, "re-execute shell command from history", func() {
					s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData, executionState)
				})
			},
		})

		// Only add stop action if command is still running
		if history.Status == "running" {
			actions = append(actions, plugin.QueryResultAction{
				Id:                     "stop",
				Name:                   "i18n:plugin_shell_stop",
				Icon:                   plugin.TerminateAppIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if stateVal, ok := s.executionStates.Load(actionContext.ResultId); ok {
						state := stateVal.(*shellExecutionState)
						state.mutex.Lock()
						if state.cmd != nil && state.cmd.Process != nil {
							state.isKilledByUser = true // Mark as killed by user
							state.cmd.Process.Kill()
							s.api.Log(ctx, plugin.LogLevelInfo, "Command killed by user")
						}
						state.mutex.Unlock()
					}
				},
			})
		}

		results = append(results, plugin.QueryResult{
			Title:    history.Command,
			SubTitle: subtitle,
			Icon:     shellIcon,
			Preview: plugin.WoxPreview{
				PreviewType:       plugin.WoxPreviewTypeText,
				PreviewData:       history.Output,
				PreviewProperties: previewProperties,
			},
			Actions: actions,
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
		FromHistory: false,
	}

	// Build subtitle with interpreter info
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_with"), interpreter, command)

	return []plugin.QueryResult{
		{
			Title:    command,
			SubTitle: subtitle,
			Icon:     shellIcon,
			Score:    100,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeText,
				PreviewData:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_enter_to_execute"),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: []plugin.QueryResultAction{
				{
					Id:                     "execute",
					Name:                   "i18n:plugin_shell_execute",
					Icon:                   plugin.CorrectIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						executionState := &shellExecutionState{}
						s.executionStates.Store(actionContext.ResultId, executionState)
						util.Go(ctx, "execute shell command", func() {
							s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData, executionState)
						})
					},
				},
				{
					Id:                     "execute_background",
					Name:                   "i18n:plugin_shell_execute_background",
					Icon:                   plugin.OpenIcon,
					PreventHideAfterAction: false,
					Hotkey:                 "ctrl+enter",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "execute shell command in background", func() {
							s.executeCommandInBackground(ctx, contextData)
						})
					},
				},
				{
					Id:                     "stop",
					Name:                   "i18n:plugin_shell_stop",
					Icon:                   plugin.TerminateAppIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if stateVal, ok := s.executionStates.Load(actionContext.ResultId); ok {
							state := stateVal.(*shellExecutionState)
							state.mutex.Lock()
							if state.cmd != nil && state.cmd.Process != nil {
								state.isKilledByUser = true // Mark as killed by user
								state.cmd.Process.Kill()
								s.api.Log(ctx, plugin.LogLevelInfo, "Command killed by user")
							}
							state.mutex.Unlock()
						}
					},
				},
				{
					Id:                     "reexecute",
					Name:                   "i18n:plugin_shell_reexecute",
					Icon:                   plugin.UpdateIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						executionState := &shellExecutionState{}
						s.executionStates.Store(actionContext.ResultId, executionState)
						util.Go(ctx, "re-execute shell command", func() {
							s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData, executionState)
						})
					},
				},
			},
		},
	}
}

// executeCommandWithUpdateResult executes a shell command and uses UpdateResult API to push updates
func (s *ShellPlugin) executeCommandWithUpdateResult(ctx context.Context, resultId string, data shellContextData, state *shellExecutionState) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command: %s with interpreter: %s", data.Command, data.Interpreter))

	// Helper function to update the result UI
	updateUI := func(subtitle, previewData string, previewProperties map[string]string, actionName *string, actionIcon *common.WoxImage) bool {
		preview := plugin.WoxPreview{
			PreviewType:       plugin.WoxPreviewTypeText,
			PreviewData:       previewData,
			PreviewProperties: previewProperties,
			ScrollPosition:    plugin.WoxPreviewScrollPositionBottom,
		}

		UpdatableResult := plugin.UpdatableResult{
			Id:       resultId,
			SubTitle: &subtitle,
			Preview:  &preview,
		}

		// Update action if provided
		if actionName != nil && actionIcon != nil {
			// Get current result to update actions
			currentResult := s.api.GetUpdatableResult(ctx, resultId)
			if currentResult != nil && currentResult.Actions != nil {
				// Update the first action (reexecute/stop action)
				actions := *currentResult.Actions
				if len(actions) > 0 {
					actions[0].Name = *actionName
					actions[0].Icon = *actionIcon
				}
				UpdatableResult.Actions = &actions
			}
		}

		success := s.api.UpdateResult(ctx, UpdatableResult)
		s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("UpdateResult called for %s, success: %v, preview length: %d", resultId, success, len(previewData)))
		return success
	}

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

	// Prepare or reuse history record
	startTs := util.GetSystemTimestamp()
	var historyID string
	if data.FromHistory && data.HistoryID != "" {
		// Reset existing record instead of creating a new one
		if err := s.historyManager.ResetForReexecute(ctx, data.HistoryID, data.Command, data.Interpreter, startTs); err != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to reset shell history for re-execute (id=%s), fallback to create new: %s", data.HistoryID, err.Error()))
			// Fallback: create a new record
			historyID = uuid.NewString()
			historyRecord := &ShellHistory{
				ID:          historyID,
				Command:     data.Command,
				Interpreter: data.Interpreter,
				Status:      "running",
				StartTime:   startTs,
			}
			if err := s.historyManager.Create(ctx, historyRecord); err != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create shell history: %s", err.Error()))
			}
		} else {
			// Success: reuse existing record
			historyID = data.HistoryID
		}
	} else {
		// Fresh execution: create a new record
		historyID = uuid.NewString()
		historyRecord := &ShellHistory{
			ID:          historyID,
			Command:     data.Command,
			Interpreter: data.Interpreter,
			Status:      "running",
			StartTime:   startTs,
		}
		if err := s.historyManager.Create(ctx, historyRecord); err != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create shell history: %s", err.Error()))
		}
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

		// Update UI with error
		updateUI(
			"❌ "+i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed"),
			fmt.Sprintf("$ %s\n\n❌ Error:\n%s", data.Command, state.errorMessage),
			nil,
			nil,
			nil,
		)
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

		// Update UI with error
		updateUI(
			"❌ "+i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed"),
			fmt.Sprintf("$ %s\n\n❌ Error:\n%s", data.Command, state.errorMessage),
			nil,
			nil,
			nil,
		)
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

		// Update UI with error
		updateUI(
			"❌ "+i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed"),
			fmt.Sprintf("$ %s\n\n❌ Error:\n%s", data.Command, state.errorMessage),
			nil,
			nil,
			nil,
		)

		// Stop tracker and save failed state
		tracker.stop(ctx, "failed", 1)
		return
	}

	// Start a goroutine to periodically update UI while running
	stopUpdater := make(chan struct{})
	util.Go(ctx, "shell command UI updater", func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopUpdater:
				return
			case <-ticker.C:
				state.mutex.RLock()
				if !state.isRunning {
					state.mutex.RUnlock()
					return
				}

				elapsed := time.Since(state.startTime)
				output := state.output.String()
				state.mutex.RUnlock()

				// Build preview
				var previewBuilder strings.Builder
				previewBuilder.WriteString(fmt.Sprintf("$ %s\n\n", data.Command))
				previewBuilder.WriteString(fmt.Sprintf("⏱️ Running... (%.1fs)\n\n", elapsed.Seconds()))
				if output != "" {
					previewBuilder.WriteString(output)
				}

				// Build properties
				previewProperties := map[string]string{
					i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_status"):      i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_running"),
					i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter"): data.Interpreter,
					i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_duration"):    fmt.Sprintf("%.1fs", elapsed.Seconds()),
					i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_start_time"):  state.startTime.Format("2006-01-02 15:04:05"),
				}

				// Build action name and icon (Stop action)
				actionName := i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_stop")
				actionIcon := plugin.TerminateAppIcon

				// Update UI - if it fails, just stop updating UI but let the command continue
				if !updateUI(
					"⏱️ "+i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_running"),
					previewBuilder.String(),
					previewProperties,
					&actionName,
					&actionIcon,
				) {
					// Result no longer visible in UI, stop updating but let command continue
					s.api.Log(ctx, plugin.LogLevelInfo, "Result no longer visible, stopping UI updates but command continues")
					return
				}
			}
		}
	})

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

	// Stop the UI updater
	close(stopUpdater)

	// Update state
	state.mutex.Lock()
	state.isRunning = false
	state.isFinished = true
	state.endTime = time.Now()

	var historyStatus string
	var statusIcon string
	var statusText string

	// Check if command was killed by user
	if state.isKilledByUser {
		state.exitCode = -1
		historyStatus = "killed"
		statusIcon = "🛑"
		statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_killed")
		s.api.Log(ctx, plugin.LogLevelInfo, "Command killed by user")
	} else if err != nil {
		// Command failed naturally
		if exitErr, ok := err.(*exec.ExitError); ok {
			state.exitCode = exitErr.ExitCode()
		} else {
			state.exitCode = 1
			state.errorMessage = err.Error()
		}
		historyStatus = "failed"
		statusIcon = "❌"
		statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed")
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Command failed: %s", err.Error()))
	} else {
		// Command completed successfully
		state.exitCode = 0
		historyStatus = "completed"
		statusIcon = "✅"
		statusText = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_success")
		s.api.Log(ctx, plugin.LogLevelInfo, "Command completed successfully")
	}
	exitCode := state.exitCode
	duration := state.endTime.Sub(state.startTime)
	output := state.output.String()
	errorMessage := state.errorMessage
	state.mutex.Unlock()

	// Stop history tracker and save final state
	tracker.stop(ctx, historyStatus, exitCode)

	// Build final preview
	var previewBuilder strings.Builder
	previewBuilder.WriteString(fmt.Sprintf("$ %s\n\n", data.Command))
	if exitCode == 0 {
		previewBuilder.WriteString(fmt.Sprintf("✅ Completed in %.2fs\n\n", duration.Seconds()))
	} else {
		previewBuilder.WriteString(fmt.Sprintf("❌ Failed with exit code %d (%.2fs)\n\n", exitCode, duration.Seconds()))
	}
	if output != "" {
		previewBuilder.WriteString(output)
	} else {
		previewBuilder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_no_output"))
	}
	if errorMessage != "" {
		previewBuilder.WriteString("\n\n❌ Error:\n")
		previewBuilder.WriteString(errorMessage)
	}

	// Build final properties
	previewProperties := map[string]string{
		i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_status"):      statusText,
		i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter"): data.Interpreter,
		i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_duration"):    fmt.Sprintf("%.2fs", duration.Seconds()),
		i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_start_time"):  state.startTime.Format("2006-01-02 15:04:05"),
		i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_exit_code"):   fmt.Sprintf("%d", exitCode),
	}

	// Build final action name and icon (Re-execute action)
	actionName := i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_reexecute")
	actionIcon := plugin.UpdateIcon

	// Final UI update
	updateUI(
		statusIcon+" "+statusText,
		previewBuilder.String(),
		previewProperties,
		&actionName,
		&actionIcon,
	)
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
