#include "my_application.h"
#include "gtk_window.h"
#include "linux_window_manager.h"

#include <flutter_linux/flutter_linux.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif

#include "flutter/generated_plugin_registrant.h"
#include <cstdint>
#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>
#include <gtk/gtk.h>

struct _MyApplication {
  GtkApplication parent_instance;
  char **dart_entrypoint_arguments;
};

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

static void method_call_cb(FlMethodChannel *channel, FlMethodCall *method_call,
                           gpointer user_data) {
  const gchar *method = fl_method_call_get_name(method_call);
  FlValue *args = fl_method_call_get_args(method_call);
  GtkWindow *window = GTK_WINDOW(user_data);

  if (g_strcmp0(method, "ensureInitialized") == 0) {
    fl_method_call_respond_success(method_call, NULL, NULL);
  } else if (g_strcmp0(method, "setSize") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      FlValue *width_value = fl_value_lookup_string(args, "width");
      FlValue *height_value = fl_value_lookup_string(args, "height");

      if (width_value && height_value) {
        int width = (int)fl_value_get_float(width_value);
        int height = (int)fl_value_get_float(height_value);

        gtk_window_resize(window, width, height);
        fl_method_call_respond_success(method_call, NULL, NULL);
      } else {
        fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                     "Invalid arguments for setSize", NULL);
      }
    } else {
      fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                   "Invalid arguments for setSize", NULL);
    }
  } else if (g_strcmp0(method, "getPosition") == 0) {
    int x, y;
    gtk_window_get_position(window, &x, &y);

    g_autoptr(FlValue) result = fl_value_new_map();
    fl_value_set_string_take(result, "x", fl_value_new_float(x));
    fl_value_set_string_take(result, "y", fl_value_new_float(y));

    fl_method_call_respond_success(method_call, result, NULL);
  } else if (g_strcmp0(method, "setPosition") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      FlValue *x_value = fl_value_lookup_string(args, "x");
      FlValue *y_value = fl_value_lookup_string(args, "y");

      if (x_value && y_value) {
        int x = (int)fl_value_get_float(x_value);
        int y = (int)fl_value_get_float(y_value);

        gtk_window_move(window, x, y);
        fl_method_call_respond_success(method_call, NULL, NULL);
      } else {
        fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                     "Invalid arguments for setPosition", NULL);
      }
    } else {
      fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                   "Invalid arguments for setPosition", NULL);
    }
  } else if (g_strcmp0(method, "center") == 0) {
    gtk_window_set_position(window, GTK_WIN_POS_CENTER);
    fl_method_call_respond_success(method_call, NULL, NULL);
  } else if (g_strcmp0(method, "show") == 0) {
    gtk_widget_show(GTK_WIDGET(window));
    gtk_window_present(window);
    fl_method_call_respond_success(method_call, NULL, NULL);
  } else if (g_strcmp0(method, "hide") == 0) {
    gtk_widget_hide(GTK_WIDGET(window));
    fl_method_call_respond_success(method_call, NULL, NULL);
  } else if (g_strcmp0(method, "focus") == 0) {
    gtk_window_present(window);
    fl_method_call_respond_success(method_call, NULL, NULL);
  } else if (g_strcmp0(method, "isVisible") == 0) {
    gboolean is_visible = gtk_widget_get_visible(GTK_WIDGET(window));
    g_autoptr(FlValue) result = fl_value_new_bool(is_visible);
    fl_method_call_respond_success(method_call, result, NULL);
  } else if (g_strcmp0(method, "setAlwaysOnTop") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_BOOL) {
      gboolean always_on_top = fl_value_get_bool(args);
      gtk_window_set_keep_above(window, always_on_top);
      fl_method_call_respond_success(method_call, NULL, NULL);
    } else {
      fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                   "Invalid arguments for setAlwaysOnTop",
                                   NULL);
    }
  } else if (g_strcmp0(method, "waitUntilReadyToShow") == 0) {
    if (fl_value_get_type(args) == FL_VALUE_TYPE_MAP) {
      FlValue *width_value = fl_value_lookup_string(args, "width");
      FlValue *height_value = fl_value_lookup_string(args, "height");
      FlValue *center_value = fl_value_lookup_string(args, "center");
      FlValue *always_on_top_value =
          fl_value_lookup_string(args, "alwaysOnTop");

      if (width_value && height_value) {
        int width = (int)fl_value_get_float(width_value);
        int height = (int)fl_value_get_float(height_value);
        gtk_window_resize(window, width, height);
      }

      if (center_value && fl_value_get_bool(center_value)) {
        gtk_window_set_position(window, GTK_WIN_POS_CENTER);
      }

      if (always_on_top_value && fl_value_get_bool(always_on_top_value)) {
        gtk_window_set_keep_above(window, TRUE);
      }

      fl_method_call_respond_success(method_call, NULL, NULL);
    } else {
      fl_method_call_respond_error(method_call, "INVALID_ARGUMENTS",
                                   "Invalid arguments for waitUntilReadyToShow",
                                   NULL);
    }
  } else {
    fl_method_call_respond_not_implemented(method_call, NULL);
  }
}

// Implements GApplication::activate.
static void my_application_activate(GApplication *application) {
  MyApplication *self = MY_APPLICATION(application);
  GtkWindow *window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));

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
    gtk_header_bar_set_title(header_bar, "wox");
    gtk_header_bar_set_show_close_button(header_bar, TRUE);
    gtk_window_set_titlebar(window, GTK_WIDGET(header_bar));
  } else {
    gtk_window_set_title(window, "wox");
  }

  gtk_window_set_default_size(window, 1280, 720);
  gtk_widget_realize(GTK_WIDGET(window));

  gtk_window_set_type_hint(window, GDK_WINDOW_TYPE_HINT_NORMAL);
  gtk_window_set_keep_above(window, TRUE);

  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  FlView *view = fl_view_new(project);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));

  fl_register_plugins(FL_PLUGIN_REGISTRY(view));

  // 设置Linux窗口管理器通道
  setup_linux_window_manager_channel(view, window);

  gtk_widget_show(GTK_WIDGET(window));
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
  *exit_status = 0;

  return TRUE;
}

// Implements GObject::dispose.
static void my_application_dispose(GObject *object) {
  MyApplication *self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);
  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass *klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication *self) {}

MyApplication *my_application_new() {
  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}
