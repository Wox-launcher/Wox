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
| `wpm` | List commands: install, update, uninstall, and create |
| `wpm install` | Browse store plugins |
| `wpm install <name>` | Search the store and install |
| `wpm update` | List available plugin updates; update one or all |
| `wpm uninstall` | List installed plugins for removal |
| `wpm create` | Create a plugin from a template |
| Double-click a `.wox` file | Open the same local installer |

Actions can install, update, uninstall, open the plugin website, or copy an AI prompt for creating a plugin. You can also create single-file or full plugins from templates.

Node.js and Python plugins need those runtimes. If an interpreter was just installed, open **Settings -> Plugins -> Runtime Settings** and refresh its status, or configure its executable path.

See the [plugin store](/store/plugins) and [how to create a plugin](/development/).
