//go:build linux

#include "platform_linux_pipewire.h"

#include <pipewire/pipewire.h>
#include <spa/param/video/format-utils.h>
#include <spa/param/video/raw-utils.h>

#include <stdbool.h>
#include <errno.h>
#include <dlfcn.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef struct {
  void *library;
  __typeof__(&pw_init) init;
  __typeof__(&pw_thread_loop_new) thread_loop_new;
  __typeof__(&pw_thread_loop_destroy) thread_loop_destroy;
  __typeof__(&pw_thread_loop_get_loop) thread_loop_get_loop;
  __typeof__(&pw_thread_loop_start) thread_loop_start;
  __typeof__(&pw_thread_loop_stop) thread_loop_stop;
  __typeof__(&pw_thread_loop_lock) thread_loop_lock;
  __typeof__(&pw_thread_loop_unlock) thread_loop_unlock;
  __typeof__(&pw_thread_loop_timed_wait) thread_loop_timed_wait;
  __typeof__(&pw_thread_loop_signal) thread_loop_signal;
  __typeof__(&pw_context_new) context_new;
  __typeof__(&pw_context_destroy) context_destroy;
  __typeof__(&pw_context_connect_fd) context_connect_fd;
  __typeof__(&pw_core_disconnect) core_disconnect;
  __typeof__(&pw_properties_new) properties_new;
  __typeof__(&pw_stream_new) stream_new;
  __typeof__(&pw_stream_destroy) stream_destroy;
  __typeof__(&pw_stream_add_listener) stream_add_listener;
  __typeof__(&pw_stream_connect) stream_connect;
  __typeof__(&pw_stream_dequeue_buffer) stream_dequeue_buffer;
  __typeof__(&pw_stream_queue_buffer) stream_queue_buffer;
} WoxPipeWireAPI;

static WoxPipeWireAPI pipewire_api;
static pthread_once_t pipewire_api_once = PTHREAD_ONCE_INIT;

static void load_pipewire_api(void) {
  pipewire_api.library = dlopen("libpipewire-0.3.so.0", RTLD_NOW | RTLD_LOCAL);
  if (pipewire_api.library == NULL) {
    return;
  }
#define LOAD_PIPEWIRE_SYMBOL(field, name) *(void **)(&pipewire_api.field) = dlsym(pipewire_api.library, name)
  LOAD_PIPEWIRE_SYMBOL(init, "pw_init");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_new, "pw_thread_loop_new");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_destroy, "pw_thread_loop_destroy");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_get_loop, "pw_thread_loop_get_loop");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_start, "pw_thread_loop_start");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_stop, "pw_thread_loop_stop");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_lock, "pw_thread_loop_lock");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_unlock, "pw_thread_loop_unlock");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_timed_wait, "pw_thread_loop_timed_wait");
  LOAD_PIPEWIRE_SYMBOL(thread_loop_signal, "pw_thread_loop_signal");
  LOAD_PIPEWIRE_SYMBOL(context_new, "pw_context_new");
  LOAD_PIPEWIRE_SYMBOL(context_destroy, "pw_context_destroy");
  LOAD_PIPEWIRE_SYMBOL(context_connect_fd, "pw_context_connect_fd");
  LOAD_PIPEWIRE_SYMBOL(core_disconnect, "pw_core_disconnect");
  LOAD_PIPEWIRE_SYMBOL(properties_new, "pw_properties_new");
  LOAD_PIPEWIRE_SYMBOL(stream_new, "pw_stream_new");
  LOAD_PIPEWIRE_SYMBOL(stream_destroy, "pw_stream_destroy");
  LOAD_PIPEWIRE_SYMBOL(stream_add_listener, "pw_stream_add_listener");
  LOAD_PIPEWIRE_SYMBOL(stream_connect, "pw_stream_connect");
  LOAD_PIPEWIRE_SYMBOL(stream_dequeue_buffer, "pw_stream_dequeue_buffer");
  LOAD_PIPEWIRE_SYMBOL(stream_queue_buffer, "pw_stream_queue_buffer");
