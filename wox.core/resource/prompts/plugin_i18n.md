# Wox Plugin Internationalization (i18n) Guide

This document guides AI agents and developers on how to implement multi-language support in Wox plugins.

## Core Principles

1.  **Raw Strings Only**: The Wox API returns the _raw_ string from the resource.
2.  **Manual Formatting**: The plugin code is responsible for formatting (replacing placeholders like `%s` or `{name}`).

## Defining Translations

There are two ways to define translations. **Method 1 (Inline) is recommended** for simplicity.

### Method 1: Inline in `plugin.json` (Recommended)

Define translations directly in your manifest. Best for most plugins.

```json
{
  "Name": "i18n:plugin_name",
  "Description": "i18n:plugin_desc",
  "I18n": {
    "en_US": {
      "plugin_name": "My Plugin",
      "plugin_desc": "A useful plugin",
      "hello": "Hello"
    },
    "zh_CN": {
      "plugin_name": "我的插件",
      "plugin_desc": "一个有用的插件",
      "hello": "你好"
    }
  }
}
```

### Method 2: Translation Files (`lang/` directory)

For plugins with a large number of strings. Store files in a `lang` directory at the plugin root.

Structure:

```text
my-plugin/
  plugin.json
  lang/
    en_US.json
    zh_CN.json
```

**en_US.json**:

```json
{
  "hello": "Hello",
  "error": {
    "file": "File not found: %s"
  }
}
```

## Usage in Code

### 1. Get Translation

Use the `GetTranslation(ctx, key)` API method.

**Node.js**:

```typescript
const raw = await this.api.GetTranslation(ctx, "hello"); // Returns "Hello" or "你好"
```

**Python**:

```python
raw = await self.api.get_translation(ctx, "hello")
```

### 2. Format String (CRITICAL)

You **MUST** handle parameter substitution in your code.

**Node.js**:

```typescript
// Assuming json: { "greet": "Hello %s" }
const raw = await this.api.GetTranslation(ctx, "greet");
const message = raw.replace("%s", "World");
```

**Python**:

```python
# Assuming json: { "greet": "Hello {}" }
raw = await self.api.get_translation(ctx, "greet")
message = raw.format("World")
```

## Using `i18n:` Prefix (Implicit Translation)

The easiest way to specific localized strings is using the `i18n:` prefix. Wox will automatically translate these strings before displaying them in the UI.

### 1. In `plugin.json` (Manifest)

Use this for static metadata like Name and Description.

```json
{
  "Name": "i18n:plugin_name", // Looks up "plugin_name" key
  "Description": "i18n:plugin_desc"
}
```

### 2. In Code

You can use the prefix directly (E.g. Title, SubTitle, Action Name, etc.). This avoids the need to call `GetTranslation`.

**Node.js**:

```typescript
return [{
  Title: "i18n:hello", // Wox translates this to "Hello" or "你好" automatically
  SubTitle: "i18n:plugin_desc",
  ...
}]
```

**Python**:

```python
return [Result(
    title="i18n:hello",
    sub_title="i18n:plugin_desc",
    ...
)]
```
