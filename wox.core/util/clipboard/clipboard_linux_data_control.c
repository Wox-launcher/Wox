//go:build linux

#include "clipboard_linux_data_control.h"

#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <wayland-client.h>

#include "ext-data-control-v1-client-protocol.h"

#define WOX_DATA_CONTROL_MAX_BYTES (64U * 1024U * 1024U)
#define WOX_DATA_CONTROL_READ_TIMEOUT_MS 2000

typedef struct WoxDataControlOffer {
    struct ext_data_control_offer_v1 *offer;
    char **mime_types;
    size_t mime_count;
    struct WoxDataControlOffer *next;
} WoxDataControlOffer;

typedef struct {
    struct wl_display *display;
    struct wl_registry *registry;
    struct wl_seat *seat;
    struct ext_data_control_manager_v1 *manager;
    struct ext_data_control_device_v1 *device;
    WoxDataControlOffer *offers;
    WoxDataControlOffer *selection;
    int finished;
} WoxDataControlState;

static void wox_data_control_set_error(WoxDataControlReadResult *result, const char *format, ...) {
    va_list args;
    va_start(args, format);
    if (vasprintf(&result->error, format, args) < 0) {
        result->error = NULL;
    }
    va_end(args);
}

static WoxDataControlOffer *wox_data_control_find_offer(
    WoxDataControlState *state,
    struct ext_data_control_offer_v1 *offer) {
    for (WoxDataControlOffer *item = state->offers; item != NULL; item = item->next) {
        if (item->offer == offer) {
            return item;
        }
    }
    return NULL;
}

static void wox_data_control_offer_mime(
    void *data,
    struct ext_data_control_offer_v1 *offer,
    const char *mime_type) {
    (void)offer;
    WoxDataControlOffer *item = data;
    char **mime_types = realloc(item->mime_types, (item->mime_count + 1) * sizeof(char *));
    if (mime_types == NULL) {
        return;
    }
    item->mime_types = mime_types;
    item->mime_types[item->mime_count] = strdup(mime_type);
    if (item->mime_types[item->mime_count] != NULL) {
        item->mime_count++;
    }
}

static const struct ext_data_control_offer_v1_listener wox_data_control_offer_listener = {
    .offer = wox_data_control_offer_mime,
};

static void wox_data_control_device_offer(
    void *data,
    struct ext_data_control_device_v1 *device,
    struct ext_data_control_offer_v1 *offer) {
    (void)device;
    WoxDataControlState *state = data;
    WoxDataControlOffer *item = calloc(1, sizeof(WoxDataControlOffer));
    if (item == NULL) {
        ext_data_control_offer_v1_destroy(offer);
        return;
    }
    item->offer = offer;
    item->next = state->offers;
    state->offers = item;
    ext_data_control_offer_v1_add_listener(offer, &wox_data_control_offer_listener, item);
}

static void wox_data_control_device_selection(
    void *data,
    struct ext_data_control_device_v1 *device,
    struct ext_data_control_offer_v1 *offer) {
    (void)device;
    WoxDataControlState *state = data;
    state->selection = offer == NULL ? NULL : wox_data_control_find_offer(state, offer);
}

static void wox_data_control_device_finished(
    void *data,
    struct ext_data_control_device_v1 *device) {
    (void)device;
    WoxDataControlState *state = data;
    state->finished = 1;
}

static void wox_data_control_device_primary_selection(
    void *data,
    struct ext_data_control_device_v1 *device,
    struct ext_data_control_offer_v1 *offer) {
    (void)data;
    (void)device;
    (void)offer;
}

static const struct ext_data_control_device_v1_listener wox_data_control_device_listener = {
    .data_offer = wox_data_control_device_offer,
    .selection = wox_data_control_device_selection,
    .finished = wox_data_control_device_finished,
    .primary_selection = wox_data_control_device_primary_selection,
};

