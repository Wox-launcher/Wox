package shell

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	shellutil "wox/util/shell"

	"github.com/google/uuid"
)

const (
	PluginID                               = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"
	PluginCommandPrepareCommandAtDirectory = "prepare_command_at_directory"
	PluginCommandDataWorkingDirectory      = "working_directory"
	QueryContextWorkingDirectoryKey        = "wox:shell:working_directory"

	shellInterpreterSettingKey = "shell_interpreter"
	shellCommandsSettingKey    = "shellCommands"
	shellActionSessionIDKey    = "session_id"
	shellActionHistoryIDKey    = "history_id"
	shellActionCommandKey      = "command"
	shellActionInterpreterKey  = "interpreter"
	shellActionTitleKey        = "title"
	shellActionWorkingDirKey   = "working_directory"
	shellActionCommandIndexKey = "command_index"
	shellOutputSummaryMaxBytes = 64 * 1024

	shellFormTitleKey       = "title"
	shellFormCommandKey     = "command"
	shellFormInterpreterKey = "interpreter"
	shellFormWorkingDirKey  = "working_directory"
	shellFormSilentKey      = "silent"
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
	Title             string `json:"title"`
	Command           string `json:"command"`
	Interpreter       string `json:"interpreter"`
	WorkingDirectory  string `json:"working_directory"`
	HistoryID         string `json:"-"`
	FromHistory       bool   `json:"-"`
	Background        bool   `json:"-"`
	IsSavedCommand    bool   `json:"-"`
	SavedCommandIndex int    `json:"-"`
}

type shellCommand struct {
	Alias            string `json:"Alias"`
	Command          string `json:"Command"`
	Interpreter      string `json:"Interpreter"`
	WorkingDirectory string `json:"WorkingDirectory"`
	Enabled          bool   `json:"Enabled"`
	Silent           bool   `json:"Silent"` // If true, execute in background; if false, show output in the launcher preview.
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
		Id:            PluginID,
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
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
								{
									Type:  validator.PluginSettingValidatorTypeUnique,
									Value: &validator.PluginSettingValidatorUnique{},
								},
							},
						},
						{
							Key:          "Command",
							Label:        "i18n:plugin_shell_command_script",
							Tooltip:      "i18n:plugin_shell_command_script_tooltip",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 5,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:           "Interpreter",
							Label:         "i18n:plugin_shell_command_interpreter",
							Tooltip:       "i18n:plugin_shell_command_interpreter_tooltip",
							Type:          definition.PluginSettingValueTableColumnTypeSelect,
							Width:         120,
							SelectOptions: getCommandInterpreterOptions(),
						},
						{
							Key:     "WorkingDirectory",
							Label:   "i18n:plugin_shell_command_working_directory",
							Tooltip: "i18n:plugin_shell_command_working_directory_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeDirPath,
							Width:   180,
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

// validateShellCommandAliases keeps command lookup deterministic because aliases are matched case-insensitively.
func validateShellCommandAliases(ctx context.Context, commands []shellCommand) error {
	seen := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		alias := strings.ToLower(strings.TrimSpace(command.Alias))
		if alias == "" {
			continue
		}
		if _, exists := seen[alias]; exists {
			return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "ui_validator_value_must_be_unique"))
		}
		seen[alias] = struct{}{}
	}

	return nil
}

// getCommandInterpreterOptions includes the global default option for saved commands.
func getCommandInterpreterOptions() []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{Label: "i18n:plugin_shell_command_interpreter_default", Value: ""},
	}
	return append(options, getInterpreterOptions()...)
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

// requiredTextValidator returns the shared non-empty validator for shell command forms.
func requiredTextValidator() []validator.PluginSettingValidator {
	return []validator.PluginSettingValidator{
		{
			Type:  validator.PluginSettingValidatorTypeNotEmpty,
			Value: &validator.PluginSettingValidatorNotEmpty{},
		},
	}
}

