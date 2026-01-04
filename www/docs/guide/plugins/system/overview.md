# System Plugins Overview

Wox includes multiple built-in system plugins that are available out of box. These plugins cover various aspects of your daily work and help improve productivity.

## Core Plugins

| Plugin Name | Description | Trigger Keyword |
|-------------|-------------|-----------------|
| **Application** | Search and launch applications | None (default) |
| **Calculator** | Mathematical calculations | None (auto-detect) |
| **Clipboard** | Clipboard history management | `cb` |
| **Converter** | Unit, currency, and number base conversion | None (auto-detect) |
| **WebSearch** | Web search | Custom |

## Search & File Management

| Plugin Name | Description | Trigger Keyword |
|-------------|-------------|-----------------|
| **File** | File search | None (default) |
| **Explorer** | Quick folder access | None (default) |
| **Browser Bookmark** | Browser bookmark search | None (default) |

## AI & Tools

| Plugin Name | Description | Trigger Keyword |
|-------------|-------------|-----------------|
| **Chat** | AI conversations | None (UI panel) |
| **AI Command** | AI command execution | Custom |
| **Emoji** | Emoji search | None (default) |

## Utility Plugins

| Plugin Name | Description |
|-------------|-------------|
| **Selection** | Quick actions on selected text |
| **Plugin Installer** | Plugin installation management |
| **Backup** | Settings backup & restore |
| **Theme** | Theme management |
| **Update** | Check for updates |
| **Doctor** | System diagnostics |

## Quick Start

### Using Default Trigger Plugins

Most core plugins work without keywords. Simply type relevant content in Wox:

- **Launch apps**: Type app name, like "Chrome", "VSCode"
- **Calculate**: Type expression, like "100+200", "12*5"
- **File search**: Type file name, like "report.pdf"
- **Unit conversion**: Type conversion, like "100 usd to cny", "1km to m"

### Using Keyword Plugins

Some plugins require a trigger keyword first:

- **Clipboard history**: Type `cb` to view history
- **Custom search**: Type trigger keyword first, then search content, like `g Wox Launcher`

## Plugin Configuration

Most system plugins can be customized in Wox settings:

1. Open Wox settings
2. Find corresponding plugin
3. Click on plugin name to enter configuration
4. Adjust settings as needed

### Common Configuration Options

- **Application**: Add custom app search directories
- **WebSearch**: Add custom search engines
- **Clipboard**: Adjust history retention days
- **Converter**: Set default currency

## Plugin Features

### MRU (Most Recently Used)

Plugins with MRU support remember your usage history and prioritize previously used results:

Plugins supporting MRU:
- Application
- Converter

### Selected Text Operations

Plugins with selected text support can perform quick actions on your selected text:

- WebSearch: Direct search of selected text
- Converter: Direct unit conversion or calculation

## Getting Help

If you have questions about specific plugin usage, check the detailed documentation:

- [Application Usage Guide](application.md)
- [Calculator Usage Guide](calculator.md)
- [Clipboard Usage Guide](clipboard.md)
- [Converter Usage Guide](converter.md)
- [WebSearch Usage Guide](websearch.md)
