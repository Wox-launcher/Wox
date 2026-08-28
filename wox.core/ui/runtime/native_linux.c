//go:build linux

#include "native_linux.h"

#include <gtk/gtk.h>
#include <epoxy/gl.h>
#include <pango/pangocairo.h>

#ifdef GDK_WINDOWING_WAYLAND
#include <gdk/gdkwayland.h>
#include "relative-pointer-unstable-v1-client-protocol.h"
#endif

#include "native_linux_background_effect.h"

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#endif

#include <dlfcn.h>
#include <math.h>
#include <pthread.h>
#include <stdbool.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

extern int32_t woxGoLinuxStart(uintptr_t context);
extern void woxGoLinuxCall(uintptr_t context);
extern void woxGoLinuxFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoLinuxRenderTrace(const char *message);
extern void woxGoLinuxInfo(const char *message);
extern void woxGoLinuxFocus(uintptr_t context, uint64_t epoch, int32_t active);
extern void woxGoLinuxDestroyed(uintptr_t context, uint64_t epoch, int32_t active);
extern int32_t woxGoLinuxKey(uintptr_t context, const char *key, uint8_t modifiers, int32_t down, int32_t repeat, int32_t composing);
extern void woxGoLinuxWebViewEscapeDiagnostic(uintptr_t context, const char *detail);
extern void woxGoLinuxTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoLinuxPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);
extern void woxGoLinuxObservePointer(uintptr_t context, float desktop_x, float desktop_y, int32_t inside);
extern void woxGoLinuxFileDrop(uintptr_t context, const char *paths);
extern int32_t woxGoLinuxAccessibilityAction(uintptr_t context, uint64_t node_id, const char *action, const char *value);

enum {
  WOX_KEY_MODIFIER_SHIFT = 1 << 0,
  WOX_KEY_MODIFIER_CONTROL = 1 << 1,
  WOX_KEY_MODIFIER_ALT = 1 << 2,
  WOX_KEY_MODIFIER_META = 1 << 3,
  WOX_TEXT_INPUT_COMMIT = 0,
  WOX_TEXT_INPUT_COMPOSE = 1,
  WOX_POINTER_MOVE = 0,
  WOX_POINTER_ENTER = 1,
  WOX_POINTER_LEAVE = 2,
  WOX_POINTER_DOWN = 3,
  WOX_POINTER_UP = 4,
  WOX_POINTER_SCROLL = 5,
  WOX_ACCESSIBILITY_STATE_ENABLED = 1 << 0,
  WOX_ACCESSIBILITY_STATE_FOCUSABLE = 1 << 1,
  WOX_ACCESSIBILITY_STATE_FOCUSED = 1 << 2,
  WOX_ACCESSIBILITY_STATE_SELECTED = 1 << 3,
  WOX_ACCESSIBILITY_STATE_CHECKED = 1 << 4,
  WOX_ACCESSIBILITY_STATE_EXPANDED = 1 << 5,
  WOX_ACCESSIBILITY_STATE_READ_ONLY = 1 << 6,
  WOX_ACCESSIBILITY_STATE_PROTECTED = 1 << 7,
  WOX_ACCESSIBILITY_STATE_HIDDEN = 1 << 8,
  WOX_ACCESSIBILITY_ACTION_FOCUS = 1 << 0,
  WOX_ACCESSIBILITY_ACTION_ACTIVATE = 1 << 1,
  WOX_ACCESSIBILITY_ACTION_SET_VALUE = 1 << 2,
  WOX_ACCESSIBILITY_ACTION_TOGGLE = 1 << 3,
  WOX_WINDOW_ROLE_APPLICATION = 1,
  WOX_WINDOW_ROLE_SCREENSHOT = 2,
};

enum {
  WOX_LINUX_IMAGE_CACHE_MAX = 32,
  WOX_LINUX_TEXT_CACHE_MAX = 1024,
  WOX_LINUX_TEXT_CACHE_MAX_CHARS = 256,
};

// wox_linux_resize_grip is the logical edge width used for frameless resize,
// matching windowsResizeGrip. Undecorated GTK windows have no SSD borders.
static const int wox_linux_resize_grip = 10;

static const uint64_t wox_linux_image_cache_max_bytes = 8ULL * 1024ULL * 1024ULL;
static const uint64_t wox_linux_image_cache_max_entry_bytes = 1ULL * 1024ULL * 1024ULL;
static const uint64_t wox_linux_large_image_max_bytes = 32ULL * 1024ULL * 1024ULL;
static const uint64_t wox_linux_text_cache_max_bytes = 16ULL * 1024ULL * 1024ULL;
static const uint64_t wox_linux_text_cache_max_entry_bytes = 1ULL * 1024ULL * 1024ULL;

typedef struct {
  uint64_t image_id;
  uint64_t byte_size;
  uint64_t last_used;
  uint64_t generation;
  GLuint texture;
} WoxLinuxImageCacheEntry;

typedef struct {
  uint64_t hash;
  uint64_t byte_size;
  uint64_t last_used;
  uint64_t generation;
  GLuint texture;
  float font_size;
  float scale;
  int pixel_width;
  int pixel_height;
  uint8_t font_weight;
  uint8_t italic;
  char text[WOX_LINUX_TEXT_CACHE_MAX_CHARS + 1];
  char family[64];
} WoxLinuxTextCacheEntry;

typedef struct {
  GLuint rect_program;
  GLuint texture_program;
  GLuint vertex_array;
  GLuint frame_texture;
  GLuint frame_framebuffer;
  GLint default_framebuffer;
  int frame_width;
  int frame_height;
  int damage_left;
  int damage_bottom;
  int damage_width;
  int damage_height;
  GLint rect_viewport;
  GLint rect_bounds;
  GLint rect_color;
  GLint rect_radius;
  GLint rect_stroke_width;
  GLint rect_polygon;
  GLint rect_polygon_count;
  GLint texture_viewport;
  GLint texture_bounds;
  GLint texture_color;
  GLint texture_rotation;
  GLint texture_radius;
  bool ready;
  bool frame_open;
  bool damage_active;
  bool clip_active;
  float clip_x;
  float clip_y;
  float clip_width;
  float clip_height;
  float logical_width;
  float logical_height;
  float scale;
  uint64_t context_generation;
  uint64_t last_presented_generation;
  WoxLinuxImageCacheEntry images[WOX_LINUX_IMAGE_CACHE_MAX];
  int32_t image_count;
  uint64_t image_bytes;
  uint64_t image_use_serial;
  GLuint cached_large_image;
  uint64_t cached_large_image_id;
  uint64_t cached_large_image_bytes;
  uint64_t cached_large_image_generation;
  WoxLinuxTextCacheEntry *texts;
  int32_t text_count;
  uint64_t text_bytes;
  uint64_t text_use_serial;
} WoxLinuxRenderer;

struct WoxLinuxWindow {
  GtkWidget *window;
  GtkWidget *overlay;
  GtkWidget *gl_area;
  GtkWidget *overlay_gl_area;
  GtkWidget *accessibility_layer;
  GHashTable *web_view_cache;
  GHashTable *web_view_signatures;
  GHashTable *web_view_content_keys;
  GtkWidget *active_web_view;
  char *active_web_view_key;
  char *active_web_view_signature;
  char *active_web_view_content_key;
  GtkIMContext *im_context;
  GHashTable *pressed_keys;
  WoxLinuxRenderer renderer;
  WoxLinuxRenderer overlay_renderer;
  WoxLinuxRenderer *active_renderer;
  GtkWidget *active_gl_area;
  GdkEvent *dispatching_pointer_event;
  bool embedded_surface_overlay_active;
  bool forwarding_embedded_pointer;
  uintptr_t context;
  uint64_t epoch;
  // trace_frame_id correlates GTK callbacks and native framebuffer stages with Go frame metrics.
  uint64_t trace_frame_id;
  unsigned long previous_active_window;
  float preferred_width;
  float preferred_height;
  float preferred_x;
  float preferred_y;
  float aspect_ratio;
  float min_width;
  float min_height;
  double pointer_root_x;
  double pointer_root_y;
  double pointer_client_x;
  double pointer_client_y;
  guint32 pointer_time;
  bool visible;
  bool active;
  bool animation_frame_pending;
  guint animation_tick_id;
  bool hide_on_blur;
  bool native_dialog_active;
  bool nonactivating;
  bool per_pixel_alpha;
  bool pointer_passthrough;
  bool restore_previous_on_hide;
  bool application_window;
  bool screenshot_window;
  bool topmost;
  bool layer_shell_enabled;
  void *background_effect;
  void *background_effect_surface;
  // Layer-shell surfaces are not xdg_toplevels, so gtk_window_begin_move_drag is a
  // no-op. Interactive moves rewrite overlay margins from pointer deltas instead.
  bool layer_move_active;
  bool layer_move_grabbed;
  guint layer_move_tick_id;
  void *layer_move_relative_pointer;
  float layer_move_pending_dx;
  float layer_move_pending_dy;
  double layer_move_last_local_x;
  double layer_move_last_local_y;
  bool layer_move_skip_sample;
  bool input_enabled;
  bool input_composing;
  bool active_web_view_transient;
  bool pointer_over_web_view;
  uint8_t pointer_cursor;
  // resize_cursor_active keeps the edge cursor from being overwritten by Go hover.
  bool resize_cursor_active;
  int resize_edge;
  // updating_accessibility swallows leave/enter from destroying GTK a11y
  // mirrors. Query keystrokes rebuild those widgets under the pointer and
  // would otherwise flicker the host cursor between text and default.
  bool updating_accessibility;
  char *web_view_cursor_name;
  bool has_preferred_position;
  bool presenting;
  bool rendering;
  guint invalidate_idle;
  bool closed;
  GdkRectangle input_cursor_rect;
  WoxRendererResourceStats frame_resource_stats;
  uint64_t large_image_candidate_id;
  int32_t large_image_candidate_frames;
  int64_t large_image_create_ns[16];
  int32_t large_image_create_count;
  bool cache_large_images;
};

static pthread_t wox_linux_main_thread;
static gint wox_linux_runtime_running = 0;
static gint wox_linux_loop_active = 0;
static gint wox_linux_window_count = 0;
static GList *linux_windows;
static bool wox_linux_render_trace_enabled = false;

void wox_linux_set_render_trace(int32_t enabled) {
  wox_linux_render_trace_enabled = enabled != 0;
}

// trace_linux_render keeps native diagnostics in Wox's configured log instead of stderr.
static void trace_linux_render(const char *format, ...) {
  if (!wox_linux_render_trace_enabled) {
    return;
  }
  va_list arguments;
  va_start(arguments, format);
  char *message = g_strdup_vprintf(format, arguments);
  va_end(arguments);
  if (message != NULL) {
    woxGoLinuxRenderTrace(message);
    g_free(message);
  }
}

static const char *linux_renderer_name(WoxLinuxWindow *window, WoxLinuxRenderer *renderer) {
  return window != NULL && renderer == &window->overlay_renderer ? "overlay" : "main";
}

// Must match util.LinuxDesktopAppID / util.LinuxDesktopWMClass.
#define WOX_LINUX_DEFAULT_APP_ID "io.github.WoxLauncher.Wox"
#define WOX_LINUX_DEFAULT_WM_CLASS "wox"

static char wox_linux_app_id[256];
static char wox_linux_wm_class[64];
static char wox_linux_icon_path[512];

// wox_linux_set_app_identity records the desktop id used for Wayland app_id,
// X11 WM_CLASS, and the installed icon before gtk_init.
void wox_linux_set_app_identity(const char *app_id, const char *wm_class, const char *icon_path) {
  if (app_id != NULL && app_id[0] != '\0') {
    g_strlcpy(wox_linux_app_id, app_id, sizeof(wox_linux_app_id));
  }
  if (wm_class != NULL && wm_class[0] != '\0') {
    g_strlcpy(wox_linux_wm_class, wm_class, sizeof(wox_linux_wm_class));
  }
  if (icon_path != NULL && icon_path[0] != '\0') {
    g_strlcpy(wox_linux_icon_path, icon_path, sizeof(wox_linux_icon_path));
  }
}

static const char *linux_app_id(void) {
  return wox_linux_app_id[0] != '\0' ? wox_linux_app_id : WOX_LINUX_DEFAULT_APP_ID;
}

static const char *linux_wm_class(void) {
  return wox_linux_wm_class[0] != '\0' ? wox_linux_wm_class : WOX_LINUX_DEFAULT_WM_CLASS;
}

static void apply_linux_app_identity(void) {
  // Plasma matches Wayland app_id to the desktop file name. prgname is the GTK3
  // default app_id, so a debug/AppImage basename would otherwise become a letter icon.
  g_set_prgname(linux_app_id());
  g_set_application_name("Wox");
  gdk_set_program_class(linux_wm_class());
}

static void apply_linux_app_icon(void) {
  gtk_window_set_default_icon_name(linux_app_id());
  if (wox_linux_icon_path[0] != '\0') {
    gtk_window_set_default_icon_from_file(wox_linux_icon_path, NULL);
  }
}

static void apply_wayland_app_id(WoxLinuxWindow *window) {
#ifdef GDK_WINDOWING_WAYLAND
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window == NULL || !GDK_IS_WAYLAND_WINDOW(gdk_window)) {
    return;
  }
  gdk_wayland_window_set_application_id(gdk_window, linux_app_id());
#else
  (void)window;
#endif
}

typedef GtkWidget *(*WoxWebKitViewNew)(void);
typedef GtkWidget *(*WoxWebKitViewNewWithManager)(gpointer manager);
typedef gpointer (*WoxWebKitUserContentManagerNew)(void);
typedef gpointer (*WoxWebKitUserStyleSheetNew)(const gchar *source, int injected_frames, int level, const gchar *const *allow_list, const gchar *const *block_list);
typedef void (*WoxWebKitUserContentManagerAddStyleSheet)(gpointer manager, gpointer style_sheet);
typedef void (*WoxWebKitUserStyleSheetUnref)(gpointer style_sheet);
typedef gpointer (*WoxWebKitUserScriptNew)(const gchar *source, int injected_frames, int injection_time, const gchar *const *allow_list, const gchar *const *block_list);
typedef void (*WoxWebKitUserContentManagerAddScript)(gpointer manager, gpointer script);
typedef void (*WoxWebKitUserScriptUnref)(gpointer script);
typedef gboolean (*WoxWebKitRegisterScriptMessageHandler)(gpointer manager, const gchar *name);
typedef void (*WoxWebKitViewLoadURI)(gpointer web_view, const gchar *uri);
typedef void (*WoxWebKitViewLoadHTML)(gpointer web_view, const gchar *content, const gchar *base_uri);
typedef gpointer (*WoxWebKitViewGetSettings)(gpointer web_view);
typedef gpointer (*WoxWebKitViewGetUserContentManager)(gpointer web_view);
typedef void (*WoxWebKitSettingsSetUserAgent)(gpointer settings, const gchar *user_agent);
typedef gpointer (*WoxWebKitJavascriptResultGetJSValue)(gpointer javascript_result);
typedef gchar *(*WoxJSCValueToString)(gpointer value);

typedef struct {
  void *library;
  WoxWebKitViewNew view_new;
  WoxWebKitViewNewWithManager view_new_with_manager;
  WoxWebKitUserContentManagerNew manager_new;
  WoxWebKitUserStyleSheetNew style_sheet_new;
  WoxWebKitUserContentManagerAddStyleSheet manager_add_style_sheet;
  WoxWebKitUserStyleSheetUnref style_sheet_unref;
  WoxWebKitUserScriptNew script_new;
  WoxWebKitUserContentManagerAddScript manager_add_script;
  WoxWebKitUserScriptUnref script_unref;
  WoxWebKitRegisterScriptMessageHandler register_script_message_handler;
  WoxWebKitViewLoadURI load_uri;
  WoxWebKitViewLoadHTML load_html;
  WoxWebKitViewGetSettings get_settings;
  WoxWebKitViewGetUserContentManager get_user_content_manager;
  WoxWebKitSettingsSetUserAgent set_user_agent;
  WoxWebKitJavascriptResultGetJSValue javascript_result_get_js_value;
  WoxJSCValueToString jsc_value_to_string;
  bool initialized;
  bool available;
} WoxWebKitRuntime;

static WoxWebKitRuntime wox_webkit;

static void *load_webkit_symbol(const char *name) {
  return wox_webkit.library != NULL ? dlsym(wox_webkit.library, name) : NULL;
}

// ensure_webkit keeps WebKitGTK optional at build time while using the system engine when installed.
static bool ensure_webkit(void) {
  if (wox_webkit.initialized) {
    return wox_webkit.available;
  }
  wox_webkit.initialized = true;
  const char *libraries[] = {"libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37", NULL};
  for (int index = 0; libraries[index] != NULL && wox_webkit.library == NULL; index++) {
    wox_webkit.library = dlopen(libraries[index], RTLD_NOW | RTLD_LOCAL);
  }
  if (wox_webkit.library == NULL) {
    return false;
  }
  wox_webkit.view_new = (WoxWebKitViewNew)load_webkit_symbol("webkit_web_view_new");
  wox_webkit.view_new_with_manager = (WoxWebKitViewNewWithManager)load_webkit_symbol("webkit_web_view_new_with_user_content_manager");
  wox_webkit.manager_new = (WoxWebKitUserContentManagerNew)load_webkit_symbol("webkit_user_content_manager_new");
  wox_webkit.style_sheet_new = (WoxWebKitUserStyleSheetNew)load_webkit_symbol("webkit_user_style_sheet_new");
  wox_webkit.manager_add_style_sheet = (WoxWebKitUserContentManagerAddStyleSheet)load_webkit_symbol("webkit_user_content_manager_add_style_sheet");
  wox_webkit.style_sheet_unref = (WoxWebKitUserStyleSheetUnref)load_webkit_symbol("webkit_user_style_sheet_unref");
  wox_webkit.script_new = (WoxWebKitUserScriptNew)load_webkit_symbol("webkit_user_script_new");
  wox_webkit.manager_add_script = (WoxWebKitUserContentManagerAddScript)load_webkit_symbol("webkit_user_content_manager_add_script");
  wox_webkit.script_unref = (WoxWebKitUserScriptUnref)load_webkit_symbol("webkit_user_script_unref");
  wox_webkit.register_script_message_handler = (WoxWebKitRegisterScriptMessageHandler)load_webkit_symbol("webkit_user_content_manager_register_script_message_handler");
  wox_webkit.load_uri = (WoxWebKitViewLoadURI)load_webkit_symbol("webkit_web_view_load_uri");
  wox_webkit.load_html = (WoxWebKitViewLoadHTML)load_webkit_symbol("webkit_web_view_load_html");
  wox_webkit.get_settings = (WoxWebKitViewGetSettings)load_webkit_symbol("webkit_web_view_get_settings");
  wox_webkit.get_user_content_manager = (WoxWebKitViewGetUserContentManager)load_webkit_symbol("webkit_web_view_get_user_content_manager");
  wox_webkit.set_user_agent = (WoxWebKitSettingsSetUserAgent)load_webkit_symbol("webkit_settings_set_user_agent");
  wox_webkit.javascript_result_get_js_value = (WoxWebKitJavascriptResultGetJSValue)load_webkit_symbol("webkit_javascript_result_get_js_value");
  wox_webkit.jsc_value_to_string = (WoxJSCValueToString)load_webkit_symbol("jsc_value_to_string");
  wox_webkit.available = wox_webkit.view_new != NULL && wox_webkit.load_uri != NULL && wox_webkit.load_html != NULL &&
                         wox_webkit.get_settings != NULL && wox_webkit.set_user_agent != NULL;
  if (!wox_webkit.available) {
    dlclose(wox_webkit.library);
    memset(&wox_webkit, 0, sizeof(wox_webkit));
    wox_webkit.initialized = true;
  }
  return wox_webkit.available;
}

// apply_linux_cursor_to_gdk_window walks entry/button inner surfaces that keep their own cursors.
static void apply_linux_cursor_to_gdk_window(GdkWindow *gdk_window, GdkCursor *cursor) {
  if (gdk_window == NULL) {
    return;
  }
  gdk_window_set_cursor(gdk_window, cursor);
  GList *children = gdk_window_get_children(gdk_window);
  for (GList *item = children; item != NULL; item = item->next) {
    apply_linux_cursor_to_gdk_window(GDK_WINDOW(item->data), cursor);
  }
  g_list_free(children);
}

// apply_linux_cursor_to_widget covers GTK accessibility mirrors, including GtkEntry's text window.
static void apply_linux_cursor_to_widget(GtkWidget *widget, GdkCursor *cursor) {
  if (widget == NULL) {
    return;
  }
  apply_linux_cursor_to_gdk_window(gtk_widget_get_window(widget), cursor);
  if (!GTK_IS_CONTAINER(widget)) {
    return;
  }
  GList *children = gtk_container_get_children(GTK_CONTAINER(widget));
  for (GList *item = children; item != NULL; item = item->next) {
    apply_linux_cursor_to_widget(GTK_WIDGET(item->data), cursor);
  }
  g_list_free(children);
}

// apply_linux_cursor_name mirrors one cursor across every GDK window that may own pointer input.
static int32_t apply_linux_cursor_name(WoxLinuxWindow *window, const char *cursor_name) {
  if (window == NULL || window->closed) {
    return -1;
  }
  GdkWindow *content_window = gtk_widget_get_window(window->gl_area);
  GdkWindow *overlay_window = gtk_widget_get_window(window->overlay_gl_area);
  GdkWindow *top_level_window = gtk_widget_get_window(window->window);
  GdkDisplay *display = top_level_window != NULL ? gdk_window_get_display(top_level_window) : NULL;
  if (content_window == NULL || top_level_window == NULL || display == NULL) {
    return -1;
  }
  GdkCursor *native_cursor = NULL;
  if (g_strcmp0(cursor_name, "none") == 0) {
    native_cursor = gdk_cursor_new_for_display(display, GDK_BLANK_CURSOR);
  } else {
    native_cursor = gdk_cursor_new_from_name(display, cursor_name != NULL ? cursor_name : "default");
  }
  if (native_cursor == NULL) {
    native_cursor = gdk_cursor_new_for_display(display, g_strcmp0(cursor_name, "text") == 0 ? GDK_XTERM : GDK_LEFT_PTR);
  }
  if (native_cursor == NULL) {
    return -1;
  }
  // The transparent overlay owns real pointer input, so every possible GDK target mirrors the page cursor.
  gdk_window_set_cursor(content_window, native_cursor);
  if (overlay_window != NULL && overlay_window != content_window) {
    gdk_window_set_cursor(overlay_window, native_cursor);
  }
  if (top_level_window != content_window && top_level_window != overlay_window) {
    gdk_window_set_cursor(top_level_window, native_cursor);
  }
  // GtkEntry accessibility mirrors cover the query box, including the trailing
  // window-drag strip. Without this they keep the native I-beam and fight the host cursor.
  apply_linux_cursor_to_widget(window->accessibility_layer, native_cursor);
  g_object_unref(native_cursor);
  return 0;
}

// apply_linux_pointer_cursor lets the active page cursor override the Go-rendered host cursor.
static int32_t apply_linux_pointer_cursor(WoxLinuxWindow *window) {
  if (window != NULL && window->resize_cursor_active) {
    return 0;
  }
  static const char *const host_cursor_names[] = {
      "default",
      "text",
      "move",
      "crosshair",
      "ew-resize",
      "ns-resize",
      "nwse-resize",
      "nesw-resize",
      "pointer",
  };
  uint8_t host_cursor = window->pointer_cursor;
  const char *host_cursor_name = host_cursor < sizeof(host_cursor_names) / sizeof(host_cursor_names[0]) ? host_cursor_names[host_cursor] : "default";
  const char *cursor_name = window->pointer_over_web_view && window->web_view_cursor_name != NULL
                                ? window->web_view_cursor_name
                                : host_cursor_name;
  return apply_linux_cursor_name(window, cursor_name);
}

// web_view_cursor_script reports computed CSS cursors because the transparent overlay owns native hit testing.
static const char *web_view_cursor_script(void) {
  return "(()=>{if(window.__woxCursorBridgeInstalled__)return;window.__woxCursorBridgeInstalled__=true;let last='';"
         "const allowed=new Set(['auto','default','none','context-menu','help','pointer','progress','wait','cell','crosshair','text','vertical-text','alias','copy','move','no-drop','not-allowed','grab','grabbing','all-scroll','col-resize','row-resize','n-resize','e-resize','s-resize','w-resize','ne-resize','nw-resize','se-resize','sw-resize','ew-resize','ns-resize','nesw-resize','nwse-resize','zoom-in','zoom-out']);"
         "const publish=e=>{const n=e.target&&e.target.nodeType===1?e.target:document.documentElement;if(!n)return;const raw=getComputedStyle(n).cursor||'auto';const fallback=raw.split(',').pop().trim();const value=allowed.has(fallback)?fallback:'default';if(value===last)return;last=value;window.webkit.messageHandlers.woxWebViewCursor.postMessage(value)};"
         "document.addEventListener('mousemove',publish,true);document.addEventListener('mouseover',publish,true)})()";
}

// on_webview_cursor_message ignores cached views and applies cursor updates only from the active WebView.
static void on_webview_cursor_message(gpointer manager, gpointer javascript_result, gpointer data) {
  WoxLinuxWindow *window = data;
  if (window == NULL || window->closed || window->active_web_view == NULL || wox_webkit.get_user_content_manager == NULL ||
      wox_webkit.get_user_content_manager(window->active_web_view) != manager ||
      wox_webkit.javascript_result_get_js_value == NULL || wox_webkit.jsc_value_to_string == NULL) {
    return;
  }
  gpointer value = wox_webkit.javascript_result_get_js_value(javascript_result);
  gchar *cursor_name = value != NULL ? wox_webkit.jsc_value_to_string(value) : NULL;
  if (cursor_name == NULL) {
    return;
  }
  g_free(window->web_view_cursor_name);
  window->web_view_cursor_name = cursor_name;
  if (window->pointer_over_web_view) {
    apply_linux_pointer_cursor(window);
  }
}

static void on_webview_script_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    gtk_widget_grab_focus(window->gl_area);
    woxGoLinuxWebViewEscapeDiagnostic(window->context, gtk_widget_has_focus(window->gl_area) ? "native-focus-restored" : "native-focus-missing");
    int32_t handled = woxGoLinuxKey(window->context, "escape", 0, 1, 0, 0);
    woxGoLinuxWebViewEscapeDiagnostic(window->context, handled != 0 ? "host-dispatch handled=true" : "host-dispatch handled=false");
  }
}

