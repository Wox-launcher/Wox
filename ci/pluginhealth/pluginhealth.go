package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	_ "wox/plugin/host"
	"wox/resource"
	"wox/setting"
	"wox/util"
)

const (
	healthStatusPassed  = "passed"
	healthStatusFailed  = "failed"
	healthStatusSkipped = "skipped"

	healthStageCatalog = "catalog"
	healthStageOS      = "os"
	healthStageInstall = "install"
	healthStageInit    = "init"
	healthStageQuery   = "query"

	defaultHealthTimeout = 3 * time.Minute
	healthCleanupTimeout = time.Minute
	// Matches the Node/Python host RPC wait in WebsocketHost.invokeMethod.
	// Script plugins default to a 10s interactive cap; health checks raise that
	// via WOX_SCRIPT_EXECUTION_TIMEOUT so slow network fallbacks can finish.
	healthQueryTimeout        = 30 * time.Second
	healthQueryAttempts       = 3
	healthQueryRetryDelay     = time.Second
	scriptExecutionTimeoutEnv = "WOX_SCRIPT_EXECUTION_TIMEOUT"
)

type healthOptions struct {
	StorePath        string
	PluginIDs        []string
	OutputPath       string
	DataDir          string
	PerPluginTimeout time.Duration
}

type healthResult struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Runtime    string   `json:"runtime"`
	Status     string   `json:"status"`
	Stage      string   `json:"stage,omitempty"`
	Error      string   `json:"error,omitempty"`
	DurationMs int64    `json:"durationMs"`
	InitMs     int64    `json:"initMs,omitempty"`
	QueryMs    int64    `json:"queryMs,omitempty"`
	Queries    []string `json:"queries,omitempty"`
	hasInitMs  bool
	hasQueryMs bool
}

type healthReport struct {
	StartedAt string         `json:"startedAt"`
	Platform  string         `json:"platform"`
	StorePath string         `json:"storePath"`
	Passed    int            `json:"passed"`
	Failed    int            `json:"failed"`
	Skipped   int            `json:"skipped"`
	Results   []healthResult `json:"results"`
}

func (r healthReport) hasFailures() bool {
	return r.Failed > 0
}

func (r *healthReport) add(result healthResult) {
	r.Results = append(r.Results, result)
	switch result.Status {
	case healthStatusPassed:
		r.Passed++
	case healthStatusFailed:
		r.Failed++
	case healthStatusSkipped:
		r.Skipped++
	}
}

type healthStringList []string

func (s *healthStringList) String() string {
	return strings.Join(*s, ",")
}

func (s *healthStringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*s = append(*s, value)
	return nil
}

func main() {
	var (
		storePath  string
		outputPath string
		dataDir    string
		timeout    string
		pluginIDs  healthStringList
	)
	fs := flag.NewFlagSet("pluginhealth", flag.ExitOnError)
	fs.StringVar(&storePath, "store", "", "Path to store-plugin.json")
	fs.StringVar(&outputPath, "out", "", "Optional JSON report path")
	fs.StringVar(&dataDir, "data-dir", "", "Optional isolated Wox data directory")
	fs.StringVar(&timeout, "timeout", "3m", "Per-plugin timeout")
	fs.Var(&pluginIDs, "plugin", "Plugin id or name to check; repeat to select several")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	perPluginTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -timeout: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	report, runErr := runStorePluginHealth(ctx, healthOptions{
		StorePath:        storePath,
		PluginIDs:        pluginIDs,
		OutputPath:       outputPath,
		DataDir:          dataDir,
		PerPluginTimeout: perPluginTimeout,
	})
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
	}
	fmt.Printf("plugin health: %d passed, %d failed, %d skipped\n", report.Passed, report.Failed, report.Skipped)
	if runErr != nil {
		os.Exit(1)
	}
}

