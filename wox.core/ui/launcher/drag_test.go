package launcher

import (
	"testing"

	"wox/plugin"
)

func TestFromCoreQueryResultCopiesFileDragData(t *testing.T) {
	result := fromCoreQueryResult(plugin.QueryResultUI{
		Id:    "file-result",
		Title: "Report",
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: []string{"C:\\Reports\\report.docx"},
		},
	})

	if !result.DragData.isFiles() || len(result.DragData.Files) != 1 || result.DragData.Files[0] != "C:\\Reports\\report.docx" {
		t.Fatalf("converted drag data = %#v", result.DragData)
	}
}

func TestQueryResultDragDataRejectsUnsupportedPayloads(t *testing.T) {
	for _, data := range []*queryResultDragData{
		nil,
		{Type: "text", Files: []string{"C:\\Reports\\report.docx"}},
		{Type: "files"},
	} {
		if data.isFiles() {
			t.Fatalf("drag data %#v should not be draggable", data)
		}
	}
}