static void on_webview_escape_dom_changed_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    woxGoLinuxWebViewEscapeDiagnostic(window->context, "page-dom-changed");
  }
}

static void on_webview_escape_focus_changed_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    woxGoLinuxWebViewEscapeDiagnostic(window->context, "page-focus-changed");
  }
}

static void on_webview_escape_forwarded_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    woxGoLinuxWebViewEscapeDiagnostic(window->context, "page-forwarded");
  }
}

static void on_webview_escape_prevented_no_change_forwarded_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    woxGoLinuxWebViewEscapeDiagnostic(window->context, "page-prevented-no-change-forwarded");
  }
}

static void on_webview_action_panel_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    gtk_widget_grab_focus(window->gl_area);
    woxGoLinuxKey(window->context, "j", WOX_KEY_MODIFIER_CONTROL, 1, 0, 0);
  }
}

static const char *const wox_webview_radius_key = "wox-webview-corner-radius";
static const char *const wox_webview_css_key = "wox-webview-corner-css";

// rounded_rect_region builds a device-pixel mask so WebKit's native GdkWindow can
// stay concentric with the Go preview shell. GTK CSS alone does not clip WebKitGTK.
static cairo_region_t *rounded_rect_region(int width, int height, float radius) {
  if (width <= 0 || height <= 0) {
    return cairo_region_create();
  }
  radius = fmaxf(0.0f, fminf(radius, fminf((float)width, (float)height) * 0.5f));
  if (radius <= 0.0f) {
    cairo_rectangle_int_t rect = {.x = 0, .y = 0, .width = width, .height = height};
    return cairo_region_create_rectangle(&rect);
  }
  cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_A8, width, height);
  cairo_t *cr = cairo_create(surface);
  cairo_new_sub_path(cr);
  cairo_arc(cr, width - radius, radius, radius, -G_PI_2, 0);
  cairo_arc(cr, width - radius, height - radius, radius, 0, G_PI_2);
  cairo_arc(cr, radius, height - radius, radius, G_PI_2, G_PI);
  cairo_arc(cr, radius, radius, radius, G_PI, 3 * G_PI_2);
  cairo_close_path(cr);
  cairo_set_source_rgba(cr, 1.0, 1.0, 1.0, 1.0);
  cairo_fill(cr);
  cairo_destroy(cr);
  cairo_region_t *region = gdk_cairo_region_create_from_surface(surface);
  cairo_surface_destroy(surface);
  return region;
}

static float stored_web_view_corner_radius(GtkWidget *web_view) {
  float *stored = g_object_get_data(G_OBJECT(web_view), wox_webview_radius_key);
  return stored != NULL ? *stored : 0.0f;
}

static void store_web_view_corner_radius(GtkWidget *web_view, float radius) {
  float *stored = g_new(float, 1);
  *stored = fmaxf(0.0f, radius);
  g_object_set_data_full(G_OBJECT(web_view), wox_webview_radius_key, stored, g_free);
}

static void apply_web_view_corner_radius(GtkWidget *web_view) {
  float radius = stored_web_view_corner_radius(web_view);
  GtkStyleContext *style = gtk_widget_get_style_context(web_view);
  GtkCssProvider *provider = g_object_get_data(G_OBJECT(web_view), wox_webview_css_key);
  if (provider == NULL) {
    provider = gtk_css_provider_new();
    gtk_style_context_add_class(style, "wox-webview-preview");
    gtk_style_context_add_provider(style, GTK_STYLE_PROVIDER(provider), GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
    g_object_set_data_full(G_OBJECT(web_view), wox_webview_css_key, provider, g_object_unref);
  }
  char *css = g_strdup_printf(".wox-webview-preview { border-radius: %.2fpx; }", radius);
  gtk_css_provider_load_from_data(provider, css, -1, NULL);
  g_free(css);

  if (!gtk_widget_get_realized(web_view)) {
    return;
  }
  GdkWindow *gdk_window = gtk_widget_get_window(web_view);
  if (gdk_window == NULL) {
    return;
  }
  int scale = gtk_widget_get_scale_factor(web_view);
  if (scale < 1) {
    scale = 1;
  }
  cairo_region_t *region = rounded_rect_region(gdk_window_get_width(gdk_window), gdk_window_get_height(gdk_window), radius * (float)scale);
  gtk_widget_shape_combine_region(web_view, region);
  cairo_region_destroy(region);
}

static void on_web_view_realize(GtkWidget *web_view, gpointer data) {
  (void)data;
  apply_web_view_corner_radius(web_view);
}

static void on_web_view_size_allocate(GtkWidget *web_view, GtkAllocation *allocation, gpointer data) {
  (void)allocation;
  (void)data;
  apply_web_view_corner_radius(web_view);
}

static GtkWidget *create_web_view(WoxLinuxWindow *window, const char *inject_css, const char *user_agent) {
  GtkWidget *web_view = NULL;
  bool supports_manager = wox_webkit.manager_new != NULL && wox_webkit.view_new_with_manager != NULL;
  if (supports_manager) {
    gpointer manager = wox_webkit.manager_new();
    if (manager != NULL) {
      bool supports_styles = inject_css != NULL && inject_css[0] != '\0' && wox_webkit.style_sheet_new != NULL && wox_webkit.manager_add_style_sheet != NULL && wox_webkit.style_sheet_unref != NULL;
      if (supports_styles) {
        gpointer style_sheet = wox_webkit.style_sheet_new(inject_css, 0, 0, NULL, NULL);
        if (style_sheet != NULL) {
          wox_webkit.manager_add_style_sheet(manager, style_sheet);
          wox_webkit.style_sheet_unref(style_sheet);
        }
      }
      bool supports_scripts = wox_webkit.script_new != NULL && wox_webkit.manager_add_script != NULL && wox_webkit.script_unref != NULL && wox_webkit.register_script_message_handler != NULL;
      bool cursor_handler_registered = supports_scripts && wox_webkit.get_user_content_manager != NULL && wox_webkit.javascript_result_get_js_value != NULL && wox_webkit.jsc_value_to_string != NULL &&
                                       wox_webkit.register_script_message_handler(manager, "woxWebViewCursor");
      bool handlers_registered = supports_scripts &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewPreview") &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewActionPanel") &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewEscapeFocusChanged") &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewEscapeDomChanged") &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewEscapePreventedNoChangeForwarded") &&
                                 wox_webkit.register_script_message_handler(manager, "woxWebViewEscapeForwarded");
      if (handlers_registered) {
        // Global page routers may always prevent Escape, so only an observable page transition claims it.
        const char *shortcut_script = "(()=>{if(window.__woxLauncherShortcutsInstalled__)return;window.__woxLauncherShortcutsInstalled__=true;document.addEventListener('keydown',e=>{if(e.repeat)return;if(e.ctrlKey&&!e.metaKey&&!e.altKey&&!e.shiftKey&&e.key.toLowerCase()==='j'){e.preventDefault();e.stopImmediatePropagation();window.webkit.messageHandlers.woxWebViewActionPanel.postMessage('action-panel');return}if(e.key!=='Escape')return;const f=document.activeElement;let m=false;const o=new MutationObserver(()=>{m=true});if(document.documentElement)o.observe(document.documentElement,{attributes:true,childList:true,characterData:true,subtree:true});setTimeout(()=>{o.disconnect();const r=(f&&f!==document.activeElement)?'FocusChanged':m?'DomChanged':e.defaultPrevented?'PreventedNoChangeForwarded':'Forwarded';window.webkit.messageHandlers['woxWebViewEscape'+r].postMessage(r);if(r==='Forwarded'||r==='PreventedNoChangeForwarded')window.webkit.messageHandlers.woxWebViewPreview.postMessage('escape')},0)},true)})()";
        gpointer script = wox_webkit.script_new(shortcut_script, 0, 0, NULL, NULL);
        if (script != NULL) {
          wox_webkit.manager_add_script(manager, script);
          wox_webkit.script_unref(script);
          g_signal_connect(manager, "script-message-received::woxWebViewPreview", G_CALLBACK(on_webview_script_message), window);
          g_signal_connect(manager, "script-message-received::woxWebViewActionPanel", G_CALLBACK(on_webview_action_panel_message), window);
          g_signal_connect(manager, "script-message-received::woxWebViewEscapeFocusChanged", G_CALLBACK(on_webview_escape_focus_changed_message), window);
          g_signal_connect(manager, "script-message-received::woxWebViewEscapeDomChanged", G_CALLBACK(on_webview_escape_dom_changed_message), window);
          g_signal_connect(manager, "script-message-received::woxWebViewEscapePreventedNoChangeForwarded", G_CALLBACK(on_webview_escape_prevented_no_change_forwarded_message), window);
          g_signal_connect(manager, "script-message-received::woxWebViewEscapeForwarded", G_CALLBACK(on_webview_escape_forwarded_message), window);
        }
      }
      if (cursor_handler_registered) {
        gpointer cursor_script = wox_webkit.script_new(web_view_cursor_script(), 0, 0, NULL, NULL);
        if (cursor_script != NULL) {
          wox_webkit.manager_add_script(manager, cursor_script);
          wox_webkit.script_unref(cursor_script);
          g_signal_connect(manager, "script-message-received::woxWebViewCursor", G_CALLBACK(on_webview_cursor_message), window);
        }
      }
      web_view = wox_webkit.view_new_with_manager(manager);
      g_object_unref(manager);
    }
  }
  if (web_view == NULL) {
    web_view = wox_webkit.view_new();
  }
  if (web_view != NULL) {
    if (user_agent[0] != '\0') {
      gpointer settings = wox_webkit.get_settings(web_view);
      if (settings == NULL) {
        gtk_widget_destroy(web_view);
        return NULL;
      }
      wox_webkit.set_user_agent(settings, user_agent);
    }
    g_object_ref_sink(web_view);
    gtk_widget_set_no_show_all(web_view, TRUE);
    gtk_widget_set_halign(web_view, GTK_ALIGN_START);
    gtk_widget_set_valign(web_view, GTK_ALIGN_START);
    g_signal_connect(web_view, "realize", G_CALLBACK(on_web_view_realize), NULL);
    g_signal_connect(web_view, "size-allocate", G_CALLBACK(on_web_view_size_allocate), NULL);
  }
  return web_view;
}

static void clear_active_web_view(WoxLinuxWindow *window, bool remove_from_parent) {
  if (window->active_web_view != NULL) {
    if (remove_from_parent && gtk_widget_get_parent(window->active_web_view) != NULL) {
      gtk_container_remove(GTK_CONTAINER(window->overlay), window->active_web_view);
    }
    if (window->active_web_view_transient) {
      g_object_unref(window->active_web_view);
    }
  }
  window->active_web_view = NULL;
  window->active_web_view_transient = false;
  g_clear_pointer(&window->active_web_view_key, g_free);
  g_clear_pointer(&window->active_web_view_signature, g_free);
  g_clear_pointer(&window->active_web_view_content_key, g_free);
  window->pointer_over_web_view = false;
  g_clear_pointer(&window->web_view_cursor_name, g_free);
  apply_linux_pointer_cursor(window);
}

static const char *const rect_vertex_source =
    "#version 330 core\n"
    "uniform vec2 u_viewport;\n"
    "uniform vec4 u_rect;\n"
    "out vec2 v_local;\n"
    "void main() {\n"
    "  vec2 corners[4] = vec2[4](vec2(0.0, 0.0), vec2(1.0, 0.0), vec2(0.0, 1.0), vec2(1.0, 1.0));\n"
    "  vec2 corner = corners[gl_VertexID];\n"
    "  vec2 point = u_rect.xy + corner * u_rect.zw;\n"
    "  gl_Position = vec4(point.x / u_viewport.x * 2.0 - 1.0, 1.0 - point.y / u_viewport.y * 2.0, 0.0, 1.0);\n"
    "  v_local = corner * u_rect.zw;\n"
    "}\n";

static const char *const rect_fragment_source =
    "#version 330 core\n"
    "uniform vec4 u_rect;\n"
    "uniform vec4 u_color;\n"
    "uniform float u_radius;\n"
    "uniform float u_stroke_width;\n"
    "uniform vec2 u_polygon[16];\n"
    "uniform int u_polygon_count;\n"
    "in vec2 v_local;\n"
    "out vec4 fragment_color;\n"
    "float cross2(vec2 left, vec2 right) { return left.x * right.y - left.y * right.x; }\n"
    "void main() {\n"
    "  if (u_polygon_count >= 3) {\n"
    "    float area = 0.0;\n"
    "    for (int index = 0; index < u_polygon_count; index++) {\n"
    "      area += cross2(u_polygon[index], u_polygon[(index + 1) % u_polygon_count]);\n"
    "    }\n"
    "    float orientation = area >= 0.0 ? 1.0 : -1.0;\n"
    "    vec2 point = u_rect.xy + v_local;\n"
    "    float distance_value = 1e20;\n"
    "    for (int index = 0; index < u_polygon_count; index++) {\n"
    "      vec2 start = u_polygon[index];\n"
    "      vec2 edge = u_polygon[(index + 1) % u_polygon_count] - start;\n"
    "      distance_value = min(distance_value, orientation * cross2(edge, point - start) / max(length(edge), 0.001));\n"
    "    }\n"
    "    float antialias = max(fwidth(distance_value), 0.001);\n"
    "    fragment_color = u_color * smoothstep(-antialias * 0.5, antialias * 0.5, distance_value);\n"
    "    return;\n"
    "  }\n"
    "  float radius = clamp(u_radius, 0.0, min(u_rect.z, u_rect.w) * 0.5);\n"
    "  vec2 half_size = u_rect.zw * 0.5;\n"
    "  vec2 edge = abs(v_local - half_size) - (half_size - radius);\n"
    "  float distance_value = length(max(edge, vec2(0.0))) + min(max(edge.x, edge.y), 0.0) - radius;\n"
    "  float antialias = max(fwidth(distance_value), 0.001);\n"
    "  float outer_coverage = 1.0 - smoothstep(-antialias * 0.5, antialias * 0.5, distance_value);\n"
    "  if (u_stroke_width <= 0.0) { fragment_color = u_color * outer_coverage; return; }\n"
    "  float inner_radius = max(radius - u_stroke_width, 0.0);\n"
    "  vec2 inner_half = max(half_size - u_stroke_width, vec2(0.0));\n"
    "  vec2 inner_edge = abs(v_local - half_size) - max(inner_half - inner_radius, vec2(0.0));\n"
    "  float inner_distance = length(max(inner_edge, vec2(0.0))) + min(max(inner_edge.x, inner_edge.y), 0.0) - inner_radius;\n"
    "  float inner_antialias = max(fwidth(inner_distance), 0.001);\n"
    "  float inner_coverage = 1.0 - smoothstep(-inner_antialias * 0.5, inner_antialias * 0.5, inner_distance);\n"
    "  float coverage = clamp(outer_coverage - inner_coverage, 0.0, 1.0);\n"
    "  fragment_color = u_color * coverage;\n"
    "}\n";

static const char *const texture_vertex_source =
    "#version 330 core\n"
    "uniform vec2 u_viewport;\n"
    "uniform vec4 u_rect;\n"
    "uniform float u_rotation;\n"
    "out vec2 v_uv;\n"
    "void main() {\n"
    "  vec2 corners[4] = vec2[4](vec2(0.0, 0.0), vec2(1.0, 0.0), vec2(0.0, 1.0), vec2(1.0, 1.0));\n"
    "  vec2 corner = corners[gl_VertexID];\n"
    "  vec2 local = (corner - vec2(0.5)) * u_rect.zw;\n"
    "  float cosine = cos(u_rotation);\n"
    "  float sine = sin(u_rotation);\n"
    "  vec2 point = u_rect.xy + u_rect.zw * 0.5 + mat2(cosine, sine, -sine, cosine) * local;\n"
    "  gl_Position = vec4(point.x / u_viewport.x * 2.0 - 1.0, 1.0 - point.y / u_viewport.y * 2.0, 0.0, 1.0);\n"
    "  v_uv = corner;\n"
    "}\n";

static const char *const texture_fragment_source =
    "#version 330 core\n"
    "uniform sampler2D u_texture;\n"
    "uniform vec4 u_color;\n"
    "uniform vec4 u_rect;\n"
    "uniform float u_radius;\n"
    "in vec2 v_uv;\n"
    "out vec4 fragment_color;\n"
    "void main() {\n"
    "  float coverage = 1.0;\n"
    "  if (u_radius > 0.0) {\n"
    "    float radius = min(u_radius, min(u_rect.z, u_rect.w) * 0.5);\n"
    "    vec2 half_size = u_rect.zw * 0.5;\n"
    "    vec2 edge = abs(v_uv * u_rect.zw - half_size) - (half_size - radius);\n"
    "    float distance_value = length(max(edge, vec2(0.0))) + min(max(edge.x, edge.y), 0.0) - radius;\n"
    "    float antialias = max(fwidth(distance_value), 0.001);\n"
    "    coverage = 1.0 - smoothstep(-antialias * 0.5, antialias * 0.5, distance_value);\n"
    "  }\n"
    "  fragment_color = texture(u_texture, v_uv) * u_color * coverage;\n"
    "}\n";

typedef void (*WoxMainFunction)(void *data);

typedef struct {
  GMutex mutex;
  GCond condition;
  bool done;
  WoxMainFunction function;
  void *data;
} WoxMainCall;

static bool is_main_thread(void) {
  return g_atomic_int_get(&wox_linux_runtime_running) != 0 && pthread_equal(pthread_self(), wox_linux_main_thread);
}

static gboolean execute_main_call(gpointer data) {
  WoxMainCall *call = data;
  call->function(call->data);
  g_mutex_lock(&call->mutex);
  call->done = true;
  g_cond_signal(&call->condition);
  g_mutex_unlock(&call->mutex);
  return G_SOURCE_REMOVE;
}

// run_on_main_sync keeps GTK and OpenGL ownership on the runtime thread.
static bool run_on_main_sync(WoxMainFunction function, void *data) {
  if (is_main_thread()) {
    function(data);
    return true;
  }
  if (g_atomic_int_get(&wox_linux_runtime_running) == 0 || g_atomic_int_get(&wox_linux_loop_active) == 0) {
    return false;
  }

  WoxMainCall call = {.done = false, .function = function, .data = data};
  g_mutex_init(&call.mutex);
  g_cond_init(&call.condition);
  GSource *source = g_idle_source_new();
  g_source_set_callback(source, execute_main_call, &call, NULL);
  guint source_id = g_source_attach(source, g_main_context_default());
  g_source_unref(source);
  if (source_id == 0) {
    g_cond_clear(&call.condition);
    g_mutex_clear(&call.mutex);
    return false;
  }

  g_mutex_lock(&call.mutex);
  while (!call.done) {
    g_cond_wait(&call.condition, &call.mutex);
  }
  g_mutex_unlock(&call.mutex);
  g_cond_clear(&call.condition);
  g_mutex_clear(&call.mutex);
  return true;
}

static void execute_go_call(void *data) {
  woxGoLinuxCall((uintptr_t)data);
}

int32_t wox_linux_call(uintptr_t context) {
  if (context == 0 || !run_on_main_sync(execute_go_call, (void *)context)) {
    return -1;
  }
  return 0;
}

static void premultiplied_color(uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha, float color[4]) {
  float a = (float)alpha / 255.0f;
  color[0] = (float)red / 255.0f * a;
  color[1] = (float)green / 255.0f * a;
  color[2] = (float)blue / 255.0f * a;
  color[3] = a;
}

static GLuint compile_shader(GLenum type, const char *source) {
  GLuint shader = glCreateShader(type);
  glShaderSource(shader, 1, &source, NULL);
  glCompileShader(shader);
  GLint compiled = GL_FALSE;
  glGetShaderiv(shader, GL_COMPILE_STATUS, &compiled);
  if (compiled == GL_TRUE) {
    return shader;
  }
  char log[2048] = {0};
  glGetShaderInfoLog(shader, sizeof(log), NULL, log);
  g_warning("Wox Go UI: OpenGL shader compilation failed: %s", log);
  glDeleteShader(shader);
  return 0;
}

static GLuint create_program(const char *vertex_source, const char *fragment_source) {
  GLuint vertex = compile_shader(GL_VERTEX_SHADER, vertex_source);
  if (vertex == 0) {
    return 0;
  }
  GLuint fragment = compile_shader(GL_FRAGMENT_SHADER, fragment_source);
  if (fragment == 0) {
    glDeleteShader(vertex);
    return 0;
  }
  GLuint program = glCreateProgram();
  glAttachShader(program, vertex);
  glAttachShader(program, fragment);
  glLinkProgram(program);
  glDeleteShader(vertex);
  glDeleteShader(fragment);
  GLint linked = GL_FALSE;
  glGetProgramiv(program, GL_LINK_STATUS, &linked);
  if (linked == GL_TRUE) {
    return program;
  }
  char log[2048] = {0};
  glGetProgramInfoLog(program, sizeof(log), NULL, log);
  g_warning("Wox Go UI: OpenGL program linking failed: %s", log);
  glDeleteProgram(program);
  return 0;
}

static uint64_t fnv1a_bytes(uint64_t hash, const void *data, size_t length) {
  const uint8_t *bytes = data;
  for (size_t index = 0; index < length; index++) {
    hash ^= bytes[index];
    hash *= 1099511628211ULL;
  }
  return hash;
}

static uint64_t linux_text_cache_hash(const char *text, const char *family, float font_size, uint8_t font_weight, uint8_t italic, float scale, int pixel_width, int pixel_height) {
  uint64_t hash = 14695981039346656037ULL;
  hash = fnv1a_bytes(hash, text, strlen(text));
  hash = fnv1a_bytes(hash, family, strlen(family));
  hash = fnv1a_bytes(hash, &font_size, sizeof(font_size));
  hash = fnv1a_bytes(hash, &font_weight, sizeof(font_weight));
  hash = fnv1a_bytes(hash, &italic, sizeof(italic));
  hash = fnv1a_bytes(hash, &scale, sizeof(scale));
  hash = fnv1a_bytes(hash, &pixel_width, sizeof(pixel_width));
  hash = fnv1a_bytes(hash, &pixel_height, sizeof(pixel_height));
  return hash;
}

static void delete_gl_texture(GLuint *texture, bool delete_texture) {
  if (texture == NULL || *texture == 0) {
    return;
  }
  if (delete_texture) {
    glDeleteTextures(1, texture);
  }
  *texture = 0;
}

// clear_cached_large_image drops the adaptive 32MiB preview slot for one GL context.
static void clear_cached_large_image(WoxLinuxRenderer *renderer, bool delete_textures) {
  if (renderer == NULL) {
    return;
  }
  delete_gl_texture(&renderer->cached_large_image, delete_textures);
  renderer->cached_large_image_id = 0;
  renderer->cached_large_image_bytes = 0;
  renderer->cached_large_image_generation = 0;
}

// reset_large_image_policy forgets the adaptive large-image candidate after hide.
static void reset_large_image_policy(WoxLinuxWindow *window) {
  if (window == NULL) {
    return;
  }
  window->large_image_candidate_id = 0;
  window->large_image_candidate_frames = 0;
  window->large_image_create_count = 0;
  window->cache_large_images = false;
}

static void clear_linux_resource_caches(WoxLinuxRenderer *renderer, bool delete_textures) {
  if (renderer == NULL) {
    return;
  }
  for (int32_t index = 0; index < renderer->image_count; index++) {
    delete_gl_texture(&renderer->images[index].texture, delete_textures);
  }
  memset(renderer->images, 0, sizeof(renderer->images));
  renderer->image_count = 0;
  renderer->image_bytes = 0;
  clear_cached_large_image(renderer, delete_textures);
  if (renderer->texts != NULL) {
    for (int32_t index = 0; index < renderer->text_count; index++) {
      delete_gl_texture(&renderer->texts[index].texture, delete_textures);
    }
    memset(renderer->texts, 0, sizeof(WoxLinuxTextCacheEntry) * (size_t)WOX_LINUX_TEXT_CACHE_MAX);
  }
  renderer->text_count = 0;
  renderer->text_bytes = 0;
}

static uint64_t linux_cache_resident_bytes(const WoxLinuxRenderer *renderer) {
  if (renderer == NULL) {
    return 0;
  }
  return renderer->image_bytes + renderer->text_bytes + renderer->cached_large_image_bytes;
}

static WoxLinuxImageCacheEntry *find_cached_gl_image(WoxLinuxRenderer *renderer, uint64_t image_id) {
  if (renderer == NULL || image_id == 0) {
    return NULL;
  }
  for (int32_t index = 0; index < renderer->image_count; index++) {
    if (renderer->images[index].image_id == image_id && renderer->images[index].generation == renderer->context_generation) {
      renderer->images[index].last_used = ++renderer->image_use_serial;
      return &renderer->images[index];
    }
  }
  return NULL;
}

static void evict_oldest_gl_image(WoxLinuxRenderer *renderer, bool delete_textures, int32_t *evictions) {
  if (renderer->image_count <= 0) {
    return;
  }
  int32_t oldest = 0;
  for (int32_t index = 1; index < renderer->image_count; index++) {
    if (renderer->images[index].last_used < renderer->images[oldest].last_used) {
      oldest = index;
    }
  }
  renderer->image_bytes -= renderer->images[oldest].byte_size;
  delete_gl_texture(&renderer->images[oldest].texture, delete_textures);
  renderer->images[oldest] = renderer->images[renderer->image_count - 1];
  renderer->image_count--;
  if (evictions != NULL) {
    (*evictions)++;
  }
}

static bool cache_gl_image(WoxLinuxRenderer *renderer, uint64_t image_id, uint64_t byte_size, GLuint texture, bool delete_textures, int32_t *evictions) {
  if (renderer == NULL || image_id == 0 || texture == 0 || byte_size == 0 || byte_size > wox_linux_image_cache_max_entry_bytes) {
    return false;
  }
  while (renderer->image_count > 0 &&
         (renderer->image_count >= WOX_LINUX_IMAGE_CACHE_MAX ||
          renderer->image_bytes + byte_size > wox_linux_image_cache_max_bytes)) {
    evict_oldest_gl_image(renderer, delete_textures, evictions);
  }
  if (renderer->image_count >= WOX_LINUX_IMAGE_CACHE_MAX || renderer->image_bytes + byte_size > wox_linux_image_cache_max_bytes) {
    return false;
  }
  renderer->images[renderer->image_count++] = (WoxLinuxImageCacheEntry){
      .image_id = image_id,
      .byte_size = byte_size,
      .last_used = ++renderer->image_use_serial,
      .generation = renderer->context_generation,
      .texture = texture,
  };
  renderer->image_bytes += byte_size;
  return true;
}