// effectiveInterpreter resolves command-level interpreters before falling back to the global shell setting.
func effectiveInterpreter(interpreter string, fallback string) string {
	interpreter = strings.TrimSpace(interpreter)
	if interpreter != "" {
		return interpreter
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return getDefaultInterpreter()
}

// commandDisplayTitle derives a compact default title from a command.
func commandDisplayTitle(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if len([]rune(command)) <= 48 {
		return command
	}
	return string([]rune(command)[:48])
}

// middleEllipsis keeps both ends of long paths visible in compact result tails.
func middleEllipsis(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 || len([]rune(text)) <= maxRunes {
		return text
	}
	if maxRunes <= 3 {
		return strings.Repeat(".", maxRunes)
	}

	runes := []rune(text)
	leftCount := (maxRunes - 3 + 1) / 2
	rightCount := maxRunes - 3 - leftCount
	return string(runes[:leftCount]) + "..." + string(runes[len(runes)-rightCount:])
}

// displayTitleForCommand prefers the user-provided title when rendering command execution results.
func displayTitleForCommand(data shellContextData) string {
	title := strings.TrimSpace(data.Title)
	if title != "" {
		return title
	}
	return data.Command
}

// buildShellPreviewTags builds the metadata chips shown below shell previews.
func (s *ShellPlugin) buildShellPreviewTags(ctx context.Context, data shellContextData) []plugin.WoxPreviewTag {
	interpreter := effectiveInterpreter(data.Interpreter, "")
	workingDirectory := strings.TrimSpace(data.WorkingDirectory)
	if workingDirectory == "" {
		if currentDirectory, err := os.Getwd(); err == nil {
			workingDirectory = currentDirectory
		} else {
			workingDirectory = "i18n:plugin_shell_default_working_directory"
		}
	}

	tags := []plugin.WoxPreviewTag{
		{Label: interpreter, Tooltip: "i18n:plugin_shell_property_interpreter"},
		{Label: workingDirectory, Tooltip: "i18n:plugin_shell_property_working_directory"},
	}
	if data.Background {
		tags = append(tags, plugin.WoxPreviewTag{Label: i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_background"), Tooltip: "i18n:plugin_shell_property_execution_mode"})
	}
	return tags
}

// shellContextDataFromActionContext keeps reused action callbacks aligned with the latest result context.
func shellContextDataFromActionContext(actionContext plugin.ActionContext, fallback shellContextData) shellContextData {
	data := fallback
	if actionContext.ContextData == nil {
		return data
	}

	if command := strings.TrimSpace(actionContext.ContextData[shellActionCommandKey]); command != "" {
		data.Command = command
		data.Title = strings.TrimSpace(actionContext.ContextData[shellActionTitleKey])
	} else if title, ok := actionContext.ContextData[shellActionTitleKey]; ok {
		data.Title = strings.TrimSpace(title)
	}
	if interpreter := strings.TrimSpace(actionContext.ContextData[shellActionInterpreterKey]); interpreter != "" {
		data.Interpreter = interpreter
	}
	if workingDirectory, ok := actionContext.ContextData[shellActionWorkingDirKey]; ok {
		data.WorkingDirectory = strings.TrimSpace(workingDirectory)
	}
	if historyID := strings.TrimSpace(actionContext.ContextData[shellActionHistoryIDKey]); historyID != "" {
		data.HistoryID = historyID
	}
	if commandIndex, ok := actionContext.ContextData[shellActionCommandIndexKey]; ok {
		if index, err := strconv.Atoi(commandIndex); err == nil {
			data.IsSavedCommand = true
			data.SavedCommandIndex = index
		}
	}
	return data
}

func (s *ShellPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	s.api = initParams.API
	s.historyManager = NewShellHistoryManager()
	s.terminalManager = terminal.GetSessionManager()
	s.api.OnHandlePluginCommand(ctx, s.handlePluginCommand)

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

// handlePluginCommand handles plugin-to-plugin commands exposed by Shell.
func (s *ShellPlugin) handlePluginCommand(ctx context.Context, request plugin.PluginCommandRequest) plugin.PluginCommandResult {
	if request.Command != PluginCommandPrepareCommandAtDirectory {
		return plugin.PluginCommandResult{Handled: false}
	}

	workingDirectory := strings.TrimSpace(request.Data[PluginCommandDataWorkingDirectory])
	if workingDirectory == "" {
		return plugin.PluginCommandResult{Handled: true, Message: "working directory is required"}
	}
	resolvedDirectory, ok := s.resolveWorkingDirectory(ctx, workingDirectory, false)
	if !ok {
		return plugin.PluginCommandResult{Handled: true, Message: fmt.Sprintf("invalid working directory: %s", workingDirectory)}
	}

	s.api.ChangeQuery(ctx, common.PlainQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: "> ",
		ContextData: common.ContextData{
			QueryContextWorkingDirectoryKey: resolvedDirectory,
		},
	})
	return plugin.PluginCommandResult{Handled: true}
}

// resolveWorkingDirectory validates a user/plugin-provided working directory before execution.
func (s *ShellPlugin) resolveWorkingDirectory(ctx context.Context, workingDirectory string, notify bool) (string, bool) {
	workingDirectory = strings.TrimSpace(workingDirectory)
	if workingDirectory == "" {
		return "", true
	}

	cleaned, cleanErr := filepath.Abs(workingDirectory)
	if cleanErr != nil {
		cleaned = filepath.Clean(workingDirectory)
	}
	info, statErr := os.Stat(cleaned)
	if statErr != nil || !info.IsDir() {
		message := fmt.Sprintf("invalid shell working directory: %s", workingDirectory)
		if statErr != nil {
			message = fmt.Sprintf("%s (%s)", message, statErr.Error())
		}
		s.api.Log(ctx, plugin.LogLevelWarning, message)
		if notify {
			s.api.Notify(ctx, message)
		}
		return "", false
	}

	return cleaned, true
}

// buildEditCommandAction updates a saved command or prepares a one-off edited run.
func (s *ShellPlugin) buildEditCommandAction(data shellContextData) plugin.QueryResultAction {
	form := s.buildCommandEditForm(data.Title, data.Command, data.Interpreter, data.WorkingDirectory)
	if data.IsSavedCommand {
		form = s.buildSavedCommandEditForm(data.Title, data.Command, data.Interpreter, data.WorkingDirectory)
	}

	return plugin.QueryResultAction{
		Id:                     "edit_command",
		Name:                   "i18n:plugin_shell_edit_command",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		ContextData:            s.buildActionContextDataForCommand(data),
		Form:                   form,
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			currentData := shellContextDataFromActionContext(actionContext.ActionContext, data)
			nextData := shellContextData{
				Title:             strings.TrimSpace(actionContext.Values[shellFormTitleKey]),
				Command:           strings.TrimSpace(actionContext.Values[shellFormCommandKey]),
				Interpreter:       effectiveInterpreter(actionContext.Values[shellFormInterpreterKey], currentData.Interpreter),
				WorkingDirectory:  strings.TrimSpace(actionContext.Values[shellFormWorkingDirKey]),
				HistoryID:         currentData.HistoryID,
				IsSavedCommand:    currentData.IsSavedCommand,
				SavedCommandIndex: currentData.SavedCommandIndex,
			}
			if nextData.Command == "" {
				s.api.Notify(ctx, "i18n:plugin_shell_command_body_required")
				return
			}
			if currentData.IsSavedCommand {
				nextData.Interpreter = strings.TrimSpace(actionContext.Values[shellFormInterpreterKey])
				if err := s.updateConfiguredCommand(ctx, currentData.SavedCommandIndex, nextData); err != nil {
					s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update shell command: %s", err.Error()))
					s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_update_failed"), err.Error()))
					return
				}
				s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_updated"))
				s.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				return
			}
			util.Go(ctx, "execute edited shell command", func() {
				s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, nextData)
			})
		},
	}
}

// buildAddCommandAction persists the current command as a configured Shell command.
func (s *ShellPlugin) buildAddCommandAction(data shellContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "add_as_command",
		Name:                   "i18n:plugin_shell_add_as_command",
		Icon:                   common.PinIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		ContextData:            s.buildActionContextDataForCommand(data),
		Form:                   s.buildAddCommandForm(data.Title, data.Command, data.Interpreter, data.WorkingDirectory),
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			currentData := shellContextDataFromActionContext(actionContext.ActionContext, data)
			command := shellCommand{
				Alias:            strings.TrimSpace(actionContext.Values[shellFormTitleKey]),
				Command:          strings.TrimSpace(actionContext.Values[shellFormCommandKey]),
				Interpreter:      strings.TrimSpace(actionContext.Values[shellFormInterpreterKey]),
				WorkingDirectory: strings.TrimSpace(actionContext.Values[shellFormWorkingDirKey]),
				Enabled:          true,
				Silent:           actionContext.Values[shellFormSilentKey] == "true",
			}
			if command.Command == "" {
				command.Command = currentData.Command
			}
			if command.Alias == "" || command.Command == "" {
				s.api.Notify(ctx, "i18n:plugin_shell_command_required")
				return
			}
			if err := s.addConfiguredCommand(ctx, command); err != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to add shell command: %s", err.Error()))
				s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_add_failed"), err.Error()))
				return
			}
			s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_added"))
			s.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	}
}

