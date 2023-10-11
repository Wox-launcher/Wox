package plugin

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func getFakePluginInstances() []*Instance {
	return []*Instance{
		{
			Metadata: Metadata{
				TriggerKeywords: []string{"wpm"},
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
		},
	}
}

func Test_NewQuery(t *testing.T) {
	q := newQueryWithPlugins("wpm", QueryTypeText, getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "wpm")

	q = newQueryWithPlugins("wpm install", QueryTypeText, getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "")
	assert.Equal(t, q.Search, "install")

	q = newQueryWithPlugins("wpm install ", QueryTypeText, getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "")

	q = newQueryWithPlugins("wpm install q q1", QueryTypeText, getFakePluginInstances())
	assert.Equal(t, q.TriggerKeyword, "wpm")
	assert.Equal(t, q.Command, "install")
	assert.Equal(t, q.Search, "q q1")
}
