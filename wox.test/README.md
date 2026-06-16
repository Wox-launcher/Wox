# wox.test

Cross-platform end-to-end orchestration for Wox.

This directory is responsible for:

- forcing `wox.core` to use isolated Wox data and user data directories
- starting `wox.core` in development mode
- cloning, initializing, packaging, and installing the official Node.js and Python plugin templates into the isolated smoke environment
- running Flutter desktop `integration_test` cases
- collecting test artifacts under `wox.test/artifacts/`

## Current scope

The initial runner executes the Flutter desktop smoke test against a real
`wox.core` backend. It prefers the usual development port `34987` and falls
back to a free local port when that port is already occupied.

This is intentionally lighter than a full packaged-binary workflow so we can
stabilize the test flow first. Once the smoke path is stable, we can add a
second layer that validates packaged builds.

## Usage

From this directory:

```bash
make smoke
make smoke "P0-SMK-04"
```

Or directly:

```bash
dart run bin/run.dart smoke
dart run bin/run.dart smoke "P0-SMK-04"
```

## Artifacts

Each run starts by clearing `wox.test/artifacts/`, then creates a timestamped directory for the current run with:

- `core.log`
- `flutter_test.log`
- `template_plugin.log`
- `wox-data/` (isolated backend data, logs, lock file, embedded resources)
- `user-data/` (isolated backend settings, database, plugins, themes)

## Notes

- The runner prefers port `34987` and automatically falls back to a free port.
- The runner overrides `WOX_TEST_DATA_DIR` and `WOX_TEST_USER_DIR` so the test
  run does not touch the developer's normal Wox data.
- The runner disables telemetry for smoke runs.
- Smoke plugin coverage currently depends on `git`, `make`, `pnpm`, `uv`, and network access so the official Node.js and Python templates can be cloned and packaged during the run.
- On Windows, stop a locally running `build/windows/.../wox-ui.exe` before
  running smoke tests, or the linker will fail to overwrite that binary.