// runStorePluginHealth installs store plugins in an isolated data dir and probes init/query.
func runStorePluginHealth(ctx context.Context, opts healthOptions) (healthReport, error) {
	if opts.StorePath == "" {
		return healthReport{}, fmt.Errorf("store-plugin.json path is required")
	}
	if opts.PerPluginTimeout <= 0 {
		opts.PerPluginTimeout = defaultHealthTimeout
	}

	fmt.Printf("plugin health timeouts: plugin=%s query=%s\n", opts.PerPluginTimeout, healthQueryTimeout)

	dataDir, err := bootstrapPluginHealth(ctx, opts.DataDir)
	if err != nil {
		return healthReport{}, err
	}
	defer plugin.GetPluginManager().Stop(ctx)

	util.GetLogger().Info(ctx, fmt.Sprintf("plugin health data directory: %s", dataDir))

	manifests, err := loadStoreManifests(opts.StorePath)
	if err != nil {
		return healthReport{}, err
	}

	report := healthReport{
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Platform:  util.GetCurrentPlatform(),
		StorePath: opts.StorePath,
	}
	selected, missing := selectManifests(manifests, opts.PluginIDs)
	for _, result := range missing {
		report.add(result)
		fmt.Println(formatHealthResultLine(result))
	}

	for _, manifest := range selected {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		result := checkStorePlugin(ctx, opts.PerPluginTimeout, manifest)
		report.add(result)
		fmt.Println(formatHealthResultLine(result))
	}

	if err := writeHealthReport(opts.OutputPath, report); err != nil {
		return report, err
	}
	if report.hasFailures() {
		return report, fmt.Errorf("%d store plugin(s) failed health checks", report.Failed)
	}
	return report, nil
}

func bootstrapPluginHealth(ctx context.Context, dataDir string) (string, error) {
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), fmt.Sprintf("wox-plugin-health-%d", os.Getpid()))
	}
	userDir := filepath.Join(dataDir, "user")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("create plugin health data directory: %w", err)
	}

	if err := os.Setenv(util.TestWoxDataDirEnv, dataDir); err != nil {
		return "", err
	}
	if err := os.Setenv(util.TestUserDataDirEnv, userDir); err != nil {
		return "", err
	}
	if err := os.Setenv(util.TestDisableTelemetryEnv, "true"); err != nil {
		return "", err
	}
	if err := os.Setenv(scriptExecutionTimeoutEnv, healthQueryTimeout.String()); err != nil {
		return "", err
	}

	if err := util.GetLocation().Init(); err != nil {
		return "", fmt.Errorf("initialize location: %w", err)
	}
	if err := resource.Extract(ctx); err != nil {
		return "", fmt.Errorf("extract host resources: %w", err)
	}
	if err := database.Init(ctx); err != nil {
		return "", fmt.Errorf("initialize database: %w", err)
	}

	_ = setting.GetSettingManager()
	if err := i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCodeEnUs); err != nil {
		return "", fmt.Errorf("initialize i18n: %w", err)
	}

	plugin.GetPluginManager().AttachUI(pluginHealthUI{})
	return dataDir, nil
}

