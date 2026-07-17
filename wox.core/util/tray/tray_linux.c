#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>
#include <unistd.h>
#include <fcntl.h>
#include <glib/gstdio.h>

extern void goMenuItemCallback(int tag);
extern void goTrayMenuItemAdded(int tag, char* label);
extern void goTrayMenuItemActivated(int tag);

typedef struct TrayIcon {
    AppIndicator *indicator;
    GtkMenu *menu;
    gboolean published;
    gchar *icon_dir;
    gchar *icon_path;
} TrayIcon;

static void menu_item_callback(GtkMenuItem *item, gpointer user_data) {
    int tag = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(item), "callback_tag"));
    goTrayMenuItemActivated(tag);
    goMenuItemCallback(tag);
}

static void publish_tray_menu(TrayIcon* tray) {
    if (!tray || !tray->indicator || !tray->menu) {
        return;
    }

    gtk_widget_show_all(GTK_WIDGET(tray->menu));
    app_indicator_set_menu(tray->indicator, tray->menu);
    app_indicator_set_status(tray->indicator, APP_INDICATOR_STATUS_ACTIVE);
    tray->published = TRUE;
}

// write_icon_file stores the tray icon as a named icon so AppIndicator can load it through its icon theme path.
static gboolean write_icon_file(TrayIcon* tray, const char* icon_data, gsize icon_data_len) {
    if (!tray->icon_dir) {
        tray->icon_dir = g_build_filename(g_get_user_cache_dir(), "wox", "tray-icons", NULL);
    }

    if (g_mkdir_with_parents(tray->icon_dir, 0700) != 0) {
        g_print("Failed to create tray icon cache directory\n");
        return FALSE;
    }

    if (!tray->icon_path) {
        tray->icon_path = g_build_filename(tray->icon_dir, "wox-tray.png", NULL);
    }

    int fd = g_open(tray->icon_path, O_CREAT | O_WRONLY | O_TRUNC, 0600);
    if (fd == -1) {
        g_print("Failed to create tray icon file\n");
        return FALSE;
    }

    ssize_t written = write(fd, icon_data, icon_data_len);
    if (close(fd) != 0 || written != icon_data_len) {
        g_print("Failed to write complete tray icon file\n");
        return FALSE;
    }

    return TRUE;
}

static gboolean setup_tray(TrayIcon* tray) {
    if (!gtk_init_check(NULL, NULL)) {
        g_print("Failed to initialize GTK for tray icon\n");
        return FALSE;
    }

    tray->menu = GTK_MENU(gtk_menu_new());

    tray->indicator = app_indicator_new(
        "wox-launcher",
        "preferences-system",
        APP_INDICATOR_CATEGORY_APPLICATION_STATUS
    );
    if (!tray->indicator) {
        g_print("Failed to create indicator\n");
        return FALSE;
    }

    app_indicator_set_title(tray->indicator, "Wox");

    return TRUE;
}

TrayIcon* create_tray() {
    TrayIcon* tray = g_new0(TrayIcon, 1);
    if (!tray) return NULL;

    // The embedded Go UI owns GTK's default context and marshals tray calls to that same thread.
    if (!setup_tray(tray)) {
        if (tray->menu) {
            gtk_widget_destroy(GTK_WIDGET(tray->menu));
        }
        if (tray->indicator) {
            g_object_unref(tray->indicator);
        }
        g_free(tray);
        return NULL;
    }

    return tray;
}

void set_tray_icon(TrayIcon* tray, const char* icon_data, gsize icon_data_len) {
    if (!tray || !tray->indicator || !icon_data || icon_data_len == 0) {
        g_print("Invalid parameters for set_tray_icon\n");
        return;
    }

    if (write_icon_file(tray, icon_data, icon_data_len)) {
        app_indicator_set_icon_theme_path(tray->indicator, tray->icon_dir);
        app_indicator_set_icon_full(tray->indicator, "wox-tray", "Wox");
    }
}

void add_menu_item(TrayIcon* tray, const char* label, int tag) {
    if (!tray || !tray->menu || !label) return;

    GtkWidget* menu_item = gtk_menu_item_new_with_label(label);
    g_object_set_data(G_OBJECT(menu_item), "callback_tag", GINT_TO_POINTER(tag));
    g_signal_connect(G_OBJECT(menu_item), "activate", G_CALLBACK(menu_item_callback), NULL);

    gtk_menu_shell_append(GTK_MENU_SHELL(tray->menu), menu_item);
    gtk_widget_show(menu_item);

    if (tray->published) {
        publish_tray_menu(tray);
    }
    goTrayMenuItemAdded(tag, (char*)label);
}

void show_tray(TrayIcon* tray) {
    publish_tray_menu(tray);
}

void cleanup_tray(TrayIcon* tray) {
    if (!tray) return;

    if (tray->indicator) {
        app_indicator_set_status(tray->indicator, APP_INDICATOR_STATUS_PASSIVE);
    }

    if (tray->menu) {
        gtk_widget_destroy(GTK_WIDGET(tray->menu));
        tray->menu = NULL;
    }

    if (tray->indicator) {
        g_object_unref(tray->indicator);
        tray->indicator = NULL;
    }

    if (tray->icon_path) {
        unlink(tray->icon_path);
        g_free(tray->icon_path);
        tray->icon_path = NULL;
    }

    if (tray->icon_dir) {
        g_free(tray->icon_dir);
        tray->icon_dir = NULL;
    }

    g_free(tray);
}
