package ui

import (
	"context"
	"errors"
	"fmt"

	"wox/plugin"
	dictationplugin "wox/plugin/system/dictation"
	"wox/ui/contract"
	"wox/util"
	"wox/util/ocr"
)

const dictationPluginID = "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e"

// ManagedModelStatuses returns live download state for one model family.
func (s *CoreServices) ManagedModelStatuses(ctx context.Context, sessionID string, kind contract.ManagedModelKind) ([]contract.ManagedModelStatus, error) {
	ctx = uiServiceContext(ctx, sessionID)
	switch kind {
	case contract.ManagedModelDictation:
		dictation, err := getDictationPlugin()
		if err != nil {
			return nil, err
		}
		statuses := dictation.GetModelStatuses(ctx)
		result := make([]contract.ManagedModelStatus, len(statuses))
		for index, status := range statuses {
			result[index] = contract.ManagedModelStatus{
				ID: status.ID, DisplayName: status.DisplayName, Description: status.Description, Languages: status.Languages,
				Recommended: status.Recommended, Status: status.Status, DownloadProgress: status.DownloadProgress, SizeMB: status.SizeMB, Error: status.Error,
			}
		}
		return result, nil
	case contract.ManagedModelOCR:
		status, err := ocr.GetPaddleModelStatus()
		if err != nil {
			return nil, err
		}
		return []contract.ManagedModelStatus{{
			ID: status.ID, DisplayName: status.DisplayName, Description: status.Description, Languages: status.Languages,
			Recommended: status.Recommended, Status: string(status.Status), DownloadProgress: status.DownloadProgress, SizeMB: status.SizeMB, Error: status.Error,
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported managed model kind %q", kind)
	}
}

// ManagedModelEngineStatus returns live inference engine state for one model family.
func (s *CoreServices) ManagedModelEngineStatus(ctx context.Context, sessionID string, kind contract.ManagedModelKind) (contract.ManagedModelEngineStatus, error) {
	ctx = uiServiceContext(ctx, sessionID)
	switch kind {
	case contract.ManagedModelDictation:
		dictation, err := getDictationPlugin()
		if err != nil {
			return contract.ManagedModelEngineStatus{}, err
		}
		status := dictation.GetNativeLibStatus(ctx)
		return contract.ManagedModelEngineStatus{State: status.State, Progress: status.Progress, Error: status.Error, Ready: status.Ready}, nil
	case contract.ManagedModelOCR:
		status, err := ocr.GetPaddleEngineStatus()
		if err != nil {
			return contract.ManagedModelEngineStatus{}, err
		}
		return contract.ManagedModelEngineStatus{State: status.State, Progress: status.Progress, Error: status.Error, Ready: status.Ready}, nil
	default:
		return contract.ManagedModelEngineStatus{}, fmt.Errorf("unsupported managed model kind %q", kind)
	}
}

// OperateManagedModel starts one asynchronous download or deletes a local dictation model.
func (s *CoreServices) OperateManagedModel(ctx context.Context, sessionID string, kind contract.ManagedModelKind, operation contract.ManagedModelOperation, modelID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	switch kind {
	case contract.ManagedModelDictation:
		dictation, err := getDictationPlugin()
		if err != nil {
			return err
		}
		switch operation {
		case contract.ManagedModelOperationDownload:
			if modelID == "" {
				return errors.New("modelId is required")
			}
			return dictation.StartModelDownload(ctx, modelID)
		case contract.ManagedModelOperationDelete:
			if modelID == "" {
				return errors.New("modelId is required")
			}
			return dictation.DeleteModel(ctx, modelID)
		case contract.ManagedModelOperationDownloadEngine:
			return dictation.StartNativeLibDownload(ctx)
		default:
			return fmt.Errorf("unsupported managed model operation %q", operation)
		}
	case contract.ManagedModelOCR:
		switch operation {
		case contract.ManagedModelOperationDownload:
			if modelID != ocr.ModelPaddlePPOCRv6Small {
				return errors.New("unsupported OCR model")
			}
			util.Go(ctx, "download OCR model", func() {
				if err := ocr.DownloadPaddleModel(ctx); err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to download OCR model: %s", err.Error()))
				}
			})
			return nil
		case contract.ManagedModelOperationDownloadEngine:
			util.Go(ctx, "download OCR engine", func() {
				if err := ocr.DownloadPaddleEngine(ctx); err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to download OCR engine: %s", err.Error()))
				}
			})
			return nil
		case contract.ManagedModelOperationDelete:
			return errors.New("OCR model deletion is not supported")
		default:
			return fmt.Errorf("unsupported managed model operation %q", operation)
		}
	default:
		return fmt.Errorf("unsupported managed model kind %q", kind)
	}
}

func getDictationPlugin() (*dictationplugin.DictationPlugin, error) {
	systemPlugin := plugin.GetPluginManager().GetSystemPlugin(dictationPluginID)
	if systemPlugin == nil {
		return nil, errors.New("dictation plugin not found")
	}
	dictation, ok := systemPlugin.(*dictationplugin.DictationPlugin)
	if !ok {
		return nil, errors.New("dictation plugin type assertion failed")
	}
	return dictation, nil
}
