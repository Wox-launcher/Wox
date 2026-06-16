---
name: wox-plugin-submit2store
description: Submit the current Wox plugin repository to the official Wox store by ensuring the user has a reusable fork of Wox-launcher/Wox, checking whether the plugin ID already exists in Wox's store-plugin.json, adding a new store entry when missing, and preparing a pull request to Wox-launcher/Wox. Use when the user wants to publish a Wox plugin to the store, verify whether it has already been listed, or have Codex run the store submission PR workflow.
---

# Wox Plugin Submit2store

## Overview

Submit the current plugin repository to the Wox store through a PR against `Wox-launcher/Wox`. Ensure a reusable fork exists first, read local plugin metadata, refuse duplicate submissions, and only modify `store-plugin.json` when the plugin ID is not already present.

## Workflow

1. Ensure the user already has a fork of `Wox-launcher/Wox`, or create one.
2. Read the current plugin repository before touching the store repo.
3. Validate that the plugin has enough public metadata and release assets for a store entry.
4. Clone the user's fork into a temporary directory and add `upstream` for `https://github.com/Wox-launcher/Wox`.
5. Check upstream `store-plugin.json` for the current plugin ID.
6. Stop and tell the user not to submit when the ID already exists.
7. Add the new store entry when the ID is absent.
8. Commit on a branch in the fork and open a PR to `Wox-launcher/Wox`.

## Ensure A Fork Exists First

Treat fork setup as the first gate. Do not clone or edit a working copy until one of these is true:

- the user already has a fork of `Wox-launcher/Wox`
- the current authenticated GitHub user can create one now

Use this order:

1. Check whether the user already has a fork and reuse it when present.
2. If `gh` is available and authenticated, create the fork from the CLI.
3. If `gh` is unavailable, unauthenticated, or fails, send the user to the web fork flow and continue only after the fork exists.

For CLI-driven fork handling:

- prefer reusing an existing fork instead of creating a new one
- if a fork is missing, create it with GitHub CLI
- after the fork exists, treat the fork as `origin` and the official repo as `upstream`

For web fallback:

- direct the user to `https://github.com/Wox-launcher/Wox/fork`
- wait for the user to confirm the fork is ready
- continue by cloning the fork repository, not the upstream repository

When the user has direct write access to `Wox-launcher/Wox`, a fork is not required. Otherwise, do not assume direct push access.

## Read Local Metadata First

Read at least these files before preparing the store entry:

- `plugin.json`
- `package.json` when version information needs confirmation
- local screenshots or icon files when URLs need to be derived
- git remotes to determine the GitHub repository URL

Extract or derive these values:

- plugin `Id`
- `Name`
- `Author`
- `Version`
- `MinWoxVersion`
- `Runtime`
- `Description`
- repository `Website`
- `.wox` download asset URL
- public icon URL
- optional screenshot URLs
- supported operating systems

Stop and ask the user to fix the repository first when any of these conditions hold:

- `plugin.json` still contains template placeholders such as `{{.Id}}` or `{{.Name}}`
- the repository URL cannot be determined
- the plugin does not have a public release asset that matches the installable `.wox` package
- the icon or screenshot is only local and no stable public URL can be formed

## Clone The Fork And Check For Duplicates

Clone the user's fork into a temporary directory instead of modifying the current plugin workspace. Add the official repository as `upstream` so the branch and PR target remain `Wox-launcher/Wox`.

Inspect `store-plugin.json` from the current upstream default branch and search by the exact plugin ID from the local `plugin.json`.

When the ID already exists:

- do not modify `store-plugin.json`
- tell the user that the plugin is already listed
- include the existing matching entry or its location so the user can verify it quickly

When the ID does not exist:

- continue to create a new entry
- preserve the file's existing formatting and ordering style

## Build The Store Entry

Use the field template and sourcing rules in [references/store-plugin-entry.md](./references/store-plugin-entry.md).

Apply these rules while building the JSON object:

- Keep `Id`, `Name`, `Author`, `Version`, `MinWoxVersion`, and `Runtime` aligned with local plugin metadata.
- Prefer a plain `Description` string when no i18n payload exists.
- Add `I18n` only when the repository already has trustworthy localized copy.
- Convert local OS names to the store style: `Windows`, `Darwin`, `Linux`.
- Set `DateCreated` and `DateUpdated` to the current local timestamp in `YYYY-MM-DD HH:MM:SS`.
- Use raw GitHub content URLs for `IconUrl` and `ScreenshotUrls`.
- Use the canonical repository URL for `Website`.
- Use the latest release asset URL for `DownloadUrl`.

Before editing `store-plugin.json`, re-check that the generated URLs and filenames match the actual repository layout and release asset names.

## Prepare The PR

After editing the cloned fork workspace:

1. Create a branch such as `codex/add-<plugin-name>-store-entry`.
2. Commit only the `store-plugin.json` change.
3. Push to the user's fork.
4. Open a PR targeting `Wox-launcher/Wox`.

Use a concise PR title such as `Add <Plugin Name> to store`.

Use a PR body that includes:

- what plugin was added
- the repository URL
- the release download URL
- screenshots or icon coverage when relevant

## Report Back To The User

Always end with one of these outcomes:

- `Already listed`: explain that no submission is needed.
- `Ready to submit`: summarize the new store entry and PR URL.
- `Waiting for fork`: tell the user to finish the web fork flow before continuing.
- `Blocked`: list the missing metadata or missing public assets that prevented submission.
