# AI Skills For Plugin Development

Wox ships [`wox-plugin-creator`](https://github.com/Wox-launcher/Wox/tree/master/.agents/skills/wox-plugin-creator) as a built-in AI skill for plugin development. It is always available and cannot be edited or removed from Wox settings. External agents can install the same skill from the repository.

## Why Use Them

The Wox skills package project-specific plugin knowledge so the agent does not have to infer Wox conventions from scratch every time.

For plugin development, this usually means:

- faster scaffolding for Python, Node.js, script plugins, and single-file SDK plugins
- more accurate `plugin.json` authoring
- better guidance for `SettingDefinitions`, validators, dynamic settings, and i18n
- clearer publishing guidance for the Wox store

## Recommended Skill

Start with `wox-plugin-creator`.

It is the main skill for Wox plugin work and covers:

- plugin scaffolding
- SDK usage
- `plugin.json` metadata
- settings and validator patterns
- script-plugin templates
- single-file SDK plugin templates
- publishing to the Wox store

## When To Use It

Use this skill when you want the agent to help with tasks such as:

- creating a new plugin
- converting an idea into a Wox plugin scaffold
- editing `plugin.json`
- implementing settings UI
- adding validators or dynamic settings
- preparing a plugin for store publishing

## Notes

- Using the skill in a conversation is optional. The built-in copy remains available in Wox.
- The skill is most useful when the agent is working inside a Wox-related workspace and can follow the bundled references.
