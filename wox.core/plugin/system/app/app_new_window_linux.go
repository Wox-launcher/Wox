package app

/*
#cgo pkg-config: gio-unix-2.0
#include <gio/gdesktopappinfo.h>
#include <stdlib.h>

// Desktop actions preserve environment wrappers, sandbox launchers and D-Bus activation.
static const char *wox_app_new_window_action(GDesktopAppInfo *app) {
    const char * const *actions = g_desktop_app_info_list_actions(app);
    for (int i = 0; actions && actions[i]; i++) {
        if (g_ascii_strcasecmp(actions[i], "new-window") == 0 ||
            g_ascii_strcasecmp(actions[i], "NewWindow") == 0 ||
            g_ascii_strcasecmp(actions[i], "new_window") == 0) {
            return actions[i];
        }
    }
    return NULL;
}

// Retain the original desktop entry, changing only its explicit launch command in memory.
static gboolean wox_app_launch_with_arguments(const char *path, const char *arguments, GError **error) {
    GKeyFile *key = g_key_file_new();
    if (!g_key_file_load_from_file(key, path, G_KEY_FILE_NONE, error)) {
        g_key_file_unref(key);
        return FALSE;
    }
    char *exec = g_key_file_get_string(key, "Desktop Entry", "Exec", error);
    if (!exec) {
        g_key_file_unref(key);
        return FALSE;
    }
    char *command = g_strconcat(exec, " ", arguments, NULL);
    g_key_file_set_string(key, "Desktop Entry", "Exec", command);
    // Parameter-based launches must reach Exec even for applications that normally use D-Bus.
    g_key_file_set_boolean(key, "Desktop Entry", "DBusActivatable", FALSE);
    GDesktopAppInfo *app = g_desktop_app_info_new_from_keyfile(key);
    g_free(command);
    g_free(exec);
    g_key_file_unref(key);
    if (!app) {
        g_set_error_literal(error, G_IO_ERROR, G_IO_ERROR_FAILED, "Invalid desktop launch command");
        return FALSE;
    }
    gboolean result = g_app_info_launch(G_APP_INFO(app), NULL, NULL, error);
    g_object_unref(app);
    return result;
}
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"
)

// linuxNewWindowArguments relies on GIO for Exec parsing and rejects wrappers it cannot identify reliably.
func linuxNewWindowArguments(app *C.GDesktopAppInfo) []string {
	info := (*C.GAppInfo)(unsafe.Pointer(app))
	executable := C.g_app_info_get_executable(info)
	command := C.g_app_info_get_commandline(info)
	if executable == nil || command == nil {
		return nil
	}
	args := appNewWindowArguments(filepath.Base(C.GoString(executable)))
	if len(args) == 0 {
		return nil
	}
	var argc C.gint
	var argv **C.gchar
	if C.g_shell_parse_argv(command, &argc, &argv, nil) == 0 {
		return nil
	}
	defer C.g_strfreev(argv)
	existing := make([]string, 0, int(argc))
	for _, arg := range unsafe.Slice(argv, int(argc)) {
		existing = append(existing, C.GoString(arg))
	}
	if len(existing) == 0 || !supportsNewWindowArguments(existing[1:]) {
		return nil
	}
	return args
}

// supportsAppNewWindow probes the standardized desktop entry, independent of the active desktop environment.
func supportsAppNewWindow(info appInfo) bool {
	if !strings.HasSuffix(info.Path, ".desktop") {
		return false
	}
	path := C.CString(info.Path)
	defer C.free(unsafe.Pointer(path))
	app := C.g_desktop_app_info_new_from_filename(path)
	if app == nil {
		return false
	}
	defer C.g_object_unref(C.gpointer(app))
	return C.wox_app_new_window_action(app) != nil || len(linuxNewWindowArguments(app)) > 0
}

// openAppNewWindow prefers an application-declared action before using known executable arguments.
func openAppNewWindow(info appInfo) error {
	path := C.CString(info.Path)
	defer C.free(unsafe.Pointer(path))
	app := C.g_desktop_app_info_new_from_filename(path)
	if app == nil {
		return errNewWindowUnsupported
	}
	defer C.g_object_unref(C.gpointer(app))
	if action := C.wox_app_new_window_action(app); action != nil {
		C.g_desktop_app_info_launch_action(app, action, nil)
		return nil
	}
	args := linuxNewWindowArguments(app)
	if len(args) == 0 {
		return errNewWindowUnsupported
	}
	// These fixed policy arguments contain no quoting or desktop field codes.
	arguments := C.CString(strings.Join(args, " "))
	defer C.free(unsafe.Pointer(arguments))
	var launchError *C.GError
	if C.wox_app_launch_with_arguments(path, arguments, &launchError) == 0 {
		if launchError != nil {
			defer C.g_error_free(launchError)
			return fmt.Errorf("launch desktop new window: %s", C.GoString(launchError.message))
		}
		return fmt.Errorf("launch desktop new window failed")
	}
	return nil
}