#undef LOAD_PIPEWIRE_SYMBOL

  if (pipewire_api.init == NULL || pipewire_api.thread_loop_new == NULL || pipewire_api.thread_loop_destroy == NULL ||
      pipewire_api.thread_loop_get_loop == NULL || pipewire_api.thread_loop_start == NULL || pipewire_api.thread_loop_stop == NULL ||
      pipewire_api.thread_loop_lock == NULL || pipewire_api.thread_loop_unlock == NULL || pipewire_api.thread_loop_timed_wait == NULL ||
      pipewire_api.thread_loop_signal == NULL || pipewire_api.context_new == NULL || pipewire_api.context_destroy == NULL ||
      pipewire_api.context_connect_fd == NULL || pipewire_api.core_disconnect == NULL || pipewire_api.properties_new == NULL ||
      pipewire_api.stream_new == NULL || pipewire_api.stream_destroy == NULL || pipewire_api.stream_add_listener == NULL ||
      pipewire_api.stream_connect == NULL || pipewire_api.stream_dequeue_buffer == NULL || pipewire_api.stream_queue_buffer == NULL) {
    dlclose(pipewire_api.library);
    memset(&pipewire_api, 0, sizeof(pipewire_api));
    return;
  }
  pipewire_api.init(NULL, NULL);
}

#define pw_init pipewire_api.init
#define pw_thread_loop_new pipewire_api.thread_loop_new
#define pw_thread_loop_destroy pipewire_api.thread_loop_destroy
#define pw_thread_loop_get_loop pipewire_api.thread_loop_get_loop
#define pw_thread_loop_start pipewire_api.thread_loop_start
#define pw_thread_loop_stop pipewire_api.thread_loop_stop
#define pw_thread_loop_lock pipewire_api.thread_loop_lock
#define pw_thread_loop_unlock pipewire_api.thread_loop_unlock
#define pw_thread_loop_timed_wait pipewire_api.thread_loop_timed_wait
#define pw_thread_loop_signal pipewire_api.thread_loop_signal
#define pw_context_new pipewire_api.context_new
#define pw_context_destroy pipewire_api.context_destroy
#define pw_context_connect_fd pipewire_api.context_connect_fd
#define pw_core_disconnect pipewire_api.core_disconnect
#define pw_properties_new pipewire_api.properties_new
#define pw_stream_new pipewire_api.stream_new
#define pw_stream_destroy pipewire_api.stream_destroy
#define pw_stream_add_listener pipewire_api.stream_add_listener
#define pw_stream_connect pipewire_api.stream_connect
#define pw_stream_dequeue_buffer pipewire_api.stream_dequeue_buffer
#define pw_stream_queue_buffer pipewire_api.stream_queue_buffer

typedef struct {
  WoxPipeWireCapture *capture;
  struct pw_stream *stream;
  struct spa_hook listener;
  struct spa_video_info_raw format;
  WoxPipeWireFrame *output;
  bool failed;
} WoxPipeWireStream;

struct WoxPipeWireCapture {
  struct pw_thread_loop *loop;
  struct pw_context *context;
  struct pw_core *core;
  WoxPipeWireStream *streams;
  int32_t stream_count;
  int32_t ready_count;
  bool failed;
};

static int32_t pipewire_pixel_size(enum spa_video_format format) {
  switch (format) {
    case SPA_VIDEO_FORMAT_RGB:
    case SPA_VIDEO_FORMAT_BGR:
      return 3;
    case SPA_VIDEO_FORMAT_RGBx:
    case SPA_VIDEO_FORMAT_BGRx:
    case SPA_VIDEO_FORMAT_xRGB:
    case SPA_VIDEO_FORMAT_xBGR:
    case SPA_VIDEO_FORMAT_RGBA:
    case SPA_VIDEO_FORMAT_BGRA:
    case SPA_VIDEO_FORMAT_ARGB:
    case SPA_VIDEO_FORMAT_ABGR:
      return 4;
    default:
      return 0;
  }
}