// buildRunWithInterpreterAction reruns the current command with a selected interpreter.
func (s *ShellPlugin) buildRunWithInterpreterAction(data shellContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "run_with_interpreter",
		Name:                   "i18n:plugin_shell_run_with_interpreter",
		Icon:                   shellIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		ContextData:            s.buildActionContextDataForCommand(data),
		Form:                   s.buildRunWithInterpreterForm(data.Interpreter),
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			currentData := shellContextDataFromActionContext(actionContext.ActionContext, data)
			interpreter := effectiveInterpreter(actionContext.Values[shellFormInterpreterKey], currentData.Interpreter)
			if strings.TrimSpace(currentData.Command) == "" {
				s.api.Notify(ctx, "i18n:plugin_shell_command_body_required")
				return
			}
			util.Go(ctx, "execute shell command with selected interpreter", func() {
				s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, shellContextData{
					Title:            currentData.Title,
					Command:          currentData.Command,
					Interpreter:      interpreter,
					WorkingDirectory: currentData.WorkingDirectory,
					HistoryID:        currentData.HistoryID,
				})
			})
		},
	}
}

// buildDeleteConfiguredCommandAction removes a saved shell command from settings.
func (s *ShellPlugin) buildDeleteConfiguredCommandAction(data shellContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "delete_command",
		Name:                   "i18n:plugin_shell_delete_command",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		ContextData:            s.buildActionContextDataForCommand(data),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			currentData := shellContextDataFromActionContext(actionContext, data)
			if !currentData.IsSavedCommand {
				return
			}
			if err := s.deleteConfiguredCommand(ctx, currentData.SavedCommandIndex); err != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to delete shell command: %s", err.Error()))
				s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_delete_failed"), err.Error()))
				return
			}
			s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_deleted"))
			s.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: false})
		},
	}
}

// buildRunWithInterpreterForm builds the interpreter selector used for one-off reruns.
func (s *ShellPlugin) buildRunWithInterpreterForm(interpreter string) definition.PluginSettingDefinitions {
	return definition.PluginSettingDefinitions{
		{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          shellFormInterpreterKey,
				Label:        "i18n:plugin_shell_form_interpreter",
				DefaultValue: effectiveInterpreter(interpreter, ""),
				Options:      getInterpreterOptions(),
			},
		},
	}
}

// buildCommandEditForm builds the form used for one-off command editing.
func (s *ShellPlugin) buildCommandEditForm(title string, command string, interpreter string, workingDirectory string) definition.PluginSettingDefinitions {
	defaultTitle := strings.TrimSpace(title)
	if defaultTitle == "" {
		defaultTitle = commandDisplayTitle(command)
	}
	return definition.PluginSettingDefinitions{
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormTitleKey,
				Label:        "i18n:plugin_shell_form_title",
				DefaultValue: defaultTitle,
				Tooltip:      "i18n:plugin_shell_form_title_tooltip",
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          shellFormInterpreterKey,
				Label:        "i18n:plugin_shell_form_interpreter",
				DefaultValue: effectiveInterpreter(interpreter, ""),
				Options:      getInterpreterOptions(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormCommandKey,
				Label:        "i18n:plugin_shell_form_command",
				DefaultValue: command,
				MaxLines:     5,
				Validators:   requiredTextValidator(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormWorkingDirKey,
				Label:        "i18n:plugin_shell_form_working_directory",
				DefaultValue: strings.TrimSpace(workingDirectory),
				Tooltip:      "i18n:plugin_shell_form_working_directory_tooltip",
			},
		},
	}
}

