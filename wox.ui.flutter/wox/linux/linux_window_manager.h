#ifndef LINUX_WINDOW_MANAGER_H_
#define LINUX_WINDOW_MANAGER_H_

#include <flutter_linux/flutter_linux.h>
#include <gtk/gtk.h>

G_BEGIN_DECLS

// 设置Linux窗口管理器的方法通道
void setup_linux_window_manager_channel(FlView *view, GtkWindow *window);

G_END_DECLS

#endif // LINUX_WINDOW_MANAGER_H_