func checkStorePlugin(parent context.Context, timeout time.Duration, manifest plugin.StorePluginManifest) healthResult {
	started := time.Now()
	result := healthResult{
		Id:      manifest.Id,
		Name:    pluginDisplayName(manifest, nil),
		Version: manifest.Version,
		Runtime: string(manifest.Runtime),
	}
	if skipped, ok := skipUnsupportedOS(manifest); ok {
		skipped.DurationMs = time.Since(started).Milliseconds()
		return skipped
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	if err := plugin.GetStoreManager().InstallLocal(ctx, manifest); err != nil {
		result.Status = healthStatusFailed
		result.Stage = healthStageInstall
		result.Error = err.Error()
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	instance := plugin.GetPluginManager().GetPluginInstanceById(manifest.Id)
	defer uninstallCheckedPlugin(ctx, instance)
	result.Name = pluginDisplayName(manifest, instance)

	if instance == nil {
		result.Status = healthStatusFailed
		result.Stage = healthStageInstall
		result.Error = "plugin instance missing after install"
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}

	if err := plugin.GetPluginManager().WaitPluginInit(ctx, manifest.Id); err != nil {
		recordInitDuration(&result, instance)
		result.Status = healthStatusFailed
		result.Stage = healthStageInit
		result.Error = err.Error()
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	recordInitDuration(&result, instance)

	for _, query := range buildHealthProbeQueries(instance) {
		result.Queries = append(result.Queries, query.RawQuery)
		queryStarted := time.Now()
		err := probePluginQueryWithRetry(ctx, instance, query)
		recordQueryDuration(&result, time.Since(queryStarted))
		if err != nil {
			result.Status = healthStatusFailed
			result.Stage = healthStageQuery
			result.Error = fmt.Sprintf("%s: %s", query.RawQuery, err.Error())
			result.DurationMs = time.Since(started).Milliseconds()
			return result
		}
	}

	result.Status = healthStatusPassed
	result.DurationMs = time.Since(started).Milliseconds()
	return result
}

type fallibleQuery interface {
	QueryWithError(ctx context.Context, query plugin.Query) (plugin.QueryResponse, error)
}

// probePluginQueryWithRetry re-runs a query when CI network jitter kills the first attempt.
func probePluginQueryWithRetry(ctx context.Context, instance *plugin.Instance, query plugin.Query) error {
	var lastErr error
	for attempt := 1; attempt <= healthQueryAttempts; attempt++ {
		queryCtx, queryCancel := context.WithTimeout(ctx, healthQueryTimeout)
		lastErr = probePluginQuery(queryCtx, instance, query)
		queryCancel()
		if lastErr == nil || !isTransientHealthQueryError(lastErr) {
			return lastErr
		}
		if attempt == healthQueryAttempts {
			break
		}
		fmt.Printf("retrying query %q after transient error (%d/%d): %s\n", query.RawQuery, attempt, healthQueryAttempts-1, lastErr)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthQueryRetryDelay):
		}
	}
	return lastErr
}

// isTransientHealthQueryError matches CI network/timeout kills that are worth retrying.
func isTransientHealthQueryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"signal: killed",
		"context deadline exceeded",
		"i/o timeout",
		"tls handshake timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
		"network is unreachable",
		"wsarecv",
		"forcibly closed",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// probePluginQuery treats empty results as success and reports execution failures.
func probePluginQuery(ctx context.Context, instance *plugin.Instance, query plugin.Query) (queryErr error) {
	if instance == nil || instance.Plugin == nil {
		return fmt.Errorf("plugin runtime is not loaded")
	}
	defer func() {
		if r := recover(); r != nil {
			queryErr = fmt.Errorf("plugin query panic: %v", r)
		}
	}()
	if fallible, ok := instance.Plugin.(fallibleQuery); ok {
		_, queryErr = fallible.QueryWithError(ctx, query)
	} else {
		_ = instance.Plugin.Query(ctx, query)
	}
	if queryErr == nil && instance.Host != nil && !instance.Host.IsStarted(ctx) {
		queryErr = fmt.Errorf("plugin host exited during query")
	}
	return queryErr
}

func uninstallCheckedPlugin(ctx context.Context, instance *plugin.Instance) {
	if instance == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), healthCleanupTimeout)
	defer cancel()
	if err := plugin.GetStoreManager().UninstallLocal(cleanupCtx, instance, true); err != nil {
		util.GetLogger().Warn(cleanupCtx, fmt.Sprintf("failed to uninstall health-check plugin %s: %s", instance.Metadata.Id, err.Error()))
	}
}

func loadStoreManifests(path string) ([]plugin.StorePluginManifest, error) {
	fileStr, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read store plugin manifest: %w", err)
	}

	var manifests []plugin.StorePluginManifest
	if err := json.Unmarshal(fileStr, &manifests); err != nil {
		return nil, fmt.Errorf("unmarshal store plugin manifest: %w", err)
	}

	for i := range manifests {
		if plugin.IsSupportedRuntime(string(manifests[i].Runtime)) {
			manifests[i].Runtime = plugin.ConvertToRuntime(string(manifests[i].Runtime))
		}
	}
	return manifests, nil
}

