package mediaplayer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
	"wox/plugin"
	"wox/util"
)

var mediaRetriever = &WindowsRetriever{}

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procGetClassNameW          = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

type WindowsRetriever struct {
	api plugin.API
}

func (w *WindowsRetriever) UpdateAPI(api plugin.API) {
	w.api = api
}

func (w *WindowsRetriever) GetPlatform() string {
	return util.PlatformWindows
}

func (w *WindowsRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	// Try different methods to get media information
	
	// Method 1: Try PowerShell Get-MediaSession (Windows 10+)
	if mediaInfo, err := w.getMediaFromPowerShell(ctx); err == nil {
		return mediaInfo, nil
	}
	
	// Method 2: Try to get info from window titles
	if mediaInfo, err := w.getMediaFromWindowTitles(ctx); err == nil {
		return mediaInfo, nil
	}
	
	return nil, errors.New("no media playing")
}

func (w *WindowsRetriever) IsMediaPlaying(ctx context.Context) bool {
	mediaInfo, err := w.GetCurrentMedia(ctx)
	return err == nil && mediaInfo != nil && mediaInfo.State == PlaybackStatePlaying
}

func (w *WindowsRetriever) TogglePlayPause(ctx context.Context) error {
	// Use PowerShell to simulate media play/pause key via user32.keybd_event
	script := `
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class K {
    [DllImport("user32.dll")] public static extern void keybd_event(byte bVk, byte bScan, int dwFlags, int dwExtraInfo);
}
"@
$VK_MEDIA_PLAY_PAUSE = 0xB3
$KEYEVENTF_EXTENDEDKEY = 0x1
$KEYEVENTF_KEYUP = 0x2
[K]::keybd_event([byte]$VK_MEDIA_PLAY_PAUSE, 0, $KEYEVENTF_EXTENDEDKEY, 0)
Start-Sleep -Milliseconds 80
[K]::keybd_event([byte]$VK_MEDIA_PLAY_PAUSE, 0, $KEYEVENTF_EXTENDEDKEY -bor $KEYEVENTF_KEYUP, 0)
`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to toggle play/pause: %w", err)
	}
	return nil
}

func (w *WindowsRetriever) getMediaFromPowerShell(ctx context.Context) (*MediaInfo, error) {
	// PowerShell script to get media session information
	script := `
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation' + '``1' })[0]

function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    $netTask.Result
}

try {
    [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager,Windows.Media.Control,ContentType=WindowsRuntime] | Out-Null
    $sessionManager = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync()
    $sessionManager = Await $sessionManager ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager])
    
    $currentSession = $sessionManager.GetCurrentSession()
    if ($currentSession) {
        $mediaProperties = $currentSession.TryGetMediaPropertiesAsync()
        $mediaProperties = Await $mediaProperties ([Windows.Media.Control.GlobalSystemMediaTransportControlsSessionMediaProperties])
        
        $playbackInfo = $currentSession.GetPlaybackInfo()
        $timelineProperties = $currentSession.GetTimelineProperties()
        
        $state = switch ($playbackInfo.PlaybackStatus) {
            "Playing" { "playing" }
            "Paused" { "paused" }
            "Stopped" { "stopped" }
            default { "unknown" }
        }
        
        $position = if ($timelineProperties) { [math]::Floor($timelineProperties.Position.TotalSeconds) } else { 0 }
        $duration = if ($timelineProperties) { [math]::Floor($timelineProperties.EndTime.TotalSeconds) } else { 0 }
        
        $result = @{
            title = $mediaProperties.Title
            artist = $mediaProperties.Artist
            album = $mediaProperties.AlbumTitle
            duration = $duration
            position = $position
            state = $state
            app = $currentSession.SourceAppUserModelId
        }
        
        $result | ConvertTo-Json -Compress
    } else {
        "no_session"
    }
} catch {
    "error: $($_.Exception.Message)"
}`

	cmd := exec.CommandContext(ctx, "powershell", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute PowerShell script: %w", err)
	}

	result := strings.TrimSpace(string(output))
	
	if result == "no_session" || strings.HasPrefix(result, "error:") {
		return nil, errors.New("no media session found")
	}

	return w.parseMediaInfoJSON(result)
}

func (w *WindowsRetriever) getMediaFromWindowTitles(ctx context.Context) (*MediaInfo, error) {
	// Get window titles from known media applications
	apps := []string{"Spotify.exe", "wmplayer.exe", "vlc.exe", "iTunes.exe", "foobar2000.exe"}
	
	for _, appName := range apps {
		if mediaInfo, err := w.getMediaFromWindowTitle(ctx, appName); err == nil {
			return mediaInfo, nil
		}
	}
	
	return nil, errors.New("no media found in window titles")
}

func (w *WindowsRetriever) getMediaFromWindowTitle(ctx context.Context, processName string) (*MediaInfo, error) {
	// Use tasklist to find the process and get window title
	cmd := exec.CommandContext(ctx, "tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if !strings.Contains(string(output), processName) {
		return nil, fmt.Errorf("process %s not found", processName)
	}

	// Try to get window title using PowerShell
	script := fmt.Sprintf(`
$processes = Get-Process | Where-Object { $_.ProcessName -like "%s*" -and $_.MainWindowTitle -ne "" }
foreach ($process in $processes) {
    $title = $process.MainWindowTitle
    if ($title -and $title -ne "") {
        Write-Output $title
        break
    }
}`, strings.TrimSuffix(processName, ".exe"))

	cmd = exec.CommandContext(ctx, "powershell", "-Command", script)
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	windowTitle := strings.TrimSpace(string(output))
	if windowTitle == "" {
		return nil, errors.New("no window title found")
	}

	return w.parseWindowTitle(windowTitle, processName)
}

