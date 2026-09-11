package filesearch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wox/util"
)

// TestExtractPDFText covers extraction, output limits, and path-bearing diagnostics.
func TestExtractPDFText(t *testing.T) {
	path := writeContentTestPDF(t, "BT (hello world) Tj ET")
	for _, limit := range []int64{0, 5, 256 * 1024} {
		text, err := extractPDFText(path, limit)
		want := "hello world"
		if limit < int64(len(want)) {
			want = want[:limit]
		}
		if err != nil || text != want {
			t.Fatalf("limit=%d: text=%q err=%v, want %q", limit, text, err, want)
		}
	}

	// A broken page reference panics during Page(), before GetPlainText can recover.
	brokenPath := writeContentTestPDF(t, "")
	data, err := os.ReadFile(brokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brokenPath, []byte(strings.Replace(string(data), "3 0 obj", "9 0 obj", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if text, err := extractPDFText(brokenPath, 256*1024); err == nil || text != "" || !strings.Contains(err.Error(), fmt.Sprintf("%q", brokenPath)) {
		t.Fatalf("broken page: text=%q err=%v, want path-bearing error", text, err)
	}
	logData, err := os.ReadFile(util.GetLogger().CurrentLogPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{
		fmt.Sprintf("PDF content extraction started: path=%q", path),
		fmt.Sprintf("PDF content extraction finished: path=%q", path),
		fmt.Sprintf("PDF content extraction failed: path=%q page=1", brokenPath),
	} {
		if !strings.Contains(string(logData), message) {
			t.Errorf("missing diagnostic %q", message)
		}
	}
}

// TestExtractPDFTextTruncatedArray isolates the historical unbounded EOF loop so
// a dependency regression fails promptly instead of hanging the whole suite.
func TestExtractPDFTextTruncatedArray(t *testing.T) {
	if os.Getenv("WOX_TEST_PDF_TRUNCATED_ARRAY") == "1" {
		text, err := extractPDFText(filepath.Join("testdata", "unterminated_array.pdf"), 256*1024)
		// Upstream tolerates a truncated array and preserves preceding text.
		if err != nil || text != "before" {
			t.Fatalf("text=%q err=%v, want preceding text without a hang", text, err)
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExtractPDFTextTruncatedArray$")
	cmd.Env = append(os.Environ(), "WOX_TEST_PDF_TRUNCATED_ARRAY=1", "GOMEMLIMIT=16MiB")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PDF regression subprocess: %v (deadline=%v)\n%s", err, ctx.Err(), output)
	}
}

// writeContentTestPDF creates a small PDF with controllable page content.
func writeContentTestPDF(t *testing.T, content string) string {
	t.Helper()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var data strings.Builder
	data.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = data.Len()
		fmt.Fprintf(&data, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := data.Len()
	fmt.Fprintf(&data, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	path := filepath.Join(t.TempDir(), "document.pdf")
	if err := os.WriteFile(path, []byte(data.String()), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
