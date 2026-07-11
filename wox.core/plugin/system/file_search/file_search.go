package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system/file_search/indexpolicy"
	shellplugin "wox/plugin/system/shell"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/fileicon"
	"wox/util/filesearch"
	"wox/util/nativecontextmenu"
	"wox/util/permission"
	"wox/util/shell"
	"wox/util/trash"
)

var fileIcon = common.PluginFileIcon

const fileRootsSettingKey = "roots"
const fileIgnorePatternsSettingKey = "ignorePatterns"
const fileSkipHiddenFilesSettingKey = "skipHiddenFiles"
const fileShowPreviewSettingKey = "showPreview"
const fileSearchToolbarMsgID = "file-search-status"
const fileSearchStatusCommand = "status"

// Content search setting keys.
const contentSearchEnabledKey = "contentSearchEnabled"
const contentSearchExtensionsKey = "contentSearchExtensions"

const contentSearchToolbarMsgID = "file-search-content-status"

const (
	slowFileSearchQueryThresholdMs    int64 = 40
	slowFileSearchStageThresholdMs    int64 = 15
	incrementalToolbarMinimumShowMs   int64 = 1000
	fullIndexCompletionToolbarHoldMs  int64 = 1000 * 5
	contentCrawlDebounceWindow              = 1 * time.Second
	toolbarActivityPathMaxChars             = 42
	toolbarErrorReasonMaxChars              = 28
	fileSearchResultLimit                   = 100
	fileSearchRefinedCandidateLimit         = 300
	fileSearchRefinementSortScoreStep       = 1000000
)

const (
	fileSearchTypeRefinementKey    = "file_type"
	fileSearchTypeRefinementAll    = "all"
	fileSearchTypeRefinementFile   = "file"
	fileSearchTypeRefinementFolder = "folder"

	fileSearchSortRefinementKey       = "file_sort"
	fileSearchSortRefinementRelevance = "relevance"
	fileSearchSortRefinementName      = "name"
	fileSearchSortRefinementModified  = "modified"
	fileSearchSortRefinementSize      = "size"
)

type fileRootSetting struct {
	Path string `json:"Path"`
}

type fileIgnorePatternSetting struct {
	Pattern string `json:"Pattern"`
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &FileSearchPlugin{})
}

type FileSearchPlugin struct {
	api                      plugin.API
	engine                   *filesearch.Engine
	indexPolicy              *fileSearchIndexPolicy
	unsubscribeStatusChange  func()
	toolbarMsgStateMu        sync.Mutex
	lastToolbarMsgSignature  string
	completionHoldUntilMs    int64
	completionHoldGeneration int64
	contentSearchStateMu     sync.Mutex
	contentSearchGeneration  int64
	contentCrawlCancel       context.CancelFunc
	contentCrawlTimer        *time.Timer
	contentCrawlPending      bool
	contentCrawlPendingGen   int64
	contentCrawlRunning      bool
	contentCrawlRunningGen   int64
}

type fileSearchQueryDiagnostics struct {
	toolbarElapsedMs int64
	searchElapsedMs  int64
	buildElapsedMs   int64
	statElapsedMs    int64
	statCount        int
	statMissCount    int
	directoryCount   int
	thumbnailCount   int
}

func (c *FileSearchPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "979d6363-025a-4f51-88d3-0b04e9dc56bf",
		Name:          "i18n:plugin_file_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_file_plugin_description",
		Icon:          fileIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"f",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:          fileRootsSettingKey,
					DefaultValue: defaultFileSearchRootPathsJSON(),
					Title:        "i18n:plugin_file_setting_roots_title",
					Tooltip:      "i18n:plugin_file_setting_roots_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Path",
							Label: "i18n:plugin_file_setting_root_path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          fileSkipHiddenFilesSettingKey,
					Label:        "i18n:plugin_file_setting_skip_hidden_files_label",
					Tooltip:      "i18n:plugin_file_setting_skip_hidden_files_tooltip",
					DefaultValue: "true",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          fileShowPreviewSettingKey,
					Label:        "i18n:plugin_file_setting_show_preview_label",
					Tooltip:      "i18n:plugin_file_setting_show_preview_tooltip",
					DefaultValue: "true",
				},
			},
			{
				Type:               definition.PluginSettingDefinitionTypeTable,
				IsPlatformSpecific: true,
				Value: &definition.PluginSettingValueTable{
					Key:          fileIgnorePatternsSettingKey,
					DefaultValue: defaultFileSearchIgnorePatternsJSON(),
					Title:        "i18n:plugin_file_setting_ignore_patterns_title",
					Tooltip:      "i18n:plugin_file_setting_ignore_patterns_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "Pattern",
							Label:   "i18n:plugin_file_setting_ignore_pattern",
							Tooltip: "i18n:plugin_file_setting_ignore_pattern_tooltip",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
			// Content search settings.
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          contentSearchEnabledKey,
					Label:        "i18n:plugin_file_setting_content_search_enabled",
					Tooltip:      "i18n:plugin_file_setting_content_search_enabled_tooltip",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:          contentSearchExtensionsKey,
					DefaultValue: defaultContentSearchExtensionsJSON(),
					Title:        "i18n:plugin_file_setting_content_extensions",
					Tooltip:      "i18n:plugin_file_setting_content_extensions_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Extension",
							Label: "i18n:plugin_file_setting_content_extension",
							Type:  definition.PluginSettingValueTableColumnTypeText,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *FileSearchPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.indexPolicy = newFileSearchIndexPolicy()
	c.indexPolicy.SetIgnorePatterns(c.getConfiguredIgnorePatternValues(ctx))
	c.indexPolicy.SetSkipHiddenFiles(c.getConfiguredSkipHiddenFiles(ctx))

	engine, initErr := filesearch.NewEngineWithOptions(ctx, filesearch.EngineOptions{
		Policy: c.indexPolicy.toFilesearchPolicy(),
	})
	if initErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, initErr.Error())
		return
	}
	c.engine = engine
	c.api.Log(ctx, plugin.LogLevelInfo, "File search engine initialized")
	c.unsubscribeStatusChange = c.engine.OnStatusChanged(func(status filesearch.StatusSnapshot) {
		c.handleStatusChanged(status)
	})
	if util.IsDev() {
		// Feature addition: expose a dev-only diagnostic command for live File
		// Search triage. Runtime commands keep this out of production builds while
		// making the `status` command discoverable during local debugging sessions.
		c.api.RegisterQueryCommands(ctx, []plugin.MetadataCommand{
			{Command: fileSearchStatusCommand, Description: "File Search internal status"},
		})
	}

	// Sync toolbar state once when the session enters file-search query mode because
	// the previous per-keystroke refresh forced a synchronous UI round-trip on every
	// Query() call even though later status changes already arrive through events.
	// Enter-time sync keeps the initial state correct and lets inactive sessions rely
	// on manager-side ignore behavior instead of blocking every search.
	c.api.OnEnterPluginQuery(ctx, func(ctx context.Context) {
		c.syncToolbarMsg(ctx, false)
	})
	c.api.OnLeavePluginQuery(ctx, func(ctx context.Context) {
		// Reset the local de-duplication state when the file-search query session ends.
		// The manager already clears the visible toolbar msg on leave, so keeping the
		// old signature here would incorrectly suppress the first toolbar refresh when
		// the user enters file-search again during the same indexing run.
		c.resetToolbarMsgState()
	})

	c.syncUserRoots(ctx)

	// Initialize content index if enabled. Full crawl requests are debounced and
	// serialized; the incremental hook is installed after the active crawl completes.
	if c.isContentSearchEnabled(ctx) {
		generation := c.nextContentSearchGeneration()
		c.requestContentCrawl(ctx, generation)
	} else if err := c.engine.ResetContentIndex(ctx); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to remove disabled content search database: %s", err.Error()))
	}

	c.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == fileRootsSettingKey {
			c.syncUserRoots(callbackCtx)
			c.onFileSearchPolicyChanged(callbackCtx)
			return
		}
		if key == fileIgnorePatternsSettingKey {
			c.syncIgnorePatterns(callbackCtx)
			c.onFileSearchPolicyChanged(callbackCtx)
			return
		}
		if key == fileSkipHiddenFilesSettingKey {
			c.syncSkipHiddenFiles(callbackCtx)
			c.onFileSearchPolicyChanged(callbackCtx)
			return
		}
		if key == contentSearchEnabledKey {
			if value == "true" {
				generation := c.nextContentSearchGeneration()
				c.requestContentCrawl(callbackCtx, generation)
			} else {
				c.nextContentSearchGeneration()
				c.stopContentCrawlWork()
				if err := c.engine.ResetContentIndex(callbackCtx); err != nil {
					c.api.Log(callbackCtx, plugin.LogLevelWarning, fmt.Sprintf("failed to remove content search database: %s", err.Error()))
				}
				c.api.ClearToolbarMsg(callbackCtx, contentSearchToolbarMsgID)
			}
			return
		}
		if key == contentSearchExtensionsKey {
			c.onContentPolicyChanged(callbackCtx)
			return
		}
	})

	c.api.OnUnload(ctx, func(ctx context.Context) {
		c.nextContentSearchGeneration()
		c.stopContentCrawlWork()
		if c.unsubscribeStatusChange != nil {
			c.unsubscribeStatusChange()
			c.unsubscribeStatusChange = nil
		}
		if c.engine != nil {
			_ = c.engine.Close()
		}
		c.api.ClearToolbarMsg(ctx, contentSearchToolbarMsgID)
	})
}

