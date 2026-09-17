package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginRuntimeLabelOmitsNativeGoHost(t *testing.T) {
	if got := pluginRuntimeLabel("Go"); got != "" {
		t.Fatalf("Go runtime label = %q, want empty so native plugins hide the chip", got)
	}
	if got := pluginRuntimeLabel("python"); got != "Python" {
		t.Fatalf("python runtime label = %q, want Python", got)
	}
}

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
