#include "my_application.h"

#include <cairo.h>
#include <flutter_linux/flutter_linux.h>
#include <gdk/gdk.h>
#include <math.h>
#include <stdarg.h>
#ifdef GDK_WINDOWING_X11
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <gdk/gdkx.h>
#endif

#include "flutter/generated_plugin_registrant.h"

struct _MyApplication {
  GtkApplication parent_instance;
  char **dart_entrypoint_arguments;
  GtkWindow *window; // Store reference to the main window
};

// Global variable to store method channel for window events
static FlMethodChannel *g_method_channel = nullptr;

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

static void log(const char *format, ...) {
  // va_list args;
  // va_start(args, format);
  // g_logv(G_LOG_DOMAIN, G_LOG_LEVEL_MESSAGE, format, args);
  // va_end(args);
}

// Function to draw rounded rectangle
static void cairo_rounded_rectangle(cairo_t *cr, double x, double y,
                                    double width, double height,
                                    double radius) {
  cairo_new_sub_path(
      cr); // Fix function name: cairo_new_subpath -> cairo_new_sub_path
  cairo_arc(cr, x + radius, y + radius, radius, M_PI, 3 * M_PI / 2);
  cairo_line_to(cr, x + width - radius, y);
  cairo_arc(cr, x + width - radius, y + radius, radius, 3 * M_PI / 2, 0);
  cairo_line_to(cr, x + width, y + height - radius);
  cairo_arc(cr, x + width - radius, y + height - radius, radius, 0, M_PI / 2);
  cairo_line_to(cr, x + radius, y + height);
  cairo_arc(cr, x + radius, y + height - radius, radius, M_PI / 2, M_PI);
  cairo_close_path(cr);
}

static void set_window_shape(GtkWindow *window) {
  GdkWindow *gdk_window = gtk_widget_get_window(GTK_WIDGET(window));
  if (!gdk_window) {
    return;
  }

  int width, height;
  gtk_window_get_size(window, &width, &height);

  cairo_surface_t *surface =
      cairo_image_surface_create(CAIRO_FORMAT_A1, width, height);
  cairo_t *cr = cairo_create(surface);

  cairo_set_source_rgba(cr, 1, 1, 1, 1); // white fill
  cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
  cairo_rounded_rectangle(cr, 0, 0, width, height, 10); // rounded radius is 10
  cairo_fill(cr);

  cairo_destroy(cr);

  cairo_region_t *region = gdk_cairo_region_create_from_surface(surface);
  gdk_window_shape_combine_region(gdk_window, region, 0, 0);
  cairo_region_destroy(region);

  cairo_surface_destroy(surface);
}

// Callback function to handle window size changes
static void on_size_allocate(GtkWidget *widget, GdkRectangle *allocation,
                             gpointer user_data) {
  set_window_shape(GTK_WINDOW(user_data));
}

// Callback function to handle window focus-out event
static gboolean on_window_focus_out(GtkWidget *widget, GdkEventFocus *event,
                                    gpointer user_data) {
  log("FLUTTER: Window lost focus");

  // Notify Flutter through method channel
  if (g_method_channel != nullptr) {
    g_autoptr(FlValue) args = fl_value_new_null();
    fl_method_channel_invoke_method(g_method_channel, "onWindowBlur", args,
                                    nullptr, nullptr, nullptr);
  }

  // Return FALSE to allow the event to propagate further
  return FALSE;
}

