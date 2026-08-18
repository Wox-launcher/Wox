---
name: wox-commit
description: Generate a repository-style commit message from the current Wox working tree without modifying the repository. Use when the user asks for a commit message or explicitly invokes `$wox-commit`.
---

# Wox Commit Message

## Contract

Treat explicit invocation as permission to inspect the current Wox diff and generate a commit message only. Do not stage, commit, amend, push, or otherwise modify the repository.

## Workflow

1. Read the repository instructions and inspect `git status --short`, the unstaged and staged diffs, and relevant untracked files.
2. Infer the intended change from the actual diff and recent commit style. If multiple unrelated changes make the message ambiguous, ask which change the message should describe.
3. Prefer `type(scope): imperative summary` for normal code changes; use the repository's established wording for automated, release, or store updates.
4. Return one concise proposed subject and add a body only when the subject cannot capture a non-obvious behavioral constraint.

## Message Rules

- Describe the reason or behavior change, not a file list.
- Keep the subject specific, imperative, and free of trailing punctuation.
- Choose the narrowest accurate scope already used by nearby commits.
- Use `feat`, `fix`, `refactor`, `docs`, `test`, `build`, or `chore` only when it matches the diff.
- Add a body only when the subject cannot capture a non-obvious behavioral constraint.
- Do not run formatting, tests, hooks, or other validation; the caller has already completed validation before requesting the message.

## Completion

Report the proposed commit subject, optional body, and a brief note if the diff is empty or ambiguous. Do not claim that a commit was created.