func (w *WindowsRetriever) parseWindowTitle(title, processName string) (*MediaInfo, error) {
	appName := w.getAppNameFromProcess(processName)
	
	// Parse different title formats
	switch {
	case strings.Contains(processName, "Spotify"):
		return w.parseSpotifyTitle(title, appName)
	case strings.Contains(processName, "vlc"):
		return w.parseVLCTitle(title, appName)
	default:
		return w.parseGenericTitle(title, appName)
	}
}

func (w *WindowsRetriever) parseSpotifyTitle(title, appName string) (*MediaInfo, error) {
	// Spotify format: "Artist - Song Title"
	if strings.Contains(title, " - ") {
		parts := strings.SplitN(title, " - ", 2)
		if len(parts) == 2 {
			return &MediaInfo{
				Title:       parts[1],
				Artist:      parts[0],
				Album:       "",
				State:       PlaybackStatePlaying,
				AppName:     appName,
				AppBundleID: "com.spotify.client",
			}, nil
		}
	}
	
	return nil, errors.New("could not parse Spotify title")
}

func (w *WindowsRetriever) parseVLCTitle(title, appName string) (*MediaInfo, error) {
	// VLC format: "filename - VLC media player"
	if strings.Contains(title, " - VLC") {
		parts := strings.SplitN(title, " - VLC", 2)
		if len(parts) >= 1 {
			return &MediaInfo{
				Title:       parts[0],
				Artist:      "",
				Album:       "",
				State:       PlaybackStatePlaying,
				AppName:     appName,
				AppBundleID: "org.videolan.vlc",
			}, nil
		}
	}
	
	return nil, errors.New("could not parse VLC title")
}

func (w *WindowsRetriever) parseGenericTitle(title, appName string) (*MediaInfo, error) {
	return &MediaInfo{
		Title:       title,
		Artist:      "",
		Album:       "",
		State:       PlaybackStatePlaying,
		AppName:     appName,
		AppBundleID: appName,
	}, nil
}

func (w *WindowsRetriever) parseMediaInfoJSON(jsonStr string) (*MediaInfo, error) {
	// Simple JSON parsing without external dependencies
	// This is a basic implementation - in production, use proper JSON parsing
	
	mediaInfo := &MediaInfo{}
	
	// Extract values using string manipulation (basic approach)
	if title := w.extractJSONValue(jsonStr, "title"); title != "" {
		mediaInfo.Title = title
	}
	if artist := w.extractJSONValue(jsonStr, "artist"); artist != "" {
		mediaInfo.Artist = artist
	}
	if album := w.extractJSONValue(jsonStr, "album"); album != "" {
		mediaInfo.Album = album
	}
	if app := w.extractJSONValue(jsonStr, "app"); app != "" {
		mediaInfo.AppName = w.getAppNameFromBundleID(app)
		mediaInfo.AppBundleID = app
	}
	
	// Parse numeric values
	if durationStr := w.extractJSONValue(jsonStr, "duration"); durationStr != "" {
		if duration, err := strconv.ParseInt(durationStr, 10, 64); err == nil {
			mediaInfo.Duration = duration
		}
	}
	if positionStr := w.extractJSONValue(jsonStr, "position"); positionStr != "" {
		if position, err := strconv.ParseInt(positionStr, 10, 64); err == nil {
			mediaInfo.Position = position
		}
	}
	
	// Parse state
	if stateStr := w.extractJSONValue(jsonStr, "state"); stateStr != "" {
		switch stateStr {
		case "playing":
			mediaInfo.State = PlaybackStatePlaying
		case "paused":
			mediaInfo.State = PlaybackStatePaused
		case "stopped":
			mediaInfo.State = PlaybackStateStopped
		default:
			mediaInfo.State = PlaybackStateUnknown
		}
	}
	
	return mediaInfo, nil
}

func (w *WindowsRetriever) extractJSONValue(jsonStr, key string) string {
	// Basic JSON value extraction
	keyPattern := fmt.Sprintf(`"%s":"`, key)
	startIndex := strings.Index(jsonStr, keyPattern)
	if startIndex == -1 {
		return ""
	}
	
	startIndex += len(keyPattern)
	endIndex := strings.Index(jsonStr[startIndex:], `"`)
	if endIndex == -1 {
		return ""
	}
	
	return jsonStr[startIndex : startIndex+endIndex]
}

func (w *WindowsRetriever) getAppNameFromProcess(processName string) string {
	appNames := map[string]string{
		"Spotify.exe":     "Spotify",
		"wmplayer.exe":    "Windows Media Player",
		"vlc.exe":         "VLC",
		"iTunes.exe":      "iTunes",
		"foobar2000.exe":  "Foobar2000",
	}
	
	if name, ok := appNames[processName]; ok {
		return name
	}
	return strings.TrimSuffix(processName, ".exe")
}

func (w *WindowsRetriever) getAppNameFromBundleID(bundleID string) string {
	// Extract app name from Windows app bundle ID
	parts := strings.Split(bundleID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return bundleID
}
