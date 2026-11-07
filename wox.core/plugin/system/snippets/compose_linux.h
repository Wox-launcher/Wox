#pragma once

#include <stdint.h>

typedef struct SnippetCompose SnippetCompose;

SnippetCompose *snippetComposeOpen(void);
int snippetComposeFeed(SnippetCompose *compose, uint32_t symbol);
void snippetComposeClose(SnippetCompose *compose);
