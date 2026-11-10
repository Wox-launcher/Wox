package shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/plugin/system/shell/interactive"
	"wox/plugin/system/shell/terminal"
	"wox/util"
)

// StartInteractiveShell connects completed PTY commands to the Shell history store.
func (s *ShellPlugin) StartInteractiveShell(ctx context.Context, options interactive.Options, size interactive.Size) (*interactive.Session, error) {
	if options.HistoryID != "" {
		if !strings.HasPrefix(options.HistoryID, interactive.HistoryIDPrefix) {
			return nil, fmt.Errorf("invalid interactive shell history ID")
		}
		history, err := s.historyManager.GetByID(ctx, options.HistoryID)
		if err != nil {
			return nil, fmt.Errorf("load interactive shell history: %w", err)
		}
		options.Interpreter = history.Interpreter
		if util.IsWindows() && options.Interpreter != "powershell" ||
			!util.IsWindows() && options.Interpreter != "bash" && options.Interpreter != "zsh" {
			options.Interpreter = getDefaultInterpreter()
		}
		options.Directory, _ = s.resolveWorkingDirectory(ctx, s.effectiveWorkingDirectory(ctx, history.WorkingDirectory), false)
		options.Scrollback = interactiveTranscriptEntry(history.Command, history.OutputSummary)
	}
	firstCommand := options.HistoryID == ""
	var pendingOutput string
	return interactive.Start(ctx, options, size, func(entry interactive.HistoryEntry, command string, exitCode int, output string) bool {
		if firstCommand {
			// The opening command is stored separately and doubles as the history title.
			firstCommand = false
		} else {
			output = "\n" + interactiveTranscriptEntry(command, output)
		}
		pendingOutput = strings.ToValidUTF8(appendSummaryOutput(pendingOutput, output, shellOutputSummaryMaxBytes), "")
		// A window close must not cancel a write for an already completed command.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		endedAt := time.Now()
		status := terminal.SessionStatusCompleted
		if exitCode != 0 {
			status = terminal.SessionStatusFailed
		}
		var err error
		if entry.Saved {
			err = s.historyManager.appendInteractiveOutput(saveCtx, entry.ID, pendingOutput, status, exitCode, endedAt)
		} else {
			err = s.historyManager.Create(saveCtx, &ShellHistory{
				ID: entry.ID, Command: entry.Title, Interpreter: options.Interpreter, WorkingDirectory: options.Directory, OutputSummary: pendingOutput,
				Status: string(status), ExitCode: exitCode, StartTime: entry.StartedAt.UnixMilli(),
				EndTime: endedAt.UnixMilli(), Duration: endedAt.Sub(entry.StartedAt).Milliseconds(),
			})
		}
		if err != nil {
			util.GetLogger().Error(saveCtx, fmt.Sprintf("save interactive shell history: %v", err))
			return false
		}
		pendingOutput = ""
		return true
	})
}

func interactiveTranscriptEntry(command, output string) string {
	return "> " + command + "\n" + output
}
