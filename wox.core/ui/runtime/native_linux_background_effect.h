#ifndef WOX_UI_GO_NATIVE_LINUX_BACKGROUND_EFFECT_H
#define WOX_UI_GO_NATIVE_LINUX_BACKGROUND_EFFECT_H

#include <gtk/gtk.h>
#include <stdint.h>

// wox_linux_background_effect_probe binds ext-background-effect-v1 when the
// compositor advertises it. Call once after gtk_init.
void wox_linux_background_effect_probe(GdkDisplay *display);
void *wox_linux_background_effect_attach(GdkWindow *gdk_window);
void *wox_linux_background_effect_surface(GdkWindow *gdk_window);
void wox_linux_background_effect_update(void *effect, int width, int height, float radius);
void wox_linux_background_effect_destroy(void *effect);
cairo_region_t *wox_linux_background_blur_region(int width, int height, float radius);

#endif
