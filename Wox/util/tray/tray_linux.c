#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>

extern void goMenuItemCallback(int tag);

typedef struct {
    AppIndicator *indicator;
    GtkMenu *menu;
    GMainLoop *loop;
} TrayIcon;

static void menu_item_callback(GtkMenuItem *item, gpointer user_data) {
    int tag = GPOINTER_TO_INT(g_object_get_data(G_OBJECT(item), "callback_tag"));
    goMenuItemCallback(tag);
}

static gchar* save_icon_to_temp_file(const char* icon_data, gsize icon_data_len) {
    GError *error = NULL;
    gchar *temp_path = g_build_filename(g_get_tmp_dir(), "wox-tray-XXXXXX.png", NULL);
    int fd = g_mkstemp(temp_path);
    
    if (fd == -1) {
        g_print("Failed to create temp file\n");
        g_free(temp_path);
        return NULL;
    }
    
    // Write icon data to temp file
    ssize_t written = write(fd, icon_data, icon_data_len);
    close(fd);
    
    if (written != icon_data_len) {
        g_print("Failed to write complete icon data\n");
        unlink(temp_path);  // Delete the file
        g_free(temp_path);
        return NULL;
    }
    
    g_print("Icon saved to: %s\n", temp_path);
    return temp_path;
}

TrayIcon* create_tray() {
    TrayIcon* tray = g_new0(TrayIcon, 1);
    if (!tray) return NULL;
    
    // Initialize GTK if needed
    if (!gtk_init_check(NULL, NULL)) {
        g_print("Failed to initialize GTK\n");
        g_free(tray);
        return NULL;
    }
    
    // Create menu
    tray->menu = GTK_MENU(gtk_menu_new());
    gtk_widget_show_all(GTK_WIDGET(tray->menu));
    
    // Create indicator
    tray->indicator = app_indicator_new(
        "wox-launcher",           // id
        "preferences-system",     // default icon name (using a standard icon)
        APP_INDICATOR_CATEGORY_APPLICATION_STATUS
    );
    
    if (!tray->indicator) {
        g_print("Failed to create indicator\n");
        g_free(tray);
        return NULL;
    }
    
    // Set menu
    app_indicator_set_menu(tray->indicator, GTK_MENU(tray->menu));
    
    // Set status to active (visible)
    app_indicator_set_status(tray->indicator, APP_INDICATOR_STATUS_ACTIVE);
    
    // Create main loop
    tray->loop = g_main_loop_new(NULL, FALSE);
    
    // Start the GTK main loop in a separate thread
    g_thread_new("gtk-main", (GThreadFunc)g_main_loop_run, tray->loop);
    
    return tray;
}

void set_tray_icon(TrayIcon* tray, const char* icon_data, gsize icon_data_len) {
    if (!tray || !tray->indicator || !icon_data || icon_data_len == 0) {
        g_print("Invalid parameters for set_tray_icon\n");
        return;
    }
    
    gchar* icon_path = save_icon_to_temp_file(icon_data, icon_data_len);
    if (icon_path) {
        app_indicator_set_icon_full(tray->indicator, icon_path, "Wox");
        g_free(icon_path);
    }
}

void add_menu_item(TrayIcon* tray, const char* label, int tag) {
    if (!tray || !tray->menu || !label) return;
    
    GtkWidget* menuItem = gtk_menu_item_new_with_label(label);
    
    g_object_set_data(G_OBJECT(menuItem), 
        "callback_tag", 
        GINT_TO_POINTER(tag));
        
    g_signal_connect(G_OBJECT(menuItem),
        "activate",
        G_CALLBACK(menu_item_callback),
        NULL);
        
    gtk_menu_shell_append(GTK_MENU_SHELL(tray->menu), menuItem);
    gtk_widget_show(menuItem);
}

void cleanup_tray(TrayIcon* tray) {
    if (!tray) return;
    
    if (tray->loop) {
        g_main_loop_quit(tray->loop);
        g_main_loop_unref(tray->loop);
    }
    
    if (tray->menu) {
        gtk_widget_destroy(GTK_WIDGET(tray->menu));
    }
    
    if (tray->indicator) {
        g_object_unref(tray->indicator);
    }
    
    g_free(tray);
    g_print("Tray cleaned up\n");
}