// buildSavedCommandEditForm builds the form used to update a command already stored in settings.
func (s *ShellPlugin) buildSavedCommandEditForm(title string, command string, interpreter string, workingDirectory string) definition.PluginSettingDefinitions {
	defaultTitle := strings.TrimSpace(title)
	if defaultTitle == "" {
		defaultTitle = commandDisplayTitle(command)
	}
	return definition.PluginSettingDefinitions{
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormTitleKey,
				Label:        "i18n:plugin_shell_form_command_title",
				DefaultValue: defaultTitle,
				Validators:   requiredTextValidator(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          shellFormInterpreterKey,
				Label:        "i18n:plugin_shell_form_interpreter",
				DefaultValue: strings.TrimSpace(interpreter),
				Options:      getCommandInterpreterOptions(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormCommandKey,
				Label:        "i18n:plugin_shell_form_command",
				DefaultValue: command,
				MaxLines:     5,
				Validators:   requiredTextValidator(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormWorkingDirKey,
				Label:        "i18n:plugin_shell_form_working_directory",
				DefaultValue: strings.TrimSpace(workingDirectory),
				Tooltip:      "i18n:plugin_shell_form_working_directory_tooltip",
			},
		},
	}
}

// buildAddCommandForm builds the form used to save a command shortcut.
func (s *ShellPlugin) buildAddCommandForm(title string, command string, interpreter string, workingDirectory string) definition.PluginSettingDefinitions {
	alias := strings.TrimSpace(title)
	if alias == "" {
		alias = commandDisplayTitle(command)
	}
	return definition.PluginSettingDefinitions{
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormTitleKey,
				Label:        "i18n:plugin_shell_form_command_title",
				DefaultValue: alias,
				Validators:   requiredTextValidator(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          shellFormInterpreterKey,
				Label:        "i18n:plugin_shell_form_interpreter",
				DefaultValue: strings.TrimSpace(interpreter),
				Options:      getCommandInterpreterOptions(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormCommandKey,
				Label:        "i18n:plugin_shell_form_command",
				DefaultValue: command,
				MaxLines:     5,
				Validators:   requiredTextValidator(),
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          shellFormWorkingDirKey,
				Label:        "i18n:plugin_shell_form_working_directory",
				DefaultValue: strings.TrimSpace(workingDirectory),
				Tooltip:      "i18n:plugin_shell_form_working_directory_tooltip",
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeCheckBox,
			Value: &definition.PluginSettingValueCheckBox{
				Key:          shellFormSilentKey,
				Label:        "i18n:plugin_shell_command_silent",
				DefaultValue: "false",
				Tooltip:      "i18n:plugin_shell_command_silent_tooltip",
			},
		},
	}
}

// addConfiguredCommand appends a shell command to the plugin setting table.
func (s *ShellPlugin) addConfiguredCommand(ctx context.Context, command shellCommand) error {
	commands := s.loadCommands(ctx)
	commands = append(commands, command)
	return s.saveConfiguredCommands(ctx, commands)
}

// updateConfiguredCommand updates an existing saved command while preserving flags not exposed in the edit form.
func (s *ShellPlugin) updateConfiguredCommand(ctx context.Context, commandIndex int, data shellContextData) error {
	commands := s.loadCommands(ctx)
	if commandIndex < 0 || commandIndex >= len(commands) {
		return fmt.Errorf("command index out of range: %d", commandIndex)
	}

	commands[commandIndex].Alias = strings.TrimSpace(data.Title)
	commands[commandIndex].Command = strings.TrimSpace(data.Command)
	commands[commandIndex].Interpreter = strings.TrimSpace(data.Interpreter)
	commands[commandIndex].WorkingDirectory = strings.TrimSpace(data.WorkingDirectory)
	if commands[commandIndex].Alias == "" || commands[commandIndex].Command == "" {
		return fmt.Errorf("command title and command cannot be empty")
	}

	return s.saveConfiguredCommands(ctx, commands)
}

// deleteConfiguredCommand removes an existing saved command from settings.
func (s *ShellPlugin) deleteConfiguredCommand(ctx context.Context, commandIndex int) error {
	commands := s.loadCommands(ctx)
	if commandIndex < 0 || commandIndex >= len(commands) {
		return fmt.Errorf("command index out of range: %d", commandIndex)
	}

	commands = append(commands[:commandIndex], commands[commandIndex+1:]...)
	return s.saveConfiguredCommands(ctx, commands)
}

// saveConfiguredCommands writes the Shell command setting table as JSON.
func (s *ShellPlugin) saveConfiguredCommands(ctx context.Context, commands []shellCommand) error {
	if err := validateShellCommandAliases(ctx, commands); err != nil {
		return err
	}

	data, err := json.Marshal(commands)
	if err != nil {
		return err
	}
	s.api.SaveSetting(ctx, shellCommandsSettingKey, string(data), false)
	return nil
}

// refreshCommandActionForms keeps reused form actions in sync with the latest command context.
func (s *ShellPlugin) refreshCommandActionForms(actions []plugin.QueryResultAction, data shellContextData) {
	for i := range actions {
		switch actions[i].Id {
		case "edit_command":
			if actions[i].ContextData != nil {
				if _, ok := actions[i].ContextData[shellActionCommandIndexKey]; ok {
					currentData := shellContextDataFromActionContext(plugin.ActionContext{ContextData: actions[i].ContextData}, data)
					actions[i].Form = s.buildSavedCommandEditForm(currentData.Title, currentData.Command, currentData.Interpreter, currentData.WorkingDirectory)
					continue
				}
			}
			actions[i].Form = s.buildCommandEditForm(data.Title, data.Command, data.Interpreter, data.WorkingDirectory)
		case "add_as_command":
			actions[i].Form = s.buildAddCommandForm(data.Title, data.Command, data.Interpreter, data.WorkingDirectory)
		case "run_with_interpreter":
			actions[i].Form = s.buildRunWithInterpreterForm(data.Interpreter)
		}
	}
}

// getHistoryResultGroup groups shell history by recency, matching clipboard-style date buckets.
func (s *ShellPlugin) getHistoryResultGroup(timestamp int64) (string, int64) {
	now := util.GetSystemTimestamp()
	if now-timestamp < 1000*60*60*24 {
		return "i18n:plugin_shell_group_today", 90
	}
	if now-timestamp < 1000*60*60*24*2 {
		return "i18n:plugin_shell_group_yesterday", 80
	}
	return "i18n:plugin_shell_group_history", 10
}

// getHistoryResultScore keeps default shell history ordered by most recent first.
func (s *ShellPlugin) getHistoryResultScore(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	return timestamp / 1000
}

// buildCommandLastRunSubtitle renders the saved command's latest execution time.
func (s *ShellPlugin) buildCommandLastRunSubtitle(ctx context.Context, title string, interpreter string, workingDirectory string) string {
	history, err := s.historyManager.GetLatestCommandRun(ctx, strings.TrimSpace(title), strings.TrimSpace(interpreter), strings.TrimSpace(workingDirectory))
	if err != nil || history == nil || history.StartTime <= 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_never_executed")
	}

	if history.Status == "running" {
		return s.formatRunningElapsedSubtitle(ctx, time.Duration(util.GetSystemTimestamp()-history.StartTime)*time.Millisecond)
	}

	lastRun := time.Unix(history.StartTime/1000, 0).Format("2006-01-02 15:04:05")
	return fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_command_last_run"), lastRun)
}

// buildGlobalCommandTails exposes shell metadata for saved commands shown in global search.
func (s *ShellPlugin) buildGlobalCommandTails(ctx context.Context, interpreter string, workingDirectory string, background bool) []plugin.QueryResultTail {
	tails := []plugin.QueryResultTail{
		{
			Type:    plugin.QueryResultTailTypeText,
			Text:    strings.TrimSpace(interpreter),
			Tooltip: i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_interpreter"),
		},
	}

	if strings.TrimSpace(workingDirectory) != "" {
		tails = append(tails, plugin.QueryResultTail{
			Type:    plugin.QueryResultTailTypeText,
			Text:    middleEllipsis(workingDirectory, 28),
			Tooltip: strings.TrimSpace(workingDirectory),
		})
	}

	if background {
		tails = append(tails, plugin.QueryResultTail{
			Type:    plugin.QueryResultTailTypeText,
			Text:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_background"),
			Tooltip: i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_property_execution_mode"),
		})
	}
	return tails
}

func (s *ShellPlugin) queryHistory(ctx context.Context, interpreter string, showPlaceholder bool) []plugin.QueryResult {
	// Get recent history from database
	histories, err := s.historyManager.GetRecentHistory(ctx, 10)
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to get history: %s", err.Error()))
		if !showPlaceholder {
			return nil
		}
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
		if !showPlaceholder {
			return nil
		}
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

		title := s.buildSessionTitle(ctx, displayTitleForCommand(shellContextData{Title: history.Title, Command: history.Command}), history.Status)
		subTitle := time.Unix(history.StartTime/1000, 0).Format("2006-01-02 15:04:05")
		historyInterpreter := effectiveInterpreter(history.Interpreter, interpreter)
		historyContextData := shellContextData{
			Title:            history.Title,
			Command:          history.Command,
			Interpreter:      historyInterpreter,
			WorkingDirectory: history.WorkingDirectory,
			HistoryID:        history.ID,
			FromHistory:      true,
			Background:       history.Background,
		}

		// Build actions based on status
		var actions []plugin.QueryResultAction

		// Always add re-execute action
		actions = append(actions, plugin.QueryResultAction{
			Id:                     "reexecute",
			Name:                   "i18n:plugin_shell_reexecute",
			Icon:                   common.UpdateIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, historyInterpreter, history.Title, history.WorkingDirectory),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				currentData := shellContextDataFromActionContext(actionContext, historyContextData)
				if s.stopSessionByHistoryID(ctx, currentData.HistoryID) {
					return
				}

				util.Go(ctx, "re-execute shell command from history", func() {
					s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, currentData)
				})
			},
		})
		actions = append(actions, s.buildEditCommandAction(historyContextData), s.buildAddCommandAction(historyContextData), s.buildRunWithInterpreterAction(historyContextData))

		// Only add stop action if command is still running
		if history.Status == "running" {
			actions = append(actions, plugin.QueryResultAction{
				Id:                     "stop",
				Name:                   "i18n:plugin_shell_stop",
				Icon:                   common.TerminateAppIcon,
				PreventHideAfterAction: true,
				ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, historyInterpreter, history.Title, history.WorkingDirectory),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					currentData := shellContextDataFromActionContext(actionContext, historyContextData)
					s.stopSessionByHistoryID(ctx, currentData.HistoryID)
				},
			})
		}

		actions = append(actions, plugin.QueryResultAction{
			Id:                     "delete",
			Name:                   "i18n:plugin_shell_delete",
			Icon:                   common.TrashIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData(history.SessionID, history.ID, history.Command, historyInterpreter, history.Title, history.WorkingDirectory),
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
		group, groupScore := s.getHistoryResultGroup(history.StartTime)

		results = append(results, plugin.QueryResult{
			Title:      title,
			SubTitle:   subTitle,
			Icon:       shellIcon,
			Score:      s.getHistoryResultScore(history.StartTime),
			Group:      group,
			GroupScore: groupScore,
			Preview: plugin.WoxPreview{
				PreviewType: previewType,
				PreviewData: previewData,
				PreviewTags: s.buildShellPreviewTags(ctx, historyContextData),
			},
			Actions: actions,
		})
	}

	return results
}