static void wox_data_control_registry_global(
    void *data,
    struct wl_registry *registry,
    uint32_t name,
    const char *interface,
    uint32_t version) {
    WoxDataControlState *state = data;
    if (strcmp(interface, ext_data_control_manager_v1_interface.name) == 0) {
        state->manager = wl_registry_bind(
            registry,
            name,
            &ext_data_control_manager_v1_interface,
            version < 1 ? version : 1);
        return;
    }
    if (state->seat == NULL && strcmp(interface, wl_seat_interface.name) == 0) {
        uint32_t seat_version = version < 5 ? version : 5;
        state->seat = wl_registry_bind(registry, name, &wl_seat_interface, seat_version);
    }
}

static void wox_data_control_registry_global_remove(
    void *data,
    struct wl_registry *registry,
    uint32_t name) {
    (void)data;
    (void)registry;
    (void)name;
}

static const struct wl_registry_listener wox_data_control_registry_listener = {
    .global = wox_data_control_registry_global,
    .global_remove = wox_data_control_registry_global_remove,
};

static const char *wox_data_control_choose_mime(const WoxDataControlOffer *offer) {
    static const char *preferred[] = {
        "text/uri-list",
        "image/png",
        "text/plain;charset=utf-8",
        "text/plain",
        "UTF8_STRING",
    };
    for (size_t candidate = 0; candidate < sizeof(preferred) / sizeof(preferred[0]); candidate++) {
        for (size_t i = 0; i < offer->mime_count; i++) {
            if (strcasecmp(offer->mime_types[i], preferred[candidate]) == 0) {
                return offer->mime_types[i];
            }
        }
    }
    return NULL;
}

static int wox_data_control_append(
    WoxDataControlReadResult *result,
    const uint8_t *data,
    size_t size) {
    if (size > WOX_DATA_CONTROL_MAX_BYTES - result->size) {
        wox_data_control_set_error(result, "clipboard payload exceeds %u bytes", WOX_DATA_CONTROL_MAX_BYTES);
        return -1;
    }
    uint8_t *buffer = realloc(result->data, result->size + size);
    if (buffer == NULL && size > 0) {
        wox_data_control_set_error(result, "failed to allocate clipboard payload buffer");
        return -1;
    }
    result->data = buffer;
    memcpy(result->data + result->size, data, size);
    result->size += size;
    return 0;
}

static int wox_data_control_receive(
    WoxDataControlState *state,
    const char *mime_type,
    WoxDataControlReadResult *result) {
    int pipe_fds[2];
    if (pipe2(pipe_fds, O_CLOEXEC) != 0) {
        wox_data_control_set_error(result, "failed to create clipboard transfer pipe: %s", strerror(errno));
        return -1;
    }

    ext_data_control_offer_v1_receive(state->selection->offer, mime_type, pipe_fds[1]);
    close(pipe_fds[1]);
    if (wl_display_flush(state->display) < 0 && errno != EAGAIN) {
        wox_data_control_set_error(result, "failed to flush clipboard receive request: %s", strerror(errno));
        close(pipe_fds[0]);
        return -1;
    }

    int flags = fcntl(pipe_fds[0], F_GETFL, 0);
    if (flags >= 0) {
        (void)fcntl(pipe_fds[0], F_SETFL, flags | O_NONBLOCK);
    }

    uint8_t buffer[16384];
    for (;;) {
        struct pollfd poll_fd = {
            .fd = pipe_fds[0],
            .events = POLLIN | POLLHUP,
        };
        int poll_result = poll(&poll_fd, 1, WOX_DATA_CONTROL_READ_TIMEOUT_MS);
        if (poll_result == 0) {
            wox_data_control_set_error(result, "timed out reading clipboard data");
            close(pipe_fds[0]);
            return -1;
        }
        if (poll_result < 0) {
            if (errno == EINTR) {
                continue;
            }
            wox_data_control_set_error(result, "failed waiting for clipboard data: %s", strerror(errno));
            close(pipe_fds[0]);
            return -1;
        }

        ssize_t read_size = read(pipe_fds[0], buffer, sizeof(buffer));
        if (read_size > 0) {
            if (wox_data_control_append(result, buffer, (size_t)read_size) != 0) {
                close(pipe_fds[0]);
                return -1;
            }
            continue;
        }
        if (read_size == 0) {
            break;
        }
        if (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR) {
            continue;
        }
        wox_data_control_set_error(result, "failed reading clipboard data: %s", strerror(errno));
        close(pipe_fds[0]);
        return -1;
    }

    close(pipe_fds[0]);
    return 0;
}

