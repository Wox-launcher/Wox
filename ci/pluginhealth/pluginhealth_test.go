package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wox/plugin"
	"wox/util"
)

type fakeProbePlugin struct {
	queryErr error
}

func (f *fakeProbePlugin) Init(context.Context, plugin.InitParams) {}

func (f *fakeProbePlugin) Query(context.Context, plugin.Query) plugin.QueryResponse {
	return plugin.QueryResponse{}
}

func (f *fakeProbePlugin) QueryWithError(context.Context, plugin.Query) (plugin.QueryResponse, error) {
	return plugin.QueryResponse{}, f.queryErr
}

func TestLoadStoreManifestsNormalizesRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store-plugin.json")
	require.NoError(t, os.WriteFile(path, []byte(`[
		{"Id":"a","Name":"Alpha","Runtime":"nodejs","SupportedOS":["Windows"]},
		{"Id":"b","Name":"Beta","Runtime":"script","SupportedOS":["Linux"]}
	]`), 0644))

	manifests, err := loadStoreManifests(path)
	require.NoError(t, err)
	require.Len(t, manifests, 2)
	assert.Equal(t, plugin.PLUGIN_RUNTIME_NODEJS, manifests[0].Runtime)
	assert.Equal(t, plugin.PLUGIN_RUNTIME_SCRIPT, manifests[1].Runtime)
}

func TestSelectManifestsReportsMissingFilters(t *testing.T) {
	manifests := []plugin.StorePluginManifest{
		{Id: "aaa", Name: "Alpha"},
		{Id: "bbb", Name: "Beta"},
	}

	selected, missing := selectManifests(manifests, []string{"Alpha", "missing"})

	require.Len(t, selected, 1)
	assert.Equal(t, "aaa", selected[0].Id)
	require.Len(t, missing, 1)
	assert.Equal(t, healthStatusFailed, missing[0].Status)
	assert.Equal(t, healthStageCatalog, missing[0].Stage)
}

func TestSkipUnsupportedOS(t *testing.T) {
	current := util.GetCurrentPlatform()
	other := "linux"
	if current == "linux" {
		other = "windows"
	}

	_, skipped := skipUnsupportedOS(plugin.StorePluginManifest{
		Id:          "x",
		Name:        "X",
		SupportedOS: []string{other},
	})
	assert.True(t, skipped)

	_, skipped = skipUnsupportedOS(plugin.StorePluginManifest{
		Id:          "y",
		Name:        "Y",
		SupportedOS: []string{current, other},
	})
	assert.False(t, skipped)

	_, skipped = skipUnsupportedOS(plugin.StorePluginManifest{Id: "z", Name: "Z"})
	assert.False(t, skipped)
}

func TestWriteHealthReportRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	report := healthReport{Platform: "windows", Passed: 1, Results: []healthResult{{Id: "a", Status: healthStatusPassed}}}
	require.NoError(t, writeHealthReport(path, report))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var decoded healthReport
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, 1, decoded.Passed)
	assert.Equal(t, "a", decoded.Results[0].Id)
}

func TestBuildHealthProbeQueriesAddsFirstCommand(t *testing.T) {
	instance := &plugin.Instance{
		Metadata: plugin.Metadata{
			TriggerKeywords: []string{"*", "demo"},
			Commands:        []plugin.MetadataCommand{{Command: "list"}, {Command: "add"}},
		},
	}

	queries := buildHealthProbeQueries(instance)

	require.Len(t, queries, 2)
	assert.Equal(t, "demo", queries[0].TriggerKeyword)
	assert.Equal(t, "", queries[0].Command)
	assert.Equal(t, "demo", queries[0].RawQuery)
	assert.Equal(t, "list", queries[1].Command)
	assert.Equal(t, "demo list", queries[1].RawQuery)
}

func TestProbePluginQueryFailsOnHostQueryError(t *testing.T) {
	instance := &plugin.Instance{Plugin: &fakeProbePlugin{queryErr: errors.New("rpc timeout")}}

	err := probePluginQuery(context.Background(), instance, plugin.Query{Type: plugin.QueryTypeInput})

	require.EqualError(t, err, "rpc timeout")
}