static int64_t linux_monotonic_nanos(void) {
  struct timespec now;
  if (clock_gettime(CLOCK_MONOTONIC, &now) != 0) {
    return 0;
  }
  return (int64_t)now.tv_sec * 1000000000LL + (int64_t)now.tv_nsec;
}

static int64_t linux_p95_nanos(const int64_t *samples, int32_t count) {
  if (samples == NULL || count <= 0) {
    return 0;
  }
  int64_t sorted[16];
  int32_t limit = count < 16 ? count : 16;
  memcpy(sorted, samples, sizeof(int64_t) * (size_t)limit);
  for (int32_t index = 1; index < limit; index++) {
    int64_t value = sorted[index];
    int32_t slot = index;
    while (slot > 0 && sorted[slot - 1] > value) {
      sorted[slot] = sorted[slot - 1];
      slot--;
    }
    sorted[slot] = value;
  }
  int32_t rank = (limit * 95 + 99) / 100 - 1;
  if (rank < 0) {
    rank = 0;
  }
  if (rank >= limit) {
    rank = limit - 1;
  }
  return sorted[rank];
}

// note_large_image_create enables one 32MiB preview slot after the same oversized image
// is recreated for three frames and image-create P95 stays above 2ms.
static void note_large_image_create(WoxLinuxWindow *window, uint64_t image_id, int64_t create_ns) {
  if (window == NULL) {
    return;
  }
  if (window->large_image_candidate_id == image_id) {
    window->large_image_candidate_frames++;
  } else {
    window->large_image_candidate_id = image_id;
    window->large_image_candidate_frames = 1;
    window->large_image_create_count = 0;
    // Admission is bound to the current candidate. Without this reset a different oversized image
    // would inherit the previous winner's verdict and evict the single slot on first sight, so two
    // alternating previews would keep replacing each other and never accumulate hits.
    window->cache_large_images = false;
  }
  if (window->large_image_create_count < 16) {
    window->large_image_create_ns[window->large_image_create_count++] = create_ns;
  }
  if (!window->cache_large_images && window->large_image_candidate_frames >= 3 &&
      linux_p95_nanos(window->large_image_create_ns, window->large_image_create_count) > 2000000LL) {
    window->cache_large_images = true;
  }
}

static WoxLinuxTextCacheEntry *find_cached_gl_text(WoxLinuxRenderer *renderer, uint64_t hash, const char *text, const char *family, float font_size, uint8_t font_weight, uint8_t italic, float scale, int pixel_width, int pixel_height) {
  if (renderer == NULL || renderer->texts == NULL) {
    return NULL;
  }
  for (int32_t index = 0; index < renderer->text_count; index++) {
    WoxLinuxTextCacheEntry *entry = &renderer->texts[index];
    if (entry->hash == hash && entry->generation == renderer->context_generation &&
        entry->font_size == font_size && entry->font_weight == font_weight && entry->italic == italic && entry->scale == scale &&
        entry->pixel_width == pixel_width && entry->pixel_height == pixel_height &&
        strcmp(entry->text, text) == 0 && strcmp(entry->family, family) == 0) {
      entry->last_used = ++renderer->text_use_serial;
      return entry;
    }
  }
  return NULL;
}

static void evict_oldest_gl_text(WoxLinuxRenderer *renderer, bool delete_textures, int32_t *evictions) {
  if (renderer->texts == NULL || renderer->text_count <= 0) {
    return;
  }
  int32_t oldest = 0;
  for (int32_t index = 1; index < renderer->text_count; index++) {
    if (renderer->texts[index].last_used < renderer->texts[oldest].last_used) {
      oldest = index;
    }
  }
  renderer->text_bytes -= renderer->texts[oldest].byte_size;
  delete_gl_texture(&renderer->texts[oldest].texture, delete_textures);
  renderer->texts[oldest] = renderer->texts[renderer->text_count - 1];
  renderer->text_count--;
  if (evictions != NULL) {
    (*evictions)++;
  }
}

static bool cache_gl_text(WoxLinuxRenderer *renderer, uint64_t hash, const char *text, const char *family, float font_size, uint8_t font_weight, uint8_t italic, float scale, int pixel_width, int pixel_height, uint64_t byte_size, GLuint texture, bool delete_textures, int32_t *evictions) {
  if (renderer == NULL || renderer->texts == NULL || texture == 0 || byte_size == 0 || byte_size > wox_linux_text_cache_max_entry_bytes) {
    return false;
  }
  if (strlen(text) > WOX_LINUX_TEXT_CACHE_MAX_CHARS || strlen(family) >= sizeof(renderer->texts[0].family)) {
    return false;
  }
  while (renderer->text_count > 0 &&
         (renderer->text_count >= WOX_LINUX_TEXT_CACHE_MAX ||
          renderer->text_bytes + byte_size > wox_linux_text_cache_max_bytes)) {
    evict_oldest_gl_text(renderer, delete_textures, evictions);
  }
  if (renderer->text_count >= WOX_LINUX_TEXT_CACHE_MAX || renderer->text_bytes + byte_size > wox_linux_text_cache_max_bytes) {
    return false;
  }
  WoxLinuxTextCacheEntry *entry = &renderer->texts[renderer->text_count++];
  memset(entry, 0, sizeof(*entry));
  entry->hash = hash;
  entry->byte_size = byte_size;
  entry->last_used = ++renderer->text_use_serial;
  entry->generation = renderer->context_generation;
  entry->texture = texture;
  entry->font_size = font_size;
  entry->scale = scale;
  entry->pixel_width = pixel_width;
  entry->pixel_height = pixel_height;
  entry->font_weight = font_weight;
  entry->italic = italic;
  memcpy(entry->text, text, strlen(text));
  memcpy(entry->family, family, strlen(family));
  renderer->text_bytes += byte_size;
  return true;
}

static void draw_bound_texture(WoxLinuxRenderer *renderer, float x, float y, float width, float height, float rotation_radians, float corner_radius, const float color[4]) {
  glUseProgram(renderer->texture_program);
  glUniform2f(renderer->texture_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->texture_bounds, x, y, width, height);
  glUniform4fv(renderer->texture_color, 1, color);
  glUniform1f(renderer->texture_rotation, rotation_radians);
  glUniform1f(renderer->texture_radius, corner_radius);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  glBindTexture(GL_TEXTURE_2D, 0);
}

static GLuint upload_gl_texture(int width, int height, GLenum format, const void *pixels, int unpack_row_length) {
  GLuint texture = 0;
  glGenTextures(1, &texture);
  glActiveTexture(GL_TEXTURE0);
  glBindTexture(GL_TEXTURE_2D, texture);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
  glPixelStorei(GL_UNPACK_ALIGNMENT, 4);
  if (unpack_row_length > 0) {
    glPixelStorei(GL_UNPACK_ROW_LENGTH, unpack_row_length);
  }
  glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, width, height, 0, format, GL_UNSIGNED_BYTE, pixels);
  if (unpack_row_length > 0) {
    glPixelStorei(GL_UNPACK_ROW_LENGTH, 0);
  }
  return texture;
}

// linux_gl_area_can_make_current reports whether gtk_gl_area_make_current is
// safe. GTK dereferences a NULL GdkGLContext when the area is unrealized, which
// is the SIGSEGV seen when hiding a layer-shell launcher on Wayland.
static bool linux_gl_area_can_make_current(GtkWidget *gl_area) {
  return gl_area != NULL && GTK_IS_GL_AREA(gl_area) && gtk_widget_get_realized(gl_area) && gtk_gl_area_get_context(GTK_GL_AREA(gl_area)) != NULL;
}

static bool initialize_renderer(WoxLinuxWindow *window, WoxLinuxRenderer *renderer, GtkWidget *gl_area) {
  gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
  GError *error = gtk_gl_area_get_error(GTK_GL_AREA(gl_area));
  if (error != NULL) {
    trace_linux_render("event=renderer_init_failed surface=%s error=%s", linux_renderer_name(window, renderer), error->message);
    g_warning("Wox Go UI: failed to create OpenGL context: %s", error->message);
    return false;
  }
  const char *vendor = (const char *)glGetString(GL_VENDOR);
  const char *gl_renderer = (const char *)glGetString(GL_RENDERER);
  const char *version = (const char *)glGetString(GL_VERSION);
  const char *shading_language = (const char *)glGetString(GL_SHADING_LANGUAGE_VERSION);
  GdkDisplay *display = gtk_widget_get_display(gl_area);
  trace_linux_render(
      "event=renderer_init surface=%s context=%p displayType=%s hasAlpha=%d vendor=%s renderer=%s version=%s shadingLanguage=%s",
      linux_renderer_name(window, renderer),
      (void *)gtk_gl_area_get_context(GTK_GL_AREA(gl_area)),
      display != NULL ? G_OBJECT_TYPE_NAME(display) : "unknown",
      gtk_gl_area_get_has_alpha(GTK_GL_AREA(gl_area)),
      vendor != NULL ? vendor : "unknown",
      gl_renderer != NULL ? gl_renderer : "unknown",
      version != NULL ? version : "unknown",
      shading_language != NULL ? shading_language : "unknown");
  renderer->rect_program = create_program(rect_vertex_source, rect_fragment_source);
  renderer->texture_program = create_program(texture_vertex_source, texture_fragment_source);
  if (renderer->rect_program == 0 || renderer->texture_program == 0) {
    if (wox_linux_render_trace_enabled) {
      trace_linux_render("event=renderer_init_failed surface=%s reason=shader_program rectProgram=%u textureProgram=%u glError=%#x", linux_renderer_name(window, renderer), renderer->rect_program, renderer->texture_program, (unsigned int)glGetError());
    }
    if (renderer->texture_program != 0) {
      glDeleteProgram(renderer->texture_program);
    }
    if (renderer->rect_program != 0) {
      glDeleteProgram(renderer->rect_program);
    }
    memset(renderer, 0, sizeof(*renderer));
    return false;
  }

  glGenVertexArrays(1, &renderer->vertex_array);
  renderer->rect_viewport = glGetUniformLocation(renderer->rect_program, "u_viewport");
  renderer->rect_bounds = glGetUniformLocation(renderer->rect_program, "u_rect");
  renderer->rect_color = glGetUniformLocation(renderer->rect_program, "u_color");
  renderer->rect_radius = glGetUniformLocation(renderer->rect_program, "u_radius");
  renderer->rect_stroke_width = glGetUniformLocation(renderer->rect_program, "u_stroke_width");
  renderer->rect_polygon = glGetUniformLocation(renderer->rect_program, "u_polygon[0]");
  renderer->rect_polygon_count = glGetUniformLocation(renderer->rect_program, "u_polygon_count");
  renderer->texture_viewport = glGetUniformLocation(renderer->texture_program, "u_viewport");
  renderer->texture_bounds = glGetUniformLocation(renderer->texture_program, "u_rect");
  renderer->texture_color = glGetUniformLocation(renderer->texture_program, "u_color");
  renderer->texture_rotation = glGetUniformLocation(renderer->texture_program, "u_rotation");
  renderer->texture_radius = glGetUniformLocation(renderer->texture_program, "u_radius");
  glUseProgram(renderer->texture_program);
  glUniform1i(glGetUniformLocation(renderer->texture_program, "u_texture"), 0);
  glUseProgram(0);
  if (renderer->texts == NULL) {
    renderer->texts = calloc((size_t)WOX_LINUX_TEXT_CACHE_MAX, sizeof(WoxLinuxTextCacheEntry));
    if (renderer->texts == NULL) {
      glDeleteVertexArrays(1, &renderer->vertex_array);
      glDeleteProgram(renderer->texture_program);
      glDeleteProgram(renderer->rect_program);
      memset(renderer, 0, sizeof(*renderer));
      return false;
    }
  }
  renderer->context_generation++;
  renderer->ready = true;
  if (wox_linux_render_trace_enabled) {
    trace_linux_render("event=renderer_ready surface=%s generation=%llu rectProgram=%u textureProgram=%u vertexArray=%u glError=%#x", linux_renderer_name(window, renderer), (unsigned long long)renderer->context_generation, renderer->rect_program, renderer->texture_program, renderer->vertex_array, (unsigned int)glGetError());
  }
  return true;
}

// ensure_frame_storage creates the persistent texture that owns pixels across GtkGLArea swaps.
static bool ensure_frame_storage(WoxLinuxRenderer *renderer, int width, int height, bool *recreated) {
  *recreated = renderer->frame_width != width || renderer->frame_height != height || renderer->frame_texture == 0 || renderer->frame_framebuffer == 0;
  if (!*recreated) {
    return true;
  }
  if (renderer->frame_texture == 0) {
    glGenTextures(1, &renderer->frame_texture);
  }
  if (renderer->frame_framebuffer == 0) {
    glGenFramebuffers(1, &renderer->frame_framebuffer);
  }
  glBindTexture(GL_TEXTURE_2D, renderer->frame_texture);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
  glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, width, height, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
  glBindTexture(GL_TEXTURE_2D, 0);
  glBindFramebuffer(GL_FRAMEBUFFER, renderer->frame_framebuffer);
  glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, renderer->frame_texture, 0);
  if (glCheckFramebufferStatus(GL_FRAMEBUFFER) != GL_FRAMEBUFFER_COMPLETE) {
    return false;
  }
  renderer->frame_width = width;
  renderer->frame_height = height;
  return true;
}

static void destroy_renderer(WoxLinuxRenderer *renderer, GtkWidget *gl_area) {
  if (!renderer->ready) {
    return;
  }
  bool deleted_gl = false;
  if (linux_gl_area_can_make_current(gl_area)) {
    gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
    if (gtk_gl_area_get_error(GTK_GL_AREA(gl_area)) == NULL) {
      clear_linux_resource_caches(renderer, true);
      if (renderer->frame_framebuffer != 0) {
        glDeleteFramebuffers(1, &renderer->frame_framebuffer);
      }
      if (renderer->frame_texture != 0) {
        glDeleteTextures(1, &renderer->frame_texture);
      }
      glDeleteVertexArrays(1, &renderer->vertex_array);
      glDeleteProgram(renderer->texture_program);
      glDeleteProgram(renderer->rect_program);
      deleted_gl = true;
    }
  }
  if (!deleted_gl) {
    clear_linux_resource_caches(renderer, false);
  }
  free(renderer->texts);
  memset(renderer, 0, sizeof(*renderer));
}

static void emit_focus(WoxLinuxWindow *window, bool active) {
  if (window == NULL || window->closed || window->active == active) {
    return;
  }
  window->active = active;
  uintptr_t context = window->context;
  if (context != 0) {
    woxGoLinuxFocus(context, window->epoch, active ? 1 : 0);
  }
}

static uint8_t portable_modifiers(GdkModifierType state) {
  uint8_t modifiers = 0;
  if ((state & GDK_SHIFT_MASK) != 0) {
    modifiers |= WOX_KEY_MODIFIER_SHIFT;
  }
  if ((state & GDK_CONTROL_MASK) != 0) {
    modifiers |= WOX_KEY_MODIFIER_CONTROL;
  }
  if ((state & GDK_MOD1_MASK) != 0) {
    modifiers |= WOX_KEY_MODIFIER_ALT;
  }
  if ((state & (GDK_SUPER_MASK | GDK_META_MASK)) != 0) {
    modifiers |= WOX_KEY_MODIFIER_META;
  }
  return modifiers;
}

// portable_key keeps GDK key symbols out of the shared Go input contract.
static const char *portable_key(guint keyval, char text[8]) {
  switch (keyval) {
  case GDK_KEY_BackSpace:
    return "backspace";
  case GDK_KEY_Tab:
  case GDK_KEY_ISO_Left_Tab:
    return "tab";
  case GDK_KEY_Return:
  case GDK_KEY_KP_Enter:
    return "enter";
  case GDK_KEY_Escape:
    return "escape";
  case GDK_KEY_space:
    return "space";
  case GDK_KEY_Page_Up:
  case GDK_KEY_KP_Page_Up:
    return "page-up";
  case GDK_KEY_Page_Down:
  case GDK_KEY_KP_Page_Down:
    return "page-down";
  case GDK_KEY_End:
  case GDK_KEY_KP_End:
    return "end";
  case GDK_KEY_Home:
  case GDK_KEY_KP_Home:
    return "home";
  case GDK_KEY_Left:
  case GDK_KEY_KP_Left:
    return "arrow-left";
  case GDK_KEY_Up:
  case GDK_KEY_KP_Up:
    return "arrow-up";
  case GDK_KEY_Right:
  case GDK_KEY_KP_Right:
    return "arrow-right";
  case GDK_KEY_Down:
  case GDK_KEY_KP_Down:
    return "arrow-down";
  case GDK_KEY_Delete:
  case GDK_KEY_KP_Delete:
    return "delete";
  case GDK_KEY_Alt_L:
  case GDK_KEY_Alt_R:
    return "alt";
  case GDK_KEY_Meta_L:
  case GDK_KEY_Meta_R:
  case GDK_KEY_Super_L:
  case GDK_KEY_Super_R:
    return "meta";
  default:
    break;
  }

  gunichar character = gdk_keyval_to_unicode(gdk_keyval_to_lower(keyval));
  if (character == 0 || !g_unichar_isprint(character)) {
    text[0] = '\0';
    return text;
  }
  int length = g_unichar_to_utf8(character, text);
  text[length] = '\0';
  return text;
}

static gboolean emit_key(WoxLinuxWindow *window, GdkEventKey *event, bool down) {
  if (window->closed || window->context == 0) {
    return FALSE;
  }
  gpointer pressed_key = GUINT_TO_POINTER(event->hardware_keycode + 1);
  bool repeat = down && g_hash_table_contains(window->pressed_keys, pressed_key);
  if (down) {
    g_hash_table_add(window->pressed_keys, pressed_key);
  } else {
    g_hash_table_remove(window->pressed_keys, pressed_key);
  }
  char key_text[8];
  return woxGoLinuxKey(window->context, portable_key(event->keyval, key_text), portable_modifiers(event->state), down ? 1 : 0, repeat ? 1 : 0, window->input_composing ? 1 : 0) != 0;
}

static void on_ime_commit(GtkIMContext *context, const gchar *text, gpointer data) {
  (void)context;
  WoxLinuxWindow *window = data;
  if (window->closed || !window->input_enabled || window->context == 0 || text == NULL || text[0] == '\0') {
    return;
  }
  window->input_composing = false;
  woxGoLinuxTextInput(window->context, WOX_TEXT_INPUT_COMMIT, text);
}

// on_ime_preedit_changed preserves preedit separately so widgets do not commit partial IME text.
static void on_ime_preedit_changed(GtkIMContext *context, gpointer data) {
  WoxLinuxWindow *window = data;
  if (window->closed || !window->input_enabled || window->context == 0) {
    return;
  }
  gchar *text = NULL;
  gtk_im_context_get_preedit_string(context, &text, NULL, NULL);
  const char *composition = text != NULL ? text : "";
  window->input_composing = composition[0] != '\0';
  woxGoLinuxTextInput(window->context, WOX_TEXT_INPUT_COMPOSE, composition);
  g_free(text);
}

static gboolean on_key_press(GtkWidget *widget, GdkEventKey *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (emit_key(window, event, true)) {
    return TRUE;
  }
  return window->input_enabled && gtk_im_context_filter_keypress(window->im_context, event);
}

static gboolean on_key_release(GtkWidget *widget, GdkEventKey *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (emit_key(window, event, false)) {
    return TRUE;
  }
  return window->input_enabled && gtk_im_context_filter_keypress(window->im_context, event);
}

static uint8_t portable_pointer_button(guint button) {
  switch (button) {
  case GDK_BUTTON_PRIMARY:
    return 1;
  case GDK_BUTTON_SECONDARY:
    return 2;
  case GDK_BUTTON_MIDDLE:
    return 3;
  default:
    return 0;
  }
}

static void update_layer_shell_move(WoxLinuxWindow *window);
static void end_layer_shell_move(WoxLinuxWindow *window);
static void store_linux_pointer_event(WoxLinuxWindow *window, double x, double y, GdkWindow *event_window, double x_root, double y_root, guint32 time);

// linux_resize_hit_test maps client coordinates onto a GdkWindowEdge, or -1 for the client area.
static int linux_resize_hit_test(double x, double y, int width, int height, int grip) {
  if (width <= 0 || height <= 0 || grip <= 0) {
    return -1;
  }
  bool left = x <= (double)grip;
  bool right = x >= (double)(width - grip);
  bool top = y <= (double)grip;
  bool bottom = y >= (double)(height - grip);
  if (top && left) {
    return GDK_WINDOW_EDGE_NORTH_WEST;
  }
  if (top && right) {
    return GDK_WINDOW_EDGE_NORTH_EAST;
  }
  if (bottom && left) {
    return GDK_WINDOW_EDGE_SOUTH_WEST;
  }
  if (bottom && right) {
    return GDK_WINDOW_EDGE_SOUTH_EAST;
  }
  if (left) {
    return GDK_WINDOW_EDGE_WEST;
  }
  if (right) {
    return GDK_WINDOW_EDGE_EAST;
  }
  if (top) {
    return GDK_WINDOW_EDGE_NORTH;
  }
  if (bottom) {
    return GDK_WINDOW_EDGE_SOUTH;
  }
  return -1;
}

static const char *linux_resize_cursor_name(int edge) {
  switch (edge) {
  case GDK_WINDOW_EDGE_NORTH_WEST:
    return "nw-resize";
  case GDK_WINDOW_EDGE_NORTH:
    return "ns-resize";
  case GDK_WINDOW_EDGE_NORTH_EAST:
    return "ne-resize";
  case GDK_WINDOW_EDGE_WEST:
  case GDK_WINDOW_EDGE_EAST:
    return "ew-resize";
  case GDK_WINDOW_EDGE_SOUTH_WEST:
    return "sw-resize";
  case GDK_WINDOW_EDGE_SOUTH:
    return "ns-resize";
  case GDK_WINDOW_EDGE_SOUTH_EAST:
    return "se-resize";
  default:
    return "default";
  }
}

static int linux_window_resize_edge(WoxLinuxWindow *window, double client_x, double client_y) {
  if (window == NULL || window->window == NULL || window->layer_shell_enabled || !gtk_window_get_resizable(GTK_WINDOW(window->window))) {
    return -1;
  }
  return linux_resize_hit_test(client_x, client_y, gtk_widget_get_allocated_width(window->window), gtk_widget_get_allocated_height(window->window), wox_linux_resize_grip);
}

static void apply_linux_resize_cursor(WoxLinuxWindow *window, int edge) {
  if (window == NULL) {
    return;
  }
  if (window->resize_cursor_active && window->resize_edge == edge) {
    return;
  }
  window->resize_edge = edge;
  window->resize_cursor_active = true;
  apply_linux_cursor_name(window, linux_resize_cursor_name(edge));
}

static void clear_linux_resize_cursor(WoxLinuxWindow *window) {
  if (window == NULL || !window->resize_cursor_active) {
    return;
  }
  window->resize_cursor_active = false;
  window->resize_edge = -1;
  apply_linux_pointer_cursor(window);
}

// pointer_client_position translates child-surface coordinates into the shared window client space.
static void pointer_client_position(WoxLinuxWindow *window, GdkWindow *event_window, double x, double y, double *client_x, double *client_y) {
  *client_x = x;
  *client_y = y;
  GdkWindow *top_level_window = gtk_widget_get_window(window->window);
  if (event_window == NULL || top_level_window == NULL || event_window == top_level_window) {
    return;
  }
  gint event_origin_x = 0;
  gint event_origin_y = 0;
  gint top_level_origin_x = 0;
  gint top_level_origin_y = 0;
  gdk_window_get_origin(event_window, &event_origin_x, &event_origin_y);
  gdk_window_get_origin(top_level_window, &top_level_origin_x, &top_level_origin_y);
  *client_x = x + (double)(event_origin_x - top_level_origin_x);
  *client_y = y + (double)(event_origin_y - top_level_origin_y);
}

static void store_linux_pointer_event(WoxLinuxWindow *window, double x, double y, GdkWindow *event_window, double x_root, double y_root, guint32 time) {
  double client_x = x;
  double client_y = y;
  pointer_client_position(window, event_window, x, y, &client_x, &client_y);
  window->pointer_root_x = x_root;
  window->pointer_root_y = y_root;
  window->pointer_client_x = client_x;
  window->pointer_client_y = client_y;
  window->pointer_time = time;
}

// observe_linux_pointer publishes desktop coordinates so tooltip tracking can
// dismiss glance hints after the cursor leaves, including on Wayland.
static void observe_linux_pointer(WoxLinuxWindow *window, double client_x, double client_y, bool inside) {
  float desktop_x = (float)client_x;
  float desktop_y = (float)client_y;
  if (window->layer_shell_enabled && window->has_preferred_position) {
    desktop_x += window->preferred_x;
    desktop_y += window->preferred_y;
  } else if (window->window != NULL) {
    int origin_x = 0;
    int origin_y = 0;
    gtk_window_get_position(GTK_WINDOW(window->window), &origin_x, &origin_y);
    desktop_x += (float)origin_x;
    desktop_y += (float)origin_y;
  }
  woxGoLinuxObservePointer(window->context, desktop_x, desktop_y, inside ? 1 : 0);
}

static void emit_pointer(WoxLinuxWindow *window, GdkEvent *event, uint8_t kind, double x, double y, uint8_t button, double scroll_x, double scroll_y, GdkModifierType state, GdkWindow *event_window) {
  if (!window->closed && window->context != 0) {
    double client_x = x;
    double client_y = y;
    pointer_client_position(window, event_window, x, y, &client_x, &client_y);
    observe_linux_pointer(window, client_x, client_y, kind != WOX_POINTER_LEAVE);
    window->dispatching_pointer_event = event;
    woxGoLinuxPointer(window->context, kind, (float)client_x, (float)client_y, button, (float)scroll_x, (float)scroll_y, portable_modifiers(state));
    window->dispatching_pointer_event = NULL;
  }
}

