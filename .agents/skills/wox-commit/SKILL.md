---
name: wox-commit
description: Generate a repository-style commit message from the current Wox working tree, stage the intended changes, and create the commit. Use when the user asks to commit current changes, generate a commit message and commit, or explicitly invokes `$wox-commit`.
---

# Wox Commit

## Contract

Treat explicit invocation as permission to create one local commit. Do not amend, push, or bypass hooks unless the user separately requests it.

## Workflow

1. Read the repository instructions and inspect `git status --short`, unstaged changes, staged changes, and relevant untracked files.
2. Separate the user's intended change from unrelated existing work. If the working tree contains multiple unrelated changes and intent cannot be determined, ask which group to commit.
3. Derive a concise message from the actual diff and recent commit style. Prefer `type(scope): imperative summary` for normal code changes; use the repository's established wording for automated, release, or store updates.
4. Stage only the exact intended paths with `git add -- <paths>`. Never stage credentials, local artifacts, or unrelated changes.
5. Review `git diff --cached --check` and `git diff --cached`. Stop if the staged diff is empty or does not match the intended change.
6. Run `git commit -m "<message>"`. Do not use `--no-verify`.
7. If a hook fails, fix only an in-scope issue, restage, and retry. Otherwise report the failure without creating a partial workaround.
8. Confirm the result with `git status --short` and `git show --stat --oneline --summary HEAD`.

## Message Rules

- Describe the reason or behavior change, not a file list.
- Keep the subject specific, imperative, and free of trailing punctuation.
- Choose the narrowest accurate scope already used by nearby commits.
- Use `feat`, `fix`, `refactor`, `docs`, `test`, `build`, or `chore` only when it matches the diff.
- Add a body only when the subject cannot capture a non-obvious behavioral constraint.

## Completion

Report the commit hash and subject, plus any changes intentionally left uncommitted.
