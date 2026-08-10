package emoji

import "testing"

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
