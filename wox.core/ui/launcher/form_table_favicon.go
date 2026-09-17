package launcher

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// formTableFaviconState is copied into each frame; request identity prevents stale writes.
type formTableFaviconState struct {
	fieldIndex int
	url        string
	error      string
	loading    bool
}

// openFormTableFavicon opens a nested dialog without replacing the row draft.
func (a *App) openFormTableFavicon(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "woxImage" {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	state.rowForm.active = false
	state.favicon = &formTableFaviconState{fieldIndex: index}
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

// closeFormTableFavicon discards the dialog; any outstanding result becomes stale.
func (a *App) closeFormTableFavicon() {
	if state := a.activeFormTableEditor(); state != nil {
		state.favicon = nil
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// normalizeFaviconURL accepts ordinary website addresses, excluding non-web schemes and credentials.
func normalizeFaviconURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || strings.ContainsAny(parsed.Host, " \t\r\n") {
		return ""
	}
	return parsed.String()
}

// fetchFormTableFavicon keeps network work off the UI thread and applies only to the original draft.
func (a *App) fetchFormTableFavicon() {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.favicon == nil || state.favicon.loading {
		return
	}
	picker, row := state.favicon, state.rowForm
	websiteURL := normalizeFaviconURL(picker.url)
	if websiteURL == "" {
		picker.error = a.translate("i18n:ui_image_editor_url_invalid")
		a.invalidateFormTableWindow()
		return
	}
	picker.loading, picker.error = true, ""
	a.invalidateFormTableWindow()
	util.Go(a.lifecycleCtx, "fetch settings favicon", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		icon, err := a.services.FetchWebsiteIcon(ctx, a.sessionID, websiteURL)
		var encoded []byte
		if err == nil {
			encoded, err = json.Marshal(icon)
		}
		if err != nil {
			util.GetLogger().Warn(ctx, "fetch settings favicon: "+err.Error())
		}
		_ = a.runOnUI("apply settings favicon", func() {
			if a.activeFormTableEditor() != state || state.rowForm != row || state.favicon != picker {
				return
			}
			picker.loading = false
			if err != nil {
				picker.error = a.translate("i18n:ui_image_editor_url_failed")
			} else {
				row.values[row.definitions[picker.fieldIndex].Value.Key] = string(encoded)
				clearFormTableRowValidationLocked(state)
				a.closeFormTableFavicon()
			}
			a.invalidateFormTableWindow()
		})
	})
}

// buildFormTableFavicon composes standard controls from a frame snapshot inside a modal focus scope.
func (a *App) buildFormTableFavicon(snapshot *formTableFaviconState, theme woxcomponent.ControlTheme, width, height float32) woxwidget.Widget {
	dialogWidth := min(float32(460), width-32)
	contentWidth := dialogWidth - 40
	title := a.translate("i18n:ui_image_editor_from_url")
	status, color := snapshot.error, theme.Error
	confirm := a.translate("i18n:ui_image_editor_url_confirm")
	if snapshot.loading {
		status, color = a.translate("i18n:ui_hotkey_ignore_apps_loading"), theme.TextSecondary
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-favicon", Label: title, Width: dialogWidth, Height: 204,
		OverlayWidth: width, OverlayHeight: height, Padding: woxwidget.Insets{Left: 20, Right: 20, Top: 20, Bottom: 20},
		InitialFocus: "form-table-favicon-url", OnEscape: a.closeFormTableFavicon, Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: woxcomponent.SettingsLabelFontSize}, Color: theme.Text},
			woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
				ID: "form-table-favicon-url", Label: "URL", Hint: "https://example.com", Width: contentWidth,
				Value: snapshot.url, Disabled: snapshot.loading, Window: a.formTableNativeWindow(), Theme: theme,
				OnKey: func(event woxui.KeyEvent) bool {
					if event.Down && !event.Composing && event.Key == woxui.KeyEnter {
						a.fetchFormTableFavicon()
						return true
					}
					return a.onFormTableKey(event)
				},
				OnChanged: func(value string) {
					if state := a.activeFormTableEditor(); state != nil && state.favicon != nil && !state.favicon.loading {
						state.favicon.url, state.favicon.error = value, ""
						a.invalidateFormTableWindow()
					}
				},
			}),
			woxwidget.Container{Height: 32, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: woxcomponent.SettingsHelpFontSize}, Color: color}},
			woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisEnd, Gap: 8, Children: []woxwidget.Widget{
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-favicon-cancel", Label: a.translate("i18n:ui_cancel"), OnTap: a.closeFormTableFavicon, Theme: theme}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-favicon-confirm", Label: confirm, Disabled: snapshot.loading, Variant: woxcomponent.ButtonPrimary, OnTap: a.fetchFormTableFavicon, Theme: theme}),
			}},
		}},
	})
}
