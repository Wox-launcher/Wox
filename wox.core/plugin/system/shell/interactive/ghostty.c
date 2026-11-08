//go:build wox_ghostty && cgo

#include "ghostty.h"
#include <stdlib.h>
#include <string.h>

struct WoxGhostty {
    GhosttyTerminal terminal;
    GhosttyRenderState render;
    GhosttyRenderStateRowIterator rows;
    GhosttyRenderStateRowCells cells;
    GhosttyKeyEncoder encoder;
    GhosttyKeyEvent key;
    uint8_t reply[65536];
    size_t reply_length;
};

static void write_reply(GhosttyTerminal terminal, void *userdata, const uint8_t *bytes, size_t length) {
    (void)terminal;
    WoxGhostty *model = userdata;
    if (length <= sizeof(model->reply) - model->reply_length) {
        memcpy(model->reply + model->reply_length, bytes, length);
        model->reply_length += length;
    }
}

WoxGhostty *wox_ghostty_new(uint16_t cols, uint16_t rows) {
    WoxGhostty *model = calloc(1, sizeof(*model));
    if (!model) return NULL;
    size_t scrollback = 1024 * 1024;
    if (ghostty_terminal_new(NULL, &model->terminal, cols, rows) != GHOSTTY_SUCCESS ||
        ghostty_terminal_set(model->terminal, GHOSTTY_TERMINAL_OPT_SCROLLBACK_MAX_BYTES, &scrollback) != GHOSTTY_SUCCESS ||
        ghostty_terminal_set(model->terminal, GHOSTTY_TERMINAL_OPT_USERDATA, model) != GHOSTTY_SUCCESS ||
        ghostty_terminal_set(model->terminal, GHOSTTY_TERMINAL_OPT_WRITE_PTY, write_reply) != GHOSTTY_SUCCESS ||
        ghostty_render_state_new(NULL, &model->render) != GHOSTTY_SUCCESS ||
        ghostty_render_state_row_iterator_new(NULL, &model->rows) != GHOSTTY_SUCCESS ||
        ghostty_render_state_row_cells_new(NULL, &model->cells) != GHOSTTY_SUCCESS ||
        ghostty_key_encoder_new(NULL, &model->encoder) != GHOSTTY_SUCCESS ||
        ghostty_key_event_new(NULL, &model->key) != GHOSTTY_SUCCESS) {
        wox_ghostty_free(model);
        return NULL;
    }
    return model;
}

void wox_ghostty_free(WoxGhostty *model) {
    if (model->key) ghostty_key_event_free(model->key);
    if (model->encoder) ghostty_key_encoder_free(model->encoder);
    if (model->cells) ghostty_render_state_row_cells_free(model->cells);
    if (model->rows) ghostty_render_state_row_iterator_free(model->rows);
    if (model->render) ghostty_render_state_free(model->render);
    if (model->terminal) ghostty_terminal_free(model->terminal);
    free(model);
}

GhosttyResult wox_ghostty_resize(WoxGhostty *model, uint16_t cols, uint16_t rows, uint32_t width, uint32_t height) {
    return ghostty_terminal_resize(model->terminal, cols, rows, width, height);
}

const uint8_t *wox_ghostty_write(WoxGhostty *model, const uint8_t *bytes, size_t length, size_t *reply_length) {
    model->reply_length = 0;
    ghostty_terminal_vt_write(model->terminal, bytes, length);
    *reply_length = model->reply_length;
    return model->reply;
}

size_t wox_ghostty_key(WoxGhostty *model, GhosttyKey key, uint32_t codepoint, GhosttyMods mods, bool repeat, uint8_t *out, size_t capacity) {
    ghostty_key_encoder_set_from_terminal(model->encoder, model->terminal);
    ghostty_key_event_set_key(model->key, key);
    ghostty_key_event_set_unshifted_codepoint(model->key, codepoint);
    ghostty_key_event_set_mods(model->key, mods);
    ghostty_key_event_set_action(model->key, repeat ? GHOSTTY_KEY_ACTION_REPEAT : GHOSTTY_KEY_ACTION_PRESS);
    size_t length = 0;
    if (ghostty_key_encoder_encode(model->encoder, model->key, out, capacity, &length) != GHOSTTY_SUCCESS) return 0;
    return length;
}

