# Development Setup

This guide is for contributors who want to build, run, and debug Wox locally.

## What you are setting up

The repository contains four parts that usually move together during development:

- `wox.core/`: Go backend, built-in plugins, settings, storage, packaging entrypoint
- `wox.core/ui/`: native Go UI compiled in the same Go module and process
- `wox.plugin.host.nodejs/`: Node.js plugin host
- `wox.plugin.host.python/`: Python plugin host

The top-level `Makefile` wires these pieces together. In most cases you should start from the repository root instead of running each subproject manually.

## Prerequisites

Install these tools first:

- [Go](https://go.dev/dl/)
- [Node.js](https://nodejs.org/)
- [pnpm](https://pnpm.io/)
- [uv](https://github.com/astral-sh/uv)

Recommended editor:

- [Visual Studio Code](https://code.visualstudio.com/) with the checked-in workspace settings

## Platform-specific requirements

### macOS

- Install [create-dmg](https://github.com/create-dmg/create-dmg) if you plan to build the packaged `.dmg`

### Windows

- Run build commands from a `MINGW64` shell
- Install [MinGW-w64](https://www.mingw-w64.org/) so native Windows runner code can compile

### Linux

- Install `patchelf`
- Install `appimagetool`, or point `APPIMAGE_TOOL` at a local binary when building AppImage packages

## Bootstrap the workspace

From the repository root:

```bash
make dev
```

What this does:

- checks required toolchain dependencies
- prepares embedded resource folders used by `go:embed`
- builds `woxmr` under `wox.core`
- builds both plugin hosts

`make dev` prepares the shared runtime pieces. Use `make build` when you need a runnable package with the embedded Go UI.

## Common commands

From the repository root:

```bash
make dev
make test
make test-go-ui-unit
make test-go-ui-smoke
make build
```

What they mean:

- `make dev`: prepare the local development environment
- `make test`: run the Go integration-style test suite under `wox.core/test`
- `make test-go-ui-unit`: run retained-widget and automation contract tests without opening a window
- `make test-go-ui-smoke`: build the test-only automation binary and run a real native launcher smoke
- `make build`: compile the Go UI into `wox.core`, then build plugin hosts and platform packaging output

If you are changing backend/plugin contracts, `make build` is the safest final verification because it catches cross-project drift.

## Working on specific areas

### Go backend (`wox.core`)

Typical tasks:

- plugin runtime and metadata changes
- built-in plugin behavior
- settings, persistence, routing, packaging

Useful command:

```bash
make -C wox.core build
```

### Go UI (`wox.core/ui`)

Typical tasks:

- launcher UI
- settings UI
- screenshot flow
- webview and preview rendering

Useful command:

```bash
make test-go-ui-unit
make test-go-ui-smoke
```

### Plugin hosts

Useful commands:

```bash
make -C wox.plugin.host.nodejs build
make -C wox.plugin.host.python build
```

Use these when you are only changing host/runtime behavior and want a faster loop than `make build`.

## Running the documentation site

The docs live in `www/docs`. To preview them locally:

```bash
cd www
pnpm install
pnpm docs:dev
```

To generate a production build:

```bash
cd www
pnpm docs:build
```

## Where Wox stores local data

Wox keeps runtime data under the user's home directory:

- macOS / Linux: `~/.wox`
- Windows: `C:\Users\<username>\.wox`

Useful subdirectories:

- `~/.wox/log/wox.log`: core log
- `~/.wox/log/ui.log`: UI log
- `~/.wox/plugins/`: local plugin development directory

## Troubleshooting

If `make dev` fails early:

- confirm `go`, `node`, `pnpm`, and `uv` are all on `PATH`
- on Windows, confirm `nuget` is also on `PATH`
- on Windows, confirm you are in a `MINGW64` shell instead of PowerShell or CMD
- on Linux packaging builds, confirm `patchelf` and `appimagetool` are installed

If a change compiles in one subproject but Wox still breaks end to end, run `make build` from the repository root. That is the fastest way to catch contract mismatches between `wox.core`, the Go UI, and the plugin hosts.
