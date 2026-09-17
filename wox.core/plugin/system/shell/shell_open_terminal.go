package shell

import (
	"context"
	"fmt"
	"os"
	"strings"
	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
)

// visibleProcessLaunch is one detached process used to hand a command to an external terminal.
type visibleProcessLaunch struct {
	File       string
	Args       []string
	Directory  string
	NewConsole bool
}

// buildOpenInSystemTerminalAction hands the current command to the OS terminal app.
func (s *ShellPlugin) buildOpenInSystemTerminalAction(data shellContextData) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "open_in_system_terminal",
		Name:                   "i18n:plugin_shell_open_in_system_terminal",
		Icon:                   icons.Get(icons.ActionOpenInSystemTerminal),
		PreventHideAfterAction: true,
		ContextData:            s.buildActionContextData("", data.HistoryID, data.Command, data.Interpreter, data.Title, data.WorkingDirectory),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			currentData := shellContextDataFromActionContext(actionContext, data)
			s.openCommandInExternalTerminal(ctx, currentData)
		},
	}
}

// openCommandInExternalTerminal hides Wox only after the external terminal starts.
func (s *ShellPlugin) openCommandInExternalTerminal(ctx context.Context, data shellContextData) {
	data.WorkingDirectory = s.resolveExecutionWorkingDirectory(ctx, data.WorkingDirectory, true)
	s.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Opening shell command in external terminal: %s with interpreter: %s", data.Command, data.Interpreter))
	if err := openExternalTerminal(data.Interpreter, data.Command, data.WorkingDirectory); err != nil {
		s.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to open external terminal: %s", err.Error()))
		s.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_shell_open_in_system_terminal_failed"), err.Error()))
		return
	}
	s.api.HideApp(ctx)
}

// posixShellQuote wraps a value for a POSIX shell word.
func posixShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// defaultUnixShell prefers the user's login shell for a handed-off terminal.
func defaultUnixShell() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	return "bash"
}

// unixRunScript builds the command that should run inside an already-open Unix shell.
func unixRunScript(interpreter string, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	interpreter = effectiveInterpreter(interpreter, "")
	switch interpreter {
	case "python", "python3":
		return posixShellQuote(interpreter) + " -c " + posixShellQuote(command)
	case "node":
		return posixShellQuote(interpreter) + " -e " + posixShellQuote(command)
	case "powershell":
		return posixShellQuote(unixPowerShellName()) + " -NoExit -Command " + posixShellQuote(command)
	default:
		return command
	}
}

// unixWorkingDirectoryCommand cds first, then runs the interpreter-specific command.
func unixWorkingDirectoryCommand(directory string, interpreter string, command string) string {
	parts := make([]string, 0, 2)
	if directory = strings.TrimSpace(directory); directory != "" {
		parts = append(parts, "cd "+posixShellQuote(directory))
	}
	if run := unixRunScript(interpreter, command); run != "" {
		parts = append(parts, run)
	}
	return strings.Join(parts, " && ")
}

// unixHoldScript keeps the terminal open after a one-shot command exits.
func unixHoldScript(interpreter string, command string) string {
	run := unixRunScript(interpreter, command)
	if run == "" {
		return ""
	}
	return run + "; exec " + posixShellQuote(defaultUnixShell())
}

func unixPowerShellName() string {
	return "pwsh"
}

func linuxHoldArgs(interpreter string, command string) []string {
	script := unixHoldScript(interpreter, command)
	if script == "" {
		return nil
	}
	return []string{defaultUnixShell(), "-lc", script}
}