static void pipewire_copy_pixel(enum spa_video_format format, const uint8_t *source, uint8_t *target) {
  switch (format) {
    case SPA_VIDEO_FORMAT_BGR:
    case SPA_VIDEO_FORMAT_BGRx:
      target[0] = source[2];
      target[1] = source[1];
      target[2] = source[0];
      target[3] = 255;
      break;
    case SPA_VIDEO_FORMAT_BGRA:
      target[0] = source[2];
      target[1] = source[1];
      target[2] = source[0];
      target[3] = source[3];
      break;
    case SPA_VIDEO_FORMAT_xBGR:
      target[0] = source[3];
      target[1] = source[2];
      target[2] = source[1];
      target[3] = 255;
      break;
    case SPA_VIDEO_FORMAT_ABGR:
      target[0] = source[3];
      target[1] = source[2];
      target[2] = source[1];
      target[3] = source[0];
      break;
    case SPA_VIDEO_FORMAT_xRGB:
      target[0] = source[1];
      target[1] = source[2];
      target[2] = source[3];
      target[3] = 255;
      break;
    case SPA_VIDEO_FORMAT_ARGB:
      target[0] = source[1];
      target[1] = source[2];
      target[2] = source[3];
      target[3] = source[0];
      break;
    case SPA_VIDEO_FORMAT_RGBA:
      target[0] = source[0];
      target[1] = source[1];
      target[2] = source[2];
      target[3] = source[3];
      break;
    case SPA_VIDEO_FORMAT_RGB:
    case SPA_VIDEO_FORMAT_RGBx:
    default:
      target[0] = source[0];
      target[1] = source[1];
      target[2] = source[2];
      target[3] = 255;
      break;
  }
}

static void pipewire_stream_failed(WoxPipeWireStream *stream) {
  if (stream->failed) {
    return;
  }
  stream->failed = true;
  stream->capture->failed = true;
  pw_thread_loop_signal(stream->capture->loop, false);
}

static void pipewire_stream_state_changed(void *data, enum pw_stream_state old_state, enum pw_stream_state state, const char *error) {
  (void)old_state;
  (void)error;
  WoxPipeWireStream *stream = data;
  if (state == PW_STREAM_STATE_ERROR || state == PW_STREAM_STATE_UNCONNECTED) {
    pipewire_stream_failed(stream);
  }
}

static void pipewire_stream_param_changed(void *data, uint32_t id, const struct spa_pod *param) {
  WoxPipeWireStream *stream = data;
  if (id != SPA_PARAM_Format || param == NULL) {
    return;
  }
  if (spa_format_video_raw_parse(param, &stream->format) < 0 || pipewire_pixel_size(stream->format.format) == 0) {
    pipewire_stream_failed(stream);
  }
}

static void pipewire_stream_process(void *data) {
  WoxPipeWireStream *stream = data;
  struct pw_buffer *pipewire_buffer = pw_stream_dequeue_buffer(stream->stream);
  if (pipewire_buffer == NULL) {
    return;
  }

  struct spa_buffer *buffer = pipewire_buffer->buffer;
  if (stream->output != NULL && stream->output->pixels == NULL && !stream->failed && buffer != NULL && buffer->n_datas > 0) {
    struct spa_data *plane = &buffer->datas[0];
    struct spa_chunk *chunk = plane->chunk;
    const int32_t pixel_size = pipewire_pixel_size(stream->format.format);
    const uint32_t width = stream->format.size.width;
    const uint32_t height = stream->format.size.height;
    const int32_t source_stride = chunk != NULL && chunk->stride != 0 ? chunk->stride : (int32_t)(width * (uint32_t)pixel_size);
    const uint32_t absolute_stride = source_stride < 0 ? (uint32_t)(-source_stride) : (uint32_t)source_stride;
    const uint32_t offset = chunk != NULL ? chunk->offset : 0;
    const uint64_t required_size = height > 0 ? (uint64_t)offset + (uint64_t)(height - 1) * absolute_stride + (uint64_t)width * (uint32_t)pixel_size : 0;

    if (plane->data == NULL || pixel_size == 0 || width == 0 || height == 0 || absolute_stride < width * (uint32_t)pixel_size || required_size > plane->maxsize) {
      pipewire_stream_failed(stream);
    } else {
      const uint32_t target_stride = width * 4;
      uint8_t *pixels = malloc((size_t)target_stride * height);
      if (pixels == NULL) {
        pipewire_stream_failed(stream);
      } else {
        const uint8_t *source_base = (const uint8_t *)plane->data + offset;
        if (source_stride < 0) {
          source_base += (size_t)(height - 1) * absolute_stride;
        }
        for (uint32_t y = 0; y < height; y++) {
          const uint8_t *source_row = source_stride < 0 ? source_base - (size_t)y * absolute_stride : source_base + (size_t)y * absolute_stride;
          uint8_t *target_row = pixels + (size_t)y * target_stride;
          for (uint32_t x = 0; x < width; x++) {
            pipewire_copy_pixel(stream->format.format, source_row + (size_t)x * (uint32_t)pixel_size, target_row + (size_t)x * 4);
          }
        }
        stream->output->width = width;
        stream->output->height = height;
        stream->output->stride = target_stride;
        stream->output->pixels = pixels;
        stream->capture->ready_count++;
        pw_thread_loop_signal(stream->capture->loop, false);
      }
    }
  }

  pw_stream_queue_buffer(stream->stream, pipewire_buffer);
}

