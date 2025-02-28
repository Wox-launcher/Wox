# Frequently Asked Questions (FAQ)

This document addresses common questions and issues that users may encounter when using Wox.

## General Questions

### What is Wox?

Wox is a cross-platform launcher that allows you to quickly search for applications, files, and perform various actions using keyboard shortcuts. It's designed to boost your productivity by reducing the time spent navigating through menus and folders.

### Which platforms does Wox support?

Wox supports Windows, macOS, and Linux.

### Is Wox free to use?

Yes, Wox is completely free and open-source.

### How do I launch Wox?

By default, you can launch Wox using:
- Windows/Linux: <kbd>Alt</kbd>+<kbd>Space</kbd>
- macOS: <kbd>Command</kbd>+<kbd>Space</kbd>

You can customize this shortcut in the settings.

## Installation and Setup

### I can't install Wox on macOS due to security restrictions. What should I do?

If you see a message saying "Wox can't be opened because it is from an unidentified developer", you can:

1. Right-click (or Control-click) on the Wox app
2. Select "Open" from the context menu
3. Click "Open" in the dialog that appears

### How do I update Wox to the latest version?

You can check for updates by:
1. Launching Wox
2. Typing `wpm update`
3. Following the prompts to update

Alternatively, you can download the latest version from the [GitHub releases page](https://github.com/Wox-launcher/Wox/releases).

## Usage

### How do I search for files and applications?

Simply launch Wox and start typing. Wox will automatically search for matching files and applications.

### How do I use plugins?

Plugins can be used by typing their trigger keyword followed by your query. For example, to search for a plugin, type `wpm search [plugin name]`.

### How do I install new plugins?

To install new plugins:
1. Launch Wox
2. Type `wpm install [plugin name]`
3. Press Enter to install the plugin

### How do I change the theme?

To change the theme:
1. Launch Wox
2. Type `theme` to see available themes
3. Select a theme to apply it

### How do I create a custom theme with AI?

To create a custom theme with AI:
1. Configure your AI settings as described in [AI Settings](ai_settings.md)
2. Launch Wox
3. Type `theme ai [your theme description]`
4. The AI will generate a theme based on your description

## Troubleshooting

### Wox is not launching when I press the shortcut

Try the following:
1. Check if Wox is running in the background
2. Restart Wox
3. Check if another application is using the same shortcut
4. Try changing the shortcut in the settings

### A plugin is not working correctly

Try the following:
1. Disable and re-enable the plugin
2. Update the plugin using `wpm update [plugin name]`
3. Reinstall the plugin using `wpm install [plugin name]`

### Wox is running slowly

Try the following:
1. Disable unused plugins
2. Clear the Wox cache by typing `wox cache clear`
3. Restart Wox

### How do I view logs for debugging?

You can view the logs by:
1. Opening a terminal or command prompt
2. Running `tail -n 100 ~/.wox/log/log`

### Wox crashes on startup

Try the following:
1. Check the logs for error messages
2. Try running Wox from the terminal to see any error output
3. Reinstall Wox

## Advanced

### Can I sync my settings across multiple devices?

Currently, Wox doesn't have a built-in sync feature. However, you can manually copy the `.wox` directory from one device to another.

### How do I develop a plugin for Wox?

Refer to the [Plugin Development Guide](plugin_development.md) for detailed instructions on creating plugins.

### Can I contribute to Wox development?

Yes! Wox is an open-source project and welcomes contributions. See the [Contributing Guide](contributing.md) for more information.

### How do I report a bug or request a feature?

You can report bugs and request features on the [GitHub Issues page](https://github.com/Wox-launcher/Wox/issues).

## Still Need Help?

If your question isn't answered here, you can:
1. Join the [GitHub Discussions](https://github.com/Wox-launcher/Wox/discussions) to ask the community
2. Check the [documentation](https://wox-launcher.github.io/Wox/#/) for more detailed information
3. Submit an issue on [GitHub](https://github.com/Wox-launcher/Wox/issues) 