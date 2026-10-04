package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"wox/common"
	woxui "wox/ui/runtime"
)

func TestFilePreviewForUsesLoadingKindBeforeContentArrives(t *testing.T) {
	app := &App{}
	content := app.filePreviewFor(filepath.Join(t.TempDir(), "notes.txt"))
	if content.Kind != "loading" {
		t.Fatalf("uncached file preview kind = %q, want loading", content.Kind)
	}
}

func TestFilePreviewForImageIncludesSizeTag(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "preview.jpeg")
	payload := bytes.Repeat([]byte{0x11}, 4096)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	content := (&App{}).filePreviewFor(filePath)
	wantSize := formatFileSize(int64(len(payload)))
	if content.Kind != "image" || len(content.Tags) != 2 || content.Tags[0].Label != "JPEG" || content.Tags[1].Label != wantSize {
		t.Fatalf("image preview = %#v, want JPEG and %s", content, wantSize)
	}
}

// TestFilePreviewForSVGUsesImagePipeline guards against treating SVG markup as source code.
func TestFilePreviewForSVGUsesImagePipeline(t *testing.T) {
	for _, extension := range []string{".svg", ".SVG"} {
		t.Run(extension, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "preview"+extension)
			payload := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24" fill="#ff0000"/></svg>`)
			if err := os.WriteFile(filePath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			app := &App{}
			app.prepareFilePreview(filePath)
			defer app.cancelScheduledFilePreview()
			content := app.filePreviewFor(filePath)
			if content.Kind != "image" || content.Image.ImageType != "absolute" || content.Image.ImageData != filePath {
				t.Fatalf("SVG preview = %#v, want image file", content)
			}
			if app.filePreviewTimer != nil || app.fileRequests[filePath] {
				t.Fatal("SVG preview scheduled a text file read")
			}
			decoded, err := decodeWoxImage(content.Image)
			if err != nil {
				t.Fatalf("decode SVG preview: %v", err)
			}
			if decoded.Width <= 0 || decoded.Height <= 0 {
				t.Fatal("SVG preview decoded an empty image")
			}
		})
	}
}

func TestInspectImagePreviewAddsTypeAndSizeTags(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "theme.jpg")
	payload := bytes.Repeat([]byte{0xFF}, 2048)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	content := inspectImagePreview(filePath, ".jpg")
	if content.Kind != "image" || content.Image.ImageData != filePath {
		t.Fatalf("image preview = %#v, want the still-image path", content)
	}
	wantSize := formatFileSize(int64(len(payload)))
	if len(content.Tags) != 2 || content.Tags[0].Label != "JPG" || content.Tags[1].Label != wantSize {
		t.Fatalf("image tags = %#v, want JPG and %s", content.Tags, wantSize)
	}
}

func TestInspectPreviewFileUsesWebViewForMP4(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("failed to write test media: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45678)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".mp4", false)
	if content.Kind != "webview" {
		t.Fatalf("MP4 preview kind = %q, want webview", content.Kind)
	}

	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode video WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("video WebView cache should be disabled")
	}
	if !strings.Contains(data.HTML, "<video controls") {
		t.Fatal("video preview HTML does not contain controls")
	}
	if !strings.Contains(data.HTML, "127.0.0.1:45678/preview/file/media") {
		t.Fatal("video preview HTML does not use the loopback media endpoint")
	}
}

func TestInspectPreviewFileKeepsMissingMP4AsError(t *testing.T) {
	content := inspectPreviewFile(filepath.Join(t.TempDir(), "missing.mp4"), ".mp4", false)
	if content.Kind != "error" {
		t.Fatalf("missing MP4 preview kind = %q, want error", content.Kind)
	}
}

func TestInspectPreviewFileUsesWebViewForPDF(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(filePath, []byte("%PDF-1.7"), 0o600); err != nil {
		t.Fatalf("failed to write test PDF: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45679)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".pdf", false)
	if content.Kind != "webview" {
		t.Fatalf("PDF preview kind = %q, want webview", content.Kind)
	}
	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode PDF WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("PDF WebView cache should be disabled")
	}
	if !strings.Contains(data.URL, "127.0.0.1:45679/preview/file/media") {
		t.Fatalf("PDF preview URL does not use the loopback media endpoint: %q", data.URL)
	}
}

func TestInspectPreviewFileUsesWebViewForAudio(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("failed to write test audio: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45680)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".mp3", false)
	if content.Kind != "webview" {
		t.Fatalf("audio preview kind = %q, want webview", content.Kind)
	}
	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode audio WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("audio WebView cache should be disabled")
	}
	if !strings.Contains(data.HTML, "<audio controls") {
		t.Fatal("audio preview HTML does not contain controls")
	}
	if !strings.Contains(data.HTML, "127.0.0.1:45680/preview/file/media") {
		t.Fatal("audio preview HTML does not use the loopback media endpoint")
	}
}

func TestInspectPreviewFileUsesNativeHandlerForOffice(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "document.docx")
	if err := os.WriteFile(filePath, []byte("office"), 0o600); err != nil {
		t.Fatalf("failed to write test Office file: %v", err)
	}

	content := inspectPreviewFile(filePath, ".docx", false)
	if content.Kind != "native_file" {
		t.Fatalf("Office preview kind = %q, want native_file", content.Kind)
	}
	if content.NativeFilePath != filePath {
		t.Fatalf("native Office preview path = %q, want %q", content.NativeFilePath, filePath)
	}
	if !content.NativeFileAutoLoad {
		t.Fatal("small Office preview should start automatically after the deferred delay")
	}
}

