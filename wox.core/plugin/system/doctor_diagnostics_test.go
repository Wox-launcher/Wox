package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/ui"
	"wox/util/screen"
)

func TestDoctorDiagnosticsRoutesCommandAndSearchForms(t *testing.T) {
	doctor := &DoctorPlugin{}
	commands := doctor.GetMetadata().Commands
	for _, command := range commands {
		for _, input := range []plugin.Query{{Command: command.Command, Search: "ignored"}, {Search: command.Command}} {
			if got := doctorQueryCommand(input); got != doctorCommand(command.Command) {
				t.Fatalf("route for %+v = %q", input, got)
			}
		}
	}
	if len(commands) != 3 || doctorQueryCommand(plugin.Query{}) != "" {
		t.Fatal("bare Doctor must still route to static checks")
	}
	result := doctor.diagnosticExportResult(context.Background(), DoctorDiagnosticReport{})
	if len(result.Actions) != 2 || result.Preview.PreviewType != plugin.WoxPreviewTypeMarkdown {
		t.Fatalf("expected Markdown preview and both actions: %+v", result)
	}
}

func TestDoctorDiagnosticExportsExcludePrivateSourceData(t *testing.T) {
	secrets := []string{`C:\Users\alice\private.log`, "/home/alice/.wox", "secret-token", "private query", "clipboard content", "note body"}
	ctx := context.Background()
	failedHost := doctorRuntimeStatus{plugin.PLUGIN_RUNTIME_PYTHON, plugin.RuntimeHostStatus{
		StatusCode: plugin.RuntimeHostStatusStartFailed, LastStartError: secrets[2], ExecutablePath: secrets[0],
	}}
	if preview := doctorHostPerformanceResult(failedHost).Preview; preview.PreviewType != plugin.WoxPreviewTypeText || preview.PreviewData != secrets[2] {
		t.Fatal("host startup error is missing from the local performance preview")
	}
	const thirdPartyID, thirdPartyName = "org.example.example-plugin", "Example `[Plug-in](https://example.test)`"
	const otherID = "org.example.unknown-runtime"
	builtin := &DoctorPlugin{}
	report := buildDoctorDiagnosticReport(ctx, []plugin.DoctorCheckResult{
		{Type: plugin.DoctorCheckDatabase, Passed: false, Ignored: true, Name: secrets[0], Description: strings.Join(secrets, " ")},
		{Type: plugin.DoctorCheckType(secrets[2])},
	}, []doctorRuntimeStatus{
		failedHost,
		{plugin.PLUGIN_RUNTIME_NODEJS, plugin.RuntimeHostStatus{StatusCode: plugin.RuntimeHostStatusCode(secrets[1])}},
	}, []plugin.PluginQueryMetrics{
		{PluginID: thirdPartyID, TotalQueries: 4, SlowQueries: 2, PanicCount: 1, P95: 27 * time.Millisecond},
		{PluginID: common.DoctorPluginID, TotalQueries: 3},
		{PluginID: otherID, TotalQueries: 2},
	}, func(id string) *plugin.Instance {
		switch id {
		case thirdPartyID:
			return &plugin.Instance{Metadata: plugin.Metadata{Id: id, Name: thirdPartyName, Description: common.I18nString(secrets[5]), Runtime: "Python"}, PluginDirectory: secrets[1]}
		case common.DoctorPluginID:
			return &plugin.Instance{Metadata: builtin.GetMetadata(), IsSystemPlugin: true}
		case otherID:
			return &plugin.Instance{Metadata: plugin.Metadata{Id: id, Runtime: secrets[0]}}
		}
		return nil
	})
	report.OS = "linux"
	report.LinuxEnvironment = &linuxEnvironment{Desktop: linuxDesktopKDE, DisplayProtocol: linuxDisplayWayland}
	report.UI.Displays = []ui.DisplayDiagnosticInfo{{
		LogicalBounds: screen.Rect{X: -1280, Width: 1280, Height: 720},
		PixelBounds:   screen.Rect{X: -1920, Width: 1920, Height: 1080},
		Scale:         1.5, Primary: true,
	}}

	dir := filepath.Join(t.TempDir(), "diagnostics")
	path, err := report.writeJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir || filepath.Ext(path) != ".json" {
		t.Fatalf("report path = %q", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DoctorDiagnosticReport
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || len(decoded.Checks) != 1 || len(decoded.Hosts) != 1 ||
		len(decoded.PluginQueries) != 3 || decoded.PluginQueries[0].P95Millis != 27 || decoded.LinuxEnvironment == nil ||
		decoded.LinuxEnvironment.DisplayProtocol != linuxDisplayWayland {
		t.Fatalf("report lost diagnostic data: %+v", decoded)
	}
	if result := decoded.PluginQueries[0]; result.System || result.PluginID != thirdPartyID || result.PluginName != thirdPartyName || result.Runtime != doctorPluginRuntimePython {
		t.Fatalf("third-party metadata is missing from report: %+v", result)
	}
	if result := decoded.PluginQueries[1]; !result.System || result.PluginID != common.DoctorPluginID || result.PluginName == "" || result.Runtime != doctorPluginRuntimeGo {
		t.Fatalf("built-in metadata is missing from report: %+v", result)
	}
	if result := decoded.PluginQueries[2]; result.Runtime != doctorPluginRuntimeOther {
		t.Fatalf("unknown runtime was not normalized: %+v", result)
	}
	markdown := report.markdown(ctx)
	const thirdPartyMarkdown = "`` Example `[Plug-in](https://example.test)` ``"
	for _, value := range []string{thirdPartyMarkdown, thirdPartyID, common.DoctorPluginID, "KDE / Wayland", "1280x720", "1920x1080", "-1280", "-1920", "1.50×", "4 queries · 2 slow · 1 panics"} {
		if !strings.Contains(markdown, value) {
			t.Errorf("Markdown missing %q", value)
		}
	}
	if preview := builtin.diagnosticExportResult(ctx, report).Preview.PreviewData; preview != markdown {
		t.Fatal("the report preview differs from the shareable Markdown")
	}
	for _, content := range []string{strings.ReplaceAll(string(body), `\\`, `\`), markdown} {
		for _, secret := range secrets {
			if strings.Contains(content, secret) {
				t.Fatalf("diagnostic export contains `%s`", secret)
			}
		}
	}
}

func TestLinuxDiagnosticEnvironmentUsesKnownLabels(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop session detection")
	}
	for _, key := range []string{"XDG_SESSION_DESKTOP", "DESKTOP_SESSION", "GDMSESSION", "GNOME_DESKTOP_SESSION_ID", "DISPLAY", "WAYLAND_DISPLAY"} {
		t.Setenv(key, "")
	}
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("XDG_CURRENT_DESKTOP", "/home/alice/private-session")
	if got := currentLinuxEnvironment(); got != (linuxEnvironment{linuxDesktopOther, linuxDisplayX11}) {
		t.Fatalf("unknown desktop must collapse to other: %+v", got)
	}
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "/home/alice/private-socket")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland:/home/alice/private-session")
	if got := currentLinuxEnvironment(); got != (linuxEnvironment{linuxDesktopHyprland, linuxDisplayWayland}) {
		t.Fatalf("expected only known labels for Hyprland: %+v", got)
	}
}
