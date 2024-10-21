#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <cairo.h>
#include <cairo-xlib.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define WINDOW_WIDTH 380
#define WINDOW_HEIGHT 80

typedef struct {
    Display *display;
    Window window;
    GC gc;
    XFontSet fontset;
    char message[1024];
    int mouseInside;
    XEvent event;
} NotificationWindow;

void draw_notification(NotificationWindow *nw) {
    cairo_surface_t *surface = cairo_xlib_surface_create(nw->display, nw->window,
                                                         DefaultVisual(nw->display, DefaultScreen(nw->display)),
                                                         WINDOW_WIDTH, WINDOW_HEIGHT);
    cairo_t *cr = cairo_create(surface);

    // 绘制圆角矩形背景
    cairo_set_source_rgba(cr, 0.25, 0.25, 0.25, 0.8);
    cairo_set_line_width(cr, 1);
    cairo_move_to(cr, 20, 0);
    cairo_line_to(cr, WINDOW_WIDTH - 20, 0);
    cairo_curve_to(cr, WINDOW_WIDTH, 0, WINDOW_WIDTH, 0, WINDOW_WIDTH, 20);
    cairo_line_to(cr, WINDOW_WIDTH, WINDOW_HEIGHT - 20);
    cairo_curve_to(cr, WINDOW_WIDTH, WINDOW_HEIGHT, WINDOW_WIDTH, WINDOW_HEIGHT, WINDOW_WIDTH - 20, WINDOW_HEIGHT);
    cairo_line_to(cr, 20, WINDOW_HEIGHT);
    cairo_curve_to(cr, 0, WINDOW_HEIGHT, 0, WINDOW_HEIGHT, 0, WINDOW_HEIGHT - 20);
    cairo_line_to(cr, 0, 20);
    cairo_curve_to(cr, 0, 0, 0, 0, 20, 0);
    cairo_close_path(cr);
    cairo_fill(cr);

    // 绘制消息文本
    cairo_select_font_face(cr, "Sans", CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_NORMAL);
    cairo_set_font_size(cr, 14);
    cairo_set_source_rgb(cr, 1, 1, 1);
    cairo_move_to(cr, 20, 30);
    cairo_show_text(cr, nw->message);

    // 如果鼠标在窗口内，绘制关闭按钮
    if (nw->mouseInside) {
        cairo_set_source_rgb(cr, 1, 1, 1);
        cairo_set_line_width(cr, 2);
        cairo_arc(cr, WINDOW_WIDTH - 20, 20, 10, 0, 2 * M_PI);
        cairo_stroke(cr);
        cairo_move_to(cr, WINDOW_WIDTH - 25, 15);
        cairo_line_to(cr, WINDOW_WIDTH - 15, 25);
        cairo_move_to(cr, WINDOW_WIDTH - 25, 25);
        cairo_line_to(cr, WINDOW_WIDTH - 15, 15);
        cairo_stroke(cr);
    }

    cairo_destroy(cr);
    cairo_surface_destroy(surface);
}

void showNotification(const char *message) {
    NotificationWindow nw;
    memset(&nw, 0, sizeof(NotificationWindow));

    nw.display = XOpenDisplay(NULL);
    if (nw.display == NULL) {
        fprintf(stderr, "Cannot open display\n");
        return;
    }

    int screen = DefaultScreen(nw.display);
    int screenWidth = DisplayWidth(nw.display, screen);
    int screenHeight = DisplayHeight(nw.display, screen);

    int x = (screenWidth - WINDOW_WIDTH) / 2;
    int y = (int)(screenHeight * 0.2) - WINDOW_HEIGHT / 2;

    nw.window = XCreateSimpleWindow(nw.display, RootWindow(nw.display, screen),
                                    x, y, WINDOW_WIDTH, WINDOW_HEIGHT, 1,
                                    BlackPixel(nw.display, screen), WhitePixel(nw.display, screen));

    XSetWindowAttributes attributes;
    attributes.override_redirect = True;
    XChangeWindowAttributes(nw.display, nw.window, CWOverrideRedirect, &attributes);

    Atom wm_delete_window = XInternAtom(nw.display, "WM_DELETE_WINDOW", False);
    XSetWMProtocols(nw.display, nw.window, &wm_delete_window, 1);

    XSelectInput(nw.display, nw.window, ExposureMask | ButtonPressMask | PointerMotionMask | LeaveWindowMask);

    nw.gc = XCreateGC(nw.display, nw.window, 0, NULL);

    strncpy(nw.message, message, sizeof(nw.message) - 1);
    nw.message[sizeof(nw.message) - 1] = '\0';

    XMapWindow(nw.display, nw.window);

    int closeTimer = 3000000; // 3 seconds in microseconds
    while (1) {
        if (XPending(nw.display)) {
            XNextEvent(nw.display, &nw.event);
            switch (nw.event.type) {
                case Expose:
                    draw_notification(&nw);
                    break;
                case MotionNotify:
                    if (!nw.mouseInside) {
                        nw.mouseInside = 1;
                        draw_notification(&nw);
                    }
                    closeTimer = 3000000; // Reset timer when mouse enters
                    break;
                case LeaveNotify:
                    nw.mouseInside = 0;
                    draw_notification(&nw);
                    break;
                case ButtonPress:
                    if (nw.event.xbutton.x >= WINDOW_WIDTH - 30 && nw.event.xbutton.x <= WINDOW_WIDTH - 10 &&
                        nw.event.xbutton.y >= 10 && nw.event.xbutton.y <= 30) {
                        goto cleanup;
                    }
                    break;
                case ClientMessage:
                    if ((Atom)nw.event.xclient.data.l[0] == wm_delete_window) {
                        goto cleanup;
                    }
                    break;
            }
        } else {
            usleep(10000); // Sleep for 10ms
            closeTimer -= 10000;
            if (closeTimer <= 0 && !nw.mouseInside) {
                break;
            }
        }
    }

cleanup:
    XDestroyWindow(nw.display, nw.window);
    XFreeGC(nw.display, nw.gc);
    XCloseDisplay(nw.display);
}
