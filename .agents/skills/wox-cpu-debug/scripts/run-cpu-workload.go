package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
	mode := flag.String("mode", "queries", "queries, hidden, profile-queries, or profile-hidden")
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
