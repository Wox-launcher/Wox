package common

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"wox/util"

	"github.com/google/uuid"
)

const chatAttachmentPrefix = "chat-attachment:"
const ChatImageMaxBytes = 20 * 1024 * 1024

// ChatAttachmentPath resolves a managed image reference without allowing path traversal.
func ChatAttachmentPath(attachment AIChatAttachment) string {
	id, ok := strings.CutPrefix(attachment.URL, chatAttachmentPrefix)
	if !ok || id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:`) {
		return ""
	}
	return filepath.Join(util.GetLocation().GetUserDataDirectory(), "chat", "attachments", id)
}

// ImportChatAttachment snapshots images; ordinary files keep their original absolute path only.
func ImportChatAttachment(path string) (AIChatAttachment, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return AIChatAttachment{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return AIChatAttachment{}, err
	}
	if !info.Mode().IsRegular() {
		return AIChatAttachment{}, fmt.Errorf("not a regular file: %s", abs)
	}
	attachment := AIChatAttachment{ID: uuid.NewString(), Kind: AIChatAttachmentFile, Name: filepath.Base(abs), URL: abs}
	if !util.IsImageFile(abs) || strings.EqualFold(filepath.Ext(abs), ".svg") {
		return attachment, nil
	}
	data, err := ReadChatImage(abs)
	if err != nil {
		return AIChatAttachment{}, err
	}
	attachment.Kind = AIChatAttachmentImage
	attachment.MimeType = http.DetectContentType(data)
	attachment.URL = chatAttachmentPrefix + attachment.ID + strings.ToLower(filepath.Ext(abs))
	dest := ChatAttachmentPath(attachment)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return AIChatAttachment{}, err
	}
	if err := os.WriteFile(dest, data, 0600); err != nil {
		_ = os.Remove(dest)
		return AIChatAttachment{}, err
	}
	return attachment, nil
}

// ReadChatImage bounds encoded bytes and decoded pixels before importing or sending an image.
func ReadChatImage(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > ChatImageMaxBytes {
		return nil, fmt.Errorf("image must be a regular file no larger than 20 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, ChatImageMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > ChatImageMaxBytes {
		return nil, fmt.Errorf("image exceeds 20 MiB")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > 40_000_000/int64(config.Height) {
		return nil, fmt.Errorf("image exceeds 40 megapixels")
	}
	return data, nil
}