// Method channel handler
static void method_call_cb(FlMethodChannel *channel, FlMethodCall *method_call,
                           gpointer user_data) {
  MyApplication *self = MY_APPLICATION(user_data);
  GtkWindow *window = self->window;
  const gchar *method = fl_method_call_get_name(method_call);
  FlValue *args = fl_method_call_get_args(method_call);
  g_autoptr(FlMethodResponse) response = nullptr;

  if (strcmp(method, "setSize") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      double width = fl_value_get_float(fl_value_lookup_string(args, "width"));
      double height =
          fl_value_get_float(fl_value_lookup_string(args, "height"));
      gtk_window_resize(window, (int)width, (int)height);
      response = FL_METHOD_RESPONSE(
          fl_method_success_response_new(fl_value_new_null()));
    }
  } else if (strcmp(method, "getPosition") == 0) {
    int x, y;
    gtk_window_get_position(window, &x, &y);
    g_autoptr(FlValue) result = fl_value_new_map();
    fl_value_set_string_take(result, "x", fl_value_new_int(x));
    fl_value_set_string_take(result, "y", fl_value_new_int(y));
    response = FL_METHOD_RESPONSE(fl_method_success_response_new(result));
  } else if (strcmp(method, "setPosition") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      double x = fl_value_get_float(fl_value_lookup_string(args, "x"));
      double y = fl_value_get_float(fl_value_lookup_string(args, "y"));
      gtk_window_move(window, (int)x, (int)y);
      log("FLUTTER: setPosition, x: %f, y: %f", x, y);
      response = FL_METHOD_RESPONSE(
          fl_method_success_response_new(fl_value_new_null()));
    }
  } else if (strcmp(method, "center") == 0) {
    // 获取窗口尺寸，优先使用传入的参数
    int window_width, window_height;

    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      FlValue *width_value = fl_value_lookup_string(args, "width");
      FlValue *height_value = fl_value_lookup_string(args, "height");

      if (width_value != nullptr &&
          fl_value_get_type(width_value) == FL_VALUE_TYPE_FLOAT &&
          height_value != nullptr &&
          fl_value_get_type(height_value) == FL_VALUE_TYPE_FLOAT) {
        // 使用传入的尺寸
        window_width = (int)fl_value_get_float(width_value);
        window_height = (int)fl_value_get_float(height_value);

        // 调整窗口大小
        gtk_window_resize(window, window_width, window_height);
      } else {
        // 使用当前窗口尺寸
        gtk_window_get_size(window, &window_width, &window_height);
      }
    } else {
      // 使用当前窗口尺寸
      gtk_window_get_size(window, &window_width, &window_height);
    }

    // 获取屏幕尺寸 (使用非弃用的 API)
    GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(window));
    GdkMonitor *monitor = gdk_display_get_primary_monitor(display);
    GdkRectangle workarea;
    gdk_monitor_get_workarea(monitor, &workarea);

    // 计算居中位置
    int x = workarea.x + (workarea.width - window_width) / 2;
    int y = workarea.y + (workarea.height - window_height) / 2;

    // 设置窗口位置
    gtk_window_move(window, x, y);

    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else if (strcmp(method, "show") == 0) {
    gtk_widget_show(GTK_WIDGET(window));
    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else if (strcmp(method, "hide") == 0) {
    gtk_widget_hide(GTK_WIDGET(window));
    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else if (strcmp(method, "focus") == 0) {
    log("FLUTTER: focus - attempting to focus window");

    GdkWindow *gdk_window = gtk_widget_get_window(GTK_WIDGET(window));
    if (gdk_window) {
      gdk_window_raise(gdk_window);
      gdk_window_focus(gdk_window, GDK_CURRENT_TIME);

#ifdef GDK_WINDOWING_X11
      if (GDK_IS_X11_WINDOW(gdk_window)) {
        Display *display =
            GDK_DISPLAY_XDISPLAY(gdk_window_get_display(gdk_window));
        Window xid = GDK_WINDOW_XID(gdk_window);

        log("FLUTTER: focus - using X11 specific methods");

        // 更安全的X11代码实现
        XRaiseWindow(display, xid);

        // 使用简化的_NET_ACTIVE_WINDOW消息
        Atom net_active_window =
            XInternAtom(display, "_NET_ACTIVE_WINDOW", False);
        if (net_active_window != None) {
          XEvent xev;
          memset(&xev, 0, sizeof(xev));
          xev.type = ClientMessage;
          xev.xclient.type = ClientMessage;
          xev.xclient.window = xid;
          xev.xclient.message_type = net_active_window;
          xev.xclient.format = 32;
          xev.xclient.data.l[0] = 2; // 来源指示: 2 = 来自应用程序的请求
          xev.xclient.data.l[1] = CurrentTime;

          XSendEvent(display, DefaultRootWindow(display), False,
                     SubstructureRedirectMask | SubstructureNotifyMask, &xev);

          XFlush(display);
        }
      }
#endif
    }

    // 使用GTK的标准方法
    gtk_window_present(window);
    gtk_widget_grab_focus(GTK_WIDGET(window));
    log("FLUTTER: focus - all focus operations completed");
    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else if (strcmp(method, "isVisible") == 0) {
    gboolean visible = gtk_widget_get_visible(GTK_WIDGET(window));
    response = FL_METHOD_RESPONSE(
        fl_method_success_response_new(fl_value_new_bool(visible)));
  } else if (strcmp(method, "setAlwaysOnTop") == 0) {
    gboolean always_on_top = FALSE;
    if (fl_value_get_type(args) == FL_VALUE_TYPE_BOOL) {
      always_on_top = fl_value_get_bool(args);
    }
    gtk_window_set_keep_above(window, always_on_top);
    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else if (strcmp(method, "waitUntilReadyToShow") == 0) {
    response =
        FL_METHOD_RESPONSE(fl_method_success_response_new(fl_value_new_null()));
  } else {
    response = FL_METHOD_RESPONSE(fl_method_not_implemented_response_new());
  }

  g_autoptr(GError) error = nullptr;
  if (!fl_method_call_respond(method_call, response, &error)) {
    g_warning("Failed to send response: %s", error->message);
  }
}

// Implements GApplication::activate.
static void my_application_activate(GApplication *application) {
  MyApplication *self = MY_APPLICATION(application);
  GtkWindow *window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));

  // Store window reference in application instance
  self->window = window;

  // Remove window decorations (titlebar)
  gtk_window_set_decorated(window, FALSE);

  // Use a header bar when running in GNOME as this is the common style used
  // by applications and is the setup most users will be using (e.g. Ubuntu
  // desktop).
  // If running on X and not using GNOME then just use a traditional title bar
  // in case the window manager does more exotic layout, e.g. tiling.
  // If running on Wayland assume the header bar will work (may need changing
  // if future cases occur).
  gboolean use_header_bar = TRUE;
#ifdef GDK_WINDOWING_X11
  GdkScreen *screen = gtk_window_get_screen(window);
  if (GDK_IS_X11_SCREEN(screen)) {
    const gchar *wm_name = gdk_x11_screen_get_window_manager_name(screen);
    if (g_strcmp0(wm_name, "GNOME Shell") != 0) {
      use_header_bar = FALSE;
    }
  }
#endif
  if (use_header_bar) {
    GtkHeaderBar *header_bar = GTK_HEADER_BAR(gtk_header_bar_new());
    gtk_widget_show(GTK_WIDGET(header_bar));
    gtk_header_bar_set_title(header_bar, "Wox");
    gtk_header_bar_set_show_close_button(header_bar, TRUE);
    gtk_window_set_titlebar(window, GTK_WIDGET(header_bar));
  } else {
    gtk_window_set_title(window, "Wox");
  }

  gtk_window_set_default_size(window, 1280, 720);

  // Prevent notifications and taskbar entries
  gtk_window_set_skip_taskbar_hint(window, TRUE);
  gtk_window_set_type_hint(window, GDK_WINDOW_TYPE_HINT_UTILITY);
  gtk_window_set_keep_above(window, TRUE);

  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  // By default the window background is transparent(not acrylic), which is not
  // what we want so following code is to make the window background opaque
  GtkBox *box = GTK_BOX(gtk_box_new(GTK_ORIENTATION_VERTICAL, 0));
  gtk_widget_show(GTK_WIDGET(box));
  const gchar *css = "box { background-color: #FFFFFF; }";
  GtkCssProvider *provider = gtk_css_provider_new();
  gtk_css_provider_load_from_data(provider, css, -1, nullptr);
  GtkStyleContext *context = gtk_widget_get_style_context(GTK_WIDGET(box));
  gtk_style_context_add_class(context, "box");
  gtk_style_context_add_provider(context, GTK_STYLE_PROVIDER(provider),
                                 GTK_STYLE_PROVIDER_PRIORITY_USER);

  g_object_unref(provider);
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(box));

  FlView *view = fl_view_new(project);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(box), GTK_WIDGET(view));
  fl_register_plugins(FL_PLUGIN_REGISTRY(view));

  // Set up method channel for window management
  g_autoptr(FlStandardMethodCodec) codec = fl_standard_method_codec_new();
  g_autoptr(FlMethodChannel) channel = fl_method_channel_new(
      fl_engine_get_binary_messenger(fl_view_get_engine(view)),
      "com.wox.linux_window_manager", FL_METHOD_CODEC(codec));
  fl_method_channel_set_method_call_handler(channel, method_call_cb,
                                            g_object_ref(self), g_object_unref);

  // Store method channel reference for window events
  g_method_channel = channel;
  g_object_add_weak_pointer(G_OBJECT(channel), (gpointer *)&g_method_channel);

  // Add signal connection to implement rounded window
  g_signal_connect(window, "realize", G_CALLBACK(set_window_shape), NULL);
  g_signal_connect(window, "size-allocate", G_CALLBACK(on_size_allocate),
                   window);

  // Add signal connection for window focus-out event
  g_signal_connect(window, "focus-out-event", G_CALLBACK(on_window_focus_out),
                   NULL);

  gtk_widget_grab_focus(GTK_WIDGET(view));
}

