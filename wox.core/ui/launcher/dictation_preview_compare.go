package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/speech"
)

type dictationModelCompareState struct {
	revision   uint64
	sessionKey string
	generation uint64
	cancel     context.CancelFunc
	busy       bool
	rawPath    string
	models     []dictationModelCompareRow
}

type dictationModelCompareRow struct {
	ID      string
	Name    string
	Type    string
	Path    string
	Status  previewview.DictationModelCompareStatus
	Error   string
	Text    string
	Elapsed time.Duration
}

// ensureDictationModelCompare keeps one comparison session per diagnostic recording.
func (a *dictationPreviewController) ensureDictationModelCompare(rawPath string) *dictationModelCompareState {
	if state := a.dictationCompare; state != nil && state.sessionKey == rawPath {
		return state
	}
	if state := a.dictationCompare; state != nil && state.cancel != nil {
		state.cancel()
	}
	state := &dictationModelCompareState{revision: 1, sessionKey: rawPath, rawPath: rawPath}
	models, err := speech.ListOfflineLocalModels(util.GetLocation().GetDictationModelsDirectory())
	if err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("dictation compare: list models: %s", err.Error()))
	}
	state.models = make([]dictationModelCompareRow, 0, len(models))
	for _, model := range models {
		state.models = append(state.models, dictationModelCompareRow{
			ID: model.ID, Name: speech.ModelDisplayName(model), Type: model.ModelType, Path: model.Path,
		})
	}
	a.dictationCompare = state
	return state
}

func (a *dictationPreviewController) dictationModelCompareProps(state *dictationModelCompareState, scrollKey, emptyResult string, style woxui.TextStyle, width, lineHeight float32) []previewview.DictationModelCompareResult {
	if state == nil {
		return nil
	}
	results := make([]previewview.DictationModelCompareResult, 0, len(state.models))
	for _, model := range state.models {
		text := model.Text
		if model.Status == previewview.DictationModelCompareDone && strings.TrimSpace(text) == "" {
			text = emptyResult
		}
		results = append(results, previewview.DictationModelCompareResult{
			ID: model.ID, Name: model.Name, Status: model.Status, Error: model.Error,
			Text: text, Duration: formatDictationCompareDuration(model.Elapsed),
			Layout: a.textLayout(a.window(), scrollKey+"|compare|"+model.ID, text, style, width, lineHeight),
		})
	}
	return results
}

func formatDictationCompareDuration(value time.Duration) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1fs", value.Seconds())
}

// startDictationModelCompare decodes the raw recording sequentially so large
// models are not loaded at the same time.
func (a *dictationPreviewController) startDictationModelCompare(modelIDs []string) {
	state := a.dictationCompare
	if state == nil || state.busy || len(modelIDs) == 0 {
		return
	}
	wanted := make(map[string]bool, len(modelIDs))
	for _, id := range modelIDs {
		wanted[id] = true
	}
	selected := make([]dictationModelCompareRow, 0, len(modelIDs))
	for i := range state.models {
		if !wanted[state.models[i].ID] {
			continue
		}
		state.models[i].Status = previewview.DictationModelCompareRunning
		state.models[i].Error = ""
		selected = append(selected, state.models[i])
	}
	if len(selected) == 0 {
		return
	}
	state.busy = true
	state.generation++
	state.revision++
	generation := state.generation
	rawPath := state.rawPath
	ctx, cancel := context.WithCancel(a.lifecycleCtx)
	state.cancel = cancel
	if a.window() != nil {
		_ = a.window().Invalidate()
	}
	util.Go(a.lifecycleCtx, "dictation model compare", func() {
		defer cancel()
		for _, model := range selected {
			if ctx.Err() != nil {
				break
			}
			results, err := speech.DecodeAudioFiles(ctx, speech.LocalModel{
				ID: model.ID, Path: model.Path, ModelType: model.Type, DisplayName: model.Name,
			}, []string{rawPath})
			_ = a.runOnUI("dictation model compare", func() {
				if a.dictationCompare != state || state.generation != generation {
					return
				}
				for i := range state.models {
					if state.models[i].ID != model.ID {
						continue
					}
					if err != nil {
						state.models[i].Status = previewview.DictationModelCompareError
						state.models[i].Error = err.Error()
						break
					}
					state.models[i].Status = previewview.DictationModelCompareDone
					if len(results) > 0 {
						state.models[i].Text = results[0].Text
						state.models[i].Elapsed = results[0].Elapsed
					}
					break
				}
				state.revision++
				if a.window() != nil {
					_ = a.window().Invalidate()
				}
			})
		}
		_ = a.runOnUI("dictation model compare finish", func() {
			if a.dictationCompare != state || state.generation != generation {
				return
			}
			state.busy = false
			state.revision++
			if a.window() != nil {
				_ = a.window().Invalidate()
			}
		})
	})
}

