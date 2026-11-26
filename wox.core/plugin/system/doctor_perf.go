package system

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"time"

	"wox/i18n"
	"wox/plugin"
	"wox/ui"
)

type doctorRuntimeStatus struct {
	runtime plugin.Runtime
	status  plugin.RuntimeHostStatus
}

// snapshotDoctorHosts keeps the three relevant hosts in a stable display order.
func snapshotDoctorHosts(ctx context.Context) []doctorRuntimeStatus {
	hosts := make([]doctorRuntimeStatus, 0, 3)
	for _, runtime := range []plugin.Runtime{plugin.PLUGIN_RUNTIME_NODEJS, plugin.PLUGIN_RUNTIME_PYTHON, plugin.PLUGIN_RUNTIME_SCRIPT} {
		status, registered := plugin.RuntimeStatusForRegisteredHost(ctx, runtime)
		if !registered {
			status.StatusCode = plugin.RuntimeHostStatusStopped
		}
		hosts = append(hosts, doctorRuntimeStatus{runtime, status})
	}
	return hosts
}

// queryPerformance shows live launcher, host and plugin metrics in diagnostic order.
func (r *DoctorPlugin) queryPerformance(ctx context.Context) plugin.QueryResponse {
	snapshot := ui.GetUIManager().SnapshotPerformanceDiagnostics(ctx)
	translate := func(key string) string { return i18n.GetI18nManager().TranslateWox(ctx, key) }
	p50, p95, maxMs := doctorLatencyStats(snapshot.WakeMilliseconds)
	results := []plugin.QueryResult{{
		Title: "i18n:plugin_doctor_wake", Icon: doctorIcon,
		SubTitle: translate("i18n:plugin_doctor_no_samples"),
	}}
	if len(snapshot.WakeMilliseconds) > 0 {
		results[0].SubTitle = doctorLatencyText(p50, p95, maxMs)
	}
	results[0].SubTitle += fmt.Sprintf(" · %s · %s", doctorRenderModeLabel(ctx, runtime.GOOS, snapshot.RenderMode), fmt.Sprintf(translate("i18n:plugin_doctor_presented_frames"), snapshot.PresentedFrameCount))

	for _, phase := range snapshot.FramePhases {
		subtitle := translate("i18n:plugin_doctor_no_samples")
		if phase.Samples > 0 {
			subtitle = doctorPhaseText(phase)
		}
		results = append(results, plugin.QueryResult{
			Title: "i18n:plugin_doctor_phase_" + string(phase.Phase), SubTitle: subtitle, Icon: doctorIcon,
		})
	}
	for _, host := range snapshotDoctorHosts(ctx) {
		results = append(results, doctorHostPerformanceResult(host))
	}
	manager := plugin.GetPluginManager()
	metrics := manager.SnapshotPluginQueryMetrics(ctx)
	if len(metrics) == 0 {
		results = append(results, plugin.QueryResult{Title: "i18n:plugin_doctor_plugin_queries", SubTitle: "i18n:plugin_doctor_no_samples", Icon: doctorIcon})
	}
	for _, metric := range metrics {
		name, icon := metric.PluginID, doctorIcon
		if instance := manager.GetPluginInstanceById(metric.PluginID); instance != nil {
			name = instance.GetName(ctx)
			icon = instance.Metadata.GetIconOrDefault(instance.PluginDirectory, doctorIcon)
		}
		results = append(results, plugin.QueryResult{
			Title: name, Icon: icon,
			SubTitle: doctorLatencyText(doctorMilliseconds(metric.P50), doctorMilliseconds(metric.P95), doctorMilliseconds(metric.Max)) + " · " +
				doctorQueryHealthText(ctx, metric.HealthState, metric.TimeoutCount),
			Group: "i18n:plugin_doctor_plugin_queries", GroupScore: -1,
		})
	}
	for i := range results {
		results[i].Score = int64(len(results) - i)
	}
	return plugin.NewQueryResponse(results)
}

// doctorQueryHealthText explains the circuit state beside a plugin's timing.
func doctorQueryHealthText(ctx context.Context, state plugin.PluginHealthState, timeouts uint64) string {
	i18nManager := i18n.GetI18nManager()
	return fmt.Sprintf(i18nManager.TranslateWox(ctx, "i18n:plugin_doctor_query_health"),
		i18nManager.TranslateWox(ctx, "i18n:plugin_doctor_query_"+string(state)), timeouts)
}

// doctorHostPerformanceResult keeps raw startup errors in the local text preview.
func doctorHostPerformanceResult(host doctorRuntimeStatus) plugin.QueryResult {
	result := plugin.QueryResult{
		Title: doctorHostName(host.runtime), SubTitle: "i18n:ui_runtime_status_" + string(host.status.StatusCode), Icon: doctorIcon,
	}
	if host.status.StatusCode == plugin.RuntimeHostStatusStartFailed && host.status.LastStartError != "" {
		result.Preview = plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: host.status.LastStartError}
	}
	return result
}

func doctorRenderModeLabel(ctx context.Context, platform string, mode ui.RenderMode) string {
	if platform == "darwin" && mode == ui.RenderModeSoftware {
		return i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_doctor_render_core_graphics")
	}
	return string(mode)
}

func doctorHostName(runtime plugin.Runtime) string {
	if runtime == plugin.PLUGIN_RUNTIME_NODEJS {
		return "Node.js"
	}
	if runtime == plugin.PLUGIN_RUNTIME_PYTHON {
		return "Python"
	}
	return "Script"
}

func doctorMilliseconds(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func doctorLatencyText(p50, p95, maxMs float64) string {
	return fmt.Sprintf("P50 %.1f ms · P95 %.1f ms · Max %.1f ms", p50, p95, maxMs)
}

func doctorPhaseText(phase ui.FramePhaseDiagnostic) string {
	return fmt.Sprintf("Avg %.2f ms · Max %.2f ms", float64(phase.TotalMicroseconds)/float64(phase.Samples)/1000, float64(phase.MaxMicroseconds)/1000)
}

// doctorLatencyStats computes nearest-rank percentiles without reordering the samples in the report.
func doctorLatencyStats(values []float64) (p50, p95, maxMs float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	samples := slices.Clone(values)
	slices.Sort(samples)
	n := len(samples)
	return samples[(50*n+99)/100-1], samples[(95*n+99)/100-1], samples[n-1]
}
