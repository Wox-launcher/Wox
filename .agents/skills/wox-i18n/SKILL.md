---
name: wox-i18n
description: Add a new Wox language or complete and improve an existing Wox translation. Use for Wox-owned locale registration, translation parity, wording quality, and placeholder-safe updates based on wox.core/resource/lang/en_US.json.
---

# Wox I18n

Use `wox.core/resource/lang/en_US.json` as the authoritative source for keys, meaning, structure, and placeholders. Work on one requested locale at a time unless the user explicitly asks for several.

## Shared rules

- Read the repository `AGENTS.md` and the relevant locale files before editing.
- Preserve every placeholder exactly, including `{name}`, Wox placeholders such as `{wox:selected_text}`, printf verbs such as `%s` and `%02d`, newlines, and other formatting tokens. Reordering placeholders is allowed only when the format mechanism supports it and the target language requires it.
- Resolve ambiguous English by searching for the key's call sites and neighboring translations. Translate the UI meaning, not the key name in isolation.
- Keep product names, commands, file paths, URLs, keyboard shortcuts, and technical identifiers unchanged when they are proper names or literal input.
- Match the tone and terminology already used by the target locale. Prefer natural, concise UI text over literal translation.
- Keep JSON valid, formatted with two-space indentation and a final newline. Preserve English key order; insert missing entries beside their English counterparts instead of reordering unrelated entries.
- Do not put Wox-owned translations in Go maps. Keep them in `wox.core/resource/lang/*.json` and reference them with `i18n:` keys, as required by the repository instructions.
- Do not blanket-translate third-party entries in `store-plugin.json`; their translations belong to their authors. Wox-owned catalogs such as `store-ai-command.json` are in scope when they contain the target locale.

## Choose the workflow

### Add a new language

Use commit `0cfb2a346af4278c5cd385aa649cb9ddb641f461` as a concrete checklist, but follow the current tree when it has evolved:

```powershell
git show --stat 0cfb2a346af4278c5cd385aa649cb9ddb641f461
git show 0cfb2a346af4278c5cd385aa649cb9ddb641f461 -- wox.core/i18n/lang.go store-ai-command.json wox.core/ui/launcher/view/form_table_view_test.go www/docs/.vitepress/theme/components/pluginStore.ts www/docs/development/plugins/specification.md www/docs/zh/development/plugins/specification.md AGENTS.md
```

Then make the smallest complete current-tree change:

1. Confirm the locale code, English language name, and native display name.
2. Create `wox.core/resource/lang/<locale>.json` with every key from `en_US.json`, in the same order, fully translated.
3. Register the `LangCode` and native display name in `wox.core/i18n/lang.go`.
4. Add the locale to Wox-owned locale maps, currently including `store-ai-command.json`, and to website locale mapping where applicable.
5. Update current supported-language documentation and any tests that explicitly enumerate languages or translated labels.
6. Search for the existing locale codes to find registration surfaces added after the reference commit. Update only surfaces whose behavior depends on the complete supported-language list.

Do not create a translated documentation site merely because the app gains a locale; add one only when the user requests and supplies that scope.

### Complete or improve an existing language

1. Run the checker to identify missing, extra, empty, identical-to-English, and placeholder-mismatched values.
2. Add every missing English key and translate it from the English value. Treat identical values as review candidates, not automatic errors: names and technical terms may legitimately remain English.
3. For requested quality improvements, inspect awkward or stale values in related groups and their call sites. Change only wording you can improve confidently.
4. Remove an extra key only after confirming that it is absent from `en_US.json` and unused; otherwise report it for review.
5. Check Wox-owned auxiliary catalogs, especially `store-ai-command.json`, for missing target-locale entries when the request is to complete the language across Wox.

Avoid replacing the whole locale file when a focused insertion or wording edit produces a clear diff.

## Verification

Run from the repository root:

```powershell
python .agents/skills/wox-i18n/scripts/check_locale.py <locale>
git diff --check
```

The checker must pass without missing keys, extra keys, empty values, or placeholder mismatches. Review its identical-value warnings manually.

If language registration or enumerating tests changed, run the focused Go tests from `wox.core`:

```powershell
go test ./i18n ./ui/launcher/view
```

Finish by reviewing the diff for accidental English fallback text, placeholder damage, unrelated rewrites, and omissions from the reference-commit checklist.
