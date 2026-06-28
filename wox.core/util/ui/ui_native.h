#ifndef WOX_UI_NATIVE_H
#define WOX_UI_NATIVE_H

#include <stdint.h>
#include <stdbool.h>

/*
 * Single source of truth for the Go<->C ABI boundary.
 *
 * Both the Go cgo preamble (in ui_windows.go / ui_darwin.go) and the
 * platform source files (ui_windows.cpp / ui_darwin.m) #include this
 * header so struct layouts and function signatures never diverge.
 *
 * To add a new platform (e.g. Linux): #include this header from your
 * ui_linux.c and implement the functions below.
 */

/* ── Shared ABI types ────────────────────────────────────────────── */

/* Mirrors Go DrawCommand (util/ui/commands.go).
   All coordinates are in DIP (logical pixels). */
typedef struct {
    int32_t cmd_type;
    float x, y, w, h;
    float r, g, b, a;
    float radius;
    float strokeWidth;
    const char* text;       /* UTF-8, owned by Go (valid during Render call) */
    int32_t textLen;
    float fontSize;
    const char* fontFamily;  /* UTF-8, owned by Go */
    int32_t fontFamilyLen;
    const uint8_t* imageData; /* PNG bytes, owned by Go */
    int32_t imageLen;
    const char* imageKey;     /* UTF-8 cache key, owned by Go */
    int32_t imageKeyLen;
    float imageWidth, imageHeight;
} CDrawCommand;

/* Controls native window creation. */
typedef struct {
    int32_t width;
    int32_t height;
    float cornerRadius;
    bool frameless;
    bool transparent;
    bool darkMode;
} CWindowConfig;

/* Text measurement result. */
typedef struct {
    float width;
    float height;
} CMeasureResult;

/* Command types (must match Go CommandType constants in commands.go) */
enum {
    CmdClear = 0,
    CmdDrawRect = 1,
    CmdDrawRoundedRect = 2,
    CmdDrawText = 3,
    CmdDrawImage = 4,
    CmdDrawLine = 5,
    CmdPushClip = 6,
    CmdPopClip = 7,
    CmdSetClipRect = 8,
};

/* Event types (must match Go EventType constants in event.go) */
enum {
    EventKeyPress = 0,
    EventKeyRelease = 1,
    EventTextInput = 2,
    EventIMECompose = 3,
    EventClick = 4,
    EventScroll = 5,
    EventFocusLost = 6,
    EventResize = 7,
};

/* Key enum (must match Go Key constants in event.go, KeyUnknown=0).
   Named without prefix to match existing ui_darwin.m usage. The K_ prefix
   on the first entry avoids collision with C99's _Generic keyword; all
   other entries use the same names the Go side expects. */
enum {
    KeyUnknown = 0,
    KeyEscape = 1,
    KeyEnter = 2,
    KeyBackspace = 3,
    KeyTab = 4,
    KeySpace = 5,
    KeyUp = 6,
    KeyDown = 7,
    KeyLeft = 8,
    KeyRight = 9,
    KeyHome = 10,
    KeyEnd = 11,
    KeyPageUp = 12,
    KeyPageDown = 13,
    KeyDelete = 14,
    KeyF1 = 15,
    KeyF2 = 16,
    KeyF3 = 17,
    KeyF4 = 18,
    KeyF5 = 19,
    KeyF6 = 20,
    KeyF7 = 21,
    KeyF8 = 22,
    KeyF9 = 23,
    KeyF10 = 24,
    KeyF11 = 25,
    KeyF12 = 26,
    KeyA = 27,
    KeyB = 28,
    KeyC = 29,
    KeyD = 30,
    KeyE = 31,
    KeyF = 32,
    KeyG = 33,
    KeyH = 34,
    KeyI = 35,
    KeyJ = 36,
    KeyK = 37,
    KeyL = 38,
    KeyM = 39,
    KeyN = 40,
    KeyO = 41,
    KeyP = 42,
    KeyQ = 43,
    KeyR = 44,
    KeyS = 45,
    KeyT = 46,
    KeyU = 47,
    KeyV = 48,
    KeyW = 49,
    KeyX = 50,
    KeyY = 51,
    KeyZ = 52,
    Key0 = 53,
    Key1 = 54,
    Key2 = 55,
    Key3 = 56,
    Key4 = 57,
    Key5 = 58,
    Key6 = 59,
    Key7 = 60,
    Key8 = 61,
    Key9 = 62,
};

/* Modifier flags (must match Go Modifiers constants in event.go) */
enum {
    ModShift   = 1,
    ModControl = 2,
    ModAlt     = 4,
    ModSuper   = 8,
};

/* ── Native function declarations ────────────────────────────────── */
/* Implemented per-platform in ui_windows.cpp / ui_darwin.m / future ui_linux.c */

#ifdef __cplusplus
extern "C" {
#endif

int32_t uiWindowCreate(CWindowConfig config);
void uiWindowDestroy(int32_t windowId);
void uiWindowShow(int32_t windowId);
void uiWindowHide(int32_t windowId);
void uiWindowSetDarkMode(int32_t windowId, bool darkMode);
void uiWindowSetPosition(int32_t windowId, int32_t x, int32_t y);
void uiWindowSetSize(int32_t windowId, int32_t w, int32_t h);
bool uiWindowIsVisible(int32_t windowId);
void uiWindowGetSize(int32_t windowId, int32_t* outW, int32_t* outH);
void uiWindowReleaseMemory(int32_t windowId);
void uiWindowRender(int32_t windowId, const CDrawCommand* commands, int32_t count);
CMeasureResult uiMeasureText(const char* text, int32_t textLen, float fontSize,
                               const char* fontFamily, int32_t fontFamilyLen);

/* Go callback — implemented via //export in the Go side. Called by the
   platform C/Obj-C code for each input event. */
void uiEventCallback(int32_t windowId, int32_t eventType, int32_t key, int32_t mods,
    char* text, int32_t textLen,
    char* composeText, int32_t composeTextLen, int32_t composeCursor,
    float x, float y, float deltaY,
    int32_t width, int32_t height);

#ifdef __cplusplus
}
#endif

#endif /* WOX_UI_NATIVE_H */