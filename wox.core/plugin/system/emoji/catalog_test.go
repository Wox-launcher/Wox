package emoji

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

func TestSharedCatalogMatchesEnglishChineseAndCategoryTerms(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(catalog) < 5000 {
		t.Fatalf("catalog count = %d, want at least 5000", len(catalog))
	}
	var robot *EmojiData
	for index := range catalog {
		if catalog[index].Emoji == "🤖" {
			robot = &catalog[index]
			break
		}
	}
	if robot == nil {
		t.Fatal("shared catalog does not contain robot emoji")
	}
	plugin := EmojiPlugin{}
	for _, query := range []string{"🤖", "robot", "机器人", "emotion", "情感"} {
		if !plugin.matchEmoji(*robot, query) {
			t.Fatalf("query %q should match robot emoji", query)
		}
	}
}

// TestCatalogContentFingerprint protects every character, translation, category,
// and search term when changing the embedded catalog's storage format.
func TestCatalogContentFingerprint(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		Emoji       string
		Names       map[string]string
		Categories  map[string]string
		SearchTerms []string
	}
	snapshot := make([]entry, len(catalog))
	for i, item := range catalog {
		terms := append([]string(nil), item.SearchTerms...)
		sort.Strings(terms)
		snapshot[i] = entry{item.Emoji, item.Names, item.Categories, terms}
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	const want = "0b8e91ee12db49a3de2292ec7e50bd5849cf16f9a743542103a4c93d4af20cc0"
	if got != want {
		t.Fatalf("catalog fingerprint = %s, want %s", got, want)
	}
}
