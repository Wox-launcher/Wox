//go:build wox_ui_smoke

package aichat

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxclipboard "wox/util/clipboard"
)

const chatDraftText = "keep this draft"

// Test001LauncherPluginAIChatPasteFileAttachment verifies Chat paste appends a copied file to the current draft.
// Flow: query chat -> enter draft text -> copy a local file onto the clipboard -> paste into the composer.
// Evidence: a chat attachment chip appears, the draft text is unchanged, and removing the chip keeps the same conversation input.
func Test001LauncherPluginAIChatPasteFileAttachment(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("file-list clipboard paste for Chat attachments is automated on Windows; other platforms keep unit coverage")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		path := filepath.Join(t.TempDir(), "wox-chat-smoke.txt")
		if err := os.WriteFile(path, []byte("attachment body"), 0600); err != nil {
			t.Fatalf("write chat attachment fixture: %v", err)
		}

		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "chat ")
		inputID := waitForChatInput(t, ctx, client)
		if err := client.Perform(ctx, inputID, woxui.AccessibilityActionSetValue, chatDraftText); err != nil {
			t.Fatalf("enter chat draft: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, inputID)
			return found && input.Value == chatDraftText
		}); err != nil {
			t.Fatalf("wait for chat draft text: %v", err)
		}

		injectFileClipboard(t, path)
		if err := client.Perform(ctx, inputID, woxui.AccessibilityActionPaste, ""); err != nil {
			t.Fatalf("paste copied file into chat: %v", err)
		}

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, inputID)
			_, attachmentFound := findChatAttachment(snapshot)
			return inputFound && input.Value == chatDraftText && attachmentFound
		})
		if err != nil {
			t.Fatalf("wait for chat file attachment: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		attachment, found := findChatAttachment(snapshot)
		if !found || !strings.Contains(attachment.Label, filepath.Base(path)) {
			t.Fatalf("chat attachment label = %q", attachment.Label)
		}
		dismissID := "chat-attachment-dismiss-" + strings.TrimPrefix(attachment.AutomationID, "chat-attachment-")
		if err := client.Perform(ctx, dismissID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("remove chat attachment: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, inputID)
			_, attachmentFound := findChatAttachment(snapshot)
			return inputFound && input.Value == chatDraftText && !attachmentFound
		})
		if err != nil {
			t.Fatalf("wait for chat attachment removal: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func waitForChatInput(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.FindByAutomationIDPrefix(snapshot, "chat-input-")
		return found
	})
	if err != nil {
		t.Fatalf("wait for chat composer: %v", err)
	}
	input, found := automationdriver.FindByAutomationIDPrefix(snapshot, "chat-input-")
	if !found {
		t.Fatal("chat composer was not exposed")
	}
	return input.AutomationID
}

func findChatAttachment(snapshot woxwidget.AutomationSnapshot) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "chat-attachment-") && !strings.HasPrefix(node.AutomationID, "chat-attachment-dismiss-") {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

func injectFileClipboard(t *testing.T, path string) {
	t.Helper()
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", `$path = [Console]::In.ReadToEnd().Trim(); Add-Type -AssemblyName System.Windows.Forms; $files = New-Object System.Collections.Specialized.StringCollection; [void]$files.Add($path); [System.Windows.Forms.Clipboard]::SetFileDropList($files)`)
	command.Stdin = strings.NewReader(path)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("write file clipboard: %v: %s", err, stderr.String())
	}
	paths, err := woxclipboard.ReadFilePaths()
	if err != nil {
		t.Fatalf("read file clipboard: %v", err)
	}
	for _, copied := range paths {
		if filepath.Clean(copied) == filepath.Clean(path) {
			return
		}
	}
	t.Fatalf("file clipboard = %q, want %q", paths, path)
}
