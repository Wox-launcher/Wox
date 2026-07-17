package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"wox/test/automationdriver"
	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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
	mode := flag.String("mode", "queries", "queries, settings, or profile")
	count := flag.Int("count", 10, "number of queries or settings cycles")
	seed := flag.Int64("seed", 1, "random seed")
	fixedQuery := flag.String("query", "", "fixed query to repeat instead of random queries")
	flag.Parse()

	if strings.TrimSpace(*infoPath) == "" {
		panic("-info is required")
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	switch *mode {
	case "queries":
		err = runQueries(ctx, client, *count, *seed, *fixedQuery)
	case "settings":
		err = runSettingsCycles(ctx, client, *count)
	case "profile":
		err = captureHeapProfile(ctx, client)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		panic(err)
	}
}

// runQueries replays one deterministic random query block through the real launcher semantics tree.
func runQueries(ctx context.Context, client *automationdriver.Client, count int, seed int64, fixedQuery string) error {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return err
	}
	random := rand.New(rand.NewSource(seed))
	for index := 0; index < count; index++ {
		query := fixedQuery
		if query == "" {
			query = queryPool[random.Intn(len(queryPool))]
		}
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			return fmt.Errorf("set query %d %q: %w", index+1, query, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == query
		}); err != nil {
			return fmt.Errorf("wait for query %d %q: %w", index+1, query, err)
		}
		time.Sleep(350 * time.Millisecond)
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			return err
		}
		resultCount := 0
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") {
				resultCount++
			}
		}
		fmt.Printf("query=%02d value=%q visible_results=%d generation=%d\n", index+1, query, resultCount, snapshot.Tree.Generation)

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
			return fmt.Errorf("clear query %d: %w", index+1, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			node, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && node.Value == ""
		}); err != nil {
			return fmt.Errorf("wait for clear %d: %w", index+1, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return client.Hide(ctx)
}

// runSettingsCycles repeatedly opens the real settings window and waits for its full close lifecycle.
func runSettingsCycles(ctx context.Context, client *automationdriver.Client, count int) error {
	if count <= 0 {
		return fmt.Errorf("settings cycle count must be positive")
	}
	for index := 0; index < count; index++ {
		cycleCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		err := runSettingsCycle(cycleCtx, client, index+1)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

// runSettingsCycle opens settings through the system result, then closes it back to the hidden launcher surface.
func runSettingsCycle(ctx context.Context, client *automationdriver.Client, cycle int) error {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return fmt.Errorf("show launcher for settings cycle %d: %w", cycle, err)
	}
	const query = "open_wox_settings"
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		return fmt.Errorf("set settings query for cycle %d: %w", cycle, err)
	}
	snapshot, err := waitForSnapshot(ctx, client, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.query.input")
		_, resultFound := findWoxSettingsResult(snapshot)
		return found && node.Value == query && resultFound
	})
	if err != nil {
		return fmt.Errorf("wait for settings result in cycle %d: %w", cycle, err)
	}

	var resultID string
	var activateErr error
	for attempt := 0; attempt < 5; attempt++ {
		resultID, _ = findWoxSettingsResult(snapshot)
		activateErr = client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, "")
		if activateErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		snapshot, err = waitForSnapshot(ctx, client, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := findWoxSettingsResult(snapshot)
			return found
		})
		if err != nil {
			return fmt.Errorf("refresh settings result in cycle %d: %w", cycle, err)
		}
	}
	if activateErr != nil {
		return fmt.Errorf("activate settings result in cycle %d: %w", cycle, activateErr)
	}

	opened, err := waitForSnapshot(ctx, client, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "settings-search-field")
		return found
	})
	if err != nil {
		return fmt.Errorf("wait for settings window in cycle %d: %w", cycle, err)
	}
	time.Sleep(time.Second)
	if settled, snapshotErr := client.Snapshot(ctx); snapshotErr == nil {
		opened = settled
	}
	if err := client.Hide(ctx); err != nil {
		return fmt.Errorf("close settings window in cycle %d: %w", cycle, err)
	}

	closed, err := waitForSnapshot(ctx, client, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, launcherFound := automationdriver.Find(snapshot, "launcher.query.input")
		_, settingsFound := automationdriver.Find(snapshot, "settings-search-field")
		return launcherFound && !settingsFound
	})
	if err != nil {
		return fmt.Errorf("wait for settings close in cycle %d: %w", cycle, err)
	}
	// The hidden launcher does not produce a new frame after query mutation, so
	// briefly show it to confirm the cleared state before returning to idle.
	if err := client.Show(ctx); err != nil {
		return fmt.Errorf("show launcher to clear settings query in cycle %d: %w", cycle, err)
	}
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
		return fmt.Errorf("clear settings query in cycle %d: %w", cycle, err)
	}
	closed, err = waitForSnapshot(ctx, client, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found && node.Value == ""
	})
	if err != nil {
		return fmt.Errorf("wait for settings query clear in cycle %d: %w", cycle, err)
	}
	if err := client.Hide(ctx); err != nil {
		return fmt.Errorf("hide launcher after settings cycle %d: %w", cycle, err)
	}
	time.Sleep(250 * time.Millisecond)
	fmt.Printf("settings_cycle=%02d opened_generation=%d opened_nodes=%d closed_generation=%d\n", cycle, opened.Tree.Generation, len(opened.Tree.Nodes), closed.Tree.Generation)
	return nil
}

// waitForSnapshot polls across host switches because settings and launcher have independent semantics generations.
func waitForSnapshot(ctx context.Context, client *automationdriver.Client, predicate func(woxwidget.AutomationSnapshot) bool) (woxwidget.AutomationSnapshot, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			return woxwidget.AutomationSnapshot{}, err
		}
		if predicate(snapshot) {
			return snapshot, nil
		}
		select {
		case <-ctx.Done():
			return woxwidget.AutomationSnapshot{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// findWoxSettingsResult finds the localized system command result without relying on query-scoped result IDs.
func findWoxSettingsResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(node.Label))
		if strings.Contains(label, "wox") && (strings.Contains(label, "setting") || strings.Contains(label, "设置") || strings.Contains(label, "настрой") || strings.Contains(label, "configura")) {
			return node.AutomationID, true
		}
	}
	return "", false
}

// captureHeapProfile invokes Wox's development heap-profile action through the launcher.
func captureHeapProfile(ctx context.Context, client *automationdriver.Client) error {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return err
	}
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "memory_profiling"); err != nil {
		return err
	}
	var resultID string
	for attempt := 0; attempt < 5; attempt++ {
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := findMemoryProfileResult(snapshot)
			return found
		})
		if err != nil {
			return err
		}
		resultID, _ = findMemoryProfileResult(snapshot)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err == nil {
			time.Sleep(time.Second)
			fmt.Printf("activated=%s\n", resultID)
			return client.Hide(ctx)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("memory profile result kept changing before activation")
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

func findMemoryProfileResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(node.Label))
		if strings.Contains(label, "memory") || strings.Contains(label, "内存") {
			return node.AutomationID, true
		}
	}
	return "", false
}
