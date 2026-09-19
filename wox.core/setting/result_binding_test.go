package setting

import (
	"testing"
	"wox/common"
)

func TestNormalizeResultAlias(t *testing.T) {
	if got := NormalizeResultAlias("  Rate  "); got != "rate" {
		t.Fatalf("NormalizeResultAlias = %q, want rate", got)
	}
}

func TestIsSingleWordResultAlias(t *testing.T) {
	if IsSingleWordResultAlias("") || IsSingleWordResultAlias("  ") || IsSingleWordResultAlias("hello world") || IsSingleWordResultAlias("hello\u00a0world") {
		t.Fatal("empty and multi-word aliases must be rejected")
	}
	if !IsSingleWordResultAlias("汇率") || !IsSingleWordResultAlias("rate") {
		t.Fatal("single-word aliases must be accepted")
	}
}

func TestCloneAndFindResultBindings(t *testing.T) {
	original := []ResultBinding{{
		Hash: "h1", PluginID: "app", Title: "Chrome", Alias: "ch",
		ContextData: common.ContextData{"path": "C:\\Chrome"},
	}}
	cloned := CloneResultBindings(original)
	cloned[0].Alias = "changed"
	cloned[0].ContextData["path"] = "other"
	if original[0].Alias != "ch" || original[0].ContextData["path"] != "C:\\Chrome" {
		t.Fatal("CloneResultBindings must copy alias and context data")
	}
	found, ok := FindResultBinding(original, "h1")
	if !ok || found.Title != "Chrome" {
		t.Fatalf("FindResultBinding = %+v, ok=%v", found, ok)
	}
	if _, ok := FindResultBinding(original, "missing"); ok {
		t.Fatal("missing hash must not match")
	}
}
