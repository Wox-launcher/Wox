package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginPrivacyAccessesIncludesFolderOnlyDialogState(t *testing.T) {
	features := []pluginFeature{
		{
			Name: "queryEnv",
			Params: map[string]any{
				"requireActiveWindowIsOpenSaveDialog":             true,
				"requireActiveWindowIsOpenSaveDialogSelectFolder": true,
			},
		},
	}

	assert.Equal(t, []string{
		"requireActiveWindowIsOpenSaveDialog",
		"requireActiveWindowIsOpenSaveDialogSelectFolder",
	}, pluginPrivacyAccesses(features))
}
