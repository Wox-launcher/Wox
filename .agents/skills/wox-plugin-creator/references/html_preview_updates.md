# Updating an existing HTML WebView preview

Keep the preview-selection guidance in `SKILL.md`: HTML remains the last option.
Once an inline HTML WebView is appropriate, prefer script-mode updates for changing
data, progress, and elements. Keep the same document alive to preserve scrolling,
text selection, focus, and CSS animations.

## Contract and lifecycle

- `query()` returns complete HTML using the latest task state, with a page updater
  such as `window.updateProgress(state)`. Returning to the query reconstructs that
  state; it must not restart the task merely to rebuild the preview.
- For later updates, use the existing `UpdateResult` / `update_result`, the same
  result ID, and its query context. Keep preview type `webview`; send JSON containing
  `updateMode: "script"` and `script`. No new public API method or SDK upgrade is
  needed to carry these fields.
- Script instructions are valid only in `UpdateResult` and only for the current
  plugin-owned inline HTML document. Do not mix them with `html` or `url`, return
  them from `query()`, or target URL previews. Noncurrent/missing targets return
  `false`; stop pushing after leaving the query and resume with its new context.
- `true` means accepted, not successful execution. Instructions wait in order for
  the target document to load. Switching results, replacing the document, or
  destroying the page discards pending work. Script exceptions are logged without
  replacing the preview; results are not returned and Promises are not awaited.
- Wox retains the complete HTML in its result cache, never caches/replays the
  script, and continues processing normal fields such as Actions. Keep current
  task state in the plugin so the next query can return a fresh full snapshot.
- Use a complete `{"html":"..."}` update when intentionally replacing the page
  structure. Do not repeatedly resend full HTML just to refresh data.
- This mode needs a Wox host containing script-mode WebView updates. Verify the
  actual first supporting release before setting `MinWoxVersion`; the SDK or
  single-file runtime's older version floor does not imply this feature exists.
  Do not invent a release number for an unreleased host change.

## Page updater

Define the updater in the initial HTML and invoke it with the serialized current
snapshot during initial rendering. For example, with `<span id="status"></span>`:

```javascript
window.updateProgress = state => {
  const node = document.getElementById("status");
  const value = String(state.message ?? "");
  if (node.textContent !== value) node.textContent = value;
};
```

Use stable category/item IDs and append new nodes. Update text only when it changes;
keep the scroll container and running animation nodes. Avoid replacing `body`, the
whole dashboard, or its entire `innerHTML` on each update. Keep follow-scroll state
in page memory: scrolling up pauses following, returning to the bottom resumes it.
Do not use `sessionStorage` or `window.name` patches to survive repeated reloads.

Serialize state into the script with `JSON.stringify` / `json.dumps`; never splice
raw logs or user text into executable code. Use `textContent` for those values.
When embedding initial JSON in an HTML `<script>` element, additionally escape `<`
as `\u003c` to prevent a literal `</script>` from ending that element.

## Python update

```python
import json
from wox_plugin import UpdatableResult, WoxPreview, WoxPreviewType

# ctx and result_id belong to the currently displayed query result.
state = {"message": "Scanning…"}
accepted = await self.api.update_result(ctx, UpdatableResult(
    id=result_id,
    preview=WoxPreview(
        preview_type=WoxPreviewType.WEBVIEW,
        preview_data=json.dumps({
            "updateMode": "script",
            "script": "window.updateProgress(" + json.dumps(state) + ");",
        }),
    ),
))
```

## Node.js update

```javascript
// ctx and resultId belong to the currently displayed query result.
const state = { message: "Scanning…" }
const accepted = await this.api.UpdateResult(ctx, {
  Id: resultId,
  Preview: {
    PreviewType: "webview",
    PreviewData: JSON.stringify({
      updateMode: "script",
      script: `window.updateProgress(${JSON.stringify(state)});`
    })
  }
})
```

For a dynamic preview, validate continuous updates with simulated data: no page
reload, uninterrupted animation, stable text selection, paused scrolling while
reading earlier output, and restoration of current state after re-entering.