static gboolean on_pointer_motion(GtkWidget *widget, GdkEventMotion *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer) {
    return FALSE;
  }
  store_linux_pointer_event(window, event->x, event->y, event->window, event->x_root, event->y_root, event->time);
  if (window->layer_move_active) {
    update_layer_shell_move(window);
    return TRUE;
  }
  double client_x = event->x;
  double client_y = event->y;
  pointer_client_position(window, event->window, event->x, event->y, &client_x, &client_y);
  int edge = linux_window_resize_edge(window, client_x, client_y);
  if (edge >= 0) {
    if (!window->resize_cursor_active) {
      emit_pointer(window, (GdkEvent *)event, WOX_POINTER_LEAVE, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
    }
    apply_linux_resize_cursor(window, edge);
    return TRUE;
  }
  if (window->resize_cursor_active) {
    clear_linux_resize_cursor(window);
    emit_pointer(window, (GdkEvent *)event, WOX_POINTER_ENTER, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
  }
  emit_pointer(window, (GdkEvent *)event, WOX_POINTER_MOVE, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
  return TRUE;
}

static gboolean on_pointer_crossing(GtkWidget *widget, GdkEventCrossing *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer || window->updating_accessibility || window->layer_move_active) {
    return TRUE;
  }
  // Inferior crossings mean the pointer is still inside this window, usually
  // because an accessibility mirror was created or destroyed under it.
  if (event->detail == GDK_NOTIFY_INFERIOR) {
    return TRUE;
  }
  uint8_t kind = event->type == GDK_ENTER_NOTIFY ? WOX_POINTER_ENTER : WOX_POINTER_LEAVE;
  if (kind == WOX_POINTER_LEAVE) {
    clear_linux_resize_cursor(window);
    emit_pointer(window, (GdkEvent *)event, kind, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
    return TRUE;
  }
  double client_x = event->x;
  double client_y = event->y;
  pointer_client_position(window, event->window, event->x, event->y, &client_x, &client_y);
  int edge = linux_window_resize_edge(window, client_x, client_y);
  if (edge >= 0) {
    apply_linux_resize_cursor(window, edge);
    return TRUE;
  }
  emit_pointer(window, (GdkEvent *)event, kind, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
  return TRUE;
}

static gboolean on_pointer_button(GtkWidget *widget, GdkEventButton *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer) {
    return FALSE;
  }
  store_linux_pointer_event(window, event->x, event->y, event->window, event->x_root, event->y_root, event->time);
  if (window->layer_move_active) {
    if (event->type == GDK_BUTTON_RELEASE) {
      update_layer_shell_move(window);
      end_layer_shell_move(window);
      emit_pointer(window, (GdkEvent *)event, WOX_POINTER_UP, event->x, event->y, portable_pointer_button(event->button), 0.0, 0.0, event->state, event->window);
    }
    return TRUE;
  }
  if (event->type == GDK_BUTTON_PRESS && event->button == GDK_BUTTON_PRIMARY) {
    double client_x = event->x;
    double client_y = event->y;
    pointer_client_position(window, event->window, event->x, event->y, &client_x, &client_y);
    int edge = linux_window_resize_edge(window, client_x, client_y);
    if (edge >= 0) {
      gtk_window_begin_resize_drag(GTK_WINDOW(window->window), (GdkWindowEdge)edge, GDK_BUTTON_PRIMARY, (int)round(window->pointer_root_x), (int)round(window->pointer_root_y), event->time);
      return TRUE;
    }
  }
  if (event->type == GDK_BUTTON_PRESS) {
    gtk_widget_grab_focus(window->gl_area);
  }
  uint8_t kind = event->type == GDK_BUTTON_RELEASE ? WOX_POINTER_UP : WOX_POINTER_DOWN;
  emit_pointer(window, (GdkEvent *)event, kind, event->x, event->y, portable_pointer_button(event->button), 0.0, 0.0, event->state, event->window);
  return TRUE;
}

static gboolean on_pointer_scroll(GtkWidget *widget, GdkEventScroll *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer) {
    return FALSE;
  }
  double scroll_x = 0.0;
  double scroll_y = 0.0;
  double delta_x = 0.0;
  double delta_y = 0.0;
  if (gdk_event_get_scroll_deltas((GdkEvent *)event, &delta_x, &delta_y)) {
    scroll_x = delta_x * 40.0;
    scroll_y = -delta_y * 40.0;
  } else {
    switch (event->direction) {
    case GDK_SCROLL_UP:
      scroll_y = 40.0;
      break;
    case GDK_SCROLL_DOWN:
      scroll_y = -40.0;
      break;
    case GDK_SCROLL_LEFT:
      scroll_x = -40.0;
      break;
    case GDK_SCROLL_RIGHT:
      scroll_x = 40.0;
      break;
    default:
      break;
    }
  }
  emit_pointer(window, (GdkEvent *)event, WOX_POINTER_SCROLL, event->x, event->y, 0, scroll_x, scroll_y, event->state, event->window);
  return TRUE;
}

static void on_drag_data_received(GtkWidget *widget, GdkDragContext *context, gint x, gint y, GtkSelectionData *data, guint info, guint time, gpointer user_data) {
  (void)widget;
  (void)x;
  (void)y;
  (void)info;
  WoxLinuxWindow *window = user_data;
  gchar **uris = gtk_selection_data_get_uris(data);
  GString *paths = g_string_new(NULL);
  if (uris != NULL) {
    for (guint index = 0; uris[index] != NULL; index++) {
      gchar *path = g_filename_from_uri(uris[index], NULL, NULL);
      if (path == NULL || path[0] == '\0') {
        g_free(path);
        continue;
      }
      if (paths->len > 0) {
        g_string_append_c(paths, '\n');
      }
      g_string_append(paths, path);
      g_free(path);
    }
    g_strfreev(uris);
  }
  if (paths->len > 0 && window != NULL && window->context != 0) {
    woxGoLinuxFileDrop(window->context, paths->str);
  }
  g_string_free(paths, TRUE);
  gtk_drag_finish(context, TRUE, FALSE, time);
}

typedef enum {
  WOX_LAYER_BACKGROUND = 0,
  WOX_LAYER_BOTTOM = 1,
  WOX_LAYER_TOP = 2,
  WOX_LAYER_OVERLAY = 3,
} WoxLayer;

typedef enum {
  WOX_EDGE_LEFT = 0,
  WOX_EDGE_RIGHT = 1,
  WOX_EDGE_TOP = 2,
  WOX_EDGE_BOTTOM = 3,
} WoxEdge;

typedef enum {
  WOX_KEYBOARD_NONE = 0,
  WOX_KEYBOARD_EXCLUSIVE = 1,
} WoxKeyboardMode;

typedef gboolean (*WoxLayerIsSupported)(void);
typedef void (*WoxLayerInitForWindow)(GtkWindow *window);
typedef void (*WoxLayerSetLayer)(GtkWindow *window, WoxLayer layer);
typedef void (*WoxLayerSetKeyboardMode)(GtkWindow *window, WoxKeyboardMode mode);
typedef void (*WoxLayerSetAnchor)(GtkWindow *window, WoxEdge edge, gboolean anchored);
typedef void (*WoxLayerSetMonitor)(GtkWindow *window, GdkMonitor *monitor);
typedef void (*WoxLayerSetMargin)(GtkWindow *window, WoxEdge edge, int margin);

static WoxLayerIsSupported layer_is_supported;
static WoxLayerInitForWindow layer_init_for_window;
static WoxLayerSetLayer layer_set_layer;
static WoxLayerSetKeyboardMode layer_set_keyboard_mode;
static WoxLayerSetAnchor layer_set_anchor;
static WoxLayerSetMonitor layer_set_monitor;
static WoxLayerSetMargin layer_set_margin;

static bool compositor_uses_layer_shell(void) {
  const char *desktop = g_getenv("XDG_CURRENT_DESKTOP");
  if (desktop == NULL || desktop[0] == '\0') {
    desktop = g_getenv("XDG_SESSION_DESKTOP");
  }
  if (desktop == NULL || desktop[0] == '\0') {
    desktop = g_getenv("DESKTOP_SESSION");
  }
  if (desktop == NULL) {
    return false;
  }
  char *lower = g_ascii_strdown(desktop, -1);
  // Layer-shell surfaces stay off the taskbar. Plasma/KWin advertise the
  // protocol, but skip_taskbar_hint is X11-only, so without this check the
  // launcher appears as a regular xdg_toplevel in the Plasma panel.
  bool result = strstr(lower, "hyprland") != NULL || strstr(lower, "sway") != NULL || strstr(lower, "wayfire") != NULL || strstr(lower, "river") != NULL || strstr(lower, "wlroots") != NULL || strstr(lower, "kde") != NULL || strstr(lower, "plasma") != NULL;
  g_free(lower);
  return result;
}

static bool resolve_layer_shell(void) {
  static bool checked;
  static bool available;
  static void *library;
  if (checked) {
    return available;
  }
  checked = true;
  library = dlopen("libgtk-layer-shell.so.0", RTLD_LAZY | RTLD_LOCAL);
  if (library == NULL) {
    return false;
  }
#define RESOLVE_LAYER_SYMBOL(target, name) *(void **)(&target) = dlsym(library, name)
  RESOLVE_LAYER_SYMBOL(layer_is_supported, "gtk_layer_is_supported");
  RESOLVE_LAYER_SYMBOL(layer_init_for_window, "gtk_layer_init_for_window");
  RESOLVE_LAYER_SYMBOL(layer_set_layer, "gtk_layer_set_layer");
  RESOLVE_LAYER_SYMBOL(layer_set_keyboard_mode, "gtk_layer_set_keyboard_mode");
  RESOLVE_LAYER_SYMBOL(layer_set_anchor, "gtk_layer_set_anchor");
  RESOLVE_LAYER_SYMBOL(layer_set_monitor, "gtk_layer_set_monitor");
  RESOLVE_LAYER_SYMBOL(layer_set_margin, "gtk_layer_set_margin");
#undef RESOLVE_LAYER_SYMBOL
  available = layer_is_supported != NULL && layer_init_for_window != NULL && layer_set_layer != NULL && layer_set_keyboard_mode != NULL && layer_set_anchor != NULL && layer_set_monitor != NULL && layer_set_margin != NULL;
  if (!available) {
    dlclose(library);
    library = NULL;
  }
  return available;
}

static bool enable_layer_shell(GtkWindow *window) {
  if (!compositor_uses_layer_shell() || !resolve_layer_shell() || !layer_is_supported()) {
    return false;
  }
  layer_init_for_window(window);
  // TOP sits above ordinary xdg_toplevels. OVERLAY is reserved for Topmost
  // HUDs so showing the launcher cannot cover a timer or tooltip.
  layer_set_layer(window, WOX_LAYER_TOP);
  layer_set_keyboard_mode(window, WOX_KEYBOARD_EXCLUSIVE);
  layer_set_anchor(window, WOX_EDGE_TOP, TRUE);
  layer_set_anchor(window, WOX_EDGE_LEFT, TRUE);
  layer_set_anchor(window, WOX_EDGE_BOTTOM, FALSE);
  layer_set_anchor(window, WOX_EDGE_RIGHT, FALSE);
  return true;
}

// layer_shell_stack_layer maps Topmost onto the layer-shell band above the launcher.
static WoxLayer layer_shell_stack_layer(bool topmost, bool screenshot) {
  if (topmost || screenshot) {
    return WOX_LAYER_OVERLAY;
  }
  return WOX_LAYER_TOP;
}

static void apply_layer_shell_stack(WoxLinuxWindow *window) {
  if (window == NULL || !window->layer_shell_enabled || layer_set_layer == NULL || window->window == NULL) {
    return;
  }
  layer_set_layer(GTK_WINDOW(window->window), layer_shell_stack_layer(window->topmost, window->screenshot_window));
}

// raise_linux_topmost_windows puts HUD/screenshot surfaces back above a just-shown launcher.
static void raise_linux_topmost_windows(WoxLinuxWindow *except) {
  for (GList *node = linux_windows; node != NULL; node = node->next) {
    WoxLinuxWindow *window = node->data;
    if (window == except || window->closed || !window->visible || (!window->topmost && !window->screenshot_window)) {
      continue;
    }
    if (window->layer_shell_enabled) {
      apply_layer_shell_stack(window);
      continue;
    }
    GdkWindow *gdk_window = gtk_widget_get_window(window->window);
    if (gdk_window != NULL) {
      gdk_window_raise(gdk_window);
    }
  }
}

static void place_window(WoxLinuxWindow *window) {
  GdkDisplay *display = gtk_widget_get_display(window->window);
  GdkMonitor *monitor = NULL;
  if (display != NULL && window->has_preferred_position) {
    monitor = gdk_display_get_monitor_at_point(display, (int)window->preferred_x, (int)window->preferred_y);
  }
  if (monitor == NULL && display != NULL) {
    monitor = gdk_display_get_primary_monitor(display);
  }
  GdkRectangle workarea = {0, 0, (int)window->preferred_width, (int)window->preferred_height};
  if (monitor != NULL) {
    gdk_monitor_get_workarea(monitor, &workarea);
  }
  int x = window->has_preferred_position ? (int)window->preferred_x : workarea.x + (workarea.width - (int)window->preferred_width) / 2;
  int y = window->has_preferred_position ? (int)window->preferred_y : workarea.y + (workarea.height - (int)window->preferred_height) / 3;
  if (window->layer_shell_enabled) {
    if (monitor != NULL) {
      layer_set_monitor(GTK_WINDOW(window->window), monitor);
    }
    layer_set_margin(GTK_WINDOW(window->window), WOX_EDGE_LEFT, x - workarea.x);
    layer_set_margin(GTK_WINDOW(window->window), WOX_EDGE_TOP, y - workarea.y);
  } else {
    gtk_window_move(GTK_WINDOW(window->window), x, y);
  }
}

#ifdef GDK_WINDOWING_WAYLAND
static struct zwp_relative_pointer_manager_v1 *wox_relative_pointer_manager;

static void wox_relative_registry_global(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
  (void)data;
  (void)version;
  if (strcmp(interface, zwp_relative_pointer_manager_v1_interface.name) == 0) {
    wox_relative_pointer_manager = wl_registry_bind(registry, name, &zwp_relative_pointer_manager_v1_interface, 1);
  }
}

static void wox_relative_registry_remove(void *data, struct wl_registry *registry, uint32_t name) {
  (void)data;
  (void)registry;
  (void)name;
}

static const struct wl_registry_listener wox_relative_registry_listener = {
    .global = wox_relative_registry_global,
    .global_remove = wox_relative_registry_remove,
};

static void ensure_relative_pointer_manager(GdkDisplay *display) {
  static bool checked;
  if (checked || display == NULL || !GDK_IS_WAYLAND_DISPLAY(display)) {
    return;
  }
  checked = true;
  struct wl_display *wl_display = gdk_wayland_display_get_wl_display(display);
  if (wl_display == NULL) {
    return;
  }
  struct wl_registry *registry = wl_display_get_registry(wl_display);
  if (registry == NULL) {
    return;
  }
  wl_registry_add_listener(registry, &wox_relative_registry_listener, NULL);
  wl_display_roundtrip(wl_display);
}

static void on_relative_pointer_motion(void *data, struct zwp_relative_pointer_v1 *pointer, uint32_t utime_hi, uint32_t utime_lo, wl_fixed_t dx, wl_fixed_t dy, wl_fixed_t dx_unaccel, wl_fixed_t dy_unaccel) {
  (void)pointer;
  (void)utime_hi;
  (void)utime_lo;
  (void)dx_unaccel;
  (void)dy_unaccel;
  WoxLinuxWindow *window = data;
  if (!window->layer_move_active) {
    return;
  }
  // Accelerated relative motion is the cursor's on-screen movement and does
  // not invert when the layer surface moves under a stationary pointer.
  window->layer_move_pending_dx += (float)wl_fixed_to_double(dx);
  window->layer_move_pending_dy += (float)wl_fixed_to_double(dy);
}

static const struct zwp_relative_pointer_v1_listener wox_relative_pointer_listener = {
    .relative_motion = on_relative_pointer_motion,
};

static void *start_relative_pointer(WoxLinuxWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  GdkDisplay *display = gdk_window != NULL ? gdk_window_get_display(gdk_window) : gtk_widget_get_display(window->window);
  ensure_relative_pointer_manager(display);
  if (wox_relative_pointer_manager == NULL || display == NULL || !GDK_IS_WAYLAND_DISPLAY(display)) {
    return NULL;
  }
  GdkSeat *seat = gdk_display_get_default_seat(display);
  GdkDevice *pointer = seat != NULL ? gdk_seat_get_pointer(seat) : NULL;
  struct wl_pointer *wl_pointer = pointer != NULL ? gdk_wayland_device_get_wl_pointer(pointer) : NULL;
  if (wl_pointer == NULL) {
    return NULL;
  }
  struct zwp_relative_pointer_v1 *relative = zwp_relative_pointer_manager_v1_get_relative_pointer(wox_relative_pointer_manager, wl_pointer);
  if (relative == NULL) {
    return NULL;
  }
  zwp_relative_pointer_v1_add_listener(relative, &wox_relative_pointer_listener, window);
  return relative;
}

static void stop_relative_pointer(WoxLinuxWindow *window) {
  if (window->layer_move_relative_pointer == NULL) {
    return;
  }
  zwp_relative_pointer_v1_destroy(window->layer_move_relative_pointer);
  window->layer_move_relative_pointer = NULL;
}
#else
static void *start_relative_pointer(WoxLinuxWindow *window) {
  (void)window;
  return NULL;
}

static void stop_relative_pointer(WoxLinuxWindow *window) {
  (void)window;
}
#endif

static void flush_layer_shell_move(WoxLinuxWindow *window) {
  if (!window->layer_move_active) {
    return;
  }
  if (window->layer_move_pending_dx == 0 && window->layer_move_pending_dy == 0) {
    return;
  }
  window->preferred_x += window->layer_move_pending_dx;
  window->preferred_y += window->layer_move_pending_dy;
  window->layer_move_pending_dx = 0;
  window->layer_move_pending_dy = 0;
  window->has_preferred_position = true;
  place_window(window);
  if (window->layer_move_relative_pointer == NULL) {
    window->layer_move_skip_sample = true;
  }
}

static void update_layer_shell_move(WoxLinuxWindow *window) {
  if (!window->layer_move_active) {
    return;
  }
  if (window->layer_move_relative_pointer != NULL) {
    return;
  }
  float dx = (float)(window->pointer_client_x - window->layer_move_last_local_x);
  float dy = (float)(window->pointer_client_y - window->layer_move_last_local_y);
  window->layer_move_last_local_x = window->pointer_client_x;
  window->layer_move_last_local_y = window->pointer_client_y;
  if (window->layer_move_skip_sample) {
    window->layer_move_skip_sample = false;
    return;
  }
  window->layer_move_pending_dx += dx;
  window->layer_move_pending_dy += dy;
}

static gboolean layer_shell_move_tick(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
  (void)widget;
  (void)clock;
  WoxLinuxWindow *window = data;
  if (!window->layer_move_active) {
    window->layer_move_tick_id = 0;
    return G_SOURCE_REMOVE;
  }
  flush_layer_shell_move(window);
  return G_SOURCE_CONTINUE;
}

static void end_layer_shell_move(WoxLinuxWindow *window) {
  if (!window->layer_move_active) {
    return;
  }
  flush_layer_shell_move(window);
  stop_relative_pointer(window);
  if (window->layer_move_tick_id != 0 && window->gl_area != NULL) {
    gtk_widget_remove_tick_callback(window->gl_area, window->layer_move_tick_id);
    window->layer_move_tick_id = 0;
  }
  if (window->layer_move_grabbed) {
    GdkWindow *gdk_window = window->window != NULL ? gtk_widget_get_window(window->window) : NULL;
    GdkDisplay *display = gdk_window != NULL ? gdk_window_get_display(gdk_window) : NULL;
    GdkSeat *seat = display != NULL ? gdk_display_get_default_seat(display) : NULL;
    if (seat != NULL) {
      gdk_seat_ungrab(seat);
    }
    window->layer_move_grabbed = false;
  }
  window->layer_move_active = false;
}

static void start_layer_shell_move(WoxLinuxWindow *window) {
  if (window->layer_move_active) {
    return;
  }
  window->layer_move_last_local_x = window->pointer_client_x;
  window->layer_move_last_local_y = window->pointer_client_y;
  window->layer_move_pending_dx = 0;
  window->layer_move_pending_dy = 0;
  window->layer_move_skip_sample = false;
  window->has_preferred_position = true;
  window->layer_move_active = true;
  window->layer_move_grabbed = false;
  window->layer_move_relative_pointer = start_relative_pointer(window);
  if (window->layer_move_tick_id == 0 && window->gl_area != NULL) {
    window->layer_move_tick_id = gtk_widget_add_tick_callback(window->gl_area, layer_shell_move_tick, window, NULL);
  }
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  GdkDisplay *display = gdk_window != NULL ? gdk_window_get_display(gdk_window) : NULL;
  if (display == NULL) {
    return;
  }
  GdkCursor *cursor = gdk_cursor_new_from_name(display, "grabbing");
  if (cursor == NULL) {
    cursor = gdk_cursor_new_for_display(display, GDK_FLEUR);
  }
  GdkGrabStatus status = gdk_seat_grab(
      gdk_display_get_default_seat(display),
      gdk_window,
      GDK_SEAT_CAPABILITY_POINTER,
      FALSE,
      cursor,
      window->dispatching_pointer_event,
      NULL,
      NULL);
  if (cursor != NULL) {
    g_object_unref(cursor);
  }
  window->layer_move_grabbed = status == GDK_GRAB_SUCCESS;
}

#ifdef GDK_WINDOWING_X11
static GdkDisplay *x11_gdk_display(WoxLinuxWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window == NULL || !GDK_IS_X11_WINDOW(gdk_window)) {
    return NULL;
  }
  return gdk_window_get_display(gdk_window);
}

static Display *x11_display(WoxLinuxWindow *window) {
  GdkDisplay *gdk_display = x11_gdk_display(window);
  return gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
}

static Window x11_window_id(WoxLinuxWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  return gdk_window != NULL && GDK_IS_X11_WINDOW(gdk_window) ? GDK_WINDOW_XID(gdk_window) : None;
}

// Foreign XIDs can disappear before we query them. GDK's default X handler
// treats BadWindow as fatal, so this check must run inside an error trap.
static bool x11_window_is_viewable(GdkDisplay *gdk_display, Display *display, Window xid) {
  if (gdk_display == NULL || display == NULL || xid == None) {
    return false;
  }
  XWindowAttributes attributes;
  gdk_x11_display_error_trap_push(gdk_display);
  int ok = XGetWindowAttributes(display, xid, &attributes);
  if (gdk_x11_display_error_trap_pop(gdk_display) != 0 || ok == 0) {
    return false;
  }
  return attributes.map_state == IsViewable;
}

static Window active_x11_window(WoxLinuxWindow *window) {
  Display *display = x11_display(window);
  if (display == NULL) {
    return None;
  }
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", True);
  if (property == None) {
    return None;
  }
  Atom actual_type = None;
  int actual_format = 0;
  unsigned long count = 0;
  unsigned long remaining = 0;
  unsigned char *data = NULL;
  Window active = None;
  if (XGetWindowProperty(display, DefaultRootWindow(display), property, 0, 1, False, XA_WINDOW, &actual_type, &actual_format, &count, &remaining, &data) == Success && actual_type == XA_WINDOW && actual_format == 32 && count == 1 && data != NULL) {
    active = *(Window *)data;
  }
  if (data != NULL) {
    XFree(data);
  }
  return active;
}

static void save_previous_x11_window(WoxLinuxWindow *window) {
  GdkDisplay *gdk_display = x11_gdk_display(window);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window current = x11_window_id(window);
  Window active = active_x11_window(window);
  if (active != None && active != current && x11_window_is_viewable(gdk_display, display, active)) {
    window->previous_active_window = active;
    window->restore_previous_on_hide = true;
  }
}

static void apply_x11_skip_taskbar_state(WoxLinuxWindow *window) {
  GdkDisplay *gdk_display = x11_gdk_display(window);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window xid = x11_window_id(window);
  if (gdk_display == NULL || display == NULL || xid == None) {
    return;
  }
  Atom state = XInternAtom(display, "_NET_WM_STATE", False);
  Atom skip_taskbar = XInternAtom(display, "_NET_WM_STATE_SKIP_TASKBAR", False);
  Atom skip_pager = XInternAtom(display, "_NET_WM_STATE_SKIP_PAGER", False);
  if (state == None || skip_taskbar == None || skip_pager == None) {
    return;
  }
  // KWin can drop SKIP_TASKBAR on remap/activate; re-add it after every show.
  XEvent event;
  memset(&event, 0, sizeof(event));
  event.xclient.type = ClientMessage;
  event.xclient.window = xid;
  event.xclient.message_type = state;
  event.xclient.format = 32;
  event.xclient.data.l[0] = 1;
  event.xclient.data.l[1] = (long)skip_taskbar;
  event.xclient.data.l[2] = (long)skip_pager;
  event.xclient.data.l[3] = 1;
  gdk_x11_display_error_trap_push(gdk_display);
  XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  XFlush(display);
  gdk_x11_display_error_trap_pop_ignored(gdk_display);
}

static void request_x11_activation(WoxLinuxWindow *window) {
  Display *display = x11_display(window);
  Window xid = x11_window_id(window);
  if (display == NULL || xid == None) {
    return;
  }
  XRaiseWindow(display, xid);
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
  if (property != None) {
    XEvent event;
    memset(&event, 0, sizeof(event));
    event.xclient.type = ClientMessage;
    event.xclient.window = xid;
    event.xclient.message_type = property;
    event.xclient.format = 32;
    event.xclient.data.l[0] = 2;
    event.xclient.data.l[1] = CurrentTime;
    XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  }
  XFlush(display);
}

static void restore_previous_x11_window(WoxLinuxWindow *window) {
  GdkDisplay *gdk_display = x11_gdk_display(window);
  Display *display = gdk_display != NULL ? GDK_DISPLAY_XDISPLAY(gdk_display) : NULL;
  Window previous = (Window)window->previous_active_window;
  window->previous_active_window = 0;
  if (display == NULL || !x11_window_is_viewable(gdk_display, display, previous)) {
    return;
  }
  Atom property = XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
  if (property == None) {
    return;
  }
  XEvent event;
  memset(&event, 0, sizeof(event));
  event.xclient.type = ClientMessage;
  event.xclient.window = previous;
  event.xclient.message_type = property;
  event.xclient.format = 32;
  event.xclient.data.l[0] = 2;
  event.xclient.data.l[1] = CurrentTime;
  // The ClientMessage still names a foreign XID that may vanish after the
  // viewable check; keep the send inside a trap so GDK cannot abort on it.
  gdk_x11_display_error_trap_push(gdk_display);
  XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  XFlush(display);
  gdk_x11_display_error_trap_pop_ignored(gdk_display);
}
#else
static void save_previous_x11_window(WoxLinuxWindow *window) {
  (void)window;
}
static void apply_x11_skip_taskbar_state(WoxLinuxWindow *window) {
  (void)window;
}
static void request_x11_activation(WoxLinuxWindow *window) {
  (void)window;
}
static void restore_previous_x11_window(WoxLinuxWindow *window) {
  (void)window;
}
#endif

