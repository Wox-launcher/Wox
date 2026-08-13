//go:build linux

#include "native_linux.h"

#include <gtk/gtk.h>
#include <epoxy/gl.h>
#include <pango/pangocairo.h>

#ifdef GDK_WINDOWING_WAYLAND
#include <gdk/gdkwayland.h>
#endif

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#endif

#include <dlfcn.h>
#include <math.h>
#include <pthread.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

extern int32_t woxGoLinuxStart(uintptr_t context);
extern void woxGoLinuxCall(uintptr_t context);
extern void woxGoLinuxFrame(uintptr_t context, float width, float height, int32_t pixel_width, int32_t pixel_height, float scale);
extern void woxGoLinuxFocus(uintptr_t context, uint64_t epoch, int32_t active);
extern void woxGoLinuxDestroyed(uintptr_t context, uint64_t epoch, int32_t active);
extern int32_t woxGoLinuxKey(uintptr_t context, const char *key, uint8_t modifiers, int32_t down, int32_t repeat, int32_t composing);
extern void woxGoLinuxWebViewEscapeDiagnostic(uintptr_t context, const char *detail);
extern void woxGoLinuxTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoLinuxPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);
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
};

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
  unsigned long previous_active_window;
  float preferred_width;
  float preferred_height;
  float preferred_x;
  float preferred_y;
  double pointer_root_x;
  double pointer_root_y;
  guint32 pointer_time;
  bool visible;
  bool active;
  bool hide_on_blur;
  bool native_dialog_active;
  bool nonactivating;
  bool restore_previous_on_hide;
  bool layer_shell_enabled;
  bool input_enabled;
  bool input_composing;
  bool active_web_view_transient;
  bool pointer_over_web_view;
  uint8_t pointer_cursor;
  char *web_view_cursor_name;
  bool has_preferred_position;
  bool closed;
  GdkRectangle input_cursor_rect;
};

static pthread_t wox_linux_main_thread;
static gint wox_linux_runtime_running = 0;
static gint wox_linux_loop_active = 0;
static gint wox_linux_window_count = 0;

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
  g_object_unref(native_cursor);
  return 0;
}

