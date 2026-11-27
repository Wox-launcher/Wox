package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type wpmCommandTestAPI struct {
	plugin.API
	query common.PlainQuery
}

// TestWPMCreateAIPrompt checks that every locale keeps the portable agent instructions intact.
func TestWPMCreateAIPrompt(t *testing.T) {
	for _, language := range i18n.GetSupportedLanguages() {
		locale := string(language.Code)
		t.Run(locale, func(t *testing.T) {
			data, err := os.ReadFile(wpmLocalePath(locale))
			require.NoError(t, err)
			var translations map[string]string
			require.NoError(t, json.Unmarshal(data, &translations))
			for _, key := range []string{"create_with_ai_description", "copy_ai_prompt", "ai_prompt_copied", "copy_ai_prompt_failed"} {
				require.NotEmpty(t, translations["plugin_wpm_"+key])
			}
			prompt := translations["plugin_wpm_creation_prompt"]
			require.NotContains(t, prompt, "%s")
			for _, text := range []string{"single-file SDK", "SKILL.md", "https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-creator", "https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-submit2store"} {
				require.Contains(t, prompt, text)
			}
			require.NotContains(t, prompt, "%!")
		})
	}
}

func (a *wpmCommandTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	a.query = query
}

// TestWPMCommandDiscovery covers filtering and entering commands through each alias.
func TestWPMCommandDiscovery(t *testing.T) {
	ctx := context.Background()
	api := &wpmCommandTestAPI{}
	w := &WPMPlugin{api: api}
	for _, keyword := range []string{"store", "wpm", "pm"} {
		hint, _ := plugin.MatchQueryHint(keyword+" ", []*plugin.Instance{{Metadata: w.GetMetadata()}})
		require.NotNil(t, hint)
		require.Equal(t, []string{"create", "install", "uninstall", "update"}, hint.Elements[1].Suggestions)
		require.True(t, hint.CommandSuggestions, "command completion must retain its trailing space")
		for _, tc := range []struct {
			search string
			count  int
		}{
			{"", 4},
			{"inst", 2},
			{" INST ", 2},
			{" DEV. ", 0},
			{"dev.add", 0},
			{"create", 1},
			{"update", 1},
			{"missing-command", 0},
		} {
			t.Run(keyword+"/"+tc.search, func(t *testing.T) {
				response := w.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: keyword, Search: tc.search})
				require.Len(t, response.Results, tc.count)
				for _, result := range response.Results {
					require.NotEmpty(t, result.SubTitle)
					require.Len(t, result.Actions, 1)
					require.True(t, result.Actions[0].PreventHideAfterAction)
					result.Actions[0].Action(ctx, plugin.ActionContext{})
					require.Equal(t, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: keyword + " " + result.Title + " "}, api.query)
				}
			})
		}
	}
	response := w.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "wpm", Command: "create"})
	require.Len(t, response.Results, 1)
	result := response.Results[0]
	require.Equal(t, "i18n:plugin_wpm_copy_ai_prompt", result.Title)
	require.Empty(t, result.Tails)
	require.Len(t, result.Actions, 1)
	require.Equal(t, result.Title, result.Actions[0].Name)
	require.True(t, result.Actions[0].IsDefault)
}

// TestWPMStoreSorting checks that each refinement controls the order and launcher scores.
func TestWPMStoreSorting(t *testing.T) {
	ctx := context.Background()
	w := &WPMPlugin{}
	response := w.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "wpm", Command: "install"})
	require.Len(t, response.Refinements, 2)
	require.Equal(t, wpmInstallStatusRefinementKey, response.Refinements[0].Id)
	refinement := response.Refinements[1]
	require.Equal(t, wpmStoreSortRefinementKey, refinement.Id)
	require.Equal(t, plugin.QueryRefinementTypeSingleSelect, refinement.Type)
	require.Equal(t, []string{string(plugin.StorePluginSortDownloads)}, refinement.DefaultValue)
	require.Equal(t, util.PrimaryHotkey("shift+s"), refinement.Hotkey)
	require.NotEqual(t, response.Refinements[0].Hotkey, refinement.Hotkey)
	var options []string
	for _, option := range refinement.Options {
		options = append(options, option.Value)
	}
	require.Equal(t, []string{"downloads", "name", "stars", "updated"}, options)

	zero, few, many := 0, 10, 100
	manifests := []plugin.StorePluginManifest{
		{Id: "alpha", Name: "Alpha", DateUpdated: "2025-01-01 12:00:00"},
		{Id: "beta", Name: "beta", DownloadCount: &zero, StarCount: &many, DateUpdated: "invalid"},
		{Id: "zulu", Name: "Zulu", DownloadCount: &many, StarCount: &few, DateUpdated: "2026-01-01 12:00:00"},
	}
	for _, tc := range []struct {
		order plugin.StorePluginSort
		want  []string
		tail  string
	}{
		{plugin.StorePluginSortDefault, []string{"zulu", "beta", "alpha"}, "↓ 100"},
		{plugin.StorePluginSortDownloads, []string{"zulu", "beta", "alpha"}, "↓ 100"},
		{plugin.StorePluginSortName, []string{"alpha", "beta", "zulu"}, ""},
		{plugin.StorePluginSortStars, []string{"beta", "zulu", "alpha"}, "★ 100"},
		{plugin.StorePluginSortUpdated, []string{"zulu", "alpha", "beta"}, "2026-01-01"},
	} {
		t.Run(string(tc.order), func(t *testing.T) {
			query := plugin.Query{Command: "install"}
			if tc.order != plugin.StorePluginSortDefault {
				query.Refinements = map[string]string{wpmStoreSortRefinementKey: string(tc.order)}
			}
			results := w.buildStorePluginResults(ctx, query, slices.Clone(manifests), nil, wpmInstallStatusRefinementAll)
			require.Len(t, results, len(tc.want))
			for i, id := range tc.want {
				require.Equal(t, id, results[i].IdentityKey)
				if i > 0 {
					require.Greater(t, results[i-1].Score, results[i].Score)
				}
			}
			if tc.tail == "" {
				require.Empty(t, results[0].Tails)
			} else {
				require.Equal(t, []plugin.QueryResultTail{plugin.NewQueryResultTailText(tc.tail)}, results[0].Tails)
			}
		})
	}
}

