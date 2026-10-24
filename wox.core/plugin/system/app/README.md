# Application launching

Normal Open keeps the existing activation-first behavior. Open New Window prefers an explicit window-creation strategy and reuses the normal Open action when no strategy is available. Errors from a supported launch are reported without retrying an ambiguous launch or cancelled elevation request.

Open New Window is always shown, including for MRU results. Support is resolved only when invoked, using the current shortcut or desktop entry. Unsupported entries launch or activate the app exactly as Open does. Query/action construction does not read launch metadata.

- Windows: recognize executable identities and preserve the original shortcut's arguments, directory, show mode and elevation through `util/shell`. Explicit executable launches accept AppUserModel.ID, IsDualMode and ToastActivatorCLSID metadata found in Firefox and Chrome shortcuts; default Shell activation and deduplication remain strict. Opaque Shell entries, incompatible launch modes and packaged applications without a supported executable entry fall back to Open.
- macOS: use `/usr/bin/open -n -a` and bundle-ID-based arguments for known browsers and VS Code. The operating system requests another instance; applications can still prohibit multiple instances.
- Linux: use GIO's desktop action API for declared `new-window`, `NewWindow`, or `new_window` actions. Otherwise, add known arguments to an in-memory copy of the desktop entry and launch through GIO. This preserves Exec field expansion, working directory and terminal handling without writing desktop files. Environment-specific behavior belongs in separate adapters if needed later; this path uses the shared freedesktop contract.

Keep application policies in this package. The Shell package exposes only generic launch metadata and execution capabilities.

Windows shortcut validation returns a specific rejection reason. Explicit launch attempts log that reason at debug level before falling back to Open; background indexing and deduplication do not log rejected shortcuts. Wire-format constants live in `util/shell/shortcut_format_windows.go`, and the default-activation and explicit-executable policies retain separate metadata allowlists.

Parameter references:

- [Firefox command line](https://firefox-source-docs.mozilla.org/browser/CommandLineParameters.html)
- [Chromium switches](https://chromium.googlesource.com/chromium/src/+/main/chrome/common/chrome_switches.cc)
- [VS Code command line](https://code.visualstudio.com/docs/configure/command-line)
- [Windows Terminal command line](https://learn.microsoft.com/en-us/windows/terminal/command-line-arguments)
- [Desktop entry actions](https://specifications.freedesktop.org/desktop-entry/latest/extra-actions.html)

Linux builds require the `gio-unix-2.0` pkg-config package (provided by GLib development packages alongside the existing GTK dependencies).