func selectManifests(manifests []plugin.StorePluginManifest, pluginIDs []string) ([]plugin.StorePluginManifest, []healthResult) {
	if len(pluginIDs) == 0 {
		return manifests, nil
	}

	var selected []plugin.StorePluginManifest
	seen := map[string]bool{}
	for _, manifest := range manifests {
		if matchesPluginFilter(manifest, pluginIDs) {
			selected = append(selected, manifest)
			seen[strings.ToLower(manifest.Id)] = true
			seen[strings.ToLower(pluginDisplayName(manifest, nil))] = true
		}
	}

	var missing []healthResult
	for _, rawID := range pluginIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || seen[strings.ToLower(id)] {
			continue
		}
		missing = append(missing, healthResult{
			Id:     id,
			Name:   id,
			Status: healthStatusFailed,
			Stage:  healthStageCatalog,
			Error:  "not found in store-plugin.json",
		})
	}
	return selected, missing
}

func matchesPluginFilter(manifest plugin.StorePluginManifest, pluginIDs []string) bool {
	for _, rawID := range pluginIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if strings.EqualFold(manifest.Id, id) ||
			strings.EqualFold(manifest.Name, id) ||
			strings.EqualFold(pluginDisplayName(manifest, nil), id) {
			return true
		}
	}
	return false
}

func skipUnsupportedOS(manifest plugin.StorePluginManifest) (healthResult, bool) {
	if len(manifest.SupportedOS) == 0 || plugin.IsAnySupportedInCurrentOS(manifest.SupportedOS) {
		return healthResult{}, false
	}
	return healthResult{
		Id:      manifest.Id,
		Name:    pluginDisplayName(manifest, nil),
		Version: manifest.Version,
		Runtime: string(manifest.Runtime),
		Status:  healthStatusSkipped,
		Stage:   healthStageOS,
		Error:   fmt.Sprintf("unsupported on %s", util.GetCurrentPlatform()),
	}, true
}

// pluginDisplayName prefers the English name from the installed plugin.json,
// then the store manifest I18n map, and never prints a raw i18n: key.
func pluginDisplayName(manifest plugin.StorePluginManifest, instance *plugin.Instance) string {
	if instance != nil {
		if name := resolvedEnglishName(instance.Metadata.GetNameEn(context.Background())); name != "" {
			return name
		}
	}
	if name := resolvedEnglishName(manifest.GetNameEnUs()); name != "" {
		return name
	}
	if name := resolvedEnglishName(manifest.Name); name != "" {
		return name
	}
	return manifest.Id
}

func resolvedEnglishName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "i18n:") {
		return ""
	}
	return name
}

func buildHealthProbeQueries(instance *plugin.Instance) []plugin.Query {
	if instance == nil {
		return nil
	}

	trigger := instance.PrimaryTriggerKeyword()
	queries := []plugin.Query{newHealthProbeQuery(trigger, "")}
	commands := instance.GetQueryCommands()
	if len(commands) == 0 {
		return queries
	}
	return append(queries, newHealthProbeQuery(trigger, commands[0].Command))
}

func newHealthProbeQuery(triggerKeyword string, command string) plugin.Query {
	return plugin.Query{
		Id:             uuid.NewString(),
		Type:           plugin.QueryTypeInput,
		RawQuery:       strings.TrimSpace(triggerKeyword + " " + command),
		TriggerKeyword: triggerKeyword,
		Command:        command,
	}
}

func writeHealthReport(path string, report healthReport) error {
	if path == "" {
		return nil
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin health report: %w", err)
	}
	if err := os.WriteFile(path, payload, 0644); err != nil {
		return fmt.Errorf("write plugin health report: %w", err)
	}
	return nil
}

func recordInitDuration(result *healthResult, instance *plugin.Instance) {
	if instance == nil || instance.InitStartTimestamp == 0 || instance.InitFinishedTimestamp == 0 {
		return
	}
	ms := instance.InitFinishedTimestamp - instance.InitStartTimestamp
	if ms < 0 {
		return
	}
	result.InitMs = ms
	result.hasInitMs = true
}

func recordQueryDuration(result *healthResult, elapsed time.Duration) {
	ms := elapsed.Milliseconds()
	if ms < 0 {
		return
	}
	result.QueryMs += ms
	result.hasQueryMs = true
}

