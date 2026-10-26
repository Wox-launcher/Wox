"""Run with python3 on Linux; needs cc and glib-2.0 development files, no display.

Exercise the actual native startup/dispatch code with GTK replaced by a GLib loop.
"""

from pathlib import Path
import shlex
import subprocess
import tempfile


source = (Path(__file__).resolve().parents[1] / "native_linux.c").read_text()
dispatch = source[source.index("typedef void (*WoxMainFunction)"):source.index("static void premultiplied_color")]
startup = source[source.index("typedef struct {\n  uintptr_t context;\n  int32_t result;"):source.index("static void apply_linux_rgba_visual")]
harness = r"""
#include <glib.h>
#include <pthread.h>
#include <stdbool.h>
#include <stdint.h>
#include <assert.h>
#include <stdio.h>

static GMainLoop *loop;
static pthread_t wox_linux_main_thread, worker;
static gint wox_linux_runtime_running, wox_linux_loop_active, wox_linux_window_count;
static gint worker_started, callback_count;
static int scenario, dispatch_result;
static guint redraw_source, dispatch_deadline;
static int redraw_count;
#define gtk_main() g_main_loop_run(loop)
#define gtk_main_quit() g_main_loop_quit(loop)
#define gtk_init_check(a, b) true
#define wox_linux_desktop_init_threads() true
#define apply_linux_app_identity() ((void)0)
#define apply_linux_app_icon() ((void)0)
#define wox_linux_background_effect_probe(display) ((void)0)
static int32_t woxGoLinuxStart(uintptr_t context);
static void woxGoLinuxCall(uintptr_t context);
""" + dispatch + startup + r"""
static gboolean quit_failed_dispatch(gpointer unused) {
  (void)unused;
  gtk_main_quit();
  return G_SOURCE_REMOVE;
}

static void *background_load(void *unused) {
  (void)unused;
  g_atomic_int_set(&worker_started, 1);
  dispatch_result = wox_linux_call(42);
  if (dispatch_result != 0) g_idle_add(quit_failed_dispatch, NULL);
  return NULL;
}

static void woxGoLinuxCall(uintptr_t context) {
  assert(context == 42 && is_main_thread());
  callback_count++;
  if (scenario >= 4) {
    assert(redraw_count > 0);
    g_source_remove(redraw_source);
    g_source_remove(dispatch_deadline);
  }
  gtk_main_quit();
}

// Fail within the fixture's existing process timeout instead of hanging a sync caller.
static gboolean fail_starved_dispatch(gpointer unused) {
  (void)unused;
  g_error("UI dispatch starved by continuous redraw");
  return G_SOURCE_REMOVE;
}

// Keep a redraw-priority source ready until a queued UI call gets a turn.
static gboolean continuous_redraw(gpointer unused) {
  (void)unused;
  redraw_count++;
  wox_linux_note_stage("gtk-after-gl-render");
  if (redraw_count == 1) {
    if (scenario == 4) {
      assert(pthread_create(&worker, NULL, background_load, NULL) == 0);
    } else {
      int before = callback_count;
      assert(wox_linux_post(42) == 0);
      assert(callback_count == before);
    }
  }
  return G_SOURCE_CONTINUE;
}

static int32_t woxGoLinuxStart(uintptr_t context) {
  assert(context == 1 && is_main_thread());
  if (scenario == 1) return -1;
  if (scenario == 2) return 0;
  if (scenario >= 4) {
    redraw_count = 0;
    // GDK_PRIORITY_REDRAW is HIGH_IDLE + 20; GTK is replaced by GLib in this fixture.
    redraw_source = g_idle_add_full(G_PRIORITY_HIGH_IDLE + 20, continuous_redraw, NULL, NULL);
    dispatch_deadline = g_timeout_add(1000, fail_starved_dispatch, NULL);
    return 0;
  }
  if (scenario == 3) {
    int before = callback_count;
    assert(wox_linux_post(42) == 0);
    assert(callback_count == before);
    return 0;
  }
  assert(pthread_create(&worker, NULL, background_load, NULL) == 0);
  while (!g_atomic_int_get(&worker_started)) g_thread_yield();
  // Force the catalog worker to dispatch while the startup callback is still running.
  char stage[160];
  int64_t stage_age, backlog_age;
  int32_t waiters;
  uint64_t completed;
  // Wait until the worker has queued its synchronous call, then simulate changing
  // render stages. Stage freshness must not erase the pending-call evidence.
  do {
    wox_linux_ui_stage_snapshot(stage, sizeof(stage), &stage_age, &waiters, &backlog_age, &completed);
    g_thread_yield();
  } while (waiters == 0);
  g_usleep(100000);
  wox_linux_note_stage("gl-render");
  wox_linux_ui_stage_snapshot(stage, sizeof(stage), &stage_age, &waiters, &backlog_age, &completed);
  assert(waiters == 1 && completed == 0);
  assert(backlog_age >= 100000 && backlog_age > stage_age);
  return 0;
}

int main(void) {
  for (scenario = 0; scenario < 6; scenario++) {
    loop = g_main_loop_new(NULL, FALSE);
    wox_linux_window_count = scenario == 2 ? 0 : 1;
    assert(wox_linux_run(1) == (scenario == 1 ? -1 : 0));
    if (scenario == 0) {
      pthread_join(worker, NULL);
      assert(dispatch_result == 0 && callback_count == 1);
      int64_t backlog_age;
      int32_t waiters;
      uint64_t completed;
      wox_linux_ui_stage_snapshot(NULL, 0, NULL, &waiters, &backlog_age, &completed);
      assert(waiters == 0 && backlog_age == 0 && completed == 1);
    }
    if (scenario == 3) assert(callback_count == 2);
    if (scenario == 4) {
      pthread_join(worker, NULL);
      assert(dispatch_result == 0 && callback_count == 3);
    }
    if (scenario == 5) assert(callback_count == 4);
    assert(!wox_linux_runtime_running && !wox_linux_loop_active);
    assert(wox_linux_post(42) == -1);
    g_main_loop_unref(loop);
  }
  return 0;
}
"""

with tempfile.TemporaryDirectory(prefix="wox-startup-test-") as directory:
    test = Path(directory) / "startup.c"
    binary = Path(directory) / "startup"
    test.write_text(harness)
    flags = shlex.split(subprocess.check_output(["pkg-config", "--cflags", "--libs", "glib-2.0"], text=True))
    subprocess.run(["cc", "-std=c11", "-Wall", "-Wextra", "-Werror", str(test), "-o", str(binary), "-pthread", *flags], check=True)
    subprocess.run([str(binary)], check=True, timeout=10)
print("PASS: startup background dispatch, backlog diagnostics across stage changes, deferred UI dispatch, sync/post dispatch during continuous redraw, startup failure, and no-window shutdown")
