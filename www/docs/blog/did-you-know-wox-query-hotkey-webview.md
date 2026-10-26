---
title: Open Your Favorite Web Panels with a Shortcut
description: Bind a website to a shortcut and check it in a compact Wox panel, with optional access from the tray.
date: 2026-05-16
category: automation
cover: /images/blog/webview-hotkey.webp
coverAlt: A compact web panel with a globe and shortcut keys
---

# Open Your Favorite Web Panels with a Shortcut

Checking a status page or glancing at a timeline may only take a few seconds. Opening the browser, finding the right tab, and returning to your work can take more attention than the check itself.

WebSearch's WebView option puts a saved website in a compact panel that you can open with a shortcut. This example starts with a working query for X, then turns it into a reusable shortcut. The feature is available on Windows, macOS and Linux; Linux requires WebKitGTK 4.1 or 4.0.

## Check that the site opens in Wox

In **Settings → Plugins → Web Search**, add a title, trigger keyword and URL, then enable **Open in WebView**. For example:

| Keyword | Site | Query |
| --- | --- | --- |
| `x` | `https://x.com` | `x ` |
| `ig` | `https://www.instagram.com` | `ig ` |

The space after the trigger is required, including in saved queries. Open Wox and enter `x ` to check that the page loads. If it requires a login, follow the site's prompts.

![Wox WebView opened by a query hotkey](/images/did-you-know-wox-webview-hotkey.png)

For a status page or dashboard of your own, open **Settings → Plugins → Web Search** and add a site keyword and full URL. With `status` as the keyword, the query becomes `status `. Enable **Open in WebView** for the entry.

Testing the query manually confirms that the site configuration and page work before you attach a shortcut to them.

## Bind the query to a shortcut

Add an entry in **Settings → General → Query Hotkeys**:

| Field | Example value |
| --- | --- |
| Preset | `Preview Query` |
| Hotkey | An unused combination, such as `ctrl+shift+x` |
| Query | `x ` |

**Preview Query** hides the query box and toolbar, leaving most of the panel for the page. Save the entry and press the shortcut to open it directly, without typing `x ` again.

> Screenshot needed: The Query Hotkey editor with `Preview Query` selected, showing the shortcut and `x ` query so readers can match the three fields above.

Try the default size first. If the page feels cramped or scrolls horizontally, adjust the query hotkey's width, position, or other display options. A width that works for a timeline may not suit a full dashboard.

Use the same setup for other sites, replacing the query with `ig ` or your own site keyword.

## Add a tray entry if you prefer clicking

To open the same panel with the mouse, add an entry in **Settings → General → Tray Queries**:

| Field | Example value |
| --- | --- |
| Icon | An icon you will recognize for the site |
| Query | `x ` |
| Hide Query Box | Enabled |
| Hide Toolbar | Enabled |
| Width | Try `420`, then adjust for the page |
| Max Result Count | `5` |

Clicking the tray or menu-bar icon opens Wox near the tray and runs the query. It uses the same saved site, so you do not need to configure the website again.

![Wox Tray Query opening a WebView panel](/images/did-you-know-wox-webview-tray-query.png)

When you want more room to read, need several pages side by side, or find the site awkward to use in a small panel, choose **Open in Browser** from the result actions and continue there.

![Wox WebView action to open the page in a browser](/images/did-you-know-wox-webview-open-browser.png)

If a particular site does not display or sign in correctly within the panel, use Open in Browser.
