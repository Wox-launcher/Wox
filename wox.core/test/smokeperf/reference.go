// Package smokeperf records launcher latency and process memory for comparison across smoke runs.
// The readings are printed after the case table. They do not decide whether a case passed.
package smokeperf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"
	"time"

	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	// LinePrefix marks one machine-readable reading in test output. The runner
	// reprints the parsed reading after the case table.
	LinePrefix = "SMOKE_PERF "

	// CollectedEnvironment tells the perf case that the runner already recorded
	// the reference on the fresh process, so the case does not sample again.
	CollectedEnvironment = "WOX_SMOKE_PERFORMANCE_REFERENCE"

	// CollectionBudget covers the show samples, the query series, and the hidden wait.
	CollectionBudget = 3 * time.Minute

	// ShowWarmupSamples are discarded so the P95 describes a warm show path.
	ShowWarmupSamples = 4
	// ShowSampleCount is the steady sample size for the show-to-visible P95.
	ShowSampleCount = 20
	// QueryCount is the number of distinct queries run before the visible memory reading.
	QueryCount = 20

	// HiddenSettle is how long the launcher stays hidden before the second memory reading.
	// Hide cleanup releases caches after 10s and, at 30s, drops prepared search text,
	// the pinyin table, and the Windows renderer. 46s is past that cleanup.
	HiddenSettle = 46 * time.Second
)

// Reference is one recorded performance sample. Nil metric pointers mean that reading was not taken.
type Reference struct {
	When               string   `json:"when,omitempty"`
	CollectedInMs      *float64 `json:"collected_in_ms,omitempty"`
	ShowToVisibleP95Ms *float64 `json:"show_to_visible_p95_ms,omitempty"`
	ShowSamples        int      `json:"show_samples,omitempty"`
	ShowError          string   `json:"show_error,omitempty"`
	QueryMemoryBytes   *uint64  `json:"query_memory_bytes,omitempty"`
	QueryMemoryError   string   `json:"query_memory_error,omitempty"`
	HiddenMemoryBytes  *uint64  `json:"hidden_memory_bytes,omitempty"`
	HiddenMemoryError  string   `json:"hidden_memory_error,omitempty"`
}

// Empty reports whether any reading or failure was recorded.
func (r Reference) Empty() bool {
	return r.When == "" && r.CollectedInMs == nil &&
		r.ShowToVisibleP95Ms == nil && r.ShowError == "" &&
		r.QueryMemoryBytes == nil && r.QueryMemoryError == "" &&
		r.HiddenMemoryBytes == nil && r.HiddenMemoryError == ""
}

// Merge keeps the receiver's readings and overlays fields that next actually set.
func (r Reference) Merge(next Reference) Reference {
	if next.When != "" {
		r.When = next.When
	}
	if next.CollectedInMs != nil {
		r.CollectedInMs = next.CollectedInMs
	}
	if next.ShowToVisibleP95Ms != nil {
		r.ShowToVisibleP95Ms = next.ShowToVisibleP95Ms
	}
	if next.ShowSamples != 0 {
		r.ShowSamples = next.ShowSamples
	}
	if next.ShowError != "" {
		r.ShowError = next.ShowError
	}
	if next.QueryMemoryBytes != nil {
		r.QueryMemoryBytes = next.QueryMemoryBytes
	}
	if next.QueryMemoryError != "" {
		r.QueryMemoryError = next.QueryMemoryError
	}
	if next.HiddenMemoryBytes != nil {
		r.HiddenMemoryBytes = next.HiddenMemoryBytes
	}
	if next.HiddenMemoryError != "" {
		r.HiddenMemoryError = next.HiddenMemoryError
	}
	return r
}

// Line is the test-log form of a reference. ParseLine accepts that line with a go test prefix.
func (r Reference) Line() string {
	payload, err := json.Marshal(r)
	if err != nil {
		return LinePrefix + "{}"
	}
	return LinePrefix + string(payload)
}

// ParseLine reads a SMOKE_PERF record from one test-output line.
func ParseLine(line string) (Reference, bool) {
	index := strings.Index(line, LinePrefix)
	if index < 0 {
		return Reference{}, false
	}
	var ref Reference
	if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(LinePrefix):])), &ref); err != nil {
		return Reference{}, false
	}
	return ref, true
}

// WriteSection prints the reference after the smoke case table.
func WriteSection(w io.Writer, ref Reference) {
	if ref.Empty() {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Performance reference")
	fmt.Fprintf(w, "Show to visible is the P95 from the product show request to the first presented frame, %d samples after %d warmups.\n", ShowSampleCount, ShowWarmupSamples)
	fmt.Fprintf(w, "Process memory is the Wox process value shown by the platform monitor, after %d distinct queries across plugins and %s after hide.\n",
		QueryCount, HiddenSettle)
	switch {
	case ref.When != "" && ref.CollectedInMs != nil:
		fmt.Fprintf(w, "%s. Collected in %s.\n", ref.When, formatCollected(*ref.CollectedInMs))
	case ref.When != "":
		fmt.Fprintln(w, ref.When)
	case ref.CollectedInMs != nil:
		fmt.Fprintf(w, "Collected in %s.\n", formatCollected(*ref.CollectedInMs))
	}

	rows := []struct {
		label string
		value string
	}{
		{"show to visible p95", formatShow(ref)},
		{fmt.Sprintf("memory after %d queries", QueryCount), formatMemory(ref.QueryMemoryBytes, ref.QueryMemoryError)},
		{fmt.Sprintf("memory after %s hidden", HiddenSettle), formatMemory(ref.HiddenMemoryBytes, ref.HiddenMemoryError)},
	}
	width := 0
	for _, row := range rows {
		if len(row.label) > width {
			width = len(row.label)
		}
	}
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s  %s\n", width, row.label, row.value)
	}
}