static void wox_data_control_state_destroy(WoxDataControlState *state) {
    WoxDataControlOffer *offer = state->offers;
    while (offer != NULL) {
        WoxDataControlOffer *next = offer->next;
        if (offer->offer != NULL) {
            ext_data_control_offer_v1_destroy(offer->offer);
        }
        for (size_t i = 0; i < offer->mime_count; i++) {
            free(offer->mime_types[i]);
        }
        free(offer->mime_types);
        free(offer);
        offer = next;
    }
    if (state->device != NULL) {
        ext_data_control_device_v1_destroy(state->device);
    }
    if (state->manager != NULL) {
        ext_data_control_manager_v1_destroy(state->manager);
    }
    if (state->seat != NULL) {
        wl_seat_destroy(state->seat);
    }
    if (state->registry != NULL) {
        wl_registry_destroy(state->registry);
    }
    if (state->display != NULL) {
        wl_display_disconnect(state->display);
    }
}

int wox_data_control_read(WoxDataControlReadResult *result) {
    memset(result, 0, sizeof(*result));
    WoxDataControlState state = {0};

    state.display = wl_display_connect(NULL);
    if (state.display == NULL) {
        wox_data_control_set_error(result, "failed to connect to the Wayland display");
        return -1;
    }
    state.registry = wl_display_get_registry(state.display);
    wl_registry_add_listener(state.registry, &wox_data_control_registry_listener, &state);
    if (wl_display_roundtrip(state.display) < 0) {
        wox_data_control_set_error(result, "failed to enumerate Wayland globals");
        wox_data_control_state_destroy(&state);
        return -1;
    }
    if (state.manager == NULL) {
        wox_data_control_set_error(result, "ext-data-control-v1 is not available");
        wox_data_control_state_destroy(&state);
        return -1;
    }
    if (state.seat == NULL) {
        wox_data_control_set_error(result, "Wayland seat is not available");
        wox_data_control_state_destroy(&state);
        return -1;
    }

    state.device = ext_data_control_manager_v1_get_data_device(state.manager, state.seat);
    ext_data_control_device_v1_add_listener(state.device, &wox_data_control_device_listener, &state);
    if (wl_display_roundtrip(state.display) < 0 || state.finished) {
        wox_data_control_set_error(result, "failed to initialize ext-data-control-v1 clipboard access");
        wox_data_control_state_destroy(&state);
        return -1;
    }
    if (state.selection == NULL) {
        wox_data_control_state_destroy(&state);
        return 1;
    }

    const char *mime_type = wox_data_control_choose_mime(state.selection);
    if (mime_type == NULL) {
        wox_data_control_state_destroy(&state);
        return 1;
    }
    result->mime_type = strdup(mime_type);
    if (result->mime_type == NULL) {
        wox_data_control_set_error(result, "failed to allocate clipboard MIME type");
        wox_data_control_state_destroy(&state);
        return -1;
    }

    int receive_result = wox_data_control_receive(&state, mime_type, result);
    wox_data_control_state_destroy(&state);
    if (receive_result != 0) {
        return -1;
    }
    return 0;
}

void wox_data_control_read_result_free(WoxDataControlReadResult *result) {
    if (result == NULL) {
        return;
    }
    free(result->mime_type);
    free(result->data);
    free(result->error);
    memset(result, 0, sizeof(*result));
}
