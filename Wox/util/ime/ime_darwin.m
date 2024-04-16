#import <Cocoa/Cocoa.h>
#include <Carbon/Carbon.h>
#include <stdio.h>
#include <CoreServices/CoreServices.h>

char* getCurrentInputMethod() {
    char* result = NULL;
    @try {
        NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];
        TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
        CFStringRef sourceID = TISGetInputSourceProperty(source, kTISPropertyInputSourceID);
        NSString *inputMethodID = (__bridge NSString *)sourceID;
        result = (char *)[inputMethodID UTF8String];
        [pool release];
    }
    @catch (NSException *exception) {
        NSLog(@"Exception occurred: %@, %@", exception, [exception userInfo]);
    }
    @finally {
        return result;
    }
}

void switchInputMethod(const char *inputMethodID) {
    @try {
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
    @catch (NSException *exception) {
        NSLog(@"Exception occurred: %@, %@", exception, [exception userInfo]);
    }
}