# Clip Query

Clip Query is a feature that allows users to select a piece of text and then invoke Wox to perform quick operations on the selected information. This is similar to the [Selection
Query](selection_query.md) feature, but instead of being triggered by a keyboard shortcut, Clip Query is triggered by clicking an icon, making it more convenient for mouse
operations.

## Setting up Clip Query on macOS

![Clip Query On Macos](https://raw.githubusercontent.com/Wox-launcher/Wox/master/docs/images/popclip.png)

On macOS, you can use the [PopClip](https://www.popclip.app/) software to enable the Clip Query feature. After installing PopClip, you can install
the [Popclip snippets extension](https://www.popclip.app/dev/snippets) by
selecting the following snippet:

```
#popclip Wox Query
name: Wox
icon: square filled W
url: wox://query?q={popclip text}
```

This will allow you to use Wox to perform operations on the selected text. You can change the query URL to match your specific needs.

## Setting up Clip Query on Windows

On Windows, we recommend installing the [SnipDo](https://snipdo-app.com/) software to enable the Clip Query feature. The plugin for SnipDo is currently under development.