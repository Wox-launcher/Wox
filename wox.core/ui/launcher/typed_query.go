package launcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/ui/contract"
	"wox/util"
	utilselection "wox/util/selection"
)

func (a *App) startTypedQuery(query plainQuery, skipCompletionHint bool) error {
	if a.destroyed.Load() {
		return context.Canceled
	}
	ctx := a.lifecycleCtx
	return a.services.StartQuery(ctx, contract.QueryRequest{
		RequestID:          newID(),
		SessionID:          a.sessionID,
		Query:              toCorePlainQuery(query),
		SkipCompletionHint: skipCompletionHint,
		SentTimestamp:      time.Now().UnixMilli(),
	}, a)
}

func toCorePlainQuery(query plainQuery) common.PlainQuery {
	return common.PlainQuery{
		QueryId:   query.QueryID,
		QueryType: query.QueryType,
		QueryText: query.QueryText,
		QuerySelection: utilselection.Selection{
			Type:      utilselection.SelectionType(query.QuerySelection.Type),
			Text:      query.QuerySelection.Text,
			FilePaths: append([]string(nil), query.QuerySelection.FilePaths...),
		},
		QueryScope:       toCoreQueryScope(query.QueryScope),
		QueryRefinements: cloneStringMap(query.QueryRefinements),
		ContextData:      common.ContextData(cloneStringMap(query.ContextData)),
	}
}

func toCoreQueryScope(scope queryScope) common.QueryScope {
	if len(scope.Plugins) == 0 {
		return common.QueryScope{}
	}
	plugins := make([]common.QueryScopePlugin, len(scope.Plugins))
	for index, item := range scope.Plugins {
		plugins[index] = common.QueryScopePlugin{PluginID: item.PluginID, Command: item.Command}
	}
	return common.QueryScope{Plugins: plugins}
}

func fromCoreQueryScope(scope common.QueryScope) queryScope {
	cloned := scope.Clone()
	if len(cloned.Plugins) == 0 {
		return queryScope{}
	}
	plugins := make([]queryScopePlugin, len(cloned.Plugins))
	for index, item := range cloned.Plugins {
		plugins[index] = queryScopePlugin{PluginID: item.PluginID, Command: item.Command}
	}
	return queryScope{Plugins: plugins}
}

// ApplyQueryResponse updates launcher rendering from one typed core snapshot.
func (a *App) ApplyQueryResponse(_ context.Context, response contract.QueryResponse) {
	if util.IsDev() {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("native crash trace: query response enter queryId=%s results=%d final=%t", response.QueryID, len(response.Response.Results), response.IsFinal))
	}
	results := make([]queryResult, len(response.Response.Results))
	for index := range response.Response.Results {
		results[index] = fromCoreQueryResult(response.Response.Results[index])
	}
	layout := fromCoreQueryLayout(response.Response.Layout)
	if len(response.Response.ScopeIcons) > 0 {
		layout.ScopeIcons = make([]woxImage, 0, len(response.Response.ScopeIcons))
		for _, icon := range response.Response.ScopeIcons {
			layout.ScopeIcons = append(layout.ScopeIcons, fromCoreImage(icon))
		}
	}
	refinements := fromCoreQueryRefinements(response.Response.Refinements)
	queryContext := queryContext{IsGlobalQuery: response.Response.Context.IsGlobalQuery, PluginID: response.Response.Context.PluginId}
	if err := a.runOnUI("apply query response", func() {
		if util.IsDev() {
			util.GetLogger().Debug(context.Background(), fmt.Sprintf("native crash trace: query response ui enter queryId=%s results=%d final=%t", response.QueryID, len(results), response.IsFinal))
		}
		if !a.isDestroyed() {
			a.applyResults(response.QueryID, results, &layout, &refinements, &queryContext, response.Response.QueryStartTimestamp, response.IsFinal)
		}
		if util.IsDev() {
			util.GetLogger().Debug(context.Background(), fmt.Sprintf("native crash trace: query response ui exit queryId=%s", response.QueryID))
		}
	}); err != nil {
		log.Printf("dispatch query response: %v", err)
	}
	if util.IsDev() {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("native crash trace: query response exit queryId=%s", response.QueryID))
	}
}

