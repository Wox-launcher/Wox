package plugin

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
						Description: "Install Wox plugins",
					},
					{
						Command:     "uninstall",
						Description: "Uninstall Wox plugins",
					},
				},
			},
			Setting: &setting.PluginSetting{},
		},
	}
}

func Test_NewQuery(t *testing.T) {
	q := newQueryInputWithPlugins("wpm", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "wpm")

	q = newQueryInputWithPlugins("wpm install", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "install")

	q = newQueryInputWithPlugins("wpm install ", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "")

	q = newQueryInputWithPlugins("wpm install q q1", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "q q1")

	q = newQueryInputWithPlugins("other install q q1", getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "other install q q1")
}
