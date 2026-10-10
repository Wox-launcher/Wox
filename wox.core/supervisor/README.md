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
