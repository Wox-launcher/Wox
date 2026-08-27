---
name: wox-commit
description: Generate a repository-style commit message from the current Wox working tree without modifying the repository. Use when the user asks for a commit message or explicitly invokes `$wox-commit`.
---

# Wox Commit Message

## Contract

Treat explicit invocation as permission to inspect the current Wox diff and generate a commit message only. Do not stage, commit, amend, push, or otherwise modify the repository.

## Workflow

1. Read the repository instructions and inspect `git status --short`, the unstaged and staged diffs, and relevant untracked files.
2. Infer every distinct behavior change from the actual diff and recent commit style. Do not ask the user to pick one feature when several unrelated changes are present.
3. Prefer `type(scope): imperative summary` for normal code changes; use the repository's established wording for automated, release, or store updates.
4. Always return both a subject and a body in one copy-ready block.

## Message Rules

- Describe the reason or behavior change, not a file list.
- Keep the subject specific, imperative, and free of trailing punctuation.
- Choose the narrowest accurate scope already used by nearby commits. When the tree spans unrelated areas, omit a misleading narrow scope rather than picking one feature.
- Use `feat`, `fix`, `refactor`, `docs`, `test`, `build`, or `chore` only when it matches the diff.
- Cover every distinct change in the same message. Unrelated features still belong in one subject-plus-body: name each behavior so a reviewer can see all of them without reading the file list.
- When one change dominates, put it in the subject and name the others in the body. When two or more are peer features, name both in the subject if they fit; otherwise use a broader subject and give each change its own sentence in the body.
- Always write a body that expands the why or the non-obvious behavior. One or two sentences is enough for a single change; use one short sentence per distinct change when the tree contains more than one. Do not leave the body empty.
- Do not run formatting, tests, hooks, or other validation; the caller has already completed validation before requesting the message.

## Completion

Return exactly one fenced text block the user can copy as a full commit message: subject, a blank line, then the body. After the block, add a brief note only if the diff is empty. Do not claim that a commit was created.

```text
type(scope): imperative summary

Body that explains the why or the non-obvious behavior.
```
