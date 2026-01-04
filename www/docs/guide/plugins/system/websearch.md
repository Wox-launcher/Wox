# WebSearch Plugin

WebSearch lets you run web searches with keyword shortcuts and a default fallback engine.

## What it does

- Keyword-based search engines
- Default fallback search when no keyword matches
- Selected-text search (uses fallback engines)
- Auto-fetch icons for engines

## Quick start

```
Wox Launcher
→ Search with the default engine

g Wox Launcher
→ Search with Google
```

## Settings

Add engines in **WebSearch** settings:

- **Keyword**: shortcut (e.g., `g`)
- **Title**: display name
- **URL(s)**: use `{query}` placeholder
- **Enabled**: show or hide the engine
- **Default**: used when no keyword matches

### URL variables

- `{query}` original text
- `{lower_query}` lowercase
- `{upper_query}` uppercase

### Multi-URL engine

If you provide multiple URLs, Wox opens them in sequence.

## Notes

- Selected-text search only shows fallback engines.
- If nothing shows up, check **Enabled** and **Default** settings.
