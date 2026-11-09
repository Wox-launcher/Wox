package plugin

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/util/permission"
)

type OSPermissionOption struct {
	Permission permission.OSPermission
	// Reason is optional plain text or a plugin i18n key shown before the first system prompt.
	Reason common.I18nString `json:",omitempty"`
}

type OSPermissionResult struct {
	Status permission.OSPermissionStatus
}

func (a *APIImpl) GetOSPermissionStatus(_ context.Context, option OSPermissionOption) OSPermissionResult {
	return OSPermissionResult{Status: permission.GetOSPermissionStatus(option.Permission)}
}

func (a *APIImpl) RequestOSPermission(ctx context.Context, option OSPermissionOption) OSPermissionResult {
	var prompt *permission.OSPermissionPrompt
	if reason := strings.TrimSpace(string(option.Reason)); reason != "" && option.Permission.IsValid() {
		translations := i18n.GetI18nManager()
		title := translations.TranslateWox(ctx, "ui_os_permission_"+string(option.Permission)+"_title")
		prompt = &permission.OSPermissionPrompt{
			Title:         fmt.Sprintf(title, a.pluginInstance.GetName(ctx)),
			Reason:        a.pluginInstance.TranslateMetadataText(ctx, common.I18nString(reason)),
			ContinueLabel: translations.TranslateWox(ctx, "ui_os_permission_continue"),
			CancelLabel:   translations.TranslateWox(ctx, "ui_cancel"),
		}
	}
	return OSPermissionResult{Status: permission.RequestOSPermission(option.Permission, prompt)}
}