// linux_x11_compositor_present reports whether the current X screen owns the standard compositor selection.
static int linux_x11_compositor_present(WoxLinuxWindow *window) {
#ifdef GDK_WINDOWING_X11
  Display *display = x11_display(window);
  if (display != NULL) {
    char selection_name[32];
    snprintf(selection_name, sizeof(selection_name), "_NET_WM_CM_S%d", DefaultScreen(display));
    Atom selection = XInternAtom(display, selection_name, True);
    return selection != None && XGetSelectionOwner(display, selection) != None ? 1 : 0;
  }
#else
  (void)window;
#endif
  return -1;
}

// trace_linux_window_environment records the display capabilities that affect GtkGLArea presentation.
static void trace_linux_window_environment(WoxLinuxWindow *window) {
  if (!wox_linux_render_trace_enabled || window == NULL) {
    return;
  }
  GdkDisplay *display = gtk_widget_get_display(window->window);
  GdkScreen *screen = gtk_widget_get_screen(window->window);
  GdkVisual *rgba_visual = screen != NULL ? gdk_screen_get_rgba_visual(screen) : NULL;
  GdkVisual *window_visual = gtk_widget_get_visual(window->window);
  unsigned long xid = 0;
#ifdef GDK_WINDOWING_X11
  xid = (unsigned long)x11_window_id(window);
#endif
  trace_linux_render(
      "event=window_environment displayType=%s xid=%#lx x11Compositor=%d rgbaVisualAvailable=%d usingRgbaVisual=%d perPixelAlpha=%d backgroundBlur=%d layerShell=%d applicationWindow=%d",
      display != NULL ? G_OBJECT_TYPE_NAME(display) : "unknown",
      xid,
      linux_x11_compositor_present(window),
      rgba_visual != NULL,
      rgba_visual != NULL && rgba_visual == window_visual,
      window->per_pixel_alpha,
      window->background_effect != NULL,
      window->layer_shell_enabled,
      window->application_window);
}

// trace_linux_window_geometry compares requested, GTK, GDK, and GL-area sizes at one lifecycle point.
static void trace_linux_window_geometry(WoxLinuxWindow *window, const char *event) {
  if (!wox_linux_render_trace_enabled || window == NULL) {
    return;
  }
  int gtk_x = 0;
  int gtk_y = 0;
  int gtk_width = 0;
  int gtk_height = 0;
  gtk_window_get_position(GTK_WINDOW(window->window), &gtk_x, &gtk_y);
  gtk_window_get_size(GTK_WINDOW(window->window), &gtk_width, &gtk_height);
  GdkWindow *native = gtk_widget_get_window(window->window);
  int gdk_x = 0;
  int gdk_y = 0;
  int gdk_width = 0;
  int gdk_height = 0;
  if (native != NULL) {
    gdk_window_get_geometry(native, &gdk_x, &gdk_y, &gdk_width, &gdk_height);
  }
  int scale = gtk_widget_get_scale_factor(window->gl_area);
  unsigned long xid = 0;
#ifdef GDK_WINDOWING_X11
  xid = (unsigned long)x11_window_id(window);
#endif
  trace_linux_render(
      "event=%s epoch=%llu preferred=%.0f,%.0f %.0fx%.0f gtk=%d,%d %dx%d gdk=%d,%d %dx%d glArea=%dx%d overlayArea=%dx%d scale=%d visible=%d mapped=%d viewable=%d xid=%#lx",
      event,
      (unsigned long long)window->epoch,
      window->preferred_x,
      window->preferred_y,
      window->preferred_width,
      window->preferred_height,
      gtk_x,
      gtk_y,
      gtk_width,
      gtk_height,
      gdk_x,
      gdk_y,
      gdk_width,
      gdk_height,
      gtk_widget_get_allocated_width(window->gl_area),
      gtk_widget_get_allocated_height(window->gl_area),
      gtk_widget_get_allocated_width(window->overlay_gl_area),
      gtk_widget_get_allocated_height(window->overlay_gl_area),
      scale,
      window->visible,
      gtk_widget_get_mapped(window->window),
      native != NULL && gdk_window_is_viewable(native),
      xid);
}

static void destroy_linux_background_effect(WoxLinuxWindow *window);

static void linux_background_effect_info(const char *message) {
  if (message != NULL) {
    woxGoLinuxInfo(message);
  }
}

// apply_linux_background_effect attaches ext-background-effect-v1 once to the
// current toplevel wl_surface. GtkGLArea is a no-window widget, so a second
// bind on the parent surface is a protocol error. GTK also recreates the
// Wayland surface across hide/show, so a stale effect object must be dropped.
static void apply_linux_background_effect(WoxLinuxWindow *window) {
  if (window == NULL || window->closed || window->window == NULL || window->screenshot_window || !wox_linux_background_blur_available()) {
    return;
  }
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window == NULL) {
    return;
  }
  gdk_window_set_opaque_region(gdk_window, NULL);
  void *surface = wox_linux_background_effect_surface(gdk_window);
  if (surface == NULL) {
    return;
  }
  if (window->background_effect != NULL && window->background_effect_surface != surface) {
    linux_background_effect_info("linux background effect: dropping stale protocol object after wl_surface recreate");
    destroy_linux_background_effect(window);
  }
  if (window->background_effect == NULL) {
    window->background_effect = wox_linux_background_effect_attach(gdk_window);
    window->background_effect_surface = window->background_effect != NULL ? surface : NULL;
    if (window->background_effect != NULL) {
      linux_background_effect_info("linux background effect: attached ext-background-effect-v1");
    }
  }
  if (window->background_effect == NULL) {
    return;
  }
  wox_linux_background_effect_update(window->background_effect, gdk_window_get_width(gdk_window), gdk_window_get_height(gdk_window));
}

// destroy_linux_background_effect drops protocol objects before GTK destroys the wl_surface.
static void destroy_linux_background_effect(WoxLinuxWindow *window) {
  if (window == NULL) {
    return;
  }
  wox_linux_background_effect_destroy(window->background_effect);
  window->background_effect = NULL;
  window->background_effect_surface = NULL;
}

static gboolean on_window_configure(GtkWidget *widget, GdkEventConfigure *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  trace_linux_render("event=configure_event epoch=%llu eventBounds=%d,%d %dx%d", (unsigned long long)window->epoch, event->x, event->y, event->width, event->height);
  trace_linux_window_geometry(window, "configure_state");
  apply_linux_background_effect(window);
  return FALSE;
}

static gboolean on_window_map(GtkWidget *widget, GdkEvent *event, gpointer data) {
  (void)widget;
  (void)event;
  apply_linux_background_effect(data);
  trace_linux_window_geometry(data, "map");
  return FALSE;
}

static gboolean on_window_unmap(GtkWidget *widget, GdkEvent *event, gpointer data) {
  (void)widget;
  (void)event;
  destroy_linux_background_effect(data);
  trace_linux_window_geometry(data, "unmap");
  return FALSE;
}

// apply_utility_taskbar_hints keeps launcher/overlay windows out of the
// taskbar and pager. GTK skip_* hints are X11-only; Wayland relies on layer-shell.
static void apply_utility_taskbar_hints(WoxLinuxWindow *window) {
  if (window->application_window) {
    return;
  }
  gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), TRUE);
  gtk_window_set_skip_pager_hint(GTK_WINDOW(window->window), TRUE);
  apply_x11_skip_taskbar_state(window);
}

// apply_linux_window_size makes GTK's allocation match the target size before the
// next frame. gtk_window_resize alone is asynchronous, and Openbox often keeps the
// previous taller X window, so GtkGLArea is left with a black framebuffer.
static void apply_linux_window_size(WoxLinuxWindow *window, int width, int height) {
  if (width <= 0 || height <= 0) {
    return;
  }
  trace_linux_render("event=resize_begin epoch=%llu requested=%dx%d", (unsigned long long)window->epoch, width, height);
  trace_linux_window_geometry(window, "resize_before");
  gtk_window_set_default_size(GTK_WINDOW(window->window), width, height);
  // Layer-shell and non-resizable X11 windows both need the size request; a resize
  // request alone can leave the widget allocation on its pre-map or previous-result size.
  if (window->layer_shell_enabled || !gtk_window_get_resizable(GTK_WINDOW(window->window))) {
    gtk_widget_set_size_request(window->window, width, height);
  }
  gtk_window_resize(GTK_WINDOW(window->window), width, height);
  gtk_widget_queue_resize(window->window);
  gtk_container_check_resize(GTK_CONTAINER(window->window));
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window != NULL) {
    gdk_window_resize(gdk_window, width, height);
  }
#ifdef GDK_WINDOWING_X11
  GdkDisplay *gdk_display = x11_gdk_display(window);
  Display *display = x11_display(window);
  Window xid = x11_window_id(window);
  if (gdk_display != NULL && display != NULL && xid != None) {
    gdk_x11_display_error_trap_push(gdk_display);
    XResizeWindow(display, xid, (unsigned int)width, (unsigned int)height);
    XFlush(display);
    gdk_x11_display_error_trap_pop_ignored(gdk_display);
  }
#endif
  trace_linux_window_geometry(window, "resize_after_request");
}

// present_linux_window_now presents a full frame at the current allocation before
// SetBounds returns, matching Windows redrawWindowAfterResize.
static void present_linux_window_now(WoxLinuxWindow *window) {
  if (window->closed || !window->visible || window->context == 0 || window->presenting) {
    return;
  }
  trace_linux_window_geometry(window, "present_now_begin");
  window->presenting = true;
  gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window != NULL) {
    gdk_window_invalidate_rect(gdk_window, NULL, TRUE);
    G_GNUC_BEGIN_IGNORE_DEPRECATIONS
    gdk_window_process_updates(gdk_window, TRUE);
    G_GNUC_END_IGNORE_DEPRECATIONS
  }
  window->presenting = false;
  trace_linux_window_geometry(window, "present_now_end");
}

static void stop_linux_animation_frames(WoxLinuxWindow *window);

static void hide_native(WoxLinuxWindow *window, bool restore_previous) {
  if (window == NULL) {
    return;
  }
  end_layer_shell_move(window);
  if (window->closed || !window->visible) {
    return;
  }
  trace_linux_window_geometry(window, "hide_begin");
  bool should_restore = restore_previous && window->active && window->restore_previous_on_hide;
  emit_focus(window, false);
  if (window->closed) {
    return;
  }
  window->visible = false;
  stop_linux_animation_frames(window);
  if (window->renderer.ready && linux_gl_area_can_make_current(window->gl_area)) {
    gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
    if (gtk_gl_area_get_error(GTK_GL_AREA(window->gl_area)) == NULL) {
      clear_linux_resource_caches(&window->renderer, true);
    } else {
      clear_linux_resource_caches(&window->renderer, false);
    }
  } else if (window->renderer.ready) {
    clear_linux_resource_caches(&window->renderer, false);
  }
  if (window->overlay_renderer.ready && linux_gl_area_can_make_current(window->overlay_gl_area)) {
    gtk_gl_area_make_current(GTK_GL_AREA(window->overlay_gl_area));
    if (gtk_gl_area_get_error(GTK_GL_AREA(window->overlay_gl_area)) == NULL) {
      clear_linux_resource_caches(&window->overlay_renderer, true);
    } else {
      clear_linux_resource_caches(&window->overlay_renderer, false);
    }
  } else if (window->overlay_renderer.ready) {
    clear_linux_resource_caches(&window->overlay_renderer, false);
  }
  reset_large_image_policy(window);
  destroy_linux_background_effect(window);
  if (window->window != NULL) {
    gtk_widget_hide(window->window);
  }
  if (should_restore) {
    restore_previous_x11_window(window);
  }
  window->restore_previous_on_hide = false;
  window->previous_active_window = 0;
  trace_linux_window_geometry(window, "hide_end");
}

static void on_gl_realize(GtkGLArea *area, gpointer data) {
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  initialize_renderer(window, renderer, GTK_WIDGET(area));
}

static void on_gl_unrealize(GtkGLArea *area, gpointer data) {
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  trace_linux_render("event=renderer_unrealize surface=%s generation=%llu", linux_renderer_name(window, renderer), (unsigned long long)renderer->context_generation);
  destroy_renderer(renderer, GTK_WIDGET(area));
}

static void present_linux_renderer(WoxLinuxWindow *window, WoxLinuxRenderer *renderer) {
  if (renderer == NULL || renderer->frame_framebuffer == 0) {
    return;
  }
  GLint default_framebuffer = 0;
  glGetIntegerv(GL_DRAW_FRAMEBUFFER_BINDING, &default_framebuffer);
  glBindFramebuffer(GL_READ_FRAMEBUFFER, renderer->frame_framebuffer);
  glBindFramebuffer(GL_DRAW_FRAMEBUFFER, default_framebuffer);
  glBlitFramebuffer(0, 0, renderer->frame_width, renderer->frame_height, 0, 0, renderer->frame_width, renderer->frame_height, GL_COLOR_BUFFER_BIT, GL_NEAREST);
  glBindFramebuffer(GL_FRAMEBUFFER, default_framebuffer);
  glFlush();
  if (wox_linux_render_trace_enabled) {
    GLenum gl_error = glGetError();
    trace_linux_render("event=gtk_present frameId=%llu surface=%s size=%dx%d frameFramebuffer=%u defaultFramebuffer=%d glError=%#x", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), renderer->frame_width, renderer->frame_height, renderer->frame_framebuffer, default_framebuffer, (unsigned int)gl_error);
  }
}

static gboolean on_gl_render(GtkGLArea *area, GdkGLContext *context, gpointer data) {
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  trace_linux_render("event=gtk_render frameId=%llu surface=%s context=%p allocated=%dx%d scale=%d closed=%d visible=%d ready=%d", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), (void *)context, gtk_widget_get_allocated_width(GTK_WIDGET(area)), gtk_widget_get_allocated_height(GTK_WIDGET(area)), gtk_widget_get_scale_factor(GTK_WIDGET(area)), window->closed, window->visible, renderer->ready);
  if (window->closed || !window->visible || window->context == 0 || !renderer->ready) {
    return TRUE;
  }
  if (GTK_WIDGET(area) == window->overlay_gl_area) {
    if (renderer->frame_framebuffer == 0) {
      glClearColor(0.0f, 0.0f, 0.0f, 0.0f);
      glClear(GL_COLOR_BUFFER_BIT);
    } else {
      present_linux_renderer(window, renderer);
    }
    return TRUE;
  }
  int width = gtk_widget_get_allocated_width(GTK_WIDGET(area));
  int height = gtk_widget_get_allocated_height(GTK_WIDGET(area));
  int scale = gtk_widget_get_scale_factor(GTK_WIDGET(area));
  if (scale <= 0) {
    scale = 1;
  }
  if (width > 0 && height > 0) {
    window->rendering = true;
    woxGoLinuxFrame(window->context, (float)width, (float)height, width * scale, height * scale, (float)scale);
    window->rendering = false;
  }
  return TRUE;
}

static void on_scale_changed(GObject *object, GParamSpec *specification, gpointer data) {
  (void)object;
  (void)specification;
  WoxLinuxWindow *window = data;
  if (!window->closed) {
    trace_linux_window_geometry(window, "scale_changed");
    gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
  }
}

static gboolean on_focus_in(GtkWidget *widget, GdkEventFocus *event, gpointer data) {
  (void)widget;
  (void)event;
  WoxLinuxWindow *window = data;
  if (!window->closed && window->visible) {
    g_hash_table_remove_all(window->pressed_keys);
    if (window->input_enabled) {
      gtk_im_context_focus_in(window->im_context);
    }
    emit_focus(window, true);
  }
  return FALSE;
}

static gboolean on_focus_out(GtkWidget *widget, GdkEventFocus *event, gpointer data) {
  (void)widget;
  (void)event;
  WoxLinuxWindow *window = data;
  if (window->closed) {
    return FALSE;
  }
  g_hash_table_remove_all(window->pressed_keys);
  if (window->input_enabled) {
    gtk_im_context_focus_out(window->im_context);
    gtk_im_context_reset(window->im_context);
  }
  window->input_composing = false;
  if (window->native_dialog_active) {
    return FALSE;
  }
  window->restore_previous_on_hide = false;
  window->previous_active_window = 0;
  emit_focus(window, false);
  if (!window->closed && window->hide_on_blur && window->visible) {
    hide_native(window, false);
  }
  return FALSE;
}

static void on_window_destroy(GtkWidget *widget, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  destroy_linux_background_effect(window);
  end_layer_shell_move(window);
  if (window->closed) {
    return;
  }
  uintptr_t context = window->context;
  uint64_t epoch = window->epoch;
  bool active = window->active;
  window->closed = true;
  window->visible = false;
  window->active = false;
  window->context = 0;
  linux_windows = g_list_remove(linux_windows, window);
  if (window->invalidate_idle != 0) {
    g_source_remove(window->invalidate_idle);
    window->invalidate_idle = 0;
  }
  clear_active_web_view(window, false);
  g_hash_table_destroy(window->web_view_cache);
  g_hash_table_destroy(window->web_view_signatures);
  g_hash_table_destroy(window->web_view_content_keys);
  window->web_view_cache = NULL;
  window->web_view_signatures = NULL;
  window->web_view_content_keys = NULL;
  gtk_im_context_set_client_window(window->im_context, NULL);
  g_object_unref(window->im_context);
  window->im_context = NULL;
  g_hash_table_destroy(window->pressed_keys);
  window->pressed_keys = NULL;
  if (context != 0) {
    if (g_atomic_int_get(&wox_linux_window_count) > 0) {
      g_atomic_int_add(&wox_linux_window_count, -1);
    }
    woxGoLinuxDestroyed(context, epoch, active ? 1 : 0);
    if (g_atomic_int_get(&wox_linux_window_count) == 0 && gtk_main_level() > 0) {
      gtk_main_quit();
    }
  }
  // ponytail: Keep this small closed handle alive so concurrent Go calls cannot observe freed memory; add reference counting if repeated window creation becomes measurable.
}

int32_t wox_linux_run(uintptr_t context) {
  if (context == 0 || g_atomic_int_get(&wox_linux_runtime_running) != 0) {
    return -1;
  }
#ifdef GDK_WINDOWING_X11
  // GTK may call Xlib during initialization, so thread support must be enabled first.
  if (XInitThreads() == 0) {
    return -3;
  }
#endif
  apply_linux_app_identity();
  if (!gtk_init_check(NULL, NULL)) {
    return -2;
  }
  apply_linux_app_icon();
  wox_linux_background_effect_probe(gdk_display_get_default());
  wox_linux_main_thread = pthread_self();
  g_atomic_int_set(&wox_linux_runtime_running, 1);
  int32_t start_result = woxGoLinuxStart(context);
  if (start_result == 0 && g_atomic_int_get(&wox_linux_window_count) > 0) {
    g_atomic_int_set(&wox_linux_loop_active, 1);
    gtk_main();
    g_atomic_int_set(&wox_linux_loop_active, 0);
  }
  g_atomic_int_set(&wox_linux_runtime_running, 0);
  return start_result == 0 ? 0 : -1;
}

static void apply_linux_rgba_visual(GtkWidget *widget) {
  GdkScreen *screen = gtk_widget_get_screen(widget);
  if (screen == NULL) {
    return;
  }
  GdkVisual *visual = gdk_screen_get_rgba_visual(screen);
  if (visual != NULL) {
    gtk_widget_set_visual(widget, visual);
  }
}

static gboolean on_linux_transparent_draw(GtkWidget *widget, cairo_t *cr, gpointer data) {
  (void)widget;
  (void)data;
  cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
  cairo_set_source_rgba(cr, 0.0, 0.0, 0.0, 0.0);
  cairo_paint(cr);
  return FALSE;
}

static void on_linux_transparent_screen_changed(GtkWidget *widget, GdkScreen *previous, gpointer data) {
  (void)previous;
  (void)data;
  apply_linux_rgba_visual(widget);
}

// Recording chrome and screenshot toolbars call Clear(Color{}) so the live
// desktop shows through. Launcher windows stay opaque unless the compositor
// advertises ext-background-effect-v1 blur, in which case the theme wash
// keeps its alpha and the compositor blurs the desktop behind it.
static void enable_linux_per_pixel_alpha(WoxLinuxWindow *window) {
  GtkCssProvider *provider = gtk_css_provider_new();
  gtk_css_provider_load_from_data(provider, "window, overlay { background-color: rgba(0,0,0,0); background-image: none; }", -1, NULL);

  GtkWidget *widgets[] = {window->window, window->overlay};
  for (size_t index = 0; index < G_N_ELEMENTS(widgets); index++) {
    GtkWidget *widget = widgets[index];
    apply_linux_rgba_visual(widget);
    gtk_widget_set_app_paintable(widget, TRUE);
    gtk_style_context_add_provider(gtk_widget_get_style_context(widget), GTK_STYLE_PROVIDER(provider), GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
    g_signal_connect(widget, "screen-changed", G_CALLBACK(on_linux_transparent_screen_changed), window);
    g_signal_connect(widget, "draw", G_CALLBACK(on_linux_transparent_draw), window);
  }
  apply_linux_rgba_visual(window->gl_area);
  apply_linux_rgba_visual(window->overlay_gl_area);
  g_object_unref(provider);
}

static void apply_pointer_passthrough_to_gdk_window(GdkWindow *native, bool enabled) {
  if (native == NULL) {
    return;
  }
  gdk_window_set_pass_through(native, enabled);
  // gdk_window_set_pass_through only affects in-process picking. Wayland and
  // X11 child GL surfaces still receive compositor input unless the input
  // region is empty, which is why the recording border ate toolbar clicks.
  if (enabled) {
    cairo_region_t *region = cairo_region_create();
    gdk_window_input_shape_combine_region(native, region, 0, 0);
    cairo_region_destroy(region);
    return;
  }
  gdk_window_input_shape_combine_region(native, NULL, 0, 0);
}

static void apply_linux_pointer_passthrough(WoxLinuxWindow *window) {
  if (window == NULL || window->closed) {
    return;
  }
  apply_pointer_passthrough_to_gdk_window(gtk_widget_get_window(window->window), window->pointer_passthrough);
  apply_pointer_passthrough_to_gdk_window(gtk_widget_get_window(window->overlay), window->pointer_passthrough);
  apply_pointer_passthrough_to_gdk_window(gtk_widget_get_window(window->gl_area), window->pointer_passthrough);
  apply_pointer_passthrough_to_gdk_window(gtk_widget_get_window(window->overlay_gl_area), window->pointer_passthrough);
}

static void on_linux_pointer_passthrough_realize(GtkWidget *widget, gpointer data) {
  (void)widget;
  apply_linux_pointer_passthrough(data);
}

// apply_window_geometry_hints keeps aspect and min-size constraints on the same GdkGeometry.
static void apply_window_geometry_hints(WoxLinuxWindow *window) {
  if (window == NULL || window->window == NULL) {
    return;
  }
  GdkGeometry geometry = {0};
  GdkWindowHints hints = 0;
  if (window->aspect_ratio > 0.0f) {
    geometry.min_aspect = window->aspect_ratio;
    geometry.max_aspect = window->aspect_ratio;
    hints |= GDK_HINT_ASPECT;
  }
  if (window->min_width > 0.0f || window->min_height > 0.0f) {
    geometry.min_width = (int)ceilf(window->min_width);
    geometry.min_height = (int)ceilf(window->min_height);
    hints |= GDK_HINT_MIN_SIZE;
  }
  if (hints != 0) {
    gtk_window_set_geometry_hints(GTK_WINDOW(window->window), NULL, &geometry, hints);
  }
}

WoxLinuxWindow *wox_linux_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t window_role, int32_t nonactivating, int32_t resizable, float aspect_ratio, uintptr_t context) {
  if (!is_main_thread() || width <= 0.0f || height <= 0.0f || context == 0) {
    return NULL;
  }
  WoxLinuxWindow *window = calloc(1, sizeof(WoxLinuxWindow));
  if (window == NULL) {
    return NULL;
  }
  window->preferred_width = width;
  window->preferred_height = height;
  window->hide_on_blur = hide_on_blur != 0;
  bool application_window = window_role == WOX_WINDOW_ROLE_APPLICATION;
  window->application_window = application_window;
  window->screenshot_window = window_role == WOX_WINDOW_ROLE_SCREENSHOT;
  window->nonactivating = nonactivating != 0;
  window->per_pixel_alpha = window->nonactivating || window->screenshot_window || wox_linux_background_blur_available();
  window->im_context = gtk_im_multicontext_new();
  window->pressed_keys = g_hash_table_new(g_direct_hash, g_direct_equal);
  window->web_view_cache = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_object_unref);
  window->web_view_signatures = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
  window->web_view_content_keys = g_hash_table_new_full(g_str_hash, g_str_equal, g_free, g_free);
  if (window->im_context == NULL || window->pressed_keys == NULL || window->web_view_cache == NULL || window->web_view_signatures == NULL || window->web_view_content_keys == NULL) {
    if (window->im_context != NULL) {
      g_object_unref(window->im_context);
    }
    if (window->pressed_keys != NULL) {
      g_hash_table_destroy(window->pressed_keys);
    }
    if (window->web_view_cache != NULL) {
      g_hash_table_destroy(window->web_view_cache);
    }
    if (window->web_view_signatures != NULL) {
      g_hash_table_destroy(window->web_view_signatures);
    }
    if (window->web_view_content_keys != NULL) {
      g_hash_table_destroy(window->web_view_content_keys);
    }
    free(window);
    return NULL;
  }
  window->window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
  window->overlay = gtk_overlay_new();
  window->gl_area = gtk_gl_area_new();
  window->overlay_gl_area = gtk_gl_area_new();
  window->accessibility_layer = gtk_fixed_new();
  gtk_window_set_title(GTK_WINDOW(window->window), title != NULL ? title : "Wox Go UI");
  gtk_window_set_default_size(GTK_WINDOW(window->window), (int)ceilf(width), (int)ceilf(height));
#ifdef GDK_WINDOWING_WAYLAND
  GdkDisplay *window_display = gtk_widget_get_display(window->window);
  if (window_display != NULL && GDK_IS_WAYLAND_DISPLAY(window_display)) {
    // Force CSD negotiation before disabling it so SSD-preferring compositors do not add a title bar.
    GtkWidget *empty_titlebar = gtk_fixed_new();
    gtk_widget_set_size_request(empty_titlebar, 0, 0);
    gtk_window_set_titlebar(GTK_WINDOW(window->window), empty_titlebar);
  }