// isContentSearchEnabled returns whether content search is toggled on in settings.
func (c *FileSearchPlugin) isContentSearchEnabled(ctx context.Context) bool {
	return c.api.GetSetting(ctx, contentSearchEnabledKey) == "true"
}

// nextContentSearchGeneration invalidates delayed content-search work that
// belongs to the previous content-search policy.
func (c *FileSearchPlugin) nextContentSearchGeneration() int64 {
	c.contentSearchStateMu.Lock()
	defer c.contentSearchStateMu.Unlock()
	c.contentSearchGeneration++
	return c.contentSearchGeneration
}

// stopContentCrawlWork cancels scheduled and running content crawl work.
func (c *FileSearchPlugin) stopContentCrawlWork() {
	c.contentSearchStateMu.Lock()
	defer c.contentSearchStateMu.Unlock()
	c.stopContentCrawlWorkLocked()
}

// stopContentCrawlWorkLocked clears queued crawl work and cancels the active
// crawl context while leaving the running slot to be released by completion.
func (c *FileSearchPlugin) stopContentCrawlWorkLocked() {
	if c.contentCrawlTimer != nil {
		c.contentCrawlTimer.Stop()
		c.contentCrawlTimer = nil
	}
	c.contentCrawlPending = false
	c.contentCrawlPendingGen = 0
	if c.contentCrawlCancel != nil {
		c.contentCrawlCancel()
		c.contentCrawlCancel = nil
	}
}

// isContentSearchCurrent checks whether delayed content-search work is still current.
func (c *FileSearchPlugin) isContentSearchCurrent(ctx context.Context, generation int64) bool {
	c.contentSearchStateMu.Lock()
	defer c.contentSearchStateMu.Unlock()
	return c.isContentSearchCurrentLocked(ctx, generation)
}

// runIfContentSearchCurrent runs fn only when the delayed content-search work is still current.
func (c *FileSearchPlugin) runIfContentSearchCurrent(ctx context.Context, generation int64, fn func()) bool {
	c.contentSearchStateMu.Lock()
	defer c.contentSearchStateMu.Unlock()
	if !c.isContentSearchCurrentLocked(ctx, generation) {
		return false
	}
	fn()
	return true
}

func (c *FileSearchPlugin) isContentSearchCurrentLocked(ctx context.Context, generation int64) bool {
	return c.contentSearchGeneration == generation && c.isContentSearchEnabled(ctx)
}

// requestContentCrawl debounces full content crawl requests and keeps only the
// latest content-search policy while a crawl is already running.
func (c *FileSearchPlugin) requestContentCrawl(ctx context.Context, generation int64) {
	if c.engine == nil {
		return
	}

	c.contentSearchStateMu.Lock()
	defer c.contentSearchStateMu.Unlock()
	if !c.isContentSearchCurrentLocked(ctx, generation) {
		return
	}
	c.contentCrawlPending = true
	c.contentCrawlPendingGen = generation
	if c.contentCrawlRunning {
		if c.contentCrawlRunningGen != generation && c.contentCrawlCancel != nil {
			c.contentCrawlCancel()
		}
		return
	}
	c.resetContentCrawlTimerLocked()
}

// resetContentCrawlTimerLocked restarts the quiet window used to coalesce
// repeated content crawl requests from settings table edits.
func (c *FileSearchPlugin) resetContentCrawlTimerLocked() {
	if c.contentCrawlTimer != nil {
		c.contentCrawlTimer.Stop()
	}
	c.contentCrawlTimer = time.AfterFunc(contentCrawlDebounceWindow, func() {
		c.startPendingContentCrawl(util.NewTraceContext())
	})
}

// startPendingContentCrawl starts the latest debounced full crawl request.
func (c *FileSearchPlugin) startPendingContentCrawl(ctx context.Context) {
	c.contentSearchStateMu.Lock()
	if c.contentCrawlRunning || !c.contentCrawlPending {
		c.contentSearchStateMu.Unlock()
		return
	}
	generation := c.contentCrawlPendingGen
	if !c.isContentSearchCurrentLocked(ctx, generation) {
		c.contentCrawlPending = false
		c.contentCrawlPendingGen = 0
		c.contentSearchStateMu.Unlock()
		return
	}
	runningCtx, cancel := context.WithCancel(ctx)
	c.contentCrawlPending = false
	c.contentCrawlPendingGen = 0
	c.contentCrawlRunning = true
	c.contentCrawlRunningGen = generation
	c.contentCrawlCancel = cancel
	c.contentCrawlTimer = nil
	c.contentSearchStateMu.Unlock()

	// Wait for filesearch to finish indexing before starting content crawl.
	c.waitForFileSearchIdle(runningCtx)
	if runningCtx.Err() != nil || !c.isContentSearchCurrent(ctx, generation) {
		c.finishContentCrawl(ctx, generation, false)
		return
	}

	crawlState, _ := c.engine.GetContentCrawlState(runningCtx)
	stats, _ := c.engine.ContentStats(runningCtx)
	if crawlState == "complete" && stats.DocCount > 0 {
		c.api.Log(runningCtx, plugin.LogLevelInfo, "Content index already complete, skipping crawl")
		c.finishContentCrawl(runningCtx, generation, true)
		return
	}

	roots := c.getContentSearchRootRecords(runningCtx)
	fsPolicy := c.indexPolicy.toFilesearchPolicy()
	exts := filesearch.ContentExtensionsFromList(filesearch.ContentExtensionListFromSetting(c.api.GetSetting(runningCtx, contentSearchExtensionsKey)))

	c.contentSearchStateMu.Lock()
	if !c.contentCrawlRunning || c.contentCrawlRunningGen != generation || !c.isContentSearchCurrentLocked(ctx, generation) {
		c.contentSearchStateMu.Unlock()
		cancel()
		c.finishContentCrawl(runningCtx, generation, false)
		return
	}
	c.contentSearchStateMu.Unlock()

	c.api.Log(runningCtx, plugin.LogLevelInfo, "Content index: starting full crawl")
	done := c.engine.StartContentCrawl(runningCtx, roots, fsPolicy, exts, filesearch.ContentDefaultMaxReadBytes, func(progress filesearch.ContentCrawlProgress) {
		if runningCtx.Err() == nil && c.isContentSearchCurrent(runningCtx, generation) {
			c.handleContentCrawlProgress(runningCtx, generation, progress)
		}
	})
	util.Go(runningCtx, "content crawl completion", func() {
		err := <-done
		c.finishContentCrawl(runningCtx, generation, err == nil && runningCtx.Err() == nil)
	})
}

// finishContentCrawl releases the single full-crawl slot and starts either the
// latest pending crawl or the incremental hook for the completed policy.
func (c *FileSearchPlugin) finishContentCrawl(ctx context.Context, generation int64, completed bool) {
	startHook := false
	c.contentSearchStateMu.Lock()
	if c.contentCrawlRunning && c.contentCrawlRunningGen == generation {
		c.contentCrawlRunning = false
		c.contentCrawlRunningGen = 0
		c.contentCrawlCancel = nil
		if c.contentCrawlPending {
			c.resetContentCrawlTimerLocked()
		} else if completed && c.isContentSearchCurrentLocked(ctx, generation) {
			startHook = true
		}
	}
	c.contentSearchStateMu.Unlock()
	if startHook {
		c.startContentHook(ctx, generation)
	}
}

// startContentHook installs the incremental content index hook so file changes
// detected by the scanner's change feed are applied to the content FTS index
// without waiting for the next full crawl. The hook reuses the same extension
// whitelist and read-byte cap as the full crawler.
func (c *FileSearchPlugin) startContentHook(ctx context.Context, generation int64) {
	if c.engine == nil {
		return
	}
	exts := filesearch.ContentExtensionsFromList(filesearch.ContentExtensionListFromSetting(c.api.GetSetting(ctx, contentSearchExtensionsKey)))
	c.runIfContentSearchCurrent(ctx, generation, func() {
		c.engine.StartContentHook(ctx, exts, filesearch.ContentDefaultMaxReadBytes)
	})
}

