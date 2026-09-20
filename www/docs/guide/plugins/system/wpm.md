# Plugin Manager

Plugin Manager installs, updates, and inspects plugins. Triggers are `wpm`, `store`, and `pm`.

![The Plugin Manager](/images/guide/plugin_wpm.jpg)

## Quick Start

```text
wpm
wpm install
wpm install everything
```

| Query | Use |
| --- | --- |
| `wpm` | Browse installed and store plugins |
| `wpm install <name>` | Search the store and install |
| Double-click a `.wox` file | Open the same local installer |

Actions can install, update, uninstall, open the plugin website, or copy an AI prompt for creating a plugin. You can also create single-file or full plugins from templates.

Node.js and Python plugins need those runtimes. Refresh runtimes from Plugin Manager settings if a host was just installed.

See the [plugin store](/store/plugins) and [how to create a plugin](/development/).
