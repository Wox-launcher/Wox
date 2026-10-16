//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"sort"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokereport"
	woxui "wox/ui/runtime"
)

// Test006LauncherPluginFileSearchRealRoot measures an explicitly selected real
// directory through Settings and Launcher. It never modifies files in that root.
func Test006LauncherPluginFileSearchRealRoot(t *testing.T) {
	runRealFileSearchSmoke(t, false)
}

// runRealFileSearchSmoke replaces the standalone benchmark binaries with the
// actual optimized app, preserving functional assertions and report-only costs.
func runRealFileSearchSmoke(t *testing.T, content bool) {
	t.Helper()
	rootKey, queryKey := "WOX_FILESEARCH_SMOKE_ROOT", "WOX_FILESEARCH_SMOKE_QUERY"
	if content {
		rootKey, queryKey = "WOX_FILESEARCH_SMOKE_CONTENT_ROOT", "WOX_FILESEARCH_SMOKE_CONTENT_QUERY"
	}
	root := strings.TrimSpace(os.Getenv(rootKey))
	if root == "" {
		t.Skip("set " + rootKey + " and " + queryKey + " to opt into a real-directory report")
	}
	query := strings.TrimSpace(os.Getenv(queryKey))
	if query == "" {
		t.Fatal(queryKey + " is required")
	}
	root = fileSearchNativeAbsolutePath(t, root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("real root is not an accessible directory: %s (%v)", root, err)
	}
	smoke.CaseWithTimeout(t, 8*time.Minute, func(ctx context.Context, client *automationdriver.Client) {
		report := smokereport.Report{Title: "File Search opt-in real directory", Scope: fmt.Sprintf("root=%s; content=%t; query=%q. Wox uses plugin policy and ranked top results; external tools use literal matching and may have different ignore rules. Timings include UI for Wox; tools include process startup. No forced GC.", root, content, query)}
		defer func() { t.Log(report.Line()) }()
		reportFileSearchMemory(ctx, client, &report, "before indexing")
		started := time.Now()
		field := fileSearchRootsFieldID
		if content {
			smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
			field = fileSearchContentRootsFieldID
			snapshot, err := client.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			checkbox, found := automationdriver.Find(snapshot, fileSearchContentEnabledFieldID)
			if !found {
				t.Fatal("content checkbox missing")
			}
			original := checkbox.Checked
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
				defer cancel()
				smoke.OpenInstalledPluginSettings(t, cleanupCtx, client, fileSearchPluginID)
				setFileSearchContentEnabled(t, cleanupCtx, client, fileSearchContentEnabledFieldID, original)
			})
			setFileSearchContentEnabled(t, ctx, client, fileSearchContentEnabledFieldID, true)
		}
		row := addFileSearchRootField(t, ctx, client, root, field)
		t.Cleanup(func() { removeFileSearchRootField(t, client, root, row, field) })
		waitForFileSearchRootIdle(t, ctx, root)
		if content {
			waitForFileSearchContentReady(t, ctx, root, -1)
		}
		report.Add("settings action to completed index", float64(time.Since(started))/float64(time.Millisecond), "ms")
		reportFileSearchMemory(ctx, client, &report, "after indexing")
		reportFileSearchDisk(&report, "after indexing")
		if content {
			reportFileSearchContentDisk(&report)
		}
		smoke.ShowLauncher(t, ctx, client)
		prefix := "f "
		if content {
			prefix = "f "
		}
		samples := make([]float64, 20)
		for i := range 23 {
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			snapshot := queryFileSearchSnapshot(t, ctx, client, prefix+query)
			smoke.AssertNoDiagnostics(t, snapshot)
			matches := 0
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.HasPrefix(strings.ToLower(node.Description), strings.ToLower(root+string(os.PathSeparator))) {
					matches++
				}
			}
			if matches == 0 {
				t.Fatalf("real-root query %q returned no files in %s; choose a known matching query", query, root)
			}
			if i == 0 {
				report.Add("visible matching results in root", float64(matches), "files")
			}
			if i >= 3 {
				samples[i-3] = float64(time.Since(started)) / float64(time.Millisecond)
			}
		}
		sort.Float64s(samples)
		report.Add("completed query p50", samples[9], "ms")
		report.Add("completed query p95", samples[18], "ms")
		reportFileSearchMemory(ctx, client, &report, "after queries")
		if content {
			reportFileSearchTool(ctx, &report, "rg", []string{"-l", "-i", "-F", "--", query, root})
		} else {
			reportFileSearchTool(ctx, &report, "fd", []string{"--type", "f", "--fixed-strings", "--ignore-case", "--", query, root})
			reportFileSearchTool(ctx, &report, "rg", []string{"--files", "--", root})
		}
	})
}

// toolOutputCounter avoids retaining real filenames or unbounded tool output.
type toolOutputCounter struct {
	bytes, lines int64
	last         byte
}

func (c *toolOutputCounter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			c.lines++
		}
	}
	if len(p) != 0 {
		c.last = p[len(p)-1]
	}
	c.bytes += int64(len(p))
	return len(p), nil
}

// reportFileSearchTool records unavailable optional tools as missing metrics.
func reportFileSearchTool(ctx context.Context, report *smokereport.Report, name string, args []string) {
	executable, err := exec.LookPath(name)
	if err != nil {
		report.Missing(name, err)
		return
	}
	toolCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(toolCtx, executable, args...)
	var output toolOutputCounter
	command.Stdout = &output
	started := time.Now()
	err = command.Run()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || name != "rg" || exit.ExitCode() != 1 {
			report.Missing(name, err)
			return
		}
	}
	report.Add(name+" elapsed", float64(time.Since(started))/float64(time.Millisecond), "ms")
	if output.bytes != 0 && output.last != '\n' {
		output.lines++
	}
	kind := "matched files"
	if len(args) > 0 && args[0] == "--files" {
		kind = "enumerated files (no query filter)"
	}
	report.Add(name+" "+kind, float64(output.lines), "files")
}
