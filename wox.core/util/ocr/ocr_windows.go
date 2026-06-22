//go:build windows

package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"wox/util"
	"wox/util/shell"
)

type windowsHelperResponse struct {
	Engine string `json:"engine"`
	Text   string `json:"text"`
	Error  string `json:"error"`
	Code   string `json:"code"`
}

func recognizePlatform(ctx context.Context, request Request) (Result, error) {
	// Windows OCR prefers the Windows App SDK AI helper because it is the newer system OCR path.
	// The helper is optional at runtime so portable builds can still fall back to the older WinRT
	// OCR engine instead of failing screenshot history text search entirely.
	if result, err := recognizeWindowsWithAIHelper(ctx, request); err == nil {
		return result, nil
	} else if !errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrUnsupported) {
		util.GetLogger().Warn(ctx, "windows ai text recognizer helper failed: "+err.Error())
	}
	if result, err := recognizeWindowsWithAITextRecognizerPowerShell(ctx, request); err == nil {
		return result, nil
	} else if !errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrUnsupported) {
		util.GetLogger().Warn(ctx, "windows ai text recognizer powershell bridge failed: "+err.Error())
	}

	return recognizeWindowsWithMediaOCR(ctx, request)
}

func recognizeWindowsWithAIHelper(ctx context.Context, request Request) (Result, error) {
	helperPath := findWindowsAITextRecognizerHelper()
	if helperPath == "" {
		return Result{}, fmt.Errorf("%w: windows ai text recognizer helper is not installed", ErrUnavailable)
	}

	helperCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := shell.BuildCommandContext(helperCtx, helperPath, nil)
	input := map[string]any{
		"imagePath": request.ImagePath,
		"languages": request.Languages,
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(input); err != nil {
		return Result{}, err
	}
	cmd.Stdin = &stdin
	output, err := cmd.Output()
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	var response windowsHelperResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return Result{}, fmt.Errorf("failed to decode windows ai text recognizer response: %w", err)
	}
	if response.Error != "" {
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, response.Error)
	}
	if response.Engine == "" {
		response.Engine = EngineWindowsAITextRecognizer
	}
	return Result{Engine: response.Engine, Text: response.Text}, nil
}

func findWindowsAITextRecognizerHelper() string {
	// Keep helper discovery file-based rather than hard-coded to the running executable directory.
	// Embedded resources are extracted into the data directory on startup, while local development
	// builds may place the helper beside wox.exe for manual OCR verification.
	candidates := []string{
		filepath.Join(util.GetLocation().GetOthersDirectory(), "ocr", "windows", "wox-ocr-helper.exe"),
	}
	if executablePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "wox-ocr-helper.exe"))
	}
	for _, candidate := range candidates {
		if util.IsFileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func recognizeWindowsWithMediaOCR(ctx context.Context, request Request) (Result, error) {
	return runWindowsPowerShellOCR(ctx, request, EngineWindowsMediaOCR, windowsMediaOCRPowerShellScript())
}

func recognizeWindowsWithAITextRecognizerPowerShell(ctx context.Context, request Request) (Result, error) {
	return runWindowsPowerShellOCR(ctx, request, EngineWindowsAITextRecognizer, windowsAITextRecognizerPowerShellScript())
}

func runWindowsPowerShellOCR(ctx context.Context, request Request, engine string, script string) (Result, error) {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
		if err != nil {
			return Result{}, fmt.Errorf("%w: powershell is unavailable", ErrUnavailable)
		}
	}

	ocrCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := shell.BuildCommandContext(ocrCtx, powershell, nil,
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
	)
	input := map[string]any{
		"imagePath": request.ImagePath,
		"languages": request.Languages,
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(input); err != nil {
		return Result{}, err
	}
	cmd.Stdin = &stdin
	output, err := cmd.Output()
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	var response windowsHelperResponse
	if err := json.Unmarshal(bytes.TrimSpace(output), &response); err != nil {
		return Result{}, fmt.Errorf("failed to decode windows ocr response: %w", err)
	}
	if response.Error != "" {
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, response.Error)
	}
	if response.Engine == "" {
		response.Engine = engine
	}
	if strings.TrimSpace(response.Text) == "" {
		return Result{Engine: response.Engine}, nil
	}
	return Result{Engine: response.Engine, Text: response.Text}, nil
}

