package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wox/test/automationdriver"
	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const cpuProfileDuration = 30 * time.Second

var queryPool = []string{
	"1+1",
	"42*17",
	"settings",
	"wox",
	"readme",
	"main.go",
	"terminal",
	"chrome",
	"clipboard",
	"theme",
	"plugin",
	"memory",
	"github",
	"calculator",
	"json",
	"calendar",
}

func main() {
	infoPath := flag.String("info", "", "automation endpoint info file")
	mode := flag.String("mode", "queries", "queries, latency, stats, hidden, profile-queries, or profile-hidden")
	queryCount := flag.Int("query-count", 160, "number of completed queries in latency mode")
	duration := flag.Duration("duration", 30*time.Second, "workload duration for queries or hidden mode")
	seed := flag.Int64("seed", 1, "random seed")
	fixedQuery := flag.String("query", "", "fixed query to repeat instead of random queries")
	outputPath := flag.String("output", "", "destination for profile modes")
	readyPath := flag.String("ready-file", "", "optional file written after the measured state starts")
	flag.Parse()

	if strings.TrimSpace(*infoPath) == "" {
		panic("-info is required")
	}
	if *duration <= 0 {
		panic("-duration must be positive")
	}

	data, err := os.ReadFile(*infoPath)
	if err != nil {
		panic(err)
	}
	var info automation.Info
	if err := json.Unmarshal(data, &info); err != nil {
		panic(err)
	}
	client, err := automationdriver.NewClient(info)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	switch *mode {
	case "latency":
		err = runLatency(ctx, client, *queryCount, *seed, *fixedQuery, *outputPath, *readyPath)
	case "stats":
		stats, statsErr := client.RuntimeStats(ctx)
		err = statsErr
		if err == nil {
			err = json.NewEncoder(os.Stdout).Encode(stats)
		}
	case "queries":
		err = runQueriesFor(ctx, client, *duration, *seed, *fixedQuery, *readyPath)
	case "hidden":
		err = runHidden(ctx, client, *duration, *readyPath)
	case "profile-queries":
		err = runProfile(ctx, client, true, *seed, *fixedQuery, *outputPath, *readyPath)
	case "profile-hidden":
		err = runProfile(ctx, client, false, *seed, *fixedQuery, *outputPath, *readyPath)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		panic(err)
	}
}

// queryLatency keeps raw samples so aggregate p95 cannot hide a slow query family.
type queryLatency struct {
	Query         string   `json:"query"`
	FirstResultMS *float64 `json:"firstResultMs"`
	CompleteMS    float64  `json:"completeMs"`
	ResultCount   int      `json:"resultCount"`
}

