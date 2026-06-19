---
name: wox-update-changelog
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

3. Classify into changelog buckets.
- `Add`: new user-visible features/settings/components.
- `Improve`: behavioral or UX improvements without new core capability.
- `Fix`: user-facing bug fixes/regressions.
- Exclude internal refactors/tests/docs/chore unless directly user-visible.
- Exclude tiny UI-only polish by default (for example spacing, alignment, minor color/wording tweaks) unless it fixes a functional UX issue or the user explicitly asks to include small UI changes.

4. Write changelog entries in repository style.
- Preserve header pattern exactly (for example: `## v2.0.1 -`).
- Keep section order: `Add`, `Improve`, `Fix`.
- Use bullet nesting style already used in file.
- Keep wording concise, user-facing, and factual.
- Prefer plugin/module prefix when clear, e.g. ``[`Shell`]`` or ``[`Clipboard`]``.
- Keep issue references in existing style, e.g. `#4339`.
- Keep existing screenshots and add new image lines only when already available in repo.

5. Validate before finishing.
- Ensure no duplicate bullets.
- Ensure every bullet maps to at least one commit in range.
- Ensure markdown renders cleanly and section spacing matches nearby versions.
- Avoid rewriting old release sections unless explicitly requested.
- If a commit set only contains tiny UI-only polish, keep it out of changelog by default.

## Command Reference

```bash
git tag --sort=-creatordate | head -n 20
sed -n '1,120p' CHANGELOG.md
git log --oneline --no-merges <last_tag>..HEAD
git show --stat --oneline <sha>
git diff -- CHANGELOG.md
```

## Output Rules

- Edit `CHANGELOG.md` directly.
- Keep final response short: what section was updated and what categories were changed.
- If commit intent is ambiguous, state the assumption briefly in the final response.
