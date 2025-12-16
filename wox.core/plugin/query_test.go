package plugin

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"wox/common"
	"wox/setting"
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