// runLatency measures from input dispatch through a current-query semantics frame.
// Unlike the CPU replay, every query finishes; empty results have no first-result sample.
func runLatency(ctx context.Context, client *automationdriver.Client, count int, seed int64, fixedQuery, outputPath, readyPath string) error {
	if count <= 0 || outputPath == "" {
		return fmt.Errorf("latency mode requires positive -query-count and -output")
	}
	if err := showAndWaitForInput(ctx, client); err != nil {
		return err
	}
	if err := signalReady(readyPath); err != nil {
		return err
	}
	random := rand.New(rand.NewSource(seed))
	samples := make([]queryLatency, 0, count)
	for index := range count {
		// Finish the empty query before timing, so retained rows and completion
		// from the previous query cannot satisfy the next observation.
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
			return err
		}
		if _, err := waitQuery(ctx, client, "", "", nil); err != nil {
			return fmt.Errorf("clear before query %d: %w", index+1, err)
		}
		before, err := client.QueryState(ctx)
		if err != nil {
			return err
		}
		query := fixedQuery
		if query == "" {
			query = queryPool[random.Intn(len(queryPool))]
		}
		sample := queryLatency{Query: query}
		started := time.Now()
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			return err
		}
		state, err := waitQuery(ctx, client, query, before.QueryID, func(state automation.QueryState) {
			if state.Visible && sample.FirstResultMS == nil {
				elapsed := float64(time.Since(started)) / float64(time.Millisecond)
				sample.FirstResultMS = &elapsed
			}
		})
		if err != nil {
			return fmt.Errorf("query %d %q: %w", index+1, query, err)
		}
		sample.CompleteMS = float64(time.Since(started)) / float64(time.Millisecond)
		sample.ResultCount = state.ResultCount
		samples = append(samples, sample)
		if err := waitUntil(ctx, time.Now().Add(50*time.Millisecond)); err != nil {
			return err
		}
	}
	if err := client.Hide(ctx); err != nil {
		return err
	}
	first, complete := make([]float64, 0, count), make([]float64, 0, count)
	for _, sample := range samples {
		if sample.FirstResultMS != nil {
			first = append(first, *sample.FirstResultMS)
		}
		complete = append(complete, sample.CompleteMS)
	}
	sort.Float64s(first)
	sort.Float64s(complete)
	report := struct {
		Seed               int64          `json:"seed"`
		PollIntervalMS     int            `json:"pollIntervalMs"`
		FirstResultSamples int            `json:"firstResultSamples"`
		FirstP50MS         float64        `json:"firstP50Ms"`
		FirstP95MS         float64        `json:"firstP95Ms"`
		CompleteP50MS      float64        `json:"completeP50Ms"`
		CompleteP95MS      float64        `json:"completeP95Ms"`
		Samples            []queryLatency `json:"samples"`
	}{seed, 5, len(first), percentile(first, .5), percentile(first, .95), percentile(complete, .5), percentile(complete, .95), samples}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("queries=%d first_samples=%d first_p50_ms=%.2f first_p95_ms=%.2f complete_p50_ms=%.2f complete_p95_ms=%.2f output=%s\n", count, len(first), report.FirstP50MS, report.FirstP95MS, report.CompleteP50MS, report.CompleteP95MS, outputPath)
	return nil
}

// waitQuery bounds each query and observes current results at 5 ms resolution.
func waitQuery(ctx context.Context, client *automationdriver.Client, query, previousID string, observe func(automation.QueryState)) (automation.QueryState, error) {
	ctx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	for {
		state, err := client.QueryState(ctx)
		if err != nil {
			return state, err
		}
		if state.QueryText == query && state.QueryID != previousID {
			if observe != nil {
				observe(state)
			}
			if state.Complete {
				return state, nil
			}
		}
		if err := waitUntil(ctx, time.Now().Add(5*time.Millisecond)); err != nil {
			return state, err
		}
	}
}

// percentile uses the nearest-rank definition on sorted samples.
func percentile(sorted []float64, fraction float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[max(0, int(math.Ceil(float64(len(sorted))*fraction))-1)]
}

// runProfile activates Wox's fixed 30-second CPU profile and drives the selected state.
func runProfile(ctx context.Context, client *automationdriver.Client, queries bool, seed int64, fixedQuery, outputPath, readyPath string) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("-output is required for profile modes")
	}
	activatedAt, err := activateCPUProfile(ctx, client)
	if err != nil {
		return err
	}
	deadline := activatedAt.Add(cpuProfileDuration)
	if queries {
		if err := showAndWaitForInput(ctx, client); err != nil {
			return err
		}
		if err := signalReady(readyPath); err != nil {
			return err
		}
		if err := replayQueriesUntil(ctx, client, deadline, seed, fixedQuery); err != nil {
			return err
		}
	} else {
		if err := client.Hide(ctx); err != nil {
			return fmt.Errorf("hide launcher for CPU profile: %w", err)
		}
		if err := signalReady(readyPath); err != nil {
			return err
		}
		if err := waitUntil(ctx, deadline); err != nil {
			return err
		}
	}
	if err := waitUntil(ctx, deadline.Add(1500*time.Millisecond)); err != nil {
		return err
	}
	profilePath, err := defaultCPUProfilePath()
	if err != nil {
		return err
	}
	if err := copyCompletedProfile(profilePath, outputPath, activatedAt); err != nil {
		return err
	}
	fmt.Printf("profile=%s duration=%s activated_at=%s\n", outputPath, cpuProfileDuration, activatedAt.Format(time.RFC3339Nano))
	return nil
}

