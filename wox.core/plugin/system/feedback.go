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
	"wox/updater"
	"wox/util"
	"wox/util/shell"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &FeedbackPlugin{})
}

type FeedbackPlugin struct {
	api plugin.API
}

const (
	feedbackGitHubNewIssueURL  = "https://github.com/Wox-launcher/Wox/issues/new"
	feedbackBugTemplate        = "bug_report.yml"
	feedbackFeatureTemplate    = "feature_request.yml"
	feedbackBugTitlePrefix     = "[Bug]: "
	feedbackFeatureTitlePrefix = "[Feature Request]: "
	feedbackCommandCrash       = "crash"
)

func (p *FeedbackPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "b7f6f0f3-9d18-4f17-b74d-f28d19b1b541",
		Name:          "i18n:plugin_feedback_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_feedback_plugin_description",
		Icon:          common.PluginFeedbackIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"feedback",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     feedbackCommandCrash,
				Description: "i18n:plugin_feedback_command_crash",
			},
		},
		SupportedOS: []string{"Windows", "Macos", "Linux"},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
	}
}

func (p *FeedbackPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

func (p *FeedbackPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Command == feedbackCommandCrash {
		return plugin.NewQueryResponse(p.buildCrashResults(ctx))
	}
	if query.Command != "" {
		return plugin.QueryResponse{}
	}
	return plugin.NewQueryResponse(p.buildDefaultResults())
}

// buildDefaultResults lists the everyday GitHub and log actions.
func (p *FeedbackPlugin) buildDefaultResults() []plugin.QueryResult {
	return []plugin.QueryResult{
		p.buildBugResult(),
		p.buildFeatureResult(),
		p.buildClearLogsResult(),
	}
}

// buildCrashResults lists retained crash events under the explicit crash command.
func (p *FeedbackPlugin) buildCrashResults(ctx context.Context) []plugin.QueryResult {
	incidents := diagnostic.GetManager().ListCrashIncidents()
	if len(incidents) == 0 {
		return []plugin.QueryResult{p.buildNoCrashResult()}
	}

	results := make([]plugin.QueryResult, 0, len(incidents))
	for _, incident := range incidents {
		results = append(results, p.buildCrashIncidentResult(ctx, incident))
	}
	return results
}

func (p *FeedbackPlugin) buildBugResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_feedback_bug_title",
		SubTitle: "i18n:plugin_feedback_bug_subtitle",
		Icon:     common.PluginFeedbackIcon,
		Score:    300,
		Actions: []plugin.QueryResultAction{
			{
				Name:      "i18n:plugin_feedback_bug_title",
				Icon:      common.PluginFeedbackIcon,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.exportDiagnostics(ctx)
				},
			},
		},
	}
}

func (p *FeedbackPlugin) buildFeatureResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_feedback_feature_title",
		SubTitle: "i18n:plugin_feedback_feature_subtitle",
		Icon:     common.PluginNotesIcon,
		Score:    200,
		Actions: []plugin.QueryResultAction{
			{
				Name:      "i18n:plugin_feedback_feature_title",
				Icon:      common.PluginNotesIcon,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.openFeatureRequest(ctx)
				},
			},
		},
	}
}

func (p *FeedbackPlugin) buildClearLogsResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_feedback_clear_logs_title",
		SubTitle: "i18n:plugin_feedback_clear_logs_subtitle",
		Icon:     common.TrashIcon,
		Score:    100,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_feedback_clear_logs_title",
				Icon:                   common.TrashIcon,
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.clearLogs(ctx)
				},
			},
		},
	}
}

func (p *FeedbackPlugin) buildNoCrashResult() plugin.QueryResult {
	return plugin.QueryResult{
		Title:    "i18n:plugin_feedback_no_crashes_title",
		SubTitle: "i18n:plugin_feedback_no_crashes_description",
		Icon:     common.PluginFeedbackIcon,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: "i18n:plugin_feedback_no_crashes_preview",
		},
	}
}