// ApplyQueryCompletionHint applies a typed inline-completion candidate.
func (a *App) ApplyQueryCompletionHint(_ context.Context, queryID string, hint *plugin.QueryCompletionHint) {
	var converted *queryCompletionHint
	if hint != nil {
		converted = &queryCompletionHint{
			InputPrefix:    hint.InputPrefix,
			CompletionText: hint.CompletionText,
			Suffix:         hint.Suffix,
			Source:         hint.Source,
			Score:          hint.Score,
		}
	}
	if err := a.runOnUI("apply query completion hint", func() {
		if a.isDestroyed() {
			return
		}
		if queryID != a.query.QueryID || !a.completionHintValidLocked(converted) {
			if queryID == a.query.QueryID {
				a.completionHint = nil
			}
			return
		}
		copy := *converted
		a.completionHint = &copy
		_ = a.window.Invalidate()
	}); err != nil {
		log.Printf("dispatch query completion hint: %v", err)
	}
}

// ApplyQueryError reports typed query failures without disturbing a newer query.
func (a *App) ApplyQueryError(_ context.Context, queryID string, err error) {
	if err == nil {
		return
	}
	if dispatchErr := a.runOnUI("apply query error", func() {
		if a.isDestroyed() {
			return
		}
		current := a.query.QueryID == queryID
		if current {
			a.resetQueryLoadingLocked()
			log.Printf("query %s failed: %v", queryID, err)
			if a.window != nil {
				_ = a.window.Invalidate()
			}
		}
	}); dispatchErr != nil {
		log.Printf("dispatch query error: %v", dispatchErr)
	}
}

func (a *App) loadTypedMRU(queryID string) {
	results, err := a.services.QueryMRU(context.Background(), a.sessionID, queryID)
	if err != nil {
		log.Printf("load MRU results: %v", err)
		return
	}
	converted := make([]queryResult, len(results))
	for index := range results {
		converted[index] = fromCoreQueryResult(results[index])
		converted[index].QueryID = queryID
	}
	if err := a.runOnUI("apply MRU results", func() {
		a.applyResults(queryID, converted, &queryLayout{}, nil, nil, 0, true)
	}); err != nil {
		log.Printf("dispatch MRU results: %v", err)
	}
}

func fromCoreQueryResult(result plugin.QueryResultUI) queryResult {
	actions := make([]resultAction, len(result.Actions))
	for index := range result.Actions {
		actions[index] = fromCoreResultAction(result.Actions[index])
	}
	tails := make([]resultTail, len(result.Tails))
	for index := range result.Tails {
		tails[index] = resultTail{
			Type:         result.Tails[index].Type,
			Text:         result.Tails[index].Text,
			TextCategory: result.Tails[index].TextCategory,
			Image:        fromCoreImage(result.Tails[index].Image),
			ImageWidth:   result.Tails[index].ImageWidth,
			ImageHeight:  result.Tails[index].ImageHeight,
			Tooltip:      result.Tails[index].Tooltip,
			ContextData:  cloneStringMap(result.Tails[index].ContextData),
		}
	}
	tags := make([]previewTag, len(result.Preview.PreviewTags))
	for index := range result.Preview.PreviewTags {
		tags[index] = previewTag{Label: result.Preview.PreviewTags[index].Label, Tooltip: result.Preview.PreviewTags[index].Tooltip}
	}
	return queryResult{
		QueryID:  result.QueryId,
		ID:       result.Id,
		Title:    result.Title,
		SubTitle: result.SubTitle,
		Icon:     fromCoreImage(result.Icon),
		Preview: queryPreview{
			PreviewType:        result.Preview.PreviewType,
			PreviewData:        result.Preview.PreviewData,
			PreviewOverlayData: result.Preview.PreviewOverlayData,
			PreviewTags:        tags,
			PreviewProperties:  cloneStringMap(result.Preview.PreviewProperties),
			ScrollPosition:     result.Preview.ScrollPosition,
		},
		Tails:   tails,
		Actions: actions,
		DragData: func() *queryResultDragData {
			if result.DragData == nil {
				return nil
			}
			return &queryResultDragData{Type: result.DragData.Type, Files: append([]string(nil), result.DragData.Files...)}
		}(),
		IsGroup: result.IsGroup,
	}
}