func (s *ShellPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	// Get the configured interpreter
	interpreter := s.api.GetSetting(ctx, shellInterpreterSettingKey)
	if interpreter == "" {
		interpreter = getDefaultInterpreter()
	}

	// Handle global query - check for shell commands
	if query.IsGlobalQuery() {
		return plugin.NewQueryResponse(s.queryCommands(ctx, query, interpreter, false))
	}

	// Get the command from the query
	command := strings.TrimSpace(query.Search)

	// If no command entered, show history
	if command == "" {
		commandResults := s.queryCommands(ctx, query, interpreter, true)
		historyResults := s.queryHistory(ctx, interpreter, len(commandResults) == 0)
		return plugin.NewQueryResponse(append(commandResults, historyResults...))
	}

	// Create context data
	workingDirectory := strings.TrimSpace(query.ContextData[QueryContextWorkingDirectoryKey])
	contextData := shellContextData{
		Command:          command,
		Interpreter:      interpreter,
		WorkingDirectory: workingDirectory,
		FromHistory:      false,
	}

	// Build subtitle with interpreter info
	subtitle := fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_with"), interpreter, command)

	actions := []plugin.QueryResultAction{
		{
			Id:                     "execute",
			Name:                   "i18n:plugin_shell_execute",
			Icon:                   common.CorrectIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData("", "", command, interpreter, "", workingDirectory),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if s.stopSessionByResultID(ctx, actionContext.ResultId) {
					return
				}
				currentData := shellContextDataFromActionContext(actionContext, contextData)
				util.Go(ctx, "execute shell command", func() {
					s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, currentData)
				})
			},
		},
		{
			Id:                     "execute_background",
			Name:                   "i18n:plugin_shell_execute_background",
			Icon:                   common.OpenIcon,
			PreventHideAfterAction: false,
			Hotkey:                 util.PrimaryHotkey("enter"),
			ContextData:            s.buildActionContextData("", "", command, interpreter, "", workingDirectory),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				currentData := shellContextDataFromActionContext(actionContext, contextData)
				util.Go(ctx, "execute shell command in background", func() {
					s.executeCommandInBackground(ctx, currentData)
				})
			},
		},
		{
			Id:                     "stop",
			Name:                   "i18n:plugin_shell_stop",
			Icon:                   common.TerminateAppIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData("", "", command, interpreter, "", workingDirectory),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				s.stopSessionByResultID(ctx, actionContext.ResultId)
			},
		},
	}
	actions = append(actions, s.buildEditCommandAction(contextData), s.buildAddCommandAction(contextData), s.buildRunWithInterpreterAction(contextData))
	actions = append(actions,
		plugin.QueryResultAction{
			Id:                     "reexecute",
			Name:                   "i18n:plugin_shell_reexecute",
			Icon:                   common.UpdateIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData("", "", command, interpreter, "", workingDirectory),
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if s.stopSessionByResultID(ctx, actionContext.ResultId) {
					return
				}
				currentData := shellContextDataFromActionContext(actionContext, contextData)
				util.Go(ctx, "re-execute shell command", func() {
					s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, currentData)
				})
			},
		},
		plugin.QueryResultAction{
			Id:                     "delete",
			Name:                   "i18n:plugin_shell_delete",
			Icon:                   common.TrashIcon,
			PreventHideAfterAction: true,
			ContextData:            s.buildActionContextData("", "", command, interpreter, "", workingDirectory),
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
	)

	return plugin.NewQueryResponse([]plugin.QueryResult{
		{
			Title:    command,
			SubTitle: subtitle,
			Icon:     shellIcon,
			Score:    100,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeText,
				PreviewData:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_enter_to_execute"),
				PreviewTags:    s.buildShellPreviewTags(ctx, contextData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: actions,
		},
	})
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

// queryCommands searches configured shell commands or returns all of them for the default Shell view.
func (s *ShellPlugin) queryCommands(ctx context.Context, query plugin.Query, interpreter string, includeAll bool) []plugin.QueryResult {
	commands := s.loadCommands(ctx)
	if len(commands) == 0 {
		return nil
	}

	search := strings.TrimSpace(query.Search)
	if search == "" && !includeAll {
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
	for commandIndex, cmd := range commands {
		if !cmd.Enabled {
			continue
		}

		// Match alias (case-insensitive, prefix match for better UX)
		if searchAlias != "" && !strings.HasPrefix(strings.ToLower(cmd.Alias), searchAlias) {
			continue
		}

		// Replace {query} placeholder with query parameter
		finalCommand := strings.ReplaceAll(cmd.Command, "{query}", queryParam)
		commandInterpreter := effectiveInterpreter(cmd.Interpreter, interpreter)

		// Create context data for execution
		contextData := shellContextData{
			Title:            cmd.Alias,
			Command:          finalCommand,
			Interpreter:      commandInterpreter,
			WorkingDirectory: strings.TrimSpace(cmd.WorkingDirectory),
			FromHistory:      false,
			Background:       cmd.Silent,
		}
		savedCommandData := shellContextData{
			Title:             cmd.Alias,
			Command:           cmd.Command,
			Interpreter:       strings.TrimSpace(cmd.Interpreter),
			WorkingDirectory:  strings.TrimSpace(cmd.WorkingDirectory),
			FromHistory:       false,
			IsSavedCommand:    true,
			SavedCommandIndex: commandIndex,
		}

		subtitle := s.buildCommandLastRunSubtitle(ctx, cmd.Alias, commandInterpreter, strings.TrimSpace(cmd.WorkingDirectory))

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
					ContextData:            s.buildActionContextData("", "", finalCommand, commandInterpreter, cmd.Alias, cmd.WorkingDirectory),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						currentData := shellContextDataFromActionContext(actionContext, contextData)
						util.Go(ctx, "execute shell command in background", func() {
							s.executeCommandInBackground(ctx, currentData)
						})
					},
				},
			}
		} else {
			// Non-silent mode: execute in place so command-specific interpreters are preserved.
			actions = []plugin.QueryResultAction{
				{
					Id:                     "execute",
					Name:                   "i18n:plugin_shell_execute",
					Icon:                   common.CorrectIcon,
					PreventHideAfterAction: true,
					ContextData:            s.buildActionContextData("", "", finalCommand, commandInterpreter, cmd.Alias, cmd.WorkingDirectory),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if s.stopSessionByResultID(ctx, actionContext.ResultId) {
							return
						}
						currentData := shellContextDataFromActionContext(actionContext, contextData)
						util.Go(ctx, "execute configured shell command", func() {
							s.executeCommandWithUpdateResult(ctx, actionContext.ResultId, currentData)
						})
					},
				},
				{
					Id:                     "execute_background",
					Name:                   "i18n:plugin_shell_execute_background",
					Icon:                   common.OpenIcon,
					PreventHideAfterAction: false,
					Hotkey:                 util.PrimaryHotkey("enter"),
					ContextData:            s.buildActionContextData("", "", finalCommand, commandInterpreter, cmd.Alias, cmd.WorkingDirectory),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						currentData := shellContextDataFromActionContext(actionContext, contextData)
						util.Go(ctx, "execute shell command in background", func() {
							s.executeCommandInBackground(ctx, currentData)
						})
					},
				},
			}
		}
		actions = append(actions, s.buildEditCommandAction(savedCommandData), s.buildDeleteConfiguredCommandAction(savedCommandData), s.buildRunWithInterpreterAction(contextData))

		result := plugin.QueryResult{
			Title:    cmd.Alias,
			SubTitle: subtitle,
			Icon:     shellIcon,
			Score:    100 + int64(len(commands)-commandIndex),
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeTerminal,
				PreviewData:    s.buildTerminalPreviewData("", finalCommand, "idle"),
				PreviewTags:    s.buildShellPreviewTags(ctx, contextData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: actions,
		}
		if includeAll {
			result.Group = "i18n:plugin_shell_commands"
			result.GroupScore = 100
		} else {
			result.Tails = s.buildGlobalCommandTails(ctx, commandInterpreter, strings.TrimSpace(cmd.WorkingDirectory), cmd.Silent)
		}
		results = append(results, result)
	}

	return results
}

