//go:build darwin && wox_ui_smoke

package query

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test044LauncherSecureInputFocus checks that the query gains native keyboard
// focus while another application has an active password field.
func Test044LauncherSecureInputFocus(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher before focusing password field: %v", err)
		}
		binary := filepath.Join(t.TempDir(), "secure-input-fixture")
		compile := exec.CommandContext(ctx, "clang", "-fblocks", "-Wno-deprecated-declarations", "-framework", "Cocoa", "-framework", "Carbon",
			"testdata/secure_input_darwin.m", "-o", binary)
		if output, err := compile.CombinedOutput(); err != nil {
			t.Fatalf("compile password field fixture: %v\n%s", err, output)
		}

		fixture := exec.CommandContext(ctx, binary)
		output, err := fixture.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := fixture.Start(); err != nil {
			t.Fatalf("start password field fixture: %v", err)
		}
		defer func() {
			_ = fixture.Process.Kill()
			_ = fixture.Wait()
		}()
		ready := bufio.NewScanner(output)
		if !ready.Scan() || ready.Text() != "secure input ready" {
			t.Fatal("fixture password field did not obtain focus with Secure Input enabled")
		}

		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			query, found := automationdriver.Find(snapshot, "launcher.query.input")
			return snapshot.Tree.WindowFocused && found && query.Focused
		}); err != nil {
			t.Fatalf("launcher query did not gain keyboard focus: %v", err)
		}
	})
}
