#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>
#import <CoreFoundation/CoreFoundation.h>
#include <dlfcn.h>
#include <stdlib.h>

// Dynamic symbols for MediaRemote (private)
static void (*MRMediaRemoteGetNowPlayingInfo)(dispatch_queue_t, void (^)(NSDictionary *)) = NULL;
static void (*MRMediaRemoteGetNowPlayingApplicationPID)(dispatch_queue_t, void (^)(int)) = NULL;
static void (*MRMediaRemoteGetNowPlayingApplicationIsPlaying)(dispatch_queue_t, void (^)(Boolean)) = NULL;
static bool (*MRMediaRemoteSendCommand)(int, id) = NULL; // MRCommand, userInfo

// MRCommand enum (subset)
static const int kMRPlay = 0;
static const int kMRPause = 1;
static const int kMRTogglePlayPause = 2;

static bool load_mediaremote_once() {
    static dispatch_once_t once;
    static bool ok = false;
    dispatch_once(&once, ^{
        void *h = dlopen("/System/Library/PrivateFrameworks/MediaRemote.framework/MediaRemote", RTLD_LAZY);
        if (!h) return;
        MRMediaRemoteGetNowPlayingInfo = dlsym(h, "MRMediaRemoteGetNowPlayingInfo");
        MRMediaRemoteGetNowPlayingApplicationPID = dlsym(h, "MRMediaRemoteGetNowPlayingApplicationPID");
        MRMediaRemoteGetNowPlayingApplicationIsPlaying = dlsym(h, "MRMediaRemoteGetNowPlayingApplicationIsPlaying");
        MRMediaRemoteSendCommand = dlsym(h, "MRMediaRemoteSendCommand");
        ok = MRMediaRemoteGetNowPlayingInfo && MRMediaRemoteGetNowPlayingApplicationPID && MRMediaRemoteGetNowPlayingApplicationIsPlaying;
    });
    return ok;
}

static double mr_get_rate_now(void) {
    if (!load_mediaremote_once()) return -1.0;
    if (!MRMediaRemoteGetNowPlayingInfo) return -1.0;
    __block NSDictionary *np = nil;
    __block BOOL done = NO;
    MRMediaRemoteGetNowPlayingInfo(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^(NSDictionary *information){ np = information; done = YES; });
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:0.7];
    while (!done && [[NSDate date] compare:deadline] == NSOrderedAscending) {
        [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
    }
    if (!np) return -1.0;
    id rateObj = np[@"kMRMediaRemoteNowPlayingInfoPlaybackRate"];
    if ([rateObj isKindOfClass:[NSNumber class]]) return [((NSNumber*)rateObj) doubleValue];
    return -1.0;
}


// Exported API: toggle play/pause via MediaRemoteSendCommand
int wox_mr_toggle(void) {
    @autoreleasepool {
        // Ensure MediaRemote is loaded and try to resolve sendCommand if needed
        (void)load_mediaremote_once();
        if (!MRMediaRemoteSendCommand) {
            void *h = dlopen("/System/Library/PrivateFrameworks/MediaRemote.framework/MediaRemote", RTLD_LAZY);
            if (h) MRMediaRemoteSendCommand = dlsym(h, "MRMediaRemoteSendCommand");
        }
        if (!MRMediaRemoteSendCommand) return 0;
        // First try toggle
        if (MRMediaRemoteSendCommand(kMRTogglePlayPause, nil)) {
            // wait a short moment for command to propagate
            if (MRMediaRemoteGetNowPlayingApplicationPID) {
                __block BOOL doneWait = NO;
                MRMediaRemoteGetNowPlayingApplicationPID(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^(int pid){ doneWait = YES; });
                NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:0.5];
                while (!doneWait && [[NSDate date] compare:deadline] == NSOrderedAscending) {
                    [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
                }
            }
            return 1;
        }
        // Fallback: query isPlaying and send explicit Play/Pause
        __block NSNumber *isPlaying = nil;
        __block BOOL done = NO;
        if (MRMediaRemoteGetNowPlayingApplicationIsPlaying) {
            MRMediaRemoteGetNowPlayingApplicationIsPlaying(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^(Boolean playing){
                isPlaying = @(playing);
                done = YES;
            });
            NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:1.0];
            while (!done && [[NSDate date] compare:deadline] == NSOrderedAscending) {
                [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.02]];
            }
        }
        int cmd = (isPlaying && [isPlaying boolValue]) ? kMRPause : kMRPlay;
        bool ok = MRMediaRemoteSendCommand(cmd, nil);
        if (ok && MRMediaRemoteGetNowPlayingApplicationPID) {
            __block BOOL doneWait2 = NO;
            MRMediaRemoteGetNowPlayingApplicationPID(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^(int pid){ doneWait2 = YES; });
            NSDate *deadline2 = [NSDate dateWithTimeIntervalSinceNow:0.5];
            while (!doneWait2 && [[NSDate date] compare:deadline2] == NSOrderedAscending) {
                [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
            }
        }
        return ok ? 1 : 0;
    }
}

