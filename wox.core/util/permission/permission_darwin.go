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
// void openAccessibilityPreferences() {
//     NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"];
//     [[NSWorkspace sharedWorkspace] openURL:url];
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
	"wox/util/mainthread"
)

func HasAccessibilityPermission(ctx context.Context) bool {
	return bool(C.hasAccessibilityPermission())
}

func GrantAccessibilityPermission(ctx context.Context) {
	mainthread.Call(func() {
		C.openAccessibilityPreferences()
	})
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
