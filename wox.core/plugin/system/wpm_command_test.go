package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"wox/common"
	"wox/plugin"

	"github.com/stretchr/testify/require"
)

type wpmCommandTestAPI struct {
	plugin.API
	query common.PlainQuery
}

// TestWPMCreateAIPrompt checks that every locale keeps the portable agent instructions intact.
func TestWPMCreateAIPrompt(t *testing.T) {
	for _, locale := range []string{"en_US", "zh_CN", "ja_JP", "ko_KR", "pt_BR", "ru_RU"} {
		t.Run(locale, func(t *testing.T) {
			data, err := os.ReadFile("../../resource/lang/" + locale + ".json")
			require.NoError(t, err)
			var translations map[string]string
			require.NoError(t, json.Unmarshal(data, &translations))
			for _, key := range []string{"create_with_ai", "create_with_ai_description", "copy_ai_prompt", "ai_prompt_copied", "copy_ai_prompt_failed"} {
				require.NotEmpty(t, translations["plugin_wpm_"+key])
			}
			format := translations["plugin_wpm_ai_prompt"]
			require.Equal(t, 1, strings.Count(format, "%s"))
			prompt := fmt.Sprintf(format, "My Plugin")
			for _, text := range []string{"My Plugin", "single-file SDK", "SKILL.md", "https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-creator", "https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-submit2store"} {
				require.Contains(t, prompt, text)
			}
			require.NotContains(t, prompt, "%!")
		})
	}
}

// TestPluginTemplates verifies both runtimes and legacy/current manifest placeholders.
func TestPluginTemplates(t *testing.T) {
	for _, runtime := range []plugin.Runtime{plugin.PLUGIN_RUNTIME_PYTHON, plugin.PLUGIN_RUNTIME_NODEJS} {
		t.Run(string(runtime), func(t *testing.T) {
			found := false
			for _, template := range pluginTemplates {
				if template.Runtime == runtime {
					found = true
					require.Equal(t, "https://codeload.github.com/Wox-launcher/"+template.Name+"/zip/refs/heads/main", template.Url)
				}
			}
			require.True(t, found)
			for _, manifest := range []string{
				`{"Id":"{{.Id}}","Name":"{{.Name}}","Runtime":"` + strings.ToLower(string(runtime)) + `","TriggerKeywords":["{{.TriggerKeyword}}"],"Description":"{{.Description}}","Author":"{{.Author}}","Website":"{{.Website}}"}`,
				`{"Id":"[Id]","Name":"[Name]","Runtime":"[Runtime]","TriggerKeywords":["[Trigger Keyword]"],"Description":"[Description]","Author":"[Author]","Website":"[Website]"}`,
			} {
				rendered := renderPluginTemplateManifest(manifest, `Test "Plugin"`, runtime)
				var metadata plugin.Metadata
				require.NoError(t, json.Unmarshal([]byte(rendered), &metadata))
				require.Equal(t, `Test "Plugin"`, string(metadata.Name))
				require.Equal(t, strings.ToLower(string(runtime)), string(metadata.Runtime))
				require.Equal(t, []string{"np"}, metadata.TriggerKeywords)
				require.Equal(t, `Test "Plugin"`, string(metadata.Description))
				require.Equal(t, "Wox User", metadata.Author)
				require.Empty(t, metadata.Website)
				require.NotContains(t, rendered, "{{.")
				require.NotEmpty(t, metadata.Id)
			}
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
		for _, tc := range []struct {
			search string
			count  int
		}{
			{"", 3},
			{"inst", 2},
			{" INST ", 2},
			{" DEV. ", 0},
			{"dev.add", 0},
			{"create", 1},
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
	response := w.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "store", Command: "create"})
	require.Len(t, response.Results, 1)
	require.Equal(t, "i18n:plugin_wpm_enter_plugin_name", response.Results[0].Title)
}
