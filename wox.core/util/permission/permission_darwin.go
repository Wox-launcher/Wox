package permission

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework AVFoundation
// #import <AVFoundation/AVFoundation.h>
// #import <Cocoa/Cocoa.h>
// #import <Dispatch/Dispatch.h>
// #import <Foundation/Foundation.h>
//
// bool hasAccessibilityPermission() {
//     NSDictionary *options = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @NO};
//     return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
// }
//
// void openPrivacySecurityPreferences() {
//     NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension"];
//     [[NSWorkspace sharedWorkspace] openURL:url];
// }
//
// bool requestMicrophonePermission() {
//     AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
//     if (status == AVAuthorizationStatusAuthorized) {
//         return true;
//     }
//     if (status != AVAuthorizationStatusNotDetermined) {
//         return false;
//     }
//
//     dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
//     __block BOOL granted = NO;
//     [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL allowed) {
//         granted = allowed;
//         dispatch_semaphore_signal(semaphore);
//     }];
//     dispatch_semaphore_wait(semaphore, DISPATCH_TIME_FOREVER);
//     return granted == YES;
// }
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"wox/util/mainthread"
)

const macOSPermissionProbeCacheDuration = 500 * time.Millisecond

var macOSPermissionProbeCache struct {
	sync.Mutex
	status    MacOSPermissionStatus
	expiresAt time.Time
}

func init() {
	probeMacOSPermissionStatusPlatform = probeMacOSPermissionStatusInFreshProcess
}

func HasAccessibilityPermission(ctx context.Context) bool {
	return bool(C.hasAccessibilityPermission())
}

// probeMacOSPermissionStatusInFreshProcess avoids the process-local denial cache kept by macOS permission APIs.
func probeMacOSPermissionStatusInFreshProcess(ctx context.Context) (MacOSPermissionStatus, error) {
	macOSPermissionProbeCache.Lock()
	defer macOSPermissionProbeCache.Unlock()

	if time.Now().Before(macOSPermissionProbeCache.expiresAt) {
		return macOSPermissionProbeCache.status, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return MacOSPermissionStatus{}, fmt.Errorf("resolve Wox executable for permission probe: %w", err)
	}

	probeContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(probeContext, executable)
	command.Env = append(os.Environ(), macOSPermissionProbeEnvironment+"=1")
	output, err := command.Output()
	if err != nil {
		return MacOSPermissionStatus{}, fmt.Errorf("run macOS permission probe: %w", err)
	}

	var status MacOSPermissionStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return MacOSPermissionStatus{}, fmt.Errorf("decode macOS permission probe response: %w", err)
	}
	macOSPermissionProbeCache.status = status
	macOSPermissionProbeCache.expiresAt = time.Now().Add(macOSPermissionProbeCacheDuration)
	return status, nil
}

// GetFullDiskAccessPermissionState probes the protected user TCC database without prompting for access.
func GetFullDiskAccessPermissionState(ctx context.Context) MacOSPermissionState {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return MacOSPermissionUnknown
	}

	tccDatabasePath := filepath.Join(homeDirectory, "Library", "Application Support", "com.apple.TCC", "TCC.db")
	file, err := os.Open(tccDatabasePath)
	if err == nil {
		_ = file.Close()
		return MacOSPermissionGranted
	}
	if errors.Is(err, os.ErrPermission) {
		return MacOSPermissionNotGranted
	}
	return MacOSPermissionUnknown
}

func OpenPrivacySecuritySettings(ctx context.Context) {
	mainthread.Call(func() {
		C.openPrivacySecurityPreferences()
	})
}

// RequestMicrophonePermission requests access when needed and reports whether recording is authorized.
func RequestMicrophonePermission(ctx context.Context) bool {
	return bool(C.requestMicrophonePermission())
}
