package ocr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	EngineMacOSVision             = "macos_vision"
	EngineWindowsAITextRecognizer = "windows_ai_text_recognizer"
	EngineWindowsMediaOCR         = "windows_media_ocr"
)

var (
	ErrUnsupported = errors.New("ocr is unsupported on this platform")
	ErrUnavailable = errors.New("ocr engine is unavailable")
)

type Request struct {
	ImagePath string
	Languages []string
}

type Result struct {
	Engine string
	Text   string
	Lines  []Line
	Words  []Word
}

type Line struct {
	Text       string
	Confidence float64
	Bounds     []Point
}

type Word struct {
	Text       string
	Confidence float64
	Bounds     []Point
}

type Point struct {
	X float64
	Y float64
}

func Recognize(ctx context.Context, request Request) (Result, error) {
	normalizedRequest, err := normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}

	result, err := recognizePlatform(ctx, normalizedRequest)
	if err != nil {
		return Result{}, err
	}

	result.Text = normalizeText(result.Text)
	if len(result.Lines) == 0 && result.Text != "" {
		for _, line := range strings.Split(result.Text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			result.Lines = append(result.Lines, Line{Text: line})
		}
	}
	return result, nil
}

func normalizeRequest(request Request) (Request, error) {
	imagePath := strings.TrimSpace(request.ImagePath)
	if imagePath == "" {
		return Request{}, fmt.Errorf("ocr image path is empty")
	}

	// Feature fix: callers may build cache paths with slash-tolerant helpers
	// that still work for normal file IO, but Windows WinRT OCR rejects mixed
	// separators in StorageFile.GetFileFromPathAsync. Normalize once at the OCR
	// boundary so every platform helper receives an absolute native path.
	imagePath = filepath.Clean(imagePath)
	if absolutePath, err := filepath.Abs(imagePath); err == nil {
		imagePath = absolutePath
	}

	info, err := os.Stat(imagePath)
	if err != nil {
		return Request{}, fmt.Errorf("failed to stat ocr image: %w", err)
	}
	if info.IsDir() {
		return Request{}, fmt.Errorf("ocr image path is a directory")
	}
	if info.Size() == 0 {
		return Request{}, fmt.Errorf("ocr image file is empty")
	}

	// Feature fix: platform OCR engines expect BCP-47 language tags, but callers may pass
	// locale-style values such as zh_CN. Normalize the request once at the OCR boundary so
	// native engines can choose the right model instead of silently falling back to English.
	request.Languages = normalizeLanguages(request.Languages)
	request.ImagePath = imagePath
	return request, nil
}

func normalizeLanguages(languages []string) []string {
	if len(languages) == 0 {
		return nil
	}

	normalizedLanguages := make([]string, 0, len(languages))
	seenLanguages := map[string]bool{}
	for _, language := range languages {
		language = strings.TrimSpace(strings.ReplaceAll(language, "_", "-"))
		if language == "" {
			continue
		}
		key := strings.ToLower(language)
		if seenLanguages[key] {
			continue
		}
		seenLanguages[key] = true
		normalizedLanguages = append(normalizedLanguages, language)
	}
	return normalizedLanguages
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}
