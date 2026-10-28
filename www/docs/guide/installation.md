# Installation

Choose the installer that matches how you normally manage desktop apps. Package managers are easier to update; the release executable is useful when you want a portable copy. Wox uses the stable update channel by default.

## Package Managers

| Platform | Method | Command |
| --- | --- | --- |
| macOS | Homebrew | `brew install --cask wox` |
| Windows | Winget | `winget install -e --id Wox.Wox` |
| Windows | Scoop | `scoop install extras/wox` |
| Arch Linux | AUR | `yay -S wox-bin` |
| Debian / Ubuntu | `.deb` | Download `wox-linux-amd64.deb` from [Releases](https://github.com/Wox-launcher/Wox/releases), then `sudo apt install ./wox-linux-amd64.deb` |
| Fedora / RHEL | `.rpm` | Download `wox-linux-amd64.rpm` from [Releases](https://github.com/Wox-launcher/Wox/releases), then `sudo dnf install ./wox-linux-amd64.rpm` |
| openSUSE | `.rpm` | Download `wox-linux-amd64.rpm` from [Releases](https://github.com/Wox-launcher/Wox/releases), then `sudo zypper install ./wox-linux-amd64.rpm` |

After installation, start Wox from your app launcher or run the installed executable once. Wox creates its data directory on first launch.

## Manual Download

Download the latest stable package from [GitHub Releases](https://github.com/Wox-launcher/Wox/releases) if your platform does not have a package-manager entry yet or you prefer a portable setup.

## Update Channels

Wox checks the stable update channel by default. To receive beta prereleases, open **Settings -> Update -> Update channel** and choose **Beta channel**. Beta users receive beta prereleases and later stable releases; stable users do not receive prereleases automatically.

### Windows

1. Download `wox-windows-amd64.exe` from Releases.
2. Place it in a directory you control, such as `C:\Tools\Wox`.
3. Run the downloaded executable.

To carry settings and plugins with this copy, create a `wox-user` folder next to the executable before the first launch. See [Portable user data](#portable-user-data).

If Windows SmartScreen asks for confirmation, check that the file came from the official Wox release page before continuing.

### macOS

1. Download the macOS disk image from Releases.
2. Open the image and drag Wox into `Applications`.
3. Start Wox from `Applications`.

If macOS blocks the first launch, open Wox from Finder once and choose **Open** from the confirmation dialog.

### Linux

1. Prefer a package when available:
   - Debian / Ubuntu: install the `.deb` from Releases with `sudo apt install ./wox-linux-amd64.deb`
   - Fedora / RHEL: install the `.rpm` from Releases with `sudo dnf install ./wox-linux-amd64.rpm`
   - openSUSE: install the `.rpm` from Releases with `sudo zypper install ./wox-linux-amd64.rpm`
   - Arch Linux: install `wox-bin` from the AUR
   - AppImage: download `wox-linux-amd64.AppImage`, mark it executable, and run it
2. Or download `wox-linux-amd64` from Releases, place it in a stable directory, make it executable, and run it.

If the binary or AppImage is not executable after download, run:

```bash
chmod +x ./wox-linux-amd64
# or
chmod +x ./wox-linux-amd64.AppImage
```

## User Data

Wox keeps settings, plugin data, cache, and logs outside the application directory:

| Platform | Data directory | Core log |
| --- | --- | --- |
| Windows | `%USERPROFILE%\.wox` | `%USERPROFILE%\.wox\log\wox.log` |
| macOS | `~/.wox` | `~/.wox/log/wox.log` |
| Linux | `~/.wox` | `~/.wox/log/wox.log` |

Back up this directory if you want to move your configuration to another machine.

### Portable user data

Create a `wox-user` directory next to the executable before you start that copy of Wox. Wox then stores settings, plugins, themes, and the user database in that directory.

Logs, plugin hosts, caches, models, and the file index stay in the per-machine data directory above. Those files differ between computers. Wox does not create `wox-user` beside the executable by itself. Remove that directory to use the normal user-data location again.

## Uninstall

Remove the application first, then decide whether to keep user data.

### Windows

- Winget: `winget uninstall -e --id Wox.Wox`
- Scoop: `scoop uninstall wox`
- Manual install: delete the downloaded executable
- Full reset: delete `%USERPROFILE%\.wox`

### macOS

- Homebrew: `brew uninstall --cask wox`
- Manual install: remove Wox from `Applications`
- Full reset: remove `~/.wox`

### Linux

- AUR: remove `wox-bin` with your AUR helper or package manager
- Debian / Ubuntu: `sudo apt remove wox`
- Fedora / RHEL: `sudo dnf remove wox`
- openSUSE: `sudo zypper remove wox`
- Manual or AppImage install: delete the binary or AppImage file
- Full reset: remove `~/.wox`
