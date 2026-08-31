---
name: wox-changelog-update
description: Update Wox CHANGELOG.md based on commits since the last release. Use when the user asks to "update changelog", "write release notes", or "summarize changes since last release" and the output must follow the repository's existing changelog format.
---

# Wox Update Changelog

## Overview

Update `CHANGELOG.md` from the latest released version to `HEAD` and keep wording, section order, and markdown style consistent with existing entries.

## Workflow

1. Identify the release boundary.

- Read the top of `CHANGELOG.md` and detect the target section (usually newest version at the top).
- Detect the last released tag with `git tag --sort=-creatordate`.
- Use commit range `last_release_tag..HEAD` by default.
- If changelog heading and git tag disagree, prefer changelog context and state the assumption.

2. Collect candidate changes.

- Run `git log --oneline --no-merges <range>`.
- Open key commits with `git show --stat --oneline <sha>` to classify user-facing impact.
- Ignore pure build/version bump commits unless they change user-visible behavior.
- Collect store catalog additions separately from app/core commits. Compare `store-plugin.json` and `store-theme.json` at `last_release_tag` vs `HEAD`. A store item is new only when its `Id` (plugin) or `ThemeId` (theme) is absent from the last release file. Ignore version bumps, field edits, and other updates to existing store entries. Ignore bundled script plugin version bumps.
- Do not put third-party store catalog changes in `Add`, `Improve`, or `Fix`. List only newly added store plugins and themes in `Store`.

3. Classify into changelog buckets.

- `Add`: major, clearly new user-visible capabilities, workflows, plugins, or standalone feature areas.
- `Improve`: behavioral or UX improvements without new core capability.
- `Fix`: user-facing bug fixes/regressions.
- `Store`: plugins and themes newly listed in the Wox store during this range.
- Omit the `Add` section when the release does not contain a substantial new feature. Prefer `Improve` for additions that extend an existing feature area rather than creating a new user workflow.
- Omit the `Store` section when no new store plugin or theme IDs appeared in range.
- Treat platform-specific implementations, provider additions, runtime dependency checks, searchable metadata, theme overrides, and similar support for existing systems as `Improve` unless the commit introduces a large new user-facing feature.
- Exclude internal refactors/tests/docs/chore unless directly user-visible.
- Exclude tiny UI-only polish by default (for example spacing, alignment, minor color/wording tweaks) unless it fixes a functional UX issue or the user explicitly asks to include small UI changes.

4. Write changelog entries in repository style.

- Preserve header pattern exactly (for example: `## v2.0.1 -`).
- Add a short highlight paragraph directly below every release heading you create or update, before screenshots and `Add`/`Improve`/`Fix`/`Store` sections. Follow the `v2.2.0` style: one concise, user-facing sentence or short paragraph that calls out the single biggest release highlight.
- Keep section order: `Add`, `Improve`, `Fix`, `Store`.
- Use bullet nesting style already used in file.
- Keep wording concise, user-facing, and factual.
- Match wording to the bucket. `Improve` entries should say "Improve", "Expand", "Support", or similar, not "Add", unless the entry is intentionally describing a small added option inside an improvement.
- For new `Add` features, explain what the feature is for and why a user would use it. Do not reduce major features to one terse implementation phrase.
- Keep the same feature in one bullet whenever possible. For example, combine Screenshot scrolling capture, pinning, and plugin API changes into one `[`Screenshot`]` bullet instead of splitting them into separate bullets.
- If a new feature needs screenshots but the images are not available yet, leave clearly named screenshot placeholder image lines in the same bullet so the screenshots can be added later.
- Prefer plugin/module prefix when clear, e.g. ``[`Shell`]`` or ``[`Clipboard`]``.
- Keep issue references in existing style, e.g. `#4339`.
- Keep existing screenshots. Add new screenshot lines when screenshots already exist, or when a user explicitly asks to reserve screenshot positions for upcoming images.
- Write `Store` as two nested groups, omitting an empty group: `Plugin` then `Theme`. Each item is ``[`Name`] Description``. Resolve `i18n:` names and descriptions from `I18n.en_US` (`plugin_name`, `plugin_desc` or `plugin_description`). Use `ThemeName` and `Description` for themes. Do not add store screenshots, download URLs, or version numbers.

  ```markdown
  - Store
    - Plugin
      - [`Color Picker`] Wox plugin to pick colors
      - [`Strava`] A plugin to interact with Strava workouts
    - Theme
      - [`Wox Dracula`] Wox theme inspired by the Dracula color scheme
  ```

5. Validate before finishing.

- Ensure no duplicate bullets.
- Ensure every `Add`/`Improve`/`Fix` bullet maps to at least one commit in range.
- Ensure every `Store` bullet maps to a plugin `Id` or theme `ThemeId` that is new since `last_release_tag`.
- Ensure every release section you create or update has a biggest-highlight paragraph under the version heading.
- Ensure markdown renders cleanly and section spacing matches nearby versions.
- Avoid rewriting old release sections unless explicitly requested.
- If a commit set only contains tiny UI-only polish, keep it out of changelog by default.

## Command Reference

```bash
git tag --sort=-creatordate | head -n 20
sed -n '1,120p' CHANGELOG.md
git log --oneline --no-merges <last_tag>..HEAD
git show --stat --oneline <sha>
git show <last_tag>:store-plugin.json
git show <last_tag>:store-theme.json
git diff -- CHANGELOG.md
```

Compare store catalogs by ID, not by file diff hunks. Example:

```bash
python -c "
import json, subprocess, sys
tag = sys.argv[1]
def load(rev, path):
    return json.loads(subprocess.check_output(['git', 'show', f'{rev}:{path}']))
old_p = {p['Id'] for p in load(tag, 'store-plugin.json')}
new_p = json.load(open('store-plugin.json', encoding='utf-8'))
for p in new_p:
    if p['Id'] not in old_p:
        print('plugin', p.get('Name'), p.get('Description'))
old_t = {t['ThemeId'] for t in load(tag, 'store-theme.json')}
new_t = json.load(open('store-theme.json', encoding='utf-8'))
for t in new_t:
    if t['ThemeId'] not in old_t:
        print('theme', t.get('ThemeName'), t.get('Description'))
" <last_tag>
```

## Output Rules

- Edit `CHANGELOG.md` directly.
- Keep final response short: what section was updated and what categories were changed, including `Store` plugin/theme counts when present.
- If commit intent is ambiguous, state the assumption briefly in the final response.
