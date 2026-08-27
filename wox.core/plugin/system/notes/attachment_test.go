package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/common"
	"wox/util"
)

func TestSanitizeNoteImageIDRejectsPathTraversal(t *testing.T) {
	if got := SanitizeNoteImageID("../secret.png"); got != "secret.png" {
		t.Fatalf("base name = %q", got)
	}
	if SanitizeNoteImageID("..") != "" || SanitizeNoteImageID(".") != "" {
		t.Fatal("dot names must be rejected")
	}
}

func TestImportNoteImageCopiesBytesOutsideNoteJSON(t *testing.T) {
	util.GetLocation().UpdateUserDataDirectory(t.TempDir())
	src := filepath.Join(t.TempDir(), "capture.png")
	writeTestNotePNG(t, src)

	imported, err := ImportNoteImage(src)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported.ID == "" || imported.FileName != "capture.png" || imported.Width != 8 || imported.Height != 6 {
		t.Fatalf("imported = %#v", imported)
	}
	if strings.Contains(imported.ID, string(filepath.Separator)) {
		t.Fatalf("attachment id must be a filename: %q", imported.ID)
	}
	dest := ResolveNoteImagePath(imported)
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("attachment file: %v", err)
	}
	if dest == src {
		t.Fatal("import must copy away from the source screenshot path")
	}
}

func TestHydrateNoteImageDimensionsReadsAttachmentHeader(t *testing.T) {
	util.GetLocation().UpdateUserDataDirectory(t.TempDir())
	imported, err := ImportNoteImage(writeTempNotePNG(t))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	document := common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockImage, Image: &common.NoteImage{ID: imported.ID, FileName: imported.FileName},
	}}}
	HydrateNoteImageDimensions(document)
	if document.Blocks[0].Image.Width != imported.Width || document.Blocks[0].Image.Height != imported.Height {
		t.Fatalf("hydrated = %#v, want %dx%d", document.Blocks[0].Image, imported.Width, imported.Height)
	}
}

func TestParseNoteImageRefAcceptsPortableRefs(t *testing.T) {
	if got := ParseNoteImageRef("notes-image:abc.png"); got != "abc.png" {
		t.Fatalf("notes-image ref = %q", got)
	}
	if got := ParseNoteImageRef("attachments/abc.png"); got != "abc.png" {
		t.Fatalf("attachments ref = %q", got)
	}
	if ParseNoteImageRef("https://example.com/a.png") != "" {
		t.Fatal("remote images must not become attachments")
	}
}

func TestAdjustNoteImageScaleKeepsPercentBounds(t *testing.T) {
	if got := AdjustNoteImageScale(0, -10); got != 90 {
		t.Fatalf("from full width = %d, want 90", got)
	}
	if got := AdjustNoteImageScale(90, 10); got != 0 {
		t.Fatalf("back to full width = %d, want 0", got)
	}
	if got := AdjustNoteImageScale(20, -10); got != 20 {
		t.Fatalf("minimum scale = %d, want 20", got)
	}
}
