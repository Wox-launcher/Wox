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
- Ignore third-party plugin additions or updates, including plugin store manifest updates and bundled script plugin version bumps. Do not mention them in `CHANGELOG.md`.

3. Classify into changelog buckets.

- `Add`: major, clearly new user-visible capabilities, workflows, plugins, or standalone feature areas.
- `Improve`: behavioral or UX improvements without new core capability.
- `Fix`: user-facing bug fixes/regressions.
- Omit the `Add` section when the release does not contain a substantial new feature. Prefer `Improve` for additions that extend an existing feature area rather than creating a new user workflow.
- Treat platform-specific implementations, provider additions, runtime dependency checks, searchable metadata, theme overrides, and similar support for existing systems as `Improve` unless the commit introduces a large new user-facing feature.
- Exclude internal refactors/tests/docs/chore unless directly user-visible.
- Exclude third-party plugin additions and updates even when they are user-visible in the plugin store; the Wox changelog should cover Wox app/core behavior, not plugin catalog changes.
- Exclude tiny UI-only polish by default (for example spacing, alignment, minor color/wording tweaks) unless it fixes a functional UX issue or the user explicitly asks to include small UI changes.

4. Write changelog entries in repository style.

- Preserve header pattern exactly (for example: `## v2.0.1 -`).
- Add a short highlight paragraph directly below every release heading you create or update, before screenshots and `Add`/`Improve`/`Fix` sections. Follow the `v2.2.0` style: one concise, user-facing sentence or short paragraph that calls out the single biggest release highlight.
- Keep section order: `Add`, `Improve`, `Fix`.
- Use bullet nesting style already used in file.
- Keep wording concise, user-facing, and factual.
- Match wording to the bucket. `Improve` entries should say "Improve", "Expand", "Support", or similar, not "Add", unless the entry is intentionally describing a small added option inside an improvement.
- For new `Add` features, explain what the feature is for and why a user would use it. Do not reduce major features to one terse implementation phrase.
- Keep the same feature in one bullet whenever possible. For example, combine Screenshot scrolling capture, pinning, and plugin API changes into one `[`Screenshot`]` bullet instead of splitting them into separate bullets.
- If a new feature needs screenshots but the images are not available yet, leave clearly named screenshot placeholder image lines in the same bullet so the screenshots can be added later.
- Prefer plugin/module prefix when clear, e.g. ``[`Shell`]`` or ``[`Clipboard`]``.
- Keep issue references in existing style, e.g. `#4339`.
- Keep existing screenshots. Add new screenshot lines when screenshots already exist, or when a user explicitly asks to reserve screenshot positions for upcoming images.

5. Validate before finishing.

- Ensure no duplicate bullets.
- Ensure every bullet maps to at least one commit in range.
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
git diff -- CHANGELOG.md
```

## Output Rules

- Edit `CHANGELOG.md` directly.
- Keep final response short: what section was updated and what categories were changed.
- If commit intent is ambiguous, state the assumption briefly in the final response.
