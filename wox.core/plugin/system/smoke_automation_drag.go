//go:build wox_automation

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
)

// querySmokeDrag supplies real files and hidden previews through the ordinary plugin result pipeline.
func querySmokeDrag(query plugin.Query) plugin.QueryResponse {
	mode, path, _ := strings.Cut(query.Search, " ")
	count := 20
	title := "Drag source"
	if query.Type == plugin.QueryTypeSelection && len(query.Selection.FilePaths) > 0 {
		path = query.Selection.FilePaths[0]
		mode = "keep"
		count = 1
		title = "Dropped file"
	}
	results := make([]plugin.QueryResult, count)
	for i := range results {
		results[i] = plugin.QueryResult{Id: fmt.Sprintf("drag-file-%d", i), Title: title,
			Icon:     icons.Get(icons.PluginApp),
			DragData: &plugin.QueryResultDragData{Type: "files", Files: []string{path}, PreventHideAfterDrag: mode == "keep"},
			Preview:  plugin.WoxPreview{PreviewType: "text", PreviewData: path},
		}
	}
	return plugin.QueryResponse{Results: results, Layout: plugin.QueryLayout{GridLayout: &plugin.MetadataFeatureParamsGridLayout{Columns: 5, ShowTitle: true}}}
}

// recordSmokeDrag records the terminal callback alongside the test-owned source file.
func (p *smokeAutomationPlugin) recordSmokeDrag(ctx context.Context, event plugin.DragOutEvent) {
	if len(event.Files) != 1 {
		return
	}
	data, err := json.Marshal(struct {
		plugin.DragOutEvent
		SessionID string
		QueryID   string
	}{event, util.GetContextSessionId(ctx), util.GetContextQueryId(ctx)})
	if err == nil {
		err = os.WriteFile(event.Files[0]+".event.json", data, 0600)
	}
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("write drag smoke evidence: %v", err))
	}
}
