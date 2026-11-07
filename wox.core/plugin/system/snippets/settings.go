package snippets

import (
	"context"
	"encoding/json"
	"errors"

	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

// settingsRepository also works before a disabled system plugin has initialized.
func (p *Plugin) settingsRepository() *Repository {
	p.expansionMu.Lock()
	repository := p.repository
	p.expansionMu.Unlock()
	if repository != nil {
		return repository
	}
	if db := database.GetDB(); db != nil {
		return NewRepository(setting.NewPluginSettingStore(db, pluginID))
	}
	return nil
}

// GetSettingOverride reads the synchronized table even when the plugin is disabled.
func (p *Plugin) GetSettingOverride(ctx context.Context, key string) (string, bool) {
	if key != snippetsSetting {
		return "", false
	}
	repository := p.settingsRepository()
	if repository == nil {
		return "", true
	}
	records, err := repository.List()
	if err != nil {
		util.GetLogger().Error(ctx, "could not load snippet settings")
		return "", true
	}
	raw, err := json.Marshal(records)
	if err != nil {
		return "", true
	}
	return string(raw), true
}

// HandleSettingUpdate saves the table as one synchronized setting and keeps the switch local.
func (p *Plugin) HandleSettingUpdate(ctx context.Context, key, value string) (bool, error) {
	if key == snippetsSetting {
		repository := p.settingsRepository()
		if repository == nil {
			return true, errors.New("snippet settings are unavailable")
		}
		if err := repository.saveTable(value); err != nil {
			if errors.Is(err, ErrKeywordConflict) {
				return true, errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_snippets_keyword_conflict"))
			}
			return true, errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_snippets_error"))
		}
		p.refreshExpansion(ctx)
		return true, nil
	}
	if key != autoExpandSetting {
		return false, nil
	}
	if value != "true" && value != "false" {
		return true, errors.New("invalid automatic expansion setting")
	}
	p.expansionMu.Lock()
	api := p.api
	p.expansionMu.Unlock()
	if api == nil {
		repository := p.settingsRepository()
		if repository == nil {
			return true, errors.New("snippet settings are unavailable")
		}
		return true, repository.store.SetWithSync(key, value, false)
	}
	result := api.SetSetting(ctx, plugin.SetSettingOption{Key: key, Value: value, IsLocal: true})
	if !result.Success {
		return true, errors.New(result.ErrMsg)
	}
	p.reconcileExpansion(ctx)
	return true, nil
}