#endif
  gtk_window_set_decorated(GTK_WINDOW(window->window), FALSE);
  // Frameless windows still need gtk_window_begin_resize_drag from the edge hit
  // test; gtk_window_set_resizable only tells the WM that interactive resize is allowed.
  gtk_window_set_resizable(GTK_WINDOW(window->window), resizable != 0);
  window->resize_edge = -1;
  window->aspect_ratio = aspect_ratio;
  apply_window_geometry_hints(window);
  // Application windows must stay visible to the desktop shell instead of using launcher-only utility hints.
  if (application_window) {
    gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), FALSE);
    gtk_window_set_skip_pager_hint(GTK_WINDOW(window->window), FALSE);
    gtk_window_set_type_hint(GTK_WINDOW(window->window), GDK_WINDOW_TYPE_HINT_NORMAL);
    gtk_window_set_keep_above(GTK_WINDOW(window->window), FALSE);
  } else {
    gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), TRUE);
    gtk_window_set_skip_pager_hint(GTK_WINDOW(window->window), TRUE);
    gtk_window_set_type_hint(GTK_WINDOW(window->window), GDK_WINDOW_TYPE_HINT_UTILITY);
    gtk_window_set_keep_above(GTK_WINDOW(window->window), TRUE);
  }
  gtk_window_set_icon_name(GTK_WINDOW(window->window), linux_app_id());
  gtk_window_set_accept_focus(GTK_WINDOW(window->window), !window->nonactivating);
  gtk_window_set_focus_on_map(GTK_WINDOW(window->window), !window->nonactivating);
  gtk_window_set_position(GTK_WINDOW(window->window), GTK_WIN_POS_CENTER);
  gtk_widget_set_app_paintable(window->window, TRUE);
  if (window->per_pixel_alpha) {
    enable_linux_per_pixel_alpha(window);
  }
  GtkTargetEntry file_drop_target = {(gchar *)"text/uri-list", 0, 0};
  gtk_drag_dest_set(window->window, GTK_DEST_DEFAULT_ALL, &file_drop_target, 1, GDK_ACTION_COPY);

  window->layer_shell_enabled = !application_window && enable_layer_shell(GTK_WINDOW(window->window));
  if (window->layer_shell_enabled) {
    apply_layer_shell_stack(window);
    if (window->nonactivating) {
      // Exclusive keyboard on the fullscreen recording border steals Escape from
      // the screenshot toolbar. Nonactivating chrome must not own the seat.
      layer_set_keyboard_mode(GTK_WINDOW(window->window), WOX_KEYBOARD_NONE);
    }
  }

  gtk_gl_area_set_required_version(GTK_GL_AREA(window->gl_area), 3, 3);
  gtk_gl_area_set_use_es(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_has_alpha(GTK_GL_AREA(window->gl_area), window->per_pixel_alpha);
  gtk_gl_area_set_has_depth_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_has_stencil_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_auto_render(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_widget_set_can_focus(window->gl_area, !window->nonactivating);
  gtk_widget_set_hexpand(window->gl_area, TRUE);
  gtk_widget_set_vexpand(window->gl_area, TRUE);
  gtk_gl_area_set_required_version(GTK_GL_AREA(window->overlay_gl_area), 3, 3);
  gtk_gl_area_set_use_es(GTK_GL_AREA(window->overlay_gl_area), FALSE);
  gtk_gl_area_set_has_alpha(GTK_GL_AREA(window->overlay_gl_area), TRUE);
  gtk_gl_area_set_has_depth_buffer(GTK_GL_AREA(window->overlay_gl_area), FALSE);
  gtk_gl_area_set_has_stencil_buffer(GTK_GL_AREA(window->overlay_gl_area), FALSE);
  gtk_gl_area_set_auto_render(GTK_GL_AREA(window->overlay_gl_area), FALSE);
  gtk_widget_set_can_focus(window->overlay_gl_area, FALSE);
  gtk_widget_set_hexpand(window->overlay_gl_area, TRUE);
  gtk_widget_set_vexpand(window->overlay_gl_area, TRUE);
  // GTK bubbles child input to the top level, while Wayland may target the top-level surface
  // directly. Use one handler set there so a gesture cannot switch dispatch paths mid-stream.
  gtk_widget_add_events(window->window, GDK_POINTER_MOTION_MASK | GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK | GDK_ENTER_NOTIFY_MASK | GDK_LEAVE_NOTIFY_MASK | GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
  gtk_container_add(GTK_CONTAINER(window->window), window->overlay);
  gtk_container_add(GTK_CONTAINER(window->overlay), window->gl_area);
  gtk_overlay_add_overlay(GTK_OVERLAY(window->overlay), window->overlay_gl_area);
  gtk_overlay_set_overlay_pass_through(GTK_OVERLAY(window->overlay), window->overlay_gl_area, FALSE);
  gtk_overlay_add_overlay(GTK_OVERLAY(window->overlay), window->accessibility_layer);
  gtk_overlay_set_overlay_pass_through(GTK_OVERLAY(window->overlay), window->accessibility_layer, TRUE);
  gtk_widget_set_opacity(window->accessibility_layer, 0.0);
  gtk_widget_set_halign(window->accessibility_layer, GTK_ALIGN_FILL);
  gtk_widget_set_valign(window->accessibility_layer, GTK_ALIGN_FILL);
  gtk_widget_show(window->overlay);
  gtk_widget_show(window->gl_area);
  gtk_widget_show(window->overlay_gl_area);
  gtk_widget_show(window->accessibility_layer);

  g_signal_connect(window->gl_area, "realize", G_CALLBACK(on_gl_realize), window);
  g_signal_connect(window->gl_area, "realize", G_CALLBACK(on_linux_pointer_passthrough_realize), window);
  g_signal_connect(window->gl_area, "unrealize", G_CALLBACK(on_gl_unrealize), window);
  g_signal_connect(window->gl_area, "render", G_CALLBACK(on_gl_render), window);
  g_signal_connect(window->gl_area, "notify::scale-factor", G_CALLBACK(on_scale_changed), window);
  g_signal_connect(window->overlay_gl_area, "realize", G_CALLBACK(on_gl_realize), window);
  g_signal_connect(window->overlay_gl_area, "realize", G_CALLBACK(on_linux_pointer_passthrough_realize), window);
  g_signal_connect(window->overlay_gl_area, "unrealize", G_CALLBACK(on_gl_unrealize), window);
  g_signal_connect(window->overlay_gl_area, "render", G_CALLBACK(on_gl_render), window);
  g_signal_connect(window->window, "motion-notify-event", G_CALLBACK(on_pointer_motion), window);
  g_signal_connect(window->window, "enter-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(window->window, "leave-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(window->window, "button-press-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(window->window, "button-release-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(window->window, "scroll-event", G_CALLBACK(on_pointer_scroll), window);
  g_signal_connect(window->window, "drag-data-received", G_CALLBACK(on_drag_data_received), window);
  g_signal_connect(window->window, "focus-in-event", G_CALLBACK(on_focus_in), window);
  g_signal_connect(window->window, "focus-out-event", G_CALLBACK(on_focus_out), window);
  g_signal_connect(window->window, "configure-event", G_CALLBACK(on_window_configure), window);
  g_signal_connect(window->window, "map-event", G_CALLBACK(on_window_map), window);
  g_signal_connect(window->window, "unmap-event", G_CALLBACK(on_window_unmap), window);
  g_signal_connect(window->window, "key-press-event", G_CALLBACK(on_key_press), window);
  g_signal_connect(window->window, "key-release-event", G_CALLBACK(on_key_release), window);
  g_signal_connect(window->window, "realize", G_CALLBACK(on_linux_pointer_passthrough_realize), window);
  g_signal_connect(window->window, "destroy", G_CALLBACK(on_window_destroy), window);
  g_signal_connect(window->im_context, "commit", G_CALLBACK(on_ime_commit), window);
  g_signal_connect(window->im_context, "preedit-changed", G_CALLBACK(on_ime_preedit_changed), window);

  gtk_widget_realize(window->window);
  gtk_widget_realize(window->gl_area);
  gtk_widget_realize(window->overlay_gl_area);
  apply_linux_background_effect(window);
  trace_linux_window_environment(window);
  trace_linux_window_geometry(window, "window_created");
  apply_wayland_app_id(window);
  gtk_im_context_set_client_window(window->im_context, gtk_widget_get_window(window->window));
  if (!window->renderer.ready || !window->overlay_renderer.ready) {
    gtk_widget_destroy(window->window);
    free(window);
    return NULL;
  }
  window->context = context;
  linux_windows = g_list_prepend(linux_windows, window);
  g_atomic_int_inc(&wox_linux_window_count);
  return window;
}

typedef struct {
  WoxLinuxWindow *window;
  uint64_t epoch;
  int32_t result;
} WoxWindowCall;

static void show_main(void *data) {
  WoxWindowCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  if (window->active) {
    // Starting a new focus epoch is not a real focus loss.
    window->active = false;
  }
  window->epoch++;
  call->epoch = window->epoch;
  window->visible = true;
  trace_linux_window_geometry(window, "show_begin");
  save_previous_x11_window(window);
  place_window(window);
  gtk_widget_show_all(window->window);
  apply_linux_background_effect(window);
  apply_linux_pointer_passthrough(window);
  apply_utility_taskbar_hints(window);
  apply_wayland_app_id(window);
  apply_linux_window_size(window, (int)ceilf(window->preferred_width), (int)ceilf(window->preferred_height));
  place_window(window);
  present_linux_window_now(window);
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window != NULL) {
    gdk_window_raise(gdk_window);
    if (!window->nonactivating) {
      gdk_window_focus(gdk_window, GDK_CURRENT_TIME);
    }
  }
  if (!window->nonactivating) {
    request_x11_activation(window);
    gtk_window_present(GTK_WINDOW(window->window));
    gtk_widget_grab_focus(window->gl_area);
  }
  apply_utility_taskbar_hints(window);
  raise_linux_topmost_windows(window);
  trace_linux_window_geometry(window, "show_end");
}

uint64_t wox_linux_window_show(WoxLinuxWindow *window) {
  if (window == NULL) {
    return 0;
  }
  WoxWindowCall call = {.window = window};
  if (!run_on_main_sync(show_main, &call) || call.result != 0) {
    return 0;
  }
  return call.epoch;
}

static void hide_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  hide_native(call->window, true);
}

int32_t wox_linux_window_hide(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(hide_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  float x;
  float y;
  float width;
  float height;
  int32_t result;
} WoxBoundsCall;

static void set_bounds_main(void *data) {
  WoxBoundsCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  trace_linux_render("event=set_bounds_begin epoch=%llu requested=%.0f,%.0f %.0fx%.0f", (unsigned long long)window->epoch, call->x, call->y, call->width, call->height);
  trace_linux_window_geometry(window, "set_bounds_before");
  // A layer-shell drag owns the live position. Go resizes still apply, but
  // writing the pre-drag origin would fight the pointer and flicker.
  if (!window->layer_move_active) {
    window->preferred_x = call->x;
    window->preferred_y = call->y;
  }
  window->preferred_width = call->width;
  window->preferred_height = call->height;
  window->has_preferred_position = true;
  int width = (int)ceilf(call->width);
  int height = (int)ceilf(call->height);
  apply_linux_window_size(window, width, height);
  if (!window->layer_move_active) {
    place_window(window);
  }
  apply_linux_pointer_passthrough(window);
  if (window->visible && !window->layer_move_active) {
    present_linux_window_now(window);
  }
  trace_linux_window_geometry(window, "set_bounds_after");
}

int32_t wox_linux_window_set_bounds(WoxLinuxWindow *window, float x, float y, float width, float height) {
  if (window == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxBoundsCall call = {.window = window, .x = x, .y = y, .width = width, .height = height};
  return run_on_main_sync(set_bounds_main, &call) ? call.result : -1;
}

static void get_bounds_main(void *data) {
  WoxBoundsCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  int x = 0;
  int y = 0;
  int width = 0;
  int height = 0;
  if (window->layer_shell_enabled && window->has_preferred_position) {
    // Wayland does not expose global top-level coordinates. Preserve the logical position
    // used for layer-shell margins so a resize does not move the launcher to the origin.
    x = (int)window->preferred_x;
    y = (int)window->preferred_y;
  } else {
    gtk_window_get_position(GTK_WINDOW(window->window), &x, &y);
  }
  gtk_window_get_size(GTK_WINDOW(window->window), &width, &height);
  call->x = (float)x;
  call->y = (float)y;
  call->width = (float)width;
  call->height = (float)height;
}

int32_t wox_linux_window_get_bounds(WoxLinuxWindow *window, float *x, float *y, float *width, float *height) {
  if (window == NULL || x == NULL || y == NULL || width == NULL || height == NULL) {
    return -1;
  }
  WoxBoundsCall call = {.window = window};
  if (!run_on_main_sync(get_bounds_main, &call) || call.result != 0) {
    return -1;
  }
  *x = call.x;
  *y = call.y;
  *width = call.width;
  *height = call.height;
  return 0;
}

typedef struct {
  WoxLinuxWindow *window;
  const char *path;
  int32_t result;
} WoxCaptureCall;

static void capture_png_main(void *data) {
  WoxCaptureCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed || !window->visible) {
    call->result = -1;
    return;
  }
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  int width = gtk_widget_get_allocated_width(window->window);
  int height = gtk_widget_get_allocated_height(window->window);
  if (gdk_window == NULL || width <= 0 || height <= 0) {
    call->result = -1;
    return;
  }
  GdkPixbuf *pixels = gdk_pixbuf_get_from_window(gdk_window, 0, 0, width, height);
  if (pixels == NULL) {
    call->result = -1;
    return;
  }
  GError *error = NULL;
  if (!gdk_pixbuf_save(pixels, call->path, "png", &error, NULL)) {
    if (error != NULL) {
      g_error_free(error);
    }
    call->result = -1;
  }
  g_object_unref(pixels);
}

int32_t wox_linux_window_capture_png(WoxLinuxWindow *window, const char *path) {
  if (window == NULL || path == NULL || path[0] == '\0') {
    return -1;
  }
  WoxCaptureCall call = {.window = window, .path = path};
  return run_on_main_sync(capture_png_main, &call) ? call.result : -1;
}

typedef struct {
  const char *path;
  float x;
  float y;
  float width;
  float height;
  int32_t result;
} WoxDesktopCaptureCall;

// capture_desktop_png_main captures the X11 root window; Wayland requires a portal-owned flow.
static void capture_desktop_png_main(void *data) {
  WoxDesktopCaptureCall *call = data;
#ifdef GDK_WINDOWING_X11
  GdkDisplay *display = gdk_display_get_default();
  if (display == NULL || !GDK_IS_X11_DISPLAY(display)) {
    call->result = -2;
    return;
  }
  Display *xdisplay = GDK_DISPLAY_XDISPLAY(display);
  Window xroot = DefaultRootWindow(xdisplay);
  XWindowAttributes attributes;
  if (!XGetWindowAttributes(xdisplay, xroot, &attributes) || attributes.width <= 0 || attributes.height <= 0) {
    call->result = -1;
    return;
  }
  GdkWindow *root = gdk_x11_window_lookup_for_display(display, xroot);
  bool owns_root = false;
  if (root == NULL) {
    root = gdk_x11_window_foreign_new_for_display(display, xroot);
    owns_root = root != NULL;
  }
  if (root == NULL) {
    call->result = -1;
    return;
  }
  int width = gdk_window_get_width(root);
  int height = gdk_window_get_height(root);
  if (width <= 0 || height <= 0) {
    if (owns_root) {
      g_object_unref(root);
    }
    call->result = -1;
    return;
  }
  GdkPixbuf *pixels = gdk_pixbuf_get_from_window(root, 0, 0, width, height);
  if (owns_root) {
    g_object_unref(root);
  }
  if (pixels == NULL) {
    call->result = -1;
    return;
  }
  GError *error = NULL;
  if (!gdk_pixbuf_save(pixels, call->path, "png", &error, NULL)) {
    if (error != NULL) {
      g_error_free(error);
    }
    call->result = -1;
  } else {
    call->x = 0.0f;
    call->y = 0.0f;
    call->width = (float)width;
    call->height = (float)height;
  }
  g_object_unref(pixels);
#else
  call->result = -2;
#endif
}

int32_t wox_linux_capture_desktop_png(const char *path, float *x, float *y, float *width, float *height) {
  if (path == NULL || path[0] == '\0' || x == NULL || y == NULL || width == NULL || height == NULL) {
    return -1;
  }
  WoxDesktopCaptureCall call = {.path = path};
  if (!run_on_main_sync(capture_desktop_png_main, &call) || call.result != 0) {
    return call.result != 0 ? call.result : -1;
  }
  *x = call.x;
  *y = call.y;
  *width = call.width;
  *height = call.height;
  return 0;
}

static void center_main(void *data) {
  WoxBoundsCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  GdkDisplay *display = gtk_widget_get_display(window->window);
  GdkMonitor *monitor = NULL;
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (display != NULL && gdk_window != NULL) {
    monitor = gdk_display_get_monitor_at_window(display, gdk_window);
  }
  if (monitor == NULL && display != NULL && window->has_preferred_position) {
    monitor = gdk_display_get_monitor_at_point(display, (int)window->preferred_x, (int)window->preferred_y);
  }
  if (monitor == NULL && display != NULL) {
    monitor = gdk_display_get_primary_monitor(display);
  }
  GdkRectangle workarea = {0, 0, (int)call->width, (int)call->height};
  if (monitor != NULL) {
    gdk_monitor_get_workarea(monitor, &workarea);
  }
  float width = fminf(call->width, (float)workarea.width);
  float height = fminf(call->height, (float)workarea.height);
  window->preferred_width = width;
  window->preferred_height = height;
  window->preferred_x = workarea.x + (workarea.width - width) * 0.5f;
  window->preferred_y = workarea.y + (workarea.height - height) * 0.5f;
  window->has_preferred_position = true;
  apply_linux_window_size(window, (int)ceilf(width), (int)ceilf(height));
  place_window(window);
  if (window->visible) {
    present_linux_window_now(window);
  }
}

int32_t wox_linux_window_center(WoxLinuxWindow *window, float width, float height) {
  if (window == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxBoundsCall call = {.window = window, .width = width, .height = height};
  return run_on_main_sync(center_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  int32_t result;
} WoxDragCall;

static void start_dragging_main(void *data) {
  WoxDragCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  // Layer-shell launchers are not xdg_toplevels. GDK's begin_move_drag returns
  // immediately on those surfaces, which is why query/toolbar drags appear dead
  // while application windows such as Notes still move.
  if (window->layer_shell_enabled) {
    start_layer_shell_move(window);
    return;
  }
  gtk_window_begin_move_drag(GTK_WINDOW(window->window), GDK_BUTTON_PRIMARY, (int)round(window->pointer_root_x), (int)round(window->pointer_root_y), window->pointer_time);
}

int32_t wox_linux_window_start_dragging(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxDragCall call = {.window = window};
  return run_on_main_sync(start_dragging_main, &call) ? call.result : -1;
}

static void minimize_main(void *data) {
  WoxDragCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  gtk_window_iconify(GTK_WINDOW(call->window->window));
}

int32_t wox_linux_window_minimize(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxDragCall call = {.window = window};
  return run_on_main_sync(minimize_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  bool enabled;
  int32_t result;
} WoxBoolCall;

static void set_hide_on_blur_main(void *data) {
  WoxBoolCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  call->window->hide_on_blur = call->enabled;
}

int32_t wox_linux_window_set_hide_on_blur(WoxLinuxWindow *window, int32_t enabled) {
  if (window == NULL) {
    return -1;
  }
  WoxBoolCall call = {.window = window, .enabled = enabled != 0};
  return run_on_main_sync(set_hide_on_blur_main, &call) ? call.result : -1;
}

static void set_topmost_main(void *data) {
  WoxBoolCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed || window->window == NULL) {
    call->result = -1;
    return;
  }
  window->topmost = call->enabled;
  if (window->layer_shell_enabled) {
    apply_layer_shell_stack(window);
    return;
  }
  gtk_window_set_keep_above(GTK_WINDOW(window->window), window->topmost || !window->application_window);
  if (!window->topmost) {
    return;
  }
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window != NULL) {
    gdk_window_raise(gdk_window);
  }
}

int32_t wox_linux_window_set_topmost(WoxLinuxWindow *window, int32_t enabled) {
  if (window == NULL) {
    return -1;
  }
  WoxBoolCall call = {.window = window, .enabled = enabled != 0};
  return run_on_main_sync(set_topmost_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  float width;
  float height;
  int32_t result;
} WoxMinSizeCall;

static void set_min_size_main(void *data) {
  WoxMinSizeCall *call = data;
  if (call->window->closed || call->window->window == NULL) {
    call->result = -1;
    return;
  }
  call->window->min_width = call->width;
  call->window->min_height = call->height;
  apply_window_geometry_hints(call->window);
}

int32_t wox_linux_window_set_min_size(WoxLinuxWindow *window, float width, float height) {
  if (window == NULL) {
    return -1;
  }
  WoxMinSizeCall call = {.window = window, .width = width, .height = height};
  return run_on_main_sync(set_min_size_main, &call) ? call.result : -1;
}

static void free_pixbuf_pixels(guchar *pixels, gpointer data);

typedef struct {
  WoxLinuxWindow *window;
  const uint8_t *pixels;
  int width;
  int height;
  int row_stride;
  int32_t result;
} WoxWindowIconCall;

static void set_icon_main(void *data) {
  WoxWindowIconCall *call = data;
  if (call->window->closed || call->window->window == NULL || call->pixels == NULL || call->width <= 0 || call->height <= 0 || call->row_stride < call->width * 4) {
    call->result = -1;
    return;
  }
  size_t byte_count = (size_t)call->row_stride * (size_t)call->height;
  guchar *copy = g_malloc(byte_count);
  if (copy == NULL) {
    call->result = -1;
    return;
  }
  memcpy(copy, call->pixels, byte_count);
  GdkPixbuf *pixbuf = gdk_pixbuf_new_from_data(
      copy,
      GDK_COLORSPACE_RGB,
      TRUE,
      8,
      call->width,
      call->height,
      call->row_stride,
      free_pixbuf_pixels,
      NULL);
  if (pixbuf == NULL) {
    g_free(copy);
    call->result = -1;
    return;
  }
  gtk_window_set_icon(GTK_WINDOW(call->window->window), pixbuf);
  g_object_unref(pixbuf);
}

int32_t wox_linux_window_set_icon(WoxLinuxWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowIconCall call = {.window = window, .pixels = pixels, .width = width, .height = height, .row_stride = row_stride};
  return run_on_main_sync(set_icon_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  bool directory;
  char *path;
  int32_t result;
} WoxFileDialogCall;

static void pick_file_main(void *data) {
  WoxFileDialogCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }

  GtkFileChooserAction action = call->directory ? GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER : GTK_FILE_CHOOSER_ACTION_OPEN;
  GtkFileChooserNative *dialog = gtk_file_chooser_native_new(
      call->directory ? "Select Folder" : "Select File",
      GTK_WINDOW(window->window),
      action,
      "_Open",
      "_Cancel");
  if (dialog == NULL) {
    call->result = -1;
    return;
  }
  gtk_native_dialog_set_modal(GTK_NATIVE_DIALOG(dialog), TRUE);

  // Keep the transient picker inside the Wox focus domain while GTK runs its nested dialog loop.
  window->native_dialog_active = true;
  gint response = gtk_native_dialog_run(GTK_NATIVE_DIALOG(dialog));
  window->native_dialog_active = false;
  if (response == GTK_RESPONSE_ACCEPT) {
    call->path = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dialog));
    if (call->path == NULL) {
      call->result = -1;
    }
  } else {
    call->result = 1;
  }
  g_object_unref(dialog);

  if (!window->closed && window->visible) {
    gtk_window_present(GTK_WINDOW(window->window));
    gtk_widget_grab_focus(window->gl_area);
  }
}

int32_t wox_linux_window_pick_file(WoxLinuxWindow *window, int32_t directory, char **path) {
  if (window == NULL || path == NULL) {
    return -1;
  }
  WoxFileDialogCall call = {.window = window, .directory = directory != 0};
  if (!run_on_main_sync(pick_file_main, &call)) {
    return -1;
  }
  *path = call.path;
  return call.result;
}

typedef struct {
  WoxLinuxWindow *window;
  const char *url;
  int32_t result;
} WoxExternalURLCall;

static void open_external_url_main(void *data) {
  WoxExternalURLCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  GError *error = NULL;
  if (!gtk_show_uri_on_window(GTK_WINDOW(call->window->window), call->url, GDK_CURRENT_TIME, &error)) {
    if (error != NULL) {
      g_error_free(error);
    }
    call->result = -1;
  }
}

int32_t wox_linux_window_open_external_url(WoxLinuxWindow *window, const char *url) {
  if (window == NULL || url == NULL) {
    return -1;
  }
  WoxExternalURLCall call = {.window = window, .url = url};
  return run_on_main_sync(open_external_url_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  const char *url;
  const char *html;
  const char *inject_css;
  const char *user_agent;
  const char *cache_key;
  float x;
  float y;
  float width;
  float height;
  float corner_radius;
  bool cache_disabled;
  int32_t result;
} WoxWebViewCall;

static void show_webview_main(void *data) {
  WoxWebViewCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  if (!ensure_webkit()) {
    call->result = -2;
    return;
  }
  bool use_cache = !call->cache_disabled && call->cache_key[0] != '\0';
  char *content_key = g_strconcat(call->html[0] != '\0' ? "html|" : "url|", call->html[0] != '\0' ? call->html : call->url, NULL);
  char *signature = g_strconcat(call->inject_css, "\nuser-agent|", call->user_agent, NULL);
  GtkWidget *web_view = NULL;
  bool should_load = true;
  if (use_cache) {
    const char *cached_signature = g_hash_table_lookup(window->web_view_signatures, call->cache_key);
    if (g_strcmp0(cached_signature, signature) == 0) {
      web_view = g_hash_table_lookup(window->web_view_cache, call->cache_key);
      should_load = g_strcmp0(g_hash_table_lookup(window->web_view_content_keys, call->cache_key), content_key) != 0;
    } else {
      GtkWidget *stale = g_hash_table_lookup(window->web_view_cache, call->cache_key);
      if (stale != NULL && gtk_widget_get_parent(stale) != NULL) {
        gtk_container_remove(GTK_CONTAINER(window->overlay), stale);
      }
      g_hash_table_remove(window->web_view_cache, call->cache_key);
      g_hash_table_remove(window->web_view_signatures, call->cache_key);
      g_hash_table_remove(window->web_view_content_keys, call->cache_key);
      if (stale == window->active_web_view) {
        window->active_web_view = NULL;
        window->active_web_view_transient = false;
        g_clear_pointer(&window->active_web_view_key, g_free);
        g_clear_pointer(&window->active_web_view_signature, g_free);
        g_clear_pointer(&window->active_web_view_content_key, g_free);
      }
    }
    if (web_view == NULL) {
      web_view = create_web_view(window, call->inject_css, call->user_agent);
      if (web_view == NULL) {
        g_free(content_key);
        g_free(signature);
        call->result = -1;
        return;
      }
      g_hash_table_replace(window->web_view_cache, g_strdup(call->cache_key), web_view);
      g_hash_table_replace(window->web_view_signatures, g_strdup(call->cache_key), g_strdup(signature));
    }
    g_hash_table_replace(window->web_view_content_keys, g_strdup(call->cache_key), g_strdup(content_key));
  } else if (window->active_web_view_transient && g_strcmp0(window->active_web_view_signature, signature) == 0 && g_strcmp0(window->active_web_view_content_key, content_key) == 0) {
    web_view = window->active_web_view;
    should_load = false;
  } else {
    web_view = create_web_view(window, call->inject_css, call->user_agent);
    if (web_view == NULL) {
      g_free(content_key);
      g_free(signature);
      call->result = -1;
      return;
    }
  }

  if (web_view != window->active_web_view) {
    clear_active_web_view(window, true);
    window->active_web_view = web_view;
    window->active_web_view_transient = !use_cache;
    window->active_web_view_key = g_strdup(call->cache_key);
    window->active_web_view_signature = g_strdup(signature);
    window->active_web_view_content_key = g_strdup(content_key);
  }
  if (gtk_widget_get_parent(web_view) == NULL) {
    gtk_overlay_add_overlay(GTK_OVERLAY(window->overlay), web_view);
    gtk_overlay_set_overlay_pass_through(GTK_OVERLAY(window->overlay), web_view, FALSE);
  }
  gtk_overlay_reorder_overlay(GTK_OVERLAY(window->overlay), window->overlay_gl_area, -1);
  gtk_widget_set_margin_start(web_view, (int)floorf(call->x));
  gtk_widget_set_margin_top(web_view, (int)floorf(call->y));
  gtk_widget_set_size_request(web_view, (int)ceilf(call->width), (int)ceilf(call->height));
  store_web_view_corner_radius(web_view, fmaxf(0.0f, fminf(call->corner_radius, fminf(call->width, call->height) * 0.5f)));
  apply_web_view_corner_radius(web_view);
  gtk_widget_show(web_view);

  if (should_load) {
    if (call->html[0] != '\0') {
      wox_webkit.load_html(web_view, call->html, NULL);
    } else {
      wox_webkit.load_uri(web_view, call->url);
    }
  }
  g_free(content_key);
  g_free(signature);
}

int32_t wox_linux_window_show_webview(WoxLinuxWindow *window, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height, float corner_radius) {
  if (window == NULL || url == NULL || html == NULL || inject_css == NULL || user_agent == NULL || cache_key == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxWebViewCall call = {
      .window = window,
      .url = url,
      .html = html,
      .inject_css = inject_css,
      .user_agent = user_agent,
      .cache_key = cache_key,
      .x = x,
      .y = y,
      .width = width,
      .height = height,
      .corner_radius = corner_radius,
      .cache_disabled = cache_disabled != 0,
  };
  return run_on_main_sync(show_webview_main, &call) ? call.result : -1;
}

static void hide_webview_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  clear_active_web_view(call->window, true);
}

int32_t wox_linux_window_hide_webview(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(hide_webview_main, &call) ? call.result : -1;
}

static void reset_webview_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  clear_active_web_view(call->window, true);
  g_hash_table_remove_all(call->window->web_view_cache);
  g_hash_table_remove_all(call->window->web_view_signatures);
  g_hash_table_remove_all(call->window->web_view_content_keys);
}

int32_t wox_linux_window_reset_webview(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(reset_webview_main, &call) ? call.result : -1;
}

int32_t wox_linux_window_forward_embedded_surface_pointer(WoxLinuxWindow *window, uint8_t kind, float x, float y) {
  if (window == NULL || window->closed || window->active_web_view == NULL || window->dispatching_pointer_event == NULL) {
    return -1;
  }
  window->pointer_over_web_view = kind != WOX_POINTER_LEAVE;
  if (kind == WOX_POINTER_LEAVE) {
    apply_linux_pointer_cursor(window);
    return 0;
  }
  GdkEvent *event = gdk_event_copy(window->dispatching_pointer_event);
  GdkWindow *target_window = gtk_widget_get_window(window->active_web_view);
  if (event == NULL || target_window == NULL) {
    if (event != NULL) {
      gdk_event_free(event);
    }
    return -1;
  }
  GdkWindow **event_window = NULL;
  switch (event->type) {
  case GDK_MOTION_NOTIFY:
    event->motion.x = x;
    event->motion.y = y;
    event_window = &event->motion.window;
    break;
  case GDK_BUTTON_PRESS:
  case GDK_BUTTON_RELEASE:
  case GDK_2BUTTON_PRESS:
  case GDK_3BUTTON_PRESS:
    event->button.x = x;
    event->button.y = y;
    event_window = &event->button.window;
    if (event->type == GDK_BUTTON_PRESS) {
      gtk_widget_grab_focus(window->active_web_view);
    }
    break;
  case GDK_SCROLL:
    event->scroll.x = x;
    event->scroll.y = y;
    event_window = &event->scroll.window;
    break;
  case GDK_ENTER_NOTIFY:
  case GDK_LEAVE_NOTIFY:
    event->crossing.x = x;
    event->crossing.y = y;
    event_window = &event->crossing.window;
    break;
  default:
    gdk_event_free(event);
    return -1;
  }
  if (*event_window != NULL) {
    g_object_unref(*event_window);
  }
  *event_window = g_object_ref(target_window);
  window->forwarding_embedded_pointer = true;
  gtk_widget_event(window->active_web_view, event);
  window->forwarding_embedded_pointer = false;
  apply_linux_pointer_cursor(window);
  gdk_event_free(event);
  return 0;
}

void wox_linux_free_string(char *value) {
  g_free(value);
}

typedef struct {
  WoxLinuxWindow *window;
  const char *text;
  int32_t result;
} WoxClipboardTextCall;

static void write_clipboard_text_main(void *data) {
  WoxClipboardTextCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  GdkDisplay *display = gtk_widget_get_display(call->window->window);
  GtkClipboard *clipboard = display != NULL ? gtk_clipboard_get_default(display) : NULL;
  if (clipboard == NULL) {
    call->result = -1;
    return;
  }
  gtk_clipboard_set_text(clipboard, call->text, -1);
  gtk_clipboard_store(clipboard);
}

int32_t wox_linux_window_write_clipboard_text(WoxLinuxWindow *window, const char *text) {
  if (window == NULL || text == NULL) {
    return -1;
  }
  WoxClipboardTextCall call = {.window = window, .text = text};
  return run_on_main_sync(write_clipboard_text_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  const uint8_t *pixels;
  int width;
  int height;
  int row_stride;
  int32_t result;
} WoxClipboardImageCall;

static void free_pixbuf_pixels(guchar *pixels, gpointer data) {
  (void)data;
  g_free(pixels);
}

static void write_clipboard_image_main(void *data) {
  WoxClipboardImageCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  size_t byte_count = (size_t)call->row_stride * (size_t)call->height;
  guchar *copy = g_malloc(byte_count);
  if (copy == NULL) {
    call->result = -1;
    return;
  }
  memcpy(copy, call->pixels, byte_count);
  GdkPixbuf *pixbuf = gdk_pixbuf_new_from_data(
      copy,
      GDK_COLORSPACE_RGB,
      TRUE,
      8,
      call->width,
      call->height,
      call->row_stride,
      free_pixbuf_pixels,
      NULL);
  if (pixbuf == NULL) {
    g_free(copy);
    call->result = -1;
    return;
  }
  GdkDisplay *display = gtk_widget_get_display(call->window->window);
  GtkClipboard *clipboard = display != NULL ? gtk_clipboard_get_default(display) : NULL;
  if (clipboard == NULL) {
    g_object_unref(pixbuf);
    call->result = -1;
    return;
  }
  gtk_clipboard_set_image(clipboard, pixbuf);
  gtk_clipboard_store(clipboard);
  g_object_unref(pixbuf);
}

int32_t wox_linux_window_write_clipboard_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t width, int32_t height, int32_t row_stride) {
  if (window == NULL || pixels == NULL || width <= 0 || height <= 0 || row_stride < width * 4) {
    return -1;
  }
  WoxClipboardImageCall call = {
      .window = window,
      .pixels = pixels,
      .width = width,
      .height = height,
      .row_stride = row_stride,
  };
  return run_on_main_sync(write_clipboard_image_main, &call) ? call.result : -1;
}

static gboolean linux_invalidate_idle(gpointer data) {
  WoxLinuxWindow *window = data;
  window->invalidate_idle = 0;
  if (!window->closed && window->visible && window->gl_area != NULL) {
    gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
  }
  return G_SOURCE_REMOVE;
}

// queue_linux_window_render defers gtk_gl_area_queue_render when called from the
// current render. GtkGLArea with auto-render off clears needs_render after the
// signal, so an in-frame Invalidate would otherwise blit the same "3" forever.
static void queue_linux_window_render(WoxLinuxWindow *window) {
  if (window == NULL || window->closed || window->gl_area == NULL) {
    return;
  }
  if (window->presenting || window->rendering) {
    if (window->invalidate_idle == 0) {
      window->invalidate_idle = g_idle_add(linux_invalidate_idle, window);
    }
    return;
  }
  gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
}

static void invalidate_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  queue_linux_window_render(call->window);
}

int32_t wox_linux_window_invalidate(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(invalidate_main, &call) ? call.result : -1;
}

static gboolean linux_animation_tick(GtkWidget *widget, GdkFrameClock *clock, gpointer data) {
  (void)widget;
  (void)clock;
  WoxLinuxWindow *window = data;
  if (window == NULL || window->closed || !window->visible || !window->animation_frame_pending) {
    return G_SOURCE_CONTINUE;
  }
  window->animation_frame_pending = false;
  gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
  return G_SOURCE_CONTINUE;
}

static void stop_linux_animation_frames(WoxLinuxWindow *window) {
  if (window == NULL) {
    return;
  }
  window->animation_frame_pending = false;
  if (window->animation_tick_id != 0 && window->gl_area != NULL) {
    gtk_widget_remove_tick_callback(window->gl_area, window->animation_tick_id);
    window->animation_tick_id = 0;
  }
}

static void request_animation_frame_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed || !call->window->visible || call->window->gl_area == NULL) {
    call->result = -1;
    return;
  }
  call->window->animation_frame_pending = true;
  if (call->window->animation_tick_id == 0) {
    call->window->animation_tick_id = gtk_widget_add_tick_callback(call->window->gl_area, linux_animation_tick, call->window, NULL);
  }
}

