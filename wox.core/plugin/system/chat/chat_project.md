# AI Chat projects

Projects belong to AI Chat. The plugin owns metadata, membership, persistence and
per-turn context. The chat controller owns the picker/editor draft and collapsed
history groups. Launcher and dedicated chat windows share the same view props.

`project:{id}` contains synced metadata only. `project-local:{id}` contains the
current device's directories and primary directory and is persisted with
`IsLocal=true`. Chat records and their lightweight summaries carry an optional
`ProjectId`; absent values preserve ordinary-chat behavior without a migration.
The last explicitly selected project ID is synced like the default model. New
chat previews use it when the project still exists; a missing project falls back
to no project without rewriting the stored choice.
Deleting a project detaches its chats and removes the project in one settings/
oplog transaction. Membership-only writes retain unknown chat JSON fields.

Each send snapshots the project configuration. Runtime prompts describe the
source directories but never include other chats or preload entire folders.
The primary directory is passed through request context to Wox's file tools and
shell subprocesses, including calls routed from installed CLI providers. No
process-wide working directory or tool-registry mutation is involved. Directory
edits take effect on the next turn; an unavailable primary directory blocks a
new project request rather than falling back to the Wox process directory.

Projects are context, not a filesystem sandbox. Enabled tools and absolute paths
retain their existing semantics. A project synced to another device requires
local directory binding before sending; its history remains readable.

The composer places a monochrome `control.folder-open` project selector before
the model selector, with an explicit no-project choice. Empty projects remain in
the history drawer. Narrow composers reserve a separate toolbar line for the
project selector.

Project and model selection share the compact composer chip and searchable
catalog popup, including keyboard navigation and current-selection markers.
The history drawer has a collapsible Projects section; its plus action creates
a project, while each project expands its chats and exposes new-chat, edit,
and delete actions on hover or keyboard focus. Project deletion has a separate
confirmation that preserves conversations and files. Unassigned chats appear in
an independently collapsible Recents section. Project editing uses a content-sized
dialog with trailing-aligned actions.

Each turn reads `AGENTS.md` (or `agents.md`) at each configured source root,
with up to 32 KiB per file included as directory-scoped guidance. Explicit user
instructions take precedence. The model is instructed to check for more specific
`AGENTS.md` files before working in nested directories; nested files are not
preloaded. Project-local `.agents/skills/**/SKILL.md` manifests are discovered
for the active project, and their full content is loaded only when invoked.
The primary root wins when project skills share a name, and project skills take
precedence over global skills of the same name. Project resources stay local and
are scoped to the current project request; they are not added to the global
skill registry or synchronized as project settings.

Explicit project choices also increment a device-local usage count. Search uses a
capped logarithmic bonus based on that count, so frequent projects can move ahead
of similarly matched results without overwhelming a strong exact match. Empty
searches retain their grouped browsing order. Usage is removed with the project.
