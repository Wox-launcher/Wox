#ifndef WOX_RECENTFILES_DARWIN_H
#define WOX_RECENTFILES_DARWIN_H

#include <stdbool.h>

// Returns NUL-separated UTF-8 paths, double-NUL terminated. Caller frees with wox_recent_files_free.
bool wox_recent_files_mdquery(int limit, char **out_paths);
void wox_recent_files_free(char *ptr);

#endif
