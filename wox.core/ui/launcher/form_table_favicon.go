package launcher

import (
	"context"
	"encoding/json"
	"errors"
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
func (c *formTableController) openFormTableFavicon(index int) {
	state := c.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "woxImage" {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	state.rowForm.active = false
	state.favicon = &formTableFaviconState{fieldIndex: index}
	c.updateFormTableTextInput(true)
	c.invalidateFormTableWindow()
}

// closeFormTableFavicon discards the dialog; any outstanding result becomes stale.
func (c *formTableController) closeFormTableFavicon() {
	if state := c.activeFormTableEditor(); state != nil {
		state.favicon = nil
	}
	c.updateFormTableTextInput(false)
	c.invalidateFormTableWindow()
}

// normalizeFaviconURL accepts ordinary website and image addresses, excluding non-web schemes and credentials.
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
	// Keep the pasted query string intact so signed image tokens are not re-encoded.
	return value
}

// fetchFormTableFavicon loads a pasted image URL or website favicon off the UI thread.
func (c *formTableController) fetchFormTableFavicon() {
	state := c.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.favicon == nil || state.favicon.loading {
		return
	}
	picker, row := state.favicon, state.rowForm
	websiteURL := normalizeFaviconURL(picker.url)
	if websiteURL == "" {
		picker.error = c.language.translate("i18n:ui_image_editor_url_invalid")
		c.invalidateFormTableWindow()
		return
	}
	picker.loading, picker.error = true, ""
	c.invalidateFormTableWindow()
	util.Go(c.lifecycleCtx, "fetch settings favicon", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		icon, err := c.services.FetchWebsiteIcon(ctx, c.sessionID, websiteURL)
		var encoded []byte
		if err == nil {
			encoded, err = json.Marshal(icon)
		}
		if err != nil {
			util.GetLogger().Warn(ctx, "fetch settings favicon: "+err.Error())
		}
		_ = c.runOnUI("apply settings favicon", func() {
			if c.activeFormTableEditor() != state || state.rowForm != row || state.favicon != picker {
				return
			}
			picker.loading = false
			if err != nil {
				picker.error = formTableURLFetchError(c.language, err)
			} else {
				row.values[row.definitions[picker.fieldIndex].Value.Key] = string(encoded)
				clearFormTableRowValidationLocked(state)
				c.closeFormTableFavicon()
			}
			c.invalidateFormTableWindow()
		})
	})
}

// formTableURLFetchError maps download failures onto a user-facing settings message.
func formTableURLFetchError(language *languageCatalog, err error) string {
	var status *util.HTTPStatusError
	if errors.As(err, &status) && (status.StatusCode == 401 || status.StatusCode == 403 || status.StatusCode == 404 || status.StatusCode == 410) {
		return language.translate("i18n:ui_image_editor_url_unavailable")
	}
	return language.translate("i18n:ui_image_editor_url_failed")
}

// buildFormTableFavicon composes standard controls from a frame snapshot inside a modal focus scope.
func (c *formTableController) buildFormTableFavicon(snapshot *formTableFaviconState, theme woxcomponent.ControlTheme, width, height float32) woxwidget.Widget {
	dialogWidth := min(float32(460), width-32)
	contentWidth := dialogWidth - 40
	title := c.language.translate("i18n:ui_image_editor_from_url")
	status, color := c.language.translate("i18n:ui_image_editor_url_hint"), theme.TextSecondary
	confirm := c.language.translate("i18n:ui_image_editor_url_confirm")
	if snapshot.error != "" {
		status, color = snapshot.error, theme.Error
	}
	if snapshot.loading {
		status, color = c.language.translate("i18n:ui_hotkey_ignore_apps_loading"), theme.TextSecondary
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-favicon", Label: title, Width: dialogWidth, Height: 204,
		OverlayWidth: width, OverlayHeight: height, Padding: woxwidget.Insets{Left: 20, Right: 20, Top: 20, Bottom: 20},
		InitialFocus: "form-table-favicon-url", OnEscape: c.closeFormTableFavicon, Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize)}, Color: theme.Text},
			woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
				ID: "form-table-favicon-url", Label: "URL", Hint: "https://example.com", Width: contentWidth,
				Value: snapshot.url, Disabled: snapshot.loading, Window: c.formTableNativeWindow(), Theme: theme,
				OnKey: func(event woxui.KeyEvent) bool {
					if event.Down && !event.Composing && event.Key == woxui.KeyEnter {
						c.fetchFormTableFavicon()
						return true
					}
					return c.onFormTableKey(event)
				},
				OnChanged: func(value string) {
					if state := c.activeFormTableEditor(); state != nil && state.favicon != nil && !state.favicon.loading {
						state.favicon.url, state.favicon.error = value, ""
						c.invalidateFormTableWindow()
					}
				},
			}),
			woxwidget.TextBlock{Value: status, Width: contentWidth, Height: 32, MaxLines: 2, LineHeight: theme.Scaled(16), Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsHelpFontSize)}, Color: color},
			woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisEnd, Gap: 8, Children: []woxwidget.Widget{
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-favicon-cancel", Label: c.language.translate("i18n:ui_cancel"), OnTap: c.closeFormTableFavicon, Theme: theme}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-favicon-confirm", Label: confirm, Disabled: snapshot.loading, Variant: woxcomponent.ButtonPrimary, OnTap: c.fetchFormTableFavicon, Theme: theme}),
			}},
		}},
	})
}
