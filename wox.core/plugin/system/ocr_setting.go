package system

import (
	"context"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/ocr"
)

// BuildOCRModelSetting creates the shared OCR model picker definition.
func BuildOCRModelSetting(ctx context.Context, key string, label string, tooltip string) definition.PluginSettingDefinitionItem {
	paddleStatus, err := ocr.GetPaddleModelStatus()
	if err != nil {
		paddleStatus = ocr.PaddleModelStatus{
			ID:          ocr.ModelPaddlePPOCRv6Small,
			DisplayName: "i18n:plugin_ocr_model_paddle",
			Status:      ocr.ModelDownloadStateFailed,
			Error:       err.Error(),
			SizeMB:      30,
		}
	}

	systemDescription := "i18n:plugin_ocr_model_system_description"
	if !ocr.IsSystemModelAvailable() {
		systemDescription = "i18n:plugin_ocr_model_system_unavailable_description"
	}
	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeOCRModel,
		Value: &definition.PluginSettingValueOCRModel{
			Key:          key,
			Label:        label,
			Tooltip:      tooltip,
			DefaultValue: ocr.ModelSystem,
			Options: []definition.OCRModelOption{
				{
					ID:          ocr.ModelSystem,
					DisplayName: "i18n:plugin_ocr_model_system",
					Description: systemDescription,
					Available:   ocr.IsSystemModelAvailable(),
					Status:      string(ocr.ModelDownloadStateDownloaded),
				},
				{
					ID:               paddleStatus.ID,
					DisplayName:      "i18n:plugin_ocr_model_paddle",
					Description:      "i18n:plugin_ocr_model_paddle_description",
					Languages:        "i18n:plugin_ocr_model_paddle_languages",
					Recommended:      paddleStatus.Recommended,
					Available:        true,
					Status:           string(paddleStatus.Status),
					DownloadProgress: paddleStatus.DownloadProgress,
					SizeMB:           paddleStatus.SizeMB,
					Error:            paddleStatus.Error,
				},
			},
		},
	}
}

// NormalizeOCRModelID keeps persisted OCR model settings compatible with the
// currently supported engines.
func NormalizeOCRModelID(raw string) string {
	modelID := strings.TrimSpace(strings.ToLower(raw))
	switch modelID {
	case "", ocr.ModelSystem:
		return ocr.ModelSystem
	case ocr.ModelPaddlePPOCRv6Small:
		return modelID
	default:
		return ocr.ModelSystem
	}
}

// NewCopyOCRTextAction copies non-empty OCR output as plain text.
func NewCopyOCRTextAction(api plugin.API, text string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name: "i18n:plugin_ocr_copy_text",
		Icon: common.CopyIcon,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: text})
		},
	}
}
