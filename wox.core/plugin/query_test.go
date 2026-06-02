package plugin

import (
	"testing"
	"wox/common"
	"wox/setting"

	"github.com/stretchr/testify/assert"
)

func getFakePluginInstances() []*Instance {
	return []*Instance{
		{
			Metadata: Metadata{
				TriggerKeywords: []string{"wpm", "*"},
				Commands: []MetadataCommand{
					{
						Command:     "install",
						Description: common.I18nString("Install Wox plugins"),
					},
					{
						Command:     "uninstall",
						Description: common.I18nString("Uninstall Wox plugins"),
					},
				},
			},
			Setting: &setting.PluginSetting{},
		},
	}
}

func Test_NewQuery(t *testing.T) {
	q, _ := newQueryInputWithPlugins("wpm", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "wpm")

	q, _ = newQueryInputWithPlugins("wpm install", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "install")

	q, _ = newQueryInputWithPlugins("wpm install ", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "")

	q, _ = newQueryInputWithPlugins("wpm install q q1", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "q q1")

	q, _ = newQueryInputWithPlugins("other install q q1", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "other install q q1")
}

func Test_BuildQueryCompletionHint_Command(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm i", getFakePluginInstances())

	hint := BuildQueryCompletionHint(q, pluginInstance, nil)

	assert.NotNil(t, hint)
	assert.Equal(t, "wpm i", hint.InputPrefix)
	assert.Equal(t, "wpm install ", hint.CompletionText)
	assert.Equal(t, "nstall ", hint.Suffix)
	assert.Equal(t, QueryCompletionSourceCommand, hint.Source)
}

func Test_BuildQueryCompletionHint_History(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm ins", getFakePluginInstances())
	histories := []setting.QueryHistory{
		{
			Query: common.PlainQuery{
				QueryType: QueryTypeInput,
				QueryText: "wpm install github",
			},
			Timestamp: 1,
		},
	}

	hint := BuildQueryCompletionHint(q, pluginInstance, histories)

	assert.NotNil(t, hint)
	assert.Equal(t, "wpm install github", hint.CompletionText)
	assert.Equal(t, "tall github", hint.Suffix)
	assert.Equal(t, QueryCompletionSourceHistory, hint.Source)
}

func Test_BuildQueryCompletionHint_CommandBeatsOlderHistory(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm i", getFakePluginInstances())
	histories := []setting.QueryHistory{
		{
			Query: common.PlainQuery{
				QueryType: QueryTypeInput,
				QueryText: "wpm install old",
			},
			Timestamp: 1,
		},
	}

	hint := BuildQueryCompletionHint(q, pluginInstance, histories)

	assert.NotNil(t, hint)
	assert.Equal(t, "wpm install ", hint.CompletionText)
	assert.Equal(t, QueryCompletionSourceCommand, hint.Source)
}

func Test_BuildQueryCompletionHint_NoHintForCompletedCommand(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm install", getFakePluginInstances())

	hint := BuildQueryCompletionHint(q, pluginInstance, nil)

	assert.Nil(t, hint)
}

func Test_BuildQueryCompletionHint_NoHintForNonPrefixHistory(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm z", getFakePluginInstances())
	histories := []setting.QueryHistory{
		{
			Query: common.PlainQuery{
				QueryType: QueryTypeInput,
				QueryText: "wpm install github",
			},
			Timestamp: 1,
		},
	}

	hint := BuildQueryCompletionHint(q, pluginInstance, histories)

	assert.Nil(t, hint)
}

func Test_BuildQueryCompletionHint_NoHintWhenOriginalInputPrefixDoesNotMatchCompletion(t *testing.T) {
	q, pluginInstance := newQueryInputWithPlugins("wpm i", getFakePluginInstances())

	hint := BuildQueryCompletionHintForInputPrefix(q, pluginInstance, nil, "wi")

	assert.Nil(t, hint)
}
