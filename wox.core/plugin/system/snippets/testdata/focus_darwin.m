#import <Foundation/Foundation.h>
#include <pthread.h>
#include <stdlib.h>

int snippetMacInputSourceAllowsExpansion(void);
static int expectedInputSource;

static void *checkBlockedMain(void *unused) {
    return (void *)(intptr_t)snippetMacInputSourceAllowsExpansion();
}

static void *checkRunningMain(void *unused) {
    for (int i = 0; i < 10; i++) {
        int result = snippetMacInputSourceAllowsExpansion();
        if (result != expectedInputSource) exit(2);
    }
    dispatch_async(dispatch_get_main_queue(), ^{ exit(0); });
    return NULL;
}

int main(void) {
    @autoreleasepool {
        expectedInputSource = snippetMacInputSourceAllowsExpansion();
        pthread_t thread;
        if (pthread_create(&thread, NULL, checkBlockedMain, NULL) != 0) return 3;
        void *result = NULL;
        if (pthread_join(thread, &result) != 0 || result != NULL) return 4;
        if (pthread_create(&thread, NULL, checkRunningMain, NULL) != 0) return 5;
        dispatch_main();
    }
}
