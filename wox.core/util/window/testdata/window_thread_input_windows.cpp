#include <windows.h>
#include <cassert>
#include <initializer_list>
#include <vector>

struct Call {
  DWORD first;
  DWORD second;
  BOOL attach;
};

static std::vector<Call> calls;
static DWORD foreground_thread;
static DWORD target_thread;
static DWORD rejected_thread;

static HWND test_foreground() { return foreground_thread ? reinterpret_cast<HWND>(20) : nullptr; }
static DWORD test_current_thread() { return 10; }
static DWORD test_window_thread(HWND window, LPDWORD) { return window == reinterpret_cast<HWND>(20) ? foreground_thread : target_thread; }
static BOOL test_attach(DWORD first, DWORD second, BOOL attach) {
  calls.push_back({first, second, attach});
  return first != rejected_thread && second != rejected_thread;
}

#define GetForegroundWindow test_foreground
#define GetCurrentThreadId test_current_thread
#define GetWindowThreadProcessId test_window_thread
#define AttachThreadInput test_attach
#include "../window_thread_input_windows.c"

// Compare the full ordered Win32 call sequence, including attachment versus cleanup.
static void expect_calls(std::initializer_list<Call> expected) {
  assert(calls.size() == expected.size());
  size_t index = 0;
  for (const auto &call : expected) {
    const auto &actual = calls[index++];
    assert(actual.first == call.first && actual.second == call.second && actual.attach == call.attach);
  }
  calls.clear();
}

// Exercise both activation policies against fake queues, without touching desktop focus.
int main() {
  struct Case {
    WoxWindowThreadInputDirection direction;
    Call foreground_attach;
    Call target_attach;
    Call target_detach;
    Call foreground_detach;
  };
  const Case cases[] = {
      {WOX_THREAD_INPUT_CURRENT_TO_QUEUES, {10, 20, TRUE}, {10, 30, TRUE}, {10, 30, FALSE}, {10, 20, FALSE}},
      {WOX_THREAD_INPUT_QUEUES_TO_CURRENT, {20, 10, TRUE}, {30, 10, TRUE}, {30, 10, FALSE}, {20, 10, FALSE}},
  };
  for (const auto &test : cases) {
    foreground_thread = 20;
    target_thread = 30;
    rejected_thread = 0;
    auto attachment = wox_window_attach_thread_input(30, test.direction);
    wox_window_detach_thread_input(&attachment);
    wox_window_detach_thread_input(&attachment);
    expect_calls({test.foreground_attach, test.target_attach, test.target_detach, test.foreground_detach});

    attachment = wox_window_attach_thread_input(0, test.direction);
    wox_window_detach_thread_input(&attachment);
    expect_calls({test.foreground_attach, test.foreground_detach});

    target_thread = 20;
    attachment = wox_window_attach_thread_input(30, test.direction);
    wox_window_detach_thread_input(&attachment);
    expect_calls({test.foreground_attach, test.foreground_detach});

    foreground_thread = target_thread = 10;
    attachment = wox_window_attach_thread_input(30, test.direction);
    wox_window_detach_thread_input(&attachment);
    expect_calls({});

    foreground_thread = 20;
    target_thread = 30;
    rejected_thread = 20;
    attachment = wox_window_attach_thread_input(30, test.direction);
    wox_window_detach_thread_input(&attachment);
    wox_window_detach_thread_input(&attachment);
    expect_calls({test.foreground_attach, test.target_attach, test.target_detach});

    rejected_thread = 30;
    attachment = wox_window_attach_thread_input(30, test.direction);
    wox_window_detach_thread_input(&attachment);
    expect_calls({test.foreground_attach, test.target_attach, test.foreground_detach});

    foreground_thread = target_thread = 0;
    attachment = wox_window_attach_thread_input(0, test.direction);
    wox_window_detach_thread_input(&attachment);
    expect_calls({});
  }
  wox_window_detach_thread_input(nullptr);
  return 0;
}
