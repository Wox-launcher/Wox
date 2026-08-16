package shell

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
	"wox/util"
)

func TestAppendExecuteAsAdministratorActionIsWindowsOnly(t *testing.T) {
	plugin := &ShellPlugin{}
	actions := plugin.appendExecuteAsAdministratorAction(nil, shellContextData{
		Command:     "scoop update -a",
		Interpreter: "powershell",
	})

	if util.IsWindows() {
		if len(actions) != 1 {
			t.Fatalf("action count = %d, want 1 on Windows", len(actions))
		}
		action := actions[0]
		if action.Id != "execute_as_administrator" {
			t.Fatalf("action id = %q, want execute_as_administrator", action.Id)
		}
		if action.Name != "i18n:plugin_shell_execute_as_administrator" {
			t.Fatalf("action name = %q", action.Name)
		}
		if action.PreventHideAfterAction {
			t.Fatal("administrator action must hide the launcher so the UAC prompt is visible")
		}
		return
	}

	if len(actions) != 0 {
		t.Fatalf("action count = %d, want 0 off Windows", len(actions))
	}
}

func TestBuildElevatedShellLaunchEncodesPowerShellCommand(t *testing.T) {
	command := `scoop update -a; Write-Host "hello & world"`
	launch := buildElevatedShellLaunch("powershell", command, `C:\temp`)
	if !strings.Contains(strings.ToLower(launch.File), "powershell") {
		t.Fatalf("file = %q, want powershell", launch.File)
	}
	if launch.Directory != `C:\temp` {
		t.Fatalf("directory = %q, want C:\\temp", launch.Directory)
	}

	const prefix = "-NoProfile -EncodedCommand "
	if !strings.HasPrefix(launch.Parameters, prefix) {
		t.Fatalf("parameters = %q, want EncodedCommand", launch.Parameters)
	}

	decoded := decodePowerShellEncodedCommand(t, strings.TrimPrefix(launch.Parameters, prefix))
	if !strings.Contains(decoded, command) {
		t.Fatalf("decoded script %q does not contain command", decoded)
	}
}

func TestBuildElevatedShellLaunchQuotesCmdCommand(t *testing.T) {
	launch := buildElevatedShellLaunch("cmd", `echo hello & echo world`, "")
	if !strings.Contains(strings.ToLower(launch.File), "cmd") {
		t.Fatalf("file = %q, want cmd", launch.File)
	}
	if launch.Directory != "" {
		t.Fatalf("directory = %q, want empty", launch.Directory)
	}
	if !strings.HasPrefix(launch.Parameters, "/C ") {
		t.Fatalf("parameters = %q, want /C", launch.Parameters)
	}
	if !strings.Contains(launch.Parameters, `echo hello & echo world`) {
		t.Fatalf("parameters = %q, want original command", launch.Parameters)
	}
}

func TestQuoteWindowsArg(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{input: "echo", want: "echo"},
		{input: "hello world", want: `"hello world"`},
		{input: `say "hi"`, want: `"say \"hi\""`},
		{input: `dir\`, want: `dir\`},
		{input: `C:\Program Files\temp\`, want: `"C:\Program Files\temp\\"`},
		{input: "", want: `""`},
	}

	for _, testCase := range testCases {
		if got := quoteWindowsArg(testCase.input); got != testCase.want {
			t.Fatalf("quoteWindowsArg(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
}

func decodePowerShellEncodedCommand(t *testing.T, encoded string) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode encoded command: %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("encoded command byte length = %d, want even", len(raw))
	}

	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	return string(utf16.Decode(units))
}
