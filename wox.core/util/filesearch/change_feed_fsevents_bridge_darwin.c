#include <CoreServices/CoreServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>

extern void woxFSEventsCallback(uintptr_t handle, size_t numEvents, char **eventPaths, FSEventStreamEventFlags *eventFlags, FSEventStreamEventId *eventIds);

void woxFSEventsBridge(ConstFSEventStreamRef streamRef, void *clientCallBackInfo, size_t numEvents, void *eventPaths, const FSEventStreamEventFlags eventFlags[], const FSEventStreamEventId eventIds[]) {
	uintptr_t handle = *((uintptr_t *)clientCallBackInfo);
	woxFSEventsCallback(handle, numEvents, (char **)eventPaths, (FSEventStreamEventFlags *)eventFlags, (FSEventStreamEventId *)eventIds);
}
