#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <glib/gstdio.h>

extern void goMenuItemCallback(int tag);
extern void goTrayMenuItemAdded(int tag, char* label);
extern void goTrayMenuItemActivated(int tag);

typedef struct TrayIcon {
    AppIndicator *indicator;
    GtkMenu *menu;
    GMainContext *context;
    GMainLoop *loop;
    GMainLoop *default_loop;
    GThread *thread;
    GThread *default_thread;
    GMutex init_mutex;
    GCond init_cond;
    gboolean ready;
    gboolean init_success;
    gchar *icon_dir;
    gchar *icon_path;
} TrayIcon;

typedef struct {
    TrayIcon *tray;
    gchar *icon_data;
    gsize icon_data_len;
} SetIconTask;

typedef struct {
    TrayIcon *tray;
    gchar *label;
    int tag;
} AddMenuItemTask;

static void menu_item_callback(GtkMenuItem *item, gpointer user_data) {
    int tag = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(item), "callback_tag"));
    goTrayMenuItemActivated(tag);
    goMenuItemCallback(tag);
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
    app_indicator_set_menu(tray->indicator, tray->menu);
    app_indicator_set_status(tray->indicator, APP_INDICATOR_STATUS_ACTIVE);

    return TRUE;
}

static gpointer tray_thread_main(gpointer user_data) {
    TrayIcon *tray = user_data;

    g_main_context_push_thread_default(tray->context);
    gboolean init_success = setup_tray(tray);

    g_mutex_lock(&tray->init_mutex);
    tray->init_success = init_success;
    tray->ready = TRUE;
    g_cond_signal(&tray->init_cond);
    g_mutex_unlock(&tray->init_mutex);

    if (init_success) {
        g_main_loop_run(tray->loop);
    }

    g_main_context_pop_thread_default(tray->context);
    return NULL;
}

static gpointer default_loop_thread_main(gpointer user_data) {
    GMainLoop *loop = user_data;
    g_main_loop_run(loop);
    return NULL;
}

TrayIcon* create_tray() {
    TrayIcon* tray = g_new0(TrayIcon, 1);
    if (!tray) return NULL;

    g_mutex_init(&tray->init_mutex);
    g_cond_init(&tray->init_cond);
    tray->context = g_main_context_new();
    tray->loop = g_main_loop_new(tray->context, FALSE);
    tray->default_loop = g_main_loop_new(NULL, FALSE);
    tray->default_thread = g_thread_new("wox-tray-default", default_loop_thread_main, tray->default_loop);
    tray->thread = g_thread_new("wox-tray", tray_thread_main, tray);

    g_mutex_lock(&tray->init_mutex);
    while (!tray->ready) {
        g_cond_wait(&tray->init_cond, &tray->init_mutex);
    }
    g_mutex_unlock(&tray->init_mutex);

    if (!tray->init_success) {
        g_thread_join(tray->thread);
        g_main_loop_quit(tray->default_loop);
        g_thread_join(tray->default_thread);
        g_main_loop_unref(tray->loop);
        g_main_loop_unref(tray->default_loop);
        g_main_context_unref(tray->context);
        g_cond_clear(&tray->init_cond);
        g_mutex_clear(&tray->init_mutex);
        g_free(tray);
        return NULL;
    }

    return tray;
}

static gboolean set_tray_icon_on_context(gpointer user_data) {
    SetIconTask *task = user_data;
    TrayIcon *tray = task->tray;

    if (write_icon_file(tray, task->icon_data, task->icon_data_len)) {
        app_indicator_set_icon_theme_path(tray->indicator, tray->icon_dir);
        app_indicator_set_icon_full(tray->indicator, "wox-tray", "Wox");
    }

    g_free(task->icon_data);
    g_free(task);
    return G_SOURCE_REMOVE;
}

void set_tray_icon(TrayIcon* tray, const char* icon_data, gsize icon_data_len) {
    if (!tray || !tray->context || !icon_data || icon_data_len == 0) {
        g_print("Invalid parameters for set_tray_icon\n");
        return;
    }

    SetIconTask *task = g_new0(SetIconTask, 1);
    task->tray = tray;
    task->icon_data_len = icon_data_len;
    task->icon_data = g_malloc(icon_data_len);
    memcpy(task->icon_data, icon_data, icon_data_len);

    g_main_context_invoke_full(tray->context, G_PRIORITY_DEFAULT, set_tray_icon_on_context, task, NULL);
}

static gboolean add_menu_item_on_context(gpointer user_data) {
    AddMenuItemTask *task = user_data;
    TrayIcon *tray = task->tray;

    GtkWidget* menu_item = gtk_menu_item_new_with_label(task->label);
    g_object_set_data(G_OBJECT(menu_item), "callback_tag", GINT_TO_POINTER(task->tag));
    g_signal_connect(G_OBJECT(menu_item), "activate", G_CALLBACK(menu_item_callback), NULL);

    gtk_menu_shell_append(GTK_MENU_SHELL(tray->menu), menu_item);
    gtk_widget_show(menu_item);
    gtk_widget_show_all(GTK_WIDGET(tray->menu));

    // Re-publish after mutation because some StatusNotifier hosts snapshot the DBusMenu when it is set.
    app_indicator_set_menu(tray->indicator, tray->menu);
    app_indicator_set_status(tray->indicator, APP_INDICATOR_STATUS_ACTIVE);
    goTrayMenuItemAdded(task->tag, task->label);

    g_free(task->label);
    g_free(task);
    return G_SOURCE_REMOVE;
}

void add_menu_item(TrayIcon* tray, const char* label, int tag) {
    if (!tray || !tray->context || !label) return;

    AddMenuItemTask *task = g_new0(AddMenuItemTask, 1);
    task->tray = tray;
    task->label = g_strdup(label);
    task->tag = tag;

    g_main_context_invoke_full(tray->context, G_PRIORITY_DEFAULT, add_menu_item_on_context, task, NULL);
}

static gboolean cleanup_tray_on_context(gpointer user_data) {
    TrayIcon *tray = user_data;

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

    g_main_loop_quit(tray->loop);
    return G_SOURCE_REMOVE;
}

void cleanup_tray(TrayIcon* tray) {
    if (!tray) return;

    g_main_context_invoke_full(tray->context, G_PRIORITY_DEFAULT, cleanup_tray_on_context, tray, NULL);

    if (tray->thread) {
        g_thread_join(tray->thread);
    }

    if (tray->default_loop) {
        g_main_loop_quit(tray->default_loop);
    }

    if (tray->default_thread) {
        g_thread_join(tray->default_thread);
    }

    g_main_loop_unref(tray->loop);
    if (tray->default_loop) {
        g_main_loop_unref(tray->default_loop);
    }
    g_main_context_unref(tray->context);
    g_cond_clear(&tray->init_cond);
    g_mutex_clear(&tray->init_mutex);
    g_free(tray);
}