// waitForFileSearchIdle polls the filesearch engine status until it finishes
// indexing AND search artifact rebuild. The content crawl is deferred until
// both are done so the DB is not locked when content crawl tries to write.
func (c *FileSearchPlugin) waitForFileSearchIdle(ctx context.Context) {
	if c.engine == nil {
		return
	}
	for {
		status, err := c.engine.GetStatus(ctx)
		if err != nil || !status.IsIndexing {
			// Also check if FTS rebuild is still in progress — the DB is locked
			// during rebuild and content crawl writes would fail.
			if !c.engine.NeedsSearchArtifactRebuild() {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// handleContentCrawlProgress shows or clears the content crawl toolbar message.
func (c *FileSearchPlugin) handleContentCrawlProgress(ctx context.Context, generation int64, progress filesearch.ContentCrawlProgress) {
	if progress.Complete {
		c.api.ShowToolbarMsg(ctx, plugin.ToolbarMsg{
			Id:    contentSearchToolbarMsgID,
			Title: fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_content_index_ready"), progress.FilesIndexed),
		})
		time.AfterFunc(5*time.Second, func() {
			if c.isContentSearchCurrent(ctx, generation) {
				c.api.ClearToolbarMsg(ctx, contentSearchToolbarMsgID)
			}
		})
		return
	}

	c.api.ShowToolbarMsg(ctx, plugin.ToolbarMsg{
		Id:            contentSearchToolbarMsgID,
		Title:         fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_content_indexing_progress"), progress.FilesIndexed),
		Indeterminate: true,
	})
}

// onFileSearchPolicyChanged handles roots/ignore/hidden setting changes by
// resetting the content index and queueing a crawl with the new policy.
func (c *FileSearchPlugin) onFileSearchPolicyChanged(ctx context.Context) {
	if !c.isContentSearchEnabled(ctx) {
		return
	}
	generation := c.nextContentSearchGeneration()
	c.stopContentCrawlWork()
	if err := c.engine.ResetContentIndex(ctx); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to reset content search database after file search policy change: %s", err.Error()))
	}
	c.requestContentCrawl(ctx, generation)
}

// onContentPolicyChanged handles extension whitelist changes by resetting the
// content index and queueing a crawl with the new extension set.
func (c *FileSearchPlugin) onContentPolicyChanged(ctx context.Context) {
	if !c.isContentSearchEnabled(ctx) {
		return
	}
	generation := c.nextContentSearchGeneration()
	c.stopContentCrawlWork()
	if err := c.engine.ResetContentIndex(ctx); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to reset content search database after content policy change: %s", err.Error()))
	}
	c.requestContentCrawl(ctx, generation)
}

// getContentSearchRootRecords builds []filesearch.RootRecord from the
// configured file search roots.
func (c *FileSearchPlugin) getContentSearchRootRecords(ctx context.Context) []filesearch.RootRecord {
	paths := c.getEffectiveRootPaths(ctx)
	roots := make([]filesearch.RootRecord, 0, len(paths))
	for i, p := range paths {
		roots = append(roots, filesearch.RootRecord{
			ID:     fmt.Sprintf("content-root-%d", i),
			Path:   p,
			Kind:   filesearch.RootKindDefault,
			Status: filesearch.RootStatusIdle,
		})
	}
	return roots
}

func (c *FileSearchPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	queryStartedAt := util.GetSystemTimestamp()
	diagnostics := fileSearchQueryDiagnostics{}

	if c.engine == nil {
		return plugin.QueryResponse{}
	}

	if c.isStatusQuery(query) {
		return c.queryStatus(ctx)
	}

	if strings.TrimSpace(query.Search) == "" {
		return plugin.QueryResponse{}
	}

	searchStartedAt := util.GetSystemTimestamp()
	usePinyin := setting.GetSettingManager().GetWoxSetting(ctx).UsePinYin.Get()
	// File search uses its own indexed engine instead of plugin.IsStringMatch,
	// so the global pinyin option must be passed explicitly. Without this bridge,
	// disabling pinyin in Wox settings still allowed pinyin-derived candidates
	// such as ASCII "abc..." cache files to appear for mixed Chinese queries.
	selectedType := selectedFileSearchType(query)
	selectedSort := selectedFileSearchSort(query)
	searchLimit := fileSearchResultLimit
	if selectedType != fileSearchTypeRefinementAll || selectedSort != fileSearchSortRefinementRelevance {
		// Feature addition: type filters and non-relevance sorting need a wider
		// candidate window before plugin-side refinement. Keeping the old limit
		// for the default path preserves the fast historical relevance search.
		searchLimit = fileSearchRefinedCandidateLimit
	}
	results, err := c.engine.Search(ctx, filesearch.SearchQuery{Raw: query.Search, DisablePinyin: !usePinyin}, searchLimit)
	diagnostics.searchElapsedMs = util.GetSystemTimestamp() - searchStartedAt
	if err != nil {
		c.logQueryDiagnostics(ctx, query.Search, diagnostics, 0, util.GetSystemTimestamp()-queryStartedAt)
		c.api.Log(ctx, plugin.LogLevelError, err.Error())
		c.api.Notify(ctx, err.Error())
		return plugin.QueryResponse{}
	}
	results = refineFileSearchResults(results, selectedType, selectedSort, fileSearchResultLimit)

	// Split result-materialization timing out from engine search timing because
	// os.Stat/icon setup can make the plugin itself look slow even when the
	// indexed lookup has already finished.
	buildStartedAt := util.GetSystemTimestamp()
	// Cache file-type icons per extension inside one query because the previous
	// per-result file-icon conversion retried embedded-icon extraction for every
	// source file path, which turned an 8ms indexed search into a much slower
	// end-to-end query even though most files only need their shared type icon.
	fileTypeIcons := map[string]common.WoxImage{}
	showPreview := c.getConfiguredShowPreview(ctx)
	queryResults := make([]plugin.QueryResult, 0, len(results))
	for index, item := range results {
		icon := resolveFileSearchResultIcon(ctx, item, fileTypeIcons, &diagnostics)
		actions := c.buildFileSearchResultActions(ctx, item)

		queryResult := plugin.QueryResult{
			Title:    item.Name,
			SubTitle: item.Path,
			Icon:     icon,
			Score:    item.Score,
			Actions:  actions,
			DragData: &plugin.QueryResultDragData{
				Type:  plugin.QueryResultDragDataTypeFiles,
				Files: []string{item.Path},
			},
		}
		if showPreview {
			queryResult.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: item.Path,
			}
		}
		if selectedSort != fileSearchSortRefinementRelevance {
			queryResult.Score = fileSearchRefinementSortScore(index, len(results))
		}
		queryResults = append(queryResults, queryResult)
	}
	diagnostics.buildElapsedMs = util.GetSystemTimestamp() - buildStartedAt

	// Content search: if enabled, search the content index and append results
	// after name/path results, de-duplicated by path (name/path results win).
	contentResults := c.searchContent(ctx, query.Search, queryResults)
	queryResults = append(queryResults, contentResults...)

	c.logQueryDiagnostics(ctx, query.Search, diagnostics, len(queryResults), util.GetSystemTimestamp()-queryStartedAt)

	response := plugin.NewQueryResponse(queryResults)
	response.Refinements = c.buildFileSearchRefinements()
	return response
}

// searchContent queries the content index and returns QueryResults for files
// whose contents match the query, excluding paths already in nameResults.
// Returns nil if content search is disabled or the crawl is still in progress.
func (c *FileSearchPlugin) searchContent(ctx context.Context, queryText string, nameResults []plugin.QueryResult) []plugin.QueryResult {
	if !c.isContentSearchEnabled(ctx) {
		return nil
	}

	contentHits, err := c.engine.SearchContent(ctx, queryText, 20)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("content search failed for query %q: %s", queryText, err.Error()))
		return nil
	}
	if len(contentHits) == 0 {
		return nil
	}

	// Build a set of paths already in nameResults for de-duplication.
	existingPaths := make(map[string]bool, len(nameResults))
	for _, r := range nameResults {
		existingPaths[r.SubTitle] = true // SubTitle is the file path
	}

	fileTypeIcons := map[string]common.WoxImage{}
	showPreview := c.getConfiguredShowPreview(ctx)
	var diagnostics fileSearchQueryDiagnostics // for icon resolution, not logged

	results := make([]plugin.QueryResult, 0, len(contentHits))
	for index, hit := range contentHits {
		if existingPaths[hit.Path] {
			continue
		}

		name := filepath.Base(hit.Path)
		item := filesearch.SearchResult{
			Path: hit.Path,
			Name: name,
		}
		icon := resolveFileSearchResultIcon(ctx, item, fileTypeIcons, &diagnostics)
		actions := c.buildFileSearchResultActions(ctx, item)

		// BM25 and filename matching use different score scales, so preserve
		// content relevance by rank without letting raw BM25 scores dominate.
		queryResult := plugin.QueryResult{
			Title:    name,
			SubTitle: hit.Path,
			Icon:     icon,
			Score:    int64(len(contentHits) - index),
			Actions:  actions,
			DragData: &plugin.QueryResultDragData{
				Type:  plugin.QueryResultDragDataTypeFiles,
				Files: []string{hit.Path},
			},
		}
		if showPreview {
			queryResult.Preview = plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: hit.Path,
			}
		}
		results = append(results, queryResult)
	}

	return results
}