static const struct pw_stream_events pipewire_stream_events = {
    PW_VERSION_STREAM_EVENTS,
    .state_changed = pipewire_stream_state_changed,
    .param_changed = pipewire_stream_param_changed,
    .process = pipewire_stream_process,
};

void wox_screenshot_pipewire_free_frames(WoxPipeWireFrame *frames, int32_t frame_count) {
  if (frames == NULL || frame_count <= 0) {
    return;
  }
  for (int32_t index = 0; index < frame_count; index++) {
    free(frames[index].pixels);
    frames[index].pixels = NULL;
  }
}

WoxPipeWireCapture *wox_screenshot_pipewire_create(int32_t remote_fd, const uint32_t *node_ids, int32_t node_count) {
  if (remote_fd < 0 || node_ids == NULL || node_count <= 0) {
    if (remote_fd >= 0) {
      close(remote_fd);
    }
    return NULL;
  }

  pthread_once(&pipewire_api_once, load_pipewire_api);
  if (pipewire_api.library == NULL) {
    close(remote_fd);
    return NULL;
  }
  WoxPipeWireCapture *capture = calloc(1, sizeof(WoxPipeWireCapture));
  if (capture == NULL) {
    close(remote_fd);
    return NULL;
  }
  capture->stream_count = node_count;
  capture->streams = calloc((size_t)node_count, sizeof(WoxPipeWireStream));
  capture->loop = pw_thread_loop_new("wox-screenshot", NULL);
  if (capture->streams == NULL || capture->loop == NULL) {
    close(remote_fd);
    free(capture->streams);
    if (capture->loop != NULL) {
      pw_thread_loop_destroy(capture->loop);
    }
    free(capture);
    return NULL;
  }

  struct pw_context *context = pw_context_new(pw_thread_loop_get_loop(capture->loop), NULL, 0);
  if (context == NULL) {
    close(remote_fd);
    pw_thread_loop_destroy(capture->loop);
    free(capture->streams);
    free(capture);
    return NULL;
  }
  struct pw_core *core = pw_context_connect_fd(context, remote_fd, NULL, 0);
  if (core == NULL || pw_thread_loop_start(capture->loop) < 0) {
    if (core != NULL) {
      pw_core_disconnect(core);
    }
    if (context != NULL) {
      pw_context_destroy(context);
    }
    pw_thread_loop_destroy(capture->loop);
    free(capture->streams);
    free(capture);
    return NULL;
  }
  capture->context = context;
  capture->core = core;

  pw_thread_loop_lock(capture->loop);
  for (int32_t index = 0; index < node_count; index++) {
    WoxPipeWireStream *stream = &capture->streams[index];
    stream->capture = capture;
    struct pw_properties *properties = pw_properties_new(
        PW_KEY_MEDIA_TYPE, "Video",
        PW_KEY_MEDIA_CATEGORY, "Capture",
        PW_KEY_MEDIA_ROLE, "Screen",
        NULL);
    stream->stream = pw_stream_new(core, "Wox screenshot", properties);
    if (stream->stream == NULL) {
      capture->failed = true;
      break;
    }
    pw_stream_add_listener(stream->stream, &stream->listener, &pipewire_stream_events, stream);

    uint8_t pod_buffer[1024];
    struct spa_pod_builder builder = SPA_POD_BUILDER_INIT(pod_buffer, sizeof(pod_buffer));
    const struct spa_pod *params[1];
    params[0] = spa_pod_builder_add_object(
        &builder,
        SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
        SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
        SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
        SPA_FORMAT_VIDEO_format, SPA_POD_CHOICE_ENUM_Id(10,
            SPA_VIDEO_FORMAT_BGRx,
            SPA_VIDEO_FORMAT_BGRA,
            SPA_VIDEO_FORMAT_RGBx,
            SPA_VIDEO_FORMAT_RGBA,
            SPA_VIDEO_FORMAT_xRGB,
            SPA_VIDEO_FORMAT_ARGB,
            SPA_VIDEO_FORMAT_xBGR,
            SPA_VIDEO_FORMAT_ABGR,
            SPA_VIDEO_FORMAT_RGB,
            SPA_VIDEO_FORMAT_BGR));
    enum pw_stream_flags flags = PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS;
    if (pw_stream_connect(stream->stream, PW_DIRECTION_INPUT, node_ids[index], flags, params, 1) < 0) {
      capture->failed = true;
      break;
    }
  }

  pw_thread_loop_unlock(capture->loop);
  if (capture->failed) {
    wox_screenshot_pipewire_destroy(capture);
    return NULL;
  }
  return capture;
}

