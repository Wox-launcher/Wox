#ifndef WOX_CLIPBOARD_LINUX_DATA_CONTROL_H
#define WOX_CLIPBOARD_LINUX_DATA_CONTROL_H

#include <stddef.h>
#include <stdint.h>

typedef struct {
    char *mime_type;
    uint8_t *data;
    size_t size;
    char *error;
} WoxDataControlReadResult;

typedef struct WoxDataControlState WoxDataControlState;
WoxDataControlState *wox_data_control_watch_start(WoxDataControlReadResult *result);
int wox_data_control_watch_next(WoxDataControlState *state, WoxDataControlReadResult *result);
void wox_data_control_watch_close(WoxDataControlState *state);

int wox_data_control_read(WoxDataControlReadResult *result);
void wox_data_control_read_result_free(WoxDataControlReadResult *result);

#endif
