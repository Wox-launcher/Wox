//go:build linux

#include "native_linux.h"

#include <gtk/gtk.h>
#include <epoxy/gl.h>
#include <pango/pangocairo.h>

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
extern void woxGoLinuxTextInput(uintptr_t context, uint8_t kind, const char *text);
extern void woxGoLinuxPointer(uintptr_t context, uint8_t kind, float x, float y, uint8_t button, float scroll_x, float scroll_y, uint8_t modifiers);

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
};

typedef struct {
  GLuint rect_program;
  GLuint texture_program;
  GLuint vertex_array;
  GLint rect_viewport;
  GLint rect_bounds;
  GLint rect_color;
  GLint rect_radius;
  GLint rect_stroke_width;
  GLint texture_viewport;
  GLint texture_bounds;
  GLint texture_color;
  bool ready;
  bool frame_open;
  float logical_width;
  float logical_height;
  float scale;
} WoxLinuxRenderer;

struct WoxLinuxWindow {
  GtkWidget *window;
  GtkWidget *overlay;
  GtkWidget *gl_area;
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
  uintptr_t context;
  uint64_t epoch;
  unsigned long previous_active_window;
  float preferred_width;
  float preferred_height;
  float preferred_x;
  float preferred_y;
  bool visible;
  bool active;
  bool hide_on_blur;
  bool native_dialog_active;
  bool restore_previous_on_hide;
  bool layer_shell_enabled;
  bool input_enabled;
  bool input_composing;
  bool active_web_view_transient;
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
  wox_webkit.available = wox_webkit.view_new != NULL && wox_webkit.load_uri != NULL && wox_webkit.load_html != NULL;
  if (!wox_webkit.available) {
    dlclose(wox_webkit.library);
    memset(&wox_webkit, 0, sizeof(wox_webkit));
    wox_webkit.initialized = true;
  }
  return wox_webkit.available;
}

static void on_webview_script_message(gpointer manager, gpointer javascript_result, gpointer data) {
  (void)manager;
  (void)javascript_result;
  WoxLinuxWindow *window = data;
  if (window != NULL && !window->closed && window->context != 0) {
    woxGoLinuxKey(window->context, "escape", 0, 1, 0, 0);
  }
}

