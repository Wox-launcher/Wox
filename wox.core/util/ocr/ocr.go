package ocr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	ModelID   string
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

	var result Result
	if normalizedRequest.ModelID == "" || normalizedRequest.ModelID == ModelSystem {
		result, err = recognizePlatform(ctx, normalizedRequest)
	} else if normalizedRequest.ModelID == ModelPaddlePPOCRv6Small {
		manager, managerErr := GetPaddleModelManager()
		if managerErr != nil {
			return Result{}, managerErr
		}
		result, err = manager.Recognize(ctx, normalizedRequest.ImagePath)
	} else {
		return Result{}, fmt.Errorf("%w: unknown OCR model %s", ErrUnavailable, normalizedRequest.ModelID)
	}
	if err != nil {
		return Result{}, err
	}

	if len(result.Lines) > 0 {
		sortLinesByReadingOrder(result.Lines)
		textLines := make([]string, 0, len(result.Lines))
		for _, line := range result.Lines {
			if text := strings.TrimSpace(line.Text); text != "" {
				textLines = append(textLines, text)
			}
		}
		result.Text = normalizeText(strings.Join(textLines, "\n"))
	} else {
		result.Text = normalizeText(result.Text)
	}
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

type lineLayout struct {
	line   Line
	center float64
	height float64
	left   float64
}

// sortLinesByReadingOrder groups text boxes by their vertical centers before
// ordering each row from left to right. Engines without complete bounds keep
// their original line order.
// ponytail: Center-based grouping targets horizontal UI text; add document layout analysis only for multi-column OCR.
func sortLinesByReadingOrder(lines []Line) {
	layouts := make([]lineLayout, 0, len(lines))
	for _, line := range lines {
		if len(line.Bounds) == 0 {
			return
		}

		top, bottom := line.Bounds[0].Y, line.Bounds[0].Y
		layout := lineLayout{line: line, left: line.Bounds[0].X}
		for _, point := range line.Bounds[1:] {
			if point.Y < top {
				top = point.Y
			}
			if point.Y > bottom {
				bottom = point.Y
			}
			if point.X < layout.left {
				layout.left = point.X
			}
		}
		layout.center = (top + bottom) / 2
		layout.height = bottom - top
		layouts = append(layouts, layout)
	}

	sort.SliceStable(layouts, func(i, j int) bool {
		if layouts[i].center != layouts[j].center {
			return layouts[i].center < layouts[j].center
		}
		return layouts[i].left < layouts[j].left
	})

	heights := make([]float64, 0, len(layouts))
	for _, layout := range layouts {
		heights = append(heights, layout.height)
	}
	sort.Float64s(heights)
	rowTolerance := heights[len(heights)/2] / 2
	if rowTolerance == 0 {
		rowTolerance = 1
	}

	rows := make([][]lineLayout, 0, len(layouts))
	rowCenters := make([]float64, 0, len(layouts))
	for _, layout := range layouts {
		rowIndex := len(rows) - 1
		if rowIndex < 0 || layout.center-rowCenters[rowIndex] > rowTolerance {
			rows = append(rows, []lineLayout{layout})
			rowCenters = append(rowCenters, layout.center)
			continue
		}

		rows[rowIndex] = append(rows[rowIndex], layout)
	}

	index := 0
	for _, row := range rows {
		sort.SliceStable(row, func(i, j int) bool {
			return row[i].left < row[j].left
		})
		for _, layout := range row {
			lines[index] = layout.line
			index++
		}
	}
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
	request.ModelID = strings.TrimSpace(strings.ToLower(request.ModelID))
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

// IsSystemModelAvailable reports whether Wox has a platform OCR implementation.
func IsSystemModelAvailable() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "windows"
}
