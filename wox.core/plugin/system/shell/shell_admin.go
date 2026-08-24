package shell

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
	shellutil "wox/util/shell"

	"github.com/google/uuid"
)

type elevatedShellLaunch struct {
	File       string
	Parameters string
	Directory  string
}

// appendExecuteAsAdministratorAction adds the Windows-only elevated run action.
func (s *ShellPlugin) appendExecuteAsAdministratorAction(actions []plugin.QueryResultAction, data shellContextData) []plugin.QueryResultAction {
	if !util.IsWindows() {
		return actions
	}
	return append(actions, s.buildExecuteAsAdministratorAction(data))
}

// buildExecuteAsAdministratorAction launches the current command through UAC elevation.
func (s *ShellPlugin) buildExecuteAsAdministratorAction(data shellContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "execute_as_administrator",
		Name:                   "i18n:plugin_shell_execute_as_administrator",
		Icon:                   common.PermissionIcon,
		PreventHideAfterAction: false,
		ContextData:            s.buildActionContextData("", data.HistoryID, data.Command, data.Interpreter, data.Title, data.WorkingDirectory),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			currentData := shellContextDataFromActionContext(actionContext, data)
			util.Go(ctx, "execute shell command as administrator", func() {
				s.executeCommandAsAdministrator(ctx, currentData)
			})
		},
	}
}

// executeCommandAsAdministrator hides the launcher, prompts UAC, then runs the command elevated.
func (s *ShellPlugin) executeCommandAsAdministrator(ctx context.Context, data shellContextData) {
	// Hide before ShellExecute so the UAC prompt is not covered by the topmost launcher.
	s.api.HideApp(ctx)
	data.WorkingDirectory = s.resolveExecutionWorkingDirectory(ctx, data.WorkingDirectory, true)

	launch := buildElevatedShellLaunch(data.Interpreter, data.Command, data.WorkingDirectory)
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Executing shell command as administrator: %s with interpreter: %s", data.Command, data.Interpreter))

	wait, err := shellutil.RunElevated(launch.File, launch.Parameters, launch.Directory)
	if err != nil {
		if shellutil.IsElevationCancelled(err) {
			s.api.Log(ctx, plugin.LogLevelInfo, "Administrator elevation cancelled")
			s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_as_administrator_cancelled"))
			return
		}
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to start elevated command: %s", err.Error()))
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_as_administrator_failed"), err.Error()))
		return
	}

	data.Background = true
	s.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_execute_as_administrator_started"))

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
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to create elevated shell history: %s", err.Error()))
	}

	if wait == nil {
		if historyCreated {
			if updateErr := s.historyManager.UpdateStatus(ctx, historyID, "completed", 0, time.Now().UnixMilli(), time.Since(startTime).Milliseconds()); updateErr != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update elevated shell history: %s", updateErr.Error()))
			}
		}
		return
	}

	util.Go(ctx, "wait for elevated command", func() {
		exitCode, waitErr := wait()
		endTime := time.Now()
		status := "completed"
		if waitErr != nil {
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Elevated command failed: %s", waitErr.Error()))
			status = "failed"
			if exitCode == 0 {
				exitCode = 1
			}
		} else if exitCode != 0 {
			status = "failed"
			s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Elevated command exited with code %d", exitCode))
		} else {
			s.api.Log(ctx, plugin.LogLevelInfo, "Elevated command completed successfully")
		}
		if historyCreated {
			if updateErr := s.historyManager.UpdateStatus(ctx, historyID, status, exitCode, endTime.UnixMilli(), endTime.Sub(startTime).Milliseconds()); updateErr != nil {
				s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update elevated shell history: %s", updateErr.Error()))
			}
		}
		s.notifyCommandFinished(ctx, data, status, exitCode)
	})
}

// buildElevatedShellLaunch maps an interpreter and command to a Windows runas ShellExecute launch.
func buildElevatedShellLaunch(interpreter string, command string, workingDirectory string) elevatedShellLaunch {
	interpreter = effectiveInterpreter(interpreter, "")
	command = prepareShellCommand(interpreter, command)
	launch := elevatedShellLaunch{
		File:      elevatedInterpreterFile(interpreter),
		Directory: strings.TrimSpace(workingDirectory),
	}

	switch interpreter {
	case "powershell":
		launch.Parameters = "-NoProfile -EncodedCommand " + encodePowerShellCommand(command)
	case "cmd":
		launch.Parameters = "/C " + quoteWindowsArg(command)
	case "python", "python3":
		launch.Parameters = joinShellExecuteArgs("-c", command)
	case "node":
		launch.Parameters = joinShellExecuteArgs("-e", command)
	default:
		launch.Parameters = joinShellExecuteArgs("-c", command)
	}
	return launch
}

// elevatedInterpreterFile prefers a PATH-resolved executable so the elevated token can still launch it.
func elevatedInterpreterFile(interpreter string) string {
	candidates := []string{interpreter}
	switch interpreter {
	case "powershell":
		candidates = []string{"powershell.exe", "powershell"}
	case "cmd":
		candidates = []string{"cmd.exe", "cmd"}
	case "bash":
		candidates = []string{"bash.exe", "bash"}
	case "python", "python3", "node", "zsh", "sh":
		if util.IsWindows() {
			candidates = []string{interpreter + ".exe", interpreter}
		}
	}

	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return candidates[0]
}

// encodePowerShellCommand base64-encodes UTF-16LE script text for powershell -EncodedCommand.
func encodePowerShellCommand(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, len(units)*2)
	for i, unit := range units {
		raw[i*2] = byte(unit)
		raw[i*2+1] = byte(unit >> 8)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// joinShellExecuteArgs quotes Windows command-line arguments for ShellExecute lpParameters.
func joinShellExecuteArgs(args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteWindowsArg(arg))
	}
	return strings.Join(quoted, " ")
}

// quoteWindowsArg quotes a single Windows command-line argument for C-runtime parsing.
func quoteWindowsArg(s string) string {
	if s == "" {
		return `""`
	}

	needsQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '"':
			needsQuote = true
		}
	}
	if !needsQuote {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s) + 2)
	builder.WriteByte('"')
	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			slashes++
			continue
		}
		if c == '"' {
			builder.WriteString(strings.Repeat(`\`, slashes*2+1))
			builder.WriteByte('"')
			slashes = 0
			continue
		}
		if slashes > 0 {
			builder.WriteString(strings.Repeat(`\`, slashes))
			slashes = 0
		}
		builder.WriteByte(c)
	}
	if slashes > 0 {
		builder.WriteString(strings.Repeat(`\`, slashes*2))
	}
	builder.WriteByte('"')
	return builder.String()
}
