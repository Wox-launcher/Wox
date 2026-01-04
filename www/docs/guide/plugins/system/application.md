# Application Plugin

The Application plugin allows you to quickly search and launch applications on your system.

## Features

- **Quick Launch**: Type app name directly to launch
- **Smart Matching**: Fuzzy matching for app names and paths
- **MRU Priority**: MRU feature remembers frequently used apps
- **Running State Display**: Shows running apps with CPU and memory usage
- **Multi-platform Support**: Works on Windows, macOS, and Linux

## Basic Usage

### Search and Launch Apps

1. Open Wox (default hotkey `Alt+Space` or `Cmd+Space`)
2. Type app name directly, such as:
   - `Chrome` - Launch Google Chrome
   - `VSCode` - Launch Visual Studio Code
   - `Notion` - Launch Notion

3. Press `Enter` to launch selected app

### Interact with Running Apps

If an app is running, search results will display CPU and memory usage:

```
Chrome
CPU: 2.5%
Memory: 450.3 MB
```

### App Actions

After selecting an app, you can perform these actions:

| Action | Hotkey | Description |
|--------|---------|-------------|
| Open App | `Enter` | Launch app (activate window if already running) |
| Open Containing Folder | - | Open app directory in file manager |
| Copy Path | - | Copy full app path to clipboard |
| Show Context Menu | `Ctrl+M` | Show system context menu (desktop apps only) |
| Terminate App | - | Terminate running app |

## Advanced Features

### Custom App Directories

By default, the Application plugin automatically indexes application directories. You can add custom directories to include more apps:

1. Open Wox settings
2. Find **Application** plugin
3. Click into plugin settings
4. Add custom paths in **App Directories** table
5. Click save

**Notes**:
- Recursive subdirectory search supported
- Default recursive depth is 3 levels
- You can exclude specific directories

### App Types

The Application plugin supports multiple app types:

- **Desktop Apps**: `.app` (macOS), `.exe` (Windows), Linux executables
- **UWP Apps**: Windows Store apps
- **System Settings**: Windows settings pages (Windows only)
- **macOS System Settings**: System Preferences panels (macOS only)

### MRU Feature

The Application plugin supports MRU (Most Recently Used) functionality:

- Frequently used apps are prioritized in results
- Smart sorting based on MRU data
- View MRU usage statistics in settings

## Platform Features

### Windows

- Automatically indexes common app directories:
  - `C:\Program Files`
  - `C:\Program Files (x86)`
  - `%APPDATA%\Microsoft\Windows\Start Menu`
  - `%LOCALAPPDATA%\Microsoft\WindowsApps`
- Supports UWP apps (Microsoft Store apps)
- Supports system settings pages
- Activates window if app is already running

### macOS

- Automatically indexes `/Applications` directory
- Supports `.app` application bundles
- Supports System Preferences panels
- Default behavior: Activate window if app is already running

### Linux

- Supports system default app directories
- Supports `.desktop` files
- May require manual app directory configuration

## FAQ

### Why can't I find a specific app?

1. **Check if it's in indexed directories**: Check app directories list in plugin settings
2. **Wait for indexing**: Newly installed apps may take a few seconds to be indexed
3. **Restart Wox**: Sometimes requires restart to re-index apps

### How do I hide unwanted apps?

The Application plugin doesn't provide hide functionality, but you can:
- Don't place apps in indexed directories
- Use third-party plugins to manage visible apps

### App launch is slow?

1. Check if app path is correct
2. Confirm app file hasn't been moved or deleted
3. Check system resources, ensure enough to launch app

### Why don't CPU and memory info show?

- Ensure app is currently running
- Only desktop apps show CPU and memory info
- Windows UWP apps and macOS system settings don't display this info

## Related Plugins

- [Calculator](calculator.md) - Mathematical calculations
- [Clipboard](clipboard.md) - Clipboard history, useful for pasting results
- [WebSearch](websearch.md) - Web search
