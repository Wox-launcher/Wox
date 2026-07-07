#import <AppKit/AppKit.h>
#import <stdlib.h>

// Retain sounds for the duration of playback; released when playback ends.
static NSMutableArray* g_playingSounds = nil;

@interface SoundDelegate : NSObject <NSSoundDelegate>
@end

@implementation SoundDelegate
- (void)sound:(NSSound*)sound didFinishPlaying:(BOOL)finishedPlaying {
    if (g_playingSounds) {
        [g_playingSounds removeObject:sound];
    }
}
@end

// playSoundFileMac loads the wav file at filePath into an NSSound and plays it
// asynchronously. Returns 1 if the sound was created and started, 0 otherwise.
int playSoundFileMac(const char* filePath) {
    @autoreleasepool {
        static SoundDelegate* delegate = nil;
        static dispatch_once_t once;
        dispatch_once(&once, ^{
            delegate = [[SoundDelegate alloc] init];
            g_playingSounds = [[NSMutableArray alloc] init];
        });

        NSString* nsPath = [NSString stringWithUTF8String:filePath];
        NSSound* sound = [[NSSound alloc] initWithContentsOfFile:nsPath byReference:YES];
        if (sound == nil) {
            return 0;
        }
        [sound setVolume:1.0f];
        [sound setDelegate:delegate];
        BOOL ok = [sound play];
        if (!ok) {
            return 0;
        }
        [g_playingSounds addObject:sound];
        return 1;
    }
}