size_t wox_ghostty_paste(WoxGhostty *model, char *data, size_t length, uint8_t *out, size_t capacity) {
    GhosttyTerminalModeConfig mode = {.mode = GHOSTTY_MODE_BRACKETED_PASTE};
    ghostty_terminal_get(model->terminal, GHOSTTY_TERMINAL_DATA_MODE, &mode);
    if (!mode.value && !ghostty_paste_is_safe(data, length)) return SIZE_MAX;
    size_t written = 0;
    if (ghostty_paste_encode(data, length, mode.value, out, capacity, &written) != GHOSTTY_SUCCESS) return 0;
    return written;
}

void wox_ghostty_scroll(WoxGhostty *model, int rows) {
    GhosttyTerminalScrollViewport action = {.tag = GHOSTTY_TERMINAL_SCROLL_VIEWPORT_DELTA, .value.delta = rows};
    ghostty_terminal_scroll_viewport(model->terminal, action);
}

static uint32_t color(GhosttyRenderStateRowCells cells, GhosttyRenderStateRowCellsData key) {
    GhosttyColorRgb rgb;
    if (ghostty_render_state_row_cells_get(cells, key, &rgb) != GHOSTTY_SUCCESS) return 0;
    return (uint32_t)rgb.r << 24 | (uint32_t)rgb.g << 16 | (uint32_t)rgb.b << 8 | 255;
}

// Copy visible cells in one cgo call. Each cell owns its UTF-8 until the next frame.
bool wox_ghostty_frame(WoxGhostty *model, WoxGhosttyCell *grid, uint16_t cols, uint16_t rows, GhosttyRenderStateCursor *cursor) {
    if (ghostty_render_state_update(model->render, model->terminal) != GHOSTTY_SUCCESS) return false;
    if (ghostty_render_state_get(model->render, GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR, &model->rows) != GHOSTTY_SUCCESS) return false;
    ghostty_render_state_get(model->render, GHOSTTY_RENDER_STATE_DATA_CURSOR, cursor);
    memset(grid, 0, (size_t)cols * rows * sizeof(*grid));
    for (uint16_t y = 0; y < rows && ghostty_render_state_row_iterator_next(model->rows); y++) {
        if (ghostty_render_state_row_get(model->rows, GHOSTTY_RENDER_STATE_ROW_DATA_CELLS, &model->cells) != GHOSTTY_SUCCESS) continue;
        for (uint16_t x = 0; x < cols && ghostty_render_state_row_cells_next(model->cells); x++) {
            WoxGhosttyCell *cell = grid + (size_t)y * cols + x;
            GhosttyCell raw;
            if (ghostty_render_state_row_cells_get(model->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_RAW, &raw) == GHOSTTY_SUCCESS) {
                ghostty_cell_get(raw, GHOSTTY_CELL_DATA_WIDE, &cell->width);
            }
            GhosttyBuffer text = {.ptr = cell->text, .cap = sizeof(cell->text)};
            GhosttyResult result = ghostty_render_state_row_cells_get(model->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_UTF8, &text);
            if (result == GHOSTTY_SUCCESS) cell->length = (uint8_t)text.len;
            if (result == GHOSTTY_OUT_OF_SPACE) {
                // Bound the bridge's storage for unusually long combining sequences.
                memcpy(cell->text, "\xEF\xBF\xBD", 3);
                cell->length = 3;
            }
            GhosttyStyle style = {0};
            if (ghostty_render_state_row_cells_get(model->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_STYLE, &style) == GHOSTTY_SUCCESS) {
                cell->style = style.flags;
            }
            cell->fg = color(model->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_FG_COLOR);
            cell->bg = color(model->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_BG_COLOR);
        }
    }
    return true;
}