func TestInspectPreviewFileDefersLargeOfficeHandlerUntilRequested(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large-document.docx")
	if err := os.WriteFile(filePath, nil, 0o600); err != nil {
		t.Fatalf("failed to create large Office file: %v", err)
	}
	if err := os.Truncate(filePath, int64(officePreviewManualLoadBytes+1)); err != nil {
		t.Fatalf("failed to size large Office file: %v", err)
	}

	content := inspectPreviewFile(filePath, ".docx", false)
	if content.Kind != "large" {
		t.Fatalf("large Office preview kind = %q, want large", content.Kind)
	}
	if content.Text != "" {
		t.Fatal("deferred Office preview should not read document bytes")
	}

	loaded := inspectPreviewFile(filePath, ".docx", true)
	if loaded.Kind != "native_file" || !loaded.NativeFileAutoLoad {
		t.Fatalf("requested Office preview = kind %q autoLoad %v, want native_file auto-load", loaded.Kind, loaded.NativeFileAutoLoad)
	}
}

func TestInspectPreviewFileAutomaticallyBoundsLargeText(t *testing.T) {
	for _, extension := range []string{".js", ".txt", ".md"} {
		t.Run(extension, func(t *testing.T) {
			filePath := writeSizedPreviewFile(t, "large"+extension, 6*1024*1024, 'a')
			content := inspectPreviewFile(filePath, extension, false)
			want := map[string]string{".js": "code", ".txt": "text", ".md": "markdown"}[extension]
			if content.Kind != want || !strings.Contains(content.Text, "aaa") {
				t.Fatalf("automatic preview = kind %q, want %q with contents", content.Kind, want)
			}
			if !content.Limited || len(content.Text) > maxFilePreviewDisplayBytes+maxFilePreviewDisplayLines {
				t.Fatalf("preview exceeds budget: limited=%v bytes=%d", content.Limited, len(content.Text))
			}
		})
	}
}

func TestInspectPreviewCodeKeepsSourceAndFallsBackForPlainText(t *testing.T) {
	for _, name := range []string{"main.go", "main.py", "main.js", "data.json", "notes.txt", "file.unknown"} {
		t.Run(name, func(t *testing.T) {
			const source = "// 中文\r\n\r\npackage main\r\n"
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			content := inspectPreviewFile(path, filepath.Ext(path), false)
			want := "code"
			if name == "notes.txt" || name == "file.unknown" {
				want = "text"
			}
			if content.Kind != want || content.Text != strings.ReplaceAll(source, "\r\n", "\n") || content.Limited {
				t.Fatalf("unexpected file preview: %#v", content)
			}
			if want == "code" && len(content.CodeTokens) == 0 {
				t.Fatal("recognized code should have cached syntax tokens")
			}
		})
	}
}

func TestBoundFilePreviewCodeKeepsOriginalLineNumbers(t *testing.T) {
	for _, test := range []struct {
		source, want string
		lines        int
		limited      bool
	}{
		{"", "", 0, false},
		{"one\r\n\r\ntwo\n", "one\n\ntwo\n", 4, false},
		{strings.Repeat("x\n", 201), strings.TrimSuffix(strings.Repeat("x\n", 200), "\n"), 200, true},
		{"first\n" + strings.Repeat("界", 241) + "\nlast", "first\n" + strings.Repeat("界", 240), 2, true},
	} {
		value, lines, limited := boundFilePreviewCode(test.source)
		if value != test.want || lines != test.lines || limited != test.limited {
			t.Fatalf("bounded code = %q, %d, %v; want %q, %d, %v", value, lines, limited, test.want, test.lines, test.limited)
		}
	}
}

