# Contributing to Wox

This guide covers the practical contribution flow for Wox contributors.

## Before you start

1. Fork [Wox-launcher/Wox](https://github.com/Wox-launcher/Wox) on GitHub
2. Clone your fork locally
3. Follow [Development Setup](./setup.md)

```bash
git clone https://github.com/YOUR-USERNAME/Wox.git
cd Wox
make dev
```

## How to work in this repository

Wox is a multi-project repository. A small change in one layer can easily break another layer if the contract drifts. Keep that in mind when scoping and verifying your change.

A useful rule of thumb:

- change only one behavior at a time
- verify at the highest layer your change touches
- update docs when user-facing behavior, APIs, or workflow changed

## Typical workflow

1. Create a branch from `master`

```bash
git checkout -b feature/your-change
```

2. Make the change in the correct layer

- `wox.core/` for backend logic, built-in plugins, settings, contracts
- `wox.ui.flutter/wox/` for launcher UI, settings UI, screenshot UI, platform presentation
- `wox.plugin.host.*` for plugin runtime bridge behavior
- `wox.plugin.*` for public plugin SDK changes
- `www/docs/` for documentation

3. Run focused verification while you work

Examples:

```bash
make -C wox.core build
make -C wox.plugin.host.nodejs build
make -C wox.ui.flutter/wox build
```

4. Run broader verification before opening a PR

```bash
make build
```

Use `make smoke` when you changed a user-facing desktop flow and need end-to-end coverage.

## Testing expectations

Use the smallest verification that proves the change is correct, then finish with the broadest verification needed for the layer you touched.

Useful commands:

```bash
make test
make smoke
make build
```

In practice:

- `make test` is the default backend regression check
- `make smoke` is valuable for launcher, screenshot, settings, and other real UI workflows
- `make build` is the final cross-project guardrail for shared contract changes

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat`
- `fix`
- `docs`
- `refactor`
- `perf`
- `test`
- `chore`

Examples:

```bash
git commit -m "feat(plugin): add screenshot API"
git commit -m "fix(webview): restore open in browser action"
git commit -m "docs(development): refresh contributor setup guide"
```

## Pull requests

A good pull request should make review easy:

- explain the behavior change, not just the files you touched
- link the related issue or discussion when available
- describe how you verified the change
- include screenshots or recordings for visible UI changes
- update docs when the workflow, API, or visible behavior changed

## Code style

Follow the conventions already used in the repository:

- Go: `gofmt`
- Dart: `dart format`
- TypeScript/JavaScript: existing repo lint/style rules
- Python: existing repo formatter/style rules

Prefer simple control flow and keep changes local to the layer that owns the behavior.

## Documentation changes

Documentation source files live under `www/docs`.

To preview the docs locally:

```bash
cd www
pnpm install
pnpm docs:dev
```

If you changed commands, APIs, setup instructions, or plugin behavior, update the corresponding docs in the same pull request.
