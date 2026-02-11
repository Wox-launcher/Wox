package shell

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/setting/definition"
	"wox/util"
	shellutil "wox/util/shell"

	"github.com/google/uuid"
)

const (
	shellInterpreterSettingKey = "shell_interpreter"
	shellCommandsSettingKey    = "shellCommands"
	shellActionSessionIDKey    = "session_id"
	shellActionHistoryIDKey    = "history_id"
	shellActionCommandKey      = "command"
	shellActionInterpreterKey  = "interpreter"
	shellOutputSummaryMaxBytes = 64 * 1024
)

var shellIcon = common.PluginShellIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ShellPlugin{})
}

type ShellPlugin struct {
	api             plugin.API
	historyManager  *ShellHistoryManager
	terminalManager *terminal.Manager
	// map[session_id]*shellExecutionState
	executionStates sync.Map
	// map[result_id]session_id
	resultSessions sync.Map
}

type shellContextData struct {
	Command     string `json:"command"`
	Interpreter string `json:"interpreter"`
	HistoryID   string `json:"-"`
	FromHistory bool   `json:"-"`
}

type shellCommand struct {
	Alias   string `json:"Alias"`
	Command string `json:"Command"`
	Enabled bool   `json:"Enabled"`
	Silent  bool   `json:"Silent"` // If true, execute in background; if false, jump to > trigger to show output
}

type shellExecutionState struct {
	sessionID      string
	summaryOutput  string
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
		Name:          "i18n:plugin_shell_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_shell_plugin_description",
		Icon:          shellIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			">",
			"*", // Enable global query for commands
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
				Params: map[string]any{
					"WidthRatio": 0.3,
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
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     shellCommandsSettingKey,
					Title:   "i18n:plugin_shell_commands",
					Tooltip: "i18n:plugin_shell_commands_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Alias",
							Label: "i18n:plugin_shell_command_alias",
							Type:  definition.PluginSettingValueTableColumnTypeText,
							Width: 100,
						},
						{
							Key:          "Command",
							Label:        "i18n:plugin_shell_command_script",
							Tooltip:      "i18n:plugin_shell_command_script_tooltip",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 5,
						},
						{
							Key:   "Enabled",
							Label: "i18n:plugin_shell_command_enabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 60,
						},
						{
							Key:     "Silent",
							Label:   "i18n:plugin_shell_command_silent",
							Tooltip: "i18n:plugin_shell_command_silent_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
						},
					},
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
	s.terminalManager = terminal.GetSessionManager()

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
		history := history
		s.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("History: %s, created_at:%s", history.Command, history.CreatedAt.String()))

		runtimeSessionID := history.SessionID
		hasRuntimeSession := runtimeSessionID != ""
		if history.Status == "running" && hasRuntimeSession {
			runtimeState, ok := s.terminalManager.GetState(runtimeSessionID)
			if !ok {
				history.Status = "failed"
				history.ExitCode = -1
				history.EndTime = util.GetSystemTimestamp()
				history.Duration = history.EndTime - history.StartTime
				err := s.historyManager.UpdateStatus(ctx, history.ID, "failed", -1, history.EndTime, history.Duration)
				if err != nil {
					s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update history status: %s", err.Error()))
				}
			} else if runtimeState.Status != terminal.SessionStatusRunning {
				history.Status = string(runtimeState.Status)
				history.ExitCode = runtimeState.ExitCode
				history.EndTime = runtimeState.EndTime
				history.Duration = history.EndTime - history.StartTime
				err := s.historyManager.UpdateStatus(ctx, history.ID, history.Status, history.ExitCode, history.EndTime, history.Duration)
				if err != nil {
					s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update history status: %s", err.Error()))
				}
			}
		}
		if history.Status == "running" && !hasRuntimeSession {
			// Running record without runtime means this command is stale.
			history.Status = "failed"
			history.ExitCode = -1
			history.EndTime = util.GetSystemTimestamp()
			history.Duration = history.EndTime - history.StartTime
			err := s.historyManager.UpdateStatus(ctx, history.ID, "failed", -1, history.EndTime, history.Duration)
			if err != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update history status: %s", err.Error()))
			}
		}

		title := s.buildSessionTitle(ctx, history.Command, history.Status)
		subTitle := time.Unix(history.StartTime/1000, 0).Format("2006-01-02 15:04:05")

		// Build actions based on status
		var actions []plugin.QueryResultAction

		// Always add re-execute action
		actions = append(actions, plugin.QueryResultAction{
			Id:                     "reexecute",
			Name:                   "i18n:plugin_shell_reexecute",
			Icon:                   common.UpdateIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, interpreter),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if s.stopSessionByHistoryID(ctx, history.ID) {
					return
				}

				contextData := shellContextData{
					Command:     history.Command,
					Interpreter: interpreter,
					HistoryID:   history.ID,
					FromHistory: true,
				}
				util.Go(ctx, "re-execute shell command from history", func() {
					s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData)
				})
			},
		})

		// Only add stop action if command is still running
		if history.Status == "running" {
			actions = append(actions, plugin.QueryResultAction{
				Id:                     "stop",
				Name:                   "i18n:plugin_shell_stop",
				Icon:                   common.TerminateAppIcon,
				PreventHideAfterAction: true,
				ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, interpreter),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					s.stopSessionByHistoryID(ctx, history.ID)
				},
			})
		}

		actions = append(actions, plugin.QueryResultAction{
			Id:                     "delete",
			Name:                   "i18n:plugin_shell_delete",
			Icon:                   common.TrashIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, interpreter),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				util.Go(ctx, "delete shell session from history", func() {
					if err := s.deleteSessionResources(ctx, history.ID, history.SessionID, actionContext.ResultId); err != nil {
						s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to delete shell session(history=%s): %s", history.ID, err.Error()))
						return
					}
					s.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: false})
				})
			},
		})

		previewType := plugin.WoxPreviewTypeText
		previewData := history.OutputSummary
		if hasRuntimeSession {
			previewType = plugin.WoxPreviewTypeTerminal
			previewData = s.buildTerminalPreviewData(runtimeSessionID, history.Command, history.Status)
		}

		results = append(results, plugin.QueryResult{
			Title:    title,
			SubTitle: subTitle,
			Icon:     shellIcon,
			Preview: plugin.WoxPreview{
				PreviewType:       previewType,
				PreviewData:       previewData,
				PreviewProperties: map[string]string{},
			},
			Actions: actions,
		})
	}

	return results
}

