---
title: Translate and Replace Selected Text with One Shortcut
description: Combine an AI command with a shortcut to translate selected text and paste the result back in place.
date: 2026-05-16
category: automation
cover: /images/blog/silent-translation.webp
coverDark: /images/blog/silent-translation-dark.webp
coverAlt: An arrow transforming text between two panels with a shortcut key
---

# Translate and Replace Selected Text with One Shortcut

Translating a paragraph often means copying it, switching to a translation tool, then copying the result back. If you only want to replace one passage in your notes, Wox can handle those handoffs.

This example combines an **AI Command** with a **Query Hotkey** to translate text into Chinese. Select a passage in an editable document, press the shortcut, and wait for the translated text to be pasted back into the original app. Start with a short sentence in a note so you can see the whole flow.

<video src="/videos/ai_command_run_paste_query_hotkey.mp4" controls muted loop playsinline style="width: 100%; border-radius: 8px;"></video>

## Define the translation and where its result goes

First, configure a working provider and model in **Settings → AI**. See [AI Settings](/guide/ai/settings) if you have not set one up yet.

Then open **Settings → Plugins → AI Commands** and create or edit a command:

| Field | Example value |
| --- | --- |
| Name | `Translate to Chinese` |
| Command | `translate` |
| Model | A model you have already configured |
| Prompt | `Translate the following text to Chinese. Return only the translated text: %s` |
| Default Action | `Run And Paste` |

`translate` is the command keyword used in the query. `%s` marks where the input text belongs in the prompt. Asking for only the translated text helps keep explanations and introductory phrases out of what gets pasted into your note.

> Screenshot needed: The AI Command editor showing the `translate` command, the translation prompt containing `%s`, the selected model, and `Run And Paste` as the default action. Do not include API keys.

**Run And Paste** determines where the answer goes. Wox waits for the complete answer, writes the final text to the clipboard, and pastes into the active window recorded before the command was triggered. It does not insert partial output as the model generates it.

## Have the shortcut supply the selected text

Add an entry in **Settings → General → Query Hotkeys**:

| Field | Example value |
| --- | --- |
| Preset | `Silent Run` |
| Hotkey | An unused combination, such as `ctrl+shift+t` |
| Query | `ai translate {wox:selected_text}` |

`{wox:selected_text}` supplies the current selection when you trigger the shortcut. The same shortcut can handle a different paragraph each time without you editing the query.

**Silent Run** executes the default action directly when the query has a single result. The two settings have separate jobs: the AI command's default action handles pasting, while the hotkey preset skips confirmation in the launcher.

> Screenshot needed: The Query Hotkey editor with `Silent Run` selected, showing the recorded shortcut and the complete query `ai translate {wox:selected_text}`.

## Try it in an editable note

Type a short English sentence into a note, such as `Please send me the updated draft tomorrow.` Select the whole sentence and press your shortcut.

Wox sends the selection to the chosen model and shows a small waiting indicator near the pointer. Once the full answer arrives, it returns to the original app and pastes. With the original selection and editing position intact, the translation replaces that sentence. The wording will vary by model.

The destination must accept pasted text. Selecting words on a read-only web page will not make the page editable. While waiting, avoid moving the caret or changing the selection in the original app, so the result goes where you expect.

To review translations first, change the AI command's default action to **Run**, then use the output after checking it. Switch back to **Run And Paste** when the prompt gives you the results you want. The same setup works for grammar corrections or tone adjustments with a different prompt.