static void stop_animation_frames_main(void *data) {
  WoxWindowCall *call = data;
  stop_linux_animation_frames(call->window);
}

int32_t wox_linux_window_request_animation_frame(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(request_animation_frame_main, &call) ? call.result : -1;
}

int32_t wox_linux_window_stop_animation_frames(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(stop_animation_frames_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  bool enabled;
  GdkRectangle cursor_rect;
  int32_t result;
} WoxTextInputCall;

// set_text_input_main keeps GtkIMContext focus and candidate geometry on the GTK thread.
static void set_text_input_main(void *data) {
  WoxTextInputCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  if (window->input_enabled && !call->enabled) {
    window->input_enabled = false;
    window->input_composing = false;
    gtk_im_context_focus_out(window->im_context);
    gtk_im_context_reset(window->im_context);
  } else if (!window->input_enabled && call->enabled) {
    window->input_enabled = true;
    if (window->active) {
      gtk_im_context_focus_in(window->im_context);
    }
  }
  window->input_cursor_rect = call->cursor_rect;
  gtk_im_context_set_cursor_location(window->im_context, &window->input_cursor_rect);
}

int32_t wox_linux_window_set_text_input_state(WoxLinuxWindow *window, int32_t enabled, float x, float y, float width, float height) {
  if (window == NULL) {
    return -1;
  }
  WoxTextInputCall call = {
      .window = window,
      .enabled = enabled != 0,
      .cursor_rect = {
          .x = (int)floorf(x),
          .y = (int)floorf(y),
          .width = (int)ceilf(fmaxf(width, 1.0f)),
          .height = (int)ceilf(fmaxf(height, 1.0f)),
      },
  };
  return run_on_main_sync(set_text_input_main, &call) ? call.result : -1;
}

int32_t wox_linux_window_set_pointer_cursor(WoxLinuxWindow *window, uint8_t cursor) {
  if (window == NULL || window->closed) {
    return -1;
  }
  window->pointer_cursor = cursor;
  return apply_linux_pointer_cursor(window);
}

typedef struct {
  WoxLinuxWindow *window;
  const char *title;
  const char *default_name;
  const char *extension;
  char *path;
  int32_t result;
} WoxSaveFileDialogCall;

static void save_file_main(void *data) {
  WoxSaveFileDialogCall *call = data;
  WoxLinuxWindow *window = call->window;
  if (window->closed) {
    call->result = -1;
    return;
  }
  const char *title = call->title != NULL && call->title[0] != '\0' ? call->title : "Save recording";
  GtkFileChooserNative *dialog = gtk_file_chooser_native_new(title, GTK_WINDOW(window->window), GTK_FILE_CHOOSER_ACTION_SAVE, "_Save", "_Cancel");
  if (dialog == NULL) {
    call->result = -1;
    return;
  }
  gtk_file_chooser_set_do_overwrite_confirmation(GTK_FILE_CHOOSER(dialog), TRUE);
  if (call->default_name != NULL && call->default_name[0] != '\0') {
    gtk_file_chooser_set_current_name(GTK_FILE_CHOOSER(dialog), call->default_name);
  }
  if (call->extension != NULL && call->extension[0] != '\0') {
    GtkFileFilter *filter = gtk_file_filter_new();
    gchar *filter_name = g_strdup_printf("%s file", call->extension);
    gchar *filter_pattern = g_strdup_printf("*.%s", call->extension);
    gtk_file_filter_set_name(filter, filter_name);
    gtk_file_filter_add_pattern(filter, filter_pattern);
    gtk_file_chooser_add_filter(GTK_FILE_CHOOSER(dialog), filter);
    g_free(filter_pattern);
    g_free(filter_name);
  }
  window->native_dialog_active = true;
  gint response = gtk_native_dialog_run(GTK_NATIVE_DIALOG(dialog));
  window->native_dialog_active = false;
  if (response == GTK_RESPONSE_ACCEPT) {
    call->path = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dialog));
    if (call->path == NULL) {
      call->result = -1;
    }
  } else {
    call->result = 1;
  }
  g_object_unref(dialog);
}

int32_t wox_linux_window_save_file(WoxLinuxWindow *window, const char *title, const char *default_name, const char *extension, char **path) {
  if (window == NULL || path == NULL) {
    return -1;
  }
  WoxSaveFileDialogCall call = {.window = window, .title = title, .default_name = default_name, .extension = extension};
  if (!run_on_main_sync(save_file_main, &call)) {
    return -1;
  }
  *path = call.path;
  return call.result;
}

static void set_pointer_passthrough_main(void *data) {
  WoxBoolCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  call->window->pointer_passthrough = call->enabled;
  apply_linux_pointer_passthrough(call->window);
}

int32_t wox_linux_window_set_pointer_passthrough(WoxLinuxWindow *window, int32_t enabled) {
  if (window == NULL) {
    return -1;
  }
  WoxBoolCall call = {.window = window, .enabled = enabled != 0};
  return run_on_main_sync(set_pointer_passthrough_main, &call) ? call.result : -1;
}

typedef struct {
  WoxLinuxWindow *window;
  uint64_t node_id;
  uint32_t action_flags;
} WoxAccessibilityBinding;

static void on_accessibility_clicked(GtkWidget *widget, gpointer data) {
  (void)widget;
  WoxAccessibilityBinding *binding = data;
  const char *action = (binding->action_flags & WOX_ACCESSIBILITY_ACTION_ACTIVATE) != 0 ? "activate" : "toggle";
  woxGoLinuxAccessibilityAction(binding->window->context, binding->node_id, action, "");
}

static void on_accessibility_changed(GtkEditable *editable, gpointer data) {
  WoxAccessibilityBinding *binding = data;
  if ((binding->action_flags & WOX_ACCESSIBILITY_ACTION_SET_VALUE) != 0) {
    woxGoLinuxAccessibilityAction(binding->window->context, binding->node_id, "set_value", gtk_entry_get_text(GTK_ENTRY(editable)));
  }
}

static gboolean on_accessibility_focus(GtkWidget *widget, GdkEventFocus *event, gpointer data) {
  (void)widget;
  WoxAccessibilityBinding *binding = data;
  if (event->in && (binding->action_flags & WOX_ACCESSIBILITY_ACTION_FOCUS) != 0) {
    woxGoLinuxAccessibilityAction(binding->window->context, binding->node_id, "focus", "");
  }
  return FALSE;
}

static AtkRole accessibility_atk_role(const char *role) {
  if (g_strcmp0(role, "window") == 0) return ATK_ROLE_FRAME;
  if (g_strcmp0(role, "dialog") == 0) return ATK_ROLE_DIALOG;
  if (g_strcmp0(role, "text") == 0) return ATK_ROLE_STATIC;
  if (g_strcmp0(role, "heading") == 0) return ATK_ROLE_HEADING;
  if (g_strcmp0(role, "button") == 0) return ATK_ROLE_PUSH_BUTTON;
  if (g_strcmp0(role, "text_field") == 0) return ATK_ROLE_ENTRY;
  if (g_strcmp0(role, "checkbox") == 0) return ATK_ROLE_CHECK_BOX;
  if (g_strcmp0(role, "radio_button") == 0) return ATK_ROLE_RADIO_BUTTON;
  if (g_strcmp0(role, "list") == 0) return ATK_ROLE_LIST;
  if (g_strcmp0(role, "list_item") == 0) return ATK_ROLE_LIST_ITEM;
  if (g_strcmp0(role, "image") == 0) return ATK_ROLE_IMAGE;
  if (g_strcmp0(role, "progress_bar") == 0) return ATK_ROLE_PROGRESS_BAR;
  if (g_strcmp0(role, "link") == 0) return ATK_ROLE_LINK;
  if (g_strcmp0(role, "menu") == 0) return ATK_ROLE_MENU;
  if (g_strcmp0(role, "menu_item") == 0) return ATK_ROLE_MENU_ITEM;
  if (g_strcmp0(role, "web_view") == 0) return ATK_ROLE_DOCUMENT_WEB;
  return ATK_ROLE_PANEL;
}

static GtkWidget *accessibility_widget(const char *role, const char *value, uint32_t state_flags, uint32_t action_flags) {
  GtkWidget *widget = NULL;
  if (g_strcmp0(role, "text_field") == 0) {
    widget = gtk_entry_new();
    gtk_entry_set_text(GTK_ENTRY(widget), value != NULL ? value : "");
    gtk_entry_set_visibility(GTK_ENTRY(widget), (state_flags & WOX_ACCESSIBILITY_STATE_PROTECTED) == 0);
    gtk_editable_set_editable(GTK_EDITABLE(widget), (state_flags & WOX_ACCESSIBILITY_STATE_READ_ONLY) == 0);
  } else if (g_strcmp0(role, "checkbox") == 0) {
    widget = gtk_check_button_new();
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(widget), (state_flags & WOX_ACCESSIBILITY_STATE_CHECKED) != 0);
  } else if (g_strcmp0(role, "radio_button") == 0) {
    widget = gtk_radio_button_new(NULL);
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(widget), (state_flags & WOX_ACCESSIBILITY_STATE_CHECKED) != 0);
  } else if ((action_flags & (WOX_ACCESSIBILITY_ACTION_ACTIVATE | WOX_ACCESSIBILITY_ACTION_TOGGLE)) != 0) {
    widget = gtk_button_new();
  } else if (g_strcmp0(role, "progress_bar") == 0) {
    widget = gtk_progress_bar_new();
  } else {
    widget = gtk_event_box_new();
  }
  return widget;
}

int32_t wox_linux_accessibility_begin(WoxLinuxWindow *window, uint64_t generation) {
  (void)generation;
  if (window == NULL || window->closed || window->accessibility_layer == NULL) {
    return -1;
  }
  window->updating_accessibility = true;
  GList *children = gtk_container_get_children(GTK_CONTAINER(window->accessibility_layer));
  for (GList *item = children; item != NULL; item = item->next) {
    gtk_widget_destroy(GTK_WIDGET(item->data));
  }
  g_list_free(children);
  return 0;
}

int32_t wox_linux_accessibility_add_node(WoxLinuxWindow *window, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region) {
  (void)parent_id;
  (void)children;
  (void)child_count;
  (void)live_region;
  if (window == NULL || window->closed || window->accessibility_layer == NULL || id == 0) {
    return -1;
  }
  GtkWidget *widget = accessibility_widget(role, value, state_flags, action_flags);
  if (widget == NULL) {
    return -1;
  }
  gtk_widget_set_name(widget, automation_id != NULL ? automation_id : "wox-accessibility-node");
  gtk_widget_set_can_focus(widget, (state_flags & WOX_ACCESSIBILITY_STATE_FOCUSABLE) != 0);
  gtk_widget_set_sensitive(widget, (state_flags & WOX_ACCESSIBILITY_STATE_ENABLED) != 0);
  gtk_widget_set_size_request(widget, MAX(1, (int)ceilf(width)), MAX(1, (int)ceilf(height)));
  gtk_fixed_put(GTK_FIXED(window->accessibility_layer), widget, (int)floorf(x), (int)floorf(y));
  AtkObject *accessible = gtk_widget_get_accessible(widget);
  atk_object_set_role(accessible, accessibility_atk_role(role));
  atk_object_set_name(accessible, label != NULL && label[0] != '\0' ? label : value);
  atk_object_set_description(accessible, description != NULL ? description : "");
  atk_object_notify_state_change(accessible, ATK_STATE_FOCUSED, (state_flags & WOX_ACCESSIBILITY_STATE_FOCUSED) != 0);
  atk_object_notify_state_change(accessible, ATK_STATE_SELECTED, (state_flags & WOX_ACCESSIBILITY_STATE_SELECTED) != 0);
  atk_object_notify_state_change(accessible, ATK_STATE_CHECKED, (state_flags & WOX_ACCESSIBILITY_STATE_CHECKED) != 0);
  atk_object_notify_state_change(accessible, ATK_STATE_EXPANDED, (state_flags & WOX_ACCESSIBILITY_STATE_EXPANDED) != 0);

  WoxAccessibilityBinding *binding = g_new0(WoxAccessibilityBinding, 1);
  binding->window = window;
  binding->node_id = id;
  binding->action_flags = action_flags;
  g_object_set_data_full(G_OBJECT(widget), "wox-accessibility-binding", binding, g_free);
  if (GTK_IS_ENTRY(widget)) {
    g_signal_connect(widget, "changed", G_CALLBACK(on_accessibility_changed), binding);
  }
  if (GTK_IS_BUTTON(widget)) {
    g_signal_connect(widget, "clicked", G_CALLBACK(on_accessibility_clicked), binding);
  }
  g_signal_connect(widget, "focus-in-event", G_CALLBACK(on_accessibility_focus), binding);
  gtk_widget_set_no_show_all(widget, (state_flags & WOX_ACCESSIBILITY_STATE_HIDDEN) != 0);
  if ((state_flags & WOX_ACCESSIBILITY_STATE_HIDDEN) == 0) {
    gtk_widget_show(widget);
    apply_linux_pointer_cursor(window);
  }
  // Native accessibility widgets mirror the GPU tree but must not consume visual pointer input.
  // Forward press/motion/scroll before GTK button defaults run. Leave/enter stay on the
  // top-level window: rebuilding these mirrors on each query keystroke would otherwise
  // report a window leave and let GtkEntry's I-beam fight the query drag-area cursor.
  gtk_widget_add_events(widget, GDK_POINTER_MOTION_MASK | GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK | GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
  g_signal_connect(widget, "motion-notify-event", G_CALLBACK(on_pointer_motion), window);
  g_signal_connect(widget, "button-press-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(widget, "button-release-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(widget, "scroll-event", G_CALLBACK(on_pointer_scroll), window);
  return 0;
}

int32_t wox_linux_accessibility_end(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  apply_linux_pointer_cursor(window);
  window->updating_accessibility = false;
  if (window->closed || window->accessibility_layer == NULL) {
    return -1;
  }
  g_signal_emit_by_name(gtk_widget_get_accessible(window->accessibility_layer), "visible-data-changed");
  return 0;
}

typedef struct {
  WoxLinuxWindow *window;
  const char *text;
	const char *font_family;
  float font_size;
  uint8_t font_weight;
  uint8_t italic;
  float *width;
  float *height;
  float *baseline;
  int32_t result;
} WoxTextMeasureCall;

// measure_text_main returns logical Pango metrics without allocating a render texture.
static void measure_text_main(void *data) {
  WoxTextMeasureCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  *call->width = 0.0f;
  *call->height = 0.0f;
  *call->baseline = 0.0f;
  if (call->text[0] == '\0') {
    return;
  }
  PangoContext *context = pango_font_map_create_context(pango_cairo_font_map_get_default());
  PangoLayout *layout = pango_layout_new(context);
  PangoFontDescription *font = pango_font_description_new();
	pango_font_description_set_family(font, call->font_family[0] == '\0' ? "Sans" : call->font_family);
  pango_font_description_set_absolute_size(font, call->font_size * PANGO_SCALE);
  pango_font_description_set_weight(font, call->font_weight == 1 ? PANGO_WEIGHT_SEMIBOLD : PANGO_WEIGHT_NORMAL);
  pango_font_description_set_style(font, call->italic ? PANGO_STYLE_ITALIC : PANGO_STYLE_NORMAL);
  pango_layout_set_font_description(layout, font);
  pango_layout_set_text(layout, call->text, -1);
  pango_layout_set_single_paragraph_mode(layout, TRUE);
  PangoRectangle logical;
  pango_layout_get_extents(layout, NULL, &logical);
  *call->width = (float)logical.width / PANGO_SCALE;
  *call->height = (float)logical.height / PANGO_SCALE;
  *call->baseline = (float)pango_layout_get_baseline(layout) / PANGO_SCALE;
  pango_font_description_free(font);
  g_object_unref(layout);
  g_object_unref(context);
}

int32_t wox_linux_window_measure_text(WoxLinuxWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, uint8_t italic, float *width, float *height, float *baseline) {
  if (window == NULL || text == NULL || font_family == NULL || width == NULL || height == NULL || baseline == NULL || font_size <= 0.0f || font_weight > 1 || italic > 1 || !g_utf8_validate(text, -1, NULL) || !g_utf8_validate(font_family, -1, NULL)) {
    return -1;
  }
  WoxTextMeasureCall call = {
      .window = window,
      .text = text,
			.font_family = font_family,
      .font_size = font_size,
      .font_weight = font_weight,
      .italic = italic,
      .width = width,
      .height = height,
      .baseline = baseline,
  };
  return run_on_main_sync(measure_text_main, &call) ? call.result : -1;
}

static void close_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window != NULL && !call->window->closed && call->window->window != NULL) {
    gtk_widget_destroy(call->window->window);
  }
}

