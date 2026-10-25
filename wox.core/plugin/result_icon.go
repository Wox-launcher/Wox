package plugin

import (
	"context"

	"wox/common"
	"wox/util/timetracking"
)

func (m *Manager) convertResultIcon(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, resultId string, resultTitle string, icon common.WoxImage) common.WoxImage {
	return m.convertResultIconWithRecorder(ctx, pluginInstance, query, layout, resultId, resultTitle, icon, nil)
}

// convertResultIconWithRecorder lets the finalize hot path aggregate icon conversion diagnostics instead of logging inside each conversion.
func (m *Manager) convertResultIconWithRecorder(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, resultId string, resultTitle string, icon common.WoxImage, recorder func(timetracking.IconConversionDiagnostics, timetracking.IconConversionTimingSummary)) common.WoxImage {
	resultIconSize := m.getResultIconSizeForQuery(pluginInstance, query, layout)
	pluginDirectory := ""
	if pluginInstance != nil {
		pluginDirectory = pluginInstance.PluginDirectory
	}

	diagnostics := timetracking.IconConversionDiagnostics{}
	if recorder != nil {
		diagnostics = timetracking.IconConversionDiagnostics{
			Purpose:     "result_icon",
			QueryId:     query.Id,
			Plugin:      queryDiagnosticPluginLabel(pluginInstance),
			ResultId:    resultId,
			ResultTitle: resultTitle,
			Recorder:    recorder,
		}
	}
	convertedIcon := pluginInstance.convertIcon(ctx, icon, common.IconConversion{Size: resultIconSize, AllowLazy: true, Diagnostics: diagnostics})
	if convertedIcon.ImageType != common.WoxImageTypeLazyLoad {
		return convertedIcon
	}

	// Lazy load markers keep large plugin icons out of every result payload. They
	// contain the original source plus a token used to retrieve the converted
	// thumbnail later. If parsing fails, return the marker so UI still receives an icon.
	payload, payloadErr := common.ParseWoxLazyLoadImagePayload(convertedIcon)
	if payloadErr != nil || payload.Source == nil || payload.Source.IsEmpty() {
		return convertedIcon
	}

	targetSize := payload.TargetSize
	if targetSize <= 0 {
		targetSize = resultIconSize
	}
	// Result icon conversion is the only place that has both plugin path context
	// and stable result/query IDs. Common returns only a source-bearing lazy marker;
	// the result store owns registration together with the cached result lifetime.
	registeredIcon := m.resultStore.registerLazyIcon(ctx, pluginInstance, query, resultId, *payload.Source, pluginDirectory, targetSize, payload.CacheScope)
	if !registeredIcon.IsEmpty() {
		return registeredIcon
	}

	// If a result cannot be tokenized, keep old behavior instead of leaking an
	// unregistered lazy marker to UI.
	if recorder != nil {
		diagnostics.Purpose = "result_icon_lazy_fallback"
		return pluginInstance.convertIcon(ctx, *payload.Source, common.IconConversion{Size: targetSize, CacheScope: payload.CacheScope, Diagnostics: diagnostics})
	}
	return pluginInstance.convertIcon(ctx, *payload.Source, common.IconConversion{Size: targetSize, CacheScope: payload.CacheScope})
}