// executeCommandWithUpdateResult executes a shell command and updates metadata via UpdateResult.
func (s *ShellPlugin) executeCommandWithUpdateResult(ctx context.Context, resultId string, data shellContextData) {
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command: %s with interpreter: %s", data.Command, data.Interpreter))
	if resolvedDirectory, ok := s.resolveWorkingDirectory(ctx, data.WorkingDirectory, true); ok {
		data.WorkingDirectory = resolvedDirectory
	} else {
		data.WorkingDirectory = ""
	}

	session, err := s.terminalManager.CreateSession(ctx, terminal.CreateSessionParams{
		Command:          data.Command,
		Interpreter:      data.Interpreter,
		WorkingDirectory: data.WorkingDirectory,
	})
	if err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create terminal session: %s", err.Error()))
		return
	}

	startTs := util.GetSystemTimestamp()
	historyID := data.HistoryID
	if data.HistoryID != "" {
		if resetErr := s.historyManager.ResetForReexecute(ctx, data.HistoryID, session.ID, data.Title, data.Command, data.Interpreter, data.WorkingDirectory, startTs, session.OutputPath); resetErr != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to reset shell history for re-execute (id=%s): %s", data.HistoryID, resetErr.Error()))
			historyID = ""
		}
	}
	if historyID == "" {
		historyID = uuid.NewString()
		createErr := s.historyManager.Create(ctx, &ShellHistory{
			ID:               historyID,
			SessionID:        session.ID,
			Title:            data.Title,
			Command:          data.Command,
			Interpreter:      data.Interpreter,
			WorkingDirectory: data.WorkingDirectory,
			Status:           "running",
			StartTime:        startTs,
			OutputSummary:    "",
			OutputPath:       session.OutputPath,
		})
		if createErr != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create shell history: %s", createErr.Error()))
		}
	}

	cmd := s.buildCommand(ctx, data.Interpreter, data.Command, data.WorkingDirectory)
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
		title := s.buildSessionTitle(ctx, displayTitleForCommand(data), status)
		subTitle := startTime.Format("2006-01-02 15:04:05")
		if isRunning {
			subTitle = s.formatRunningElapsedSubtitle(ctx, time.Since(startTime))
		}

		preview := plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeTerminal,
			PreviewData:    s.buildTerminalPreviewData(session.ID, data.Command, status),
			PreviewTags:    s.buildShellPreviewTags(ctx, data),
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
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
				if _, isSavedCommandAction := actions[i].ContextData[shellActionCommandIndexKey]; isSavedCommandAction {
					continue
				}
				actions[i].ContextData[shellActionCommandKey] = data.Command
				actions[i].ContextData[shellActionInterpreterKey] = data.Interpreter
				if strings.TrimSpace(data.WorkingDirectory) != "" {
					actions[i].ContextData[shellActionWorkingDirKey] = data.WorkingDirectory
				} else {
					delete(actions[i].ContextData, shellActionWorkingDirKey)
				}
				if strings.TrimSpace(data.Title) != "" {
					actions[i].ContextData[shellActionTitleKey] = data.Title
				} else {
					delete(actions[i].ContextData, shellActionTitleKey)
				}
			}
			s.refreshCommandActionForms(actions, data)
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
		s.notifyCommandFinished(ctx, data, "failed", 1)
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
		s.notifyCommandFinished(ctx, data, "failed", 1)
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
		s.notifyCommandFinished(ctx, data, "failed", 1)
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
	s.notifyCommandFinished(ctx, data, historyStatus, exitCode)
}