func TestBoundFilePreviewTextHardWrapsMinifiedLine(t *testing.T) {
	text, lines, limited := boundFilePreviewText(strings.Repeat("x", maxFilePreviewLineRunes*3+10))
	if limited {
		t.Fatal("wrapping a short minified line within the display budget should not report a limit")
	}
	if lines != 4 {
		t.Fatalf("wrapped lines = %d, want 4", lines)
	}
	for _, line := range strings.Split(text, "\n") {
		if len([]rune(line)) > maxFilePreviewLineRunes {
			t.Fatalf("wrapped line has %d runes, want at most %d", len([]rune(line)), maxFilePreviewLineRunes)
		}
	}
}

func TestBoundFilePreviewTextStopsAtLineBudget(t *testing.T) {
	source := strings.Repeat("line\n", maxFilePreviewDisplayLines+25)
	text, lines, limited := boundFilePreviewText(source)
	if !limited || lines != maxFilePreviewDisplayLines {
		t.Fatalf("line budget = limited %v lines %d, want limited at %d", limited, lines, maxFilePreviewDisplayLines)
	}
	if strings.Count(text, "\n")+1 != maxFilePreviewDisplayLines {
		t.Fatalf("bounded text has %d lines, want %d", strings.Count(text, "\n")+1, maxFilePreviewDisplayLines)
	}
}

func TestInspectPreviewFileStillAutoloadsSmallText(t *testing.T) {
	filePath := writeSizedPreviewFile(t, "notes.txt", 32, 'b')
	content := inspectPreviewFile(filePath, ".txt", false)
	if content.Kind != "text" || !strings.Contains(content.Text, "bbb") {
		t.Fatalf("small text preview = kind %q text %q, want autoloaded text", content.Kind, content.Text)
	}
}

func TestInspectPreviewFileDefersLargePDFUntilRequested(t *testing.T) {
	filePath := writeSizedPreviewFile(t, "manual.pdf", officePreviewManualLoadBytes+1, '%')
	content := inspectPreviewFile(filePath, ".pdf", false)
	if content.Kind != "large" {
		t.Fatalf("large PDF preview kind = %q, want large", content.Kind)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45681)
	defer common.SetServerPort(previousPort)

	loaded := inspectPreviewFile(filePath, ".pdf", true)
	if loaded.Kind != "webview" {
		t.Fatalf("requested PDF preview kind = %q, want webview", loaded.Kind)
	}
}

func TestInspectPreviewFileRejectsHugeOfficePreview(t *testing.T) {
	filePath := writeSizedPreviewFile(t, "huge.docx", 0, 0)
	if err := os.Truncate(filePath, int64(tooLargePreviewBytes+1)); err != nil {
		t.Fatalf("failed to size huge Office file: %v", err)
	}
	content := inspectPreviewFile(filePath, ".docx", true)
	if content.Kind != "too_large" {
		t.Fatalf("huge Office preview kind = %q, want too_large", content.Kind)
	}
}

func TestPrepareFilePreviewDelaysSmallTextUntilSelectionSettles(t *testing.T) {
	app := &App{filePreviews: map[string]filePreviewContent{}, fileRequests: map[string]bool{}, filePreviewManualPaths: map[string]bool{}}
	filePath := writeSizedPreviewFile(t, "notes.js", 32, 'd')
	app.prepareFilePreview(filePath)
	if app.fileRequests[filePath] {
		t.Fatal("small file preview should wait for the Flutter-style settle delay")
	}
	if app.filePreviewPendingPath != filePath || app.filePreviewTimer == nil {
		t.Fatal("small file preview should schedule one delayed load")
	}
	app.cancelScheduledFilePreview()
}

func TestPrepareFilePreviewCancelsObsoleteDelayedLoad(t *testing.T) {
	app := &App{filePreviews: map[string]filePreviewContent{}, fileRequests: map[string]bool{}, filePreviewManualPaths: map[string]bool{}}
	first := writeSizedPreviewFile(t, "first.js", 32, 'e')
	second := writeSizedPreviewFile(t, "second.js", 32, 'f')
	app.prepareFilePreview(first)
	generation := app.filePreviewLoadGeneration
	app.prepareFilePreview(second)
	if app.filePreviewPendingPath != second {
		t.Fatalf("pending preview = %q, want the latest selection", app.filePreviewPendingPath)
	}
	app.startScheduledFilePreview(first, ".js", generation)
	if app.fileRequests[first] {
		t.Fatal("obsolete delayed preview started a read")
	}
	app.cancelScheduledFilePreview()
}

