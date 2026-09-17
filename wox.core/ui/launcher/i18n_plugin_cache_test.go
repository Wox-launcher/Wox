package launcher

import (
	"context"
	"testing"
	"wox/i18n"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type languageCacheServices struct {
	contract.Services
	language i18n.LangCode
}

func (s *languageCacheServices) GeneralSettings(context.Context, string) (contract.GeneralSettings, error) {
	return contract.GeneralSettings{LangCode: s.language}, nil
}
func (s *languageCacheServices) LanguageJSON(context.Context, string, i18n.LangCode) (string, error) {
	return `{}`, nil
}

func TestLanguageChangeInvalidatesTranslatedPluginCaches(t *testing.T) {
	service := &languageCacheServices{language: i18n.LangCodeEnUs}
	a := newApp(false, service, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.uiCall = func(callback func()) error { callback(); return nil }
	if err := a.reloadTranslations(); err != nil {
		t.Fatal(err)
	}
	a.pluginSettings.cachePlugins(false, nil)
	a.pluginSettings.cachePlugins(true, nil)
	a.pluginSettings.SetPluginsLoaded(true)
	a.settingsSearch.SetLoaded(true)
	if err := a.reloadTranslations(); err != nil {
		t.Fatal(err)
	}
	if !a.pluginSettings.PluginsLoaded() {
		t.Fatal("same-language reload must retain catalogs")
	}
	service.language = i18n.LangCodeZhCn
	if err := a.reloadTranslations(); err != nil {
		t.Fatal(err)
	}
	for _, store := range []bool{false, true} {
		if _, loaded := a.pluginSettings.CachedPlugins(store); loaded {
			t.Fatal("old-language catalog remains cached")
		}
	}
	if a.pluginSettings.PluginsLoaded() || a.settingsSearch.Loaded() {
		t.Fatal("old-language active catalog or search index remains loaded")
	}
}