// Collect samples show-to-visible latency, memory after QueryCount queries, and memory after HiddenSettle.
// A failed reading is stored on the reference and collection continues with the remaining readings.
// log receives short stage lines; it may be nil.
func Collect(ctx context.Context, client *automationdriver.Client, log io.Writer) (ref Reference) {
	started := time.Now()
	defer func() {
		ms := float64(time.Since(started)) / float64(time.Millisecond)
		ref.CollectedInMs = &ms
	}()
	if client == nil {
		ref.ShowError = "automation client is nil"
		ref.QueryMemoryError = ref.ShowError
		ref.HiddenMemoryError = ref.ShowError
		return ref
	}
	if err := client.Reset(ctx); err != nil {
		message := clipError(fmt.Errorf("reset launcher: %w", err))
		ref.ShowError = message
		ref.QueryMemoryError = message
		ref.HiddenMemoryError = message
		return ref
	}

	stage(log, "performance reference: measuring show to visible")
	samples := make([]time.Duration, 0, ShowWarmupSamples+ShowSampleCount)
	for range ShowWarmupSamples + ShowSampleCount {
		sample, err := sampleShowToVisible(ctx, client)
		if err != nil {
			ref.ShowError = clipError(err)
			break
		}
		samples = append(samples, sample)
	}
	if ref.ShowError == "" && len(samples) == ShowWarmupSamples+ShowSampleCount {
		steady := samples[ShowWarmupSamples:]
		ms := float64(percentile(steady, 0.95)) / float64(time.Millisecond)
		ref.ShowToVisibleP95Ms = &ms
		ref.ShowSamples = len(steady)
	}

	stage(log, fmt.Sprintf("performance reference: running %d queries", QueryCount))
	if err := showLauncher(ctx, client); err != nil {
		ref.QueryMemoryError = clipError(err)
	} else if err := runQueries(ctx, client, QueryCount); err != nil {
		ref.QueryMemoryError = clipError(err)
	} else if bytes, err := processMemory(ctx, client); err != nil {
		ref.QueryMemoryError = clipError(err)
	} else {
		ref.QueryMemoryBytes = &bytes
	}

	stage(log, fmt.Sprintf("performance reference: waiting %s hidden", HiddenSettle))
	if err := hideLauncher(ctx, client); err != nil {
		ref.HiddenMemoryError = clipError(err)
		return ref
	}
	if err := waitQuiet(ctx, HiddenSettle); err != nil {
		ref.HiddenMemoryError = clipError(err)
		return ref
	}
	bytes, err := processMemory(ctx, client)
	if err != nil {
		ref.HiddenMemoryError = clipError(err)
		return ref
	}
	ref.HiddenMemoryBytes = &bytes
	return ref
}

