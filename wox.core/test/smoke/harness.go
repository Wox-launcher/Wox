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

// CaseTimeout is the budget for one smoke case body. Individual waits still cap at ActionTimeout.
const CaseTimeout = 30 * time.Second

// Case runs one smoke case between mandatory resets of the shared Wox process.
func Case(t *testing.T, run func(context.Context, *automationdriver.Client)) {
	t.Helper()
	connectCtx, connectCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	client := sharedClient(t, connectCtx)
	connectCancel()
	resetCtx, resetCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	err := client.Reset(resetCtx)
	resetCancel()
	if err != nil {
		t.Fatalf("reset shared Wox before case: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), CaseTimeout)
	t.Cleanup(cancel)
	t.Cleanup(func() {
		afterCtx, afterCancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
		defer afterCancel()
		if err := client.Reset(afterCtx); err != nil {
			t.Errorf("reset shared Wox after case: %v", err)
		}
	})
	run(ctx, client)
}

// SharedClient connects lifecycle phases that intentionally terminate Wox before the test returns.
func SharedClient(t *testing.T, ctx context.Context) *automationdriver.Client {
	t.Helper()
	return sharedClient(t, ctx)
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
