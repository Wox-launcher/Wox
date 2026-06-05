# Installation

Choose the installer that matches how you normally manage desktop apps. Package managers are easier to update; the release archive is useful when you want a portable copy. Wox uses the stable update channel by default.

## Package Managers

| Platform | Method | Command |
| --- | --- | --- |
| macOS | Homebrew | `brew install --cask wox` |
| Windows | Winget | `winget install -e --id Wox.Wox` |
| Windows | Scoop | `scoop install extras/wox` |
| Arch Linux | AUR | `yay -S wox-bin` |

After installation, start Wox from your app launcher or run the installed executable once. Wox creates its data directory on first launch.

## Manual Download

Download the latest stable package from [GitHub Releases](https://github.com/Wox-launcher/Wox/releases) if your platform does not have a package-manager entry yet or you prefer a portable setup.

## Update Channels

Wox checks the stable update channel by default. To receive beta prereleases, open **Settings -> General -> Update channel** and choose **Beta channel**. Beta users receive beta prereleases and later stable releases; stable users do not receive prereleases automatically.

### Windows

1. Download the Windows archive from Releases.
2. Extract it to a directory you control, such as `C:\Tools\Wox`.
3. Run `Wox.exe`.

If Windows SmartScreen asks for confirmation, check that the file came from the official Wox release page before continuing.

### macOS

1. Download the macOS disk image from Releases.
2. Open the image and drag Wox into `Applications`.
3. Start Wox from `Applications`.

If macOS blocks the first launch, open Wox from Finder once and choose **Open** from the confirmation dialog.

### Linux

1. Download the Linux archive from Releases.
2. Extract it to a stable location, such as `~/Applications/wox`.
3. Run `./wox`.

If the binary is not executable after extraction, run:

```bash
chmod +x ./wox
```

## User Data

Wox keeps settings, plugin data, cache, and logs outside the application directory:

| Platform | Data directory | Logs |
| --- | --- | --- |
| Windows | `%USERPROFILE%\.wox` | `%USERPROFILE%\.wox\log` |
| macOS | `~/.wox` | `~/.wox/log` |
| Linux | `~/.wox` | `~/.wox/log` |

Back up this directory if you want to move your configuration to another machine.

## Uninstall

Remove the application first, then decide whether to keep user data.

### Windows

- Winget: `winget uninstall -e --id Wox.Wox`
- Scoop: `scoop uninstall wox`
- Manual install: delete the extracted Wox directory
- Full reset: delete `%USERPROFILE%\.wox`

### macOS

- Homebrew: `brew uninstall --cask wox`
- Manual install: remove Wox from `Applications`
- Full reset: remove `~/.wox`

### Linux

- AUR: remove `wox-bin` with your AUR helper or package manager
- Manual install: delete the extracted Wox directory
- Full reset: remove `~/.wox`
