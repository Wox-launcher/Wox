# WebSearch Plugin

WebSearch opens search URLs from Wox. It can work as a fallback result for normal text or through explicit engine keywords.

## Quick Start

```text
Wox Launcher
g Wox Launcher
```

The default configuration includes Google with the `g` keyword. Add more engines in plugin settings.

![WebSearch plugin result list](/images/system-plugin-websearch.png)

## Engine Settings

| Field | Use |
| --- | --- |
| Keyword | Shortcut typed before the query, such as `g` |
| Title | Result label shown in Wox |
| URL(s) | Search URL templates |
| Browser | Browser that opens this search |
| Incognito | Open the search in a private/incognito window when the browser supports it. Off by default. |
| Enabled | Whether the engine appears |
| Default | Whether the engine is used for fallback searches |

## URL Variables

| Variable | Value |
| --- | --- |
| `{wox:parameter?name=query}` | Named input parameter |
| `{wox:parameter?name=query&case=lower}` | Lowercase parameter value |
| `{wox:selected_text}` | Text selected before Wox opens |
| `{wox:clipboard_text}` | Clipboard text captured before Wox opens |

Example URL:

```text
https://www.google.com/search?q={wox:parameter?name=query}
```

If an engine has multiple URLs, Wox opens each URL in order.

Existing `{query}`, `{lower_query}` and `{upper_query}` placeholders in URLs
and titles migrate automatically to `{wox:parameter?name=query}` and
`{wox:parameter?name=query&case=lower}` / `case=upper`.

For multiple inputs, use a URL such as
`https://example.com/search?text={wox:parameter?name=text}&language={wox:parameter?name=language}`.
Typing the keyword followed by a space enters that search's scope and shows its
input slots. Tab / Shift+Tab moves between slots; each value can contain spaces.
Parameters are ordered by first appearance across the URL list, with repeated
names sharing one value. Names can use letters in any language, digits, underscores and spaces, and
cannot start with a digit or space. Titles may reference parameters declared in URLs.
In the Title and URLs editors, type `{` or use the `{}` button to insert
variables; complete `{wox:...}` placeholders appear as chips. Saving is
rejected when the title names a parameter that does not appear in the URLs.

Single-input searches still accept the entire text after the keyword. Pasting
a plain multi-input query offers a fill-parameters action that preserves the
text in the first slot; Wox does not guess boundaries.
Variables are substituted once and URL-encoded for the path, query or fragment.
They cannot replace the URL scheme or host. Missing environment text produces
an explanation instead of opening an incomplete search.

## Selected Text

When you trigger Wox on selected text, WebSearch can show fallback engines for that selection. This is useful for quickly searching an error message, symbol, or phrase from another app.

Fallback and selected-text search require exactly one distinct input parameter.
Environment variables do not count as inputs; searches with zero or multiple
inputs are excluded even if fallback is enabled.