int32_t wox_screenshot_pipewire_capture(WoxPipeWireCapture *capture, WoxPipeWireFrame *frames, int32_t frame_count, int32_t timeout_seconds) {
  if (capture == NULL || frames == NULL || frame_count != capture->stream_count || timeout_seconds <= 0) {
    return -1;
  }

  pw_thread_loop_lock(capture->loop);
  if (capture->failed) {
    pw_thread_loop_unlock(capture->loop);
    return -2;
  }
  capture->ready_count = 0;
  for (int32_t index = 0; index < frame_count; index++) {
    memset(&frames[index], 0, sizeof(WoxPipeWireFrame));
    capture->streams[index].output = &frames[index];
  }

  int32_t result = 0;
  while (!capture->failed && capture->ready_count < frame_count) {
    int wait_result = pw_thread_loop_timed_wait(capture->loop, timeout_seconds);
    if (wait_result == -ETIMEDOUT) {
      result = -4;
      break;
    }
    if (wait_result < 0) {
      result = -5;
      break;
    }
  }
  if (capture->failed && result == 0) {
    result = -6;
  }
  for (int32_t index = 0; index < frame_count; index++) {
    capture->streams[index].output = NULL;
  }
  pw_thread_loop_unlock(capture->loop);

  if (result != 0) {
    wox_screenshot_pipewire_free_frames(frames, frame_count);
  }
  return result;
}

void wox_screenshot_pipewire_destroy(WoxPipeWireCapture *capture) {
  if (capture == NULL) {
    return;
  }
  pw_thread_loop_lock(capture->loop);
  for (int32_t index = 0; index < capture->stream_count; index++) {
    if (capture->streams[index].stream != NULL) {
      pw_stream_destroy(capture->streams[index].stream);
    }
  }
  pw_thread_loop_unlock(capture->loop);
  pw_thread_loop_stop(capture->loop);
  if (capture->core != NULL) {
    pw_core_disconnect(capture->core);
  }
  if (capture->context != NULL) {
    pw_context_destroy(capture->context);
  }
  pw_thread_loop_destroy(capture->loop);
  free(capture->streams);
  free(capture);
}
