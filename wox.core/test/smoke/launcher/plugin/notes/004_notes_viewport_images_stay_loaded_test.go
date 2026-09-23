//go:build wox_ui_smoke

package notes

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test004NotesViewportImagesStayLoaded verifies Notes pictures stay decoded while they share one viewport and unload once they leave it.
// Flow: attach three large pictures -> widen the Notes window -> show all three together -> scroll them out of view -> scroll back.
// Evidence: the three pictures report loaded bitmaps together, report deferred after leaving the viewport, and report loaded again after returning.
func Test004NotesViewportImagesStayLoaded(t *testing.T) {
	const edge = 2048
	names := []string{"one", "two", "three"}
	dir := filepath.Join(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment), "notes", "attachments")
	if strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)) == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create note attachments directory: %v", err)
	}
	for _, name := range names {
		path := filepath.Join(dir, "wox-smoke-viewport-"+name+".png")
		if err := writeSolidPNG(path, edge); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	var markdown strings.Builder
	for _, name := range names {
		fmt.Fprintf(&markdown, "![%s](notes-image:wox-smoke-viewport-%s.png?scale=20&width=%d&height=%d)\n\n", name, name, edge, edge)
	}
	// The caret opens on the trailing text, so the editor must be tall enough to park every picture above the viewport.
	markdown.WriteString(strings.Repeat("spacer\n", 40))

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openNewNoteEditor(t, ctx, client)
		restore := widenNotesWindow(t, ctx, client)
		t.Cleanup(restore)
		enterNoteMarkdownSource(t, ctx, client, markdown.String())
		// Opening scrolls to the caret below the pictures. Bring them back before measuring the round trip.
		waitForNotePictures(t, ctx, client)
		scrollNotesEditor(t, ctx, client, "0")
		waitForNoteImages(t, ctx, client, "loaded", true)
		scrollNotesEditor(t, ctx, client, "100000")
		waitForNoteImages(t, ctx, client, "deferred", false)
		scrollNotesEditor(t, ctx, client, "0")
		waitForNoteImages(t, ctx, client, "loaded", true)
		if err := client.RequestFrame(ctx); err != nil {
			t.Fatalf("request notes frame: %v", err)
		}
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read notes snapshot: %v", err)
		}
		assertNoteImages(t, snapshot, "loaded", true)
		smoke.AssertNoDiagnostics(t, snapshot)

		openMoreMenu(t, ctx, client)
		if err := client.Perform(ctx, "notes.menu.delete", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("delete viewport note: %v", err)
		}
	})
}

func enterNoteMarkdownSource(t *testing.T, ctx context.Context, client *automationdriver.Client, markdown string) {
	t.Helper()
	toggleNoteMarkdownView(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "notes.editor.markdown")
		return found
	}); err != nil {
		t.Fatalf("wait for Notes Markdown editor: %v", err)
	}
	if err := client.Perform(ctx, "notes.editor.markdown", woxui.AccessibilityActionSetValue, markdown); err != nil {
		t.Fatalf("enter Notes Markdown images: %v", err)
	}
	toggleNoteMarkdownView(t, ctx, client)
}

func toggleNoteMarkdownView(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	openMoreMenu(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "notes.menu.view")
		return found
	}); err != nil {
		t.Fatalf("wait for Notes view action: %v", err)
	}
	if err := client.Perform(ctx, "notes.menu.view", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("toggle Notes Markdown view: %v", err)
	}
}

func widenNotesWindow(t *testing.T, ctx context.Context, client *automationdriver.Client) func() {
	t.Helper()
	before, err := client.Bounds(ctx)
	if err != nil {
		t.Fatalf("read Notes bounds: %v", err)
	}
	widened := before
	widened.Width = 1200
	widened.Height = 640
	if err := client.SetBounds(ctx, widened); err != nil {
		t.Fatalf("widen Notes window: %v", err)
	}
	return func() {
		if err := client.SetBounds(context.Background(), before); err != nil {
			t.Errorf("restore Notes bounds: %v", err)
		}
	}
}