// runQueriesFor establishes the launcher state before starting the measured duration.
func runQueriesFor(ctx context.Context, client *automationdriver.Client, duration time.Duration, seed int64, fixedQuery, readyPath string) error {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return err
	}
	if err := signalReady(readyPath); err != nil {
		return err
	}
	return replayQueriesUntil(ctx, client, time.Now().Add(duration), seed, fixedQuery)
}

// replayQueriesUntil continuously replays safe mixed or fixed queries until the deadline.
func replayQueriesUntil(ctx context.Context, client *automationdriver.Client, deadline time.Time, seed int64, fixedQuery string) error {
	random := rand.New(rand.NewSource(seed))
	count := 0
	for time.Now().Before(deadline) {
		query := fixedQuery
		if query == "" {
			query = queryPool[random.Intn(len(queryPool))]
		}
		count++
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			return fmt.Errorf("set query %d %q: %w", count, query, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == query
		}); err != nil {
			return fmt.Errorf("wait for query %d %q: %w", count, query, err)
		}
		time.Sleep(200 * time.Millisecond)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
			return fmt.Errorf("clear query %d: %w", count, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == ""
		}); err != nil {
			return fmt.Errorf("wait for query clear %d: %w", count, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := client.Hide(ctx); err != nil {
		return err
	}
	fmt.Printf("queries=%d seed=%d fixed_query=%q\n", count, seed, fixedQuery)
	return nil
}

// runHidden establishes the hidden state before signaling an external sampler.
func runHidden(ctx context.Context, client *automationdriver.Client, duration time.Duration, readyPath string) error {
	if err := client.Hide(ctx); err != nil {
		return err
	}
	if err := signalReady(readyPath); err != nil {
		return err
	}
	return waitUntil(ctx, time.Now().Add(duration))
}

// activateCPUProfile invokes the development CPU-profile action through the launcher.
func activateCPUProfile(ctx context.Context, client *automationdriver.Client) (time.Time, error) {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return time.Time{}, err
	}
	const query = "cpu_profiling"
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		return time.Time{}, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := findCPUProfileResult(snapshot)
			return found
		})
		if err != nil {
			return time.Time{}, err
		}
		resultID, _ := findCPUProfileResult(snapshot)
		activatedAt := time.Now()
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err == nil {
			fmt.Printf("activated=%s\n", resultID)
			return activatedAt, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return time.Time{}, fmt.Errorf("CPU profile result kept changing before activation")
}

func showAndWaitForInput(ctx context.Context, client *automationdriver.Client) error {
	if err := client.Show(ctx); err != nil {
		return err
	}
	_, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	})
	return err
}

func findCPUProfileResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(node.Label)), "cpu") {
			return node.AutomationID, true
		}
	}
	return "", false
}

func waitUntil(ctx context.Context, deadline time.Time) error {
	delay := time.Until(deadline)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func signalReady(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0o644)
}

func defaultCPUProfilePath() (string, error) {
	if dataDir := strings.TrimSpace(os.Getenv("WOX_TEST_DATA_DIR")); dataDir != "" {
		return filepath.Join(dataDir, "cpu.prof"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wox", "cpu.prof"), nil
}

func copyCompletedProfile(source, destination string, activatedAt time.Time) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat CPU profile: %w", err)
	}
	if info.Size() == 0 || info.ModTime().Before(activatedAt.Add(-2*time.Second)) {
		return fmt.Errorf("CPU profile was not completed for this capture: size=%d modified=%s", info.Size(), info.ModTime().Format(time.RFC3339Nano))
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}