func TestStartScheduledFilePreviewLoadsCurrentSelection(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%v", remote), func(t *testing.T) {
			filePath := writeSizedPreviewFile(t, "ready.js", 32, 'g')
			applied := make(chan func(), 1)
			app := &App{
				uiCall:  func(fn func()) error { applied <- fn; return nil },
				visible: true, query: plainQuery{QueryID: "q"}, resultsQueryID: "q", selected: 0,
				results:                   []queryResult{{Preview: queryPreview{PreviewType: "file", PreviewData: filePath}}},
				filePreviews:              map[string]filePreviewContent{},
				fileRequests:              map[string]bool{},
				filePreviewManualPaths:    map[string]bool{},
				filePreviewPendingPath:    filePath,
				filePreviewLoadGeneration: 4,
			}
			if remote {
				app.remotePreviews = map[string]queryPreview{"/preview?id=file": app.results[0].Preview}
				app.results[0].Preview = queryPreview{PreviewType: "remote", PreviewData: "/preview?id=file"}
			}
			app.startScheduledFilePreview(filePath, ".js", 4)
			if !app.fileRequests[filePath] {
				t.Fatal("settled file preview did not start its delayed read")
			}
			select {
			case apply := <-applied:
				apply()
			case <-time.After(5 * time.Second):
				t.Fatal("file inspection did not finish")
			}
			if app.filePreviewFor(filePath).Kind != "code" {
				t.Fatal("selected file was not loaded")
			}
		})
	}
}

func TestPrepareFilePreviewDelaysLargeTextInspection(t *testing.T) {
	app := &App{filePreviews: map[string]filePreviewContent{}, fileRequests: map[string]bool{}, filePreviewManualPaths: map[string]bool{}}
	filePath := writeSizedPreviewFile(t, "vendor.js", 6*1024*1024, 'c')
	app.prepareFilePreview(filePath)
	defer app.cancelScheduledFilePreview()
	content := app.filePreviewFor(filePath)
	if content.Kind != "loading" || app.fileRequests[filePath] || app.filePreviewPendingPath != filePath {
		t.Fatal("large files must wait for background inspection")
	}
	if content.Text != "" {
		t.Fatal("selection scheduling should not read contents during prepare")
	}
}

func TestDeferredFilePreviewUsesBoundedCacheAndRemoteLoadHotkey(t *testing.T) {
	filePath := writeSizedPreviewFile(t, "large.docx", officePreviewManualLoadBytes+1, 'x')
	app := &App{filePreviews: map[string]filePreviewContent{}, fileRequests: map[string]bool{}}
	for i := 0; i < 128; i++ {
		path := fmt.Sprint(i)
		app.filePreviews[path] = filePreviewContent{Kind: "large"}
		app.fileRequests[path] = true
	}
	app.loadFilePreview(filePath, ".docx", false)
	if len(app.filePreviews) != 1 || len(app.fileRequests) != 1 || app.filePreviewFor(filePath).Kind != "large" {
		t.Fatal("deferred previews must use the bounded cache")
	}
	app.visible, app.selected = true, 0
	app.query.QueryID, app.resultsQueryID = "q", "q"
	app.results = []queryResult{{Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=file"}}}
	app.remotePreviews = map[string]queryPreview{"/preview?id=file": {PreviewType: "file", PreviewData: filePath}}
	applied := make(chan func(), 1)
	app.uiCall = func(fn func()) error { applied <- fn; return nil }
	modifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierMeta
	}
	if !app.onFilePreviewKey(woxui.KeyEvent{Key: "l", Modifiers: modifier, Down: true}) {
		t.Fatal("remote file preview did not handle its load hotkey")
	}
	select {
	case apply := <-applied:
		apply()
	case <-time.After(5 * time.Second):
		t.Fatal("manual preview did not finish")
	}
	if content := app.filePreviewFor(filePath); content.Kind != "native_file" || !content.NativeFileAutoLoad {
		t.Fatal("manual preview did not enable the Office preview")
	}
}

func TestInspectPreviewFilePreservesUTF8AndMarkdown(t *testing.T) {
	for _, test := range []struct {
		name, source, kind string
		limited            bool
	}{
		{"unicode.txt", strings.Repeat("中", 6000), "text", true},
		{"invalid.txt", "\xff" + strings.Repeat("中", 6000), "info", false},
		{"link.md", "[link](https://example.com/" + strings.Repeat("a", 300) + ")", "markdown", false},
		{"table.md", "| Title | Value |\n| --- | --- |\n| " + strings.Repeat("a", 300) + " | value |", "markdown", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.name)
			if err := os.WriteFile(path, []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			content := inspectPreviewFile(path, filepath.Ext(path), false)
			if content.Kind != test.kind || content.Limited != test.limited || !utf8.ValidString(content.Text) {
				t.Fatalf("kind=%s limited=%v, want %s limited=%v", content.Kind, content.Limited, test.kind, test.limited)
			}
			if test.kind == "markdown" && content.Text != test.source {
				t.Fatal("Markdown source was changed before parsing")
			}
		})
	}
}

func writeSizedPreviewFile(t *testing.T, name string, size int, fill byte) string {
	t.Helper()
	filePath := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(filePath, bytes.Repeat([]byte{fill}, size), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return filePath
}
