#include "linux_window_manager.h"

#include <flutter_linux/flutter_linux.h>
#include <gtk/gtk.h>

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

void setup_linux_window_manager_channel(FlView *view, GtkWindow *window) {
  FlEngine *engine = fl_view_get_engine(view);
  g_autoptr(FlStandardMethodCodec) codec = fl_standard_method_codec_new();
  g_autoptr(FlMethodChannel) channel =
      fl_method_channel_new(fl_engine_get_binary_messenger(engine),
                            "com.wox.window_manager", FL_METHOD_CODEC(codec));
  fl_method_channel_set_method_call_handler(channel, method_call_cb, window,
                                            NULL);
}