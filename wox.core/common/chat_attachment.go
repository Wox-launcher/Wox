package common

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"wox/util"

	"github.com/google/uuid"
)

const (
	ChatAttachmentErrorMissing       = "missing"
	ChatAttachmentErrorPermission    = "permission"
	ChatAttachmentErrorDirectory     = "directory"
	ChatAttachmentErrorInvalidImage  = "invalid_image"
	ChatAttachmentErrorTooLarge      = "too_large"
	ChatAttachmentErrorTooManyPixels = "too_many_pixels"
	chatAttachmentReasonKeyPrefix    = "plugin_ai_chat_attachment_reason_"
)

// ChatAttachmentError names one failed import without embedding a full filesystem path.
type ChatAttachmentError struct {
	Name string
	Kind string
	Err  error
}

func (e *ChatAttachmentError) Error() string {
	if e == nil || e.Err == nil {
		return "chat attachment import failed"
	}
	if e.Name == "" {
		return e.Err.Error()
	}
	return e.Name + ": " + e.Err.Error()
}

func (e *ChatAttachmentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ChatAttachmentFailureReasonKey maps an import error to a localized reason key.
func ChatAttachmentFailureReasonKey(err error) string {
	kind := ClassifyChatAttachmentError(err)
	if kind == "" {
		return ""
	}
	return chatAttachmentReasonKeyPrefix + kind
}

// ChatAttachmentErrorName is the basename shown in user-facing import failures.
func ChatAttachmentErrorName(err error) string {
	var typed *ChatAttachmentError
	if errors.As(err, &typed) && typed.Name != "" {
		return typed.Name
	}
	return ""
}

// ClassifyChatAttachmentError returns a stable reason token for UI and plugin messages.
func ClassifyChatAttachmentError(err error) string {
	if err == nil {
		return ""
	}
	var typed *ChatAttachmentError
	if errors.As(err, &typed) && typed.Kind != "" {
		return typed.Kind
	}
	if os.IsNotExist(err) {
		return ChatAttachmentErrorMissing
	}
	if os.IsPermission(err) {
		return ChatAttachmentErrorPermission
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "not a regular file"):
		return ChatAttachmentErrorDirectory
	case strings.Contains(message, "20 MiB"):
		return ChatAttachmentErrorTooLarge
	case strings.Contains(message, "40 megapixel"):
		return ChatAttachmentErrorTooManyPixels
	case strings.Contains(message, "decode image"):
		return ChatAttachmentErrorInvalidImage
	default:
		return ""
	}
}

func wrapChatAttachmentError(path string, err error) error {
	if err == nil {
		return nil
	}
	var typed *ChatAttachmentError
	if errors.As(err, &typed) {
		return err
	}
	return &ChatAttachmentError{Name: filepath.Base(path), Kind: ClassifyChatAttachmentError(err), Err: err}
}

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

// ImportChatAttachments imports every path and rolls back newly created files if any import fails.
func ImportChatAttachments(paths []string) ([]AIChatAttachment, error) {
	imported := make([]AIChatAttachment, 0, len(paths))
	for _, path := range paths {
		attachment, err := ImportChatAttachment(path)
		if err != nil {
			RemoveImportedChatAttachments(imported)
			return nil, wrapChatAttachmentError(path, err)
		}
		imported = append(imported, attachment)
	}
	return imported, nil
}

// ImportChatImage writes a clipboard bitmap to a temporary PNG, then reuses file import.
func ImportChatImage(img image.Image) (AIChatAttachment, error) {
	if img == nil {
		return AIChatAttachment{}, &ChatAttachmentError{Name: "clipboard.png", Kind: ChatAttachmentErrorInvalidImage, Err: fmt.Errorf("decode image: empty bitmap")}
	}
	if err := chatImagePixelLimit(img); err != nil {
		return AIChatAttachment{}, &ChatAttachmentError{Name: "clipboard.png", Kind: ClassifyChatAttachmentError(err), Err: err}
	}
	temp, err := os.CreateTemp("", "wox-chat-clip-*.png")
	if err != nil {
		return AIChatAttachment{}, wrapChatAttachmentError("clipboard.png", err)
	}
	tempPath := temp.Name()
	encodeErr := png.Encode(temp, img)
	closeErr := temp.Close()
	if encodeErr != nil || closeErr != nil {
		_ = os.Remove(tempPath)
		if encodeErr == nil {
			encodeErr = closeErr
		}
		return AIChatAttachment{}, &ChatAttachmentError{Name: "clipboard.png", Kind: ChatAttachmentErrorInvalidImage, Err: fmt.Errorf("decode image: %w", encodeErr)}
	}
	defer os.Remove(tempPath)
	attachment, err := ImportChatAttachment(tempPath)
	if err != nil {
		return AIChatAttachment{}, wrapChatAttachmentError("clipboard.png", err)
	}
	return attachment, nil
}

// RemoveImportedChatAttachments deletes managed image snapshots that this batch created.
func RemoveImportedChatAttachments(attachments []AIChatAttachment) {
	for _, attachment := range attachments {
		if path := ChatAttachmentPath(attachment); path != "" {
			_ = os.Remove(path)
		}
	}
}

func chatImagePixelLimit(img image.Image) error {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || int64(width) > 40_000_000/int64(height) {
		return fmt.Errorf("image exceeds 40 megapixels")
	}
	return nil
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