func (a *dictationPreviewController) compareDictationModel(modelID string) {
	a.startDictationModelCompare([]string{modelID})
}

func (a *dictationPreviewController) compareAllDictationModels() {
	state := a.dictationCompare
	if state == nil {
		return
	}
	ids := make([]string, 0, len(state.models))
	for _, model := range state.models {
		ids = append(ids, model.ID)
	}
	a.startDictationModelCompare(ids)
}

// buildDictationHistoryPreview prepares portable text layout before composing the pure comparison view.
func (a *dictationPreviewController) buildDictationHistoryPreview(scrollKey string, data dictationHistoryPreviewData, palette uiPalette, width, height float32) woxwidget.Widget {
	scale := a.appearance.densityMetrics.normalized().scale
	scaled := func(value float32) float32 { return a.appearance.densityMetrics.scaled(value) }
	innerWidth := max(float32(0), width-scaled(52))
	refinedWidth := max(float32(0), innerWidth-scaled(16))
	refinedStyle := woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}
	originalStyle := woxui.TextStyle{Size: scaled(14)}
	refinedLayout := a.textLayout(a.window(), scrollKey+"|dictation-refined", data.RefinedText, refinedStyle, refinedWidth, scaled(28))
	originalLayout := a.textLayout(a.window(), scrollKey+"|dictation-original", data.OriginalText, originalStyle, innerWidth, scaled(22))
	statusWidth := float32(0)
	if status := strings.TrimSpace(data.StatusLabel); status != "" {
		metrics, _ := a.window().MeasureText(status, woxui.TextStyle{Size: scaled(11)})
		statusWidth = scaled(17) + metrics.Size.Width
	}
	props := previewview.DictationHistoryPreviewProps{
		ID: scrollKey, Width: width, Height: height, Scale: scale, Theme: palette.componentTheme(),
		RefinedText: data.RefinedText, OriginalText: data.OriginalText, RefinedLabel: data.RefinedLabel, OriginalLabel: data.OriginalLabel,
		StatusLabel: data.StatusLabel, IsChanged: data.IsChanged, RefinedLayout: refinedLayout, OriginalLayout: originalLayout, StatusWidth: statusWidth,
		AudioLabel: data.AudioLabel, RawAudioLabel: data.RawAudioLabel, RawAudioPath: data.RawAudioPath,
		ProcessedAudioLabel: data.ProcessedAudioLabel, ProcessedAudioPath: data.ProcessedAudioPath,
		RawPlayback: dictationPlaybackProps(a.dictationAudioSnapshot(data.RawAudioPath)), ProcessedPlayback: dictationPlaybackProps(a.dictationAudioSnapshot(data.ProcessedAudioPath)),
		PlayLabel: a.language.translate("i18n:plugin_mediaplayer_play"), PauseLabel: a.language.translate("i18n:plugin_mediaplayer_pause"), OnPlayDiagnosticAudio: a.toggleDictationAudio,
	}
	if a.isDev && strings.TrimSpace(data.RawAudioPath) != "" && strings.TrimSpace(data.ProcessedAudioPath) != "" {
		compare := a.ensureDictationModelCompare(data.RawAudioPath)
		compareTextWidth := max(float32(0), innerWidth-scaled(28))
		props.CompareLabel = a.language.translate("i18n:plugin_dictation_history_model_compare")
		props.CompareAllLabel = a.language.translate("i18n:plugin_dictation_history_model_compare_run_all")
		props.CompareRunLabel = a.language.translate("i18n:plugin_dictation_history_model_compare_run")
		props.CompareRunningLabel = a.language.translate("i18n:plugin_dictation_history_model_compare_running")
		props.CompareEmptyLabel = a.language.translate("i18n:plugin_dictation_history_model_compare_empty")
		props.CompareModels = a.dictationModelCompareProps(compare, scrollKey, a.language.translate("i18n:plugin_dictation_history_model_compare_empty_result"), originalStyle, compareTextWidth, scaled(22))
		props.CompareBusy = compare.busy
		props.OnCompareModel = a.compareDictationModel
		props.OnCompareAll = a.compareAllDictationModels
	}
	return previewview.DictationHistoryPreviewView(props)
}
