# Emoji Plugin

Use `emoji` to search and copy emoji from Wox.

## Quick Start

```text
emoji smile
emoji check
emoji heart
emoji flag
```

Results use a grid layout so you can scan many choices quickly. Press `Enter` to copy the selected emoji.

![Emoji plugin grid results](/images/system-plugin-emoji.png)

## AI Matching

AI matching is optional. When enabled, Wox can match descriptive phrases that are not part of the built-in emoji names.

1. Configure an AI provider in [AI Settings](../../ai/settings.md).
2. Open **Settings -> Plugins -> Emoji**.
3. Enable AI matching and choose the model.

Examples:

```text
emoji green success mark
emoji red warning
emoji happy face
emoji cloudy weather
```

AI matching sends your query text to the selected model. Keep it disabled if you want emoji search to stay fully local.

## Ordering

Frequently used emoji are promoted over time. If a common emoji is not first the first time you search, copy it normally; Wox will learn from usage.

## Actions

Open the Action Panel for additional options such as copying a larger emoji image, adding a keyword, or removing an item from frequent results when available.

- **Enable AI Matching**: Whether to use AI-assisted search
- **AI Model**: Choose AI model to use

**Recommendations**:
- Daily use: Can disable AI matching for better speed
- Need natural language search: Enable AI matching

## FAQ

### Why can't I find a certain emoji?

1. **Check spelling**: Ensure emoji name is spelled correctly
2. **Use keywords**: Try using more general keywords
3. **Enable AI Matching**: If you know description but not name, enable AI matching
4. **Check language**: Some emojis may have different names in other languages

### AI matching inaccurate?

1. **Try different descriptions**: Use more specific or different description words
2. **Change AI model**: Try using different AI model
3. **Disable AI Matching**: Use traditional keyword search

### Emoji not displaying?

1. **Check system support**: Confirm OS supports that emoji
2. **Update system**: Some new emojis may need updated system version
3. **Check font**: Confirm system font supports that emoji

### How to quickly insert multiple emojis?

1. Search and copy first emoji
2. Paste to target location
3. Search and copy second emoji
4. Continue pasting

## Usage Scenarios

### Social Media

Quickly insert emojis when sending messages:

```
happy
→ 😊

laughing
→ 😂

thumbs up
→ 👍
```

### Document Writing

Add emojis to enhance expression in documents:

```
note
→ 📝

warning
→ ⚠️

important
→ ⭐
```

### Task Management

Mark task status:

```
todo
→ 📝

done
→ ✅

in progress
→ 🚧
```

## Related Plugins

- [Clipboard](clipboard.md) - Clipboard history, useful for pasting multiple emojis
- [WebSearch](websearch.md) - Search emoji meanings or usage
- [Converter](converter.md) - Other types of content conversion

## Technical Notes

Emoji plugin uses built-in emoji database:
- Data embedded in plugin
- Supports thousands of standard emojis
- Supports multiple languages
- Grid layout controlled by plugin feature

AI matching feature:
- Uses AI model to process natural language descriptions
- Returns matching emoji list
- May require network connection (depending on AI model)
