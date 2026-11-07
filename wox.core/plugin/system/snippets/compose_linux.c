//go:build linux && cgo

#include "compose_linux.h"
#include <stdlib.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-compose.h>

struct SnippetCompose {
    struct xkb_context *context;
    struct xkb_compose_table *table;
    struct xkb_compose_state *state;
};

// Use the desktop locale without changing the process-wide C locale.
SnippetCompose *snippetComposeOpen(void) {
    const char *locale = getenv("LC_ALL");
    if (!locale || !*locale) locale = getenv("LC_CTYPE");
    if (!locale || !*locale) locale = getenv("LANG");
    if (!locale || !*locale) locale = "C";
    SnippetCompose *compose = calloc(1, sizeof(*compose));
    if (!compose) return NULL;
    compose->context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    if (compose->context) compose->table = xkb_compose_table_new_from_locale(compose->context, locale, XKB_COMPOSE_COMPILE_NO_FLAGS);
    if (compose->table) compose->state = xkb_compose_state_new(compose->table, XKB_COMPOSE_STATE_NO_FLAGS);
    if (!compose->state) { snippetComposeClose(compose); return NULL; }
    return compose;
}

// A composed or cancelled sequence is not the literal key stream seen by the application.
int snippetComposeFeed(SnippetCompose *compose, uint32_t symbol) {
    if (!compose || !compose->state) return 1;
    if (xkb_compose_state_feed(compose->state, symbol) == XKB_COMPOSE_FEED_IGNORED) return 0;
    enum xkb_compose_status status = xkb_compose_state_get_status(compose->state);
    if (status == XKB_COMPOSE_COMPOSED || status == XKB_COMPOSE_CANCELLED) xkb_compose_state_reset(compose->state);
    return status != XKB_COMPOSE_NOTHING;
}

void snippetComposeClose(SnippetCompose *compose) {
    if (!compose) return;
    if (compose->state) xkb_compose_state_unref(compose->state);
    if (compose->table) xkb_compose_table_unref(compose->table);
    if (compose->context) xkb_context_unref(compose->context);
    free(compose);
}
