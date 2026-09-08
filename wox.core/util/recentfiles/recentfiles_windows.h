#ifndef WOX_RECENTFILES_WINDOWS_H
#define WOX_RECENTFILES_WINDOWS_H

#include <wchar.h>

// Resolves a .lnk without IShellLink::Resolve so network targets cannot block.
int wox_recent_resolve_lnk(const wchar_t *lnk, wchar_t *out, int out_chars);
void wox_recent_record_access(const wchar_t *path);
// Reads one OLE compound-file stream. Caller frees *out with wox_recent_free.
int wox_recent_read_ole_stream(const wchar_t *storage_path, const wchar_t *stream_name, unsigned char **out, int *out_len);
void wox_recent_free(void *ptr);

#endif
