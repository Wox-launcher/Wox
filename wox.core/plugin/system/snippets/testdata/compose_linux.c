#include "compose_linux.h"
#include <xkbcommon/xkbcommon-keysyms.h>

int main(void) {
    SnippetCompose *compose = snippetComposeOpen();
    if (!compose) return 1;
    int failed = snippetComposeFeed(compose, XKB_KEY_a) != 0 ||
                 snippetComposeFeed(compose, XKB_KEY_dead_acute) != 1 ||
                 snippetComposeFeed(compose, XKB_KEY_a) != 1 ||
                 snippetComposeFeed(compose, XKB_KEY_b) != 0 ||
                 snippetComposeFeed(compose, XKB_KEY_dead_acute) != 1 ||
                 snippetComposeFeed(compose, XKB_KEY_Escape) != 1 ||
                 snippetComposeFeed(compose, XKB_KEY_b) != 0;
    snippetComposeClose(compose);
    return failed ? 2 : 0;
}
