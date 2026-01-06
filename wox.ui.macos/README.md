# Wox macOS UI

Native macOS launcher UI for Wox, built with Swift and SwiftUI.

## Architecture

This project implements the frontend UI for Wox, communicating with `wox.core` via WebSockets.

- **Models.swift**: Defines JSON structures for WebSocket messages (`WoxWebsocketMsg`, `WoxQueryResult`, etc.).
- **WebSocketManager.swift**: Handles low-level WebSocket connection using `URLSession`.
- **ViewModel.swift**: Manages state, processes incoming query results, and handles user actions.
- **ContentView.swift**: Main UI View using SwiftUI.

## Prerequisites

- macOS 12.0+
- Swift 5.9+

## Building

```bash
swift build
```

The executable will be located in `.build/debug/wox.ui.macos`.

## Running

1. Start `wox.core` (which runs the WebSocket server).
   - By default, `wox.core` (dev mode) listens on port `34987`.
2. Run the UI:

```bash
.build/debug/wox.ui.macos 34987
```

## Features

- [x] WebSocket connection to Core
- [x] Incremental Query search
- [x] Result list display
- [x] Keyboard navigation (Up/Down/Enter)
- [x] Execute default action
- [x] Hide/Show app signals handling

## TODO

- [ ] Global shortcut handling (needs Core registration or specialized Swift logic)
- [ ] Richer UI (Icons, Preview panel)
- [ ] Settings View
- [ ] Packaging as `.app` bundle