func fromCoreResultAction(action plugin.QueryResultActionUI) resultAction {
	definitions := make([]formDefinition, 0, len(action.Form))
	for _, item := range action.Form {
		definition, ok := fromCoreFormDefinition(item)
		if ok {
			definitions = append(definitions, definition)
		}
	}
	return resultAction{
		ID:                     action.Id,
		Type:                   action.Type,
		Name:                   action.Name,
		SearchAliases:          append([]string(nil), action.SearchAliases...),
		Icon:                   fromCoreImage(action.Icon),
		IsDefault:              action.IsDefault,
		PreventHideAfterAction: action.PreventHideAfterAction,
		Hotkey:                 action.Hotkey,
		Form:                   definitions,
	}
}

func fromCoreQueryLayout(layout plugin.QueryLayout) queryLayout {
	converted := queryLayout{ResultPreviewWidthRatio: layout.ResultPreviewWidthRatio, ChatMode: layout.ChatMode}
	if layout.Icon != nil {
		converted.Icon = fromCoreImage(*layout.Icon)
	}
	if layout.GridLayout != nil {
		converted.GridLayout = &gridLayout{
			Columns:     layout.GridLayout.Columns,
			ShowTitle:   layout.GridLayout.ShowTitle,
			ItemPadding: layout.GridLayout.ItemPadding,
			ItemMargin:  layout.GridLayout.ItemMargin,
			AspectRatio: layout.GridLayout.AspectRatio,
			Commands:    append([]string(nil), layout.GridLayout.Commands...),
		}
	}
	return converted
}

func fromCoreQueryRefinements(refinements []plugin.QueryRefinement) []queryRefinement {
	converted := make([]queryRefinement, len(refinements))
	for index, refinement := range refinements {
		options := make([]queryRefinementOption, len(refinement.Options))
		for optionIndex, option := range refinement.Options {
			options[optionIndex] = queryRefinementOption{
				Value: option.Value, Title: option.Title, Icon: fromCoreImage(option.Icon), Keywords: append([]string(nil), option.Keywords...), Count: option.Count,
			}
		}
		converted[index] = queryRefinement{
			ID: refinement.Id, Title: refinement.Title, Type: refinement.Type, Options: options,
			DefaultValue: append([]string(nil), refinement.DefaultValue...), Hotkey: refinement.Hotkey, Persist: refinement.Persist,
		}
	}
	return converted
}

func fromCoreImage(image common.WoxImage) woxImage {
	return woxImage{ImageType: image.ImageType, ImageData: image.ImageData}
}

