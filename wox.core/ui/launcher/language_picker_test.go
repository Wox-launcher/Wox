package launcher

import (
	"context"
	"errors"
	"testing"
	"time"
	"wox/i18n"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

type pickerLanguageService struct {
	change     func(context.Context, func(i18n.PackProgress)) error
	redownload func(context.Context, func(i18n.PackProgress)) error
}

func (s pickerLanguageService) ChangeLanguage(ctx context.Context, _ string, _ i18n.LangCode, progress func(i18n.PackProgress)) error {
	return s.change(ctx, progress)
}

func (s pickerLanguageService) RedownloadLanguage(ctx context.Context, _ string, _ i18n.LangCode, progress func(i18n.PackProgress)) error {
	return s.redownload(ctx, progress)
}

func TestLanguagePickerLocalSelectionSkipsProgressStatus(t *testing.T) {
	for _, code := range []i18n.LangCode{i18n.LangCodeEnUs, i18n.LangCodeZhCn, i18n.LangCodeJaJp} {
		t.Run(string(code), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ui := make(chan func())
			picker := languagePickerController{
				lifecycle: ctx,
				runOnUI: func(_ string, apply func()) error {
					select {
					case ui <- apply:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				},
				invalidate: func() {},
				reload:     func() error { return nil },
				translate:  func(key string) string { return key },
				image:      func(woxImage, *woxui.Color, int) *woxui.Image { return nil },
				services: pickerLanguageService{change: func(_ context.Context, progress func(i18n.PackProgress)) error {
					progress(i18n.PackProgress{Applying: true})
					return nil
				}},
			}
			picker.setCatalog([]i18n.Lang{{Code: code, Installed: true}})
			closed := false
			picker.start(settingsLanguagePicker, code, false, func() { closed = true })
			choice := settingChoice{value: string(code)}
			if status := picker.row(choice, woxcomponent.ControlTheme{}, 1, nil).Status; status != "" {
				t.Fatalf("pending local selection displays %q", status)
			}
			for range 2 {
				select {
				case apply := <-ui:
					apply()
				case <-ctx.Done():
					t.Fatal("missing local selection update")
				}
				if status := picker.row(choice, woxcomponent.ControlTheme{}, 1, nil).Status; status != "" {
					t.Fatalf("local selection displays %q", status)
				}
			}
			if !closed {
				t.Fatal("successful local selection did not close the picker")
			}
		})
	}
}

func TestLanguagePickerKeepsOpenOnProgressFailureAndManualRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := make(chan func())
	closed, reloaded := 0, 0
	picker := languagePickerController{
		lifecycle: ctx,
		errors:    make(map[i18n.LangCode]string),
		runOnUI: func(_ string, apply func()) error {
			select {
			case ui <- apply:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		invalidate: func() {},
		reload:     func() error { return nil },
	}
	nextUI := func() {
		t.Helper()
		select {
		case apply := <-ui:
			apply()
		case <-time.After(3 * time.Second):
			t.Fatal("missing picker update")
		}
	}
	finish := make(chan error, 1)
	operation := func(ctx context.Context, progress func(i18n.PackProgress)) error {
		progress(i18n.PackProgress{Received: 25, Total: 100})
		select {
		case err := <-finish:
			if err == nil {
				progress(i18n.PackProgress{Applying: true})
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	picker.services = pickerLanguageService{
		change: operation, redownload: operation,
	}
	reloadCalls := make(chan struct{}, 2)
	picker.reload = func() error { reloadCalls <- struct{}{}; return nil }
	closePicker := func() { closed++ }
	code := i18n.LangCodeJaJp

	picker.start(settingsLanguagePicker, code, false, closePicker)
	nextUI()
	if closed != 0 || picker.attempt.progress.Received != 25 {
		t.Fatal("progress closed the dropdown or was not delivered")
	}
	finish <- errors.New("invalid language pack")
	nextUI()
	if closed != 0 || picker.errors[code] == "" {
		t.Fatal("failed selection closed the dropdown or lost the error")
	}

	picker.start(settingsLanguagePicker, code, false, closePicker)
	nextUI()
	finish <- nil
	nextUI()
	if closed != 0 || !picker.attempt.progress.Applying {
		t.Fatal("picker lost the apply stage after a download")
	}
	nextUI()
	if closed != 1 || picker.errors[code] != "" {
		t.Fatal("successful retry did not clear the error and close the dropdown")
	}
	picker.start(onboardingLanguagePicker, code, true, closePicker)
	nextUI()
	finish <- nil
	nextUI()
	nextUI()
	reloaded = len(reloadCalls)
	if closed != 1 || reloaded != 2 || picker.updated == nil || picker.updated.code != code {
		t.Fatal("manual refresh did not keep the dropdown open with success feedback")
	}

	picker.start(onboardingLanguagePicker, code, false, closePicker)
	nextUI()
	picker.dismiss(onboardingLanguagePicker)
	nextUI()
	if closed != 1 || picker.attempt != nil || picker.errors[code] != "" {
		t.Fatal("a dismissed attempt changed the picker when it completed")
	}
}