func windowsAITextRecognizerPowerShellScript() string {
	return `
$ErrorActionPreference = 'Stop'
try {
  $inputJson = [Console]::In.ReadToEnd()
  $request = $inputJson | ConvertFrom-Json
  Add-Type -AssemblyName System.Runtime.WindowsRuntime
  [Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime] > $null
  [Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType=WindowsRuntime] > $null
  [Microsoft.Windows.AI.Imaging.TextRecognizer, Microsoft.Windows.AI.Imaging, ContentType=WindowsRuntime] > $null
  [Microsoft.Graphics.Imaging.ImageBuffer, Microsoft.Graphics.Imaging, ContentType=WindowsRuntime] > $null

  $asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -like 'IAsyncOperation*'
  })[0]
  function Await-WinRT($operation, $typeName) {
    $generic = $asTaskGeneric.MakeGenericMethod([Type]::GetType($typeName))
    return $generic.Invoke($null, @($operation)).GetAwaiter().GetResult()
  }

  $readyState = [Microsoft.Windows.AI.Imaging.TextRecognizer]::GetReadyState()
  if ([string]$readyState -ne 'Ready') {
    $readyResult = Await-WinRT ([Microsoft.Windows.AI.Imaging.TextRecognizer]::EnsureReadyAsync()) 'Microsoft.Windows.AI.AIFeatureReadyResult, Microsoft.Windows.AI, ContentType=WindowsRuntime'
    if ([string]$readyResult.Status -ne 'Success' -and [string]$readyResult.Status -ne 'Ready') {
      throw "Windows AI TextRecognizer model is not ready: $($readyResult.Status)"
    }
  }

  $file = Await-WinRT ([Windows.Storage.StorageFile]::GetFileFromPathAsync([string]$request.imagePath)) 'Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime'
  $stream = Await-WinRT ($file.OpenReadAsync()) 'Windows.Storage.Streams.IRandomAccessStreamWithContentType, Windows.Storage.Streams, ContentType=WindowsRuntime'
  $decoder = Await-WinRT ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) 'Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType=WindowsRuntime'
  $bitmap = Await-WinRT ($decoder.GetSoftwareBitmapAsync()) 'Windows.Graphics.Imaging.SoftwareBitmap, Windows.Graphics.Imaging, ContentType=WindowsRuntime'
  $imageBufferType = [Microsoft.Graphics.Imaging.ImageBuffer]
  $imageBufferMethod = $imageBufferType.GetMethod('CreateBufferAttachedToBitmap')
  if ($null -eq $imageBufferMethod) {
    $imageBufferMethod = $imageBufferType.GetMethod('CreateForSoftwareBitmap')
  }
  if ($null -eq $imageBufferMethod) {
    throw 'Microsoft.Graphics.Imaging.ImageBuffer does not expose a supported SoftwareBitmap factory'
  }
  $imageBuffer = $imageBufferMethod.Invoke($null, @($bitmap))
  $recognizer = Await-WinRT ([Microsoft.Windows.AI.Imaging.TextRecognizer]::CreateAsync()) 'Microsoft.Windows.AI.Imaging.TextRecognizer, Microsoft.Windows.AI.Imaging, ContentType=WindowsRuntime'
  $recognized = $recognizer.RecognizeTextFromImage($imageBuffer)
  $lines = @()
  foreach ($line in $recognized.Lines) {
    if ($null -ne $line.Text -and [string]$line.Text -ne '') {
      $lines += [string]$line.Text
    }
  }
  [Console]::Out.Write((@{ engine = 'windows_ai_text_recognizer'; text = ($lines -join [Environment]::NewLine) } | ConvertTo-Json -Compress))
} catch {
  [Console]::Out.Write((@{ engine = 'windows_ai_text_recognizer'; code = 'unavailable'; error = $_.Exception.Message } | ConvertTo-Json -Compress))
}
`
}

