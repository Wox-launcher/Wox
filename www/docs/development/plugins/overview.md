# Plugin Overview

## Plugin Categories

Wox plugins are categorized by their installation type:

1. **System Plugin**: These are plugins bundled with Wox and cannot be uninstalled. They provide essential functionalities. For instance, `wpm` is a system plugin used for plugin management.

2. **User Plugin**: These are plugins installed by the user. They can be installed, uninstalled, updated, or disabled by the user.

## Plugin Implementation Types

Wox supports three plugin implementation approaches:

If you use Codex or another compatible agent, see [AI Skills For Plugin Development](./ai-skills.md).

### Script Plugin

Script plugins are lightweight, single-file plugins that are perfect for simple automation tasks and quick utilities.

**Features:**

- **Single File Implementation**: Entire plugin logic contained in one script file
- **On-demand Execution**: Scripts are executed per query, no persistent running required
- **Multi-language Support**: Supports Python, JavaScript, Bash and other scripting languages
- **Simplified Development**: Metadata defined through comments, no complex configuration files needed
- **Instant Effect**: Changes take effect immediately after modifying script file, no restart required
- **JSON-RPC Communication**: Communicates with Wox through standard input/output using JSON-RPC

**Use Cases:**

- Simple file operations and system commands
- Quick text processing and format conversion
- Calling external APIs for simple data retrieval
- Personal automation scripts and utilities
- Learning and prototype development
- Functions that don't require complex state management

**Limitations:**

- Scripts are re-executed for each query, relatively lower performance
- No support for complex async operations and state management
- Limited API functionality
- No support for plugin settings interface
- Execution timeout limit (10 seconds)

### Single-file SDK Plugin

Single-file SDK plugins are one `.py` or CommonJS `.js` file with the full Wox Public API. They load into the existing Python or Node.js runtime host.

See the [Single-file SDK Plugin](./single-file-plugin.md) guide.

**Features:**

- One file, same as a script plugin
- Persistent host process: query/action do not start a new interpreter
- Full Public API: settings, AI, UpdateResult, PushResults, deep links, unload callbacks
- Save to reload
- Python can import `wox_plugin`; Node.js uses `params.API`

**Limitations:**

- No pip/npm dependencies, extra files, or relative images
- Node.js first version is CommonJS only

### Full-featured Plugin

Full-featured plugins are comprehensive plugins designed for complex application scenarios and high-performance requirements.

**Features:**

- **Complete Architecture**: Runs through dedicated plugin host processes
- **Persistent Running**: Plugins remain loaded and running, supporting state management
- **Rich APIs**: Supports AI integration, previews, settings UI, MRU, deep links, screenshot capture, and other advanced features
- **WebSocket Communication**: Efficient communication with Wox core through WebSocket
- **Async Support**: Full support for asynchronous operations
- **Lifecycle Management**: Complete plugin initialization, query, and unload lifecycle

**Use Cases:**

- Applications requiring complex state management
- High-frequency queries and real-time data processing
- Smart plugins with AI integration
- Plugins requiring custom settings interface
- Complex async operations and network requests
- Performance-sensitive application scenarios
- Commercial-grade plugin development

**Supported Languages:**

- **Python**: Using `wox-plugin` SDK
- **Node.js**: Using `@wox-launcher/wox-plugin` SDK

## Selection Guide

### Choose Script Plugin when:

- Functionality is relatively simple with clear logic
- No complex state management required
- You only need to wrap a command or one-shot script
- Rapid prototyping and personal tools
- Learning Wox plugin development

### Choose Single-file SDK Plugin when:

- You want one file, but need the Wox API
- You need settings, AI, UpdateResult, or unload callbacks
- You can stay on Python, or CommonJS Node.js, without extra files

### Choose Full-featured Plugin when:

- You need dependencies, resources, TypeScript, or multiple files
- Complex business logic required
- High-frequency queries and real-time responses
- Commercial plugin development

## Plugin Commands

Wox plugins can have commands that provide specific functionalities. For example, the `wpm` plugin has commands like `install` and `remove` for plugin management.
