package plugin

import (
	"context"
	"testing"
)

func TestScriptInterpreterRuntime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Runtime
	}{
		{name: "python file", in: "Wox.Plugin.Script.Timestamp.py", want: PLUGIN_RUNTIME_PYTHON},
		{name: "python gist url", in: "https://gist.githubusercontent.com/qianlifeng/31363d95905325e9969d93d999e94b07/raw/Wox.Plugin.Script.Timestamp.py", want: PLUGIN_RUNTIME_PYTHON},
		{name: "nodejs file", in: "Wox.Plugin.Script.Demo.js", want: PLUGIN_RUNTIME_NODEJS},
		{name: "nodejs url with query", in: "https://example.com/plugin.js?token=1", want: PLUGIN_RUNTIME_NODEJS},
		{name: "windows path", in: `C:\Users\me\.wox\plugins\scripts\Wox.Plugin.Script.Timestamp.py`, want: PLUGIN_RUNTIME_PYTHON},
		{name: "extensionless gist", in: "https://gist.githubusercontent.com/qianlifeng/82a2f748177ce47a900b4c4da3abfd28/raw/Wox.Plugin.Script.UUID", want: ""},
		{name: "empty", in: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ScriptInterpreterRuntime(test.in); got != test.want {
				t.Fatalf("ScriptInterpreterRuntime(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestEnsureScriptInterpreterReady(t *testing.T) {
	originalHosts := AllHosts
	t.Cleanup(func() {
		AllHosts = originalHosts
	})

	ctx := context.Background()
	AllHosts = []Host{
		&fakeHost{
			status: RuntimeHostStatus{
				StatusCode:    RuntimeHostStatusUnsupportedVersion,
				StatusMessage: "Python 3.7.0 is below the minimum required version 3.10.0.",
			},
		},
	}

	err := EnsureScriptInterpreterReady(ctx, "https://example.com/Wox.Plugin.Script.Timestamp.py")
	if err == nil {
		t.Fatal("expected unsupported Python to block install")
	}
	if err.Error() != "Python 3.7.0 is below the minimum required version 3.10.0." {
		t.Fatalf("error = %q", err)
	}

	if err := EnsureScriptInterpreterReady(ctx, "https://example.com/Wox.Plugin.Script.UUID"); err != nil {
		t.Fatalf("extensionless script should skip interpreter check, got %v", err)
	}

	AllHosts = []Host{
		&fakeHost{
			status: RuntimeHostStatus{StatusCode: RuntimeHostStatusStopped},
		},
	}
	if err := EnsureScriptInterpreterReady(ctx, "plugin.py"); err != nil {
		t.Fatalf("stopped host with a valid interpreter should allow install, got %v", err)
	}
}

func TestRuntimeRefreshStillPending(t *testing.T) {
	if !runtimeRefreshStillPending(RuntimeHostStatus{StatusCode: RuntimeHostStatusUnsupportedVersion}) {
		t.Fatal("unsupported version should stay pending after refresh")
	}
	if !runtimeRefreshStillPending(RuntimeHostStatus{StatusCode: RuntimeHostStatusExecutableMissing}) {
		t.Fatal("missing interpreter should stay pending after refresh")
	}
	if runtimeRefreshStillPending(RuntimeHostStatus{StatusCode: RuntimeHostStatusStartFailed}) {
		t.Fatal("a host start failure is not an expected pending refresh")
	}
}
