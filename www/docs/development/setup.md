# Development Setup

## Recommended IDE

- [Visual Studio Code](https://code.visualstudio.com/) - Recommended IDE as the workspace is pre-configured with all necessary settings and extensions.

## Required Dependencies

- Install [Golang SDK](https://go.dev/dl/)
- Install [Flutter](https://docs.flutter.dev/get-started/install)
- Install [Nodejs](https://nodejs.org) and [pnpm](https://pnpm.io/)
- Install [uv](https://github.com/astral-sh/uv)

## Platform Specific Dependencies

### Windows

- Install [MinGW-w64](https://www.mingw-w64.org/) (provides `mingw64`) so the Windows native notifier can be compiled when running `go build`.

### macOS

- Install [create-dmg](https://github.com/create-dmg/create-dmg)

## Getting Started

Run the following command to setup the development environment:

```bash
make dev
```
