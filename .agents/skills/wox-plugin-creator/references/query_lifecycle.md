# Query lifecycle and long-running work

## Query budgets

These limits describe the Wox scheduler in this repository. Check the supported Wox and SDK versions before relying on cancellation APIs in a distributed plugin.

| Query | Per-plugin execution budget | What the user sees on timeout |
| --- | --- | --- |
| Global search, such as typing `fix` without a trigger keyword | 1.5 seconds | Wox omits the timeout row and completes the query with the available results. |
| Query directed to a plugin, such as `repo search fix` using the plugin's `repo` trigger keyword | 5 seconds initially. Each accepted nonempty push refreshes that window up to 15 seconds from the start. | Wox shows a plugin timeout result. |

Plugin metadata has no per-plugin timeout override. An empty push, a rejected batch, or a status-only toolbar or preview update does not extend the deadline. Use the original `ctx` and `query` for every batch. Push only new usable results, and stop producing query results if a push returns `false`. `PushResults` extends the budget only while the directed `query()` call is still running.

Wox supersedes the previous query in the same launcher session on a new query or when the launcher hides. It discards late results. An invocation that ignores cancellation can continue to hold the plugin's execution slot in that session until it finishes. Three consecutive timeouts within 60 seconds pause that plugin's global participation for 60 seconds. A directed query can still run during the pause, and a successful directed query clears it. `doctor perf` shows the plugin's timeout count and circuit state.

## Guide the user to the right entry point

- **Fast search while typing.** Use `*` only when the plugin can return relevant results quickly from local data or a warm cache. For example, a repository plugin can show recent local matches and offer `Search remotely` on a relevant row. Use that action to open a directed query such as `repo search fix`, using the configured trigger keyword if the user has renamed it. Skip unrelated input instead of displaying a generic help row in global results.
- **A slow lookup the user expects to wait for.** Put it behind a named trigger or command and show what the lookup is waiting on. For example, `repo search fix` can immediately show cached matches and stream new remote matches as they arrive. If the first data batch usually takes longer than 5 seconds, offer an action such as `Search remote repositories` and track progress separately. A toolbar message can explain the wait but leaves the query deadline unchanged.
- **A task that should survive the launcher.** For exports, transcription, or index rebuilds, return a row such as `Export 42 notes` with a `Start export` action. Start the job after the user invokes the action, give it its own cancellation and state, and surface progress in a task view the user can reopen. Offer `Cancel` while running and show the output or a useful failure reason when it finishes. A Toast can report completion after the launcher hides. Keep durable job state in storage if the job also needs to survive a host or Wox restart.

## Handle cancellation in the runtime

- **Node.js.** The query callback can receive an optional `ctx.Signal` (`AbortSignal`). Pass it to cancellable I/O such as `fetch(url, { signal: ctx.Signal })` and stop scheduling follow-up work when it is aborted. Release readers, timers, and listeners in `finally`. Keep an operation-level timeout for older hosts that lack the signal. Treat cancellation as the end of an outdated query and leave error rows for failures the user can act on.
- **Python.** The host requests cancellation of the running query task with `asyncio.Task.cancel()`. Use awaitable I/O with bounded timeouts, clean up in `finally`, and re-raise `asyncio.CancelledError` after any cancellation-specific cleanup. Avoid blocking the event loop with synchronous I/O or long CPU work. Offloading a blocking call to a thread keeps the event loop responsive, and the underlying call still needs its own timeout or cancellation support.

Verify that rapid edits or hiding the launcher stop query-owned work, that directed queries can accept partial results without duplicates, and that a rejected push ends the stream. Check that quickly typing unrelated text stays responsive and that an action-owned job can be reopened or canceled after hiding the launcher.