func fromCoreFormDefinition(item definition.PluginSettingDefinitionItem) (formDefinition, bool) {
	if item.Type == definition.PluginSettingDefinitionTypeNewLine {
		return formDefinition{Type: string(item.Type)}, true
	}
	if item.Value == nil {
		return formDefinition{}, false
	}
	converted := formDefinition{Type: string(item.Type)}
	switch value := item.Value.(type) {
	case *definition.PluginSettingValueHead:
		converted.Value = formDefinitionValue{Content: value.Content, Tooltip: value.Tooltip}
	case *definition.PluginSettingValueTextBox:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Suffix: value.Suffix, DefaultValue: value.DefaultValue, Tooltip: value.Tooltip, MaxLines: value.MaxLines, Validators: fromCoreValidators(value.Validators)}
	case *definition.PluginSettingValueDirPath:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, DefaultValue: value.DefaultValue, Tooltip: value.Tooltip, Validators: fromCoreValidators(value.Validators)}
	case *definition.PluginSettingValueCheckBox:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, DefaultValue: value.DefaultValue, Tooltip: value.Tooltip}
	case *definition.PluginSettingValueSelect:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Suffix: value.Suffix, DefaultValue: value.DefaultValue, Tooltip: value.Tooltip, IsMulti: value.IsMulti, Options: fromCoreSelectOptions(value.Options), Validators: fromCoreValidators(value.Validators)}
	case *definition.PluginSettingValueSelectAIModel:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Suffix: value.Suffix, DefaultValue: value.DefaultValue, Tooltip: value.Tooltip, Validators: fromCoreValidators(value.Validators)}
	case *definition.PluginSettingValueLabel:
		converted.Value = formDefinitionValue{Content: value.Content, Tooltip: value.Tooltip}
	case *definition.PluginSettingValueTable:
		columns := make([]formTableColumn, len(value.Columns))
		for index, column := range value.Columns {
			columns[index] = formTableColumn{
				Key: column.Key, Label: column.Label, Tooltip: column.Tooltip, Width: column.Width, Type: column.Type,
				Validators: fromCoreValidators(column.Validators), SelectOptions: fromCoreSelectOptions(column.SelectOptions), TextMaxLines: column.TextMaxLines,
				HideInTable: column.HideInTable, HideInUpdate: column.HideInUpdate, AllowedHotkeyKinds: append([]string(nil), column.AllowedHotkeyKinds...),
			}
		}
		converted.Value = formDefinitionValue{Key: value.Key, DefaultValue: value.DefaultValue, Title: value.Title, Tooltip: value.Tooltip, Columns: columns, SortColumnKey: value.SortColumnKey, SortOrder: value.SortOrder, MaxHeight: value.MaxHeight}
	case *definition.PluginSettingValueDictationHotkey:
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Tooltip: value.Tooltip, DefaultValue: value.DefaultValue}
	case *definition.PluginSettingValueDictationModel:
		options := make([]formOption, len(value.Options))
		for index, option := range value.Options {
			options[index] = formOption{ID: option.ID, Value: option.ID, DisplayName: option.DisplayName, Description: option.Description, Languages: option.Languages, Recommended: option.Recommended, Status: string(option.Status), DownloadProgress: option.DownloadProgress, SizeMB: option.SizeMB, Error: option.Error}
		}
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Tooltip: value.Tooltip, DefaultValue: value.DefaultValue, Options: options}
	case *definition.PluginSettingValueOCRModel:
		options := make([]formOption, len(value.Options))
		for index, option := range value.Options {
			options[index] = formOption{ID: option.ID, Value: option.ID, DisplayName: option.DisplayName, Description: option.Description, Languages: option.Languages, Recommended: option.Recommended, Available: option.Available, Status: option.Status, DownloadProgress: option.DownloadProgress, SizeMB: option.SizeMB, Error: option.Error}
		}
		converted.Value = formDefinitionValue{Key: value.Key, Label: value.Label, Tooltip: value.Tooltip, DefaultValue: value.DefaultValue, Options: options}
	case *definition.PluginSettingValueStats:
		rows := make([]formStatsRow, len(value.Rows))
		for index, row := range value.Rows {
			rows[index] = formStatsRow{Label: row.Label, Value: row.Value}
		}
		converted.Value = formDefinitionValue{Key: value.Key, Title: value.Title, Tooltip: value.Tooltip, Rows: rows}
	case *definition.PluginSettingValueService:
		actions := make([]formServiceAction, len(value.Actions))
		for index, action := range value.Actions {
			actions[index] = formServiceAction{ID: action.ID, Label: action.Label, Primary: action.Primary, Danger: action.Danger, Enabled: action.Enabled}
		}
		converted.Value = formDefinitionValue{Key: value.Key, Title: value.Title, Description: value.Description, Status: value.Status, Detail: value.Detail, Actions: actions}
	default:
		log.Printf("skip unsupported typed form definition %s (%T)", item.Type, item.Value)
		return formDefinition{}, false
	}
	return converted, true
}

func fromCoreSelectOptions(options []definition.PluginSettingValueSelectOption) []formOption {
	converted := make([]formOption, len(options))
	for index, option := range options {
		converted[index] = formOption{
			Label: option.Label, Value: option.Value,
			Icon: woxImage{ImageType: option.Icon.ImageType, ImageData: option.Icon.ImageData},
		}
	}
	return converted
}

func fromCoreValidators(validators []validator.PluginSettingValidator) []formValidator {
	converted := make([]formValidator, len(validators))
	for index, item := range validators {
		converted[index].Type = string(item.Type)
		if value, ok := item.Value.(*validator.PluginSettingValidatorIsNumber); ok {
			converted[index].Value = formValidatorValue{IsInteger: value.IsInteger, IsFloat: value.IsFloat}
		}
	}
	return converted
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func typedQueryError(method string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", method, err)
}
