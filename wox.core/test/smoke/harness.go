package smoke

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	woxwidget "wox/ui/widget"
)

// Case runs one smoke case between mandatory resets of the shared Wox process.
func Case(t *testing.T, run func(context.Context, *automationdriver.Client)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	client := sharedClient(t, ctx)
	if err := client.Reset(ctx); err != nil {
		t.Fatalf("reset shared Wox before case: %v", err)
	}
	t.Cleanup(func() {
		resetCtx, resetCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer resetCancel()
		if err := client.Reset(resetCtx); err != nil {
			t.Errorf("reset shared Wox after case: %v", err)
		}
	})
	run(ctx, client)
}

// sharedClient connects one test package to the suite-owned Wox process.
func sharedClient(t *testing.T, ctx context.Context) *automationdriver.Client {
	t.Helper()
	infoFile := strings.TrimSpace(os.Getenv(automationdriver.SharedInfoFileEnvironment))
	if infoFile == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedInfoFileEnvironment)
	}
	info, err := automationdriver.ReadInfo(ctx, infoFile)
	if err != nil {
		t.Fatalf("read shared Wox automation endpoint: %v", err)
	}
	client, err := automationdriver.NewClient(info)
	if err != nil {
		t.Fatalf("connect to shared Wox automation endpoint: %v", err)
	}
	return client
}

// ShowLauncher presents the launcher and waits for its query input.
func ShowLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Show(ctx); err != nil {
		t.Fatalf("show launcher: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	}); err != nil {
		t.Fatalf("wait for query input: %v", err)
	}
}
