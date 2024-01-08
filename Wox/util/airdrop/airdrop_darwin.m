#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

void sendFilesViaAirDrop(const char **filePaths, int count) {
    @try {
        NSMutableArray<NSURL *> *fileURLs = [[NSMutableArray alloc] initWithCapacity:count];
        for (int i = 0; i < count; i++) {
            NSString *path = [NSString stringWithUTF8String:filePaths[i]];
            NSURL *url = [NSURL fileURLWithPath:path];
            if (url != nil) {
                [fileURLs addObject:url];
            } else {
                NSLog(@"Invalid file path: %s", filePaths[i]);
            }
        }

        NSSharingService *sharingService = [NSSharingService sharingServiceNamed:NSSharingServiceNameSendViaAirDrop];
        if (sharingService != nil && fileURLs.count > 0) {
            [sharingService performWithItems:fileURLs];
        } else {
            NSLog(@"Unable to create sharing service or no valid files to share.");
        }
    } @catch (NSException *exception) {
        NSLog(@"Exception in sendFilesViaAirDrop: %@", exception);
    }
}
