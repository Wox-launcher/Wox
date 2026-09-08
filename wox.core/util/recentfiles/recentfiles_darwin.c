#include <CoreFoundation/CoreFoundation.h>
#include <CoreServices/CoreServices.h>

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include "recentfiles_darwin.h"

static const char *kRecentPredicate =
	"kMDItemLastUsedDate >= $time.today(-365) && "
	"kMDItemFSInvisible != 1 && "
	"kMDItemContentTypeTree != com.apple.application-bundle";

bool wox_recent_files_mdquery(int limit, char **out_paths) {
	CFStringRef query_string = NULL;
	CFStringRef sort_keys[1];
	CFArrayRef sort_order = NULL;
	CFArrayRef scope = NULL;
	MDQueryRef query = NULL;
	CFIndex count;
	CFIndex i;
	size_t capacity = 1024;
	size_t length = 0;
	char *buffer = NULL;

	if (out_paths == NULL) {
		return false;
	}
	*out_paths = NULL;
	if (limit <= 0) {
		return true;
	}

	query_string = CFStringCreateWithCString(kCFAllocatorDefault, kRecentPredicate, kCFStringEncodingUTF8);
	if (query_string == NULL) {
		return false;
	}

	sort_keys[0] = kMDItemLastUsedDate;
	sort_order = CFArrayCreate(kCFAllocatorDefault, (const void **)sort_keys, 1, &kCFTypeArrayCallBacks);
	query = MDQueryCreate(kCFAllocatorDefault, query_string, NULL, sort_order);
	CFRelease(query_string);
	if (sort_order != NULL) {
		CFRelease(sort_order);
	}
	if (query == NULL) {
		return false;
	}

	MDQuerySetSortOptionFlagsForAttribute(query, kMDItemLastUsedDate, kMDQueryReverseSortOrderFlag);
	scope = CFArrayCreate(kCFAllocatorDefault, (const void **)&kMDQueryScopeHome, 1, &kCFTypeArrayCallBacks);
	if (scope != NULL) {
		MDQuerySetSearchScope(query, scope, 0);
		CFRelease(scope);
	}

	if (!MDQueryExecute(query, kMDQuerySynchronous)) {
		CFRelease(query);
		return false;
	}

	buffer = (char *)malloc(capacity);
	if (buffer == NULL) {
		CFRelease(query);
		return false;
	}

	count = MDQueryGetResultCount(query);
	for (i = 0; i < count && (int)i < limit; i++) {
		MDItemRef item = (MDItemRef)MDQueryGetResultAtIndex(query, i);
		CFTypeRef path_value;
		CFStringRef path_str;
		CFIndex max_path_bytes;
		char *path_buf;
		size_t path_len;
		size_t required;

		if (item == NULL) {
			continue;
		}
		path_value = MDItemCopyAttribute(item, kMDItemPath);
		if (path_value == NULL || CFGetTypeID(path_value) != CFStringGetTypeID()) {
			if (path_value != NULL) {
				CFRelease(path_value);
			}
			continue;
		}

		path_str = (CFStringRef)path_value;
		max_path_bytes = CFStringGetMaximumSizeForEncoding(CFStringGetLength(path_str), kCFStringEncodingUTF8) + 1;
		path_buf = (char *)malloc((size_t)max_path_bytes);
		if (path_buf == NULL) {
			CFRelease(path_value);
			free(buffer);
			CFRelease(query);
			return false;
		}
		if (!CFStringGetCString(path_str, path_buf, max_path_bytes, kCFStringEncodingUTF8)) {
			free(path_buf);
			CFRelease(path_value);
			continue;
		}
		CFRelease(path_value);

		path_len = strlen(path_buf);
		required = length + path_len + 2;
		if (required > capacity) {
			char *grown;

			while (capacity < required) {
				capacity *= 2;
			}
			grown = (char *)realloc(buffer, capacity);
			if (grown == NULL) {
				free(path_buf);
				free(buffer);
				CFRelease(query);
				return false;
			}
			buffer = grown;
		}
		memcpy(buffer + length, path_buf, path_len);
		length += path_len;
		buffer[length++] = '\0';
		free(path_buf);
	}

	CFRelease(query);
	if (length == 0) {
		free(buffer);
		return true;
	}
	buffer[length] = '\0';
	*out_paths = buffer;
	return true;
}

void wox_recent_files_free(char *ptr) {
	free(ptr);
}