// buildFileSearchResultActions keeps folder navigation integrated with the path-browse plugin.
func (c *FileSearchPlugin) buildFileSearchResultActions(ctx context.Context, item filesearch.SearchResult) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{
		{
			Name: "i18n:plugin_file_open",
			Icon: common.PreviewIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				shell.Open(item.Path)
			},
		},
	}

	if item.IsDir {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_folder_enter",
			Icon:                   common.FolderIcon,
			Hotkey:                 util.PrimaryHotkey("enter"),
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.api.ChangeQuery(ctx, common.PlainQuery{
					QueryType: plugin.QueryTypeInput,
					QueryText: ensureFileSearchFolderBrowseQuery(item.Path),
				})
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Name: "i18n:plugin_file_open_containing_folder",
			Icon: common.OpenContainingFolderIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				shell.OpenFileInFolder(item.Path)
			},
			Hotkey: util.PrimaryHotkey("enter"),
		})
	}
	actions = append(actions, c.buildExecuteCommandAtLocationAction(item))

	actions = append(actions, plugin.QueryResultAction{
		Name: "i18n:plugin_clipboard_delete",
		Icon: common.TrashIcon,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			err := trash.MoveToTrash(item.Path)
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, err.Error())
				c.api.Notify(ctx, err.Error())
				return
			}
		},
	})

	// Bug fix: Linux only has file-manager-specific fallbacks here, not a true
	// native system context menu. Hide the action when the platform cannot
	// deliver the behavior promised by the label instead of showing a no-op.
	if nativecontextmenu.IsSupported() {
		actions = append(actions, plugin.QueryResultAction{
			Name: "i18n:plugin_file_show_context_menu",
			Icon: common.PluginMenusIcon,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				c.api.Log(ctx, plugin.LogLevelInfo, "Showing context menu for: "+item.Path)
				err := nativecontextmenu.ShowContextMenu(item.Path)
				if err != nil {
					c.api.Log(ctx, plugin.LogLevelError, err.Error())
					c.api.Notify(ctx, err.Error())
				}
			},
			Hotkey:                 util.PrimaryHotkey("m"),
			PreventHideAfterAction: true,
		})
	}

	actions = append(actions,
		// Feature addition: manual full reindex belongs in the action panel
		// of file-search results instead of appearing as a separate result.
		// Keeping it off the main result list avoids polluting empty queries
		// and ordinary filename searches while still making recovery easy.
		c.buildIndexFilesAction(),
	)

	return actions
}

// buildExecuteCommandAtLocationAction hands the selected filesystem location to Shell without exposing it in the visible query.
func (c *FileSearchPlugin) buildExecuteCommandAtLocationAction(item filesearch.SearchResult) plugin.QueryResultAction {
	workingDirectory := item.Path
	if !item.IsDir {
		workingDirectory = filepath.Dir(item.Path)
	}

	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_file_execute_command_here",
		Icon:                   common.PluginShellIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			result, err := c.api.InvokePluginCommand(ctx, plugin.PluginCommandRequest{
				PluginId: shellplugin.PluginID,
				Command:  shellplugin.PluginCommandPrepareCommandAtDirectory,
				Data: common.ContextData{
					shellplugin.PluginCommandDataWorkingDirectory: workingDirectory,
				},
			})
			if err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to invoke shell plugin command: %s", err.Error()))
				c.api.Notify(ctx, err.Error())
				return
			}
			if !result.Handled {
				message := result.Message
				if message == "" {
					message = "shell plugin command was not handled"
				}
				c.api.Log(ctx, plugin.LogLevelWarning, message)
				c.api.Notify(ctx, message)
				return
			}
			if result.Message != "" {
				c.api.Notify(ctx, result.Message)
			}
		},
	}
}