// TestWPMStoreGroups separates status from rank and shows statistics only on uninstalled rows.
func TestWPMStoreGroups(t *testing.T) {
	w := &WPMPlugin{}
	zero, many := 0, 100
	manifests := []plugin.StorePluginManifest{
		{Id: "installed", Name: "Installed", Version: "1.0.0", DownloadCount: &many},
		{Id: "new", Name: "New", DownloadCount: &zero},
		{Id: "unknown", Name: "Unknown"},
	}
	query := plugin.Query{Command: "install"}
	installed := map[string]*plugin.Instance{"installed": {Metadata: plugin.Metadata{Version: "1.0.0", TriggerKeywords: []string{"example"}}}}
	results := w.buildStorePluginResults(context.Background(), query, manifests, installed, wpmInstallStatusRefinementAll)
	require.Len(t, results, 3)
	require.Equal(t, "i18n:plugin_wpm_refinement_status_installed", results[0].Group)
	require.Equal(t, []plugin.QueryResultTail{{Type: plugin.QueryResultTailTypeImage, Image: icons.Get(icons.StatusInstalled)}}, results[0].Tails)
	require.Equal(t, "i18n:plugin_wpm_refinement_status_not_installed", results[1].Group)
	require.Greater(t, results[1].GroupScore, results[0].GroupScore)
	require.Equal(t, []plugin.QueryResultTail{plugin.NewQueryResultTailText("↓ 0")}, results[1].Tails)
	require.Equal(t, results[1].Group, results[2].Group)
	require.Empty(t, results[2].Tails)
	require.Len(t, results[0].Actions, 1)

	query.ContextData = common.ContextData{wpmRecentlyInstalledContextKey: "installed"}
	afterInstall := w.buildStorePluginResults(context.Background(), query, manifests, installed, wpmInstallStatusRefinementAll)
	require.Equal(t, results[0].Id, afterInstall[0].Id)
	require.Len(t, afterInstall[0].Actions, 2)
	require.Equal(t, "i18n:plugin_wpm_start_using", afterInstall[0].Actions[1].Name)
	require.True(t, afterInstall[0].Actions[1].IsDefault)

	query.ContextData = nil
	afterUninstall := w.buildStorePluginResults(context.Background(), query, manifests, nil, wpmInstallStatusRefinementNotInstalled)
	require.Equal(t, results[0].Id, afterUninstall[0].Id)
	require.Equal(t, results[1].Group, afterUninstall[0].Group)
	require.Equal(t, []plugin.QueryResultTail{plugin.NewQueryResultTailText("↓ 100")}, afterUninstall[0].Tails)
}

// TestWPMUpgradeablePlugins excludes missing, current, and newer installed plugins.
func TestWPMUpgradeablePlugins(t *testing.T) {
	manifests := []plugin.StorePluginManifest{
		{Id: "old", Version: "2.0.0"},
		{Id: "current", Version: "1.0.0"},
		{Id: "newer", Version: "1.0.0"},
		{Id: "missing", Version: "2.0.0"},
	}
	installed := map[string]*plugin.Instance{
		"old":     {Metadata: plugin.Metadata{Version: "1.0.0"}},
		"current": {Metadata: plugin.Metadata{Version: "1.0.0"}},
		"newer":   {Metadata: plugin.Metadata{Version: "2.0.0"}},
	}
	require.Equal(t, manifests[:1], wpmUpgradeablePlugins(manifests, installed))
	require.Empty(t, wpmUpgradeablePlugins(manifests, nil))
}

// TestWPMUpdateEmpty reports when no store plugins need an upgrade.
func TestWPMUpdateEmpty(t *testing.T) {
	w := &WPMPlugin{}
	response := w.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "wpm", Command: "update"})
	require.Len(t, response.Results, 1)
	require.Equal(t, "i18n:plugin_wpm_all_up_to_date", response.Results[0].Title)
	require.Equal(t, plugin.WoxPreview{}, response.Results[0].Preview)
	for _, language := range i18n.GetSupportedLanguages() {
		locale := string(language.Code)
		data, err := os.ReadFile(wpmLocalePath(locale))
		require.NoError(t, err)
		var translations map[string]string
		require.NoError(t, json.Unmarshal(data, &translations))
		require.NotEmpty(t, translations["plugin_wpm_all_up_to_date"], locale)
	}
}

func wpmLocalePath(locale string) string {
	if i18n.IsBundled(i18n.LangCode(locale)) {
		return filepath.Join("../../resource/lang", locale+".json")
	}
	return filepath.Join("../../../lang", locale+".json")
}
