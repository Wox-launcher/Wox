# Browser Bookmark Plugin

The Browser Bookmark plugin provides quick search and access to browser bookmarks.

## Features

- **Multi-browser Support**: Supports mainstream browsers like Chrome, Edge
- **Multiple Profiles**: Supports multiple browser configuration files
- **Auto Sync**: Auto-reads browser bookmarks
- **Fuzzy Search**: Supports bookmark name and URL search
- **Favicon Cache**: Caches website icons for faster display
- **MRU Feature**: Remembers frequently used bookmarks, prioritizes display

## Basic Usage

### Search Bookmarks

1. Open Wox
2. Type bookmark name or part of URL directly
3. Press Enter to open bookmark

```
Wox Launcher
→ Search for bookmarks containing "Wox Launcher"

github
→ Search for all bookmarks containing "github"

https://github.com
→ Search for bookmarks matching that URL
```

### Strict Matching

Browser Bookmark plugin uses stricter match scoring (minimum 50 score), avoiding showing too many unrelated results:
- Requires bookmark name or URL to closely match search term
- URL match must be exact partial match

## Supported Browsers

### Windows

- Google Chrome
- Microsoft Edge

**Bookmark Locations**:
- Chrome: `%LOCALAPPDATA%\Google\Chrome\User Data\Default\Bookmarks`
- Edge: `%LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Bookmarks`

Supports multiple profiles: Default, Profile 1, Profile 2, Profile 3

### macOS

- Google Chrome
- Microsoft Edge

**Bookmark Locations**:
- Chrome: `~/Library/Application Support/Google/Chrome/Default/Bookmarks`
- Edge: `~/Library/Application Support/Microsoft Edge/Default\Bookmarks`

Supports multiple profiles: Default, Profile 1, Profile 2, Profile 3

### Linux

- Google Chrome
- Microsoft Edge

**Bookmark Locations**:
- Chrome: `~/.config/google-chrome/Default/Bookmarks`
- Edge: `~/.config/microsoft-edge/Default\Bookmarks`

Supports multiple profiles: Default, Profile 1, Profile 2, Profile 3

### Safari

Currently doesn't support Safari bookmarks. If needed, you can submit issue feedback.

## Bookmark Icons

### Favicon Cache

Browser Bookmark plugin automatically prefetches and caches website icons:
- Prefetches on startup in background
- Avoids real-time loading affecting performance
- Uses cache files for fast display

### Icon Overlay

When displaying bookmarks:
- Base icon is bookmark icon
- If cached favicon exists, overlays on base icon
- Overlay position: bottom-right corner, size 60%

## MRU Feature

### Recent Usage

Browser Bookmark plugin supports MRU (Most Recently Used) functionality:
- Frequently accessed bookmarks are prioritized in results
- Smart sorting based on MRU data
- Quick access to frequently used bookmarks

### Usage Frequency

MRU tracks:
- Bookmark access count
- Last access time
- Calculates recommendation score

## Search Tips

### Bookmark Name Search

Use full or partial bookmark name:

```
Wox GitHub
→ Wox Launcher GitHub repository

Python Docs
→ Python official documentation
```

### URL Search

Type part of URL to match bookmarks:

```
github.com/wox
→ Wox Launcher GitHub repository

stackoverflow.com
→ StackOverflow bookmarks
```

### Fuzzy Matching

Still finds bookmarks even if not perfectly accurate:
- Supports typos
- Supports partial matches
- Intelligent scoring system

## FAQ

### Why can't I find a certain bookmark?

1. **Check browser**: Confirm bookmark is in supported browser
2. **Check configuration file**: Some bookmarks may be in non-default profiles
3. **Wait for sync**: When browser is syncing bookmarks, Wox may not be able to read them
4. **Restart Wox**: Sometimes requires restart to reload bookmarks

### How do I add new bookmarks?

Add bookmarks normally in browser, Wox will automatically read them:
- Ensure browser has closed bookmark file (some browsers require this)
- Restart Wox or wait for auto-reload

### Does it support other browsers?

Currently only supports Chrome and Edge. If you need support for other browsers:
- Submit GitHub Issue feedback
- Or consider using third-party plugins

### Favicon not displaying?

1. **Wait for prefetch**: On first startup, needs to prefetch, may take a few seconds
2. **Check network**: Favicon prefetch requires network connection
3. **Check cache**: View Wox cache directory, confirm favicon files exist

### Duplicate bookmarks?

Plugin automatically deduplicates:
- Bookmarks with same name and URL keep only one
- Deduplication happens automatically during load
- Doesn't affect original bookmark files

## Configuration Options

Browser Bookmark plugin currently has no user-configurable options. All settings are automatic.

### Auto Reload

Plugin automatically reloads bookmarks in following situations:
- When browser bookmark file changes (via file watching)
- When restarting Wox

### Manual Reload

If you need to reload bookmarks immediately:
1. Restart Wox
2. Or close browser to save bookmarks then open

## Usage Scenarios

### Quick Access to Common Websites

```
github
reddit
hacker news
```

### Open Work-Related Bookmarks

```
company docs
project tracker
jira
```

### Developer Tools

```
stackoverflow
mdn docs
can i use
```

## Related Plugins

- [WebSearch](websearch.md) - Web search
- [Application](application.md) - Launch browser
- [Clipboard](clipboard.md) - Clipboard history, useful for pasting URLs

## Technical Notes

Browser Bookmark plugin:

**Bookmark Reading**:
- Directly reads browser's bookmark JSON files
- Supports multiple browser profiles
- Auto-deduplicates

**Icon Caching**:
- Prefetches favicon on startup in background
- Uses separate cache files
- Supports icon overlay display

**Search Matching**:
- Bookmark names: Fuzzy matching
- URLs: Exact partial matching
- Minimum match score is 50, filters low-score results

**MRU**:
- Uses global MRU system
- Sorts by access frequency and time
- Supports quick access to frequently used bookmarks