func (s *ShellPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	// Get the configured interpreter
	interpreter := s.api.GetSetting(ctx, shellInterpreterSettingKey)
	if interpreter == "" {
		interpreter = getDefaultInterpreter()
	}

	// Handle global query - check for shell commands
	if query.IsGlobalQuery() {
		return s.queryCommands(ctx, query, interpreter)
	}

	// Get the command from the query
	command := strings.TrimSpace(query.Search)

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
					Icon:                   common.CorrectIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if s.stopSessionByResultID(ctx, actionContext.ResultId) {
							return
						}
						util.Go(ctx, "execute shell command", func() {
							s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData)
						})
					},
				},
				{
					Id:                     "execute_background",
					Name:                   "i18n:plugin_shell_execute_background",
					Icon:                   common.OpenIcon,
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
					Icon:                   common.TerminateAppIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						s.stopSessionByResultID(ctx, actionContext.ResultId)
					},
				},
				{
					Id:                     "reexecute",
					Name:                   "i18n:plugin_shell_reexecute",
					Icon:                   common.UpdateIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if s.stopSessionByResultID(ctx, actionContext.ResultId) {
							return
						}
						util.Go(ctx, "re-execute shell command", func() {
							s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, contextData)
						})
					},
				},
				{
					Id:                     "delete",
					Name:                   "i18n:plugin_shell_delete",
					Icon:                   common.TrashIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "delete shell session from current result", func() {
							if err := s.deleteSessionByActionContext(ctx, actionContext); err != nil {
								s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to delete shell session(result=%s): %s", actionContext.ResultId, err.Error()))
								return
							}
							s.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: false})
						})
					},
				},
			},
		},
	}
}

