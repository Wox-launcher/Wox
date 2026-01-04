# Emoji Plugin

The Emoji plugin provides fast emoji search and insertion features.

## Features

- **Massive Emoji Library**: Contains thousands of emoji
- **Multi-language Support**: Emoji names and categories in multiple languages
- **Grid Layout**: Displayed in grid format for easy browsing
- **AI Matching**: Optional AI-assisted search, supports natural language descriptions
- **Usage Statistics**: Records emoji usage frequency, prioritizes commonly used
- **Quick Copy**: One-click copy to clipboard

## Basic Usage

### Search Emoji

1. Open Wox
2. Type emoji name or keyword directly
3. Browse search results (grid layout)

```
smile
→ Shows all emojis related to "smile"

cat
→ Shows all emojis related to "cat"

heart
→ Shows all emojis related to "heart"
```

### Use Trigger Keyword

You can also use trigger keyword `emoji`:

```
emoji smile
→ Shows emojis related to "smile"
```

## Display Format

### Grid Layout

Emoji plugin uses grid layout to display search results:
- Displays 12 emojis per row
- Each emoji shown as icon, no title
- Easy to browse and select

### Default Behavior

- Emoji titles not displayed
- Only emoji icons shown
- Hover to view details (if supported)

## AI Matching Feature

### Enable AI Matching

1. Open Wox settings
2. Find **Emoji** plugin
3. Check **Enable AI Matching**
4. Choose AI model (if needed)

### AI Matching Benefits

After enabling AI matching, you can use natural language descriptions to find emojis:

```
一个开心的笑脸
→ 😄

红色的爱心
→ ❤️

哭泣的脸
→ 😢

绿色的勾
→ ✅

太阳和云
→ ⛅
```

### AI Model Selection

You can choose different AI models in plugin settings:
- Different models may have different matching effects
- Recommend using default model
- If matching effect isn't ideal, try other models

## Emoji Categories

### Supported Categories

Emoji plugin includes multiple emoji categories:

| Category | Examples |
|-----------|-----------|
| **Face Expressions** | 😀 😂 😢 😡 |
| **Gestures** | 👍 👎 👋 ✌️ |
| **People** | 👨 👩 👶 👵 |
| **Animals** | 🐱 🐶 🐼 🦊 |
| **Food** | 🍎 🍔 🍕 🍦 |
| **Activities** | ⚽ 🎮 🎵 🚗 |
| **Travel** | 🚗 ✈️ 🚢 🏰 |
| **Objects** | 💻 📱 💡 📷 |
| **Symbols** | ❤️ ⭐ 🔥 ✅ |
| **Flags** | 🇨🇳 🇺🇸 🇬🇧 🇯🇵 |

### Multi-language Support

Emoji names and categories support multiple languages:
- Simplified Chinese
- English
- Other languages (depending on version)

Plugin auto-displays corresponding language emoji names based on system language.

## Search Tips

### Keyword Search

Use part of emoji name for search:

```
face
→ Shows all face expressions

love
→ Shows all love-related emojis

color
→ Shows all colored emojis
```

### Fuzzy Matching

Still finds related emojis even if not exactly accurate:

```
smil
→ May match: 😄 😊 😃

heart
→ May match: ❤️ 💕 💖
```

### Combined Search

Combine multiple keywords to narrow scope:

```
face happy
→ Happy face expressions

animal cat
→ Cats in animals
```

## Usage Statistics

### Usage Frequency

Emoji plugin records emoji usage frequency:
- More frequently used emojis get higher priority
- Common emojis appear earlier in results
- Newly used emojis quickly increase priority

### View Statistics

Usage frequency is recorded in background, not directly displayed. Search auto-prioritizes commonly used emojis.

## Configuration Options

### AI Matching Settings

Configure in plugin settings:

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
