package shell

import (
	"context"
	"slices"
	"strings"
	"testing"
	"wox/common/icons"
	"wox/plugin"
)

func TestBuildOpenInSystemTerminalActionHandsOffAndHidesOnSuccess(t *testing.T) {
	pluginInstance := &ShellPlugin{}
	action := pluginInstance.buildOpenInSystemTerminalAction(shellContextData{
		Command:          "git status",
		Interpreter:      "powershell",
		WorkingDirectory: `D:\dev\apps`,
	})
	if action.Id != "open_in_system_terminal" {
		t.Fatalf("id = %q, want open_in_system_terminal", action.Id)
	}
	if action.Name != "i18n:plugin_shell_open_in_system_terminal" {
		t.Fatalf("name = %q", action.Name)
	}
	if action.Icon != icons.Get(icons.ActionOpenInSystemTerminal) {
		t.Fatal("icon should use action.open-in-system-terminal")
	}
	if !action.PreventHideAfterAction {
		t.Fatal("action should hide only after the terminal starts")
	}
	if action.ContextData[shellActionCommandKey] != "git status" {
		t.Fatalf("command context = %q", action.ContextData[shellActionCommandKey])
	}
}

func TestEmptyAndCommandResultsIncludeOpenInSystemTerminal(t *testing.T) {
	pluginInstance := &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{}}}
	empty := pluginInstance.buildEmptyCommandResult(context.Background(), "powershell", `D:\dev\apps`)
	if !resultHasOpenInSystemTerminalAction(empty) {
		t.Fatal("empty command result is missing Open in System Terminal")
	}

	response := pluginInstance.Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: ">",
		Search:         "git status",
	})
	if len(response.Results) == 0 || !resultHasOpenInSystemTerminalAction(response.Results[0]) {
		t.Fatal("command result is missing Open in System Terminal")
	}
}

func TestPosixShellQuoteEscapesSingleQuotes(t *testing.T) {
	if got := posixShellQuote("it's"); got != `'it'"'"'s'` {
		t.Fatalf("posixShellQuote = %q", got)
	}
}

func TestUnixWorkingDirectoryCommandCdsThenRuns(t *testing.T) {
	got := unixWorkingDirectoryCommand(`/tmp/wo x`, "bash", `git status`)
	if got != `cd '/tmp/wo x' && git status` {
		t.Fatalf("unix command = %q", got)
	}
	if got := unixRunScript("python", "print('hi')"); got != `'python' -c 'print('"'"'hi'"'"')'` {
		t.Fatalf("python script = %q", got)
	}
}

func TestUnixHoldScriptKeepsShellOpen(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	got := unixHoldScript("bash", "ls")
	if got != "ls; exec '/bin/zsh'" {
		t.Fatalf("hold script = %q", got)
	}
}

func TestLinuxTerminalArgsForCommonEmulators(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	args, dir := linuxTerminalArgs("gnome-terminal", "bash", "git status", "/tmp/work")
	if dir != "" {
		t.Fatalf("gnome-terminal should pass the directory as a flag, got %q", dir)
	}
	if !slices.Contains(args, "--working-directory=/tmp/work") || !slices.Contains(args, "--") {
		t.Fatalf("gnome-terminal args = %#v", args)
	}

	args, dir = linuxTerminalArgs("kitty", "bash", "", "/tmp/work")
	if dir != "" || !slices.Equal(args, []string{"--directory", "/tmp/work"}) {
		t.Fatalf("empty kitty launch = %#v dir=%q", args, dir)
	}

	args, dir = linuxTerminalArgs("xterm", "bash", "ls", "/tmp/work")
	if dir != "" || len(args) < 4 || args[0] != "-e" || !strings.Contains(args[len(args)-1], "cd '/tmp/work' && ls") {
		t.Fatalf("xterm args = %#v dir=%q", args, dir)
	}
}

func TestBuildDarwinTerminalScriptQuotesPathAndCommand(t *testing.T) {
	script := buildDarwinTerminalScript("Terminal", "bash", `echo "hi"`, `/tmp/wo"x`)
	want := "tell application \"Terminal\"\ndo script \"cd '/tmp/wo\\\"x' && echo \\\"hi\\\"\"\nactivate\nend tell"
	if script != want {
		t.Fatalf("terminal script = %q, want %q", script, want)
	}
	if !strings.Contains(buildDarwinTerminalScript("iTerm", "bash", "", "/tmp"), `create window with default profile`) {
		t.Fatal("empty iTerm script should open a window")
	}
}

func TestBuildWindowsExternalTerminalLaunchPrefersWindowsTerminal(t *testing.T) {
	launch := buildWindowsExternalTerminalLaunch("powershell", `Write-Host "hi"`, `C:\dev`, `C:\wt.exe`)
	if launch.File != `C:\wt.exe` || launch.NewConsole {
		t.Fatalf("launch = %#v", launch)
	}
	if !slices.Equal(launch.Args[:4], []string{"-w", "new", "-d", `C:\dev`}) {
		t.Fatalf("args prefix = %#v", launch.Args)
	}
	if !slices.Contains(launch.Args, "--") || !slices.Contains(launch.Args, "-EncodedCommand") {
		t.Fatalf("payload = %#v", launch.Args)
	}

	fallback := buildWindowsExternalTerminalLaunch("cmd", `echo hello`, `C:\dev`, "")
	if !fallback.NewConsole || fallback.Directory != `C:\dev` || !slices.Contains(fallback.Args, "/K") {
		t.Fatalf("fallback = %#v", fallback)
	}
}

func resultHasOpenInSystemTerminalAction(result plugin.QueryResult) bool {
	for _, action := range result.Actions {
		if action.Id == "open_in_system_terminal" {
			return true
		}
	}
	return false
}
