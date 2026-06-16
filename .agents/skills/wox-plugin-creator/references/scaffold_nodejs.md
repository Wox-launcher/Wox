# Node.js Plugin Scaffold

Best practice: clone the official template repo instead of hardcoding a scaffold.

```bash
git clone https://github.com/Wox-launcher/Wox.Plugin.Template.Nodejs my-plugin
```

Before editing a new project, run `make init` in the project root when initialization has not been done yet.

Then follow the template README, update `plugin.json` fields (ID, Name, Description, TriggerKeywords), and implement your plugin logic.

Build and publish:

```bash
make publish
```
