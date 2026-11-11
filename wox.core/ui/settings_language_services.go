package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"wox/cloudsync"
	"wox/i18n"
	"wox/privacy"
	"wox/setting"
	"wox/util"
	"wox/util/locale"
)

// Language preference transitions are ordered under the same lock as Cloud Sync writes.
var languagePreferenceRevision uint64
var languageAutomaticErrors = util.NewHashMap[i18n.LangCode, string]()

// InitializeLanguage loads the local preference before plugins start. Ready schedules network work.
func InitializeLanguage(ctx context.Context) {
	manager := i18n.GetI18nManager()
	manager.ConfigurePacks(filepath.Join(util.GetLocation().GetCacheDirectory(), "languages"))
	code := setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get()
	if err := manager.UpdateLang(ctx, code); err != nil {
		fallback := i18n.LangCodeEnUs
		if locale.IsZhCN() {
			fallback = i18n.LangCodeZhCn
		}
		if fallbackErr := manager.UpdateLang(ctx, fallback); fallbackErr != nil {
			util.GetLogger().Error(ctx, "load bundled language: "+fallbackErr.Error())
		}
	}
}

// ChangeLanguage switches to a validated local or downloaded pack and persists the preference.
func (s *CoreServices) ChangeLanguage(ctx context.Context, sessionID string, code i18n.LangCode, progress func(i18n.PackProgress)) error {
	ctx = util.WithSessionContext(ctx, sessionID)
	if !i18n.IsSupportedLangCode(string(code)) {
		return fmt.Errorf("unsupported lang code: %s", code)
	}
	var revision uint64
	if err := cloudsync.WithLocalSyncMutation(func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		languagePreferenceRevision++
		revision = languagePreferenceRevision
		return nil
	}); err != nil {
		return err
	}
	manager := i18n.GetI18nManager()
	pack, err := manager.ReadPack(ctx, code)
	if err != nil && !i18n.IsBundled(code) {
		pack, err = manager.DownloadPack(ctx, code, progress)
	}
	if err != nil {
		return err
	}
	if progress != nil {
		progress(i18n.PackProgress{Applying: true})
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	return cloudsync.WithLocalSyncMutation(func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if revision != languagePreferenceRevision {
			return errors.New("the language preference changed")
		}
		if !i18n.IsBundled(code) {
			// A refresh can finish between the initial cache read and this commit.
			latest, err := manager.ReadPack(ctx, code)
			if err != nil {
				return fmt.Errorf("load cached language pack: %w", err)
			}
			pack = latest
		}
		previous := woxSetting.LangCode.Get()
		if previous != code {
			if err := woxSetting.LangCode.Set(code); err != nil {
				return fmt.Errorf("save language preference: %w", err)
			}
			if err := privacy.RefreshPreservedSettings(woxSetting); err != nil {
				// Keep the live table on the previous language until both persisted copies are ready.
				restoreErr := woxSetting.LangCode.Set(previous)
				return fmt.Errorf("save private language preference: %w", errors.Join(err, restoreErr))
			}
		}
		manager.ActivatePack(pack)
		languageAutomaticErrors.Delete(code)
		return nil
	})
}

// RedownloadLanguage bypasses the cache and only changes the live table if it is still in use.
func (s *CoreServices) RedownloadLanguage(ctx context.Context, sessionID string, code i18n.LangCode, progress func(i18n.PackProgress)) error {
	ctx = util.WithSessionContext(ctx, sessionID)
	manager := i18n.GetI18nManager()
	_, err := manager.DownloadPack(ctx, code, progress)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(i18n.PackProgress{Applying: true})
	}
	_, err = applyRefreshedLanguage(ctx, code)
	return err
}

// applyRefreshedLanguage publishes a refreshed pack if it is still the active language.
func applyRefreshedLanguage(ctx context.Context, code i18n.LangCode) (bool, error) {
	var active bool
	err := cloudsync.WithLocalSyncMutation(func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		manager := i18n.GetI18nManager()
		active = manager.GetCurrentLangCode() == code
		if err := manager.RefreshActivePack(ctx, code); err != nil {
			return fmt.Errorf("load refreshed language pack: %w", err)
		}
		languageAutomaticErrors.Delete(code)
		return nil
	})
	return active, err
}

// refreshCachedLanguages updates installed language packs once the UI can receive language changes.
func refreshCachedLanguages(ctx context.Context) {
	ctx = util.WithCoreSessionContext(context.WithoutCancel(ctx))
	util.Go(ctx, "refresh cached languages", func() {
		i18n.GetI18nManager().RefreshCachedPacks(ctx, func(code i18n.LangCode) {
			active, err := applyRefreshedLanguage(ctx, code)
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("apply refreshed language %s: %v", code, err))
			} else if active {
				GetUIManager().GetUI(ctx).ReloadSetting(ctx)
			}
		})
	})
}

// reconcilePreferredLanguage is used after startup and remote setting writes. It does no network I/O
// under the mutation lock, which is also held while applying Cloud Sync oplogs.
func reconcilePreferredLanguage(ctx context.Context) {
	_ = cloudsync.WithLocalSyncMutation(func() error {
		manager := i18n.GetI18nManager()
		code := setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get()
		languagePreferenceRevision++
		pack, err := manager.ReadPack(ctx, code)
		if err == nil && manager.GetCurrentLangCode() != code {
			manager.ActivatePack(pack)
		}
		if err != nil && !i18n.IsBundled(code) && i18n.IsSupportedLangCode(string(code)) {
			downloadPreferredLanguage(ctx, code, languagePreferenceRevision)
		}
		return nil
	})
}

// downloadPreferredLanguage recovers a missing preference while preserving superseding choices.
func downloadPreferredLanguage(ctx context.Context, code i18n.LangCode, revision uint64) {
	ctx = util.WithCoreSessionContext(context.WithoutCancel(ctx))
	util.Go(ctx, "download preferred language", func() {
		manager := i18n.GetI18nManager()
		_, err := manager.DownloadPack(ctx, code, nil)
		notify := false
		_ = cloudsync.WithLocalSyncMutation(func() error {
			if revision != languagePreferenceRevision || setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get() != code {
				return nil
			}
			var pack i18n.Pack
			if err == nil {
				pack, err = manager.ReadPack(ctx, code)
			}
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("download preferred language %s: %v", code, err))
				languageAutomaticErrors.Store(code, err.Error())
			} else {
				languageAutomaticErrors.Delete(code)
				manager.ActivatePack(pack)
			}
			notify = true
			return nil
		})
		if notify {
			GetUIManager().GetUI(ctx).ReloadSetting(ctx)
		}
	})
}
