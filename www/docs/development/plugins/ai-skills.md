# AI Skills For Plugin Development

If you use Codex or another compatible agent, install the Wox skills published in [`wox.core/resource/ai/skills`](https://github.com/Wox-launcher/Wox/tree/master/wox.core/resource/ai/skills) to speed up plugin development.

## Why Use Them

The Wox skills package project-specific plugin knowledge so the agent does not have to infer Wox conventions from scratch every time.

For plugin development, this usually means:

- faster scaffolding for Python, Node.js, and script plugins
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

- The skills are optional. You can still develop plugins directly with the SDK and docs.
- The skill is most useful when the agent is working inside a Wox-related workspace and can follow the bundled references.