func windowsMediaOCRPowerShellScript() string {
	return `
$ErrorActionPreference = 'Stop'
try {
  $inputJson = [Console]::In.ReadToEnd()
  $request = $inputJson | ConvertFrom-Json
  Add-Type -AssemblyName System.Runtime.WindowsRuntime
  [Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime] > $null
  [Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType=WindowsRuntime] > $null
  [Windows.Globalization.Language, Windows.Globalization, ContentType=WindowsRuntime] > $null
  [Windows.System.UserProfile.GlobalizationPreferences, Windows.System.UserProfile, ContentType=WindowsRuntime] > $null
  [Windows.Media.Ocr.OcrEngine, Windows.Media.Ocr, ContentType=WindowsRuntime] > $null

  $asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -like 'IAsyncOperation*'
  })[0]
  function Await-WinRT($operation, $typeName) {
    $generic = $asTaskGeneric.MakeGenericMethod([Type]::GetType($typeName))
    return $generic.Invoke($null, @($operation)).GetAwaiter().GetResult()
  }

  $file = Await-WinRT ([Windows.Storage.StorageFile]::GetFileFromPathAsync([string]$request.imagePath)) 'Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime'
  $stream = Await-WinRT ($file.OpenReadAsync()) 'Windows.Storage.Streams.IRandomAccessStreamWithContentType, Windows.Storage.Streams, ContentType=WindowsRuntime'
  $decoder = Await-WinRT ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) 'Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType=WindowsRuntime'
  $bitmap = Await-WinRT ($decoder.GetSoftwareBitmapAsync()) 'Windows.Graphics.Imaging.SoftwareBitmap, Windows.Graphics.Imaging, ContentType=WindowsRuntime'

  # Feature fix: TryCreateFromUserProfileLanguages can pick the first profile language even
  # when another installed OCR language is needed for the image. Try explicit or installed
  # non-English recognizers first, and only fall back to profile/English recognizers when
  # the first pass finds no text so English OCR does not append mojibake to Chinese results.
  $primaryLanguageCandidates = New-Object 'System.Collections.Generic.List[string]'
  $fallbackLanguageCandidates = New-Object 'System.Collections.Generic.List[string]'
  $seenPrimaryLanguageCandidates = @{}
  $seenFallbackLanguageCandidates = @{}
  function Add-LanguageCandidate($list, [hashtable]$seen, [string]$tag) {
    if ($null -eq $tag) {
      return
    }
    $trimmed = $tag.Trim()
    if ($trimmed -eq '') {
      return
    }
    $key = $trimmed.ToLowerInvariant()
    if (-not $seen.ContainsKey($key)) {
      $seen[$key] = $true
      [void]$list.Add($trimmed)
    }
  }

  $hasRequestedLanguages = $false
  if ($null -ne $request.languages) {
    foreach ($languageTag in @($request.languages)) {
      Add-LanguageCandidate $primaryLanguageCandidates $seenPrimaryLanguageCandidates ([string]$languageTag)
      $hasRequestedLanguages = $true
    }
  }
  if (-not $hasRequestedLanguages) {
    foreach ($language in [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages) {
      $languageTag = [string]$language.LanguageTag
      if ($languageTag -notmatch '^en($|-)') {
        Add-LanguageCandidate $primaryLanguageCandidates $seenPrimaryLanguageCandidates $languageTag
      }
    }
  }
  foreach ($languageTag in [Windows.System.UserProfile.GlobalizationPreferences]::Languages) {
    Add-LanguageCandidate $fallbackLanguageCandidates $seenFallbackLanguageCandidates ([string]$languageTag)
  }
  foreach ($language in [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages) {
    Add-LanguageCandidate $fallbackLanguageCandidates $seenFallbackLanguageCandidates ([string]$language.LanguageTag)
  }

  $recognizedTexts = New-Object 'System.Collections.Generic.List[string]'
  function Recognize-WithLanguages($candidateTags) {
    foreach ($languageTag in $candidateTags) {
      try {
        $language = [Windows.Globalization.Language]::new($languageTag)
        $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($language)
        if ($null -eq $engine) {
          continue
        }
        $result = Await-WinRT ($engine.RecognizeAsync($bitmap)) 'Windows.Media.Ocr.OcrResult, Windows.Media.Ocr, ContentType=WindowsRuntime'
        if ($null -ne $result.Text -and [string]$result.Text -ne '') {
          [void]$recognizedTexts.Add([string]$result.Text)
        }
      } catch {
        continue
      }
    }
  }

  Recognize-WithLanguages $primaryLanguageCandidates
  if ($recognizedTexts.Count -eq 0) {
    Recognize-WithLanguages $fallbackLanguageCandidates
  }
  if ($recognizedTexts.Count -eq 0) {
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
    if ($null -eq $engine) {
      throw 'Windows.Media.Ocr.OcrEngine is unavailable for requested and installed languages'
    }
    $result = Await-WinRT ($engine.RecognizeAsync($bitmap)) 'Windows.Media.Ocr.OcrResult, Windows.Media.Ocr, ContentType=WindowsRuntime'
    if ($null -ne $result.Text -and [string]$result.Text -ne '') {
      [void]$recognizedTexts.Add([string]$result.Text)
    }
  }

  $uniqueLines = New-Object 'System.Collections.Generic.List[string]'
  $seenLines = @{}
  foreach ($text in $recognizedTexts) {
    foreach ($line in ([string]$text -split '\r?\n')) {
      $trimmed = $line.Trim()
      if ($trimmed -eq '') {
        continue
      }
      if (-not $seenLines.ContainsKey($trimmed)) {
        $seenLines[$trimmed] = $true
        [void]$uniqueLines.Add($trimmed)
      }
    }
  }
  [Console]::Out.Write((@{ engine = 'windows_media_ocr'; text = ($uniqueLines -join [Environment]::NewLine) } | ConvertTo-Json -Compress))
} catch {
  [Console]::Out.Write((@{ engine = 'windows_media_ocr'; code = 'unavailable'; error = $_.Exception.Message } | ConvertTo-Json -Compress))
}
`
}
