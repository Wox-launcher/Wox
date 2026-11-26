package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/ui"
	"wox/updater"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/processmemory"
	"wox/util/shell"
)

type doctorCheckHealth struct {
	Type    plugin.DoctorCheckType `json:"type"`
	Passed  bool                   `json:"passed"`
	Ignored bool                   `json:"ignored"`
}

type doctorHostHealth struct {
	Runtime plugin.Runtime               `json:"runtime"`
	Status  plugin.RuntimeHostStatusCode `json:"status"`
}

type doctorPluginRuntime string

const (
	doctorPluginRuntimeGo     doctorPluginRuntime = "go"
	doctorPluginRuntimeNode   doctorPluginRuntime = "nodejs"
	doctorPluginRuntimePython doctorPluginRuntime = "python"
	doctorPluginRuntimeScript doctorPluginRuntime = "script"
	doctorPluginRuntimeOther  doctorPluginRuntime = "other"
)

type doctorQueryTiming struct {
	TimeoutCount     uint64                   `json:"timeoutCount"`
	HealthState      plugin.PluginHealthState `json:"healthState"`
	CircuitOpenUntil *time.Time               `json:"circuitOpenUntil,omitempty"`
	Rank             int                      `json:"rank"`
	PluginID         string                   `json:"pluginId"`
	PluginName       string                   `json:"pluginName,omitempty"`
	System           bool                     `json:"system"`
	Runtime          doctorPluginRuntime      `json:"runtime"`
	Samples          int                      `json:"samples"`
	TotalQueries     uint64                   `json:"totalQueries"`
	SlowQueries      uint64                   `json:"slowQueries"`
	PanicCount       uint64                   `json:"panicCount"`
	P50Millis        float64                  `json:"p50Ms"`
	P95Millis        float64                  `json:"p95Ms"`
	MaxMillis        float64                  `json:"maxMs"`
}

// DoctorDiagnosticReport projects shareable fields, including plugin identity,
// without collecting user content, check details, or runtime error messages.
type DoctorDiagnosticReport struct {
	LinuxEnvironment *linuxEnvironment        `json:"linuxEnvironment,omitempty"`
	SchemaVersion    int                      `json:"schemaVersion"`
	WoxVersion       string                   `json:"woxVersion"`
	OS               string                   `json:"os"`
	Arch             string                   `json:"arch"`
	Checks           []doctorCheckHealth      `json:"checks"`
	UI               ui.UIPerformanceSnapshot `json:"ui"`
	Hosts            []doctorHostHealth       `json:"hosts"`
	PluginQueries    []doctorQueryTiming      `json:"pluginQueries"`
	ProcessBytes     uint64                   `json:"processBytes"`
	GPU              ui.GPUMemoryUsage        `json:"gpu"`
	Resilience       doctorResilience         `json:"resilience"`
}

// diagnosticExportResult uses the same report for preview, copy, and save.
func (r *DoctorPlugin) diagnosticExportResult(ctx context.Context, report DoctorDiagnosticReport) plugin.QueryResult {
	markdown := report.markdown(ctx)
	return plugin.QueryResult{
		Title: "i18n:plugin_doctor_export", SubTitle: "i18n:plugin_doctor_export_description", Icon: doctorIcon,
		Preview: plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: markdown},
		Actions: []plugin.QueryResultAction{
			{
				Name: "i18n:plugin_doctor_copy_markdown", Icon: icons.Get(icons.ActionCopy), IsDefault: true,
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					r.finishDoctorExport(ctx, clipboard.WriteText(markdown), "i18n:plugin_doctor_copied")
				},
			},
			{
				Name: "i18n:plugin_doctor_save_json", Icon: icons.Get(icons.ActionOpenContainingFolder),
				Action: func(ctx context.Context, _ plugin.ActionContext) {
					dir := filepath.Join(util.GetLocation().GetWoxDataDirectory(), "diagnostics")
					path, err := report.writeJSON(dir)
					if err != nil {
						r.finishDoctorExport(ctx, err, "")
						return
					}
					if err := shell.OpenFileInFolder(path); err != nil {
						util.GetLogger().Warn(ctx, fmt.Sprintf("reveal Doctor diagnostic report: %v", err))
					}
					r.finishDoctorExport(ctx, nil, "i18n:plugin_doctor_saved")
				},
			},
		},
	}
}

func (r *DoctorPlugin) finishDoctorExport(ctx context.Context, err error, message string) {
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("export Doctor diagnostics: %v", err))
		message = "i18n:plugin_doctor_export_failed"
	}
	r.api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: message})
}