func linuxTerminalArgs(base string, interpreter string, command string, directory string) ([]string, string) {
	hold := linuxHoldArgs(interpreter, command)
	directory = strings.TrimSpace(directory)
	base = strings.ToLower(strings.TrimSuffix(base, ".exe"))
	switch base {
	case "gnome-terminal", "kgx":
		args := make([]string, 0, 4+len(hold))
		if directory != "" {
			args = append(args, "--working-directory="+directory)
		}
		if len(hold) > 0 {
			args = append(args, "--")
			args = append(args, hold...)
		}
		return args, ""
	case "konsole":
		args := make([]string, 0, 4+len(hold))
		if directory != "" {
			args = append(args, "--workdir", directory)
		}
		if len(hold) > 0 {
			args = append(args, "-e")
			args = append(args, hold...)
		}
		return args, ""
	case "kitty":
		args := make([]string, 0, 3+len(hold))
		if directory != "" {
			args = append(args, "--directory", directory)
		}
		return append(args, hold...), ""
	case "foot":
		args := make([]string, 0, 3+len(hold))
		if directory != "" {
			args = append(args, "-D", directory)
		}
		return append(args, hold...), ""
	case "alacritty":
		args := make([]string, 0, 4+len(hold))
		if directory != "" {
			args = append(args, "--working-directory", directory)
		}
		if len(hold) > 0 {
			args = append(args, "-e")
			args = append(args, hold...)
		}
		return args, ""
	case "wezterm":
		args := []string{"start"}
		if directory != "" {
			args = append(args, "--cwd", directory)
		}
		if len(hold) > 0 {
			args = append(args, "--")
			args = append(args, hold...)
		}
		return args, ""
	case "ghostty":
		args := make([]string, 0, 4+len(hold))
		if directory != "" {
			args = append(args, "--working-directory="+directory)
		}
		if len(hold) > 0 {
			args = append(args, "-e")
			args = append(args, hold...)
		}
		return args, ""
	case "xterm":
		script := unixWorkingDirectoryCommand(directory, interpreter, command)
		if script == "" {
			return []string{"-e", defaultUnixShell(), "-l"}, directory
		}
		return []string{"-e", defaultUnixShell(), "-lc", script + "; exec " + posixShellQuote(defaultUnixShell())}, ""
	default:
		if len(hold) == 0 {
			return nil, directory
		}
		return append([]string{"-e"}, hold...), directory
	}
}

func appleScriptEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

// buildDarwinTerminalScript asks Terminal or iTerm to run the command and stay open.
func buildDarwinTerminalScript(app string, interpreter string, command string, directory string) string {
	line := unixWorkingDirectoryCommand(directory, interpreter, command)
	escaped := appleScriptEscape(line)
	if strings.EqualFold(app, "iTerm") || strings.EqualFold(app, "iTerm2") {
		if line == "" {
			return "tell application \"iTerm\"\nactivate\ncreate window with default profile\nend tell"
		}
		return "tell application \"iTerm\"\nactivate\nset newWindow to (create window with default profile)\ntell current session of newWindow\nwrite text \"" + escaped + "\"\nend tell\nend tell"
	}
	if line == "" {
		return "tell application \"Terminal\"\ndo script \"\"\nactivate\nend tell"
	}
	return "tell application \"Terminal\"\ndo script \"" + escaped + "\"\nactivate\nend tell"
}

func startVisibleLaunch(ctx context.Context, launch visibleProcessLaunch) error {
	if strings.TrimSpace(launch.File) == "" {
		return fmt.Errorf("terminal executable is empty")
	}
	return startVisibleProcess(ctx, launch)
}

func reapVisibleProcess(ctx context.Context, wait func() error) {
	util.Go(ctx, "reap external terminal process", func() {
		_ = wait()
	})
}

// buildWindowsExternalTerminalLaunch prefers Windows Terminal and keeps the session open.
func buildWindowsExternalTerminalLaunch(interpreter string, command string, directory string, terminalExe string) visibleProcessLaunch {
	payload := windowsKeepOpenPayload(interpreter, command)
	directory = strings.TrimSpace(directory)
	if terminalExe != "" {
		args := []string{"-w", "new"}
		if directory != "" {
			args = append(args, "-d", directory)
		}
		if len(payload) > 0 {
			args = append(args, "--")
			args = append(args, payload...)
		}
		return visibleProcessLaunch{File: terminalExe, Args: args}
	}
	if len(payload) == 0 {
		payload = []string{elevatedInterpreterFile(effectiveInterpreter(interpreter, ""))}
	}
	return visibleProcessLaunch{
		File:       payload[0],
		Args:       payload[1:],
		Directory:  directory,
		NewConsole: true,
	}
}

func windowsKeepOpenPayload(interpreter string, command string) []string {
	interpreter = effectiveInterpreter(interpreter, "")
	exe := elevatedInterpreterFile(interpreter)
	command = strings.TrimSpace(command)
	if command == "" {
		switch interpreter {
		case "python", "python3", "node":
			return []string{elevatedInterpreterFile("cmd")}
		case "powershell":
			return []string{exe, "-NoExit"}
		case "cmd":
			return []string{exe}
		default:
			return []string{exe, "-l"}
		}
	}

	switch interpreter {
	case "powershell":
		return []string{exe, "-NoExit", "-EncodedCommand", encodePowerShellCommand(command)}
	case "cmd":
		return []string{exe, "/K", command}
	case "python", "python3":
		return []string{elevatedInterpreterFile("cmd"), "/K", quoteWindowsArg(exe) + " -c " + quoteWindowsArg(command)}
	case "node":
		return []string{elevatedInterpreterFile("cmd"), "/K", quoteWindowsArg(exe) + " -e " + quoteWindowsArg(command)}
	default:
		return []string{exe, "-lc", command}
	}
}
