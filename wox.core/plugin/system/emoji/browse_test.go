package emoji

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"wox/database"
	"wox/plugin"
	"wox/util"
)

// TestBrowseQueryPreservesRankingInScores checks the contract consumed by the manager's score sort.
func TestBrowseQueryPreservesRankingInScores(t *testing.T) {
	// Isolate global settings and log handles so Windows can remove the temporary data after process exit.
	if os.Getenv("WOX_EMOJI_BROWSE_TEST_CHILD") == "" {
		t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
		t.Setenv(util.TestUserDataDirEnv, t.TempDir())
		t.Setenv("WOX_EMOJI_BROWSE_TEST_CHILD", "1")
		command := exec.Command(os.Args[0], "-test.run=^TestBrowseQueryPreservesRankingInScores$")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("browse query check: %v\n%s", err, output)
		}
		return
	}
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	e := newTestEmojiPlugin(nil)
	e.emojiLoadOnce.Do(func() {
		e.emojis = []EmojiData{
			{Emoji: "😀", Names: map[string]string{"en": "Z first"}, Categories: map[string]string{"en": "Smileys"}},
			{Emoji: "😃", Names: map[string]string{"en": "A second"}, Categories: map[string]string{"en": "Smileys"}},
		}
	})
	response := e.Query(context.Background(), plugin.Query{TriggerKeyword: "emoji"})
	if len(response.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(response.Results))
	}
	first, second := response.Results[0], response.Results[1]
	if first.Icon.ImageData != "😀" || second.Icon.ImageData != "😃" || first.Score <= second.Score {
		t.Fatalf("browse scores must preserve catalog order over title order: %s=%d, %s=%d", first.Title, first.Score, second.Title, second.Score)
	}
}

func TestSelectBrowseEmojisLimitsEachCategoryAndSkipsComponent(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	items := selectBrowseEmojis(catalog, nil, nil, func(entry EmojiData) string {
		return entry.Categories["en"]
	}, browsePerCategory)

	counts := make(map[string]int)
	for _, item := range items {
		if item.Category == skipBrowseCategoryEn {
			t.Fatal("browse results include Component")
		}
		if item.Entry.Categories["en"] == skipBrowseCategoryEn {
			t.Fatal("browse results include a Component emoji")
		}
		counts[item.Category]++
	}

	if len(counts) < 5 {
		t.Fatalf("browse categories = %d (%v), want several catalog groups", len(counts), counts)
	}
	for category, count := range counts {
		if count > browsePerCategory {
			t.Fatalf("%s count = %d, want <= %d", category, count, browsePerCategory)
		}
	}
}

func TestSelectBrowseEmojisRanksUsedFirst(t *testing.T) {
	emojis := []EmojiData{
		{Emoji: "😀", Categories: map[string]string{"en": "Smileys"}},
		{Emoji: "😃", Categories: map[string]string{"en": "Smileys"}},
		{Emoji: "🐶", Categories: map[string]string{"en": "Animals"}},
		{Emoji: "🐱", Categories: map[string]string{"en": "Animals"}},
	}

	items := selectBrowseEmojis(emojis, map[string]int{"😃": 3, "🐱": 1}, nil, func(entry EmojiData) string {
		return entry.Categories["en"]
	}, browsePerCategory)

	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	if items[0].Entry.Emoji != "😃" || items[1].Entry.Emoji != "😀" {
		t.Fatalf("smileys order = %s %s, want used 😃 first", items[0].Entry.Emoji, items[1].Entry.Emoji)
	}
	if items[2].Entry.Emoji != "🐱" || items[3].Entry.Emoji != "🐶" {
		t.Fatalf("animals order = %s %s, want used 🐱 first", items[2].Entry.Emoji, items[3].Entry.Emoji)
	}
}

func TestSelectBrowseEmojisExcludesFrequentlyUsed(t *testing.T) {
	emojis := []EmojiData{
		{Emoji: "😀", Categories: map[string]string{"en": "Smileys"}},
		{Emoji: "😃", Categories: map[string]string{"en": "Smileys"}},
	}

	items := selectBrowseEmojis(emojis, map[string]int{"😀": 9}, map[string]bool{"😀": true}, func(entry EmojiData) string {
		return entry.Categories["en"]
	}, browsePerCategory)

	if len(items) != 1 || items[0].Entry.Emoji != "😃" {
		t.Fatalf("items = %+v, want only 😃 after excluding frequently used 😀", items)
	}
}
