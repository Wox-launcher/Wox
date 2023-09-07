## Query

Let's take `wpm install wox` as an example, `wpm` is the trigger keyword, `install` is the command, `wox` is the search term.

### Trigger keyword

Trigger keyword can be used to trigger a plugin. A plugin must have at least one trigger keyword.

```json
{
  "TriggerKeywords": ["wpm", "p"]
}
```

There is one special trigger keyword `*`, which means the plugin will be triggered by any query term. We called this **Global trigger keyword**.

```json
{
  "TriggerKeywords": ["*"]
}
```

**Global trigger keyword** has following restrictions:

* if you defined a global trigger keyword, you shouldn't define any other trigger keywords.
* if you defined a global trigger keyword, you shouldn't define any command.

### Command

Command can be used to tell user what functionality the plugin provides. A plugin can have zero or multiple commands, which can be predefined in the plugin.json.

```json
{
  "Commands": [
    {
      "Name": "install",
      "Description": "Install plugin"
    },
    {
      "Name": "remove",
      "Description": "Remove plugin"
    }
  ]
}
```

### Search term

All other terms besides of `Trigger Keyword` and `Command` are considered as search term. Search term is the input for the plugin to do the actual work.