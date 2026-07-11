#import <AVFoundation/AVFoundation.h>
#import <stdlib.h>

extern void audioPlaybackFinished(char* filePath, int success);
extern void audioPlaybackDecodeFailed(char* filePath, char* message);

@class SoundDelegate;

// Retain and prewarm players by file path so a short dictation cue does not
// race player creation with the microphone device startup.
static NSMutableDictionary* g_audioPlayers = nil;
static SoundDelegate* g_audioPlayerDelegate = nil;

@interface SoundDelegate : NSObject <AVAudioPlayerDelegate>
@end

static void initializeAudioPlayers(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        g_audioPlayerDelegate = [[SoundDelegate alloc] init];
        g_audioPlayers = [[NSMutableDictionary alloc] init];
    });
}

static NSString* pathForPlayer(AVAudioPlayer* player) {
    for (NSString* path in g_audioPlayers) {
        if ([g_audioPlayers objectForKey:path] == player) {
            return path;
        }
    }
    return @"";
}

@implementation SoundDelegate
- (void)audioPlayerDidFinishPlaying:(AVAudioPlayer*)player successfully:(BOOL)successfully {
    audioPlaybackFinished((char*)[pathForPlayer(player) UTF8String], successfully ? 1 : 0);
}

- (void)audioPlayerDecodeErrorDidOccur:(AVAudioPlayer*)player error:(NSError*)error {
    const char* message = error == nil ? "unknown decode error" : [[error localizedDescription] UTF8String];
    if (message == NULL) {
        message = "unknown decode error";
    }
    audioPlaybackDecodeFailed((char*)[pathForPlayer(player) UTF8String], (char*)message);
}
@end

static AVAudioPlayer* preparedPlayerForPath(const char* filePath) {
    @autoreleasepool {
        initializeAudioPlayers();

        NSString* nsPath = [NSString stringWithUTF8String:filePath];
        AVAudioPlayer* player = [g_audioPlayers objectForKey:nsPath];
        if (player != nil) {
            return player;
        }

        NSError* error = nil;
        player = [[AVAudioPlayer alloc] initWithContentsOfURL:[NSURL fileURLWithPath:nsPath] error:&error];
        if (player == nil) {
            return nil;
        }

        [player setVolume:1.0f];
        [player setDelegate:g_audioPlayerDelegate];
        if (![player prepareToPlay]) {
            [player release];
            return nil;
        }
        [g_audioPlayers setObject:player forKey:nsPath];
        [player release];
        return [g_audioPlayers objectForKey:nsPath];
    }
}

// prepareSoundFileMac loads a dictation cue once so playback can start
// immediately when the recording overlay becomes visible.
int prepareSoundFileMac(const char* filePath) {
    return preparedPlayerForPath(filePath) == nil ? 0 : 1;
}

// playSoundFileMac plays a prepared cue asynchronously. Returns 1 when the
// system accepts playback; completion is reported through the delegate.
int playSoundFileMac(const char* filePath) {
    @autoreleasepool {
        AVAudioPlayer* player = preparedPlayerForPath(filePath);
        if (player == nil) {
            return 0;
        }
        if ([player isPlaying]) {
            [player stop];
        }
        [player setCurrentTime:0];
        if (![player play]) {
            return 0;
        }
        return 1;
    }
}