int32_t wox_linux_window_close(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  if (window->closed) {
    return 0;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(close_main, &call) ? call.result : -1;
}

static int32_t begin_linux_renderer_frame(WoxLinuxWindow *window, WoxLinuxRenderer *renderer, GtkWidget *gl_area, bool force_opaque, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->closed || renderer == NULL || !linux_gl_area_can_make_current(gl_area) || !renderer->ready || renderer->frame_open || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    if (window != NULL) {
      trace_linux_render("event=frame_begin_failed frameId=%llu surface=%s reason=invalid_state closed=%d ready=%d frameOpen=%d logical=%.1fx%.1f scale=%.2f", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), window->closed, renderer != NULL && renderer->ready, renderer != NULL && renderer->frame_open, logical_width, logical_height, scale);
    }
    return -1;
  }
  gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
  GError *context_error = gtk_gl_area_get_error(GTK_GL_AREA(gl_area));
  if (context_error != NULL) {
    trace_linux_render("event=frame_begin_failed frameId=%llu surface=%s reason=context error=%s", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), context_error->message);
    return -1;
  }
  gtk_gl_area_attach_buffers(GTK_GL_AREA(gl_area));
  if (renderer->scale > 0.0f && fabsf(renderer->scale - scale) > 0.001f) {
    clear_linux_resource_caches(renderer, true);
    renderer->text_use_serial = 0;
  }
  renderer->logical_width = logical_width;
  renderer->logical_height = logical_height;
  renderer->scale = scale;
  int pixel_width = (int)ceilf(logical_width * scale);
  int pixel_height = (int)ceilf(logical_height * scale);
  glGetIntegerv(GL_DRAW_FRAMEBUFFER_BINDING, &renderer->default_framebuffer);
  bool recreated = false;
  if (!ensure_frame_storage(renderer, pixel_width, pixel_height, &recreated)) {
    if (wox_linux_render_trace_enabled) {
      GLenum framebuffer_status = glCheckFramebufferStatus(GL_FRAMEBUFFER);
      GLenum gl_error = glGetError();
      trace_linux_render("event=frame_begin_failed frameId=%llu surface=%s reason=framebuffer_incomplete size=%dx%d framebufferStatus=%#x glError=%#x", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), pixel_width, pixel_height, (unsigned int)framebuffer_status, (unsigned int)gl_error);
    }
    glBindFramebuffer(GL_FRAMEBUFFER, renderer->default_framebuffer);
    return -1;
  }
  glBindFramebuffer(GL_FRAMEBUFFER, renderer->frame_framebuffer);
  float clear[4];
  premultiplied_color(red, green, blue, force_opaque ? 255 : alpha, clear);
  glViewport(0, 0, pixel_width, pixel_height);
  glDisable(GL_DEPTH_TEST);
  bool generation_changed = renderer->last_presented_generation == 0 || renderer->last_presented_generation != renderer->context_generation;
  renderer->damage_active = !recreated && !generation_changed && damage_width > 0.0f && damage_height > 0.0f;
  if (renderer->damage_active) {
    int left = (int)floorf(fmaxf(0.0f, damage_x) * scale);
    int right = (int)ceilf(fminf(logical_width, damage_x + damage_width) * scale);
    int top = (int)floorf(fmaxf(0.0f, damage_y) * scale);
    int bottom = (int)ceilf(fminf(logical_height, damage_y + damage_height) * scale);
    renderer->damage_left = left;
    renderer->damage_bottom = pixel_height - bottom;
    renderer->damage_width = (int)fmaxf(0.0f, (float)(right - left));
    renderer->damage_height = (int)fmaxf(0.0f, (float)(bottom - top));
    glEnable(GL_SCISSOR_TEST);
    glScissor(renderer->damage_left, renderer->damage_bottom, renderer->damage_width, renderer->damage_height);
  } else {
    glDisable(GL_SCISSOR_TEST);
  }
  glEnable(GL_BLEND);
  glBlendEquation(GL_FUNC_ADD);
  glBlendFunc(GL_ONE, GL_ONE_MINUS_SRC_ALPHA);
  glClearColor(clear[0], clear[1], clear[2], clear[3]);
  glClear(GL_COLOR_BUFFER_BIT);
  glBindVertexArray(renderer->vertex_array);
  renderer->frame_open = true;
  renderer->clip_active = false;
  if (wox_linux_render_trace_enabled) {
    GLenum framebuffer_status = glCheckFramebufferStatus(GL_FRAMEBUFFER);
    GLenum gl_error = glGetError();
    trace_linux_render(
        "event=frame_begin frameId=%llu surface=%s logical=%.1fx%.1f pixel=%dx%d scale=%.2f damage=%.1f,%.1f %.1fx%.1f damageActive=%d scissor=%d,%d %dx%d recreated=%d generation=%llu lastPresentedGeneration=%llu frameTexture=%u frameFramebuffer=%u defaultFramebuffer=%d framebufferStatus=%#x glError=%#x",
        (unsigned long long)window->trace_frame_id,
        linux_renderer_name(window, renderer),
        logical_width,
        logical_height,
        pixel_width,
        pixel_height,
        scale,
        damage_x,
        damage_y,
        damage_width,
        damage_height,
        renderer->damage_active,
        renderer->damage_left,
        renderer->damage_bottom,
        renderer->damage_width,
        renderer->damage_height,
        recreated,
        (unsigned long long)renderer->context_generation,
        (unsigned long long)renderer->last_presented_generation,
        renderer->frame_texture,
        renderer->frame_framebuffer,
        renderer->default_framebuffer,
        (unsigned int)framebuffer_status,
        (unsigned int)gl_error);
  }
  return 0;
}

int32_t wox_linux_window_begin_frame(WoxLinuxWindow *window, uint64_t frame_id, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL) {
    return -1;
  }
  window->trace_frame_id = frame_id;
  window->embedded_surface_overlay_active = false;
  window->active_renderer = &window->renderer;
  window->active_gl_area = window->gl_area;
  memset(&window->frame_resource_stats, 0, sizeof(window->frame_resource_stats));
  // Screenshot and recording surfaces must keep the requested clear alpha. Forcing
  // opaque here turns Clear(Color{}) into a black desktop-covering backdrop.
  return begin_linux_renderer_frame(window, window->active_renderer, window->active_gl_area, !window->per_pixel_alpha, logical_width, logical_height, scale, damage_x, damage_y, damage_width, damage_height, red, green, blue, alpha);
}

int32_t wox_linux_window_fill_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->rect_program);
  glUniform2f(renderer->rect_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->rect_bounds, x, y, width, height);
  glUniform4fv(renderer->rect_color, 1, color);
  glUniform1f(renderer->rect_radius, radius);
  glUniform1f(renderer->rect_stroke_width, 0.0f);
  glUniform1i(renderer->rect_polygon_count, 0);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  return 0;
}

int32_t wox_linux_window_fill_convex_polygon(WoxLinuxWindow *window, const float *points, int32_t point_count, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (points == NULL || point_count < 3 || point_count > 16) {
    return -1;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  float min_x = points[0];
  float max_x = points[0];
  float min_y = points[1];
  float max_y = points[1];
  for (int32_t index = 1; index < point_count; index++) {
    float point_x = points[index * 2];
    float point_y = points[index * 2 + 1];
    min_x = fminf(min_x, point_x);
    max_x = fmaxf(max_x, point_x);
    min_y = fminf(min_y, point_y);
    max_y = fmaxf(max_y, point_y);
  }
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->rect_program);
  glUniform2f(renderer->rect_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->rect_bounds, min_x, min_y, max_x - min_x, max_y - min_y);
  glUniform4fv(renderer->rect_color, 1, color);
  glUniform1f(renderer->rect_radius, 0.0f);
  glUniform1f(renderer->rect_stroke_width, 0.0f);
  glUniform2fv(renderer->rect_polygon, point_count, points);
  glUniform1i(renderer->rect_polygon_count, point_count);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  return 0;
}

int32_t wox_linux_window_stroke_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f || stroke_width <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->rect_program);
  glUniform2f(renderer->rect_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->rect_bounds, x, y, width, height);
  glUniform4fv(renderer->rect_color, 1, color);
  glUniform1f(renderer->rect_radius, radius);
  glUniform1f(renderer->rect_stroke_width, stroke_width);
  glUniform1i(renderer->rect_polygon_count, 0);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  return 0;
}

int32_t wox_linux_window_draw_text(WoxLinuxWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t italic, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || text == NULL || font_family == NULL || italic > 1) {
    return -1;
  }
  if (text[0] == '\0' || width <= 0.0f || height <= 0.0f || font_size <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  int pixel_width = (int)ceilf(width * renderer->scale);
  int pixel_height = (int)ceilf(height * renderer->scale);
  if (pixel_width <= 0 || pixel_height <= 0 || pixel_width > 16384 || pixel_height > 16384) {
    return -1;
  }
  const char *family = font_family[0] == '\0' ? "Sans" : font_family;
  size_t text_len = strlen(text);
  bool cacheable = text_len <= WOX_LINUX_TEXT_CACHE_MAX_CHARS;
  uint64_t hash = 0;
  WoxLinuxTextCacheEntry *cached = NULL;
  if (cacheable) {
    hash = linux_text_cache_hash(text, family, font_size, font_weight, italic, renderer->scale, pixel_width, pixel_height);
    cached = find_cached_gl_text(renderer, hash, text, family, font_size, font_weight, italic, renderer->scale, pixel_width, pixel_height);
  }
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  if (cached != NULL) {
    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, cached->texture);
    draw_bound_texture(renderer, x, y, width, height, 0.0f, 0.0f, color);
    window->frame_resource_stats.cache_hits++;
    return 0;
  }

  cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, pixel_width, pixel_height);
  if (cairo_surface_status(surface) != CAIRO_STATUS_SUCCESS) {
    cairo_surface_destroy(surface);
    return -1;
  }
  cairo_t *cairo = cairo_create(surface);
  cairo_set_source_rgba(cairo, 1.0, 1.0, 1.0, 1.0);
  PangoLayout *layout = pango_cairo_create_layout(cairo);
  PangoFontDescription *font = pango_font_description_new();
  pango_font_description_set_family(font, family);
  pango_font_description_set_absolute_size(font, font_size * renderer->scale * PANGO_SCALE);
  pango_font_description_set_weight(font, font_weight == 1 ? PANGO_WEIGHT_SEMIBOLD : PANGO_WEIGHT_NORMAL);
  pango_font_description_set_style(font, italic ? PANGO_STYLE_ITALIC : PANGO_STYLE_NORMAL);
  pango_layout_set_font_description(layout, font);
  pango_layout_set_text(layout, text, -1);
  pango_layout_set_width(layout, pixel_width * PANGO_SCALE);
  pango_layout_set_single_paragraph_mode(layout, TRUE);
  // Do not set a positive layout height. CJK fonts can report a line box taller
  // than the destination slot, and Pango then omits the only line. The cairo
  // image already clips overflow to pixel_height.
  pango_cairo_show_layout(cairo, layout);
  cairo_surface_flush(surface);

  GLuint texture = upload_gl_texture(pixel_width, pixel_height, GL_BGRA, cairo_image_surface_get_data(surface), 0);
  if (texture == 0) {
    pango_font_description_free(font);
    g_object_unref(layout);
    cairo_destroy(cairo);
    cairo_surface_destroy(surface);
    return -1;
  }
  window->frame_resource_stats.text_rasterizations++;
  bool cached_texture = false;
  if (cacheable) {
    uint64_t byte_size = (uint64_t)pixel_width * (uint64_t)pixel_height * 4ULL;
    cached_texture = cache_gl_text(renderer, hash, text, family, font_size, font_weight, italic, renderer->scale, pixel_width, pixel_height, byte_size, texture, true, &window->frame_resource_stats.cache_evictions);
  }
  draw_bound_texture(renderer, x, y, width, height, 0.0f, 0.0f, color);
  if (!cached_texture) {
    glDeleteTextures(1, &texture);
  }

  pango_font_description_free(font);
  g_object_unref(layout);
  cairo_destroy(cairo);
  cairo_surface_destroy(surface);
  return 0;
}

int32_t wox_linux_window_draw_image(WoxLinuxWindow *window, uint64_t image_id, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height, float rotation_radians, float corner_radius) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  float color[4];
  premultiplied_color(255, 255, 255, 255, color);
  uint64_t image_bytes = (uint64_t)row_stride * (uint64_t)image_height;
  bool large_image = image_bytes > wox_linux_image_cache_max_entry_bytes;
  // Lookup is deliberately not gated on cache_large_images: that flag governs admission only, so a
  // slot already holding this image keeps serving hits while an alternating image churns candidacy.
  if (large_image && renderer->cached_large_image_id == image_id &&
      renderer->cached_large_image != 0 && renderer->cached_large_image_generation == renderer->context_generation) {
    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, renderer->cached_large_image);
    draw_bound_texture(renderer, x, y, width, height, rotation_radians, corner_radius, color);
    window->frame_resource_stats.cache_hits++;
    return 0;
  }
  WoxLinuxImageCacheEntry *cached = !large_image && image_id != 0 ? find_cached_gl_image(renderer, image_id) : NULL;
  if (cached != NULL) {
    glActiveTexture(GL_TEXTURE0);
    glBindTexture(GL_TEXTURE_2D, cached->texture);
    draw_bound_texture(renderer, x, y, width, height, rotation_radians, corner_radius, color);
    window->frame_resource_stats.cache_hits++;
    return 0;
  }
  int64_t create_start = linux_monotonic_nanos();
  GLuint texture = upload_gl_texture(image_width, image_height, GL_RGBA, pixels, row_stride / 4);
  if (texture == 0) {
    return -1;
  }
  window->frame_resource_stats.image_creates++;
  window->frame_resource_stats.image_uploads++;
  bool cached_texture = false;
  if (large_image) {
    note_large_image_create(window, image_id, linux_monotonic_nanos() - create_start);
    if (window->cache_large_images && image_id != 0 && image_bytes <= wox_linux_large_image_max_bytes) {
      clear_cached_large_image(renderer, true);
      renderer->cached_large_image = texture;
      renderer->cached_large_image_id = image_id;
      renderer->cached_large_image_bytes = image_bytes;
      renderer->cached_large_image_generation = renderer->context_generation;
      cached_texture = true;
    }
  } else {
    cached_texture = image_id != 0 && cache_gl_image(renderer, image_id, image_bytes, texture, true, &window->frame_resource_stats.cache_evictions);
  }
  draw_bound_texture(renderer, x, y, width, height, rotation_radians, corner_radius, color);
  if (!cached_texture) {
    glDeleteTextures(1, &texture);
  }
  return 0;
}

int32_t wox_linux_window_set_clip_rect(WoxLinuxWindow *window, float x, float y, float width, float height) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  float left = fmaxf(0.0f, fminf(renderer->logical_width, x));
  float top = fmaxf(0.0f, fminf(renderer->logical_height, y));
  float right = fmaxf(left, fminf(renderer->logical_width, x + fmaxf(0.0f, width)));
  float bottom = fmaxf(top, fminf(renderer->logical_height, y + fmaxf(0.0f, height)));
  int pixel_left = (int)floorf(left * renderer->scale);
  int pixel_right = (int)ceilf(right * renderer->scale);
  int pixel_top = (int)floorf(top * renderer->scale);
  int pixel_bottom = (int)ceilf(bottom * renderer->scale);
  int framebuffer_height = (int)ceilf(renderer->logical_height * renderer->scale);
  int scissor_left = pixel_left;
  int scissor_bottom = framebuffer_height - pixel_bottom;
  int scissor_right = pixel_right;
  int scissor_top = framebuffer_height - pixel_top;
  if (renderer->damage_active) {
    scissor_left = fmax(scissor_left, renderer->damage_left);
    scissor_bottom = fmax(scissor_bottom, renderer->damage_bottom);
    scissor_right = fmin(scissor_right, renderer->damage_left + renderer->damage_width);
    scissor_top = fmin(scissor_top, renderer->damage_bottom + renderer->damage_height);
  }
  glEnable(GL_SCISSOR_TEST);
  glScissor(scissor_left, scissor_bottom, (int)fmaxf(0.0f, (float)(scissor_right - scissor_left)), (int)fmaxf(0.0f, (float)(scissor_top - scissor_bottom)));
  renderer->clip_active = true;
  renderer->clip_x = left;
  renderer->clip_y = top;
  renderer->clip_width = right - left;
  renderer->clip_height = bottom - top;
  return 0;
}

int32_t wox_linux_window_clear_clip(WoxLinuxWindow *window) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open) {
    return -1;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  renderer->clip_active = false;
  if (renderer->damage_active) {
    glEnable(GL_SCISSOR_TEST);
    glScissor(renderer->damage_left, renderer->damage_bottom, renderer->damage_width, renderer->damage_height);
  } else {
    glDisable(GL_SCISSOR_TEST);
  }
  return 0;
}

// wox_linux_window_trace_encode separates draw-command failures from the following framebuffer blit.
void wox_linux_window_trace_encode(WoxLinuxWindow *window) {
  if (!wox_linux_render_trace_enabled || window == NULL || window->active_renderer == NULL) {
    return;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  GLenum framebuffer_status = glCheckFramebufferStatus(GL_FRAMEBUFFER);
  GLenum gl_error = glGetError();
  trace_linux_render("event=frame_encode frameId=%llu surface=%s framebuffer=%u framebufferStatus=%#x glError=%#x", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), renderer->frame_framebuffer, (unsigned int)framebuffer_status, (unsigned int)gl_error);
}

static int32_t finish_linux_renderer_frame(WoxLinuxWindow *window, WoxLinuxRenderer *renderer) {
  if (renderer == NULL || !renderer->frame_open) {
    if (window != NULL) {
      trace_linux_render("event=frame_finish_failed frameId=%llu surface=%s reason=frame_not_open", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer));
    }
    return -1;
  }
  glBindVertexArray(0);
  glUseProgram(0);
  glDisable(GL_SCISSOR_TEST);
  glBindFramebuffer(GL_READ_FRAMEBUFFER, renderer->frame_framebuffer);
  GLenum read_status = GL_FRAMEBUFFER_COMPLETE;
  if (wox_linux_render_trace_enabled) {
    read_status = glCheckFramebufferStatus(GL_READ_FRAMEBUFFER);
  }
  glBindFramebuffer(GL_DRAW_FRAMEBUFFER, renderer->default_framebuffer);
  GLenum draw_status = GL_FRAMEBUFFER_COMPLETE;
  if (wox_linux_render_trace_enabled) {
    draw_status = glCheckFramebufferStatus(GL_DRAW_FRAMEBUFFER);
  }
  glBlitFramebuffer(0, 0, renderer->frame_width, renderer->frame_height, 0, 0, renderer->frame_width, renderer->frame_height, GL_COLOR_BUFFER_BIT, GL_NEAREST);
  glBindFramebuffer(GL_FRAMEBUFFER, renderer->default_framebuffer);
  glFlush();
  renderer->frame_open = false;
  renderer->last_presented_generation = renderer->context_generation;
  if (wox_linux_render_trace_enabled) {
    GLenum gl_error = glGetError();
    trace_linux_render("event=frame_finish frameId=%llu surface=%s size=%dx%d frameFramebuffer=%u defaultFramebuffer=%d readStatus=%#x drawStatus=%#x generation=%llu glError=%#x", (unsigned long long)window->trace_frame_id, linux_renderer_name(window, renderer), renderer->frame_width, renderer->frame_height, renderer->frame_framebuffer, renderer->default_framebuffer, (unsigned int)read_status, (unsigned int)draw_status, (unsigned long long)renderer->context_generation, (unsigned int)gl_error);
  }
  return 0;
}

int32_t wox_linux_window_begin_embedded_surface_overlay(WoxLinuxWindow *window) {
  if (window == NULL || window->active_renderer != &window->renderer || !window->renderer.frame_open) {
    return -1;
  }
  WoxLinuxRenderer *background = &window->renderer;
  bool restore_clip = background->clip_active;
  float clip_x = background->clip_x;
  float clip_y = background->clip_y;
  float clip_width = background->clip_width;
  float clip_height = background->clip_height;
  int32_t result = finish_linux_renderer_frame(window, background);
  if (result != 0) {
    return result;
  }
  window->active_renderer = &window->overlay_renderer;
  window->active_gl_area = window->overlay_gl_area;
  result = begin_linux_renderer_frame(window, window->active_renderer, window->active_gl_area, false, background->logical_width, background->logical_height, background->scale, 0.0f, 0.0f, 0.0f, 0.0f, 0, 0, 0, 0);
  if (result != 0) {
    return result;
  }
  window->embedded_surface_overlay_active = true;
  if (restore_clip) {
    return wox_linux_window_set_clip_rect(window, clip_x, clip_y, clip_width, clip_height);
  }
  return 0;
}

int32_t wox_linux_window_end_frame(WoxLinuxWindow *window) {
  if (window == NULL || window->active_renderer == NULL) {
    return -1;
  }
  int32_t result = finish_linux_renderer_frame(window, window->active_renderer);
  if (result == 0 && !window->embedded_surface_overlay_active) {
    WoxLinuxRenderer *background = &window->renderer;
    window->active_renderer = &window->overlay_renderer;
    window->active_gl_area = window->overlay_gl_area;
    result = begin_linux_renderer_frame(window, window->active_renderer, window->active_gl_area, false, background->logical_width, background->logical_height, background->scale, 0.0f, 0.0f, 0.0f, 0.0f, 0, 0, 0, 0);
    if (result == 0) {
      result = finish_linux_renderer_frame(window, window->active_renderer);
    }
  }
  if (result == 0 && window->overlay_gl_area != NULL) {
    gtk_gl_area_queue_render(GTK_GL_AREA(window->overlay_gl_area));
  }
  if (linux_gl_area_can_make_current(window->gl_area)) {
    gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
  }
  window->active_renderer = NULL;
  window->active_gl_area = NULL;
  return result;
}

int32_t wox_linux_window_take_frame_resource_stats(WoxLinuxWindow *window, WoxRendererResourceStats *out) {
  if (window == NULL || out == NULL) {
    return -1;
  }
  *out = window->frame_resource_stats;
  out->resident_bytes = (int64_t)(linux_cache_resident_bytes(&window->renderer) + linux_cache_resident_bytes(&window->overlay_renderer));
  return 0;
}

// wox_linux_test_resource_cache_generation exercises LRU bookkeeping and context generations without GL.
int32_t wox_linux_test_resource_cache_generation(void) {
  WoxLinuxRenderer renderer;
  memset(&renderer, 0, sizeof(renderer));
  renderer.context_generation = 1;
  renderer.texts = calloc((size_t)WOX_LINUX_TEXT_CACHE_MAX, sizeof(WoxLinuxTextCacheEntry));
  if (renderer.texts == NULL) {
    return -1;
  }
  if (!cache_gl_image(&renderer, 11, 64, 1, false, NULL) || find_cached_gl_image(&renderer, 11) == NULL) {
    free(renderer.texts);
    return -1;
  }
  if (!cache_gl_text(&renderer, 7, "Hello", "Sans", 13.0f, 0, 0, 2.0f, 40, 16, 128, 2, false, NULL) ||
      find_cached_gl_text(&renderer, 7, "Hello", "Sans", 13.0f, 0, 0, 2.0f, 40, 16) == NULL) {
    free(renderer.texts);
    return -1;
  }
  renderer.context_generation++;
  if (find_cached_gl_image(&renderer, 11) != NULL || find_cached_gl_text(&renderer, 7, "Hello", "Sans", 13.0f, 0, 0, 2.0f, 40, 16) != NULL) {
    free(renderer.texts);
    return -1;
  }
  if (cache_gl_image(&renderer, 12, wox_linux_image_cache_max_entry_bytes + 1, 3, false, NULL)) {
    free(renderer.texts);
    return -1;
  }
  renderer.cached_large_image = 3;
  renderer.cached_large_image_id = 12;
  renderer.cached_large_image_bytes = 2ULL * 1024ULL * 1024ULL;
  renderer.cached_large_image_generation = 1;
  clear_linux_resource_caches(&renderer, false);
  WoxLinuxWindow window;
  memset(&window, 0, sizeof(window));
  note_large_image_create(&window, 99, 3000000LL);
  note_large_image_create(&window, 99, 3000000LL);
  note_large_image_create(&window, 99, 3000000LL);
  bool large_slot_admitted = window.cache_large_images;
  // A different oversized image must re-earn the slot instead of inheriting the verdict above.
  note_large_image_create(&window, 100, 3000000LL);
  bool large_slot_requires_new_candidate = !window.cache_large_images;
  int32_t result = renderer.image_count == 0 && renderer.text_count == 0 && renderer.image_bytes == 0 &&
                           renderer.text_bytes == 0 && renderer.cached_large_image == 0 && renderer.cached_large_image_bytes == 0 &&
                           renderer.context_generation == 2 && large_slot_admitted && large_slot_requires_new_candidate
                       ? 0
                       : -1;
  free(renderer.texts);
  return result;
}

int32_t wox_linux_test_resize_hit(float x, float y, int32_t width, int32_t height, int32_t grip) {
  return (int32_t)linux_resize_hit_test((double)x, (double)y, (int)width, (int)height, (int)grip);
}

int32_t wox_linux_test_layer_shell_stack_layer(int32_t topmost, int32_t screenshot) {
  return (int32_t)layer_shell_stack_layer(topmost != 0, screenshot != 0);
}