// collectDoctorDiagnosticReport reads metrics without reading user content or logs.
func collectDoctorDiagnosticReport(ctx context.Context) DoctorDiagnosticReport {
	manager := ui.GetUIManager()
	plugins := plugin.GetPluginManager()
	processBytes, _ := processmemory.GetProcessMemoryBytes(os.Getpid())
	report := buildDoctorDiagnosticReport(ctx, plugin.RunDoctorChecks(ctx), snapshotDoctorHosts(ctx), plugins.SnapshotPluginQueryMetrics(ctx), plugins.GetPluginInstanceById)
	if util.IsLinux() {
		environment := currentLinuxEnvironment()
		report.LinuxEnvironment = &environment
	}
	report.UI = manager.SnapshotPerformanceDiagnostics(ctx)
	report.ProcessBytes = processBytes
	report.GPU = manager.GPUMemoryDiagnostics()
	report.Resilience = collectDoctorResilience(ctx)
	return report
}

// buildDoctorDiagnosticReport projects source data onto the export allowlist.
func buildDoctorDiagnosticReport(ctx context.Context, checks []plugin.DoctorCheckResult, hosts []doctorRuntimeStatus, metrics []plugin.PluginQueryMetrics, lookupPlugin func(string) *plugin.Instance) DoctorDiagnosticReport {
	report := DoctorDiagnosticReport{
		SchemaVersion: 1, WoxVersion: updater.CURRENT_VERSION, OS: runtime.GOOS, Arch: runtime.GOARCH,
		Checks: make([]doctorCheckHealth, 0, len(checks)), Hosts: make([]doctorHostHealth, 0, len(hosts)),
		PluginQueries: make([]doctorQueryTiming, 0, len(metrics)),
	}
	for _, check := range checks {
		switch check.Type {
		case plugin.DoctorCheckUpdate, plugin.DoctorCheckAccessibility, plugin.DoctorCheckDatabase,
			plugin.DoctorCheckTriggerKeywordConflict, plugin.DoctorCheckGnomeTrayIndicator,
			plugin.DoctorCheckWaylandDesktopLaunch, plugin.DoctorCheckLinuxInputGroup, plugin.DoctorCheckLinuxUinputGroup:
			report.Checks = append(report.Checks, doctorCheckHealth{check.Type, check.Passed, check.Ignored})
		}
	}
	for _, host := range hosts {
		switch host.runtime {
		case plugin.PLUGIN_RUNTIME_NODEJS, plugin.PLUGIN_RUNTIME_PYTHON, plugin.PLUGIN_RUNTIME_SCRIPT:
			switch host.status.StatusCode {
			case plugin.RuntimeHostStatusRunning, plugin.RuntimeHostStatusStopped, plugin.RuntimeHostStatusExecutableMissing,
				plugin.RuntimeHostStatusUnsupportedVersion, plugin.RuntimeHostStatusStartFailed:
				report.Hosts = append(report.Hosts, doctorHostHealth{host.runtime, host.status.StatusCode})
			}
		}
	}
	for i, metric := range metrics {
		timing := doctorQueryTiming{
			TimeoutCount: metric.TimeoutCount, HealthState: metric.HealthState,
			Rank: i + 1, PluginID: metric.PluginID, Samples: metric.Samples, TotalQueries: metric.TotalQueries, SlowQueries: metric.SlowQueries, PanicCount: metric.PanicCount,
			P50Millis: doctorMilliseconds(metric.P50), P95Millis: doctorMilliseconds(metric.P95), MaxMillis: doctorMilliseconds(metric.Max),
			Runtime: doctorPluginRuntimeOther,
		}
		if metric.HealthState == plugin.PluginHealthCircuitOpen {
			until := metric.CircuitOpenUntil.UTC()
			timing.CircuitOpenUntil = &until
		}
		if instance := lookupPlugin(metric.PluginID); instance != nil {
			timing.PluginName = instance.Metadata.GetNameEn(ctx)
			timing.System = instance.IsSystemPlugin
			timing.Runtime = doctorExportRuntime(instance.Metadata.Runtime)
		}
		report.PluginQueries = append(report.PluginQueries, timing)
	}
	return report
}

// doctorExportRuntime projects plugin-supplied metadata onto a finite set.
func doctorExportRuntime(value string) doctorPluginRuntime {
	switch plugin.ConvertToRuntime(value) {
	case plugin.PLUGIN_RUNTIME_GO:
		return doctorPluginRuntimeGo
	case plugin.PLUGIN_RUNTIME_NODEJS:
		return doctorPluginRuntimeNode
	case plugin.PLUGIN_RUNTIME_PYTHON:
		return doctorPluginRuntimePython
	case plugin.PLUGIN_RUNTIME_SCRIPT:
		return doctorPluginRuntimeScript
	default:
		return doctorPluginRuntimeOther
	}
}

