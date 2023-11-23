#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>

// typedef void (*KeyboardEventHandler)(int keyCode);
//
// KeyboardEventHandler goKeyboardEventHandler = NULL;
//
// void RegisterKeyboardListener(KeyboardEventHandler handler) {
//     goKeyboardEventHandler = handler;
//
//     NSEventMask mask = NSEventMaskKeyDown | NSEventMaskKeyUp;
//     [NSEvent addGlobalMonitorForEventsMatchingMask:mask handler:^(NSEvent *event) {
//         if (goKeyboardEventHandler != NULL) {
//             goKeyboardEventHandler((int)[event keyCode]);
//         }
//     }];
// }