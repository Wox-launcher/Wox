# Shell Plugin

Shell runs commands from Wox and keeps command history. Use `>` for an explicit shell query. Wox can also detect some commands globally.

![The Shell plugin](/images/guide/plugin_shell.jpg)

## Quick Start

```text
> git status
> npm test
```

Type `>` followed by a space to start an interactive shell in the right pane. Press Enter on **New shell** to focus it, or keep typing in the query box to enter a one-off command. Interactive PowerShell, Bash, and Zsh create one history entry per session, titled after its first command. Completed commands and their recent output are saved to that entry. Press `Esc` to focus the query box, or `Shift+Esc` to send Escape to the shell. Leaving Shell or hiding the preview or launcher ends the session.

Use the arrow keys to select a history entry and press Enter to open it in the Shell preview. Interactive history restores the saved text in a live terminal. Scroll up to read earlier output. Commands run there extend that same history entry. Press `Esc` to return focus to the query box. For one-off commands, the Action Panel also offers **Re-execute**.

A running command shows a live indicator in history. Set a default working directory, or pick a per-command directory (home, last used, or custom) in plugin settings.

The Action Panel can save output to Notes, rerun a previous command, or open the command in the system terminal.