// loadCommands loads configured shell commands from settings
func (s *ShellPlugin) loadCommands(ctx context.Context) []shellCommand {
	commandsJson := s.api.GetSetting(ctx, shellCommandsSettingKey)
	if commandsJson == "" {
		return nil
	}

	var commands []shellCommand
	err := json.Unmarshal([]byte(commandsJson), &commands)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to unmarshal shell commands: %s", err.Error()))
		return nil
	}

	return commands
}

// queryCommands handles global query to search for configured shell commands
func (s *ShellPlugin) queryCommands(ctx context.Context, query plugin.Query, interpreter string) []plugin.QueryResult {
	commands := s.loadCommands(ctx)
	if len(commands) == 0 {
		return nil
	}

	search := strings.TrimSpace(query.Search)
	if search == "" {
		return nil
	}

	// Parse alias and query parameter
	parts := strings.SplitN(search, " ", 2)
	searchAlias := strings.ToLower(parts[0])
	queryParam := ""
	if len(parts) > 1 {
		queryParam = parts[1]
	}

	var results []plugin.QueryResult
	for _, cmd := range commands {
		if !cmd.Enabled {
			continue
		}

		// Match alias (case-insensitive, prefix match for better UX)
		if !strings.HasPrefix(strings.ToLower(cmd.Alias), searchAlias) {
			continue
		}

		// Replace {query} placeholder with query parameter
		finalCommand := strings.ReplaceAll(cmd.Command, "{query}", queryParam)

		// Create context data for execution
		contextData := shellContextData{
			Command:     finalCommand,
			Interpreter: interpreter,
			FromHistory: false,
		}

		subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_with"), interpreter, finalCommand)

		// Build actions based on Silent option
		var actions []plugin.QueryResultAction

		if cmd.Silent {
			// Silent mode: execute in background and hide Wox
			actions = []plugin.QueryResultAction{
				{
					Id:                     "execute_background",
					Name:                   "i18n:plugin_shell_execute_background",
					Icon:                   common.OpenIcon,
					PreventHideAfterAction: false,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "execute shell command in background", func() {
							s.executeCommandInBackground(ctx, contextData)
						})
					},
				},
			}
		} else {
			// Non-silent mode: change query to > trigger to show output
			actions = []plugin.QueryResultAction{
				{
					Id:                     "execute",
					Name:                   "i18n:plugin_shell_execute",
					Icon:                   common.CorrectIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// Change query to > trigger with the command to show output
						s.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("> %s", finalCommand),
						})
					},
				},
				{
					Id:                     "execute_background",
					Name:                   "i18n:plugin_shell_execute_background",
					Icon:                   common.OpenIcon,
					PreventHideAfterAction: false,
					Hotkey:                 "ctrl+enter",
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						util.Go(ctx, "execute shell command in background", func() {
							s.executeCommandInBackground(ctx, contextData)
						})
					},
				},
			}
		}

		result := plugin.QueryResult{
			Title:    cmd.Alias,
			SubTitle: subtitle,
			Icon:     shellIcon,
			Score:    100,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeText,
				PreviewData:    fmt.Sprintf("$ %s", finalCommand),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: actions,
		}
		results = append(results, result)
	}

	return results
}

