//go:build wox_ui_smoke

package aichat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherPluginAIChatProjects exercises real project storage and both composer choices.
// Flow: create with two directories -> change primary -> switch out/in without losing draft ->
// hover project actions -> edit name and directories -> confirm deletion.
// Evidence: draft and project membership survive selection, hover follows the pointer, and open surfaces have no layout diagnostics.
func Test002LauncherPluginAIChatProjects(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "chat ")
		inputID := waitForChatInput(t, ctx, client)
		perform := func(id string, action woxui.AccessibilityAction, value string) {
			t.Helper()
			if err := client.Perform(ctx, id, action, value); err != nil {
				t.Fatalf("%s: %v", id, err)
			}
		}
		prefixID := func(prefix string) string {
			t.Helper()
			snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, ok := automationdriver.FindByAutomationIDPrefix(s, prefix)
				return ok
			})
			if err != nil {
				t.Fatalf("wait for %s: %v", prefix, err)
			}
			node, _ := automationdriver.FindByAutomationIDPrefix(snapshot, prefix)
			return node.AutomationID
		}
		click := func(id string) { perform(id, woxui.AccessibilityActionActivate, "") }
		set := func(id, value string) { perform(id, woxui.AccessibilityActionSetValue, value) }
		checkSurface := func(name string) {
			t.Helper()
			// Wait for presentation so state changes and native pixels describe the same surface.
			for range 2 {
				before, err := client.FrameMetrics(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if err := client.RequestFrame(ctx); err != nil {
					t.Fatal(err)
				}
				snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
					after, err := client.FrameMetrics(ctx)
					return err == nil && after.PresentedFrameCount > before.PresentedFrameCount
				})
				if err != nil {
					t.Fatal(err)
				}
				smoke.AssertNoDiagnostics(t, snapshot)
			}
			if directory := os.Getenv("WOX_SMOKE_ARTIFACT_DIR"); directory != "" {
				if err := client.Capture(ctx, filepath.Join(directory, "chat-project-"+name+".png")); err != nil {
					t.Fatal(err)
				}
			}
		}
		click(prefixID("chat-history-"))
		checkSurface("sidebar-empty")
		click("chat-projects-section-new")
		prefixID("chat-project-name")
		const name = "Wox project smoke"
		set("chat-project-name", name)
		checkSurface("create")
		for _, directory := range []string{t.TempDir(), t.TempDir()} {
			set("chat-project-directory-input", directory)
			click("chat-project-add-directory")
		}
		click("chat-project-directory-1")
		checkSurface("directories")
		click("chat-project-save")
		selectorID := prefixID("chat-project-selector-")
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(s, selectorID)
			_, modal := automationdriver.Find(s, "chat-project-dialog")
			return ok && node.Label == name && !modal
		}); err != nil {
			t.Fatalf("save project: %v", err)
		}
		set(inputID, chatDraftText)
		checkSurface("sidebar")
		click(selectorID)
		prefixID("chat-project-search-")
		checkSurface("picker")
		click(prefixID("chat-project-none"))
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(s, selectorID)
			input, found := automationdriver.Find(s, inputID)
			return ok && node.Label != name && found && input.Value == chatDraftText
		}); err != nil {
			t.Fatalf("no-project selection lost draft: %v", err)
		}
		click(selectorID)
		choiceID := prefixID("chat-project-choice-")
		click(choiceID)
		groupID := strings.Replace(choiceID, "chat-project-choice-", "chat-project-group-", 1)
		if _, err := client.MovePointerTo(ctx, groupID); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(s, groupID+"-group")
			return ok && node.Hovered
		}); err != nil {
			t.Fatalf("project row did not expose hover actions: %v", err)
		}
		checkSurface("sidebar-hover")
		if err := client.LeavePointer(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(s, groupID+"-group")
			return ok && !node.Hovered
		}); err != nil {
			t.Fatalf("project row kept hover actions after pointer left: %v", err)
		}

		click(strings.Replace(choiceID, "chat-project-choice-", "chat-project-group-", 1) + "-edit")
		set("chat-project-name", name+" renamed")
		click("chat-project-remove-0")
		click("chat-project-save")
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			node, ok := automationdriver.Find(s, selectorID)
			return ok && strings.Contains(node.Label, "renamed")
		}); err != nil {
			t.Fatalf("rename project: %v", err)
		}
		click(groupID + "-delete")
		prefixID("chat-project-delete-dialog")
		checkSurface("delete-confirmation")
		click("chat-project-delete-cancel")
		click(groupID + "-delete")
		click("chat-project-delete-confirm")
		snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, modal := automationdriver.Find(s, "chat-project-delete-dialog")
			_, selector := automationdriver.Find(s, selectorID)
			input, ok := automationdriver.Find(s, inputID)
			return !modal && !selector && ok && input.Value == chatDraftText
		})
		if err != nil {
			t.Fatalf("delete project preserved draft: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
