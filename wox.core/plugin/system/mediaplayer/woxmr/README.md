WoxMR (MediaRemote XS Bridge)

Why do we need WoxMR?

- On macOS 15+, Apple restricts the private MediaRemote framework: only processes with bundle ids starting with `com.apple.*` may call it.
- The Wox process is not `com.apple.*`, so calls would be rejected even with dynamic loading from Go/CGO.
- The system `/usr/bin/perl` process has the bundle id `com.apple.perl`, which is permitted to access MediaRemote.
- Therefore we run MediaRemote inside the `com.apple.perl` process and pass results back to Wox.

How it works

- We ship a tiny Perl XS module `woxmr.bundle` that is loaded and executed inside the Perl process.
- In the XS module (Objective‑C), we dynamically load MediaRemote (dlopen + dlsym) and call:
  - `MRMediaRemoteGetNowPlayingInfo`
  - `MRMediaRemoteGetNowPlayingApplicationIsPlaying`
  - `MRMediaRemoteGetNowPlayingApplicationPID`
- We assemble Now Playing data (title/artist/album/duration/position/playing, plus appName/bundleIdentifier) and return JSON to Perl.
- Wox runs a small Perl adapter (adapter.pl) to get the JSON and then parses it.

Why not Go/CGO directly?

- Even with dynamic loading from CGO, the caller remains the Wox process (not `com.apple.*`) and will be denied by the system.
- Moving the callsite into `/usr/bin/perl` (com.apple.perl) naturally bypasses the restriction with zero extra requirements on end‑user machines.

Why not an external framework?

- Avoid shipping a large private framework: simpler distribution, signing, and maintenance.
- We only ship a tiny `woxmr.bundle` (XS) + `WoxMR.pm` + `adapter.pl`: small, stable, and maintainable.

Build & distribution

- Build happens only on a developer machine (requires Xcode CLT and system Perl).
- End users need no dependencies. Distribute:
  - `resource/others/woxmr/woxmr.bundle`
  - `resource/others/woxmr/WoxMR.pm`
  - `resource/others/woxmr/adapter.pl`
- The top‑level Makefile exposes `woxmr-build`:
  - It auto‑cleans, builds for the current arch (arm64/x86_64), force‑signs, and copies artifacts to the paths above.
  - It is also invoked automatically by the overall `make build`.

Runtime flow

1. Wox runs adapter.pl:
   - `/usr/bin/perl resource/others/woxmr/adapter.pl get`
2. adapter.pl uses `WoxMR` to load and execute the XS code inside Perl:
   - The XS code accesses MediaRemote and returns a JSON string.
3. Wox parses the JSON and displays the Now Playing information.