// executeCommandWithUpdateResult executes a shell command and updates metadata via UpdateResult.
func (s *ShellPlugin) executeCommandWithUpdateResult(ctx context.Context, resultId string, data shellContextData) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command: %s with interpreter: %s", data.Command, data.Interpreter))

	session, err := s.terminalManager.CreateSession(ctx, terminal.CreateSessionParams{
		Command:     data.Command,
		Interpreter: data.Interpreter,
	})
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create terminal session: %s", err.Error()))
		return
	}

	startTs := util.GetSystemTimestamp()
	historyID := data.HistoryID
	if data.FromHistory && data.HistoryID != "" {
		if resetErr := s.historyManager.ResetForReexecute(ctx, data.HistoryID, session.ID, data.Command, data.Interpreter, startTs, session.OutputPath); resetErr != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to reset shell history for re-execute (id=%s): %s", data.HistoryID, resetErr.Error()))
			historyID = ""
		}
	}
	if historyID == "" {
		historyID = uuid.NewString()
		createErr := s.historyManager.Create(ctx, &ShellHistory{
			ID:            historyID,
			SessionID:     session.ID,
			Command:       data.Command,
			Interpreter:   data.Interpreter,
			Status:        "running",
			StartTime:     startTs,
			OutputSummary: "",
			OutputPath:    session.OutputPath,
		})
		if createErr != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create shell history: %s", createErr.Error()))
		}
	}

	cmd := s.buildCommand(ctx, data.Interpreter, data.Command)
	setCommandProcessGroup(cmd)

	state := &shellExecutionState{
		sessionID: session.ID,
		isRunning: true,
		startTime: time.Now(),
		cmd:       cmd,
	}
	s.executionStates.Store(session.ID, state)
	s.resultSessions.Store(resultId, session.ID)
	s.terminalManager.SetState(ctx, session.ID, terminal.SessionStatusRunning, 0, "")

	updateUI := func() bool {
		state.mutex.RLock()
		isRunning := state.isRunning
		startTime := state.startTime
		exitCode := state.exitCode
		state.mutex.RUnlock()

		var status string
		if isRunning {
			status = "running"
		} else if exitCode == 0 {
			status = "completed"
		} else if exitCode == -1 {
			status = "killed"
		} else {
			status = "failed"
		}
		title := s.buildSessionTitle(ctx, data.Command, status)
		subTitle := startTime.Format("2006-01-02 15:04:05")

		preview := plugin.WoxPreview{
			PreviewType:       plugin.WoxPreviewTypeTerminal,
			PreviewData:       s.buildTerminalPreviewData(session.ID, data.Command, status),
			PreviewProperties: map[string]string{},
			ScrollPosition:    plugin.WoxPreviewScrollPositionBottom,
		}

		updatable := plugin.UpdatableResult{
			Id:       resultId,
			Title:    &title,
			SubTitle: &subTitle,
			Preview:  &preview,
		}

		currentResult := s.api.GetUpdatableResult(ctx, resultId)
		if currentResult != nil && currentResult.Actions != nil {
			actions := *currentResult.Actions
			for i := range actions {
				if actions[i].ContextData == nil {
					actions[i].ContextData = map[string]string{}
				}
				actions[i].ContextData[shellActionSessionIDKey] = session.ID
				actions[i].ContextData[shellActionHistoryIDKey] = historyID
				actions[i].ContextData[shellActionCommandKey] = data.Command
				actions[i].ContextData[shellActionInterpreterKey] = data.Interpreter
			}
			if len(actions) > 0 {
				if isRunning {
					actions[0].Name = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_stop")
					actions[0].Icon = common.TerminateAppIcon
				} else {
					actions[0].Name = i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_reexecute")
					actions[0].Icon = common.UpdateIcon
				}
			}
			updatable.Actions = &actions
		}

		return s.api.UpdateResult(ctx, updatable)
	}

	tracker := newShellHistoryTracker(s.historyManager, historyID, state, session.OutputPath)
	tracker.start(ctx)
	_ = updateUI()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to create stdout pipe: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.exitCode = 1
		state.mutex.Unlock()
		s.terminalManager.SetState(ctx, session.ID, terminal.SessionStatusFailed, 1, err.Error())
		tracker.stop(ctx, "failed", 1)
		_ = updateUI()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to create stderr pipe: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.exitCode = 1
		state.mutex.Unlock()
		s.terminalManager.SetState(ctx, session.ID, terminal.SessionStatusFailed, 1, err.Error())
		tracker.stop(ctx, "failed", 1)
		_ = updateUI()
		return
	}

	if err := cmd.Start(); err != nil {
		state.mutex.Lock()
		state.errorMessage = fmt.Sprintf("Failed to start command: %s", err.Error())
		state.isRunning = false
		state.isFinished = true
		state.endTime = time.Now()
		state.exitCode = 1
		state.mutex.Unlock()
		s.terminalManager.SetState(ctx, session.ID, terminal.SessionStatusFailed, 1, err.Error())
		tracker.stop(ctx, "failed", 1)
		_ = updateUI()
		return
	}

	stopUpdater := make(chan struct{})
	util.Go(ctx, "shell command metadata updater", func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopUpdater:
				return
			case <-ticker.C:
				if !updateUI() {
					return
				}
			}
		}
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.pipeOutputToSession(ctx, stdout, state)
	}()
	go func() {
		defer wg.Done()
		s.pipeOutputToSession(ctx, stderr, state)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	close(stopUpdater)

	state.mutex.Lock()
	state.isRunning = false
	state.isFinished = true
	state.endTime = time.Now()

	historyStatus := "completed"
	terminalStatus := terminal.SessionStatusCompleted
	if state.isKilledByUser {
		state.exitCode = -1
		historyStatus = "killed"
		terminalStatus = terminal.SessionStatusKilled
	} else if waitErr != nil {
		historyStatus = "failed"
		terminalStatus = terminal.SessionStatusFailed
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			state.exitCode = exitErr.ExitCode()
		} else {
			state.exitCode = 1
			state.errorMessage = waitErr.Error()
		}
	} else {
		state.exitCode = 0
	}
	exitCode := state.exitCode
	errMsg := state.errorMessage
	state.mutex.Unlock()

	s.terminalManager.SetState(ctx, session.ID, terminalStatus, exitCode, errMsg)
	tracker.stop(ctx, historyStatus, exitCode)
	_ = updateUI()
}