static GtkWidget *create_web_view(WoxLinuxWindow *window, const char *inject_css) {
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
      if (supports_scripts && wox_webkit.register_script_message_handler(manager, "woxWebViewPreview")) {
        const char *escape_script = "(()=>{if(window.__woxUnhandledEscapeInstalled__)return;window.__woxUnhandledEscapeInstalled__=true;document.addEventListener('keydown',e=>{if(e.key!=='Escape'||e.repeat)return;setTimeout(()=>{if(e.defaultPrevented||e.cancelBubble)return;window.webkit.messageHandlers.woxWebViewPreview.postMessage('escape')},0)},true)})()";
        gpointer script = wox_webkit.script_new(escape_script, 0, 0, NULL, NULL);
        if (script != NULL) {
          wox_webkit.manager_add_script(manager, script);
          wox_webkit.script_unref(script);
          g_signal_connect(manager, "script-message-received::woxWebViewPreview", G_CALLBACK(on_webview_script_message), window);
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
    "in vec2 v_local;\n"
    "out vec4 fragment_color;\n"
    "void main() {\n"
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
    "out vec2 v_uv;\n"
    "void main() {\n"
    "  vec2 corners[4] = vec2[4](vec2(0.0, 0.0), vec2(1.0, 0.0), vec2(0.0, 1.0), vec2(1.0, 1.0));\n"
    "  vec2 corner = corners[gl_VertexID];\n"
    "  vec2 point = u_rect.xy + corner * u_rect.zw;\n"
    "  gl_Position = vec4(point.x / u_viewport.x * 2.0 - 1.0, 1.0 - point.y / u_viewport.y * 2.0, 0.0, 1.0);\n"
    "  v_uv = corner;\n"
    "}\n";

static const char *const texture_fragment_source =
    "#version 330 core\n"
    "uniform sampler2D u_texture;\n"
    "uniform vec4 u_color;\n"
    "in vec2 v_uv;\n"
    "out vec4 fragment_color;\n"
    "void main() { fragment_color = texture(u_texture, v_uv) * u_color; }\n";

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

static bool initialize_renderer(WoxLinuxWindow *window) {
  WoxLinuxRenderer *renderer = &window->renderer;
  gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
  GError *error = gtk_gl_area_get_error(GTK_GL_AREA(window->gl_area));
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
  renderer->texture_viewport = glGetUniformLocation(renderer->texture_program, "u_viewport");
  renderer->texture_bounds = glGetUniformLocation(renderer->texture_program, "u_rect");
  renderer->texture_color = glGetUniformLocation(renderer->texture_program, "u_color");
  glUseProgram(renderer->texture_program);
  glUniform1i(glGetUniformLocation(renderer->texture_program, "u_texture"), 0);
  glUseProgram(0);
  renderer->ready = true;
  return true;
}

static void destroy_renderer(WoxLinuxWindow *window) {
  WoxLinuxRenderer *renderer = &window->renderer;
  if (!renderer->ready) {
    return;
  }
  gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
  if (gtk_gl_area_get_error(GTK_GL_AREA(window->gl_area)) == NULL) {
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

static void emit_pointer(WoxLinuxWindow *window, uint8_t kind, double x, double y, uint8_t button, double scroll_x, double scroll_y, GdkModifierType state) {
  if (!window->closed && window->context != 0) {
    woxGoLinuxPointer(window->context, kind, (float)x, (float)y, button, (float)scroll_x, (float)scroll_y, portable_modifiers(state));
  }
}

static gboolean on_pointer_motion(GtkWidget *widget, GdkEventMotion *event, gpointer data) {
  (void)widget;
  emit_pointer(data, WOX_POINTER_MOVE, event->x, event->y, 0, 0.0, 0.0, event->state);
  return TRUE;
}

static gboolean on_pointer_crossing(GtkWidget *widget, GdkEventCrossing *event, gpointer data) {
  (void)widget;
  uint8_t kind = event->type == GDK_ENTER_NOTIFY ? WOX_POINTER_ENTER : WOX_POINTER_LEAVE;
  emit_pointer(data, kind, event->x, event->y, 0, 0.0, 0.0, event->state);
  return TRUE;
}

static gboolean on_pointer_button(GtkWidget *widget, GdkEventButton *event, gpointer data) {
  if (event->type == GDK_BUTTON_PRESS) {
    gtk_widget_grab_focus(widget);
  }
  uint8_t kind = event->type == GDK_BUTTON_RELEASE ? WOX_POINTER_UP : WOX_POINTER_DOWN;
  emit_pointer(data, kind, event->x, event->y, portable_pointer_button(event->button), 0.0, 0.0, event->state);
  return TRUE;
}

static gboolean on_pointer_scroll(GtkWidget *widget, GdkEventScroll *event, gpointer data) {
  (void)widget;
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
  emit_pointer(data, WOX_POINTER_SCROLL, event->x, event->y, 0, scroll_x, scroll_y, event->state);
  return TRUE;
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
  (void)area;
  WoxLinuxWindow *window = data;
  initialize_renderer(window);
}

static void on_gl_unrealize(GtkGLArea *area, gpointer data) {
  (void)area;
  WoxLinuxWindow *window = data;
  destroy_renderer(window);
}

static gboolean on_gl_render(GtkGLArea *area, GdkGLContext *context, gpointer data) {
  (void)context;
  WoxLinuxWindow *window = data;
  if (window->closed || !window->visible || window->context == 0 || !window->renderer.ready) {
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

WoxLinuxWindow *wox_linux_window_create(const char *title, float width, float height, int32_t hide_on_blur, uintptr_t context) {
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
  gtk_window_set_title(GTK_WINDOW(window->window), title != NULL ? title : "Wox Go UI");
  gtk_window_set_default_size(GTK_WINDOW(window->window), (int)ceilf(width), (int)ceilf(height));
  gtk_window_set_decorated(GTK_WINDOW(window->window), FALSE);
  gtk_window_set_skip_taskbar_hint(GTK_WINDOW(window->window), TRUE);
  gtk_window_set_type_hint(GTK_WINDOW(window->window), GDK_WINDOW_TYPE_HINT_UTILITY);
  gtk_window_set_keep_above(GTK_WINDOW(window->window), TRUE);
  gtk_window_set_accept_focus(GTK_WINDOW(window->window), TRUE);
  gtk_window_set_focus_on_map(GTK_WINDOW(window->window), TRUE);
  gtk_window_set_position(GTK_WINDOW(window->window), GTK_WIN_POS_CENTER);
  gtk_widget_set_app_paintable(window->window, TRUE);

  GdkScreen *screen = gtk_widget_get_screen(window->window);
  GdkVisual *visual = screen != NULL ? gdk_screen_get_rgba_visual(screen) : NULL;
  if (visual != NULL) {
    gtk_widget_set_visual(window->window, visual);
  }
  window->layer_shell_enabled = enable_layer_shell(GTK_WINDOW(window->window));

  gtk_gl_area_set_required_version(GTK_GL_AREA(window->gl_area), 3, 3);
  gtk_gl_area_set_use_es(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_has_alpha(GTK_GL_AREA(window->gl_area), TRUE);
  gtk_gl_area_set_has_depth_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_has_stencil_buffer(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_gl_area_set_auto_render(GTK_GL_AREA(window->gl_area), FALSE);
  gtk_widget_set_can_focus(window->gl_area, TRUE);
  gtk_widget_set_hexpand(window->gl_area, TRUE);
  gtk_widget_set_vexpand(window->gl_area, TRUE);
  gtk_widget_add_events(window->gl_area, GDK_POINTER_MOTION_MASK | GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK | GDK_ENTER_NOTIFY_MASK | GDK_LEAVE_NOTIFY_MASK | GDK_SCROLL_MASK | GDK_SMOOTH_SCROLL_MASK);
  gtk_container_add(GTK_CONTAINER(window->window), window->overlay);
  gtk_container_add(GTK_CONTAINER(window->overlay), window->gl_area);
  gtk_widget_show(window->overlay);
  gtk_widget_show(window->gl_area);

  g_signal_connect(window->gl_area, "realize", G_CALLBACK(on_gl_realize), window);
  g_signal_connect(window->gl_area, "unrealize", G_CALLBACK(on_gl_unrealize), window);
  g_signal_connect(window->gl_area, "render", G_CALLBACK(on_gl_render), window);
  g_signal_connect(window->gl_area, "notify::scale-factor", G_CALLBACK(on_scale_changed), window);
  g_signal_connect(window->gl_area, "motion-notify-event", G_CALLBACK(on_pointer_motion), window);
  g_signal_connect(window->gl_area, "enter-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(window->gl_area, "leave-notify-event", G_CALLBACK(on_pointer_crossing), window);
  g_signal_connect(window->gl_area, "button-press-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(window->gl_area, "button-release-event", G_CALLBACK(on_pointer_button), window);
  g_signal_connect(window->gl_area, "scroll-event", G_CALLBACK(on_pointer_scroll), window);
  g_signal_connect(window->window, "focus-in-event", G_CALLBACK(on_focus_in), window);
  g_signal_connect(window->window, "focus-out-event", G_CALLBACK(on_focus_out), window);
  g_signal_connect(window->window, "key-press-event", G_CALLBACK(on_key_press), window);
  g_signal_connect(window->window, "key-release-event", G_CALLBACK(on_key_release), window);
  g_signal_connect(window->window, "destroy", G_CALLBACK(on_window_destroy), window);
  g_signal_connect(window->im_context, "commit", G_CALLBACK(on_ime_commit), window);
  g_signal_connect(window->im_context, "preedit-changed", G_CALLBACK(on_ime_preedit_changed), window);

  gtk_widget_realize(window->window);
  gtk_widget_realize(window->gl_area);
  gtk_im_context_set_client_window(window->im_context, gtk_widget_get_window(window->window));
  if (!window->renderer.ready) {
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
    emit_focus(window, false);
    if (window->closed) {
      call->result = -1;
      return;
    }
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
    gdk_window_focus(gdk_window, GDK_CURRENT_TIME);
  }
  request_x11_activation(window);
  gtk_window_present(GTK_WINDOW(window->window));
  gtk_widget_grab_focus(window->gl_area);
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
  gtk_window_resize(GTK_WINDOW(window->window), (int)ceilf(call->width), (int)ceilf(call->height));
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
  GtkWidget *web_view = NULL;
  bool should_load = true;
  if (use_cache) {
    const char *cached_signature = g_hash_table_lookup(window->web_view_signatures, call->cache_key);
    if (g_strcmp0(cached_signature, call->inject_css) == 0) {
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
      web_view = create_web_view(window, call->inject_css);
      if (web_view == NULL) {
        g_free(content_key);
        call->result = -1;
        return;
      }
      g_hash_table_replace(window->web_view_cache, g_strdup(call->cache_key), web_view);
      g_hash_table_replace(window->web_view_signatures, g_strdup(call->cache_key), g_strdup(call->inject_css));
    }
    g_hash_table_replace(window->web_view_content_keys, g_strdup(call->cache_key), g_strdup(content_key));
  } else if (window->active_web_view_transient && g_strcmp0(window->active_web_view_signature, call->inject_css) == 0 && g_strcmp0(window->active_web_view_content_key, content_key) == 0) {
    web_view = window->active_web_view;
    should_load = false;
  } else {
    web_view = create_web_view(window, call->inject_css);
    if (web_view == NULL) {
      g_free(content_key);
      call->result = -1;
      return;
    }
  }

  if (web_view != window->active_web_view) {
    clear_active_web_view(window, true);
    window->active_web_view = web_view;
    window->active_web_view_transient = !use_cache;
    window->active_web_view_key = g_strdup(call->cache_key);
    window->active_web_view_signature = g_strdup(call->inject_css);
    window->active_web_view_content_key = g_strdup(content_key);
  }
  if (gtk_widget_get_parent(web_view) == NULL) {
    gtk_overlay_add_overlay(GTK_OVERLAY(window->overlay), web_view);
    gtk_overlay_set_overlay_pass_through(GTK_OVERLAY(window->overlay), web_view, FALSE);
  }
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
}

int32_t wox_linux_window_show_webview(WoxLinuxWindow *window, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, float x, float y, float width, float height) {
  if (window == NULL || url == NULL || html == NULL || inject_css == NULL || cache_key == NULL || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxWebViewCall call = {
      .window = window,
      .url = url,
      .html = html,
      .inject_css = inject_css,
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

int32_t wox_linux_window_begin_frame(WoxLinuxWindow *window, float logical_width, float logical_height, float scale, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || window->closed || !window->renderer.ready || window->renderer.frame_open || logical_width <= 0.0f || logical_height <= 0.0f || scale <= 0.0f) {
    return -1;
  }
  gtk_gl_area_make_current(GTK_GL_AREA(window->gl_area));
  if (gtk_gl_area_get_error(GTK_GL_AREA(window->gl_area)) != NULL) {
    return -1;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
  renderer->logical_width = logical_width;
  renderer->logical_height = logical_height;
  renderer->scale = scale;
  int pixel_width = (int)ceilf(logical_width * scale);
  int pixel_height = (int)ceilf(logical_height * scale);
  float clear[4];
  premultiplied_color(red, green, blue, alpha, clear);
  glViewport(0, 0, pixel_width, pixel_height);
  glDisable(GL_DEPTH_TEST);
  glDisable(GL_SCISSOR_TEST);
  glEnable(GL_BLEND);
  glBlendEquation(GL_FUNC_ADD);
  glBlendFunc(GL_ONE, GL_ONE_MINUS_SRC_ALPHA);
  glClearColor(clear[0], clear[1], clear[2], clear[3]);
  glClear(GL_COLOR_BUFFER_BIT);
  glBindVertexArray(renderer->vertex_array);
  renderer->frame_open = true;
  return 0;
}

int32_t wox_linux_window_fill_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || !window->renderer.frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->rect_program);
  glUniform2f(renderer->rect_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->rect_bounds, x, y, width, height);
  glUniform4fv(renderer->rect_color, 1, color);
  glUniform1f(renderer->rect_radius, radius);
  glUniform1f(renderer->rect_stroke_width, 0.0f);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  return 0;
}

int32_t wox_linux_window_stroke_rounded_rect(WoxLinuxWindow *window, float x, float y, float width, float height, float radius, float stroke_width, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || !window->renderer.frame_open) {
    return -1;
  }
  if (width <= 0.0f || height <= 0.0f || stroke_width <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
  float color[4];
  premultiplied_color(red, green, blue, alpha, color);
  glUseProgram(renderer->rect_program);
  glUniform2f(renderer->rect_viewport, renderer->logical_width, renderer->logical_height);
  glUniform4f(renderer->rect_bounds, x, y, width, height);
  glUniform4fv(renderer->rect_color, 1, color);
  glUniform1f(renderer->rect_radius, radius);
  glUniform1f(renderer->rect_stroke_width, stroke_width);
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  return 0;
}

int32_t wox_linux_window_draw_text(WoxLinuxWindow *window, const char *text, const char *font_family, float x, float y, float width, float height, float font_size, uint8_t font_weight, uint8_t red, uint8_t green, uint8_t blue, uint8_t alpha) {
  if (window == NULL || !window->renderer.frame_open || text == NULL || font_family == NULL) {
    return -1;
  }
  if (text[0] == '\0' || width <= 0.0f || height <= 0.0f || font_size <= 0.0f) {
    return 0;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
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
  pango_layout_set_height(layout, pixel_height * PANGO_SCALE);
  pango_layout_set_single_paragraph_mode(layout, TRUE);
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
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  glBindTexture(GL_TEXTURE_2D, 0);
  glDeleteTextures(1, &texture);

  pango_font_description_free(font);
  g_object_unref(layout);
  cairo_destroy(cairo);
  cairo_surface_destroy(surface);
  return 0;
}

int32_t wox_linux_window_draw_image(WoxLinuxWindow *window, const uint8_t *pixels, int32_t image_width, int32_t image_height, int32_t row_stride, float x, float y, float width, float height) {
  if (window == NULL || !window->renderer.frame_open || pixels == NULL || image_width <= 0 || image_height <= 0 || row_stride < image_width * 4 || width <= 0.0f || height <= 0.0f) {
    return -1;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
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
  glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
  glBindTexture(GL_TEXTURE_2D, 0);
  glDeleteTextures(1, &texture);
  return 0;
}

int32_t wox_linux_window_set_clip_rect(WoxLinuxWindow *window, float x, float y, float width, float height) {
  if (window == NULL || !window->renderer.frame_open) {
    return -1;
  }
  WoxLinuxRenderer *renderer = &window->renderer;
  float left = fmaxf(0.0f, fminf(renderer->logical_width, x));
  float top = fmaxf(0.0f, fminf(renderer->logical_height, y));
  float right = fmaxf(left, fminf(renderer->logical_width, x + fmaxf(0.0f, width)));
  float bottom = fmaxf(top, fminf(renderer->logical_height, y + fmaxf(0.0f, height)));
  int pixel_left = (int)floorf(left * renderer->scale);
  int pixel_right = (int)ceilf(right * renderer->scale);
  int pixel_top = (int)floorf(top * renderer->scale);
  int pixel_bottom = (int)ceilf(bottom * renderer->scale);
  int framebuffer_height = (int)ceilf(renderer->logical_height * renderer->scale);
  glEnable(GL_SCISSOR_TEST);
  glScissor(pixel_left, framebuffer_height - pixel_bottom, pixel_right - pixel_left, pixel_bottom - pixel_top);
  return 0;
}

int32_t wox_linux_window_clear_clip(WoxLinuxWindow *window) {
  if (window == NULL || !window->renderer.frame_open) {
    return -1;
  }
  glDisable(GL_SCISSOR_TEST);
  return 0;
}

int32_t wox_linux_window_end_frame(WoxLinuxWindow *window) {
  if (window == NULL || !window->renderer.frame_open) {
    return -1;
  }
  glBindVertexArray(0);
  glUseProgram(0);
  glDisable(GL_SCISSOR_TEST);
  glFlush();
  window->renderer.frame_open = false;
  return 0;
}
