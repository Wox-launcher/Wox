package permission

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa
// #import <Cocoa/Cocoa.h>
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
