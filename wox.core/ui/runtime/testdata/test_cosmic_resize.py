"""Run in a COSMIC Wayland session with python3-gi and libgtk-layer-shell.so.0.

Set LD_LIBRARY_PATH to an AppImage's usr/lib to test its bundled backend.
Run on outputs at different compositor scales; reported sizes are GTK logical units.
"""

import ctypes
import gi

gi.require_version("Gtk", "3.0")
from gi.repository import Gtk, GLib

layer = ctypes.CDLL("libgtk-layer-shell.so.0", mode=ctypes.RTLD_GLOBAL)
layer.gtk_layer_init_for_window.argtypes = [ctypes.c_void_p]
layer.gtk_layer_set_layer.argtypes = [ctypes.c_void_p, ctypes.c_int]
layer.gtk_layer_set_keyboard_mode.argtypes = [ctypes.c_void_p, ctypes.c_int]
layer.gtk_layer_set_anchor.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int]
assert layer.gtk_layer_is_supported(), "Compositor does not support layer-shell"

window = Gtk.Window(title="Wox live resize regression")
window.set_decorated(False)
window.set_resizable(False)
# PyGObject hashes a wrapper by its underlying GObject address.
pointer = hash(window)
layer.gtk_layer_init_for_window(pointer)
layer.gtk_layer_set_layer(pointer, 2)
layer.gtk_layer_set_keyboard_mode(pointer, 0)  # Do not steal the user's keyboard during this check.
layer.gtk_layer_set_anchor(pointer, 0, True)
layer.gtk_layer_set_anchor(pointer, 2, True)
window.add(Gtk.DrawingArea())
sizes = [(500, 400), (600, 180), (450, 450), (500, 400)]
index = 0
failures = []


def resize(width, height):
    """Use the same GTK size requests as apply_linux_window_size, without hiding."""
    window.set_default_size(width, height)
    window.set_size_request(width, height)
    window.resize(width, height)
    window.queue_resize()
    window.check_resize()
    if window.get_window():
        window.get_window().resize(width, height)


def check():
    global index
    actual = tuple(window.get_size())
    if actual != sizes[index]:
        failures.append((sizes[index], actual))
    print(f"scale={window.get_scale_factor()} expected={sizes[index]} actual={actual}", flush=True)
    index += 1
    if index == len(sizes):
        window.destroy()
        Gtk.main_quit()
        return False
    resize(*sizes[index])
    return True


resize(*sizes[0])
window.show_all()
GLib.timeout_add(700, check)
Gtk.main()
assert not failures, failures
print("PASS: mapped window shrinks and grows without toggling")
