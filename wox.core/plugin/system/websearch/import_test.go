package websearch

import (
	"context"
	"os"
	"strings"
	"testing"

	"wox/common"
	"wox/plugin"
)

func TestMain(m *testing.M) {
	fetchEmbeddedWebsiteIcon = func(context.Context, string) (common.WoxImage, error) {
		return webSearchIcon, nil
	}
	os.Exit(m.Run())
}

func TestImportSearchesMergesAndRegisters(t *testing.T) {
	api := &webSearchParametersTestAPI{settings: map[string]string{
		webSearchesSettingKey: `[{"Title":"Google","Keyword":"g","Urls":["https://example.com/?q=1"],"Browser":"system"}]`,
	}}
	plugin := &WebSearchPlugin{api: api}
	results := plugin.ImportSearches(context.Background(), []ImportedSearch{
		{Title: "Google", Keyword: "g", URL: "https://example.com/?q=1"},
		{Title: "Other", Keyword: "g", URL: "https://example.com/?q=2"},
		{Title: "Smoke", Keyword: "woxsmokesearch", URL: "https://example.com/search"},
		{Title: "Taken", Keyword: "occupied", URL: "https://example.com/taken"},
	})
	if results[0].Status != importStatusExists || results[1].Status != importStatusConflict || results[2].Status != importStatusImported || results[3].ErrorCode != "keyword_taken" {
		t.Fatalf("results = %+v", results)
	}
	stored := api.settings[webSearchesSettingKey]
	if !strings.Contains(stored, "woxsmokesearch") || strings.Contains(stored, "https://example.com/?q=2") {
		t.Fatalf("stored = %s", stored)
	}
	if plugin.webSearches == nil || len(api.keywords) == 0 {
		t.Fatal("keyword was not registered")
	}
	found := false
	for _, keyword := range api.keywords {
		if keyword == "woxsmokesearch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("keywords = %#v", api.keywords)
	}
	for _, search := range plugin.webSearches {
		if search.Keyword == "g" && !search.triggerRegistered {
			t.Fatal("existing search lost its trigger registration")
		}
	}
}

func TestImportSearchStoresFetchedSiteIcon(t *testing.T) {
	previous := fetchEmbeddedWebsiteIcon
	fetchEmbeddedWebsiteIcon = func(context.Context, string) (common.WoxImage, error) {
		return common.WoxImage{ImageType: common.WoxImageTypeBase64, ImageData: "data:image/png;base64,imported"}, nil
	}
	t.Cleanup(func() { fetchEmbeddedWebsiteIcon = previous })

	api := &webSearchParametersTestAPI{settings: map[string]string{}}
	searchPlugin := &WebSearchPlugin{api: api}
	results := searchPlugin.ImportSearches(context.Background(), []ImportedSearch{
		{Title: "wox", Keyword: "wox", URL: "https://github.com/Wox-launch?{wox:parameter?name=test1}"},
	})
	if results[0].Status != importStatusImported {
		t.Fatalf("result = %+v", results[0])
	}
	if !strings.Contains(api.settings[webSearchesSettingKey], "data:image/png;base64,imported") {
		t.Fatalf("stored = %s", api.settings[webSearchesSettingKey])
	}
}

func TestIndexIconsReplacesGenericImportedIcon(t *testing.T) {
	previous := fetchEmbeddedWebsiteIcon
	fetchEmbeddedWebsiteIcon = func(context.Context, string) (common.WoxImage, error) {
		return common.WoxImage{ImageType: common.WoxImageTypeBase64, ImageData: "data:image/png;base64,github"}, nil
	}
	t.Cleanup(func() { fetchEmbeddedWebsiteIcon = previous })

	api := &webSearchParametersTestAPI{settings: map[string]string{}}
	searchPlugin := &WebSearchPlugin{api: api, webSearches: []webSearch{{
		Title: "wox", Keyword: "wox", Urls: []string{"https://github.com/Wox-launch"}, Icon: webSearchIcon,
	}}}
	searchPlugin.indexIcons(context.Background())
	if searchPlugin.webSearches[0].Icon.ImageData != "data:image/png;base64,github" {
		t.Fatalf("icon = %+v", searchPlugin.webSearches[0].Icon)
	}
	if !strings.Contains(api.settings[webSearchesSettingKey], "data:image/png;base64,github") {
		t.Fatalf("stored = %s", api.settings[webSearchesSettingKey])
	}
}

func TestQueryRestoresResultAfterReloadClearsRegistration(t *testing.T) {
	api := &webSearchParametersTestAPI{settings: map[string]string{
		webSearchesSettingKey: `[{"Title":"Search Google for {wox:parameter?name=query}","Keyword":"g","Urls":["https://www.google.com/search?q={wox:parameter?name=query}"],"IsFallback":true}]`,
	}}
	searchPlugin := &WebSearchPlugin{api: api}
	searchPlugin.reloadWebSearches(context.Background())
	for i := range searchPlugin.webSearches {
		searchPlugin.webSearches[i].triggerRegistered = false
	}
	response := searchPlugin.Query(context.Background(), plugin.Query{
		Type: plugin.QueryTypeInput, TriggerKeyword: "g", RawQuery: "g alkdsf",
	})
	if len(response.Results) != 1 || response.Results[0].Title != "Search Google for alkdsf" {
		t.Fatalf("results = %+v", response.Results)
	}
}
