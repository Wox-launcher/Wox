#import <CoreServices/CoreServices.h>
#import <CoreFoundation/CoreFoundation.h>

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

static void wox_set_error(char **outError, const char *message) {
    if (outError == NULL) {
        return;
    }

    if (message == NULL) {
        *outError = NULL;
        return;
    }

    size_t len = strlen(message);
    char *buf = (char *)malloc(len + 1);
    if (buf == NULL) {
        *outError = NULL;
        return;
    }

    memcpy(buf, message, len + 1);
    *outError = buf;
}

bool wox_mdquery_search_paths(const char *query, int maxResults, char **outPaths, char **outError) {
    if (outPaths == NULL) {
        wox_set_error(outError, "outPaths is NULL");
        return false;
    }

    *outPaths = NULL;
    if (outError != NULL) {
        *outError = NULL;
    }

    if (query == NULL) {
        wox_set_error(outError, "query is NULL");
        return false;
    }

    CFStringRef queryStr = CFStringCreateWithCString(kCFAllocatorDefault, query, kCFStringEncodingUTF8);
    if (queryStr == NULL) {
        wox_set_error(outError, "failed to create query string");
        return false;
    }

    MDQueryRef mdQuery = MDQueryCreate(kCFAllocatorDefault, queryStr, NULL, NULL);
    CFRelease(queryStr);
    if (mdQuery == NULL) {
        wox_set_error(outError, "failed to create mdquery");
        return false;
    }

    Boolean ok = MDQueryExecute(mdQuery, kMDQuerySynchronous);
    if (!ok) {
        CFRelease(mdQuery);
        wox_set_error(outError, "failed to execute mdquery");
        return false;
    }

    CFIndex count = MDQueryGetResultCount(mdQuery);
    if (maxResults > 0 && count > maxResults) {
        count = maxResults;
    }

    size_t capacity = 1024;
    size_t length = 0;
    char *buffer = (char *)malloc(capacity);
    if (buffer == NULL) {
        CFRelease(mdQuery);
        wox_set_error(outError, "failed to allocate result buffer");
        return false;
    }
    buffer[0] = '\0';

    for (CFIndex i = 0; i < count; i++) {
        MDItemRef item = (MDItemRef)MDQueryGetResultAtIndex(mdQuery, i);
        if (item == NULL) {
            continue;
        }

        CFTypeRef pathValue = MDItemCopyAttribute(item, kMDItemPath);
        if (pathValue == NULL) {
            continue;
        }
        if (CFGetTypeID(pathValue) != CFStringGetTypeID()) {
            CFRelease(pathValue);
            continue;
        }

        CFStringRef pathStr = (CFStringRef)pathValue;
        CFIndex maxPathBytes = CFStringGetMaximumSizeForEncoding(CFStringGetLength(pathStr), kCFStringEncodingUTF8) + 1;
        char *pathBuf = (char *)malloc((size_t)maxPathBytes);
        if (pathBuf == NULL) {
            CFRelease(pathValue);
            free(buffer);
            CFRelease(mdQuery);
            wox_set_error(outError, "failed to allocate path buffer");
            return false;
        }

        Boolean pathOk = CFStringGetCString(pathStr, pathBuf, maxPathBytes, kCFStringEncodingUTF8);
        CFRelease(pathValue);
        if (!pathOk) {
            free(pathBuf);
            continue;
        }

        size_t pathLen = strlen(pathBuf);
        size_t required = length + pathLen + 2;
        if (required > capacity) {
            while (capacity < required) {
                capacity *= 2;
            }

            char *newBuffer = (char *)realloc(buffer, capacity);
            if (newBuffer == NULL) {
                free(pathBuf);
                free(buffer);
                CFRelease(mdQuery);
                wox_set_error(outError, "failed to expand result buffer");
                return false;
            }
            buffer = newBuffer;
        }

        memcpy(buffer + length, pathBuf, pathLen);
        length += pathLen;
        buffer[length++] = '\n';
        buffer[length] = '\0';

        free(pathBuf);
    }

    CFRelease(mdQuery);

    if (length == 0) {
        free(buffer);
        *outPaths = NULL;
        return true;
    }

    buffer[length-1] = '\0';
    *outPaths = buffer;
    return true;
}

void wox_mdquery_free(char *ptr) {
    if (ptr != NULL) {
        free(ptr);
    }
}