// markdown formats the same allowlisted fields for pasting into an issue.
func (r DoctorDiagnosticReport) markdown(ctx context.Context) string {
	translate := func(key string) string { return i18n.GetI18nManager().TranslateWox(ctx, key) }
	var out strings.Builder
	fmt.Fprintf(&out, "## Wox Doctor · %s\n\nWox %s · %s/%s · %s", translate("i18n:plugin_doctor_export"), r.WoxVersion, r.OS, r.Arch, doctorRenderModeLabel(ctx, r.OS, r.UI.RenderMode))
	if r.LinuxEnvironment != nil {
		fmt.Fprintf(&out, " · %s / %s", r.LinuxEnvironment.Desktop, r.LinuxEnvironment.DisplayProtocol.label())
	}
	out.WriteString("\n\n")
	for _, check := range r.Checks {
		status := "❌"
		if check.Passed {
			status = "✅"
		}
		fmt.Fprintf(&out, "- %s `%s`", status, check.Type)
		if check.Ignored {
			fmt.Fprintf(&out, " (%s)", translate("i18n:plugin_doctor_ignored"))
		}
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "\n**%s**\n\n", translate("i18n:plugin_doctor_perf"))
	fmt.Fprintf(&out, "- %s\n", fmt.Sprintf(translate("i18n:plugin_doctor_presented_frames"), r.UI.PresentedFrameCount))
	wake := translate("i18n:plugin_doctor_no_samples")
	if len(r.UI.WakeMilliseconds) > 0 {
		wake = doctorLatencyText(doctorLatencyStats(r.UI.WakeMilliseconds))
	}
	fmt.Fprintf(&out, "- %s · %s\n", translate("i18n:plugin_doctor_wake"), wake)
	for i, display := range r.UI.Displays {
		logical, pixels := display.LogicalBounds, display.PixelBounds
		line := fmt.Sprintf(translate("i18n:plugin_doctor_display"), i+1,
			logical.Width, logical.Height, logical.X, logical.Y, pixels.Width, pixels.Height, pixels.X, pixels.Y, display.Scale)
		if display.Primary {
			line += " · " + translate("i18n:plugin_doctor_primary_display")
		}
		fmt.Fprintf(&out, "- %s\n", line)
	}
	for _, phase := range r.UI.FramePhases {
		if phase.Samples > 0 {
			fmt.Fprintf(&out, "- %s · %s\n", translate("i18n:plugin_doctor_phase_"+string(phase.Phase)), doctorPhaseText(phase))
		}
	}
	for _, host := range r.Hosts {
		fmt.Fprintf(&out, "- %s · %s\n", doctorHostName(host.Runtime), translate("i18n:ui_runtime_status_"+string(host.Status)))
	}
	fmt.Fprintf(&out, "- %s · %.1f MiB\n", translate("i18n:plugin_doctor_process_memory"), float64(r.ProcessBytes)/(1<<20))
	if r.GPU.Available {
		fmt.Fprintf(&out, "- %s · %.1f MiB\n", translate("i18n:plugin_doctor_gpu_memory"), float64(r.GPU.SystemBytes+r.GPU.DedicatedBytes)/(1<<20))
	}
	fmt.Fprintf(&out, "\n**%s**\n\n", translate("i18n:plugin_doctor_plugin_queries"))
	for _, query := range r.PluginQueries {
		fmt.Fprintf(&out, "%d. ", query.Rank)
		if query.PluginName != "" {
			fmt.Fprintf(&out, "%s · ", doctorMarkdownCode(query.PluginName))
		}
		counts := fmt.Sprintf(translate("i18n:plugin_doctor_query_counts"), query.TotalQueries, query.SlowQueries, query.PanicCount)
		fmt.Fprintf(&out, "%s · `%s` · %s · %s · %s\n", doctorMarkdownCode(query.PluginID), query.Runtime,
			doctorLatencyText(query.P50Millis, query.P95Millis, query.MaxMillis), counts, doctorQueryHealthText(ctx, query.HealthState, query.TimeoutCount))
	}
	if len(r.PluginQueries) == 0 {
		out.WriteString(translate("i18n:plugin_doctor_no_samples") + "\n")
	}
	r.Resilience.markdown(ctx, &out)
	return out.String()
}

// doctorMarkdownCode renders manifest labels literally, including embedded backticks.
func doctorMarkdownCode(value string) string {
	longest, consecutive := 0, 0
	for _, char := range value {
		if char == '`' {
			consecutive++
			longest = max(longest, consecutive)
		} else {
			consecutive = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	return fence + " " + strings.Join(strings.Fields(value), " ") + " " + fence
}

// writeJSON creates a private, uniquely named report and removes incomplete writes.
func (r DoctorDiagnosticReport) writeJSON(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "wox-doctor-"+time.Now().UTC().Format("20060102T150405Z")+"-*.json")
	if err != nil {
		return "", err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	writeErr := encoder.Encode(r)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())
		if writeErr != nil {
			return "", writeErr
		}
		return "", closeErr
	}
	return file.Name(), nil
}
