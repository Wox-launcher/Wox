package system

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"
	"wox/common"
	"wox/diagnostic"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BugReportPlugin{})
}

type BugReportPlugin struct {
	api plugin.API
}

const bugReportIssueURL = "https://github.com/Wox-launcher/Wox/issues/new?template=bug_report.yml"

func (p *BugReportPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "b7f6f0f3-9d18-4f17-b74d-f28d19b1b541",
		Name:            "i18n:plugin_bug_report_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_bug_report_plugin_description",
		Icon:            common.PluginBugReportIcon.String(),
		Entry:           "",
		TriggerKeywords: []string{"bugreport"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
	}
}

func (p *BugReportPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

func (p *BugReportPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	incidents := diagnostic.GetManager().ListCrashIncidents()
	if len(incidents) == 0 {
		return plugin.NewQueryResponse([]plugin.QueryResult{p.buildNoCrashResult()})
	}

	results := make([]plugin.QueryResult, 0, len(incidents))
	for _, incident := range incidents {
		results = append(results, p.buildCrashIncidentResult(ctx, incident))
	}
	return plugin.NewQueryResponse(results)
}

// buildNoCrashResult keeps manual diagnostics export available for a clean history.
func (p *BugReportPlugin) buildNoCrashResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_bug_report_no_crashes_title",
		SubTitle: "i18n:plugin_bug_report_no_crashes_description",
		Icon:     common.PluginBugReportIcon,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: "i18n:plugin_bug_report_no_crashes_preview",
		},
	}
}

// buildCrashIncidentResult creates one actionable result for a retained crash event.
func (p *BugReportPlugin) buildCrashIncidentResult(ctx context.Context, incident diagnostic.CrashIncident) plugin.QueryResult {
	signal := incident.Signal
	if signal == "" {
		signal = p.api.GetTranslation(ctx, "plugin_bug_report_crash_signal_none")
	}
	title := fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_crash_title"), incident.ID)
	subtitle := fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_crash_subtitle"), util.FormatTimestampWithMs(incident.DetectedAt))
	preview := fmt.Sprintf(
		"## %s\n\n%s\n\n- %s: `%d`\n- %s: `%d`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`",
		title,
		subtitle,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_pid"),
		incident.PID,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_exit_code"),
		incident.ExitCode,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_signal"),
		signal,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_duration"),
		formatCrashDuration(incident.DurationMs),
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_version"),
		incident.Version,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_id"),
		incident.ID,
		p.api.GetTranslation(ctx, "plugin_bug_report_crash_package"),
		incident.ReportPath,
	)
	return plugin.QueryResult{
		Title:    title,
		SubTitle: subtitle,
		Icon:     common.PluginBugReportIcon,
		// The launcher re-sorts cached results by score, so preserve newest-first event ordering here.
		Score: incident.DetectedAt,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: preview,
		},
		Actions: p.buildCrashIncidentActions(incident),
	}
}

// buildCrashIncidentActions provides event-scoped packaging and issue actions.
func (p *BugReportPlugin) buildCrashIncidentActions(incident diagnostic.CrashIncident) []plugin.QueryResultAction {
	return []plugin.QueryResultAction{
		{
			Name:                   "i18n:plugin_bug_report_action_package_issue",
			Icon:                   common.OpenContainingFolderIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.openCrashIssue(ctx, incident)
			},
		},
		{
			Name:                   "i18n:plugin_bug_report_action_open_package",
			Icon:                   common.OpenContainingFolderIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				reportPath, _, err := p.ensureCrashReport(ctx, incident)
				if err != nil {
					p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_export_failed"), err.Error()))
					return
				}
				if err := shell.OpenFileInFolder(reportPath); err != nil {
					util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open crash report package: %s", err.Error()))
				}
			},
		},
	}
}

// buildActions provides actions that are not tied to one crash event.
func (p *BugReportPlugin) buildActions() []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{}
	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_bug_report_action_export",
		Icon:                   common.PluginInstalledIcon,
		IsDefault:              true,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			p.exportDiagnostics(ctx)
		},
	})

	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_bug_report_action_open_logs",
		Icon:                   common.OpenContainingFolderIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			_ = shell.OpenFileInFolder(util.GetLocation().GetLogDirectory())
		},
	})
	return actions
}

// openCrashIssue prepares the selected package and opens its GitHub issue form.
func (p *BugReportPlugin) openCrashIssue(ctx context.Context, incident diagnostic.CrashIncident) {
	reportPath, created, err := p.ensureCrashReport(ctx, incident)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_export_failed"), err.Error()))
		return
	}
	if err := shell.OpenFileInFolder(reportPath); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open crash report package: %s", err.Error()))
	}
	if err := shell.Open(crashIssueURL(incident)); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open GitHub issue page for crash report: %s", err.Error()))
	}
	if created {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_exported"), reportPath))
	}
}

// ensureCrashReport reuses an automatic package or creates one if it was removed.
func (p *BugReportPlugin) ensureCrashReport(ctx context.Context, incident diagnostic.CrashIncident) (string, bool, error) {
	if info, err := os.Stat(incident.ReportPath); err == nil && !info.IsDir() {
		return incident.ReportPath, false, nil
	}
	reportPath, err := diagnostic.GetManager().ExportCrash(ctx)
	if err != nil {
		return "", false, err
	}
	incident.ReportPath = reportPath
	if err := diagnostic.GetManager().SaveCrashIncident(incident); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to update crash incident package path: %s", err.Error()))
	}
	return reportPath, true, nil
}

// crashIssueURL pre-fills the GitHub issue title with the selected event time.
func crashIssueURL(incident diagnostic.CrashIncident) string {
	query := url.Values{}
	query.Set("template", "bug_report.yml")
	query.Set("title", fmt.Sprintf("[Crash] %s", util.FormatTimestampWithMs(incident.DetectedAt)))
	return bugReportIssueURL[:len(bugReportIssueURL)-len("?template=bug_report.yml")] + "?" + query.Encode()
}

// formatCrashDuration keeps short startup crashes readable in result subtitles.
func formatCrashDuration(durationMs int64) string {
	if durationMs < 1000 {
		return fmt.Sprintf("%d ms", durationMs)
	}
	return (time.Duration(durationMs) * time.Millisecond).Round(time.Millisecond).String()
}

// crashPackageStatus describes whether the event package can currently be opened.
func crashPackageStatus(reportPath string) string {
	if reportPath == "" {
		return "not generated"
	}
	if _, err := os.Stat(reportPath); err != nil {
		return "not found"
	}
	return reportPath
}

func (p *BugReportPlugin) exportDiagnostics(ctx context.Context) {
	exportPath, err := diagnostic.GetManager().Export(ctx)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_export_failed"), err.Error()))
		return
	}
	_ = shell.OpenFileInFolder(exportPath)
	if openErr := shell.Open(bugReportIssueURL); openErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open bug report issue page after diagnostics export: %s", openErr.Error()))
	}
	p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_exported"), exportPath))
}
