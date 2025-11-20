# Wox Architecture

This document provides an overview of the Wox project architecture, explaining how different components interact with each other.

## Overview

Wox is a cross-platform launcher built with a microservices architecture. The application consists of several key components:

- **wox.core**: The Go backend that handles the core functionality
- **wox.ui.flutter**: The Flutter frontend that provides the user interface
- **wox.plugin.host.python**: Host for Python plugins
- **wox.plugin.host.nodejs**: Host for NodeJS plugins
- **wox.plugin.python**: Python plugin library
- **wox.plugin.nodejs**: NodeJS plugin library

## Component Interaction

```
┌─────────────────┐           ┌─────────────────┐
│                 │           │                 │
│  wox.ui.flutter │◄─────────►│    wox.core     │
│  (Flutter UI)   │  WebSocket│   (Go Backend)  │
│                 │    & HTTP │                 │
└─────────────────┘           └────────┬────────┘
                                       │
                                       │ WebSocket
                                       │
                              ┌────────▼────────┐
                              │                 │
                              │  Plugin Hosts   │
                              │                 │
                              └────────┬────────┘
                                       │
                                       │
                              ┌────────▼────────┐
                              │                 │
                              │    Plugins      │
                              │                 │
                              └─────────────────┘
```

### Communication Flow

1. **UI to Core**: The Flutter UI communicates with the Go backend via WebSocket and HTTP
2. **Core to Plugin Hosts**: The Go backend communicates with plugin hosts via WebSocket
3. **Plugin Hosts to Plugins**: Plugin hosts load and communicate with plugins

## Key Components in Detail

### wox.core

The Go backend that serves as the central component of the application. It handles:

- User queries and search functionality
- Plugin management
- Settings management
- Communication with the UI and plugin hosts

Key directories:

- `wox.core/setting`: Contains settings-related definitions
- `wox.core/plugin`: Contains API definitions and implementations

### wox.ui.flutter

The Flutter-based user interface that provides:

- Search interface
- Results display
- Settings management
- Theme customization

### Plugin System

Wox supports plugins written in multiple languages:

- **Python Plugins**: Managed by `wox.plugin.host.python`
- **NodeJS Plugins**: Managed by `wox.plugin.host.nodejs`

Plugin hosts are responsible for:

- Loading plugins
- Executing plugin code
- Communicating results back to the core

## Development Workflow

The development workflow for Wox is managed through the Makefile:

1. `make dev`: Sets up the development environment
2. `make test`: Runs tests
3. `make publish`: Builds and publishes all components
4. `make plugins`: Updates the plugin store

## Platform-Specific Considerations

Wox is designed to be cross-platform, with specific considerations for:

- **Windows**: Standard build artifacts from `make publish` (no UPX compression)
- **macOS**: Uses create-dmg for packaging the app bundle
- **Linux**: Standard build artifacts from `make publish` (no UPX compression)

## Data Flow

1. User enters a query in the UI
2. Query is sent to the core via WebSocket
3. Core processes the query and determines which plugins to invoke
4. Core sends requests to appropriate plugin hosts
5. Plugin hosts execute plugin code and return results
6. Core aggregates results and sends them back to the UI
7. UI displays the results to the user

## Configuration and Data Storage

All user data, including settings and plugin data, is stored in the `.wox` directory in the user's home directory:

- Windows: `C:\Users\<username>\.wox`
- macOS/Linux: `~/.wox`

## Logging

Logs are stored in the `.wox/log` directory and can be accessed for debugging purposes.
