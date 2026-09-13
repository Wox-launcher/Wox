//go:build wox_ui_smoke

package attention

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"

	_ "github.com/mattn/go-sqlite3"
)

const (
	attentionFixturePluginID    = "0cb0d21c-45ce-4fe0-987e-24d645eca58c"
	attentionFixtureKey         = "attention-smoke-item"
	attentionFixtureIdentityKey = attentionFixturePluginID + ":" + attentionFixtureKey
	attentionFixtureTitle       = "Attention smoke item"
	attentionFixtureQuery       = "wox-smoke attention "
	attentionUnreadBadgeID      = "launcher.query.attention"
)

type attentionFixtureState struct {
	count       int
	isRead      bool
	fingerprint string
}

// prepareAttentionFixture isolates the shared-process database row owned by these smoke cases.
func prepareAttentionFixture(t *testing.T) {
	t.Helper()
	deleteAttentionFixture(t)
	t.Cleanup(func() { deleteAttentionFixture(t) })
}

// activateFixtureAction invokes one deterministic fixture action through the launcher action panel.
func activateFixtureAction(t *testing.T, ctx context.Context, client *automationdriver.Client, actionPrefix string) {
	t.Helper()
	completedTitle := ""
	switch actionPrefix {
	case "action-result-push-fresh-attention-":
		completedTitle = "Attention smoke fixture: fresh pushed"
	case "action-result-repeat-attention-":
		completedTitle = "Attention smoke fixture: repeat pushed"
	default:
		t.Fatalf("unsupported Attention fixture action %q", actionPrefix)
	}
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, attentionFixtureQuery)
	resultID, found := smoke.FindLauncherResult(snapshot, "Attention smoke fixture")
	if !found {
		t.Fatal("Attention smoke fixture result was not found")
	}
	smoke.SelectLauncherResult(t, ctx, client, resultID)
	snapshot = smoke.OpenResultActionPanel(t, ctx, client)
	action, found := automationdriver.FindByAutomationIDPrefix(snapshot, actionPrefix)
	if !found {
		t.Fatalf("Attention fixture action %q was not found", actionPrefix)
	}
	if err := client.Perform(ctx, action.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate Attention fixture action %q: %v", actionPrefix, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		return smoke.HasLauncherResultLabel(snapshot, completedTitle)
	}); err != nil {
		t.Fatalf("wait for Attention fixture action %q to complete: %v", actionPrefix, err)
	}
	smoke.WaitForResultActionsClosed(t, ctx, client)
}

// showUnreadAttentionBadge pushes the fixture and waits until the query-box unread badge is visible.
func showUnreadAttentionBadge(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	activateFixtureAction(t, ctx, client, "action-result-push-fresh-attention-")
	waitForAttentionState(t, ctx, func(state attentionFixtureState) bool {
		return state.count == 1 && !state.isRead && state.fingerprint != ""
	})
	smoke.ReplaceLauncherQuery(t, ctx, client, "")
	return waitForAttentionUnreadBadge(t, ctx, client)
}

// waitForAttentionUnreadBadge waits until the query-box badge reports a non-zero unread count.
func waitForAttentionUnreadBadge(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		badge, found := automationdriver.Find(snapshot, attentionUnreadBadgeID)
		if !found {
			return false, "unread badge missing"
		}
		if badge.Value == "" || badge.Value == "0" {
			return false, fmt.Sprintf("unread badge value %q", badge.Value)
		}
		return true, ""
	})
	if err != nil {
		t.Fatalf("wait for Attention unread badge: %v", err)
	}
	return snapshot
}

// waitForAttentionUnreadBadgeHidden waits until the query-box unread badge leaves the tree.
func waitForAttentionUnreadBadgeHidden(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		if _, found := automationdriver.Find(snapshot, attentionUnreadBadgeID); found {
			return false, "unread badge still visible"
		}
		return true, ""
	})
	if err != nil {
		t.Fatalf("wait for Attention unread badge to hide: %v", err)
	}
	return snapshot
}

// waitForAttentionInboxOpened waits until the launcher is showing the Attention inbox query.
func waitForAttentionInboxOpened(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		if !inputFound || input.Value != "attention " {
			query := ""
			if inputFound {
				query = input.Value
			}
			return false, fmt.Sprintf("query %q", query)
		}
		if !resultsFound || results.Value != "complete" {
			return false, "attention results incomplete"
		}
		if !smoke.HasLauncherResultLabel(snapshot, attentionFixtureTitle) {
			return false, "attention fixture item missing"
		}
		return true, ""
	})
	if err != nil {
		t.Fatalf("wait for Attention inbox: %v", err)
	}
	return snapshot
}

func primaryModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierMeta
	}
	return woxui.KeyModifierControl
}

// openAttentionItem selects the persisted fixture item and returns its current semantics snapshot.
func openAttentionItem(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "attention ")
	resultID, found := smoke.FindLauncherResult(snapshot, attentionFixtureTitle)
	if !found {
		t.Fatalf("Attention item %q was not visible", attentionFixtureTitle)
	}
	return smoke.SelectLauncherResult(t, ctx, client, resultID)
}

// waitForAttentionState polls the real shared database until the expected persisted state is visible.
func waitForAttentionState(t *testing.T, ctx context.Context, predicate func(attentionFixtureState) bool) attentionFixtureState {
	t.Helper()
	db := openAttentionSmokeDatabase(t)
	defer db.Close()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var state attentionFixtureState
	var lastErr error
	for {
		lastErr = db.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(MAX(is_read), 0), COALESCE(MAX(content_fingerprint), '')
			FROM attention_items
			WHERE identity_key = ?`, attentionFixtureIdentityKey).Scan(&state.count, &state.isRead, &state.fingerprint)
		if lastErr == nil && predicate(state) {
			return state
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Attention database state: last state %+v, last error %v", state, lastErr)
		case <-ticker.C:
		}
	}
}

func deleteAttentionFixture(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := openAttentionSmokeDatabase(t)
	defer db.Close()
	if _, err := db.ExecContext(ctx, "DELETE FROM attention_items WHERE identity_key = ?", attentionFixtureIdentityKey); err != nil {
		t.Errorf("delete Attention smoke fixture: %v", err)
	}
}

func openAttentionSmokeDatabase(t *testing.T) *sql.DB {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	db, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open shared smoke database: %v", err)
	}
	return db
}