func ensureFileSearchFolderBrowseQuery(folderPath string) string {
	if strings.HasSuffix(folderPath, "/") || strings.HasSuffix(folderPath, `\`) {
		return folderPath
	}
	return folderPath + string(os.PathSeparator)
}

func (c *FileSearchPlugin) buildIndexFilesAction() plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:                   "i18n:plugin_file_index_files",
		Icon:                   common.ExecuteRunIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			c.indexFilesFromScratch(ctx)
		},
	}
}

func (c *FileSearchPlugin) indexFilesFromScratch(ctx context.Context) {
	if c.engine == nil {
		return
	}
	generation := c.nextContentSearchGeneration()
	c.stopContentCrawlWork()

	// Use a fresh trace context instead of the action ctx — the action ctx is
	// tied to the query session and may be cancelled before the rebuild
	// finishes, which would abort waitForFileSearchIdle and skip the content
	// crawl restart.
	rebuildCtx := util.NewTraceContext()
	util.Go(rebuildCtx, "filesearch reset index", func() {
		// RebuildIndex deletes the entire filesearch storage directory and opens
		// a fresh filesearch.db. contentsearch.db is recreated lazily only if the
		// content crawl restarts below.
		if err := c.engine.RebuildIndex(rebuildCtx); err != nil {
			c.api.Log(rebuildCtx, plugin.LogLevelError, "Failed to reset file search index: "+err.Error())
			c.api.Notify(rebuildCtx, "i18n:plugin_file_index_files_failed")
			return
		}
		c.syncUserRoots(rebuildCtx)
		c.syncToolbarMsg(rebuildCtx, true)

		// Wait for filesearch to finish indexing before starting content crawl.
		c.waitForFileSearchIdle(rebuildCtx)

		// Restart content crawl if enabled. RebuildIndex removed the old
		// contentsearch.db, so this will open a fresh DB and do a full crawl.
		// The hook is installed after that crawl completes.
		if c.isContentSearchCurrent(rebuildCtx, generation) {
			c.requestContentCrawl(rebuildCtx, generation)
		}
	})
}

func (c *FileSearchPlugin) buildFileSearchRefinements() []plugin.QueryRefinement {
	return []plugin.QueryRefinement{
		c.buildFileSearchTypeRefinement(),
		c.buildFileSearchSortRefinement(),
	}
}

func (c *FileSearchPlugin) buildFileSearchTypeRefinement() plugin.QueryRefinement {
	// Feature addition: type filtering belongs in QueryRefinement instead of
	// command syntax so users can keep typing the same file query while quickly
	// narrowing results to files or folders from the keyboard.
	return plugin.QueryRefinement{
		Id:           fileSearchTypeRefinementKey,
		Title:        "i18n:plugin_file_refinement_type",
		Type:         plugin.QueryRefinementTypeSingleSelect,
		DefaultValue: []string{fileSearchTypeRefinementAll},
		Hotkey:       fileSearchPlatformHotkey("t"),
		Persist:      false,
		Options: []plugin.QueryRefinementOption{
			{Value: fileSearchTypeRefinementAll, Title: "i18n:plugin_file_refinement_type_all"},
			{Value: fileSearchTypeRefinementFile, Title: "i18n:plugin_file_refinement_type_file"},
			{Value: fileSearchTypeRefinementFolder, Title: "i18n:plugin_file_refinement_type_folder"},
		},
	}
}

func (c *FileSearchPlugin) buildFileSearchSortRefinement() plugin.QueryRefinement {
	// Feature addition: sort stays plugin-owned because the indexed engine owns
	// the metadata used for modified-time and size ordering. Relevance remains
	// the default so the existing search ranking is unchanged until selected.
	return plugin.QueryRefinement{
		Id:           fileSearchSortRefinementKey,
		Title:        "i18n:plugin_file_refinement_sort",
		Type:         plugin.QueryRefinementTypeSort,
		DefaultValue: []string{fileSearchSortRefinementRelevance},
		Hotkey:       fileSearchPlatformHotkey("s"),
		Persist:      false,
		Options: []plugin.QueryRefinementOption{
			{Value: fileSearchSortRefinementRelevance, Title: "i18n:plugin_file_refinement_sort_relevance"},
			{Value: fileSearchSortRefinementName, Title: "i18n:plugin_file_refinement_sort_name"},
			{Value: fileSearchSortRefinementModified, Title: "i18n:plugin_file_refinement_sort_modified"},
			{Value: fileSearchSortRefinementSize, Title: "i18n:plugin_file_refinement_sort_size"},
		},
	}
}

func fileSearchPlatformHotkey(key string) string {
	return util.PrimaryHotkey(key)
}

func selectedFileSearchType(query plugin.Query) string {
	switch query.Refinements[fileSearchTypeRefinementKey] {
	case fileSearchTypeRefinementFile, fileSearchTypeRefinementFolder:
		return query.Refinements[fileSearchTypeRefinementKey]
	default:
		return fileSearchTypeRefinementAll
	}
}

func selectedFileSearchSort(query plugin.Query) string {
	switch query.Refinements[fileSearchSortRefinementKey] {
	case fileSearchSortRefinementName, fileSearchSortRefinementModified, fileSearchSortRefinementSize:
		return query.Refinements[fileSearchSortRefinementKey]
	default:
		return fileSearchSortRefinementRelevance
	}
}

func refineFileSearchResults(results []filesearch.SearchResult, selectedType string, selectedSort string, limit int) []filesearch.SearchResult {
	refined := make([]filesearch.SearchResult, 0, len(results))
	for _, result := range results {
		switch selectedType {
		case fileSearchTypeRefinementFile:
			if result.IsDir {
				continue
			}
		case fileSearchTypeRefinementFolder:
			if !result.IsDir {
				continue
			}
		}
		refined = append(refined, result)
	}

	switch selectedSort {
	case fileSearchSortRefinementName:
		sort.SliceStable(refined, func(i, j int) bool {
			leftName := strings.ToLower(refined[i].Name)
			rightName := strings.ToLower(refined[j].Name)
			if leftName == rightName {
				return refined[i].Path < refined[j].Path
			}
			return leftName < rightName
		})
	case fileSearchSortRefinementModified:
		sort.SliceStable(refined, func(i, j int) bool {
			return refined[i].Mtime > refined[j].Mtime
		})
	case fileSearchSortRefinementSize:
		sort.SliceStable(refined, func(i, j int) bool {
			return refined[i].Size > refined[j].Size
		})
	}

	if limit > 0 && len(refined) > limit {
		return append([]filesearch.SearchResult(nil), refined[:limit]...)
	}
	return refined
}

// fileSearchRefinementSortScore keeps explicit sort refinement order stable after manager-side score sorting.
func fileSearchRefinementSortScore(index int, count int) int64 {
	if count <= 0 {
		return 0
	}

	return int64(count-index) * fileSearchRefinementSortScoreStep
}

func resolveFileSearchResultIcon(ctx context.Context, result filesearch.SearchResult, fileTypeIcons map[string]common.WoxImage, diagnostics *fileSearchQueryDiagnostics) common.WoxImage {
	if result.IsDir {
		diagnostics.directoryCount++
		// Feature addition: macOS .app bundles are indexed as directories, so the
		// old directory-first branch always returned the generic folder icon and
		// bypassed the same bundle icon resolver used by the App plugin. Keep this
		// narrow to .app packages so ordinary folders do not pay per-path icon cost.
		if icon := resolveFileSearchMacAppBundleIcon(ctx, result.Path); !icon.IsEmpty() {
			return icon
		}
		return common.FolderIcon
	}

	if shouldUseFileSearchImageThumbnail(result.Path) {
		diagnostics.thumbnailCount++
		// Trust indexed metadata for regular files because the old per-result os.Stat
		// spent several milliseconds confirming directory state that the scanner had
		// already stored. Keep a thumbnail existence check only for image paths so UI
		// does not try to render a deleted file after the index falls briefly behind.
		statStartedAt := util.GetSystemTimestamp()
		_, statErr := os.Stat(result.Path)
		diagnostics.statElapsedMs += util.GetSystemTimestamp() - statStartedAt
		diagnostics.statCount++
		if statErr == nil {
			return common.NewWoxImageAbsolutePath(result.Path)
		}
		diagnostics.statMissCount++
	}

	// Resolve regular files to a cached type icon here because letting manager-side
	// icon conversion inspect every file path forces repeated embedded-icon probes.
	// That fallback work was the main reason logs showed 30ms+ end-to-end latency
	// even when file search itself had already finished within the single-digit budget.
	extension := strings.ToLower(strings.TrimSpace(filepath.Ext(result.Path)))
	if cachedIcon, ok := fileTypeIcons[extension]; ok {
		return cachedIcon
	}

	iconPath, err := fileicon.GetFileTypeIcon(ctx, extension)
	if err == nil && strings.TrimSpace(iconPath) != "" {
		icon := common.NewWoxImageAbsolutePath(iconPath)
		fileTypeIcons[extension] = icon
		return icon
	}

	return common.NewWoxImageFileIcon(result.Path)
}

func resolveFileSearchMacAppBundleIcon(ctx context.Context, directoryPath string) common.WoxImage {
	if !shouldResolveFileSearchMacAppBundleIcon(directoryPath) {
		return common.WoxImage{}
	}

	iconPath, err := fileicon.GetFileIconByPath(ctx, directoryPath)
	if err != nil || strings.TrimSpace(iconPath) == "" {
		return common.WoxImage{}
	}
	return common.NewWoxImageAbsolutePath(iconPath)
}

func shouldResolveFileSearchMacAppBundleIcon(directoryPath string) bool {
	if !util.IsMacOS() {
		return false
	}

	cleanPath := strings.TrimRight(strings.TrimSpace(directoryPath), string(filepath.Separator))
	return strings.EqualFold(filepath.Ext(cleanPath), ".app")
}

func shouldUseFileSearchImageThumbnail(filePath string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filePath))) {
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func (c *FileSearchPlugin) syncUserRoots(ctx context.Context) {
	if c.engine == nil {
		return
	}

	effectiveRoots := c.getEffectiveRootPaths(ctx)
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Syncing file search roots: %d roots", len(effectiveRoots)))
	if err := c.engine.SyncUserRoots(ctx, effectiveRoots); err != nil {
		c.api.Log(ctx, plugin.LogLevelError, "Failed to sync file search roots: "+err.Error())
	}
}

func (c *FileSearchPlugin) getEffectiveRootPaths(ctx context.Context) []string {
	paths := c.getConfiguredRootPaths(ctx)
	// Bug fix: settings can accidentally contain overlapping roots such as the
	// home directory plus a child project directory. The engine also normalizes
	// this boundary, but doing it here keeps plugin logs and status messages in
	// terms of the roots that will actually be indexed.
	return filesearch.NormalizeUserRootPaths(ctx, paths)
}

func (c *FileSearchPlugin) getConfiguredRootPaths(ctx context.Context) []string {
	raw := strings.TrimSpace(c.api.GetSetting(ctx, fileRootsSettingKey))
	if raw == "" {
		return nil
	}

	var roots []fileRootSetting
	if err := json.Unmarshal([]byte(raw), &roots); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Failed to parse file search roots setting: "+err.Error())
		return nil
	}

	paths := make([]string, 0, len(roots))
	for _, root := range roots {
		if path := expandFileSearchRootPath(root.Path); path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}

func (c *FileSearchPlugin) syncIgnorePatterns(ctx context.Context) {
	if c.indexPolicy == nil {
		return
	}

	patterns := c.getConfiguredIgnorePatternValues(ctx)
	c.indexPolicy.SetIgnorePatterns(patterns)
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Syncing file search ignore patterns: %d patterns", len(patterns)))
	if c.engine != nil {
		// Feature addition: ignore rules now come from plugin settings, so changing
		// them must rebuild the index. Updating the shared policy alone would only
		// affect future file-system events and would leave already-indexed ignored
		// paths visible until some unrelated full scan happened.
		c.engine.UpdatePolicy(c.indexPolicy.toFilesearchPolicy())
	}
}

func (c *FileSearchPlugin) syncSkipHiddenFiles(ctx context.Context) {
	if c.indexPolicy == nil {
		return
	}

	enabled := c.getConfiguredSkipHiddenFiles(ctx)
	c.indexPolicy.SetSkipHiddenFiles(enabled)
	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Syncing file search hidden-file policy: skipHiddenFiles=%t", enabled))
	if c.engine != nil {
		// Feature addition: hidden-file behavior is now controlled independently
		// from user glob patterns. Updating it must request a rescan so rows that
		// became included or excluded are reconciled with the new policy.
		c.engine.UpdatePolicy(c.indexPolicy.toFilesearchPolicy())
	}
}

func (c *FileSearchPlugin) getConfiguredIgnorePatternValues(ctx context.Context) []string {
	raw := strings.TrimSpace(c.api.GetSetting(ctx, fileIgnorePatternsSettingKey))
	if raw == "" {
		return appendRequiredFileSearchIgnorePatterns(defaultFileSearchIgnorePatterns)
	}

	var patterns []fileIgnorePatternSetting
	if err := json.Unmarshal([]byte(raw), &patterns); err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Failed to parse file search ignore patterns setting: "+err.Error())
		return appendRequiredFileSearchIgnorePatterns(defaultFileSearchIgnorePatterns)
	}

	values := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if value := strings.TrimSpace(pattern.Pattern); value != "" {
			values = append(values, value)
		}
	}
	return appendRequiredFileSearchIgnorePatterns(values)
}

func appendRequiredFileSearchIgnorePatterns(patterns []string) []string {
	values := append([]string(nil), patterns...)
	required := indexpolicy.WoxFileSearchStorageIgnorePattern
	if containsFileSearchIgnorePattern(values, required) {
		return values
	}

	// Bug fix: existing users can already have a serialized ignorePatterns value
	// from before Wox's own storage was excluded. Keep the internal storage rule
	// mandatory so the scanner cannot index its SQLite DB and self-trigger future
	// FSEvents/USN dirty batches.
	return append(values, required)
}

func containsFileSearchIgnorePattern(patterns []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, pattern := range patterns {
		if strings.EqualFold(strings.TrimSpace(pattern), target) {
			return true
		}
	}
	return false
}

func (c *FileSearchPlugin) getConfiguredSkipHiddenFiles(ctx context.Context) bool {
	raw := strings.TrimSpace(c.api.GetSetting(ctx, fileSkipHiddenFilesSettingKey))
	if raw == "" {
		return true
	}

	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Failed to parse file search skip hidden files setting: "+err.Error())
		return true
	}
	return enabled
}

func (c *FileSearchPlugin) getConfiguredShowPreview(ctx context.Context) bool {
	raw := strings.TrimSpace(c.api.GetSetting(ctx, fileShowPreviewSettingKey))
	if raw == "" {
		return true
	}

	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Failed to parse file search show preview setting: "+err.Error())
		return true
	}
	return enabled
}

func defaultFileSearchRootPathsJSON() string {
	// Feature change: search roots are now fully visible configuration. The old
	// implementation appended hidden Desktop/Documents/Downloads/Pictures roots,
	// which made the table look optional while the engine still indexed paths the
	// user could not see or remove. New installs show the home directory as the
	// default row, and migration backfills the same visible root for existing users.
	if util.IsTestMode() {
		return "[]"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return "[]"
	}

	data, err := json.Marshal([]fileRootSetting{{Path: filepath.Clean(homeDir)}})
	if err != nil {
		return "[]"
	}
	return string(data)
}

func expandFileSearchRootPath(rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(homeDir) == "" {
			return filepath.Clean(path)
		}
		if path == "~" {
			return filepath.Clean(homeDir)
		}
		// Bug fix: users and migrations can store a home-relative root. Expanding it
		// before SyncUserRoots keeps the engine's persisted root identity absolute,
		// so duplicate checks and change-feed paths compare the same representation.
		return filepath.Clean(filepath.Join(homeDir, strings.TrimPrefix(path, "~/")))
	}

	return filepath.Clean(path)
}

func defaultFileSearchIgnorePatternsJSON() string {
	rows := make([]fileIgnorePatternSetting, 0, len(defaultFileSearchIgnorePatterns))
	for _, pattern := range defaultFileSearchIgnorePatterns {
		rows = append(rows, fileIgnorePatternSetting{Pattern: pattern})
	}

	data, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// contentSearchExtensionSetting is the JSON row shape for the
// contentSearchExtensions table setting: [{"Extension":"txt"},...].
type contentSearchExtensionSetting struct {
	Extension string `json:"Extension"`
}

func defaultContentSearchExtensionsJSON() string {
	exts := filesearch.ContentDefaultExtensions()
	rows := make([]contentSearchExtensionSetting, 0, len(exts))
	for _, ext := range exts {
		rows = append(rows, contentSearchExtensionSetting{Extension: ext})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// getConfiguredContentExtensions reads the contentSearchExtensions table
// setting and returns the extension list. Falls back to defaults on parse error.
func (c *FileSearchPlugin) getConfiguredContentExtensions(ctx context.Context) []string {
	return filesearch.ContentExtensionListFromSetting(c.api.GetSetting(ctx, contentSearchExtensionsKey))
}

func (c *FileSearchPlugin) syncToolbarMsg(ctx context.Context, includeReady bool) {
	if c.engine == nil {
		c.api.ClearToolbarMsg(ctx, fileSearchToolbarMsgID)
		return
	}

	status, err := c.engine.GetStatus(ctx)
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelWarning, "Failed to load file search status: "+err.Error())
		return
	}

	c.syncToolbarMsgWithStatus(ctx, status, includeReady)
}

func (c *FileSearchPlugin) syncToolbarMsgWithStatus(ctx context.Context, status filesearch.StatusSnapshot, includeReady bool) {
	completionSummary := isFullIndexCompletionSummary(status)
	toolbarMsg, found := c.buildToolbarMsgFromStatus(ctx, status, includeReady)
	if !found {
		// Bug fix: a full-index completion summary is followed immediately by an
		// idle status snapshot after the scanner clears its transient run state.
		// Keep that summary visible briefly instead of letting the idle snapshot
		// clear it within a single UI frame.
		if c.shouldHoldCompletionToolbar() {
			return
		}
		c.cancelCompletionToolbarHold()
		// Avoid repeating the same clear request on every identical idle snapshot.
		// The previous implementation always cleared and re-sent status updates, which
		// produced long runs of duplicate file-search and UI bridge logs without any
		// visible toolbar change.
		if !c.takeToolbarMsgUpdate("") {
			return
		}
		c.api.ClearToolbarMsg(ctx, fileSearchToolbarMsgID)
		return
	}

	if !completionSummary && !statusHasToolbarError(status) && c.shouldHoldCompletionToolbar() {
		// Bug fix: the completion hold used to protect only against the idle clear
		// emitted after the scanner removed its transient run state. A quick follow-up
		// incremental run could still replace the "Indexed ..." summary in the next
		// frame, so keep non-error progress behind the minimum completion window.
		return
	}

	signature := buildToolbarMsgSignature(toolbarMsg)
	// Only push toolbar updates when the rendered snapshot changes. The status
	// listener can emit many identical preparation snapshots, and forwarding each one
	// forced redundant ShowToolbarMsg round-trips plus duplicate debug logs.
	if !c.takeToolbarMsgUpdate(signature) {
		return
	}

	if completionSummary {
		c.scheduleCompletionToolbarClear(ctx, c.armCompletionToolbarHold())
	} else {
		// Any live progress/error toolbar replaces the completion summary and must
		// invalidate its delayed clear so a new index run is not cleared by an old timer.
		c.cancelCompletionToolbarHold()
	}
	c.logToolbarStatusSnapshot(ctx, status)
	c.api.ShowToolbarMsg(ctx, toolbarMsg)
}

func (c *FileSearchPlugin) buildToolbarMsgFromStatus(ctx context.Context, status filesearch.StatusSnapshot, includeReady bool) (plugin.ToolbarMsg, bool) {
	if isFullIndexCompletionSummary(status) {
		return plugin.ToolbarMsg{
			Id:    fileSearchToolbarMsgID,
			Title: c.buildFullIndexCompletedToolbarTitle(ctx, status),
			Icon:  fileIcon,
		}, true
	}

	if !includeReady && !status.IsIndexing && status.ErrorRootCount == 0 {
		return plugin.ToolbarMsg{}, false
	}

	title := c.api.GetTranslation(ctx, "plugin_file_status_error")
	icon := common.PermissionIcon
	progress := (*int)(nil)
	indeterminate := false
	hasPermissionError := util.IsMacOS() && isFileAccessPermissionError(status.LastError)
	if status.IsIndexing {
		if shouldSuppressShortIncrementalToolbar(status) {
			return plugin.ToolbarMsg{}, false
		}
		// Feature change: keep the Raycast-style compact status, but name the
		// active run kind explicitly. Full runs are user-visible rebuilds, while
		// incremental runs often reconcile dirty watcher events immediately after
		// a rebuild; using the same text made that expected follow-up look like a
		// duplicate full index.
		title = c.buildIndexingToolbarTitle(ctx, status)
		// Bug fix: the compact indexing message still belongs to File Search.
		// Keep the plugin icon visible and let the progress field own only the
		// activity spinner, instead of clearing the icon to remove old phase noise.
		icon = fileIcon
		indeterminate = true
	} else if hasPermissionError {
		title = c.api.GetTranslation(ctx, "plugin_file_status_permission")
	} else if status.ErrorRootCount == 0 {
		return plugin.ToolbarMsg{}, false
	}

	if status.ErrorRootCount > 0 && !status.IsIndexing {
		title = c.decorateRootErrorToolbarTitle(ctx, title, status)
	}

	return plugin.ToolbarMsg{
		Id:            fileSearchToolbarMsgID,
		Title:         title,
		Icon:          icon,
		Progress:      progress,
		Indeterminate: indeterminate,
		Actions:       c.toolbarMsgActions(ctx, hasPermissionError),
	}, true
}

func shouldSuppressShortIncrementalToolbar(status filesearch.StatusSnapshot) bool {
	// UX optimization: most direct-delta incremental runs finish in well under a
	// second. Showing a toolbar message for those fast background reconciles
	// creates visible noise without helping the user, so keep them silent unless
	// the run crosses the minimum display threshold. Error snapshots are never
	// suppressed because they may require user action.
	return status.IsIndexing &&
		status.ActiveRunKind == filesearch.RunKindIncremental &&
		status.ActiveRunElapsedMs < incrementalToolbarMinimumShowMs &&
		status.ErrorRootCount == 0 &&
		status.LastError == ""
}

func isFullIndexCompletionSummary(status filesearch.StatusSnapshot) bool {
	return status.ActiveRunKind == filesearch.RunKindFull &&
		status.ActiveRunStatus == filesearch.RunStatusCompleted &&
		status.ActiveRunElapsedMs > 0
}

func statusHasToolbarError(status filesearch.StatusSnapshot) bool {
	return status.ErrorRootCount > 0 || status.LastError != ""
}

func (c *FileSearchPlugin) buildFullIndexCompletedToolbarTitle(ctx context.Context, status filesearch.StatusSnapshot) string {
	fileCount := status.ActiveRunFileCount
	if fileCount < 0 {
		fileCount = 0
	}
	if fileCount == 0 {
		// Bug fix: the scanner may occasionally be unable to read final counts at
		// the exact completion boundary even though the index is searchable. In that
		// case, omit the count instead of showing a false "0 files" summary.
		return fmt.Sprintf(
			c.api.GetTranslation(ctx, "plugin_file_status_index_complete_no_count"),
			c.formatFileSearchIndexDuration(ctx, status.ActiveRunElapsedMs),
		)
	}

	// Feature addition: full indexing now ends with a Raycast-style summary.
	// The core emits this only after full-run bulk SQLite maintenance finishes,
	// so the count and duration describe the complete persisted index rather
	// than the earlier executor completion snapshot.
	averageRate := estimateCompletedIndexRate(fileCount, status.ActiveRunElapsedMs)
	return fmt.Sprintf(
		c.api.GetTranslation(ctx, "plugin_file_status_index_complete"),
		formatFileSearchCount(fileCount),
		c.formatFileSearchIndexDuration(ctx, status.ActiveRunElapsedMs),
		formatFileSearchCount(averageRate),
	)
}

func (c *FileSearchPlugin) buildIndexingToolbarTitle(ctx context.Context, status filesearch.StatusSnapshot) string {
	indexedFileCount := status.ActiveRunFileCount
	translationKey := "plugin_file_status_indexing_progress"
	switch status.ActiveRunKind {
	case filesearch.RunKindFull:
		if status.ActiveRunStatus == filesearch.RunStatusFinalizing {
			// Feature addition: full indexing now reports the deferred SQLite/FTS
			// save phase explicitly. The previous rate-only title stopped changing
			// once scan jobs finished, which made long bulk finalization look like a
			// stalled or silent toolbar until the completion summary arrived.
			return c.api.GetTranslation(ctx, "plugin_file_status_full_indexing_saving")
		}
		translationKey = "plugin_file_status_full_indexing_progress"
	case filesearch.RunKindIncremental:
		if status.ActiveJobKind == filesearch.JobKindFinalizeRoot && status.ActiveRunStatus == filesearch.RunStatusFinalizing {
			// UX fix: incremental streaming can write many small SQLite batches while
			// it scans, so showing "saving index" for every write makes the toolbar
			// alternate with elapsed progress. Reserve the saving text for the final
			// root finalize job, which is the single end-of-run persistence boundary.
			return c.api.GetTranslation(ctx, "plugin_file_status_incremental_indexing_saving")
		}
		// Bug fix: incremental reconciles can spend most of their time applying a
		// scoped SQLite diff after the scanner has already counted files. Showing
		// a per-second rate during that long apply phase made old snapshots look
		// impossibly fast, so incremental status reports the real elapsed boundary
		// instead of a derived throughput.
		return fmt.Sprintf(
			c.api.GetTranslation(ctx, "plugin_file_status_incremental_indexing_elapsed"),
			formatFileSearchCount(indexedFileCount),
			c.formatFileSearchIndexDuration(ctx, status.ActiveRunElapsedMs),
		)
	}
	indexRate := estimateActiveIndexRate(indexedFileCount, status.ActiveRunElapsedMs)
	// Bug fix: live indexing status should never fall back to internal phase
	// text such as "writing index" or "syncing". A zero value is valid at the
	// beginning of a run. The run-kind-specific key also prevents post-full
	// incremental reconciliation from being rendered as a second identical index;
	// the generic key remains only for legacy/root-only snapshots that do not
	// carry a run kind yet.
	return fmt.Sprintf(
		c.api.GetTranslation(ctx, translationKey),
		formatFileSearchCount(indexedFileCount),
		formatFileSearchCount(indexRate),
	)
}

func formatFileSearchCount(value int64) string {
	if value < 0 {
		value = 0
	}
	text := fmt.Sprintf("%d", value)
	if len(text) <= 3 {
		return text
	}

	var builder strings.Builder
	prefixLen := len(text) % 3
	if prefixLen == 0 {
		prefixLen = 3
	}
	builder.WriteString(text[:prefixLen])
	for index := prefixLen; index < len(text); index += 3 {
		builder.WriteByte(',')
		builder.WriteString(text[index : index+3])
	}
	return builder.String()
}

func estimateActiveIndexRate(indexedFileCount int64, elapsedMs int64) int64 {
	if indexedFileCount <= 0 || elapsedMs <= 0 {
		return 0
	}
	// Bug fix: toolbar signatures include the rendered title. Computing the rate
	// from raw milliseconds changed the title on nearly every status snapshot,
	// which defeated duplicate suppression and spammed ShowToolbarMsg during one
	// run. Whole-second buckets preserve the visible throughput signal without
	// making every millisecond a distinct UI state.
	elapsedSeconds := elapsedMs / 1000
	if elapsedSeconds <= 0 {
		elapsedSeconds = 1
	}
	return indexedFileCount / elapsedSeconds
}

func estimateCompletedIndexRate(indexedFileCount int64, elapsedMs int64) int64 {
	if indexedFileCount <= 0 || elapsedMs <= 0 {
		return 0
	}
	rate := (indexedFileCount*1000 + elapsedMs/2) / elapsedMs
	if rate <= 0 {
		return 1
	}
	return rate
}

func (c *FileSearchPlugin) formatFileSearchIndexDuration(ctx context.Context, elapsedMs int64) string {
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	seconds := (elapsedMs + 999) / 1000
	if seconds <= 0 {
		seconds = 1
	}
	if seconds < 60 {
		return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_status_index_duration_seconds"), seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_status_index_duration_minutes"), minutes, remainingSeconds)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_status_index_duration_hours"), hours, remainingMinutes)
}

func (c *FileSearchPlugin) handleStatusChanged(status filesearch.StatusSnapshot) {
	c.syncToolbarMsgWithStatus(util.NewTraceContext(), status, false)
}

func (c *FileSearchPlugin) takeToolbarMsgUpdate(signature string) bool {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	if c.lastToolbarMsgSignature == signature {
		return false
	}

	c.lastToolbarMsgSignature = signature
	return true
}

func (c *FileSearchPlugin) resetToolbarMsgState() {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	c.lastToolbarMsgSignature = ""
	c.completionHoldUntilMs = 0
	c.completionHoldGeneration++
}

func (c *FileSearchPlugin) shouldHoldCompletionToolbar() bool {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	return c.completionHoldUntilMs > util.GetSystemTimestamp()
}

func (c *FileSearchPlugin) armCompletionToolbarHold() int64 {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	// Bug fix: the scanner reports the completion summary and then clears its
	// transient run state immediately. Store an explicit hold window in the
	// plugin layer because this is display policy, not indexer state.
	c.completionHoldGeneration++
	c.completionHoldUntilMs = util.GetSystemTimestamp() + fullIndexCompletionToolbarHoldMs
	return c.completionHoldGeneration
}

func (c *FileSearchPlugin) cancelCompletionToolbarHold() {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	if c.completionHoldUntilMs == 0 {
		return
	}
	c.completionHoldUntilMs = 0
	c.completionHoldGeneration++
}

func (c *FileSearchPlugin) clearCompletionToolbarIfCurrent(ctx context.Context, generation int64) {
	c.toolbarMsgStateMu.Lock()
	defer c.toolbarMsgStateMu.Unlock()

	if c.completionHoldGeneration != generation || c.completionHoldUntilMs == 0 || c.completionHoldUntilMs > util.GetSystemTimestamp() {
		return
	}
	// Keep the generation check and clear request under the same lock. Otherwise
	// a new index run could publish progress with the same toolbar id in the tiny
	// gap between validating the old timer and sending ClearToolbarMsg.
	c.completionHoldUntilMs = 0
	c.completionHoldGeneration++
	c.lastToolbarMsgSignature = ""
	c.api.ClearToolbarMsg(ctx, fileSearchToolbarMsgID)
}

func (c *FileSearchPlugin) scheduleCompletionToolbarClear(ctx context.Context, generation int64) {
	clearCtx := util.NewTraceContext()
	util.Go(clearCtx, "filesearch completion toolbar clear", func() {
		time.Sleep(time.Duration(fullIndexCompletionToolbarHoldMs) * time.Millisecond)
		// The generation check prevents a delayed completion clear from removing
		// toolbar progress for a newer index run that reused the same toolbar id.
		c.clearCompletionToolbarIfCurrent(clearCtx, generation)
	})
}

func (c *FileSearchPlugin) logToolbarStatusSnapshot(ctx context.Context, status filesearch.StatusSnapshot) {
	// c.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf(
	// 	"File search status: roots=%d preparing=%d scanning=%d syncing=%d writing=%d finalizing=%d errors=%d active=%s run=%s stage=%s progress=%d/%d run_progress=%d/%d root=%d/%d dirs=%d/%d items=%d/%d pending=%d/%d discovered=%d initial=%v",
	// 	status.RootCount,
	// 	status.PreparingRootCount,
	// 	status.ScanningRootCount,
	// 	status.SyncingRootCount,
	// 	status.WritingRootCount,
	// 	status.FinalizingRootCount,
	// 	status.ErrorRootCount,
	// 	status.ActiveRootStatus,
	// 	status.ActiveRunStatus,
	// 	status.ActiveStage,
	// 	status.ActiveProgressCurrent,
	// 	status.ActiveProgressTotal,
	// 	status.RunProgressCurrent,
	// 	status.RunProgressTotal,
	// 	status.ActiveRootIndex,
	// 	status.ActiveRootTotal,
	// 	status.ActiveDirectoryIndex,
	// 	status.ActiveDirectoryTotal,
	// 	status.ActiveItemCurrent,
	// 	status.ActiveItemTotal,
	// 	status.PendingDirtyRootCount,
	// 	status.PendingDirtyPathCount,
	// 	status.ActiveDiscoveredCount,
	// 	status.IsInitialIndexing,
	// ))
}

func buildToolbarMsgSignature(msg plugin.ToolbarMsg) string {
	progress := "nil"
	if msg.Progress != nil {
		progress = fmt.Sprintf("%d", *msg.Progress)
	}

	actionParts := make([]string, 0, len(msg.Actions))
	for _, action := range msg.Actions {
		actionParts = append(actionParts, strings.Join([]string{
			action.Id,
			action.Name,
			action.Icon.String(),
			action.Hotkey,
			fmt.Sprintf("%t", action.IsDefault),
			fmt.Sprintf("%t", action.PreventHideAfterAction),
		}, "|"))
	}

	return strings.Join([]string{
		msg.Id,
		msg.Title,
		msg.Icon.String(),
		progress,
		fmt.Sprintf("%t", msg.Indeterminate),
		strings.Join(actionParts, "||"),
	}, ":::")
}

func (c *FileSearchPlugin) logQueryDiagnostics(ctx context.Context, rawQuery string, diagnostics fileSearchQueryDiagnostics, resultCount int, totalElapsedMs int64) {
	msg := fmt.Sprintf(
		"file_search query diagnostics: query=%q total=%dms toolbar=%dms search=%dms build=%dms stat=%dms stat_calls=%d stat_miss=%d results=%d dirs=%d thumbnails=%d",
		rawQuery,
		totalElapsedMs,
		diagnostics.toolbarElapsedMs,
		diagnostics.searchElapsedMs,
		diagnostics.buildElapsedMs,
		diagnostics.statElapsedMs,
		diagnostics.statCount,
		diagnostics.statMissCount,
		resultCount,
		diagnostics.directoryCount,
		diagnostics.thumbnailCount,
	)

	if totalElapsedMs >= slowFileSearchQueryThresholdMs ||
		diagnostics.searchElapsedMs >= slowFileSearchStageThresholdMs ||
		diagnostics.buildElapsedMs >= slowFileSearchStageThresholdMs ||
		diagnostics.statElapsedMs >= slowFileSearchStageThresholdMs {
		c.api.Log(ctx, plugin.LogLevelInfo, "slow "+msg)
		return
	}

	c.api.Log(ctx, plugin.LogLevelDebug, msg)
}

func (c *FileSearchPlugin) buildPreparingToolbarTitle(ctx context.Context, status filesearch.StatusSnapshot) string {
	if status.ActiveDiscoveredCount <= 0 {
		return c.api.GetTranslation(ctx, "plugin_file_status_preparing")
	}

	return fmt.Sprintf(
		c.api.GetTranslation(ctx, "plugin_file_status_preparing_progress"),
		status.ActiveDiscoveredCount,
	)
}

func (c *FileSearchPlugin) buildScanningToolbarTitle(ctx context.Context, status filesearch.StatusSnapshot) string {
	if status.ActiveDirectoryTotal <= 0 || status.ActiveItemTotal <= 0 {
		return c.api.GetTranslation(ctx, "plugin_file_status_indexing")
	}

	return fmt.Sprintf(
		c.api.GetTranslation(ctx, "plugin_file_status_scanning_progress"),
		status.ActiveDirectoryIndex,
		status.ActiveDirectoryTotal,
		status.ActiveItemCurrent,
		status.ActiveItemTotal,
	)
}

func (c *FileSearchPlugin) buildSyncingToolbarTitle(ctx context.Context, status filesearch.StatusSnapshot) string {
	if status.PendingDirtyRootCount <= 0 && status.PendingDirtyPathCount <= 0 {
		return c.api.GetTranslation(ctx, "plugin_file_status_syncing")
	}

	return fmt.Sprintf(
		c.api.GetTranslation(ctx, "plugin_file_status_syncing_progress"),
		status.PendingDirtyRootCount,
		status.PendingDirtyPathCount,
	)
}

func resolveToolbarProgressPercent(current int64, total int64) (int, bool) {
	if total <= 0 {
		return 0, false
	}

	progressValue := int((current * 100) / total)
	if progressValue < 0 {
		progressValue = 0
	}
	if progressValue > 100 {
		progressValue = 100
	}

	return progressValue, true
}

func decorateRunToolbarTitle(title string, status filesearch.StatusSnapshot) string {
	activity := buildRunActivityLabel(status)
	if strings.TrimSpace(activity) == "" {
		return title
	}
	return title + " · " + activity
}

func buildRunActivityLabel(status filesearch.StatusSnapshot) string {
	scopePath := strings.TrimSpace(status.ActiveScopePath)
	if scopePath == "" {
		scopePath = strings.TrimSpace(status.ActiveRootPath)
	}
	return shortenToolbarPath(scopePath, toolbarActivityPathMaxChars)
}

func normalizeToolbarPath(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	for strings.Contains(normalized, `\\`) {
		normalized = strings.ReplaceAll(normalized, `\\`, `\`)
	}
	return strings.TrimRight(normalized, `\`)
}

func shortenToolbarPath(value string, maxChars int) string {
	normalized := normalizeToolbarPath(value)
	if normalized == "" || maxChars <= 0 || len(normalized) <= maxChars {
		return normalized
	}

	rootPrefix, segments := splitToolbarPath(normalized)
	if len(segments) == 0 {
		return normalized
	}
	if len(segments) == 1 {
		// The previous single-segment fallback returned only the tail with a
		// leading ellipsis, which hid the path head entirely. Keeping both ends
		// visible makes long file names and deep folder hints easier to
		// distinguish in the launcher toolbar.
		return trimToolbarMiddle(normalized, maxChars)
	}

	first := segments[0]
	last := segments[len(segments)-1]
	if candidate := joinToolbarPath(rootPrefix, []string{first, "...", last}); len(candidate) <= maxChars {
		return candidate
	}
	if candidate := joinToolbarPath(rootPrefix, []string{"...", last}); len(candidate) <= maxChars {
		return candidate
	}
	// The previous final fallback still produced `...\\tail`, which made
	// multiple active roots look identical whenever they shared the same
	// suffix. Center truncation keeps the drive/root and trailing segment at
	// the same time, matching the toolbar expectation for scan progress paths.
	return trimToolbarMiddle(normalized, maxChars)
}

func splitToolbarPath(normalized string) (string, []string) {
	if normalized == "" {
		return "", nil
	}

	rootPrefix := ""
	remainder := normalized
	if len(normalized) >= 3 && normalized[1] == ':' && normalized[2] == '\\' {
		rootPrefix = normalized[:3]
		remainder = normalized[3:]
	} else if strings.HasPrefix(normalized, `\`) {
		rootPrefix = `\`
		remainder = strings.TrimLeft(normalized, `\`)
	}

	rawSegments := strings.Split(remainder, `\`)
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		segments = append(segments, segment)
	}
	return rootPrefix, segments
}

func joinToolbarPath(rootPrefix string, segments []string) string {
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		filtered = append(filtered, segment)
	}
	if len(filtered) == 0 {
		return strings.TrimRight(rootPrefix, `\`)
	}
	if rootPrefix == "" {
		return strings.Join(filtered, `\`)
	}
	return strings.TrimRight(rootPrefix, `\`) + `\` + strings.Join(filtered, `\`)
}

func trimToolbarMiddle(value string, maxChars int) string {
	if maxChars <= 0 || len(value) <= maxChars {
		return value
	}
	return util.EllipsisMiddle(value, maxChars)
}

func (c *FileSearchPlugin) decorateRootErrorToolbarTitle(ctx context.Context, title string, status filesearch.StatusSnapshot) string {
	parts := make([]string, 0, 2)
	errorRootPath := shortenToolbarPath(status.ErrorRootPath, toolbarActivityPathMaxChars)
	if errorRootPath != "" {
		parts = append(parts, errorRootPath)
	}

	errorReason := c.buildRootErrorToolbarReason(ctx, status.LastError)
	if errorReason != "" {
		parts = append(parts, errorReason)
	}

	if len(parts) == 0 {
		return title
	}
	// A generic "needs attention" banner is too vague when one configured root
	// fails. Keep the root and a short cause visible without turning the toolbar
	// into the full diagnostic surface.
	return title + " · " + strings.Join(parts, " · ")
}

// buildRootErrorToolbarReason condenses persisted scanner errors into a short
// cause that can fit beside the failing root in the launcher toolbar.
func (c *FileSearchPlugin) buildRootErrorToolbarReason(ctx context.Context, message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	normalized := strings.ToLower(message)
	if strings.Contains(normalized, "access is denied") || strings.Contains(normalized, "permission denied") || strings.Contains(normalized, "operation not permitted") {
		return c.getToolbarTranslation(ctx, "plugin_file_status_error_reason_access_denied", "Access denied")
	}

	return trimToolbarMiddle(lastToolbarErrorSegment(message), toolbarErrorReasonMaxChars)
}

// getToolbarTranslation keeps toolbar helpers usable in tests that do not wire
// the full plugin translation service.
func (c *FileSearchPlugin) getToolbarTranslation(ctx context.Context, key string, fallback string) string {
	if c == nil || c.api == nil {
		return fallback
	}

	translation := strings.TrimSpace(c.api.GetTranslation(ctx, key))
	if translation == "" || translation == key {
		return fallback
	}
	return translation
}

// lastToolbarErrorSegment keeps generic error fallbacks compact while retaining
// the usually actionable suffix from wrapped Go errors.
func lastToolbarErrorSegment(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	parts := strings.Split(message, ":")
	for index := len(parts) - 1; index >= 0; index-- {
		part := strings.TrimSpace(parts[index])
		if part != "" {
			return part
		}
	}
	return message
}

func (c *FileSearchPlugin) toolbarMsgActions(ctx context.Context, hasPermissionError bool) []plugin.ToolbarMsgAction {
	if !hasPermissionError || !util.IsMacOS() {
		return nil
	}

	return []plugin.ToolbarMsgAction{
		{
			Name:   "i18n:plugin_file_status_open_privacy_settings",
			Icon:   common.PermissionIcon,
			Hotkey: util.PrimaryHotkey("enter"),
			Action: func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
				permission.OpenPrivacySecuritySettings(ctx)
			},
		},
	}
}

func isFileAccessPermissionError(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}

	return strings.Contains(message, "operation not permitted") || strings.Contains(message, "permission denied")
}
