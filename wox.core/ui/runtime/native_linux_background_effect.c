//go:build linux

// ext-background-effect-v1 is compositor-capability based, not desktop-specific.
// Plasma 6.7+, GNOME 51, Niri, and COSMIC advertise the same protocol.

#include "native_linux_background_effect.h"
#include "native_linux.h"

#include <stdbool.h>
#include <string.h>

#ifdef GDK_WINDOWING_WAYLAND
#include <gdk/gdkwayland.h>
#include "ext-background-effect-v1-client-protocol.h"
#endif

#ifdef GDK_WINDOWING_WAYLAND
static struct ext_background_effect_manager_v1 *wox_background_effect_manager;
static struct wl_compositor *wox_background_effect_compositor;
static uint32_t wox_background_effect_capabilities;
static bool wox_background_effect_probed;

static void on_background_effect_capabilities(void *data, struct ext_background_effect_manager_v1 *manager, uint32_t flags) {
  (void)data;
  (void)manager;
  wox_background_effect_capabilities = flags;
}

static const struct ext_background_effect_manager_v1_listener wox_background_effect_listener = {
    .capabilities = on_background_effect_capabilities,
};

static void wox_background_effect_registry_global(void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
  (void)data;
  (void)version;
  if (strcmp(interface, ext_background_effect_manager_v1_interface.name) == 0) {
    wox_background_effect_manager = wl_registry_bind(registry, name, &ext_background_effect_manager_v1_interface, 1);
  }
}

static void wox_background_effect_registry_remove(void *data, struct wl_registry *registry, uint32_t name) {
  (void)data;
  (void)registry;
  (void)name;
}

static const struct wl_registry_listener wox_background_effect_registry_listener = {
    .global = wox_background_effect_registry_global,
    .global_remove = wox_background_effect_registry_remove,
};
#endif

// wox_linux_background_blur_available reports whether the compositor both
// advertises ext-background-effect-v1 and currently exposes the blur capability.
int32_t wox_linux_background_blur_available(void) {
#ifdef GDK_WINDOWING_WAYLAND
  return wox_background_effect_manager != NULL && (wox_background_effect_capabilities & EXT_BACKGROUND_EFFECT_MANAGER_V1_CAPABILITY_BLUR) != 0 ? 1 : 0;
#else
  return 0;
#endif
}

void wox_linux_background_effect_probe(GdkDisplay *display) {
#ifdef GDK_WINDOWING_WAYLAND
  if (wox_background_effect_probed || display == NULL || !GDK_IS_WAYLAND_DISPLAY(display)) {
    return;
  }
  wox_background_effect_probed = true;
  struct wl_display *wl_display = gdk_wayland_display_get_wl_display(display);
  wox_background_effect_compositor = gdk_wayland_display_get_wl_compositor(display);
  if (wl_display == NULL || wox_background_effect_compositor == NULL) {
    return;
  }
  struct wl_registry *registry = wl_display_get_registry(wl_display);
  if (registry == NULL) {
    return;
  }
  wl_registry_add_listener(registry, &wox_background_effect_registry_listener, NULL);
  wl_display_roundtrip(wl_display);
  if (wox_background_effect_manager == NULL) {
    return;
  }
  ext_background_effect_manager_v1_add_listener(wox_background_effect_manager, &wox_background_effect_listener, NULL);
  wl_display_roundtrip(wl_display);
#else
  (void)display;
#endif
}

// wox_linux_background_effect_surface returns the current Wayland surface for a GdkWindow.
void *wox_linux_background_effect_surface(GdkWindow *gdk_window) {
#ifdef GDK_WINDOWING_WAYLAND
  if (gdk_window == NULL || !GDK_IS_WAYLAND_WINDOW(gdk_window)) {
    return NULL;
  }
  return gdk_wayland_window_get_wl_surface(gdk_window);
#else
  (void)gdk_window;
  return NULL;
#endif
}

// wox_linux_background_effect_attach creates a per-surface background-effect object.
void *wox_linux_background_effect_attach(GdkWindow *gdk_window) {
#ifdef GDK_WINDOWING_WAYLAND
  if (!wox_linux_background_blur_available()) {
    return NULL;
  }
  struct wl_surface *surface = wox_linux_background_effect_surface(gdk_window);
  if (surface == NULL) {
    return NULL;
  }
  return ext_background_effect_manager_v1_get_background_effect(wox_background_effect_manager, surface);
#else
  (void)gdk_window;
  return NULL;
#endif
}

// wox_linux_background_effect_update sets an explicit surface-local blur region.
// The protocol only accepts axis-aligned rectangles, so the region is the full
// window. Painted window chrome on this path stays square to match.
void wox_linux_background_effect_update(void *effect, int width, int height) {
#ifdef GDK_WINDOWING_WAYLAND
  if (effect == NULL || wox_background_effect_compositor == NULL) {
    return;
  }
  struct ext_background_effect_surface_v1 *surface = effect;
  if (width <= 0 || height <= 0) {
    ext_background_effect_surface_v1_set_blur_region(surface, NULL);
    return;
  }
  struct wl_region *region = wl_compositor_create_region(wox_background_effect_compositor);
  if (region == NULL) {
    return;
  }
  wl_region_add(region, 0, 0, width, height);
  ext_background_effect_surface_v1_set_blur_region(surface, region);
  wl_region_destroy(region);
#else
  (void)effect;
  (void)width;
  (void)height;
#endif
}

// wox_linux_background_effect_destroy releases the per-surface protocol object.
void wox_linux_background_effect_destroy(void *effect) {
#ifdef GDK_WINDOWING_WAYLAND
  if (effect != NULL) {
    ext_background_effect_surface_v1_destroy(effect);
  }
#else
  (void)effect;
#endif
}

// wox_linux_test_window_requests_background_blur encodes the screenshot opt-out.
int32_t wox_linux_test_window_requests_background_blur(int32_t screenshot, int32_t blur_available) {
  return screenshot == 0 && blur_available != 0 ? 1 : 0;
}