func (s *ShellPlugin) executeCommandInBackground(ctx context.Context, data shellContextData) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command in background: %s with interpreter: %s", data.Command, data.Interpreter))

	cmd := s.buildCommand(ctx, data.Interpreter, data.Command)
	setCommandProcessGroup(cmd)

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

func (s *ShellPlugin) buildCommand(ctx context.Context, interpreter string, command string) *exec.Cmd {
	switch interpreter {
	case "powershell":
		return shellutil.BuildCommandContext(ctx, "powershell", nil, "-Command", command)
	case "cmd":
		return shellutil.BuildCommandContext(ctx, "cmd", nil, "/C", command)
	case "bash":
		return shellutil.BuildCommandContext(ctx, "bash", nil, "-c", command)
	case "zsh":
		return shellutil.BuildCommandContext(ctx, "zsh", nil, "-c", command)
	case "sh":
		return shellutil.BuildCommandContext(ctx, "sh", nil, "-c", command)
	case "python", "python3":
		return shellutil.BuildCommandContext(ctx, interpreter, nil, "-c", command)
	case "node":
		return shellutil.BuildCommandContext(ctx, "node", nil, "-e", command)
	default:
		return shellutil.BuildCommandContext(ctx, interpreter, nil, "-c", command)
	}
}

func (s *ShellPlugin) pipeOutputToSession(ctx context.Context, reader io.Reader, state *shellExecutionState) {
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadString('\n')
		if line != "" {
			state.mutex.Lock()
			state.summaryOutput = appendSummaryOutput(state.summaryOutput, line, shellOutputSummaryMaxBytes)
			sessionID := state.sessionID
			state.mutex.Unlock()
			s.terminalManager.AppendChunk(ctx, sessionID, line)
		}
		if err != nil {
			if err != io.EOF {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read command output: %s", err.Error()))
			}
			return
		}
	}
}

func appendSummaryOutput(existing string, chunk string, maxBytes int) string {
	if maxBytes <= 0 {
		return existing + chunk
	}
	combined := existing + chunk
	if len(combined) <= maxBytes {
		return combined
	}
	return combined[len(combined)-maxBytes:]
}

func (s *ShellPlugin) stopSessionByResultID(ctx context.Context, resultID string) bool {
	sessionID, ok := s.resultSessions.Load(resultID)
	if !ok {
		return false
	}
	return s.stopSessionByID(ctx, sessionID.(string))
}

func (s *ShellPlugin) stopSessionByHistoryID(ctx context.Context, historyID string) bool {
	history, err := s.historyManager.GetByID(ctx, historyID)
	if err != nil || history == nil || history.SessionID == "" {
		return false
	}
	return s.stopSessionByID(ctx, history.SessionID)
}