// Exported API: returns malloc'ed JSON string; caller must free via wox_mr_free
const char *wox_mr_get_now_playing_json(void) {
    @autoreleasepool {
        if (!load_mediaremote_once()) return NULL;

        __block NSDictionary *np = nil;
        __block NSNumber *isPlaying = nil;
        __block NSNumber *pidNum = nil;
        __block BOOL doneInfo = NO, donePlay = NO, donePID = NO;

        dispatch_queue_t q = dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0);

        MRMediaRemoteGetNowPlayingInfo(q, ^(NSDictionary *information){
            np = information;
            doneInfo = YES;
        });
        MRMediaRemoteGetNowPlayingApplicationIsPlaying(q, ^(Boolean playing){
            // Use true boolean to ensure JSON encodes as true/false, not 1/0
            isPlaying = @(playing);
            donePlay = YES;
        });
        MRMediaRemoteGetNowPlayingApplicationPID(q, ^(int pid){
            pidNum = @(pid);
            donePID = YES;
        });

        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:3.0];
        while ((!(doneInfo && donePlay && donePID)) && [[NSDate date] compare:deadline] == NSOrderedAscending) {
            [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.05]];
        }
        if (!(doneInfo && donePlay && donePID)) return NULL;
        if (!np || np.count == 0) return NULL;

        NSMutableDictionary *out = [NSMutableDictionary dictionary];
        NSString *title = np[@"kMRMediaRemoteNowPlayingInfoTitle"]; if (title) out[@"title"] = title;
        NSString *artist = np[@"kMRMediaRemoteNowPlayingInfoArtist"]; if (artist) out[@"artist"] = artist;
        NSString *album = np[@"kMRMediaRemoteNowPlayingInfoAlbum"]; if (album) out[@"album"] = album;
        NSNumber *dur = np[@"kMRMediaRemoteNowPlayingInfoDuration"]; if (dur) out[@"duration"] = dur;
        id elapsedObj = np[@"kMRMediaRemoteNowPlayingInfoElapsedTime"];
        id rateObj = np[@"kMRMediaRemoteNowPlayingInfoPlaybackRate"];
        id tsObj = np[@"kMRMediaRemoteNowPlayingInfoTimestamp"];
        double baseElapsed = 0.0;
        if ([elapsedObj isKindOfClass:[NSNumber class]]) {
            baseElapsed = [((NSNumber*)elapsedObj) doubleValue];
        }
        double playbackRate = 0.0;
        if ([rateObj isKindOfClass:[NSNumber class]]) {
            playbackRate = [((NSNumber*)rateObj) doubleValue];
        }
        // Determine playing state first
        BOOL playingFlag = NO;
        if (isPlaying != nil) {
            playingFlag = [isPlaying boolValue];
        } else if ([rateObj isKindOfClass:[NSNumber class]]) {
            playingFlag = (playbackRate > 0.01);
        }
        // Compute current position: only advance when playing
        double posVal = baseElapsed;
        if (playingFlag && [tsObj isKindOfClass:[NSDate class]]) {
            NSTimeInterval tsEpoch = [((NSDate*)tsObj) timeIntervalSince1970];
            NSTimeInterval nowEpoch = [[NSDate date] timeIntervalSince1970];
            NSTimeInterval diff = nowEpoch - tsEpoch;
            if (diff > -10 && diff < 86400) {
                posVal += diff * playbackRate;
            }
        }
        if (dur) {
            double d = [dur doubleValue];
            if (posVal < 0) posVal = 0; else if (d > 0 && posVal > d) posVal = d;
        }
        out[@"position"] = @(posVal);
        out[@"playing"] = playingFlag ? (id)kCFBooleanTrue : (id)kCFBooleanFalse;
        if (rateObj) { out[@"playbackRate"] = rateObj; }

        // artwork (Base64)

        id art = np[@"kMRMediaRemoteNowPlayingInfoArtworkData"];
        if ([art isKindOfClass:[NSData class]]) {
            NSString *b64 = [(NSData *)art base64EncodedStringWithOptions:0];
            if (b64) out[@"artwork"] = b64;
        }

        // app info
        if (pidNum && pidNum.intValue > 0) {
            NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pidNum.intValue];
            if (app.bundleIdentifier) out[@"bundleIdentifier"] = app.bundleIdentifier;
            if (app.localizedName) out[@"appName"] = app.localizedName;
        }

        NSError *err = nil;
        NSData *data = [NSJSONSerialization dataWithJSONObject:out options:0 error:&err];
        if (!data || err) return NULL;
        NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        if (!json) return NULL;
        return strdup(json.UTF8String);
    }
}

void wox_mr_free(char *p) {
    if (p) free(p);
}