func (s *ShellPlugin) executeCommandInBackground(ctx context.Context, data shellContextData) {
	data.Background = true
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command in background: %s with interpreter: %s", data.Command, data.Interpreter))
	if resolvedDirectory, ok := s.resolveWorkingDirectory(ctx, data.WorkingDirectory, true); ok {
		data.WorkingDirectory = resolvedDirectory
	} else {
		data.WorkingDirectory = ""
	}

	cmd := s.buildCommand(ctx, data.Interpreter, data.Command, data.WorkingDirectory)
	setCommandProcessGroup(cmd)

	// Start command in background without waiting
	if err := cmd.Start(); err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to start background command: %s", err.Error()))
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_failed"), err.Error()))
		return
	}

	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Background command started with PID: %d", cmd.Process.Pid))
	s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_background_started"))

	historyID := uuid.NewString()
	startTime := time.Now()
	historyCreated := true
	if err := s.historyManager.Create(ctx, &ShellHistory{
		ID:               historyID,
		SessionID:        historyID,
		Title:            data.Title,
		Command:          data.Command,
		Interpreter:      data.Interpreter,
		WorkingDirectory: data.WorkingDirectory,
		Background:       true,
		Status:           "running",
		StartTime:        startTime.UnixMilli(),
	}); err != nil {
		historyCreated = false
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create background shell history: %s", err.Error()))
	}

	// Optionally wait for completion in background and log result
	util.Go(ctx, "wait for background command", func() {
		err := cmd.Wait()
		endTime := time.Now()
		status := "completed"
		exitCode := 0
		if err != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Background command failed: %s", err.Error()))
			status = "failed"
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		} else {
			s.api.Log(ctx, plugin.LogLevelInfo, "Background command completed successfully")
		}
		if historyCreated {
			if updateErr := s.historyManager.UpdateStatus(ctx, historyID, status, exitCode, endTime.UnixMilli(), endTime.Sub(startTime).Milliseconds()); updateErr != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update background shell history: %s", updateErr.Error()))
			}
		}
		s.notifyCommandFinished(ctx, data, status, exitCode)
	})
}

