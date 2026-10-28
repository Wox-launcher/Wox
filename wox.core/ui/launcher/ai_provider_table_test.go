package launcher

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/setting"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	"wox/util"
)

func TestAIProviderTableActionsFollowSourceRows(t *testing.T) {
	app := bindTestApp(&App{})
	c := app.aiSettings
	c.imageForTint = func(woxImage, *woxui.Color, int) *woxui.Image { return nil }
	raw := `[{"Name":"magpie","Host":"http://localhost:3425"},{"Name":"deepseek","Host":"https://example.test"}]`
	providers := []setting.AIProvider{{Name: "magpie", Host: "http://localhost:3425"}, {Name: "deepseek", Host: "https://example.test"}}
	tests := map[int]aiProviderRowTest{0: {provider: providers[0], message: "result for magpie"}}
	field := launcherview.FormTableFieldProps{Rows: []launcherview.FormTableRow{{Index: 1}, {Index: 0}}}
	c.addAIProviderTestActions(&field, raw, tests, 1)
	for _, row := range field.Rows {
		if len(row.TrailingActions) != 1 || row.TrailingActions[0].OnTap == nil || row.TrailingActions[0].OnHover == nil {
			t.Fatalf("missing action for row %d", row.Index)
		}
	}
	if strings.Contains(field.Rows[0].TrailingActions[0].Label, "result for magpie") || !strings.Contains(field.Rows[1].TrailingActions[0].Label, "result for magpie") {
		t.Fatal("result was attached to the wrong sorted row")
	}
	changed := providers[0]
	changed.Host = "http://localhost:9999"
	if strings.Contains(c.providerTestTooltip(changed, tests[0]), "result for magpie") {
		t.Fatal("changed configuration retained old result")
	}
	form := newAISettingsForm(settingsData{})
	form.values["AIProviders"] = raw
	c.SetForm(&form)
	if !c.savedAIProviderMatches(0, providers[0]) || c.savedAIProviderMatches(1, providers[0]) {
		t.Fatal("saved row identity did not match")
	}
	c.providerTests = tests
	snap := c.Snapshot()
	c.providerTests[0] = aiProviderRowTest{}
	if snap.ProviderTests[0].message != "result for magpie" {
		t.Fatal("snapshot changed with controller state")
	}
}

func TestAIProviderTooltipRefreshesVisibleLinuxTooltip(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux inline tooltip")
	}
	app := bindTestApp(&App{})
	c := app.aiSettings
	c.settings.open = true
	c.settings.tab = "ai"
	provider := setting.AIProvider{Name: "magpie"}
	anchor := woxui.Rect{X: 1, Y: 2, Width: 20, Height: 20}
	c.providerTestHover = &aiProviderTestHover{index: 0, provider: provider, anchor: anchor}
	c.providerTests = map[int]aiProviderRowTest{0: {provider: provider, message: "testing"}}
	c.settings.inlineTooltip = &settingsInlineTooltipState{Anchor: anchor, Text: "old"}
	c.refreshAIProviderTestTooltip(0, provider)
	if !strings.Contains(c.settings.inlineTooltip.Text, "testing") {
		t.Fatal("visible tooltip did not update")
	}
	c.providerTests[0] = aiProviderRowTest{provider: provider, message: "finished"}
	c.refreshAIProviderTestTooltip(0, provider)
	if !strings.Contains(c.settings.inlineTooltip.Text, "finished") {
		t.Fatal("visible tooltip did not show result")
	}
}

// TestAIProviderRowProbe ensures a running request cannot publish after its row changes.
func TestAIProviderRowProbe(t *testing.T) {
	app := bindTestApp(&App{})
	c := app.aiSettings
	c.lifecycleCtx = context.Background()
	pending := make(chan func(), 2)
	c.runOnUI = func(_ string, fn func()) error { pending <- fn; return nil }
	app.language.translations = map[string]string{
		"ui_ai_provider_test_running": "testing", "ui_ai_provider_test_success": "success %s",
	}
	form := newAISettingsForm(settingsData{})
	form.values["AIProviders"] = `[{"Name":"magpie","Host":"http://example.test/v1"}]`
	c.SetForm(&form)
	provider := setting.AIProvider{Name: "magpie", Host: "http://example.test/v1"}
	client := util.GetHTTPClient(context.Background())
	oldTransport := client.Transport
	client.Transport = providerTestTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)), Request: r}, nil
	})
	defer func() { client.Transport = oldTransport }()
	c.testSavedAIProvider(0, provider)
	if !c.providerTests[0].loading {
		t.Fatal("test did not enter loading state")
	}
	select {
	case apply := <-pending:
		apply()
	case <-time.After(time.Second * 3):
		t.Fatal("test did not complete")
	}
	if c.providerTests[0].loading || !strings.Contains(c.providerTests[0].message, "success") {
		t.Fatalf("unexpected test result: %+v", c.providerTests[0])
	}
	c.testSavedAIProvider(0, provider)
	form.values["AIProviders"] = `[{"Name":"magpie","Host":"http://changed.test/v1"}]`
	select {
	case apply := <-pending:
		apply()
	case <-time.After(time.Second * 3):
		t.Fatal("second test did not complete")
	}
	if !c.providerTests[0].loading {
		t.Fatal("stale test result was applied")
	}
}
