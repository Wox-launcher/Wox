#ifndef WOX_MEDIAPLAYER_WINDOWS_H
#define WOX_MEDIAPLAYER_WINDOWS_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct WoxMediaInfo {
    int has_media;
    char* title;
    char* artist;
    char* album;
    char* app_name;
    char* app_id;
    int playback_status;
    int64_t duration;
    int64_t position;
    unsigned char* artwork;
    int artwork_len;
    char* error;
} WoxMediaInfo;

WoxMediaInfo wox_get_media_info(void);
int wox_control_media(const char* command, char** error);
void wox_free_media_info(WoxMediaInfo* info);
void wox_free_string(char* value);

#ifdef __cplusplus
}
#endif

#endif
