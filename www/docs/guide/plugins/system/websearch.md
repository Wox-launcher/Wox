# WebSearch Plugin

WebSearch opens search URLs from Wox. It can work as a fallback result for normal text or through explicit engine keywords.

![Filling named web search parameters with query hints](/images/guide/plugin_websearch.jpg)

## Quick Start

```text
Wox Launcher
g Wox Launcher
```

The default configuration includes Google with the `g` keyword. Add more engines in plugin settings.

## Engine Settings

| Field | Use |
| --- | --- |
| Keyword | Shortcut typed before the query, such as `g` |
| Title | Result label shown in Wox |
| URL(s) | Search URL templates |
| Browser | Browser that opens this search |
| Incognito | Open the search in a private/incognito window when the browser supports it. Off by default. Grouped under Advanced. |
| Disabled | Check to hide the engine. New rows stay on. |
| Default | Whether the engine is used for fallback searches. Grouped under Advanced. |
| Preview Width | Optional launcher width in pixels for **Open in WebView**. Grouped under WebView. |
| Preview Height | Optional launcher height in pixels for **Open in WebView**. Grouped under WebView. |
| Inject CSS | Optional CSS injected into that search's in-launcher WebView. Grouped under WebView. |
| Open in WebView by default | Display a single completed search directly inside Wox. Grouped under WebView. |
| User Agent | Desktop/mobile browser preset for the embedded page. Auto uses the native engine default. |
| Run in background | Keep the embedded page loaded after hiding Wox. Off by default. |

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

Enable **Open in WebView by default** to display a single search result directly inside Wox. The default **Focus Web Page** action uses Enter to move keyboard focus to the displayed page without reloading it. Otherwise, Enter opens the browser and `Ctrl+Enter` (`Cmd+Enter` on macOS) opens the first URL in an embedded preview. Windows, macOS and Linux support WebView; Linux needs WebKitGTK 4.1 or 4.0 installed.

On Windows and macOS, the preview Action Panel offers Refresh, Go Back, Go Forward, Open in Browser, and Close Webpage. **Run in background** retains the page when Wox is hidden; explicitly closing the page releases it. Preview-only query-hotkey panels also offer **Hide Webpage** when background retention is enabled. Linux supports WebView previews but does not currently expose these page controls in the Action Panel. The WebSearch result can still provide its own Open in Browser action.

## Fixed Websites

URLs do not need parameters. For example, add keyword `x`, title `X`, URL `https://x.com`, and enable **Open in WebView by default**. Type `x` followed by a space to open the site. Fixed sites focus the page immediately; searches with parameter slots keep focus in the query box.

To open a site directly from a Query Hotkey, use its WebSearch trigger followed by a space. See [Hotkeys](../../usage/hotkeys.md).

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
