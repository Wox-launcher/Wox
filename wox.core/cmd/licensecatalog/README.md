# License catalog sources

The release build generates an offline catalog for Settings → About → Open
Source Licenses. It follows the target's Go build graph and includes the
Go runtime, SQLite, signed libghostty-vt distribution notices, Wayland
protocol code, and the GTK Layer Shell library copied into the Linux AppImage.

The Node.js host uses ncc's bundle license report. The Raycast build reads
license files from packages that contribute code to the esbuild output.
The Python zipapp provides wheel metadata, bundled license files and the
shiv bootstrap version. The Windows Flow host supplies its NuGet dependency
list in its published `.deps.json`.

Versioned files in `notices/` supply original texts omitted from the built
artifacts. Update the relevant file when the corresponding package changes.

- [loguru 0.7.3](https://github.com/Delgan/loguru/blob/0.7.3/LICENSE)
- [shiv 1.0.8](https://github.com/linkedin/shiv/blob/1.0.8/LICENSE)
- [Flow.Launcher.Plugin 5.3.2](https://github.com/Flow-Launcher/Flow.Launcher/blob/9fe822b398ad9541a1a35fb1c688a006a27f2f9d/LICENSE)
- [JetBrains.Annotations](https://github.com/JetBrains/JetBrains.Annotations/blob/main/license.md)
- [ModernWpfUI 0.9.4](https://github.com/Kinnara/ModernWpf/blob/v0.9.4/LICENSE)

When adding a build step that embeds third-party files, include its notices
with that build step and register the input here. Build-only tools, dependencies
supplied by the operating system, and plugins or runtimes fetched separately
are outside the catalog's scope.
