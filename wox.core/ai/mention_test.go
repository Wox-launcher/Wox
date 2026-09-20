package ai

import "testing"

func TestStripMentionTagsRemovesPluginKeepsSkill(t *testing.T) {
	if got := StripMentionTags("{plugin:Notes}"); got != "" {
		t.Fatalf("plugin tag = %q", got)
	}
	if got := StripMentionTags("use {plugin:Notes} please"); got != "use  please" {
		t.Fatalf("surrounding text = %q", got)
	}
	if got := StripMentionTags("{skill:Review} and {plugin:Notes}"); got != "{skill:Review} and" {
		t.Fatalf("skill tag must stay: %q", got)
	}
}
