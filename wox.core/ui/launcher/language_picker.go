package launcher

import (
	"context"
	"fmt"
	"time"
	"wox/common/icons"
	"wox/i18n"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	"wox/util"
)

type languagePickerOwner uint8

const (
	settingsLanguagePicker languagePickerOwner = iota
	onboardingLanguagePicker
)

type languagePickerAttempt struct {
	owner    languagePickerOwner
	code     i18n.LangCode
	cancel   context.CancelFunc
	progress i18n.PackProgress
}

// languagePickerController shares download feedback and cancellation between Settings and onboarding.
// All of its state is owned by the UI thread.
type languagePickerController struct {
	services   contract.LanguagePackServices
	sessionID  string
	lifecycle  context.Context
	runOnUI    func(string, func()) error
	invalidate func()
	reload     func() error
	translate  func(string) string
	image      func(woxImage, *woxui.Color, int) *woxui.Image

	attempt *languagePickerAttempt
	catalog map[i18n.LangCode]i18n.Lang
	errors  map[i18n.LangCode]string
	updated *languagePickerAttempt
}

func (a *App) bindLanguagePicker() {
	services, _ := a.services.(contract.LanguagePackServices)
	a.generalSettings.languagePicker = &languagePickerController{
		services: services, sessionID: a.sessionID, lifecycle: a.lifecycleCtx, runOnUI: a.runOnUI,
		invalidate: func() { a.settingsWindow.invalidateSettingsWindow(); a.onboarding.invalidateOnboardingWindow() },
		reload:     a.reloadLanguagePicker, translate: a.language.translate, image: a.imageForTint,
		errors: make(map[i18n.LangCode]string),
	}
}

// reloadLanguagePicker updates only language-owned UI state after core commits a selection.
func (a *App) reloadLanguagePicker() error {
	if err := a.reloadTranslations(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(util.NewTraceContext(), 5*time.Second)
	defer cancel()
	catalog, err := a.services.AvailableLanguages(ctx, a.sessionID)
	if err != nil {
		return err
	}
	if err := a.runOnUI("apply language picker result", func() {
		a.generalSettings.languagePicker.setCatalog(catalog)
		a.generalSettings.Update(func(data *settingsData) { data.LangCode = a.language.activeCode() })
	}); err != nil {
		return err
	}
	a.publishSettingsChanged("LangCode")
	util.Go(a.lifecycleCtx, "reload glance catalog after language change", a.reloadGlanceCatalogFromCore)
	return nil
}

func (c *languagePickerController) setCatalog(catalog []i18n.Lang) {
	if c == nil {
		return
	}
	c.catalog = make(map[i18n.LangCode]i18n.Lang, len(catalog))
	for _, language := range catalog {
		c.catalog[language.Code] = language
	}
}

func (c *languagePickerController) dismiss(owner languagePickerOwner) {
	if c != nil && c.attempt != nil && c.attempt.owner == owner {
		c.attempt.cancel()
		c.attempt = nil
	}
}

// start keeps the picker alive until the language operation has committed and the UI has reloaded.
func (c *languagePickerController) start(owner languagePickerOwner, code i18n.LangCode, refresh bool, closePicker func()) {
	if c.services == nil || (c.attempt != nil && c.attempt.code == code) {
		return
	}
	if c.attempt != nil {
		c.attempt.cancel()
	}
	delete(c.errors, code)
	c.updated = nil
	ctx, cancel := context.WithCancel(c.lifecycle)
	attempt := &languagePickerAttempt{owner: owner, code: code, cancel: cancel}
	c.attempt = attempt
	c.invalidate()
	util.Go(ctx, "apply language selection", func() {
		defer cancel()
		progress := func(progress i18n.PackProgress) {
			_ = c.runOnUI("show language pack progress", func() {
				// Show the apply phase only as a continuation of a download.
				if c.attempt != attempt || (progress.Applying && attempt.progress == (i18n.PackProgress{})) {
					return
				}
				attempt.progress = progress
				c.invalidate()
			})
		}
		apply := c.services.ChangeLanguage
		if refresh {
			apply = c.services.RedownloadLanguage
		}
		err := apply(ctx, c.sessionID, code, progress)
		if err == nil {
			err = c.reload()
		}
		_ = c.runOnUI("finish language selection", func() {
			if c.attempt != attempt {
				return
			}
			c.attempt = nil
			if err != nil {
				c.errors[code] = err.Error()
			} else if refresh {
				c.updated = attempt
				time.AfterFunc(2*time.Second, func() {
					if c.lifecycle.Err() == nil {
						_ = c.runOnUI("clear language update feedback", func() {
							if c.updated == attempt {
								c.updated = nil
								c.invalidate()
							}
						})
					}
				})
			} else {
				closePicker()
			}
			c.invalidate()
		})
	})
}

func (c *languagePickerController) row(choice settingChoice, theme woxcomponent.ControlTheme, imageScale float32, refresh func()) launcherview.SettingsChoice {
	row := launcherview.SettingsChoice{Value: choice.value, Label: choice.label}
	if c == nil {
		return row
	}
	code := i18n.LangCode(choice.value)
	language := c.catalog[code]
	if !i18n.IsBundled(code) && language.Installed {
		row.ActionIcon = c.image(fromCoreImage(icons.Get(icons.ControlRefresh)), &theme.TextSecondary, physicalImageSize(16, imageScale))
		row.ActionLabel = c.translate("i18n:ui_language_pack_redownload")
		row.OnAction = refresh
	}
	if c.attempt != nil && c.attempt.code == code {
		progress := c.attempt.progress
		row.Disabled = true
		switch {
		case progress.Applying:
			row.Status = c.translate("i18n:ui_language_pack_applying")
		case progress.Total > 0:
			row.StatusDescription = c.translate("i18n:ui_language_pack_downloading")
			row.Status = fmt.Sprintf(c.translate("i18n:ui_language_pack_percent"), min(int64(100), progress.Received*100/progress.Total))
		case progress.Received > 0:
			row.StatusDescription = c.translate("i18n:ui_language_pack_downloading")
			row.Status = fmt.Sprintf(c.translate("i18n:ui_language_pack_bytes"), (progress.Received+1023)/1024)
		case progress.Total < 0:
			row.Status = c.translate("i18n:ui_language_pack_downloading")
		}
		return row
	}
	if !i18n.IsBundled(code) && !language.Installed {
		row.StatusIcon = c.image(fromCoreImage(icons.Get(icons.ControlDownload)), &theme.TextSecondary, physicalImageSize(16, imageScale))
		row.StatusDescription = c.translate("i18n:ui_language_pack_download")
	}
	row.Error = firstNonEmpty(c.errors[code], language.DownloadError)
	if row.Error != "" {
		row.ErrorSummary = c.translate("i18n:ui_language_pack_failed")
		row.ErrorIcon = c.image(fromCoreImage(icons.Get(icons.ControlWarning)), &theme.Error, physicalImageSize(16, imageScale))
	} else if c.updated != nil && c.updated.code == code {
		row.Status = c.translate("i18n:ui_language_pack_updated")
	}
	return row
}
