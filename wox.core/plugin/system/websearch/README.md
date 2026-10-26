# Web Search WebView previews

Each search has an opt-in `OpenInWebView` checkbox inside the collapsed WebView
settings group. Unchecked rows keep browser behavior. Browser selection and private-window preferences
apply to the explicit browser action. WebView displays only the first resolved
HTTP(S) URL. With automatic WebView enabled, the default Enter action focuses
the displayed page without replacing its content or entering manual preview mode.
Otherwise Ctrl+Enter / Cmd+Enter opens a manual preview.
Windows, macOS and Linux support previews; Linux requires WebKitGTK 4.1 or 4.0.

Parameterized WebView searches register a 500 ms input debounce for their trigger keyword.
The core scheduler waits until typing pauses before running the search. Browser
searches and fixed sites use zero delay; selection and fallback queries keep their existing timing.

Normal input, text selection, fallback and restored results use the same action
builder. Enabling WebView makes page focus the default action for explicit execution and
also returns WebView content. A query response with one preview result sets
`Layout.ResultPreviewWidthRatio = 0`, using the existing full-preview layout.
It also supplies the configured logical window dimensions through internal
`Layout.WindowWidth` / `WindowHeight` hints. These fields stay out of the plugin
JSON contract and SDKs. Non-positive dimensions use normal sizing, and replacing
the response with an ordinary layout restores the normal window size.
Results and layout arrive together, so both partial and final snapshots display
the preview directly without first showing a result row or executing an action.
Searches with input slots keep focus in the query box so typing can continue;
fixed sites and preview-only query-hotkey panels focus the page. Multiple results still require a choice.
Fallback and restored results retain their explicit action flow.

Silent queries and result hotkeys execute the action without a visible launcher.
Their callback opens a named secondary window and carries captured variable values
in query context. The window rebuilds the search result from those values so a
focus change cannot replace the original selection or clipboard contents. Native
page rendering and focus remain owned by the WebView controller. Query transitions
and preview visibility use the existing launcher lifecycle without a separate
automatic-presentation policy or state.

`UserAgent` and `KeepInBackground` apply to automatic previews, manual actions and
secondary windows. The default releases pages on hide. Opted-in pages keep native
sessions when switching URLs or hiding Wox; explicit page close destroys them.
The launcher delegates URL replacement to the native hide/show lifecycle rather
than resetting all cached sessions.

Fixed sites use a WebSearch trigger followed by a space, including in query
hotkeys, and support history restoration without input parameters.
