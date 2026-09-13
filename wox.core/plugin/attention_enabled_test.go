package plugin

import (
	"testing"
	"wox/setting"
)

func TestAttentionInstanceDisabled(t *testing.T) {
	if attentionInstanceDisabled(nil) {
		t.Fatal("missing instance is not a known disabled plugin")
	}
	if attentionInstanceDisabled(&Instance{}) {
		t.Fatal("instance without settings is not a known disabled plugin")
	}
	if attentionInstanceDisabled(&Instance{Setting: &setting.PluginSetting{}}) {
		t.Fatal("instance without a Disabled setting is not a known disabled plugin")
	}
	if IsAttentionPluginDisabled() {
		t.Fatal("missing Attention plugin instance should not report disabled")
	}
}
