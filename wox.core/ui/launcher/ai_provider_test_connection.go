package launcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/openai/openai-go/v3"
	"wox/ai"
	"wox/common"
	"wox/setting"
	"wox/util"
)

type aiProviderTestState struct {
	provider setting.AIProvider
	form     *formFieldsState
	loading  bool
	message  string
}

func aiProviderFromValues(values map[string]string) setting.AIProvider {
	return setting.AIProvider{Name: common.ProviderName(values["Name"]), Host: values["Host"], ApiKey: values["ApiKey"], Executable: values["Executable"], ReasoningEffort: values["ReasoningEffort"]}
}

// snapshotAIProviderTest hides results as soon as the tested connection settings change.
func snapshotAIProviderTest(test *aiProviderTestState, form *formFieldsSnapshot) *aiProviderTestState {
	if test == nil || form == nil || test.provider != aiProviderFromValues(form.values) {
		return nil
	}
	copy := *test
	return &copy
}

// aiProviderTestDescription keeps the usage warning visible even after a test completes.
func (c *formTableController) aiProviderTestDescription(snapshot *formTableEditorSnapshot) string {
	description := c.language.translate("i18n:ui_ai_provider_test_description")
	if snapshot.providerTest != nil {
		description += "\n" + snapshot.providerTest.message
	}
	return description
}

// testAIProvider probes the unsaved row on demand and discards results from replaced editors.
func (c *formTableController) testAIProvider() {
	state := c.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.definition.Value.Key != "AIProviders" {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	config := aiProviderFromValues(state.rowForm.values)
	test := &aiProviderTestState{provider: config, form: state.rowForm, loading: true, message: c.language.translate("i18n:ui_ai_provider_test_running")}
	state.providerTest = test
	c.invalidateFormTableWindow()
	util.Go(c.lifecycleCtx, "test AI provider", func() {
		key := probeAIProvider(c.lifecycleCtx, config)
		_ = c.runOnUI("apply AI provider test", func() {
			if c.activeFormTableEditor() != state || state.rowForm != test.form || state.providerTest != test {
				return
			}
			test.loading = false
			test.message = fmt.Sprintf(c.language.translate(key), time.Now().Format("15:04:05"))
			c.invalidateFormTableWindow()
		})
	})
}

// probeAIProvider uses the provider's own protocol and limits how long a test can run.
func probeAIProvider(parent context.Context, config setting.AIProvider) string {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	provider, err := ai.NewProvider(ctx, config)
	if err == nil {
		err = provider.Ping(ctx)
	}
	if err != nil && config.Name.IsInstalledCLI() {
		return "i18n:ui_ai_provider_test_cli_failed"
	}
	return aiProviderTestResultKey(err)
}

// aiProviderTestResultKey avoids exposing credentials or raw server responses in the UI.
func aiProviderTestResultKey(err error) string {
	if err == nil {
		return "i18n:ui_ai_provider_test_success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "i18n:ui_ai_provider_test_timeout"
	}
	var apiError *openai.Error
	if errors.As(err, &apiError) && (apiError.StatusCode == 401 || apiError.StatusCode == 403) {
		return "i18n:ui_ai_provider_test_auth"
	}
	var netError *net.OpError
	if errors.As(err, &netError) {
		return "i18n:ui_ai_provider_test_unreachable"
	}
	return "i18n:ui_ai_provider_test_failed"
}