func formatTimingSuffix(result healthResult) string {
	var parts []string
	if result.hasInitMs {
		parts = append(parts, fmt.Sprintf("%dms@init", result.InitMs))
	}
	if result.hasQueryMs {
		parts = append(parts, fmt.Sprintf("%dms@query", result.QueryMs))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func formatHealthResultLine(result healthResult) string {
	timing := formatTimingSuffix(result)
	switch result.Status {
	case healthStatusPassed:
		return fmt.Sprintf("[PASS] %s %s (%s)%s", result.Name, result.Version, result.Runtime, timing)
	case healthStatusSkipped:
		return fmt.Sprintf("[SKIP] %s %s: %s", result.Name, result.Version, result.Error)
	default:
		if result.Stage == "" {
			return fmt.Sprintf("[FAIL] %s %s: %s%s", result.Name, result.Version, result.Error, timing)
		}
		return fmt.Sprintf("[FAIL] %s %s [%s]: %s%s", result.Name, result.Version, result.Stage, result.Error, timing)
	}
}

// pluginHealthUI satisfies plugin APIs during headless health checks without a window.
type pluginHealthUI struct{}

func (pluginHealthUI) ChangeQuery(context.Context, common.PlainQuery) {}

func (pluginHealthUI) RefreshQuery(context.Context, bool) {}

func (pluginHealthUI) HideApp(context.Context) {}

func (pluginHealthUI) ShowApp(context.Context, common.ShowContext) {}

func (pluginHealthUI) ToggleApp(context.Context, common.ShowContext) {}

func (pluginHealthUI) OpenWoxInstance(context.Context, common.OpenWoxInstanceRequest) {}

func (pluginHealthUI) RecordHotkey(context.Context, string, string) {}

func (pluginHealthUI) OpenSettingWindow(context.Context, common.SettingWindowContext) {}

func (pluginHealthUI) OpenOnboardingWindow(context.Context) {}

func (pluginHealthUI) OpenNotesWindow(context.Context, common.NotesWindowRequest) {}

func (pluginHealthUI) RefreshNotesWindow(context.Context, string) {}

func (pluginHealthUI) OpenMacOSPermissionFlow(context.Context, string) {}

func (pluginHealthUI) PickFiles(context.Context, common.PickFilesParams) []string { return nil }

func (pluginHealthUI) CaptureScreenshot(context.Context, common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusCancelled}, nil
}

func (pluginHealthUI) WriteClipboardImageFile(context.Context, string) error { return nil }

func (pluginHealthUI) GetActiveWindowSnapshot(context.Context) common.ActiveWindowSnapshot {
	return common.ActiveWindowSnapshot{}
}

func (pluginHealthUI) GetServerPort(context.Context) int { return 0 }

func (pluginHealthUI) GetAllThemes(context.Context) []common.Theme { return nil }

func (pluginHealthUI) ChangeTheme(context.Context, common.Theme) {}

func (pluginHealthUI) InstallTheme(context.Context, common.Theme) {}

func (pluginHealthUI) UninstallTheme(context.Context, common.Theme) {}

func (pluginHealthUI) RestoreTheme(context.Context) {}

func (pluginHealthUI) Notify(context.Context, common.NotifyMsg) {}

func (pluginHealthUI) UpdateAttentionUnreadCount(context.Context, int) {}

func (pluginHealthUI) ShowToolbarMsg(context.Context, interface{}) {}

func (pluginHealthUI) ClearToolbarMsg(context.Context, string) {}

func (pluginHealthUI) UpdateResult(context.Context, interface{}) bool { return false }

func (pluginHealthUI) PushResults(context.Context, interface{}) bool { return false }

func (pluginHealthUI) IsVisible(context.Context) bool { return false }

func (pluginHealthUI) SendChatResponse(context.Context, common.AIChatData) {}

func (pluginHealthUI) ReloadChatResources(context.Context, string) {}

func (pluginHealthUI) SendAIQuestion(context.Context, string, string, []common.AIQuestionOption) {}

func (pluginHealthUI) ReloadSettingPlugins(context.Context) {}

func (pluginHealthUI) ReloadSetting(context.Context) {}

func (pluginHealthUI) ReloadSettingThemes(context.Context) {}

func (pluginHealthUI) CloudSyncProgressChanged(context.Context, any) {}

func (pluginHealthUI) RefreshGlance(context.Context, string, []string) {}
