package util

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#include <Carbon/Carbon.h>
#include <stdio.h>
#include <CoreServices/CoreServices.h>

char* getCurrentInputMethod() {
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
    TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
    CFStringRef sourceID = TISGetInputSourceProperty(source, kTISPropertyInputSourceID);
    NSString *inputMethodID = (__bridge NSString *)sourceID;
    [pool release];
    return (char *)[inputMethodID UTF8String];
}

void switchInputMethod(const char *inputMethodID) {
    CFStringRef inputMethodIDString = CFStringCreateWithCString(NULL, inputMethodID, kCFStringEncodingUTF8);

    CFArrayRef sources = TISCreateInputSourceList(NULL, false);
    CFIndex sourceCount = CFArrayGetCount(sources);

    for (CFIndex i = 0; i < sourceCount; i++) {
        TISInputSourceRef source = (TISInputSourceRef)CFArrayGetValueAtIndex(sources, i);
        CFStringRef sourceID = TISGetInputSourceProperty(source, kTISPropertyInputSourceID);

        if (CFStringCompare(inputMethodIDString, sourceID, 0) == kCFCompareEqualTo) {
            TISSelectInputSource(source);
            break;
        }
    }

    CFRelease(inputMethodIDString);
}
*/
import "C"
import (
	"unsafe"
)

func SwitchInputMethodABC() {
	abcInputMethodID := "com.apple.keylayout.ABC"

	inputMethod := C.GoString(C.getCurrentInputMethod())
	if inputMethod == abcInputMethodID {
		return
	}

	inputMethodIDStr := C.CString(abcInputMethodID)
	defer C.free(unsafe.Pointer(inputMethodIDStr))
	C.switchInputMethod(inputMethodIDStr)
}
