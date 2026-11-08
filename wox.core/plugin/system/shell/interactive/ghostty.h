#pragma once

#include <ghostty/vt.h>

typedef struct WoxGhostty WoxGhostty;

typedef struct {
    uint32_t fg, bg;
    GhosttyStyleFlags style;
    GhosttyCellWide width;
    uint8_t length;
    uint8_t text[64];
} WoxGhosttyCell;

WoxGhostty *wox_ghostty_new(uint16_t cols, uint16_t rows);
void wox_ghostty_free(WoxGhostty *model);
GhosttyResult wox_ghostty_resize(WoxGhostty *model, uint16_t cols, uint16_t rows, uint32_t width, uint32_t height);
const uint8_t *wox_ghostty_write(WoxGhostty *model, const uint8_t *bytes, size_t length, size_t *reply_length);
size_t wox_ghostty_key(WoxGhostty *model, GhosttyKey key, uint32_t codepoint, GhosttyMods mods, bool repeat, uint8_t *out, size_t capacity);
size_t wox_ghostty_paste(WoxGhostty *model, char *data, size_t length, uint8_t *out, size_t capacity);
void wox_ghostty_scroll(WoxGhostty *model, int rows);
bool wox_ghostty_frame(WoxGhostty *model, WoxGhosttyCell *grid, uint16_t cols, uint16_t rows, GhosttyRenderStateCursor *cursor);