func (s *ShellPlugin) buildCommand(ctx context.Context, interpreter string, command string, workingDirectory string) *exec.Cmd {
	command = prepareShellCommand(interpreter, command)

	var cmd *exec.Cmd
	switch interpreter {
	case "powershell":
		cmd = shellutil.BuildCommandContext(ctx, "powershell", nil, "-Command", command)
	case "cmd":
		cmd = shellutil.BuildCommandContext(ctx, "cmd", nil, "/C", command)
	case "bash":
		cmd = shellutil.BuildCommandContext(ctx, "bash", nil, "-c", command)
	case "zsh":
		cmd = shellutil.BuildCommandContext(ctx, "zsh", nil, "-c", command)
	case "sh":
		cmd = shellutil.BuildCommandContext(ctx, "sh", nil, "-c", command)
	case "python", "python3":
		cmd = shellutil.BuildCommandContext(ctx, interpreter, nil, "-c", command)
	case "node":
		cmd = shellutil.BuildCommandContext(ctx, "node", nil, "-e", command)
	default:
		cmd = shellutil.BuildCommandContext(ctx, interpreter, nil, "-c", command)
	}
	if strings.TrimSpace(workingDirectory) != "" {
		cmd.Dir = strings.TrimSpace(workingDirectory)
	}
	return cmd
}

func (s *ShellPlugin) pipeOutputToSession(ctx context.Context, reader io.Reader, state *shellExecutionState) {
	bufReader := bufio.NewReader(reader)
	for {
		lineBytes, err := bufReader.ReadBytes('\n')
		if len(lineBytes) > 0 {
			// Bug fix: Windows console commands commonly emit bytes in the active
			// OEM code page instead of UTF-8. Converting those bytes directly to a
			// string left invalid UTF-8 in terminal state, and JSON serialization
			// replaced Chinese text with U+FFFD. Decode at the shell boundary so
			// history, the ring buffer, and live preview all store valid UTF-8.
			line := decodeShellOutputChunk(lineBytes)
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

func (s *ShellPlugin) buildActionContextData(sessionID string, historyID string, command string, interpreter string, title string, workingDirectory string) map[string]string {
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
	if title != "" {
		contextData[shellActionTitleKey] = title
	}
	if workingDirectory != "" {
		contextData[shellActionWorkingDirKey] = workingDirectory
	}
	return contextData
}

// buildActionContextDataForCommand includes saved-command identity when the action mutates command settings.
func (s *ShellPlugin) buildActionContextDataForCommand(data shellContextData) map[string]string {
	contextData := s.buildActionContextData("", data.HistoryID, data.Command, data.Interpreter, data.Title, data.WorkingDirectory)
	if data.IsSavedCommand {
		contextData[shellActionCommandIndexKey] = strconv.Itoa(data.SavedCommandIndex)
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

	if history != nil && history.Background {
		sessionID = ""
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

// formatRunningElapsedSubtitle renders the live subtitle for commands that are still running.
func (s *ShellPlugin) formatRunningElapsedSubtitle(ctx context.Context, elapsed time.Duration) string {
	return fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_running_elapsed"), s.formatElapsedDurationForSubtitle(ctx, elapsed))
}

// formatElapsedDurationForSubtitle rounds elapsed runtime into a stable user-facing unit.
func (s *ShellPlugin) formatElapsedDurationForSubtitle(ctx context.Context, elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}

	seconds := (elapsed.Milliseconds() + 999) / 1000
	if seconds <= 0 {
		seconds = 1
	}
	if seconds < 60 {
		return s.formatElapsedDurationUnit(ctx, "second", seconds)
	}

	minutes := seconds / 60
	if minutes < 60 {
		return s.formatElapsedDurationUnit(ctx, "minute", minutes)
	}

	hours := minutes / 60
	if hours < 24 {
		return s.formatElapsedDurationUnit(ctx, "hour", hours)
	}

	return s.formatElapsedDurationUnit(ctx, "day", hours/24)
}

// formatElapsedDurationUnit applies singular/plural translation keys for elapsed runtime.
func (s *ShellPlugin) formatElapsedDurationUnit(ctx context.Context, unit string, value int64) string {
	key := fmt.Sprintf("plugin_shell_elapsed_%s", unit)
	if value != 1 {
		key = key + "s"
	}
	return fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, key), value)
}

// notifyCommandFinished reports foreground and background shell command completion.
func (s *ShellPlugin) notifyCommandFinished(ctx context.Context, data shellContextData, status string, exitCode int) {
	title := displayTitleForCommand(data)
	switch status {
	case "completed":
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_completed_notify"), title))
	case "killed":
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_killed_notify"), title))
	default:
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_failed_notify"), title, exitCode))
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
