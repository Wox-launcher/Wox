---
title: "Did You Know: Wox Can Translate Selected Text Silently"
description: Combine Query Hotkeys with AI Commands to translate selected text and paste the result back without opening a confirmation window.
date: 2026-05-16
---

# Did You Know: Wox Can Translate Selected Text Silently

AI Commands are useful from the launcher, but they become much faster when you pair them with Query Hotkeys. For repeated text work, you can select text in any app, press one hotkey, let AI translate it, and have Wox paste the final answer back into the original selection.

<video src="/videos/ai_command_run_paste_query_hotkey.mp4" controls muted loop playsinline style="width: 100%; border-radius: 8px;"></video>

The key is to make the AI command's default action explicit. In the AI Commands plugin settings, create or edit a translation command:

| AI Command field | Example value |
| --- | --- |
| Name | `Translate to Chinese` |
| Command | `translate` |
| Prompt | `Translate the following text to Chinese. Return only the translated text: %s` |
| Default Action | `Run And Paste` |

`Run And Paste` waits for the final AI answer, writes that final text to the clipboard, then simulates paste back into the active window that was captured before Wox opened. It does not paste partial streaming output.

Next, create a Query Hotkey, choose the **Silent Run** preset, and bind it to the command:

| Query Hotkey field | Example value |
| --- | --- |
| Preset | `Silent Run` |
| Hotkey | Any available shortcut, such as `ctrl+shift+t` |
| Query | `ai translate {wox:selected_text}` |
| Optional tweaks | Switch to `Custom` only if you need to override display behavior |

The Silent Run preset already enables silent execution, so in many cases you only need to fill in the hotkey and query. Silent execution does not add hidden paste behavior on its own. In this setup, it simply runs the AI command's configured default action, and that default action is `Run And Paste`.

The result is a compact workflow:

1. Select text in another app.
2. Press the Query Hotkey.
3. Wox sends the selected text to the AI command.
4. Wox shows a small thinking indicator near the pointer.
5. When the AI answer finishes, Wox replaces the selected text with the translated result.

This pattern also works for other text transformations: grammar correction, tone rewrite, summarization, or format cleanup. Use `Run` when you want to inspect the output in Wox, and use `Run And Paste` only when the command is safe enough to replace the selected text directly.
