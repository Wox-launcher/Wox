# Supervisor

The supervisor is a separate process. It watches Wox, restarts it after a crash, and runs tasks that can only happen while Wox is fully stopped.

## Independence

Production files in this package must not import any other Wox package (`wox/...`). The supervisor keeps running after Wox exits, so it cannot depend on the database, settings, plugins, UI, logging, or embedded resources.

Paths are resolved from `~/.wox` unless `main` calls `SetDataDirectory`. The application version is passed in with `SetVersion`. Launch flags that must survive a crash restart are registered with `PreserveArgOnRestart`. Windows crash-handler bytes are passed into `ConfigureCrashCapture`; this package does not embed them.

Feature work is registered from `main` with `Register`. The restore task's directory replacement stays in `setting`.

## Tasks

Crash monitoring is the process loop, not a queued task:

- A clean exit with no accepted task stops the supervisor.
- An abnormal exit is recorded and Wox is restarted within a small limit.
- An accepted task runs after the process has exited, and then Wox starts again.
- If the launching Wox process does not exit within the wait limit, the supervisor stops without running tasks or launching another Wox. An accepted task remains in the journal for a later launch.
- An abnormal exit is captured before any accepted task runs, including crashes during shutdown.

Wox submits a task over a local control connection. Each detached supervisor has a unique address: a named pipe on Windows (`\\.\pipe\WoxSupervisor-<id>`) and a Unix socket at `~/.wox/supervisor/control.sock-<id>` elsewhere. The launching Wox selects that address after starting the supervisor; the supervisor and its Wox children inherit it through `WOX_SUPERVISOR_CONTROL_ADDRESS`. Explicit restarts can overlap without replacing another supervisor's endpoint. Unix listeners never remove an existing socket to bind, and the supervisor closes its own listener before exiting.

The supervisor confirms the task, Wox exits, and the supervisor runs the registered handler before starting the next process. That process reads the outcome with `TakeTaskResult`.

`~/.wox/supervisor/task.json` is a private journal for a task that was accepted but not yet run. Callers do not read or write it.

## Log

The supervisor writes its own log to `~/.wox/log/supervisor.log`.

On Windows, the out-of-process WER helper appends capture stages and Windows
error codes to that same log. Registration success does not imply that WER ran:
look for `fatal_callback_entered`, then `dump_completed` or a failing stage.
After an abnormal exit the supervisor waits up to 10 seconds for a dump and
explicitly logs when none was collected. A late WER dump remains in the dump
directory but is not retroactively added to an already exported report.

## Windows UI hangs

While a child is alive, the supervisor probes its top-level windows (including
hidden windows) every 5 seconds with a bounded `WM_NULL` request. At least 30
seconds of consecutive unresponsiveness triggers a live minidump and diagnostic
ZIP. A single stall produces one capture attempt until responsiveness returns;
long sampling gaps, such as sleep, do not count toward a new timeout. Capture
errors include the failing API stage in the supervisor log. The probe does not
change rendering, free memory, terminate the child, or request a restart.

This detects blocked window message loops, not every application-level hang:
a thread that continues dispatching messages may still have a stalled workflow.
No window means there is nothing to probe yet. Non-Windows platforms retain
their existing crash handling without this native window watchdog.

`go test ./supervisor` exercises a hidden blocked window in a test subprocess,
live dump generation and report inclusion. Set `WOX_CRASH_CAPTURE_INTEGRATION=1`
to additionally exercise real WER crashes, including a deliberately unwritable
dump destination. These tests restore the WER settings they change.

Run the WER integration tests from a normal Windows terminal with access to the
host user's registry. A registry-isolated execution environment can return
success from both registration and registry readback while the host WerFault
process cannot see the module allowlist. In that case the callback never runs;
neither a direct DLL callback test nor registration success validates the full
WER path. The integration tests require the expected fail-fast exit, a valid
minidump and completion log, or the expected dump failure stage.