func (s *ShellPlugin) stopSessionByID(ctx context.Context, sessionID string) bool {
	stateVal, ok := s.executionStates.Load(sessionID)
	if !ok {
		return false
	}
	state := stateVal.(*shellExecutionState)
	state.mutex.Lock()
	defer state.mutex.Unlock()
	if !state.isRunning || state.cmd == nil || state.cmd.Process == nil {
		return false
	}
	state.isKilledByUser = true
	if err := killProcessGroup(state.cmd); err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to kill shell process group: %s", err.Error()))
		return false
	}
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Shell session stopped: %s", sessionID))
	return true
}

func (s *ShellPlugin) buildActionContextData(sessionID string, historyID string, command string, interpreter string) map[string]string {
	contextData := map[string]string{}
	if sessionID != "" {
		contextData[shellActionSessionIDKey] = sessionID
	}
	if historyID != "" {
		contextData[shellActionHistoryIDKey] = historyID
	}
	if command != "" {
		contextData[shellActionCommandKey] = command
	}
	if interpreter != "" {
		contextData[shellActionInterpreterKey] = interpreter
	}
	return contextData
}

func (s *ShellPlugin) deleteSessionByActionContext(ctx context.Context, actionContext plugin.ActionContext) error {
	historyID := actionContext.ContextData[shellActionHistoryIDKey]
	sessionID := actionContext.ContextData[shellActionSessionIDKey]

	if sessionID == "" {
		if mappedSessionID, ok := s.resultSessions.Load(actionContext.ResultId); ok {
			if value, valid := mappedSessionID.(string); valid {
				sessionID = value
			}
		}
	}

	if historyID == "" && sessionID != "" {
		history, err := s.historyManager.GetBySessionID(ctx, sessionID)
		if err == nil && history != nil {
			historyID = history.ID
		}
	}

	if historyID == "" && sessionID == "" {
		return fmt.Errorf("shell session context not found")
	}

	return s.deleteSessionResources(ctx, historyID, sessionID, actionContext.ResultId)
}

func (s *ShellPlugin) deleteSessionResources(ctx context.Context, historyID string, sessionID string, resultID string) error {
	var history *ShellHistory

	if historyID != "" {
		record, err := s.historyManager.GetByID(ctx, historyID)
		if err != nil {
			s.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Failed to load shell history by id before delete(id=%s): %s", historyID, err.Error()))
		} else {
			history = record
		}
	}

	if history == nil && sessionID != "" {
		record, err := s.historyManager.GetBySessionID(ctx, sessionID)
		if err == nil {
			history = record
			if historyID == "" {
				historyID = record.ID
			}
		}
	}

	if history != nil && sessionID == "" {
		sessionID = history.SessionID
	}

	if sessionID != "" {
		s.stopSessionByID(ctx, sessionID)
	}

	if sessionID != "" {
		if err := s.terminalManager.DeleteSession(sessionID); err != nil {
			return err
		}
		s.executionStates.Delete(sessionID)
		s.resultSessions.Range(func(key any, value any) bool {
			existingSessionID, ok := value.(string)
			if ok && existingSessionID == sessionID {
				s.resultSessions.Delete(key)
			}
			return true
		})
	}

	if historyID != "" {
		if err := s.historyManager.Delete(ctx, historyID); err != nil {
			return err
		}
	} else if sessionID != "" {
		if err := s.historyManager.DeleteBySessionID(ctx, sessionID); err != nil {
			return err
		}
	}

	if history != nil && history.OutputPath != "" {
		if err := os.Remove(history.OutputPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if resultID != "" {
		s.resultSessions.Delete(resultID)
	}

	return nil
}

func (s *ShellPlugin) buildTerminalPreviewData(sessionID string, command string, status string) string {
	payload, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"command":    command,
		"status":     status,
	})
	return string(payload)
}

func (s *ShellPlugin) statusText(ctx context.Context, status string) string {
	switch status {
	case "completed":
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_success")
	case "failed":
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_failed")
	case "killed":
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_killed")
	default:
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_status_running")
	}
}

func (s *ShellPlugin) buildSessionTitle(ctx context.Context, command string, status string) string {
	return command
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