// buildCrashIncidentResult creates one actionable result for a retained crash event.
func (p *FeedbackPlugin) buildCrashIncidentResult(ctx context.Context, incident diagnostic.CrashIncident) plugin.QueryResult {
	signal := incident.Signal
	if signal == "" {
		signal = p.api.GetTranslation(ctx, "plugin_feedback_crash_signal_none")
	}
	title := fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_crash_title"), incident.ID)
	subtitle := fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_crash_subtitle"), util.FormatTimestampWithMs(incident.DetectedAt))
	preview := fmt.Sprintf(
		"## %s\n\n%s\n\n- %s: `%d`\n- %s: `%d`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`\n- %s: `%s`",
		title,
		subtitle,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_pid"),
		incident.PID,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_exit_code"),
		incident.ExitCode,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_signal"),
		signal,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_duration"),
		formatCrashDuration(incident.DurationMs),
		p.api.GetTranslation(ctx, "plugin_feedback_crash_version"),
		incident.Version,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_id"),
		incident.ID,
		p.api.GetTranslation(ctx, "plugin_feedback_crash_package"),
		incident.ReportPath,
	)
	return plugin.QueryResult{
		Title:    title,
		SubTitle: subtitle,
		Icon:     common.PluginFeedbackIcon,
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
func (p *FeedbackPlugin) buildCrashIncidentActions(incident diagnostic.CrashIncident) []plugin.QueryResultAction {
	return []plugin.QueryResultAction{
		{
			Name:                   "i18n:plugin_feedback_action_package_issue",
			Icon:                   common.OpenContainingFolderIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.openCrashIssue(ctx, incident)
			},
		},
		{
			Name:                   "i18n:plugin_feedback_action_open_package",
			Icon:                   common.OpenContainingFolderIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				reportPath, _, err := p.ensureCrashReport(ctx, incident)
				if err != nil {
					p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_export_failed"), err.Error()))
					return
				}
				if err := shell.OpenFileInFolder(reportPath); err != nil {
					util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open crash report package: %s", err.Error()))
				}
			},
		},
	}
}

// openCrashIssue prepares the selected package and opens its GitHub issue form.
func (p *FeedbackPlugin) openCrashIssue(ctx context.Context, incident diagnostic.CrashIncident) {
	reportPath, created, err := p.ensureCrashReport(ctx, incident)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_export_failed"), err.Error()))
		return
	}
	p.openGitHubThenRevealFile(ctx, crashIssueURL(incident), reportPath)
	if created {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_exported"), reportPath))
	}
}

// ensureCrashReport reuses an automatic package or creates one if it was removed.
func (p *FeedbackPlugin) ensureCrashReport(ctx context.Context, incident diagnostic.CrashIncident) (string, bool, error) {
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

// githubIssueURL opens a GitHub issue form and prefills text fields that the form supports.
func githubIssueURL(template, title string) string {
	query := url.Values{}
	query.Set("template", template)
	if title != "" {
		query.Set("title", title)
	}
	query.Set("wox_version", updater.CURRENT_VERSION)
	return feedbackGitHubNewIssueURL + "?" + query.Encode()
}

// crashIssueURL pre-fills the GitHub issue title with the selected event time.
func crashIssueURL(incident diagnostic.CrashIncident) string {
	return githubIssueURL(feedbackBugTemplate, fmt.Sprintf("[Crash] %s", util.FormatTimestampWithMs(incident.DetectedAt)))
}

// formatCrashDuration keeps short startup crashes readable in result subtitles.
func formatCrashDuration(durationMs int64) string {
	if durationMs < 1000 {
		return fmt.Sprintf("%d ms", durationMs)
	}
	return (time.Duration(durationMs) * time.Millisecond).Round(time.Millisecond).String()
}

func (p *FeedbackPlugin) exportDiagnostics(ctx context.Context) {
	exportPath, err := diagnostic.GetManager().Export(ctx)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_export_failed"), err.Error()))
		return
	}
	p.openGitHubThenRevealFile(ctx, githubIssueURL(feedbackBugTemplate, feedbackBugTitlePrefix), exportPath)
	p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_exported"), exportPath))
}

// openGitHubThenRevealFile opens the issue page first so Explorer is not covered by the browser.
func (p *FeedbackPlugin) openGitHubThenRevealFile(ctx context.Context, issueURL, filePath string) {
	if err := shell.Open(issueURL); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open GitHub issue page: %s", err.Error()))
	} else {
		time.Sleep(500 * time.Millisecond)
	}
	if err := shell.OpenFileInFolder(filePath); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to reveal exported file: %s", err.Error()))
	}
}

func (p *FeedbackPlugin) openFeatureRequest(ctx context.Context) {
	if err := shell.Open(githubIssueURL(feedbackFeatureTemplate, feedbackFeatureTitlePrefix)); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to open GitHub feature request page: %s", err.Error()))
	}
}

func (p *FeedbackPlugin) clearLogs(ctx context.Context) {
	if err := util.GetLogger().ClearHistory(); err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_feedback_notify_clear_failed"), err.Error()))
		return
	}
	p.api.Notify(ctx, p.api.GetTranslation(ctx, "plugin_feedback_notify_logs_cleared"))
}
