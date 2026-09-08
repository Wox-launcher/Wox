package ai

import (
	"context"
	"testing"
	"wox/common"
	"wox/setting"
)

func TestInstalledCLIProvidersUseBrandIcons(t *testing.T) {
	openai := NewOpenAIClient(context.Background(), setting.AIProvider{Name: "openai"}).GetIcon()
	codex := providerFactories["codex-cli"](context.Background(), setting.AIProvider{Name: "codex-cli"}).GetIcon()
	if openai.ImageData == "" || openai.ImageData != codex.ImageData {
		t.Fatal("codex-cli should reuse the OpenAI brand icon")
	}

	terminal := common.UIIcon("control.terminal")
	for _, name := range []common.ProviderName{"claude-cli", "codex-cli", "opencode-cli", "grok-cli"} {
		icon := providerFactories[name](context.Background(), setting.AIProvider{Name: name}).GetIcon()
		if icon.ImageData == "" || icon.ImageData == terminal.ImageData {
			t.Fatalf("%s still uses the generic terminal icon", name)
		}
	}
}
