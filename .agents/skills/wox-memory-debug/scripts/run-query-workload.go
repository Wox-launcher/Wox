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
	mode := flag.String("mode", "queries", "queries or profile")
	count := flag.Int("count", 10, "number of queries")
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

// captureHeapProfile invokes Wox's development heap-profile action through the launcher.
func captureHeapProfile(ctx context.Context, client *automationdriver.Client) error {
	if err := showAndWaitForInput(ctx, client); err != nil {
		return err
	}
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "memory_profiling"); err != nil {
		return err
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := findMemoryProfileResult(snapshot)
		return found
	})
	if err != nil {
		return err
	}
	resultID, _ := findMemoryProfileResult(snapshot)
	if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
		return err
	}
	time.Sleep(time.Second)
	fmt.Printf("activated=%s\n", resultID)
	return client.Hide(ctx)
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