// Implements GApplication::local_command_line.
static gboolean my_application_local_command_line(GApplication *application,
                                                  gchar ***arguments,
                                                  int *exit_status) {
  MyApplication *self = MY_APPLICATION(application);
  // Strip out the first argument as it is the binary name.
  self->dart_entrypoint_arguments = g_strdupv(*arguments + 1);

  g_autoptr(GError) error = nullptr;
  if (!g_application_register(application, nullptr, &error)) {
    g_warning("Failed to register: %s", error->message);
    *exit_status = 1;
    return TRUE;
  }

  g_application_activate(application);

  // hide at startup
  if (self->window != NULL) {
    gtk_widget_hide(GTK_WIDGET(self->window));
  }

  *exit_status = 0;

  return TRUE;
}

// Implements GApplication::startup.
static void my_application_startup(GApplication *application) {
  G_APPLICATION_CLASS(my_application_parent_class)->startup(application);
}

// Implements GApplication::shutdown.
static void my_application_shutdown(GApplication *application) {
  G_APPLICATION_CLASS(my_application_parent_class)->shutdown(application);
}

// Implements GObject::dispose.
static void my_application_dispose(GObject *object) {
  MyApplication *self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);

  // Clear method channel reference
  if (g_method_channel != nullptr) {
    g_object_remove_weak_pointer(G_OBJECT(g_method_channel),
                                 (gpointer *)&g_method_channel);
    g_method_channel = nullptr;
  }

  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass *klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_APPLICATION_CLASS(klass)->startup = my_application_startup;
  G_APPLICATION_CLASS(klass)->shutdown = my_application_shutdown;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication *self) { self->window = NULL; }

MyApplication *my_application_new() {
  g_set_prgname(APPLICATION_ID);
  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}