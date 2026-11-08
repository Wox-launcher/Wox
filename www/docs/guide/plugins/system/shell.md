# Shell Plugin

Shell runs commands from Wox and keeps command history. Use `>` for an explicit shell query. Wox can also detect some commands globally.

![The Shell plugin](/images/guide/plugin_shell.jpg)

## Quick Start

```text
> git status
> npm test
```

To open an interactive shell in the preview, type `>` followed by a space. Press Enter or select **Open Interactive Shell**. Press `Esc` to focus the query box, or `Shift+Esc` to send Escape to the shell. Switching results or hiding the preview or launcher ends the session.

A running command shows a live indicator in history. Set a default working directory, or pick a per-command directory (home, last used, or custom) in plugin settings.

The Action Panel can save output to Notes, rerun a previous command, or open the command in the system terminal.
