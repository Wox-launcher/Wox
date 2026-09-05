package common

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/util"
)

func TestChatAttachmentsKeepFilePathsAndSnapshotImages(t *testing.T) {
	directory := t.TempDir()
	previous := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(directory)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(previous) })
	filePath := filepath.Join(directory, "report.bin")
	if err := os.WriteFile(filePath, []byte("private file content\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := ImportChatAttachment(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if file.Kind != AIChatAttachmentFile || file.URL != filePath || file.Text != "" {
		t.Fatalf("ordinary file must be a path-only reference: %+v", file)
	}
	message := ChatMessageText("Explain", []AIChatAttachment{file})
	if !strings.Contains(message, filePath) || strings.Contains(message, "private file content") {
		t.Fatalf("file request = %q", message)
	}
	imagePath := filepath.Join(directory, "photo.png")
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 3)))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("write image: %v, %v", err, closeErr)
	}
	attachment, err := ImportChatAttachment(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Kind != AIChatAttachmentImage || attachment.MimeType != "image/png" {
		t.Fatalf("image attachment = %+v", attachment)
	}
	if err := os.Remove(imagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadChatImage(ChatAttachmentPath(attachment)); err != nil {
		t.Fatalf("removing source lost the snapshot: %v", err)
	}
	for _, ref := range []string{"chat-attachment:../secret", "chat-attachment:..\\secret", "chat-attachment:C:secret", "chat-attachment:", filePath} {
		if ChatAttachmentPath(AIChatAttachment{URL: ref}) != "" {
			t.Fatalf("accepted unsafe image reference: %q", ref)
		}
	}
	if _, err := ImportChatAttachment(directory); err == nil {
		t.Fatal("accepted a directory")
	}
	if _, err := ReadChatImage(filePath); err == nil {
		t.Fatal("accepted binary data as an image")
	}
	large, err := os.Create(filepath.Join(directory, "large.png"))
	if err != nil {
		t.Fatal(err)
	}
	err = large.Truncate(ChatImageMaxBytes + 1)
	_ = large.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportChatAttachment(large.Name()); err == nil {
		t.Fatal("accepted an oversized image")
	}
}