func scrollNotesEditor(t *testing.T, ctx context.Context, client *automationdriver.Client, offset string) {
	t.Helper()
	if err := client.Perform(ctx, "notes.editor.scroll", woxui.AccessibilityActionScroll, offset); err != nil {
		t.Fatalf("scroll Notes editor to %s: %v", offset, err)
	}
}

func waitForNotePictures(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		return notePictureCount(snapshot) == 3
	})
	if err != nil {
		t.Fatalf("wait for note pictures: %v; %s", err, formatNoteImageNodes(snapshot))
	}
}

func waitForNoteImages(t *testing.T, ctx context.Context, client *automationdriver.Client, value string, inside bool) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		return noteImagesMatch(snapshot, value, inside)
	})
	if err != nil {
		t.Fatalf("wait for note images %s inside=%v: %v; %s", value, inside, err, formatNoteImageNodes(snapshot))
	}
	assertNoteImages(t, snapshot, value, inside)
}

func assertNoteImages(t *testing.T, snapshot woxwidget.AutomationSnapshot, value string, inside bool) {
	t.Helper()
	if !noteImagesMatch(snapshot, value, inside) {
		t.Fatalf("note images are not %s inside=%v; %s", value, inside, formatNoteImageNodes(snapshot))
	}
}

func notePictureCount(snapshot woxwidget.AutomationSnapshot) int {
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if notePictureNode(node) && node.Bounds.Width > 0 && node.Bounds.Height > 0 {
			count++
		}
	}
	return count
}

func noteImagesMatch(snapshot woxwidget.AutomationSnapshot, value string, inside bool) bool {
	scroll, found := automationdriver.Find(snapshot, "notes.editor.scroll")
	if !found || scroll.Bounds.Width <= 0 || scroll.Bounds.Height <= 0 {
		return false
	}
	// A parked offset of 0 cannot have moved the pictures out of view.
	atTop := strings.HasPrefix(scroll.Value, "0/")
	if inside != atTop {
		return false
	}
	visible := map[string]bool{"one": false, "two": false, "three": false}
	for _, node := range snapshot.Tree.Nodes {
		if !notePictureNode(node) {
			continue
		}
		inView := boundsIntersect(node.Bounds, scroll.Bounds)
		if _, want := visible[node.Label]; !want {
			continue
		}
		if node.Value != value || inView != inside {
			return false
		}
		visible[node.Label] = true
	}
	return visible["one"] && visible["two"] && visible["three"]
}

func notePictureNode(node woxui.AccessibilityNode) bool {
	if node.Role != woxui.AccessibilityRoleImage || !strings.HasPrefix(node.AutomationID, "notes.editor.image.") {
		return false
	}
	// Toolbar actions reuse the image id as a prefix. Only the picture itself counts.
	return !strings.Contains(node.AutomationID, ".image-")
}

func boundsIntersect(a, b woxui.Rect) bool {
	return a.Width > 0 && a.Height > 0 && b.Width > 0 && b.Height > 0 &&
		a.X < b.X+b.Width && a.X+a.Width > b.X &&
		a.Y < b.Y+b.Height && a.Y+a.Height > b.Y
}

func formatNoteImageNodes(snapshot woxwidget.AutomationSnapshot) string {
	rows := make([]string, 0, 4)
	for _, node := range snapshot.Tree.Nodes {
		if node.Role == woxui.AccessibilityRoleImage && strings.HasPrefix(node.AutomationID, "notes.editor.image.") {
			rows = append(rows, fmt.Sprintf("%s label=%q value=%q bounds=%v", node.AutomationID, node.Label, node.Value, node.Bounds))
		}
	}
	scroll, found := automationdriver.Find(snapshot, "notes.editor.scroll")
	if !found {
		return "scroll missing; images=[" + strings.Join(rows, "; ") + "]"
	}
	return fmt.Sprintf("scroll=%v value=%q images=[%s]", scroll.Bounds, scroll.Value, strings.Join(rows, "; "))
}

func writeSolidPNG(path string, edge int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, image.NewRGBA(image.Rect(0, 0, edge, edge)))
}
