package plugin

import (
	"context"
	"sort"
)

// PolishUpdatableResult normalizes only supplied fields before merging them into the cached result.
// Keep this patch flow separate from initial polish: nil fields retain cached values,
// and updates must not repeat ranking or wrap the cached preview in a remote URL.
func (m *Manager) PolishUpdatableResult(ctx context.Context, pluginInstance *Instance, result UpdatableResult) UpdatableResult {
	// Get result cache to update it
	resultCache, found := m.resultStore.findWithContext(ctx, result.Id)
	if !found {
		return result // Result not in cache, just return as-is
	}

	// Polish actions if they are being updated
	if result.Actions != nil {
		actions := *result.Actions

		normalizeResultActionDefaults(actions)
		// Updates must convert both action images, just like initial results.
		convertActionIcons(ctx, actions, pluginInstance, nil)
		m.attachExternalActionCallbacks(pluginInstance, actions)

		ensureDefaultResultAction(actions)
		normalizeResultActionHotkeys(actions)

		// Move default action to first position
		sort.Slice(actions, func(i, j int) bool {
			return actions[i].IsDefault
		})

		bindingResult := resultCache.Result
		bindingResult.Actions = actions
		if resultCache.Query.Env.IsMRU {
			// MRU restore owns its system actions, including the removal callback's
			// captured item hash. Reuse them after GetUpdatableResult filters them out.
			for _, action := range resultCache.Result.Actions {
				if action.IsSystemAction && action.Id != systemActionSetResultHotkeyID && action.Id != systemActionSetResultAliasID {
					actions = append(actions, action)
				}
			}
			actions = append(actions, m.newResultBindingActions(ctx, pluginInstance, resultCache.Query, bindingResult, resultCache.SourceMRUHash)...)
		} else {
			// Regular queries regenerate system actions to reflect pin/unpin changes.
			actions = append(actions, m.getDefaultActions(ctx, pluginInstance, resultCache.Query, bindingResult, resultCache.SourceMRUHash)...)
		}

		m.translateResultActions(ctx, pluginInstance, actions)

		result.Actions = &actions

		// Merge callbacks that cannot round-trip through plugin hosts.
		for i := range actions {
			// Find matching action in cache by ID
			var cachedAction *QueryResultAction
			for j := range resultCache.Result.Actions {
				if resultCache.Result.Actions[j].Id == actions[i].Id {
					cachedAction = &resultCache.Result.Actions[j]
					break
				}
			}

			// Preserve callbacks when a host returns only UI-safe fields.
			if cachedAction != nil {
				if actions[i].Action == nil && cachedAction.Action != nil {
					actions[i].Action = cachedAction.Action
				}
				if actions[i].Type == QueryResultActionTypeForm && actions[i].OnSubmit == nil && cachedAction.OnSubmit != nil {
					actions[i].OnSubmit = cachedAction.OnSubmit
				}
			}
		}

		// Update cache with merged actions
		resultCache.Result.Actions = actions
	}

	// Translate title if present
	if result.Title != nil {
		translated := m.translatePlugin(ctx, pluginInstance, *result.Title)
		result.Title = &translated
		resultCache.Result.Title = translated
	}

	// Translate subtitle if present
	if result.SubTitle != nil {
		translated := m.translatePlugin(ctx, pluginInstance, *result.SubTitle)
		result.SubTitle = &translated
		resultCache.Result.SubTitle = translated
	}

	// Translate tails if present
	if result.Tails != nil {
		tails := *result.Tails
		m.translateResultTails(ctx, pluginInstance, tails)
		convertResultTailImages(ctx, pluginInstance, tails)

		tails = m.appendPinnedResultTail(ctx, pluginInstance, resultCache.Query, resultCache.Result, tails)

		tails = m.appendDevScoreTail(ctx, tails, resultCache.Result.Score)

		result.Tails = &tails
		resultCache.Result.Tails = tails
	}

	// Translate preview properties if present
	_, scriptUpdate, _ := WebViewScriptUpdate(result.Preview)
	if result.Preview != nil && !scriptUpdate {
		preview := *result.Preview
		if shouldHidePreviewForGlobalQuery(resultCache.Query, preview) {
			preview = WoxPreview{}
		} else {
			// Updated previews must use the same list normalization as initial
			// query results. Long-running actions commonly update list rows in place,
			// so icon conversion and row text translation cannot live only in the
			// first result-processing path.
			preview = m.applyDefaultSelectionPreview(ctx, resultCache.Query, preview)
			preview = m.normalizeListPreviewData(ctx, pluginInstance, preview)
			preview = m.normalizePreviewMetadata(ctx, pluginInstance, preview)
		}
		result.Preview = &preview
		resultCache.Result.Preview = preview
	}

	// Update drag data in cache if present. An empty or invalid payload clears the
	// current drag surface instead of leaving stale file paths on a live result.
	if result.DragData != nil {
		dragData := normalizeQueryResultDragData(result.DragData)
		result.DragData = dragData
		resultCache.Result.DragData = dragData
	}

	m.applyResultBindingTitleTags(ctx, pluginInstance, resultCache.Query, &resultCache.Result, resultCache.SourceMRUHash)
	titleTags := append([]QueryResultTitleTag(nil), resultCache.Result.TitleTags...)
	result.TitleTags = &titleTags

	// Update icon in cache if present
	if result.Icon != nil {
		// GetUpdatableResult returns the current icon as a non-nil field, and
		// callers often pass it back after changing only tails or actions. Treat an
		// unchanged icon as a no-op so refresh updates do not repeatedly touch the
		// image conversion/cache path.
		if result.Icon.ImageType == resultCache.Result.Icon.ImageType && result.Icon.ImageData == resultCache.Result.Icon.ImageData {
			result.Icon = nil
			return result
		}

		// Updated result icons must keep the same surface-aware size as initial results;
		// otherwise a grid plugin can become blurry after sending an icon update.
		convertedIcon := m.convertResultIcon(ctx, pluginInstance, resultCache.Query, resultCache.Layout, result.Id, resultCache.Result.Title, *result.Icon)
		result.Icon = &convertedIcon
		resultCache.Result.Icon = *result.Icon
	}

	return result
}