// sampleShowToVisible times one hidden-to-presented-frame activation through the product show path.
func sampleShowToVisible(ctx context.Context, client *automationdriver.Client) (time.Duration, error) {
	if err := hideLauncher(ctx, client); err != nil {
		return 0, err
	}
	if err := client.ResetFrameMetrics(ctx); err != nil {
		return 0, fmt.Errorf("reset frame metrics before show: %w", err)
	}
	started := time.Now()
	if err := client.Show(ctx); err != nil {
		return 0, fmt.Errorf("show launcher: %w", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	}); err != nil {
		return 0, fmt.Errorf("wait for query input: %w", err)
	}
	state, err := client.WindowState(ctx, "primary")
	if err != nil {
		return 0, fmt.Errorf("read launcher state after show: %w", err)
	}
	if !state.Visible {
		return 0, fmt.Errorf("launcher was not visible after show")
	}
	waitCtx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var metrics woxui.FrameMetricsSnapshot
	for {
		var metricsErr error
		metrics, metricsErr = client.FrameMetrics(waitCtx)
		if metricsErr != nil && waitCtx.Err() == nil {
			return 0, fmt.Errorf("read frame metrics after show: %w", metricsErr)
		}
		if metrics.PresentedFrameCount > 0 {
			return time.Since(started), nil
		}
		select {
		case <-waitCtx.Done():
			return 0, fmt.Errorf("wait for presented show frame: %w", waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// showLauncher presents the primary launcher and waits until its query input is in the tree.
func showLauncher(ctx context.Context, client *automationdriver.Client) error {
	if err := client.Show(ctx); err != nil {
		return fmt.Errorf("show launcher: %w", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && state.Visible
	}); err != nil {
		return fmt.Errorf("wait for visible launcher: %w", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	}); err != nil {
		return fmt.Errorf("wait for query input: %w", err)
	}
	return nil
}

// hideLauncher dismisses the primary launcher and waits until it is no longer visible.
func hideLauncher(ctx context.Context, client *automationdriver.Client) error {
	if err := client.Hide(ctx); err != nil {
		return fmt.Errorf("hide launcher: %w", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && !state.Visible
	}); err != nil {
		return fmt.Errorf("wait for launcher to hide: %w", err)
	}
	return nil
}

// runQueries submits the fixed plugin workload and waits for each result generation.
func runQueries(ctx context.Context, client *automationdriver.Client, count int) error {
	queries := performanceQueries()
	if len(queries) != count {
		return fmt.Errorf("performance queries = %d, want %d", len(queries), count)
	}
	for _, query := range queries {
		if err := setQuery(ctx, client, query); err != nil {
			return fmt.Errorf("query %q: %w", query, err)
		}
	}
	return nil
}

// performanceQueries is the comparable 20-search workload.
// Texts without a trigger keyword fan out to every global plugin, and each one
// is chosen so a different plugin does real work: calculator, converter, color,
// apps, system commands, pinyin, file search, folders, and URLs.
// A trigger keyword followed by a space reaches only that plugin, including
// plugins that ignore a global query. Nothing here is executed.
func performanceQueries() []string {
	// A trailing separator would list every child of the home directory.
	home, err := os.UserHomeDir()
	home = strings.TrimRight(strings.TrimSpace(home), `/\`)
	if err != nil || home == "" {
		home = "wox-perf-folder"
	}
	return []string{
		"12*34",
		"meters in 10 km",
		"100 usd in eur",
		"#3a7bd5",
		"notepad",
		"volume",
		"设置",
		"readme",
		home,
		"https://example.com",
		"emoji smile",
		"cb perf-ref",
		"note perf-ref",
		"timer 5m",
		"theme dark",
		"h perf-ref",
		"> echo wox-perf",
		"window left",
		"hotkeys show",
		"screenshot perf-ref",
	}
}

// setQuery changes the launcher query and waits until that generation's results are complete.
func setQuery(ctx context.Context, client *automationdriver.Client, query string) error {
	before, err := client.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read launcher query: %w", err)
	}
	generation := before.Tree.Generation
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		return fmt.Errorf("set launcher query: %w", err)
	}
	_, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		if snapshot.Tree.Generation <= generation {
			return false
		}
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		if !resultsFound || results.Value != "complete" {
			return false
		}
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		return !inputFound || input.Value == query
	})
	if err != nil {
		return fmt.Errorf("wait for launcher query: %w", err)
	}
	return nil
}

// processMemory reads the platform monitor figure for the Wox process.
func processMemory(ctx context.Context, client *automationdriver.Client) (uint64, error) {
	stats, err := client.RuntimeStats(ctx)
	if err != nil {
		return 0, fmt.Errorf("read process memory: %w", err)
	}
	if stats.ProcessMemoryBytes == 0 {
		return 0, fmt.Errorf("process memory was empty")
	}
	return stats.ProcessMemoryBytes, nil
}

// waitQuiet blocks for the hide settle interval, or until ctx is cancelled.
func waitQuiet(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// percentile returns the nearest-rank percentile of a non-empty sample set.
func percentile(samples []time.Duration, quantile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	ordered := slices.Clone(samples)
	slices.Sort(ordered)
	index := int(math.Round(quantile * float64(len(ordered)-1)))
	index = max(0, min(index, len(ordered)-1))
	return ordered[index]
}

func formatShow(ref Reference) string {
	if ref.ShowToVisibleP95Ms == nil {
		return formatUnavailable(ref.ShowError)
	}
	text := fmt.Sprintf("%.1f ms", *ref.ShowToVisibleP95Ms)
	if ref.ShowSamples > 0 {
		text += fmt.Sprintf(" (%d samples)", ref.ShowSamples)
	}
	return text
}

func formatMemory(bytes *uint64, errText string) string {
	if bytes == nil {
		return formatUnavailable(errText)
	}
	return fmt.Sprintf("%.1f MB", float64(*bytes)/(1024*1024))
}

func formatUnavailable(errText string) string {
	if errText == "" {
		return "unavailable"
	}
	return "unavailable: " + errText
}

func formatCollected(ms float64) string {
	elapsed := time.Duration(ms * float64(time.Millisecond))
	if elapsed < 0 {
		elapsed = 0
	}
	elapsed = elapsed.Round(10 * time.Millisecond)
	if elapsed < time.Minute {
		return fmt.Sprintf("%.2fs", elapsed.Seconds())
	}
	minutes := int(elapsed / time.Minute)
	seconds := (elapsed % time.Minute).Seconds()
	return fmt.Sprintf("%dm%05.2fs", minutes, seconds)
}

func clipError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.Join(strings.Fields(err.Error()), " ")
	const limit = 180
	if len(text) > limit {
		text = text[:limit]
	}
	return text
}

func stage(log io.Writer, message string) {
	if log == nil {
		return
	}
	fmt.Fprintln(log, message)
}
