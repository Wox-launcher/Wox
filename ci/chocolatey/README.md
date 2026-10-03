# Chocolatey releases

Run `make chocolatey-update` on Windows with Node.js 20+, an authenticated GitHub CLI (`gh auth login`), and Chocolatey on PATH. The command uses the latest published stable GitHub release, not the working tree version.

Configure a Chocolatey API key once using an account that maintains the `wox` package:

```powershell
choco apikey add --source="https://push.chocolatey.org/" --key="<API_KEY>"
```

The command checks whether that exact version already exists in Chocolatey, downloads the Windows amd64 release, computes SHA256 and compares it with GitHub's digest when available, renders the package sources, and runs `choco pack`. It then asks for confirmation before submitting with `choco push`. Submission still requires Chocolatey moderation; an existing version is skipped even if it is awaiting moderation.

```sh
make chocolatey-update
make chocolatey-update CHOCOLATEY_ARGS=--dry-run
make chocolatey-update CHOCOLATEY_ARGS=--pack-only
make chocolatey-update CHOCOLATEY_ARGS=--yes
```

- `--dry-run` generates sources without invoking Chocolatey or publishing. It also works on macOS and Linux.
- `--pack-only` builds the package without publishing so it can be tested first.
- `--yes` submits without prompting, for unattended runs on a Windows runner with a configured API key.

Generated files remain in `release/chocolatey/<version>/`, which is ignored by Git. The downloaded installer is temporary; the package contains scripts that download the release from GitHub. The scripts preserve the existing Chocolatey installation path and Start Menu shortcut.

Before submitting a packaging change, test a fresh install, upgrade from the previous package, and uninstall in a disposable Windows environment. Exit Wox before upgrading or uninstalling because these operations replace or remove its executable. For example, after generating version 2.4.5:

```powershell
choco install wox --version=2.4.5 --source="./release/chocolatey/2.4.5" -y
# In a separate environment with the previous version installed:
choco upgrade wox --version=2.4.5 --source="./release/chocolatey/2.4.5" -y
choco uninstall wox -y
```

API keys belong in Chocolatey's credential store (or CI secrets used to configure it), never in templates or source control. `--yes` does not schedule updates or modify the GitHub release workflow; invoke the target after the stable release assets are available.