func (m *Manager) GetUpdatableResult(ctx context.Context, resultId string) *UpdatableResult {
	// Try to find the result in the cache
	resultCache, found := m.resultStore.findWithContext(ctx, resultId)
	if !found {
		return nil // Result not found (no longer visible)
	}

	// Construct UpdatableResult from cache
	title := resultCache.Result.Title
	subTitle := resultCache.Result.SubTitle
	icon := resultCache.Result.Icon
	preview := resultCache.Result.Preview
	var dragData *QueryResultDragData
	if resultCache.Result.DragData != nil {
		dragDataCopy := *resultCache.Result.DragData
		dragDataCopy.Files = append([]string(nil), resultCache.Result.DragData.Files...)
		dragData = &dragDataCopy
	}

	// Make a copy of tails to avoid modifying cache when developer appends to it
	// Filter out system tails (they will be added back in polish)
	tails := []QueryResultTail{}
	for _, tail := range resultCache.Result.Tails {
		if !tail.IsSystemTail {
			tails = append(tails, tail)
		}
	}

	// Make a copy of actions to avoid modifying cache when developer modifies it
	// Filter out system actions (they will be added back in polish)
	actions := []QueryResultAction{}
	for _, action := range resultCache.Result.Actions {
		if !action.IsSystemAction {
			actions = append(actions, action)
		}
	}

	titleTags := append([]QueryResultTitleTag(nil), resultCache.Result.TitleTags...)
	return &UpdatableResult{
		Id:        resultId,
		Title:     &title,
		SubTitle:  &subTitle,
		Icon:      &icon,
		Preview:   &preview,
		Tails:     &tails,
		TitleTags: &titleTags,
		Actions:   &actions,
		DragData:  dragData,
	}
}
