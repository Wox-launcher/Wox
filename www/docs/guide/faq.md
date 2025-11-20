# FAQ

## General

### Wox is not starting?

Check the log file at:

- Windows: `%USERPROFILE%\\.wox\\log`
- macOS/Linux: `~/.wox/log`

### How to reset Wox?

Delete the user data directory:

- Windows: `%USERPROFILE%\\.wox`
- macOS/Linux: `~/.wox`

## Plugins

### Plugin installation failed?

- Check your internet connection.
- Ensure you have the required runtime (Python/Node.js) installed if the plugin requires it.
- Check the logs for detailed error messages.

### Everything plugin?

Wox ships a built-in file plugin (`f`) that depends on the Everything engine. Install and run [Everything](https://www.voidtools.com/) so its service is active and indexed; keep it running in the background so Wox can query it.

### How to update plugins?

Use the `wpm update` command to update all plugins or a specific plugin.

## Customization

### How to change the theme?

Type `theme` in Wox to list available themes, or go to Settings -> Theme to select one.

### How to change the hotkey?

Go to Settings -> General -> Hotkey.
