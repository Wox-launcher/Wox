package ui

import (
	"context"

	"wox/i18n"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/ui/contract"
	"wox/util/font"
)

// SystemFontFamilies returns portable system font family names.
func (s *CoreServices) SystemFontFamilies(ctx context.Context, sessionID string) ([]string, error) {
	families := font.GetSystemFontFamilies(uiServiceContext(ctx, sessionID))
	return append([]string(nil), families...), nil
}

// GlanceCatalog returns translated glance metadata from installed plugins.
func (s *CoreServices) GlanceCatalog(ctx context.Context, sessionID string) ([]contract.GlanceCatalogItem, error) {
	ctx = uiServiceContext(ctx, sessionID)
	langCode := setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get()
	if langCode != "" && langCode != i18n.GetI18nManager().GetCurrentLangCode() {
		if err := i18n.GetI18nManager().UpdateLang(ctx, langCode); err != nil {
			return nil, err
		}
	}
	instances := plugin.GetPluginManager().GetPluginInstances()
	catalog := make([]contract.GlanceCatalogItem, 0)
	for _, instance := range instances {
		for _, glance := range instance.Metadata.Glances {
			icon, _ := common.ParseWoxImage(glance.Icon)
			catalog = append(catalog, contract.GlanceCatalogItem{
				PluginID:          instance.Metadata.Id,
				GlanceID:          glance.Id,
				PluginName:        instance.GetName(ctx),
				Name:              instance.TranslateMetadataText(ctx, glance.Name),
				Description:       instance.TranslateMetadataText(ctx, glance.Description),
				Icon:              icon,
				RefreshIntervalMs: glance.RefreshIntervalMs,
			})
		}
	}
	return catalog, nil
}