// apply_linux_pointer_cursor lets the active page cursor override the Go-rendered host cursor.
static int32_t apply_linux_pointer_cursor(WoxLinuxWindow *window) {
  static const char *const host_cursor_names[] = {
      "default",
      "text",
      "move",
      "crosshair",
      "ew-resize",
      "ns-resize",
      "nwse-resize",
      "nesw-resize",
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

static bool initialize_renderer(WoxLinuxRenderer *renderer, GtkWidget *gl_area) {
  gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
  GError *error = gtk_gl_area_get_error(GTK_GL_AREA(gl_area));
  if (error != NULL) {
    g_warning("Wox Go UI: failed to create OpenGL context: %s", error->message);
    return false;
  }
  renderer->rect_program = create_program(rect_vertex_source, rect_fragment_source);
  renderer->texture_program = create_program(texture_vertex_source, texture_fragment_source);
  if (renderer->rect_program == 0 || renderer->texture_program == 0) {
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
  renderer->ready = true;
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
  gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
  if (gtk_gl_area_get_error(GTK_GL_AREA(gl_area)) == NULL) {
    if (renderer->frame_framebuffer != 0) {
      glDeleteFramebuffers(1, &renderer->frame_framebuffer);
    }
    if (renderer->frame_texture != 0) {
      glDeleteTextures(1, &renderer->frame_texture);
    }
    glDeleteVertexArrays(1, &renderer->vertex_array);
    glDeleteProgram(renderer->texture_program);
    glDeleteProgram(renderer->rect_program);
  }
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

static void emit_pointer(WoxLinuxWindow *window, GdkEvent *event, uint8_t kind, double x, double y, uint8_t button, double scroll_x, double scroll_y, GdkModifierType state, GdkWindow *event_window) {
  if (!window->closed && window->context != 0) {
    double client_x = x;
    double client_y = y;
    pointer_client_position(window, event_window, x, y, &client_x, &client_y);
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
  window->pointer_root_x = event->x_root;
  window->pointer_root_y = event->y_root;
  window->pointer_time = event->time;
  emit_pointer(window, (GdkEvent *)event, WOX_POINTER_MOVE, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
  return TRUE;
}

static gboolean on_pointer_crossing(GtkWidget *widget, GdkEventCrossing *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer) {
    return FALSE;
  }
  uint8_t kind = event->type == GDK_ENTER_NOTIFY ? WOX_POINTER_ENTER : WOX_POINTER_LEAVE;
  emit_pointer(window, (GdkEvent *)event, kind, event->x, event->y, 0, 0.0, 0.0, event->state, event->window);
  return TRUE;
}

static gboolean on_pointer_button(GtkWidget *widget, GdkEventButton *event, gpointer data) {
  (void)widget;
  WoxLinuxWindow *window = data;
  if (window->forwarding_embedded_pointer) {
    return FALSE;
  }
  window->pointer_root_x = event->x_root;
  window->pointer_root_y = event->y_root;
  window->pointer_time = event->time;
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

static bool is_wlroots_compositor(void) {
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
  bool result = strstr(lower, "hyprland") != NULL || strstr(lower, "sway") != NULL || strstr(lower, "wayfire") != NULL || strstr(lower, "river") != NULL || strstr(lower, "wlroots") != NULL;
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
  if (!is_wlroots_compositor() || !resolve_layer_shell() || !layer_is_supported()) {
    return false;
  }
  layer_init_for_window(window);
  layer_set_layer(window, WOX_LAYER_OVERLAY);
  layer_set_keyboard_mode(window, WOX_KEYBOARD_EXCLUSIVE);
  layer_set_anchor(window, WOX_EDGE_TOP, TRUE);
  layer_set_anchor(window, WOX_EDGE_LEFT, TRUE);
  layer_set_anchor(window, WOX_EDGE_BOTTOM, FALSE);
  layer_set_anchor(window, WOX_EDGE_RIGHT, FALSE);
  return true;
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

#ifdef GDK_WINDOWING_X11
static Display *x11_display(WoxLinuxWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  if (gdk_window == NULL || !GDK_IS_X11_WINDOW(gdk_window)) {
    return NULL;
  }
  return GDK_DISPLAY_XDISPLAY(gdk_window_get_display(gdk_window));
}

static Window x11_window_id(WoxLinuxWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(window->window);
  return gdk_window != NULL && GDK_IS_X11_WINDOW(gdk_window) ? GDK_WINDOW_XID(gdk_window) : None;
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
  Window current = x11_window_id(window);
  Window active = active_x11_window(window);
  if (active != None && active != current) {
    window->previous_active_window = active;
    window->restore_previous_on_hide = true;
  }
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
  Display *display = x11_display(window);
  Window previous = (Window)window->previous_active_window;
  window->previous_active_window = 0;
  if (display == NULL || previous == None) {
    return;
  }
  XWindowAttributes attributes;
  if (XGetWindowAttributes(display, previous, &attributes) == 0 || attributes.map_state != IsViewable) {
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
  XSendEvent(display, DefaultRootWindow(display), False, SubstructureRedirectMask | SubstructureNotifyMask, &event);
  XFlush(display);
}
#else
static void save_previous_x11_window(WoxLinuxWindow *window) {
  (void)window;
}
static void request_x11_activation(WoxLinuxWindow *window) {
  (void)window;
}
static void restore_previous_x11_window(WoxLinuxWindow *window) {
  (void)window;
}
#endif

static void hide_native(WoxLinuxWindow *window, bool restore_previous) {
  if (window->closed || !window->visible) {
    return;
  }
  bool should_restore = restore_previous && window->active && window->restore_previous_on_hide;
  emit_focus(window, false);
  if (window->closed) {
    return;
  }
  window->visible = false;
  gtk_widget_hide(window->window);
  if (should_restore) {
    restore_previous_x11_window(window);
  }
  window->restore_previous_on_hide = false;
  window->previous_active_window = 0;
}

static void on_gl_realize(GtkGLArea *area, gpointer data) {
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  initialize_renderer(renderer, GTK_WIDGET(area));
}

static void on_gl_unrealize(GtkGLArea *area, gpointer data) {
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  destroy_renderer(renderer, GTK_WIDGET(area));
}

static void present_linux_renderer(WoxLinuxRenderer *renderer) {
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
}

static gboolean on_gl_render(GtkGLArea *area, GdkGLContext *context, gpointer data) {
  (void)context;
  WoxLinuxWindow *window = data;
  WoxLinuxRenderer *renderer = GTK_WIDGET(area) == window->overlay_gl_area ? &window->overlay_renderer : &window->renderer;
  if (window->closed || !window->visible || window->context == 0 || !renderer->ready) {
    return TRUE;
  }
  if (GTK_WIDGET(area) == window->overlay_gl_area) {
    if (renderer->frame_framebuffer == 0) {
      glClearColor(0.0f, 0.0f, 0.0f, 0.0f);
      glClear(GL_COLOR_BUFFER_BIT);
    } else {
      present_linux_renderer(renderer);
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
    woxGoLinuxFrame(window->context, (float)width, (float)height, width * scale, height * scale, (float)scale);
  }
  return TRUE;
}

static void on_scale_changed(GObject *object, GParamSpec *specification, gpointer data) {
  (void)object;
  (void)specification;
  WoxLinuxWindow *window = data;
  if (!window->closed) {
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
  if (!gtk_init_check(NULL, NULL)) {
    return -2;
  }
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

WoxLinuxWindow *wox_linux_window_create(const char *title, float width, float height, int32_t hide_on_blur, int32_t window_role, int32_t nonactivating, uintptr_t context) {
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
  bool application_window = window_role == 1;
  window->nonactivating = nonactivating != 0;
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
  // Application windows must stay visible to the desktop shell instead of using launcher-only utility hints.
  if (application_window) {
    gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), FALSE);
    gtk_window_set_type_hint(GTK_WINDOW(window->window), GDK_WINDOW_TYPE_HINT_NORMAL);
    gtk_window_set_keep_above(GTK_WINDOW(window->window), FALSE);
  } else {
    gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), TRUE);
    gtk_window_set_type_hint(GTK_WINDOW(window->window), GDK_WINDOW_TYPE_HINT_UTILITY);
    gtk_window_set_keep_above(GTK_WINDOW(window->window), TRUE);
  }
  gtk_window_set_accept_focus(GTK_WINDOW(window->window), !window->nonactivating);
  gtk_window_set_focus_on_map(GTK_WINDOW(window->window), !window->nonactivating);
  gtk_window_set_position(GTK_WINDOW(window->window), GTK_WIN_POS_CENTER);
  gtk_widget_set_app_paintable(window->window, TRUE);
  GtkTargetEntry file_drop_target = {(gchar *)"text/uri-list", 0, 0};
  gtk_drag_dest_set(window->window, GTK_DEST_DEFAULT_ALL, &file_drop_target, 1, GDK_ACTION_COPY);

  window->layer_shell_enabled = !application_window && enable_layer_shell(GTK_WINDOW(window->window));

  gtk_gl_area_set_required_version(GTK_GL_AREA(window->gl_area), 3, 3);
  gtk_gl_area_set_use_es(GTK_GL_AREA(window->gl_area), FALSE);
  // Recording surfaces need real alpha; ordinary Linux windows remain opaque because compositor blur is inconsistent.
  gtk_gl_area_set_has_alpha(GTK_GL_AREA(window->gl_area), window->nonactivating);
  gtk_gl_area_set_has_depth_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_has_stencil_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_auto_render(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_widget_set_can_focus(window->gl_area, TRUE);
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
  g_signal_connect(window->gl_area, "unrealize", G_CALLBACK(on_gl_unrealize), window);
  g_signal_connect(window->gl_area, "render", G_CALLBACK(on_gl_render), window);
  g_signal_connect(window->gl_area, "notify::scale-factor", G_CALLBACK(on_scale_changed), window);
  g_signal_connect(window->overlay_gl_area, "realize", G_CALLBACK(on_gl_realize), window);
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
  g_signal_connect(window->window, "key-press-event", G_CALLBACK(on_key_press), window);
  g_signal_connect(window->window, "key-release-event", G_CALLBACK(on_key_release), window);
  g_signal_connect(window->window, "destroy", G_CALLBACK(on_window_destroy), window);
  g_signal_connect(window->im_context, "commit", G_CALLBACK(on_ime_commit), window);
  g_signal_connect(window->im_context, "preedit-changed", G_CALLBACK(on_ime_preedit_changed), window);

  gtk_widget_realize(window->window);
  gtk_widget_realize(window->gl_area);
  gtk_widget_realize(window->overlay_gl_area);
  gtk_im_context_set_client_window(window->im_context, gtk_widget_get_window(window->window));
  if (!window->renderer.ready || !window->overlay_renderer.ready) {
    gtk_widget_destroy(window->window);
    free(window);
    return NULL;
  }
  window->context = context;
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
  save_previous_x11_window(window);
  place_window(window);
  gtk_widget_show_all(window->window);
  gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
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
  window->preferred_x = call->x;
  window->preferred_y = call->y;
  window->preferred_width = call->width;
  window->preferred_height = call->height;
  window->has_preferred_position = true;
  int width = (int)ceilf(call->width);
  int height = (int)ceilf(call->height);
  if (window->layer_shell_enabled) {
    // Layer-shell size negotiation needs all GTK size hints to agree; a resize request
    // alone can leave the widget allocation on its pre-map size and suppress rendering.
    gtk_window_set_default_size(GTK_WINDOW(window->window), width, height);
    gtk_widget_set_size_request(window->window, width, height);
  }
  gtk_window_resize(GTK_WINDOW(window->window), width, height);
  place_window(window);
  if (window->visible) {
    gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
  }
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
  gtk_window_resize(GTK_WINDOW(window->window), (int)ceilf(width), (int)ceilf(height));
  place_window(window);
  if (window->visible) {
    gtk_gl_area_queue_render(GTK_GL_AREA(window->gl_area));
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

int32_t wox_linux_window_show_webview(WoxLinuxWindow *window, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height) {
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

static void invalidate_main(void *data) {
  WoxWindowCall *call = data;
  if (call->window->closed) {
    call->result = -1;
    return;
  }
  gtk_gl_area_queue_render(GTK_GL_AREA(call->window->gl_area));
}

int32_t wox_linux_window_invalidate(WoxLinuxWindow *window) {
  if (window == NULL) {
    return -1;
  }
  WoxWindowCall call = {.window = window};
  return run_on_main_sync(invalidate_main, &call) ? call.result : -1;
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
  GdkWindow *native = gtk_widget_get_window(call->window->window);
  if (native == NULL) {
    call->result = -1;
    return;
  }
  gdk_window_set_pass_through(native, call->enabled);
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
  }
  // Native accessibility widgets mirror the GPU tree but must not consume visual pointer input.
  // Forward their events through the same portable Host path before GTK button defaults run.
  gtk_widget_add_events(widget, GDK_POINTER_MOTION_MASK | GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK | GDK_ENTER_NOTIFY_MASK | GDK_LEAVE_NOTIFY_MASK | GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
  g_signal_connect(widget, "motion-notify-event", G_CALLBACK(on_pointer_motion), window);
  g_signal_connect(widget, "enter-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(widget, "leave-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(widget, "button-press-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(widget, "button-release-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(widget, "scroll-event", G_CALLBACK(on_pointer_scroll), window);
  return 0;
}

int32_t wox_linux_accessibility_end(WoxLinuxWindow *window) {
  if (window == NULL || window->closed || window->accessibility_layer == NULL) {
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

int32_t wox_linux_window_measure_text(WoxLinuxWindow *window, const char *text, const char *font_family, float font_size, uint8_t font_weight, float *width, float *height, float *baseline) {
  if (window == NULL || text == NULL || font_family == NULL || width == NULL || height == NULL || baseline == NULL || font_size <= 0.0f || font_weight > 1 || !g_utf8_validate(text, -1, NULL) || !g_utf8_validate(font_family, -1, NULL)) {
    return -1;
  }
  WoxTextMeasureCall call = {
      .window = window,
      .text = text,
			.font_family = font_family,
      .font_size = font_size,
      .font_weight = font_weight,
      .width = width,
      .height = height,
      .baseline = baseline,
  };
  return run_on_main_sync(measure_text_main, &call) ? call.result : -1;
}

static void close_main(void *data) {
  WoxWindowCall *call = data;
  if (!call->window->closed) {
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
  if (window == NULL || window->closed || renderer == NULL || gl_area == NULL || !renderer->ready || renderer->frame_open || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    return -1;
  }
  gtk_gl_area_make_current(GTK_GL_AREA(gl_area));
  if (gtk_gl_area_get_error(GTK_GL_AREA(gl_area)) != NULL) {
    return -1;
  }
  gtk_gl_area_attach_buffers(GTK_GL_AREA(gl_area));
  renderer->logical_width = logical_width;
  renderer->logical_height = logical_height;
  renderer->scale = scale;
  int pixel_width = (int)ceilf(logical_width * scale);
  int pixel_height = (int)ceilf(logical_height * scale);
  glGetIntegerv(GL_DRAW_FRAMEBUFFER_BINDING, &renderer->default_framebuffer);
  bool recreated = false;
  if (!ensure_frame_storage(renderer, pixel_width, pixel_height, &recreated)) {
    glBindFramebuffer(GL_FRAMEBUFFER, renderer->default_framebuffer);
    return -1;
  }
  glBindFramebuffer(GL_FRAMEBUFFER, renderer->frame_framebuffer);
  float clear[4];
  premultiplied_color(red, green, blue, force_opaque ? 255 : alpha, clear);
  glViewport(0, 0, pixel_width, pixel_height);
  glDisable(GL_DEPTH_TEST);
  renderer->damage_active = !recreated && damage_width > 0.0f && damage_height > 0.0f;
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
  return 0;
}

int32_t wox_linux_window_begin_frame(WoxLinuxWindow *window, float logical_width, float logical_height, float scale, float damage_x, float damage_y, float damage_width, float damage_height, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL) {
    return -1;
  }
  window->embedded_surface_overlay_active = false;
  window->active_renderer = &window->renderer;
  window->active_gl_area = window->gl_area;
  return begin_linux_renderer_frame(window, window->active_renderer, window->active_gl_area, true, logical_width, logical_height, scale, damage_x, damage_y, damage_width, damage_height, red, green, blue, alpha);
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

int32_t wox_linux_window_draw_text(WoxLinuxWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || text == NULL || font_family == NULL) {
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

  cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, pixel_width, pixel_height);
  if (cairo_surface_status(surface) != CAIRO_STATUS_SUCCESS) {
    cairo_surface_destroy(surface);
    return -1;
  }
  cairo_t *cairo = cairo_create(surface);
  cairo_set_source_rgba(cairo, 1.0, 1.0, 1.0, 1.0);
  PangoLayout *layout = pango_cairo_create_layout(cairo);
  PangoFontDescription *font = pango_font_description_new();
	pango_font_description_set_family(font, font_family[0] == '\0' ? "Sans" : font_family);
  pango_font_description_set_absolute_size(font, font_size * renderer->scale * PANGO_SCALE);
  pango_font_description_set_weight(font, font_weight == 1 ? PANGO_WEIGHT_SEMIBOLD : PANGO_WEIGHT_NORMAL);
  pango_layout_set_font_description(layout, font);
  pango_layout_set_text(layout, text, -1);
  pango_layout_set_width(layout, pixel_width * PANGO_SCALE);
  pango_layout_set_single_paragraph_mode(layout, TRUE);
  // Do not set a positive layout height. CJK fonts can report a line box taller
  // than the destination slot, and Pango then omits the only line. The cairo
  // image already clips overflow to pixel_height.
  pango_cairo_show_layout(cairo, layout);
  cairo_surface_flush(surface);

  GLuint texture = 0;
  glGenTextures(1, &texture);
  glActiveTexture(GL_TEXTURE0);
  glBindTexture(GL_TEXTURE_2D, texture);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
  glPixelStorei(GL_UNPACK_ALIGNMENT, 4);
  glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, pixel_width, pixel_height, 0, GL_BGRA, GL_UNSIGNED_BYTE, cairo_image_surface_get_data(surface));

  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->texture_program);
  glUniform2f(renderer->texture_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->texture_bounds, x, y, width, height);
  glUniform4fv(renderer->texture_color, 1, color);
  glUniform1f(renderer->texture_rotation, 0.0f);
  glUniform1f(renderer->texture_radius, 0.0f);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  glBindTexture(GL_TEXTURE_2D, 0);
  glDeleteTextures(1, &texture);

  pango_font_description_free(font);
  g_object_unref(layout);
  cairo_destroy(cairo);
  cairo_surface_destroy(surface);
  return 0;
}

int32_t wox_linux_window_draw_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height, float rotation_radians, float corner_radius) {
  if (window == NULL || window->active_renderer == NULL || !window->active_renderer->frame_open || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxLinuxRenderer *renderer = window->active_renderer;
  GLuint texture = 0;
  glGenTextures(1, &texture);
  glActiveTexture(GL_TEXTURE0);
  glBindTexture(GL_TEXTURE_2D, texture);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
  glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
  glPixelStorei(GL_UNPACK_ALIGNMENT, 4);
  glPixelStorei(GL_UNPACK_ROW_LENGTH, row_stride / 4);
  glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, image_width, image_height, 0, GL_RGBA, GL_UNSIGNED_BYTE, pixels);
  glPixelStorei(GL_UNPACK_ROW_LENGTH, 0);

  float color[4];
  premultiplied_color(255, 255, 255, 255, color);
  glUseProgram(renderer->texture_program);
  glUniform2f(renderer->texture_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->texture_bounds, x, y, width, height);
  glUniform4fv(renderer->texture_color, 1, color);
  glUniform1f(renderer->texture_rotation, rotation_radians);
  glUniform1f(renderer->texture_radius, corner_radius);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  glBindTexture(GL_TEXTURE_2D, 0);
  glDeleteTextures(1, &texture);
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

static int32_t finish_linux_renderer_frame(WoxLinuxRenderer *renderer) {
  if (renderer == NULL || !renderer->frame_open) {
    return -1;
  }
  glBindVertexArray(0);
  glUseProgram(0);
  glDisable(GL_SCISSOR_TEST);
  glBindFramebuffer(GL_READ_FRAMEBUFFER, renderer->frame_framebuffer);
  glBindFramebuffer(GL_DRAW_FRAMEBUFFER, renderer->default_framebuffer);
  glBlitFramebuffer(0, 0, renderer->frame_width, renderer->frame_height, 0, 0, renderer->frame_width, renderer->frame_height, GL_COLOR_BUFFER_BIT, GL_NEAREST);
  glBindFramebuffer(GL_FRAMEBUFFER, renderer->default_framebuffer);
  glFlush();
  renderer->frame_open = false;
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
  int32_t result = finish_linux_renderer_frame(background);
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
  int32_t result = finish_linux_renderer_frame(window->active_renderer);
  if (result == 0 && !window->embedded_surface_overlay_active) {
    WoxLinuxRenderer *background = &window->renderer;
    window->active_renderer = &window->overlay_renderer;
    window->active_gl_area = window->overlay_gl_area;
    result = begin_linux_renderer_frame(window, window->active_renderer, window->active_gl_area, false, background->logical_width, background->logical_height, background->scale, 0.0f, 0.0f, 0.0f, 0.0f, 0, 0, 0, 0);
    if (result == 0) {
      result = finish_linux_renderer_frame(window->active_renderer);
    }
  }
  if (result == 0) {
    gtk_gl_area_queue_render(GTK_GL_AREA(window->overlay_gl_area));
  }
  gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
  window->active_renderer = NULL;
  window->active_gl_area = NULL;
  return result;
}
