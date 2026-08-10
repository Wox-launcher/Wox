package emojisearch

import "testing"

func TestMatchUsesEmojiAndMultilingualTerms(t *testing.T) {
	entry := Entry{Emoji: "🤖", SearchTerms: BuildTerms(
		map[string]string{"en": "robot", "zh_CN": "机器人"},
		map[string]string{"en": "Smileys & Emotion", "zh_CN": "表情与情感"},
	)}
	for _, query := range []string{"🤖", "robot", "机器人", "emotion", "情感"} {
		if !Match(entry, query) {
			t.Fatalf("query %q should match robot entry", query)
		}
	}
	if Match(entry, "rocket") {
		t.Fatal("unrelated query should not match robot entry")
	}
}

func TestFilterPreservesOrderAndLimit(t *testing.T) {
	entries := []Entry{
		{Emoji: "❤️", SearchTerms: []string{"red heart", "红心"}},
		{Emoji: "🧡", SearchTerms: []string{"orange heart", "橙心"}},
		{Emoji: "🚀", SearchTerms: []string{"rocket", "火箭"}},
	}
	results := Filter(entries, "heart", 1)
	if len(results) != 1 || results[0].Emoji != "❤️" {
		t.Fatalf("limited results = %#v, want first heart", results)
	}
}
