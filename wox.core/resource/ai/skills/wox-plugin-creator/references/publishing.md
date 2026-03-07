# Packaging and Publishing

- SDK plugins: run `make publish` inside the template repo (Node.js/Python).
- Script plugins: no build step; zip only if needed.
- For publishing into the Wox plugin store, read `references/store_publishing.md`.
- For Wox store metadata, do not assume repo-only files are available. Use the plugin store requirements or template guidance that ships with the plugin/template you are publishing.
- If you need a .skill file for Codex skills, use the skill packaging flow (not plugin packaging).
