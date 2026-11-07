#import <ApplicationServices/ApplicationServices.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>

extern void woxSnippetMacPointerEvent(uintptr_t callback);

typedef struct SnippetMacPointer {
    pthread_t thread;
    pthread_mutex_t mutex;
    pthread_cond_t readyCondition;
    int ready;
    uintptr_t callback;
    CFMachPortRef tap;
    CFRunLoopRef loop;
} SnippetMacPointer;

static CGEventRef snippetMacPointerCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *context) {
    SnippetMacPointer *pointer = context;
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        // Pointer events may have been missed while the tap was disabled.
        woxSnippetMacPointerEvent(pointer->callback);
        CGEventTapEnable(pointer->tap, true);
    } else {
        woxSnippetMacPointerEvent(pointer->callback);
    }
    return event;
}

static void *snippetMacPointerThread(void *arg) {
    @autoreleasepool {
        SnippetMacPointer *pointer = arg;
        CGEventMask mask = CGEventMaskBit(kCGEventLeftMouseDown) | CGEventMaskBit(kCGEventRightMouseDown) |
                           CGEventMaskBit(kCGEventOtherMouseDown) | CGEventMaskBit(kCGEventScrollWheel);
        CFMachPortRef tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly,
                                             mask, snippetMacPointerCallback, pointer);
        CFRunLoopSourceRef source = tap ? CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0) : NULL;
        pthread_mutex_lock(&pointer->mutex);
        pointer->tap = source ? tap : NULL;
        pointer->loop = source ? CFRunLoopGetCurrent() : NULL;
        pointer->ready = 1;
        pthread_cond_signal(&pointer->readyCondition);
        pthread_mutex_unlock(&pointer->mutex);
        if (!source) { if (tap) CFRelease(tap); return NULL; }
        CFRunLoopAddSource(pointer->loop, source, kCFRunLoopCommonModes);
        CGEventTapEnable(tap, true);
        CFRunLoopRun();
        CFRunLoopRemoveSource(pointer->loop, source, kCFRunLoopCommonModes);
        CFRelease(source);
        CFRelease(tap);
        return NULL;
    }
}

SnippetMacPointer *snippetMacPointerStart(uintptr_t callback) {
    SnippetMacPointer *pointer = calloc(1, sizeof(*pointer));
    if (!pointer) return NULL;
    pointer->callback = callback;
    pthread_mutex_init(&pointer->mutex, NULL);
    pthread_cond_init(&pointer->readyCondition, NULL);
    if (pthread_create(&pointer->thread, NULL, snippetMacPointerThread, pointer) != 0) {
        pthread_cond_destroy(&pointer->readyCondition);
        pthread_mutex_destroy(&pointer->mutex);
        free(pointer);
        return NULL;
    }
    pthread_mutex_lock(&pointer->mutex);
    while (!pointer->ready) pthread_cond_wait(&pointer->readyCondition, &pointer->mutex);
    int available = pointer->tap != NULL;
    pthread_mutex_unlock(&pointer->mutex);
    if (!available) {
        pthread_join(pointer->thread, NULL);
        pthread_cond_destroy(&pointer->readyCondition);
        pthread_mutex_destroy(&pointer->mutex);
        free(pointer);
        return NULL;
    }
    return pointer;
}

void snippetMacPointerStop(SnippetMacPointer *pointer) {
    if (!pointer) return;
    CFRunLoopPerformBlock(pointer->loop, kCFRunLoopCommonModes, ^{ CFRunLoopStop(pointer->loop); });
    CFRunLoopWakeUp(pointer->loop);
    pthread_join(pointer->thread, NULL);
    pthread_cond_destroy(&pointer->readyCondition);
    pthread_mutex_destroy(&pointer->mutex);
    free(pointer);
}
